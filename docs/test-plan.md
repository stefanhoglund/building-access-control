# Test Plan

Status: current as of the final submission pass (scenario/population and
Docker Compose work, on top of the earlier finalization pass: anomaly
detection, dashboard, fault experiments, performance/stress evaluation).

This document describes how the test suite is organized: unit tests,
end-to-end tests, integration tests, Docker Compose deployment
verification, and the standalone experiments. **The requirement-by-
requirement status table (all 54 `REQ-*` IDs, each with its implementing
component, test/experiment, evidence, status, and rationale) now lives in
`docs/traceability-matrix.md`** — this file used to duplicate that table;
keeping one copy avoids the two drifting apart. See that document for the
41 PASS / 6 PARTIAL / 7 UNIMPLEMENTED breakdown and for Part A, the
mandatory course pass/fail checklist.

## Unit tests (pass/fail requirement)

| Package | File | What it covers |
|---|---|---|
| `internal/decision` | `authz_test.go` | Authorization precedence, default deny, credential validation (13 tests) |
| `internal/decision` | `anomaly_test.go` | Anomaly scorer in isolation (6 tests) |
| `internal/decision` | `server_decide_test.go` | Full policy matrix: normal/anomalous/unauthorized (3 tests) |
| `internal/actuator` | `server_test.go` | Command confirmation, deduplication, unknown action, BuildSim-unavailable (4 tests) |
| `internal/modeltime` | `clock_test.go` | Model-time format/parse/advance determinism (4 tests) |

## End-to-end / integration tests

| File | Covers |
|---|---|
| `test/e2e/allow_test.go`, `deny_test.go`, `relock_test.go`, `passage_test.go` | Full ALLOW/DENY/timeout-relock/successful-passage sequences (FakeBuildSimClient) |
| `test/e2e/decision_restart_test.go` | Fail-closed → restart → recover, with measured recovery time |
| `test/e2e/scenario_test.go` (5 tests) | Named-scenario authorized crossing, denied access, after-hours anomaly, rapid-repeat anomaly, burst count — via the real `POST /scenario/run` interface |
| `internal/simulator/scenarios_test.go::TestRunScenarioSeedReproducibility` | Same seed → identical ordering; different seed → different ordering |
| `test/integration/realbuildsim_test.go` | Same ALLOW/relock sequence against the real BuildSim binary |

## Docker Compose deployment verification

Not `go test` — a manual verification pass against the real containerized
stack (`docker compose build && docker compose up -d`), documented in full
in `docs/deployment.md`. All items below were directly observed, not
assumed:

| Check | Result |
|---|---|
| All 9 long-running Compose services (8 project + buildsim) reach running/healthy state; the 10th service, the one-shot `door-seed` init container, exits as expected (status 0) | **PASS** |
| Initial state: door-1 exists, LOCKED, CLOSED | **PASS** |
| `authorized-passage`: full ALLOW → UNLOCK → OPEN → occupancy change → CLOSE → RELOCK_AFTER_PASSAGE → LOCKED | **PASS** |
| `unauthorized-access`: DENY, no UNLOCK, no crossing, occupancy unchanged | **PASS** |
| `after-hours-access`: ALLOW + ANOMALOUS/`UNUSUAL_ACCESS_TIME` + `alert:true`, final_action unaffected | **PASS** |
| `rapid-repeat-access`: first attempt NORMAL, subsequent attempts ANOMALOUS/`REPEATED_ATTEMPT` | **PASS** |
| `docker compose restart decision` then a fresh authorized attempt → full recovery | **PASS** (see `docs/deployment.md` section 7 for a self-inflicted overlap artifact caught and corrected during this check — not a Docker/regression issue) |
| Stop `actuator`, present a card → command issued, no outcome, no false physical confirmation, door stays LOCKED; restart → recovery | **PASS** |
| Dashboard reachable at `http://localhost:8085/`, renders live evidence (lock state, decisions, anomaly, occupancy) | **PASS** |
| Evidence (`data/evidence.jsonl`, `evidence/scenarios/runs.jsonl`) survives a full `docker compose restart` | **PASS** |
| Native workflow regression after all Docker changes: `gofmt`, `go vet`, `go test ./...`, `-race`, `-tags=integration` | **PASS** (all green) |
| Final verification pass (fresh run): all of the above repeated against a rebuilt stack | **PASS** — `evidence/final-verification/` (logs, timestamps, commands) |
| Found during final verification: a bulk `docker compose restart` (every container at once) could race `door-seed` against `buildsim`'s own restart, leaving door-1 unseeded | **Found, root-caused, fixed, and reverified** — `compose.yaml` `door-seed` now retries (`--retry-all-errors`); re-run twice, confirmed fixed both times. See `evidence/final-verification/README.md` and `docs/deployment.md` §3. |

## Experiments (not `go test`, run on demand)

| Tool | Purpose | Evidence directory |
|---|---|---|
| `go run ./cmd/latencybench` | Access-decision and actuation-confirmation latency | `evidence/latency/` |
| `go run ./cmd/stressbench` | Concurrent-load breaking-point | `evidence/stress/` |
| Manual (`scripts/run-local.sh demo-decision-failure`, or by hand) | Decision Service kill/restart against the real system | `evidence/manual-decision-restart-fault/` |
| Manual (see `evidence/actuator-failure/README.md`) | Real BuildSim kill during an ALLOW | `evidence/actuator-failure/` |
| Manual (see `evidence/anomaly/README.md`) | Three required anomaly scenarios against the live system | `evidence/anomaly/` |

## Honest summary

**41 PASS / 6 PARTIAL / 7 UNIMPLEMENTED, of 54 requirements.** The full
per-requirement breakdown, component mapping, and one-sentence rationale
for each is `docs/traceability-matrix.md` (Part B); the mandatory course
pass/fail checklist is the same document's Part A, all nine satisfied. The
remaining UNIMPLEMENTED items cluster around two themes not built: device
re-registration after restart (REQ-DIST-005/006), and data-quality fault
handling (REQ-AUTH-005, REQ-DATA-004/005, REQ-RES-003/005) — both
explicitly deferred, consistently, across this project's development, not
overlooked. See `docs/final-report-gap-analysis.md` for the prioritized
view of what remains and `docs/evaluation-results.md` for the measured
evidence behind every PASS that depends on a number rather than a
pass/fail test.
