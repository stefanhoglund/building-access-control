#!/usr/bin/env bash
# Operator reproducibility script, not a deployment tool: builds each
# process as a real binary and tracks its real PID in .run/<name>.pid, so
# `stop` reliably kills what `start` launched. Using prebuilt binaries
# rather than `go run` is deliberate: `go run` spawns a wrapper process
# around the real compiled binary, and killing only the wrapper (e.g. by
# pattern-matching "go run ./cmd/X") leaves the real binary running and
# still holding its port — this was discovered the hard way during manual
# testing and is exactly the failure mode this script avoids.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

BIN_DIR="bin"
PID_DIR=".run"
BUILDSIM_BIN="docs/internal/D7065E/buildingsim/bin/buildsim"
BUILDSIM_PORT=9090
BUILDSIM_URL="http://127.0.0.1:${BUILDSIM_PORT}"

PROCESSES=(simulator eventdata decision actuator cardreader occupancysensor doorstatesensor dashboard)

mkdir -p "$BIN_DIR" "$PID_DIR" data evidence

build_all() {
  echo "building all processes into ${BIN_DIR}/ ..."
  for c in "${PROCESSES[@]}"; do
    go build -o "$BIN_DIR/$c" "./cmd/$c"
  done
  echo "build complete"
}

start_buildsim() {
  if curl -fs --max-time 1 "${BUILDSIM_URL}/healthz" >/dev/null 2>&1; then
    echo "BuildSim already running on :${BUILDSIM_PORT}"
    return
  fi
  if [ ! -x "$BUILDSIM_BIN" ]; then
    echo "error: ${BUILDSIM_BIN} not found or not executable" >&2
    exit 1
  fi
  "$BUILDSIM_BIN" start --port "$BUILDSIM_PORT" >"$PID_DIR/buildsim.log" 2>&1 &
  echo $! > "$PID_DIR/buildsim.pid"
  for _ in $(seq 1 50); do
    curl -fs --max-time 1 "${BUILDSIM_URL}/healthz" >/dev/null 2>&1 && break
    sleep 0.1
  done
  echo "BuildSim started (pid $(cat "$PID_DIR/buildsim.pid"))"
}

seed_door() {
  curl -fs -X PUT "${BUILDSIM_URL}/api/doors" \
    -H 'Content-Type: application/json' \
    -d '[{"id":"door-1","name":"PUBLIC to EMPLOYEE door","kind":"door","level":"level0","room":"A109","state":"closed","lock_state":"locked"}]' \
    >/dev/null
  echo "door-1 seeded LOCKED/CLOSED (BuildSim has no persistence — re-seed after every BuildSim restart)"
}

start_process() {
  local name="$1"
  if [ -f "$PID_DIR/$name.pid" ] && kill -0 "$(cat "$PID_DIR/$name.pid")" 2>/dev/null; then
    echo "$name already running (pid $(cat "$PID_DIR/$name.pid"))"
    return
  fi
  "$BIN_DIR/$name" >"$PID_DIR/$name.log" 2>&1 &
  echo $! > "$PID_DIR/$name.pid"
  echo "started $name (pid $(cat "$PID_DIR/$name.pid"))"
}

start_all() {
  build_all
  start_buildsim
  seed_door
  start_process simulator
  start_process eventdata
  sleep 0.3
  start_process decision
  start_process actuator
  sleep 0.3
  start_process cardreader
  start_process occupancysensor
  start_process doorstatesensor
  start_process dashboard
  echo
  echo "system up. Dashboard: http://localhost:8085/"
}

