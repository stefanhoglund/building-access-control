# Final Report Gap Analysis

Status: updated after the scenario/population and Docker Compose
implementation pass (named seeded scenarios, a configured
simulated-person population, and a verified Docker Compose deployment),
on top of the earlier finalization pass (unit tests, dashboard,
decision-service and actuator/BuildSim fault experiments, performance
baseline, anomaly detection, stress/breaking-point experiment,
reproducibility script, support docs, and the four previously-missing
diagrams). This supersedes both earlier versions; neither is kept as a
separate file.

**The report this document was tracking readiness for now exists**:
`report/final-report.tex`, compiled to `report/final-report.pdf` (21
pages). This document is kept as the historical record of what was
missing and when it closed, not as a live checklist anymore — see
`docs/report-rubric-audit.md` for a critical read of the finished
report against the grading rubric, which is the more current document
for "is the report actually good enough."

## 1. Required report sections — current support

| Section | Status | Evidence |
|---|---|---|
| Summary | Supported | `README.md` |
| Use case / building context | Supported | `architecture.md` §1–2, `design-specification.md` |
| Requirements | Supported | `docs/requirements.md` (54 IDs, acceptance criteria, REQ-PERF-001 now has a measured threshold) |
| Context diagram | Supported | `docs/diagrams/context.d2` |
| Container diagram | Supported | `docs/diagrams/container.d2` (already reflects the real 8-process system) |
| Component diagram | **Supported (new)** | `docs/diagrams/component.d2` — Decision Service internals |
| Design decisions table | Strong raw material | `docs/decisions/001`–`006` (ADRs) — still needs transcribing into report-table form, not new content |
| Interfaces and data contracts | Supported | `docs/data-contracts.md` |
| Simulating sensor values | Reframed, not a gap | This project's "physics" is door/occupancy state, not temperature/CO₂ — stated explicitly, not pending |
| Autonomous service | Supported | `design-specification.md` §5–8, `internal/decision/` incl. anomaly module |
| Data pipeline | Supported | `internal/eventdata/`, `internal/evidence/` |
| Dynamic/sequence diagram | **Supported (new)** | `docs/diagrams/dynamic.d2` — successful-passage sequence |
| State diagram | **Supported (new)** | `docs/diagrams/state.d2` — door-control lifecycle |
| Deployment diagram | **Supported** | `docs/diagrams/deployment.d2` — now the Docker Compose topology, not just native processes |
| Test plan | **Supported (new)** | `docs/test-plan.md` — full requirement → test → evidence → status table |
| Evaluation | **Supported (new)** | `docs/evaluation-plan.md` + 8 `evidence/` directories with raw data |
| Dashboard | **Supported (new)** | `internal/dashboard`, `cmd/dashboard`, live-verified |
| Risks/reflection | **Supported** | `report/final-report.tex` §13, an 11-item evidence-backed critique, assembled from exactly this material (the PARTIAL/UNIMPLEMENTED rows in `docs/traceability-matrix.md`, every README's limitation section, and this document's §6 below) |

## 2. Diagrams — complete, and now Compose-consistent

All six required views exist: context, container, component, dynamic,
state, deployment (`docs/diagrams/*.d2`, rendered to `docs/figures/*.svg`
via `make diagrams`). The previously-remaining checklist item is now
closed: **`compose.yaml` exists** at the repo root (`docker compose up
--build` starts the complete system, verified — `docs/deployment.md`),
and `docs/diagrams/deployment.d2` was rewritten to depict that exact
Compose topology (container names, internal DNS, published ports, bind
mounts), satisfying the report template's "container diagram...
consistent with the docker-compose.yml" requirement directly.
`docs/diagrams/container.d2` itself was deliberately left unchanged: no
new architectural service was introduced (the one init step, `door-seed`,
is deployment-only plumbing, not a logical component) — see
`docs/deployment.md` for why. `scripts/run-local.sh` + the Makefile
targets (`make run`/`stop`/`status`) remain fully supported alongside
Compose, not replaced by it.

## 3. Tests — now substantial

- **Unit tests**: 24 across `internal/decision` (authz precedence,
  anomaly scorer, full policy matrix), `internal/actuator` (confirmation,
  deduplication, BuildSim-unavailable), `internal/modeltime`.
- **Fault-injection tests**: Decision Service kill/restart
  (`decision_restart_test.go`, 10/10 stable runs + real-process capture);
  actuator/BuildSim-unavailable (automated + real BuildSim kill).
