# Fault experiment: BuildSim unavailable during an ALLOW → unverified, no false success

Verifies REQ-RES-002: "If an actuator command cannot be confirmed, the
resulting door state shall be marked as unverified and the failure shall
be recorded."

## Two captures in this directory's evidence.jsonl

**Lines 1–4 — ALLOW while BuildSim is down.** The real BuildSim process
was killed (`SIGKILL`) before presenting `card-allow`. The Decision
Service, which has no visibility into BuildSim's availability, still
produces a correct `ALLOW`/`ROLE_ALLOW` decision and issues the `UNLOCK`
command — this is correct: authorization is a logical decision, not a
physical confirmation. The Door Actuator's `SetDoorState` call to BuildSim
fails (connection refused), and the recorded outcome is:

```json
{"result":"UNVERIFIED","observed_state":""}
```

No `CONFIRMED`, no `observed_state` value is ever claimed. This is the
behavior under test: a failed physical write is never reported as if it
succeeded.

**Lines 5–13 — recovery after restart.** BuildSim was restarted (its
state resets on restart — in-memory only, matching
`design-specification.md` §10.8/architecture.md §9) and `door-1` was
re-seeded. A fresh `card-allow` presentation then produced the full
correct sequence: `CONFIRMED`/`UNLOCKED` → passage → `CONFIRMED`/`LOCKED`.

## Reproduction

```bash
# 1. baseline
curl -s http://127.0.0.1:9090/api/doors

# 2. kill the real BuildSim process
pgrep -f "buildsim start" | xargs kill -9

# 3. present while it's down
curl -X POST http://localhost:8081/scenario/present -H 'Content-Type: application/json' \
  -d '{"reader_id":"reader-1","card_id":"card-allow","door_id":"door-1","direction":"PUBLIC_TO_EMPLOYEE"}'

# 4. check evidence: expect decision=ALLOW, command=UNLOCK, outcome=UNVERIFIED/""

# 5. restart and re-seed
./docs/internal/D7065E/buildingsim/bin/buildsim start --port 9090 &
curl -X PUT http://127.0.0.1:9090/api/doors -H 'Content-Type: application/json' \
  -d '[{"id":"door-1","name":"PUBLIC to EMPLOYEE door","kind":"door","level":"level0","room":"A109","state":"closed","lock_state":"locked"}]'

# 6. fresh presentation confirms recovery
curl -X POST http://localhost:8081/scenario/present -H 'Content-Type: application/json' \
  -d '{"reader_id":"reader-1","card_id":"card-allow","door_id":"door-1","direction":"PUBLIC_TO_EMPLOYEE"}'
```

An automated, CI-reliable version of the same invariant (no external
BuildSim process needed) is
`internal/actuator/server_test.go::TestHandleCommand_BuildSimUnavailable_RecordsUnverifiedNotFalseSuccess`.

## Observed system-level consequence (worth noting honestly)

BuildSim is a single shared dependency for three different components at
once: the Door Actuator (lock/unlock), the Simulator (door-position
writes and occupancy mirroring), and the DoorStateSensor (door_position
polling). Killing it affects all three simultaneously — this experiment
only asserts on the Actuator's behavior, but the DoorStateSensor's polls
and the Simulator's writes were also failing (and logging errors) for the
same window, unobserved by any assertion here. A single-point-of-failure
dependency shared by three independent processes is a legitimate
architectural trade-off to name in the report's risk/limitations section.
