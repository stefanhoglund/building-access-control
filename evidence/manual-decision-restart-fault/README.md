# Manual fault experiment: Decision Service unavailable → fail closed → restart → recover

Real-process capture, against the real BuildSim binary and all seven
deployed processes already running (not `FakeBuildSimClient`, not the
in-process test harness).

## Timeline (operator actions, wall-clock, this host)

| Time (UTC) | Action |
|---|---|
| 20:23:00.905 | Decision Service's real OS process (`exe/decision`, PID 29687) killed with `SIGKILL` |
| 20:23:06.712 | `card-allow` presented while Decision Service is down → `observation` recorded, nothing further |
| 20:39:04.178 | Decision Service restarted: prebuilt binary (`go build -o decision-manual-restart ./cmd/decision`) launched fresh, no carried-over state |
| 20:39:08.690–20:39:09.056 | fresh `card-allow` presented → full ALLOW → UNLOCK → OPEN → CLOSE → RELOCK_AFTER_PASSAGE → LOCK sequence recorded correctly |

## What this does and does not measure

This run **confirms correctness** (fail-closed held with a real killed
process against real BuildSim; recovery produced a correct decision with
no stale state) but the ~34-minute-to-eyeball, actually ~4.5s-from-restart
gap above is **not** a valid latency measurement: most of it is this
operator's own pacing between manual command invocations (confirming the
process was up, then issuing the next curl), not the system's own
recovery latency.

The clean, confound-free recovery-time measurement — restart issued to
first correct decision, with no human/tool pacing in between — is the
automated test's number: see `test/e2e/decision_restart_test.go` and the
session report for the measured value (~11–34 ms across 10 runs).

## Evidence

`evidence.jsonl` in this directory is lines 14 onward of the live
`data/evidence.jsonl` at the time of this experiment: the one
observation recorded during the outage (no decision, no command — fail
closed), followed by the full recovery sequence after restart.