- **Stress/load test**: `cmd/stressbench`, found and explained a genuine
  breaking point.
- **Scenario tests**: `test/e2e/scenario_test.go` (5 tests, via the real
  `POST /scenario/run` interface) + `internal/simulator/scenarios_test.go`
  (seed reproducibility) — covering the six scenario behaviors Phase 1
  required.
- **Docker Compose verification**: all eight named scenarios, a
  container restart, and an actuator-stop fault, each run manually
  against the real containerized stack and confirmed correct
  (`docs/test-plan.md`'s "Docker Compose deployment verification"
  table), plus full native regression (`gofmt`/`vet`/`test`/`-race`/
  `-tags=integration`) re-run and green after all Docker changes.

Still not done: a unit/fault test for a sensor process crash specifically
(only the Decision Service and the Actuator↔BuildSim link have been
fault-tested; Card Reader/OccupancySensor/DoorStateSensor crash-recovery
is architecturally identical — same restart pattern, same "unrelated
processes unaffected" property already proven for Decision — but was not
separately exercised, to avoid repeating the same proof three more times
for diminishing evidentiary value).

## 4. Measured evaluation evidence — now exists

- REQ-PERF-001: p95 = 0.60 ms measured, threshold set at 10 ms
  afterward. `evidence/latency/`.
- Recovery time: 10.83–32.26 ms, mean 18.8 ms (automated, 10 fresh runs),
  real-process capture confirms correctness (not a clean timing number —
  stated honestly). `evidence/final-verification/10-recovery-time-measurements.log`,
  `evidence/manual-decision-restart-fault/`.
- Breaking point: concurrency ~75–100 collapses throughput to zero;
  root cause isolated to `internal/httpx`'s default connection pool, not
  assumed. `evidence/stress/`.
- Anomaly: three required scenarios, all three field values recorded as
  evidence. `evidence/anomaly/`.

## 5. Screenshots

Dashboard is running HTML at `http://localhost:8085/`, confirmed rendering
real evidence correctly (door state, latest decision with anomaly
classification, component health, recent-evidence table) — an actual
screenshot/image capture for the report document itself has not been
taken (this is a one-click manual step for whoever assembles the report,
not a code or evidence gap).

## 6. Requirements still lacking verification (honest, from `test-plan.md`)

7 of 54 requirements are **NOT VERIFIED** — unimplemented, not merely
untested — clustering around two remaining themes (down from three: the
simulated-behavioral-realism theme is now partially closed, see below):

- **Device re-registration** (REQ-DIST-005/006): no `/api/equipment`
  registration flow exists; the real BuildSim adapter talks to
  `/api/doors` directly.
- **Data-quality fault handling** (REQ-AUTH-005, REQ-DATA-004/005,
  REQ-RES-003/005): no permission time-window evaluation, no
  duplicate/late/stale detection anywhere in the pipeline.

6 more are **PARTIAL** (REQ-AUTH-001/006, REQ-TOPO-001, REQ-SIM-001/002,
REQ-DATA-006) — see `test-plan.md` for exactly what's missing in each.
REQ-SIM-001/002 moved here from NOT VERIFIED: a configured 8-person
population and seeded, reproducible scenario generation now exist
(`config/population.json`, `config/scenarios.json`,
`internal/simulator/scenarios.go`), short of full configurable
arrival/departure/lunch behavioral-profile generation
(design-specification.md §3.3) — deliberately out of scope ("the purpose
of this phase is reproducible system evaluation, not simulation
realism"). REQ-SIM-003 (seed reproducibility) and REQ-SIM-004 (occupant
never crosses a locked door, now directly tested) moved all the way to
**PASS**, as did REQ-TEST-001 (scenario ID + seed reproducibility, now
backed by `evidence/scenarios/runs.jsonl`).

## 7. Highest-value remaining item, if more time existed

Of the themes above, **data-quality fault handling** (duplicate/late/
malformed observation detection, REQ-DATA-004/005/006) would be the
single highest-value addition: it is explicitly named in the grading
criteria's Dimension D examples ("injecting corrupted or missing data"),
it is cheap relative to full simulated-behavioral-profile generation (no
new process, just validation logic in the already-existing Event/Data
Service), and —
unlike device re-registration, which has no test surface until a second
BuildSim restart scenario is built — it would immediately produce new,
easy-to-demonstrate evidence using the exact same evidence-capture
pattern already established four times over in this pass. This was not
implemented in this finalization pass because it was not on the
requested priority list; it is named here as the answer to "what next,"
not undertaken.
