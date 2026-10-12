# Deployment

Source of truth for runtime processes, containers, ports, health checks,
startup/restart behavior, and persistence (CLAUDE.md §1 item 7). Everything
in this course runs on one laptop — there is no edge/cloud split in
practice — so "deployment" here is about process/container composition,
isolation, and communication, not physical distribution.

Two supported ways to run the system exist side by side and must both keep
working:

1. **Native** — `scripts/run-local.sh` + the `make build/run/stop/status`
   targets, building and running eight real Go binaries directly on the
   host plus the real BuildSim binary (`docs/internal/D7065E/buildingsim`).
2. **Docker Compose** (CLAUDE.md §3: "Docker Compose is the default
   deployment mechanism") — `compose.yaml` at the repo root, runs the same
   eight processes plus BuildSim as containers.

Neither is a thin wrapper around the other: both start the same binaries
from the same source, configured only through command-line flags
(addresses/paths), never through Docker-specific branches in application
code.

## 1. Service / container mapping

| Service (compose name) | Binary | Listens | Talks to | Notes |
|---|---|---|---|---|
| `buildsim` | course-provided `buildsim` | `:9090` | — | external system (CLAUDE.md §4); its own Dockerfile/compose.yaml (`docs/internal/D7065E/buildingsim`), reused unmodified |
| `door-seed` | `curlimages/curl` (no project code) | — | `buildsim` | one-shot init: PUTs door-1 as LOCKED/CLOSED, then exits (BuildSim has no persistence, CLAUDE.md §4) |
| `simulator` | `cmd/simulator` | `:8081` | `buildsim` | owns model time + occupancy/door-position truth; loads `config/population.json` + `config/scenarios.json` |
| `eventdata` | `cmd/eventdata` | `:8082` | `decision` | sole writer to the Historical Evidence Store (ADR-005) |
| `decision` | `cmd/decision` | `:8083` | `eventdata`, `actuator`, `simulator` | loads `config/system-config.json`; authorization + anomaly + policy + runtime assurance |
| `actuator` | `cmd/actuator` | `:8084` | `buildsim`, `eventdata` | issues LOCK/UNLOCK, verifies via BuildSim before confirming |
| `cardreader` | `cmd/cardreader` | *(none — polls out)* | `simulator`, `eventdata` | |
| `occupancysensor` | `cmd/occupancysensor` | *(none — polls out)* | `simulator`, `eventdata` | polls Simulator truth, never BuildSim (design-specification.md §4.5) |
| `doorstatesensor` | `cmd/doorstatesensor` | *(none — polls out)* | `buildsim`, `simulator` (model time only), `eventdata` | the only producer of `DoorStateObservation` |
| `dashboard` | `cmd/dashboard` | `:8085` | reads the evidence file directly | re-reads on every request, no API of its own |

Each row is one independently deployable, independently restartable
process/container (CLAUDE.md §3, §11) — nothing here is merged into a
monolith. `compose.yaml` builds all eight project binaries from one
multi-stage `Dockerfile` and runs eight containers from that one image,
each with a different `command:`; this mirrors `scripts/run-local.sh`,
which already builds eight separate binaries from one `go build` loop and
runs eight separate processes.

## 2. Networking

**Native**: every base-URL flag defaults to `127.0.0.1`/`localhost` with
the ports above — correct because every process is a sibling on the same
host loopback interface.

**Docker Compose**: Compose's default bridge network gives every service a
DNS name equal to its service name (`buildsim`, `simulator`, `eventdata`,
`decision`, `actuator`, `cardreader`, `occupancysensor`, `doorstatesensor`,
`dashboard`) — `127.0.0.1` inside a container would mean *that container*,
not a sibling. `compose.yaml`'s `command:` arrays simply pass each
process's existing CLI flags (`-buildsim-url`, `-eventdata-url`,
`-decision-url`, `-actuator-url`, `-simulator-url`, ...) the internal DNS
name instead of `127.0.0.1` — e.g. `decision` is started with
`-eventdata-url=http://eventdata:8082 -actuator-url=http://actuator:8084
-simulator-url=http://simulator:8081`. **No `cmd/*/main.go` file changes
were needed for this** — addressing is entirely a Compose-file
configuration concern, exactly as CLAUDE.md requires ("do not add
Docker-specific branching in application logic; configuration should
determine addresses").

Only three host ports are published, each loopback-only
(`127.0.0.1:<port>:<port>`), matching "expose only needed host ports":

- `9090` — BuildSim API/viewer
- `8081` — Simulator, so an operator can `POST /scenario/run` from the host
- `8085` — Dashboard

`eventdata`/`decision`/`actuator` and the three poller processes are not
published — nothing outside the Compose network needs to reach them
directly.

## 3. Door-1 initialization

BuildSim keeps no state across restarts (CLAUDE.md §4), so door-1 must be
(re-)created as LOCKED/CLOSED every time the stack starts. Natively,
`scripts/run-local.sh start` does this with one `curl -X PUT /api/doors`
call (`seed_door`). In Compose, the same call is the `door-seed` service: a
tiny one-shot container (`curlimages/curl`, no project image, no shell
script baked into the project Dockerfile) that waits for BuildSim's own
healthcheck (`condition: service_healthy`, inherited from BuildSim's
Dockerfile `HEALTHCHECK`) and then exits. `simulator`, `actuator`, and
`doorstatesensor` — the three services that touch BuildSim's door
directly — declare `depends_on: door-seed: condition:
service_completed_successfully`, so they never race door-1's creation.
Re-running `door-seed` (e.g. after `docker compose start actuator`
re-evaluates its dependency chain) is harmless: `PUT /api/doors` is a
wholesale, idempotent replace, which is exactly why CLAUDE.md §4 requires
device/equipment registration to be idempotent in the first place. No
fixed sleep delays are used anywhere in the startup path — only this one
explicit, checkable readiness condition.

`door-seed`'s `curl` call also carries `--retry 10 --retry-all-errors
--retry-delay 1`. This is not defensive decoration: final verification
(`evidence/final-verification/README.md`) found that a bulk `docker compose
restart` (every container restarted together, not `up`) can race
`door-seed`'s one-shot run against `buildsim`'s own container coming back —
`depends_on: condition: service_healthy` is honored by `up`, not by a bulk
`restart`, so `door-seed` could attempt its call before Compose's internal
DNS had (re-)registered the `buildsim` hostname, failing outright
(`curl: (6) Could not resolve host: buildsim`) and leaving door-1 unseeded.
The retry flags make this one step tolerant of that transient race
regardless of which restart path triggered it; they do not change normal
startup timing (a fresh `up` still only runs `door-seed` once `buildsim` is
already healthy). Restarting any *single* project service (`decision`,
`actuator`, etc.) is unaffected by this — the race only exists when
`buildsim` itself is restarting at the same moment.

## 4. Persistence

Two host-visible bind mounts carry everything that must survive a
container or full-stack restart:

- `./data:/app/data` — `data/evidence.jsonl`, the Historical Evidence
  Store (`eventdata`, read by `dashboard`).
- `./evidence:/app/evidence` — `evidence/scenarios/runs.jsonl`, the
  scenario-run reproducibility log (`simulator` only — see
  `docs/data-contracts.md` §10.1), alongside the existing experiment
  evidence directories (`evidence/decision-restart-fault/`, etc.).

Neither path is overridden inside the container: the Dockerfile sets
`WORKDIR /app`, so each process's existing native-default relative path
(`data/evidence.jsonl`, `evidence/scenarios/runs.jsonl`) already resolves
onto the bind mount without any Docker-specific flag value. Verified
directly: a full `docker compose restart` (every container) does not
reset or truncate either file (Phase 3 verification, item I).

`config/` is baked into the image at build time (`COPY config/
/app/config/`) rather than bind-mounted — there is no requirement to edit
configuration without a rebuild, so the simpler option was chosen.

## 5. Health checks and startup ordering

Deliberately minimal, per CLAUDE.md §3/§11 ("do not over-engineer
orchestration"):

- **BuildSim** keeps its own course-provided `HEALTHCHECK` (its `buildsim
  health --url ...` subcommand) — reused, not reinvented.
- **The eight project processes have no added healthchecks.** Every one of
  them already tolerates a temporarily-unavailable peer by design and by
  test (poll loops retry on their own schedule; `eventdata`'s
  forward-to-`decision` call and `decision`'s relock-ticker calls are
  logged-and-continue, never fatal; this is the same resilience property
  already proven in `evidence/decision-restart-fault/` and
  `evidence/actuator-failure/`). Adding healthchecks for these would only
  restate properties already verified, not materially improve startup
  correctness.
- The one place ordering is *not* self-healing — door-1 must exist before
  anything touches it — gets the one explicit `condition:
  service_completed_successfully` dependency described above. That is the
  only orchestration logic in `compose.yaml` beyond plain `depends_on`
  start ordering.

## 6. Why Compose

CLAUDE.md §3 names Docker Compose as this project's default deployment
mechanism directly, and the course's own BuildSim ships a Compose file —
reusing the same tool keeps the whole stack (including the external
system) describable and startable the same way. Compose's `depends_on` +
one shared image + bind-mounted volumes is exactly the amount of
orchestration eight independently-restartable, already-resilient
processes need; Kubernetes or a bespoke process supervisor would add
complexity (CLAUDE.md §3: "do not introduce Kubernetes or other
orchestration complexity unless explicitly required by an experiment")
without making anything more explainable or more correct.

## 7. Known limitation surfaced during verification

`internal/decision/server.go`'s per-door control state (`pending
map[string]doorControlState`, keyed by `door_id`, not by `decision_id`) is
single-slot: if a second ALLOW is issued for the same door before the
first one's relock has been observed and processed, the second overwrites
the first's bookkeeping entry. With exactly one simulated presentation in
flight at a time — the normal case for every named scenario — this never
happens. It was surfaced once during Docker verification by deliberately
overlapping leftover `rapid-repeat-access` presentations with a fresh
request immediately after a `decision` container restart; a clean,
isolated request immediately afterward completed its full
relock-after-passage cycle correctly, confirming the restart itself was
not the cause. This is a pre-existing property of the single-pending
per-door design (not introduced by, or specific to, the Docker Compose
work), and fixing it would mean redesigning decision's per-door state to
be per-decision or queued — out of scope for this phase (CLAUDE.md
§12: "do not implement multiple unrelated components in one change").