stop_all() {
  for f in "$PID_DIR"/*.pid; do
    [ -e "$f" ] || continue
    local pid name
    pid=$(cat "$f")
    name=$(basename "$f" .pid)
    if kill -0 "$pid" 2>/dev/null; then
      kill -9 "$pid" 2>/dev/null && echo "stopped $name (pid $pid)"
    fi
    rm -f "$f"
  done
}

status() {
  local any=0
  for f in "$PID_DIR"/*.pid; do
    [ -e "$f" ] || continue
    any=1
    local pid name
    pid=$(cat "$f")
    name=$(basename "$f" .pid)
    if kill -0 "$pid" 2>/dev/null; then
      echo "$name: running (pid $pid)"
    else
      echo "$name: NOT running (stale pidfile — cleaning up)"
      rm -f "$f"
    fi
  done
  [ "$any" = 1 ] || echo "nothing started by this script right now"
}

demo_allow() {
  curl -s -X POST http://localhost:8081/scenario/present -H 'Content-Type: application/json' \
    -d '{"reader_id":"reader-1","card_id":"card-allow","door_id":"door-1","direction":"PUBLIC_TO_EMPLOYEE"}'
  echo
  echo "presented card-allow; watch data/evidence.jsonl or http://localhost:8085/"
}

demo_deny() {
  curl -s -X POST http://localhost:8081/scenario/present -H 'Content-Type: application/json' \
    -d '{"reader_id":"reader-1","card_id":"card-deny","door_id":"door-1","direction":"PUBLIC_TO_EMPLOYEE"}'
  echo
  echo "presented card-deny; watch data/evidence.jsonl or http://localhost:8085/"
}

run_scenario() {
  local name="$1" seed="${2:-1}" count="${3:-0}" interval_ms="${4:-0}"
  curl -s -X POST http://localhost:8081/scenario/run -H 'Content-Type: application/json' \
    -d "{\"scenario\":\"${name}\",\"seed\":${seed},\"count\":${count},\"interval_ms\":${interval_ms}}"
  echo
  echo "ran scenario '${name}'; watch data/evidence.jsonl, evidence/scenarios/runs.jsonl, or http://localhost:8085/"
}

demo_normal_day() { run_scenario normal-day "${1:-42}"; }
demo_unauthorized() { run_scenario unauthorized-access "${1:-1}"; }
demo_after_hours() { run_scenario after-hours-access "${1:-1}"; }
demo_rapid_repeat() { run_scenario rapid-repeat-access "${1:-1}"; }
demo_burst() { run_scenario burst-access "${1:-1}" "${2:-50}" "${3:-20}"; }

demo_actuator_failure() {
  echo "--- stopping Door Actuator ---"
  if [ -f "$PID_DIR/actuator.pid" ]; then
    kill -9 "$(cat "$PID_DIR/actuator.pid")" 2>/dev/null || true
    rm -f "$PID_DIR/actuator.pid"
  fi
  echo "--- presenting card-allow while the Actuator is down (expect: ALLOW decision recorded, but no CONFIRMED outcome and no physical unlock) ---"
  run_scenario actuator-or-buildsim-failure
  sleep 1.0
  echo "--- restarting Door Actuator ---"
  start_process actuator
  sleep 0.3
  echo "--- presenting a fresh card-allow (expect: full ALLOW -> UNLOCK, CONFIRMED) ---"
  demo_allow
  echo
  echo "see evidence/actuator-failure/ for the documented version of this experiment"
}

demo_decision_failure() {
  echo "--- killing Decision Service ---"
  if [ -f "$PID_DIR/decision.pid" ]; then
    kill -9 "$(cat "$PID_DIR/decision.pid")" 2>/dev/null || true
    rm -f "$PID_DIR/decision.pid"
  fi
  echo "--- running the 'decision-service-failure' scenario while Decision Service is down (expect: observation only, no decision/command, door stays LOCKED) ---"
  run_scenario decision-service-failure
  sleep 1.5
  echo "--- restarting Decision Service ---"
  start_process decision
  sleep 0.3
  echo "--- presenting a fresh card-allow (expect: full ALLOW -> UNLOCK) ---"
  demo_allow
  echo
  echo "see evidence/decision-restart-fault/ (automated) and evidence/manual-decision-restart-fault/ (real-process, measured) for the documented version of this experiment"
}

usage() {
  cat <<'EOF'
Usage: scripts/run-local.sh <command>

  build                   build all process binaries into bin/
  start                   start BuildSim, seed door-1, start all processes + dashboard
  stop                    stop everything this script started
  status                  show which started processes are still running
  seed                    re-seed door-1 as LOCKED/CLOSED (needed after any BuildSim restart)
  demo-allow              present card-allow
  demo-deny               present card-deny
  demo-normal-day [seed]          run the 'normal-day' scenario (default seed 42)
  demo-unauthorized [seed]        run the 'unauthorized-access' scenario
  demo-after-hours [seed]         run the 'after-hours-access' scenario
  demo-rapid-repeat [seed]        run the 'rapid-repeat-access' scenario
  demo-burst [seed] [count] [ms]  run the 'burst-access' scenario
  demo-decision-failure   kill Decision Service, show fail-closed, restart, show recovery
  demo-actuator-failure   kill Door Actuator, show unconfirmed outcome, restart, show recovery

For the latency and stress experiments, run the standalone tools directly:
  go run ./cmd/latencybench -n 200 -interval 20ms
  go run ./cmd/stressbench -levels=1,10,50,100,200,400,800 -duration=2s

Named scenario definitions live in config/scenarios.json; the configured
simulated-person population lives in config/population.json
(design-specification.md section 3). See docs/scenarios.md.
EOF
}

case "${1:-}" in
  build) build_all ;;
  start) start_all ;;
  stop) stop_all ;;
  status) status ;;
  seed) seed_door ;;
  demo-allow) demo_allow ;;
  demo-deny) demo_deny ;;
  demo-normal-day) demo_normal_day "${2:-}" ;;
  demo-unauthorized) demo_unauthorized "${2:-}" ;;
  demo-after-hours) demo_after_hours "${2:-}" ;;
  demo-rapid-repeat) demo_rapid_repeat "${2:-}" ;;
  demo-burst) demo_burst "${2:-}" "${3:-}" "${4:-}" ;;
  demo-decision-failure) demo_decision_failure ;;
  demo-actuator-failure) demo_actuator_failure ;;
  *) usage ;;
esac
