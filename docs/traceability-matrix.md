# Requirements Traceability Matrix

This is the definitive, final traceability document for the project. It
has three parts:

- **Part A** — the course's mandatory Stage-1 pass/fail checklist (a
  project that fails any one of these is not assessable at all).
- **Part B** — every internal project requirement (`docs/requirements.md`,
  54 `REQ-*` IDs), each with its implementing component, the test or
  experiment that verifies it, the evidence, a status, and a one-sentence
  rationale.
- **Part C** — a short map from clusters of Part B requirements to the
  course's graded Dimensions A–D, at the grade-4/5 evaluation-goal level.
  The full rubric-by-rubric audit (what evidence supports which grade
  level, and what the capping weaknesses are) is a separate, dedicated
  document: `docs/report-rubric-audit.md`. This section exists only so
  the requirement-level and dimension-level views are linked in one
  place.

`docs/test-plan.md` remains the place that describes *how* each test
suite is organized (unit/e2e/integration/experiments) and documents the
Docker Compose verification run; it no longer repeats the full
requirement-by-requirement status table — this document is now that
table's single source of truth, to avoid the two drifting apart.

No status here was upgraded relative to `docs/test-plan.md`'s existing,
already-verified status column (itself built incrementally and honestly
across this project's whole development) merely because an adjacent
feature exists. Where this matrix's status differs from an earlier
draft, it is because new evidence was produced (the scenario/population
and Docker Compose work), not because the bar for PASS moved.

---

## Part A — Mandatory course pass/fail requirements (Stage 1)

Verbatim from the course's grading criteria. **All nine are satisfied.**

| # | Requirement (verbatim, condensed) | Status | Evidence |
|---|---|---|---|
| 1 | Sensor, actuator, and autonomous decision-making components run as separate, independently deployable and restartable processes (e.g. Docker containers); not a single monolithic application. | **PASS** | 8 independent Go binaries (`cmd/simulator`, `cmd/eventdata`, `cmd/decision`, `cmd/actuator`, `cmd/cardreader`, `cmd/occupancysensor`, `cmd/doorstatesensor`, `cmd/dashboard`), each its own `compose.yaml` container; independent restartability directly demonstrated for Decision (`evidence/decision-restart-fault/`, `evidence/manual-decision-restart-fault/`, and a live `docker compose restart decision`) and Actuator (`evidence/actuator-failure/`, a live `docker compose stop/start actuator`) with the other processes continuing unaffected both times (`evidence/final-verification/`). |
| 2 | The system operates end-to-end: sensor data generated → transmitted → processed by an autonomous component → decision → actuator → visible BuildSim effect → feeds back into subsequent sensor readings. | **PASS** | `docs/diagrams/dynamic.d2` / `evidence/manual-allow-passage/evidence.jsonl` trace the full loop: Card Reader observation → Event/Data Service → Decision Service → Actuator → BuildSim `lock_state`/`door_position` → Simulator causes the physical crossing → DoorStateSensor/OccupancySensor observe the *new* BuildSim-reflected state → recorded again as evidence. Reproduced identically against the real BuildSim binary (`test/integration/realbuildsim_test.go`) and the Docker Compose stack (`evidence/final-verification/`). |
| 3 | At least one component performs autonomous decision-making based on sensor data and influences actuator behaviour; the logic is genuine, justified, and integrated. | **PASS** | `internal/decision`: deterministic rule-based authorization (ADR-001 precedence) + a deterministic rule-based anomaly scorer (`internal/decision/anomaly.go`), combined by an explicit decision policy (`handleDecide`) whose `final_action` drives real `UNLOCK`/`LOCK` commands to the Actuator. Justification for the rule-based (not ML) choice: `docs/decisions/`, `docs/scenarios.md` §4, and §7.3 of the report. |
| 4 | A data pipeline collects sensor data, stores it, and makes it available to other components for monitoring, analysis, decision-making, training, or evaluation. | **PASS** | `internal/eventdata` (collects, validates, forwards to `internal/decision`) + `internal/evidence` (append-only `data/evidence.jsonl`, survives restart — REQ-DATA-002) + `internal/dashboard` (consumes it for monitoring) + every `evidence/*` experiment directory (consumes it for evaluation). |
| 5 | An architecture document exists with: C4 Context diagram, C4 Container diagram, a requirements table, and documentation of key interfaces/communication paths. | **PASS** | `docs/diagrams/context.d2`/`container.d2` (+ rendered SVGs); `docs/requirements.md` (54 IDs); `docs/data-contracts.md` (every REST endpoint, schema, and example payload). |
| 6 | A test plan is provided and executed: unit tests for selected components, at least one end-to-end integration test, and documented test results. | **PASS** | `docs/test-plan.md`; unit tests in `internal/decision`, `internal/actuator`, `internal/modeltime`; end-to-end tests in `test/e2e/` (9 files, including the new `scenario_test.go`); integration test against the real BuildSim binary in `test/integration/realbuildsim_test.go`; results captured in `evidence/final-verification/01-regression.log`. |
| 7 | A dashboard visualizes relevant system state (sensor readings, actuator states, environmental conditions, or autonomous decisions). | **PASS** | `internal/dashboard`/`cmd/dashboard`, reachable at `http://localhost:8085/` both natively and via Compose; shows lock/door-position state, latest decision (incl. authorization reason, anomaly result/reason, alert), destination occupancy, per-source last-seen health, and a recent-evidence table. Live-verified in `evidence/final-verification/`. |
| 8 | A written report documents architecture, implementation, evaluation, and key design decisions. | **PASS** | `report/final-report.tex` (this pass's deliverable; see `docs/report-rubric-audit.md` for a critical read of its depth per dimension). |
| 9 | In the oral examination, each student can draw, explain, and justify the system architecture from memory, including major design decisions and trade-offs. | **Preparation PASS; outcome is individual** | `docs/oral-defense-prep.md`. This specific criterion is validated *in* the oral exam, per student — no document can satisfy it in advance; what is provided here is preparation material, not evidence of the exam itself. |

---

## Part B — Internal project requirements (`docs/requirements.md`)

Status legend (unchanged from `docs/test-plan.md`): **PASS** — directly
demonstrated by an automated test and/or captured real-system evidence.
**PARTIAL** — structurally supported or partially exercised, not fully
verified. **UNIMPLEMENTED** — not built; stated as a limitation, not
hidden. (This matrix uses "UNIMPLEMENTED" where the earlier
`test-plan.md` draft said "NOT VERIFIED" — same meaning, the term the
task tonight asked for.)

| ID | Requirement (concise) | Component | Test / experiment | Evidence | Status | Rationale |
|---|---|---|---|---|---|---|
| REQ-FUNC-001 | Deny access with no valid permission | `internal/decision` (authz.go) | `test/e2e/deny_test.go::TestDeny` | `evidence/manual-deny-passage/` | **PASS** | DENY, LOCKED, no command, both automated and real-system traces agree. |
| REQ-FUNC-002 | Allow access with valid permission | `internal/decision`, `internal/actuator` | `test/e2e/allow_test.go::TestAllow` | `evidence/manual-allow-passage/` | **PASS** | ALLOW → UNLOCK → BuildSim confirms, both automated and real-system traces agree. |
| REQ-FUNC-003 | Relock within 10 model s if never opened | `internal/decision` (relock ticker) | `test/e2e/relock_test.go::TestRelockTimeout` | test log | **PASS** | `DOOR_NOT_OPENED` + LOCK fires deterministically on model time, independent of wall clock. |
| REQ-FUNC-004 | Relock after a completed passage | `internal/decision`, `internal/simulator`, `internal/doorstatesensor` | `test/e2e/passage_test.go::TestSuccessfulPassage` | test log | **PASS** | Full ALLOW→UNLOCK→OPEN→cross→CLOSE→RELOCK_AFTER_PASSAGE chain asserted end-to-end. |
| REQ-AUTH-001 | One active user per card | `internal/config` | schema only | config files | PARTIAL | Enforced by data shape (`AccessCard.user_id` is singular), not independently tested for the "at a time" temporal clause. |
| REQ-AUTH-002 | Users have one or more roles | `internal/config`, `internal/decision/authz.go` | `authz_test.go` | test log | **PASS** | Role-level permission matching directly tested. |
| REQ-AUTH-003 | Role-level permission assignment | same | `authz_test.go` | test log | **PASS** | Exercised by every ALLOW test via the `employee` role. |
| REQ-AUTH-004 | User-specific permission override | `internal/decision/authz.go` | `authz_test.go` precedence tests | test log | **PASS** | `TestAuthorize_Precedence_User*` covers override-wins-over-role in both directions. |
| REQ-AUTH-005 | Time-of-day/day-of-week permission windows | `internal/config` | — | — | **UNIMPLEMENTED** | `PermissionsFor` has no day/time field to evaluate; never built, named consistently as a gap since the pre-implementation review. |
| REQ-AUTH-006 | Disabled/expired/unknown card denies | `internal/decision/authz.go` | `authz_test.go` | test log | PARTIAL | Unknown/disabled covered by name; "expired" has no `valid_from`/`valid_until` field to test against. |
| REQ-AUTH-007 | Default deny with no applicable permission | `internal/decision/authz.go` | `authz_test.go` | test log | **PASS** | `TestAuthorize_DefaultDeny_NoApplicablePermission`. |
| REQ-AUTH-008 | Deterministic role/user precedence | `internal/decision/authz.go`, ADR-001 | `authz_test.go` (6 precedence tests) | test log | **PASS** | Every conflict combination tested; `TestAuthorize_Deterministic` re-runs the same input and asserts identical output. |
| REQ-TOPO-001 | Rooms assigned to 4 security zones | `config/system-config.json` | — | config file | PARTIAL | 4 zones defined; only the PUBLIC↔EMPLOYEE boundary is ever exercised by a test — STUDENT/RESTRICTED are inert data. |
| REQ-TOPO-002 | Directed boundary between 2 BuildSim locations | `config/system-config.json`, `internal/buildsim` | `test/integration/realbuildsim_test.go` | test log | **PASS** | `door-1` registered and exercised against the real BuildSim binary. |
| REQ-TOPO-003 | Reader+actuator+door-state per direction | architecture + config | all e2e tests | test logs | **PASS** | `door-1` has all three bindings, exercised end-to-end repeatedly. |
| REQ-TOPO-004 | Level-qualified room IDs | `config/system-config.json` | — | config file | **PASS** | `"level0/1540"`, `"level0/A109"` used throughout. |
| REQ-SIM-001 | Configurable behavioral profiles/schedules/stochastic generation | `internal/simulator` (population.go, scenarios.go) | `scenarios_test.go::TestRunScenarioSeedReproducibility`; `normal-day` scenario | `config/population.json`, `config/scenarios.json` | PARTIAL | A seeded, reproducible random subset/order/spacing of an 8-person population exists; full configurable arrival/departure/lunch *distributions* (design-specification.md §3.3) were deliberately not built — "reproducible evaluation, not simulation realism" (`docs/scenarios.md` §4). |
| REQ-SIM-002 | Structured weekday patterns with stochastic variation | `internal/simulator` | `normal-day` scenario | `config/scenarios.json` | PARTIAL | Structured, seeded arrival spacing/order exists; no departure/lunch/room-use pattern (single door; out of scope by design). |
| REQ-SIM-003 | Configurable, reproducible random seed | `internal/simulator/scenarios.go` | `scenarios_test.go::TestRunScenarioSeedReproducibility` | test log | **PASS** | Same seed → identical ordering; different seed → different ordering, both directly asserted. |
| REQ-SIM-004 | Occupant never crosses a locked door | `internal/simulator` (bounded-wait crossing guard) | `test/e2e/scenario_test.go::TestScenarioUnauthorizedAccessDenied` | test log | **PASS** | Crossing-enabled harness + a DENY scenario: destination occupancy stays 0, door stays LOCKED, directly asserted. |
| REQ-SIM-005 | Occupancy truth updates on successful crossing | `internal/simulator` | `passage_test.go::TestSuccessfulPassage` | test log | **PASS** | Source −1 / destination +1 directly asserted; denial case covered by REQ-SIM-004. |
| REQ-SIM-006 | Actuator action feeds back into subsequent sensing | `internal/simulator`, sensors | `passage_test.go` | test log | **PASS** | DoorStateSensor/OccupancySensor observe the state the Simulator caused — the CPS loop closes. |
| REQ-DEC-001 | Autonomous decision influencing an actuator | `internal/decision` | all e2e tests | test logs | **PASS** | Every ALLOW scenario demonstrates this. |
| REQ-DEC-002 | Authorization and anomaly are separate signals | `internal/decision/authz.go`, `anomaly.go` | `server_decide_test.go` | test log | **PASS** | Neither function reads the other's output; structurally, not just by convention. |
| REQ-DEC-003 | Anomaly alone cannot grant access | `internal/decision/server.go` (`handleDecide`) | `TestDecide_Unauthorized_DeniedRegardlessOfAnomaly` | test log | **PASS** | Explicit invariant test: an anomalous-looking but unauthorized attempt still DENYs. |
| REQ-DEC-004 | Decision/reason/event/model_time recorded | `internal/contracts` (`AccessDecision`) | every decision test | evidence records | **PASS** | All four fields present on every recorded decision. |
| REQ-DEC-005 | Identify behavior differing from normal | `internal/decision/anomaly.go` | `anomaly_test.go` | test log | **PASS** | Unusual-time and repeated-attempt detection both tested in isolation. |
| REQ-DEC-006 | Anomalous-but-authorized ≠ unauthorized | `internal/decision/server.go` | `server_decide_test.go` (3 scenarios) | test log | **PASS** | All three cases produce distinguishable `authorization_result`/`anomaly_result` pairs. |
| REQ-DEC-007 | Decision traceable to its inputs | `internal/contracts` | all decision tests | evidence records | **PASS** | Every `AccessDecision` carries `event_id`, `authorization_result`, `anomaly_result`, `policy_version`. |
| REQ-DIST-001 | Independent, restartable processes | all 8 `cmd/*` + Compose | `evidence/decision-restart-fault/`; live `docker compose restart/stop` | logs, `evidence/final-verification/` | **PASS** | Demonstrated natively and in Docker, for both Decision and Actuator, with unrelated processes untouched both times. |
| REQ-DIST-002 | Not a monolith | architecture, `compose.yaml` | — | compose topology | **PASS** | 8 processes natively; 8 containers in Compose sharing one image only to avoid an 8x rebuild — never merged into one process. |
| REQ-DIST-003 | Documented network interfaces | `docs/data-contracts.md` | — | doc | **PASS** | Every endpoint documented with schema and example. |
| REQ-DIST-004 | Bounded network timeouts | `internal/httpx` | — | source | **PASS** | `DefaultTimeout = 2s` used by every inter-service call (see `evidence/stress/` for what breaks when the *connection pool*, not the timeout, runs out). |
| REQ-DIST-005 | Restarted device re-registers with BuildSim | — | — | — | **UNIMPLEMENTED** | No `/api/equipment` registration flow exists; the real adapter talks to `/api/doors` directly. |
| REQ-DIST-006 | No duplicate devices on re-registration | — | — | — | **UNIMPLEMENTED** | Not applicable — no registration flow exists to duplicate against. |
| REQ-DIST-007 | Duplicate delivery → no duplicate physical effect | `internal/actuator` | `server_test.go::TestHandleCommand_DuplicateCommandID_NoSecondPhysicalEffect` | test log | **PASS** | Redelivers the same `command_id` with a *different* action to prove dedup, not just luck. |
| REQ-DATA-001 | Defined data pipeline | `internal/eventdata`, `internal/evidence`, `internal/dashboard` | — | source + live dashboard | **PASS** | Collects, validates, stores, serves — all four roles exist. |
| REQ-DATA-002 | Survives process restart | `internal/evidence` (plain file) | `evidence/decision-restart-fault/` | evidence dir | **PASS** | Evidence file untouched by a Decision Service restart. |
| REQ-DATA-003 | Required observation fields present | `internal/contracts` | — | evidence records | **PASS** | `event_id`/`source_id`/`model_time`/`source_time`/`receive_time`/`schema_version` on every observation type. |
| REQ-DATA-004 | Duplicate observation detection | — | — | — | **UNIMPLEMENTED** | No dedup/idempotency-key logic in `internal/eventdata`'s ingestion path. |
| REQ-DATA-005 | Late/out-of-order detection | — | — | — | **UNIMPLEMENTED** | No ordering/freshness check exists in the pipeline. |
| REQ-DATA-006 | Malformed observation rejected + counted | `internal/eventdata` | manual malformed-JSON POST | HTTP 400 | PARTIAL | 400 is returned; rejected input is not retained as evidence or counted by any telemetry. |
| REQ-RES-001 | Decision unavailable → door stays locked | `internal/decision`, `internal/eventdata` | `decision_restart_test.go` | `evidence/decision-restart-fault/` | **PASS** | No command issued while down, automated and real-process. |
| REQ-RES-002 | Unconfirmed command → UNVERIFIED | `internal/actuator` | `TestHandleCommand_BuildSimUnavailable_...` | `evidence/actuator-failure/` | **PASS** | Automated + real BuildSim kill, both confirm `UNVERIFIED`/empty `observed_state`; also reproduced with the Actuator container stopped (`evidence/final-verification/`). |
| REQ-RES-003 | Stale observations need an explicit quality flag | — | — | — | **UNIMPLEMENTED** | No freshness/staleness tracking exists anywhere in the pipeline. |
| REQ-RES-004 | Recover from restart without restarting others; recovery time measured | `internal/decision` | `decision_restart_test.go` (10 runs) | `evidence/final-verification/10-recovery-time-measurements.log` | **PASS** | 10.83–32.26 ms, mean 18.8 ms, automated (10 fresh runs); real-process kill/restart functionally confirmed. |
| REQ-RES-005 | Detect missing/stale required sensor observation | — | — | — | **UNIMPLEMENTED** | No staleness-detection logic exists to degrade a decision's quality status. |
| REQ-VIS-001 | Dashboard visualizes occupancy/door/decision/anomaly/health | `internal/dashboard` | manual check | live `http://localhost:8085/` | **PASS** | Verified rendering real evidence, natively and via Compose. |
| REQ-BS-001 | BuildSim authoritative for topology/door/equipment | `internal/buildsim` | `test/integration` | real-BuildSim runs | **PASS** | All manual and automated real-BuildSim runs depend on and confirm this. |
| REQ-BS-002 | History independent of BuildSim persistence | `internal/evidence` | — | `evidence/actuator-failure/` | **PASS** | `data/evidence.jsonl` survives a BuildSim restart/kill. |
| REQ-BS-003 | Actuator command → observable BuildSim change feeding back | `internal/actuator`, `internal/simulator` | `test/integration`; all passage tests | test logs | **PASS** | The entire successful-passage sequence depends on exactly this property. |
| REQ-PERF-001 | p95 decision latency within threshold | `cmd/latencybench` | — | `evidence/latency/` | **PASS** | p95 = 0.60 ms measured; threshold (10 ms) set *after* measurement, not tuned to pass. |
| REQ-TEST-001 | Reproducible scenarios via scenario ID + seed | `internal/simulator` (`POST /scenario/run`) | `scenario_test.go` (5 tests) | `evidence/scenarios/runs.jsonl` | **PASS** | 8 named scenarios, each run recording `{scenario, seed, model_start, card_ids}`; reproducibility directly tested. |
| REQ-TEST-002 | Trace observation → decision → command → BuildSim state | all | every e2e test; dashboard | evidence records | **PASS** | Demonstrated repeatedly, including the live dashboard view. |

**Totals: 41 PASS / 6 PARTIAL / 7 UNIMPLEMENTED, of 54.**

---

## Part C — Mapping to the graded dimensions (summary; full audit in `docs/report-rubric-audit.md`)

| Dimension | Primarily fed by | What it demonstrates |
|---|---|---|
| A — Architecture & design | REQ-DIST-*, REQ-TOPO-*, REQ-DATA-001, architecture docs/ADRs | Component boundaries, service decomposition, and the reasoning recorded in `docs/decisions/` and `docs/deployment.md` §6. |
| B — Systems integration & the CPS loop | REQ-FUNC-*, REQ-SIM-*, REQ-RES-001/002/004, REQ-BS-* | The full sense→decide→act→physical-effect→sense loop, and its behavior under process/dependency failure. |
| C — Autonomous services & data engineering | REQ-DEC-*, REQ-DATA-*, REQ-AUTH-* | The authorization/anomaly/policy separation, and the data pipeline's (still partial) handling of data quality. |
| D — Critical evaluation | REQ-PERF-001, REQ-RES-*, `evidence/stress/`, `evidence/final-verification/` | Quantified latency, fault injection with measured recovery, and a genuine stress/breaking-point experiment plus the door-seed race found and fixed during this pass. |

The 7 UNIMPLEMENTED and 6 PARTIAL requirements above are exactly the
material `docs/report-rubric-audit.md` and report §13 (Risks,
limitations, and future work) draw on for the Dimension D "honest
architectural critique" requirement — they are evidence *for* a strong
Dimension D showing, not merely gaps to apologize for.
