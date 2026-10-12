# Resilient Anomaly-Aware Building Access Control

A distributed cyber-physical access-control system for LTU course **D7065E – Embedded Intelligence at the Edge**.

The project explores how an autonomous edge system can make safe building-access decisions from imperfect sensor data while remaining robust to component and communication failures.

## Project Goals

The system is designed to:

- observe simulated access attempts and occupancy-related sensor data;
- evaluate deterministic authorization separately from behavioral anomaly detection;
- make autonomous access decisions;
- issue commands to independently running door-actuator processes;
- reflect resulting physical state changes in BuildSim;
- feed those state changes back into subsequent simulator behavior and sensor observations;
- retain evidence for replay, evaluation, and fault analysis;
- remain safe under partial failures, stale data, duplicate messages, and process restarts.

The core conceptual distinction is:

```text
authorization = what may happen
anomaly       = what usually happens
```

An anomaly score does not grant access that is otherwise unauthorized.

## Architecture

The project follows a specification-first systems-engineering approach.

The major runtime elements are:

```text
Occupant & Movement Simulator
        ↓
Sensor Processes
        ↓
Event & Data Service
        ↓
Autonomous Decision Service
        ↓
Door Actuator Processes
        ↓
BuildSim
        ↓
Simulator observes resulting physical state
```

Historical observations, decisions, actuator commands, outcomes, and fault evidence are retained separately from BuildSim.

### Major Components

- **Occupant & Movement Simulator**
  - owns model time;
  - owns simulated occupant location and occupancy truth;
  - generates normal and injected scenario behavior;
  - respects controlled-door state when moving simulated occupants.

- **Sensor Processes**
  - card readers;
  - occupancy sensors;
  - door-state sensors;
  - run as independently restartable processes.

- **Event & Data Service**
  - validates observations;
  - detects malformed, duplicate, late, or out-of-order data;
  - forwards validated observations to the decision service;
  - acts as the logical writer to the historical evidence store.

- **Autonomous Decision Service**
  - authorization;
  - anomaly detection;
  - decision policy;
  - runtime assurance;
  - logical per-door control state and relock lifecycle.

- **Door Actuator Processes**
  - execute `LOCK` / `UNLOCK` commands;
  - interact with BuildSim;
  - avoid duplicate physical side effects.

- **Historical Evidence Store**
  - retains observations, decisions, commands, outcomes, and evaluation evidence.

- **Dashboard**
  - exposes current operational state, decisions, alerts, health, and historical evidence.

- **BuildSim**
  - external course-provided environment;
  - authoritative for building topology, controlled-door state, and registered equipment state;
  - provides visualization and current shared physical state;
  - is not the authorization engine, anomaly detector, occupant simulator, or historical event store.

## Documentation

The repository uses documents as the source of truth for design and implementation.

```text
docs/
├── requirements.md
├── architecture.md
├── design-specification.md
├── data-contracts.md
├── scenarios.md
├── deployment.md
├── test-plan.md
├── diagrams/
│   ├── context.d2
│   ├── container.d2
│   └── ...
├── figures/
│   ├── context.svg
│   ├── container.svg
│   └── ...
└── decisions/
    ├── 001-authorization-precedence.md
    ├── 002-single-autonomous-decision-service.md
    ├── 003-model-time.md
    ├── 004-versioned-system-configuration.md
    ├── 005-control-path-through-data-service.md
    └── 006-occupancy-state-ownership.md
```

### Document Roles

- `docs/requirements.md` — what the system must do.
- `docs/architecture.md` — system boundary, runtime structure, responsibilities, and major flows.
- `docs/design-specification.md` — domain model, state ownership, authorization, decision behavior, runtime assurance, and degraded-state behavior.
- `docs/data-contracts.md` — event schemas, API contracts, timestamps, IDs, quality fields, retries, and idempotency.
- `docs/scenarios.md` — normal behavior, deterministic scenarios, injected anomalies, and fault cases.
- `docs/deployment.md` — processes, containers, startup, health checks, persistence, and restart behavior.
- `docs/test-plan.md` — verification strategy and requirements traceability.
- `docs/decisions/` — Architecture Decision Records (ADRs) documenting significant design choices and tradeoffs.

## Architecture Diagrams

Architecture diagrams are written in [D2](https://d2lang.com/) and rendered to SVG.

Example:

```bash
make diagrams
```

The Makefile uses rules similar to:

```make
D2FLAGS := --theme 4 --pad 25

docs/figures/%.svg: docs/diagrams/%.d2
	mkdir -p docs/figures
	d2 $(D2FLAGS) --layout dagre $< $@
```

The `.d2` files are the source artifacts; generated `.svg` files are used in documentation and reports.

## Architecture Decisions

Important decisions are recorded as ADRs.

Current accepted decisions include:

1. deterministic authorization precedence;
2. one autonomous decision-service process containing logically separated authorization, anomaly, policy, and runtime-assurance modules;
3. simulator ownership of model time;
4. versioned system configuration for users, cards, permissions, zones, controlled doors, and device bindings;
5. sensor-to-decision control flow through the Event & Data Service;
6. simulator ownership of occupancy truth, with independent occupancy-sensor observations.

## Time Model

The system distinguishes:

- **model time** — simulated date/time owned by the simulator and used for permission windows, schedules, scenario timing, and simulation behavior;
- **wall-clock time** — real execution time used for latency, health checks, network timeouts, and recovery measurement.

Components must not infer model time independently from their local wall clock.

## Safety and Runtime Assurance

Key invariants include:

- invalid, disabled, lost, or expired credentials cannot unlock a door;
- a `DENY` authorization cannot produce an `UNLOCK` command;
- anomaly detection cannot override an authorization denial;
- stale required authorization inputs cannot silently authorize access;
- duplicate commands must not cause duplicate physical side effects;
- commanded state is distinct from observed physical state;
- unconfirmed actuation must remain visibly unverified;
- a locked controlled boundary cannot be crossed unless an explicit scenario models forced entry;
- failures in authorization or decision processing fail closed.

## Development Approach

The project follows:

```text
EXPLORE
→ SPECIFY
→ PLAN
→ IMPLEMENT
→ VERIFY
→ REVIEW
```

Implementation should trace back to requirements and documented design decisions.

Before implementing a non-trivial change, identify:

1. the relevant `REQ-*` requirement(s);
2. the owning component;
3. the relevant design section;
4. the affected interface/data contract;
5. the acceptance test.

See `CLAUDE.md` for coding-agent guidance.

## Prerequisites and Fresh Installation

This project supports two execution modes:

1. **Docker Compose** — recommended for a fresh machine because it builds and runs the complete application stack reproducibly.
2. **Native Go binaries** — useful for development, debugging, testing, and running individual services directly.

The final report can also be rebuilt locally from the LaTeX source in `report/`.

### Required tools

At minimum, install:

- Git
- Docker with Docker Compose
- Make

For native development and testing, also install:

- Go

For rebuilding the final report, also install:

- A LaTeX distribution providing `pdflatex`

Optional developer tools may be needed for regenerating architecture diagrams, but they are not required to run the application or build the existing report PDF.

### macOS

Using Homebrew:

```bash
brew install git make go
brew install --cask docker
brew install --cask mactex-no-gui


## Running the System

### Native (no Docker)

```bash
make build      # builds every process binary into bin/
make run        # starts BuildSim, seeds door-1 LOCKED/CLOSED, starts all 8 processes + dashboard
make status     # show which processes are running
make stop       # stop everything this started
```

- Dashboard: <http://localhost:8085/>
- BuildSim: <http://localhost:9090/>

Drive a named scenario (see `docs/scenarios.md` for what each one does):

```bash
make demo-allow             # authorized-passage
make demo-deny              # unauthorized-access
make demo-after-hours       # after-hours-access (authorized but anomalous)
make demo-rapid-repeat      # rapid-repeat-access (anomalous)
make demo-normal-day        # normal-day (seeded population sample)
make demo-decision-failure  # kill/restart the Decision Service mid-scenario
make demo-actuator-failure  # stop/restart the Door Actuator mid-scenario
```

`scripts/run-local.sh` lists every available subcommand directly.

### Docker Compose

```bash
make docker-up    # docker compose up --build -d: BuildSim + all 8 processes + dashboard
make docker-logs  # docker compose logs -f
make docker-down  # docker compose down
```

- Dashboard: <http://localhost:8085/>
- BuildSim: <http://localhost:9090/>
- Simulator (scenario control): <http://localhost:8081/>

The same named scenarios, against the containerized stack:

```bash
make docker-demo-allow
make docker-demo-deny
make docker-demo-after-hours
make docker-demo-rapid-repeat
make docker-demo-normal-day
make docker-demo-decision-failure
```

Evidence persists on the host under `./data/` and `./evidence/` (bind-mounted
into every container that needs it), so it survives any single container's
restart or a full `docker compose restart`. See `docs/deployment.md` for the
complete container/service mapping, networking, and the reasoning behind
choosing Compose.

<!-- ## Current Development Status

The vertical slice described by architecture baseline v0.2 (ADR-001 through
ADR-006) is implemented end-to-end: one controlled door, a configured
population of simulated people, authorization, anomaly detection, decision
policy, runtime assurance, actuation, sensing, evidence retention, a
dashboard, named/seeded scenarios (`docs/scenarios.md`), and a Docker Compose
deployment (`docs/deployment.md`) alongside the native workflow above. See
`docs/test-plan.md` for per-requirement verification status and
`docs/final-report-gap-analysis.md` for what remains out of scope. -->

## Testing and Evaluation

Evaluation will include:

- deterministic ALLOW / DENY scenarios;
- reproducible seeded simulation;
- actuator and decision-service failures;
- stale and malformed observations;
- duplicate and out-of-order messages;
- process restart and recovery;
- end-to-end decision latency;
- actuator verification and recovery behavior;
- anomaly false positives, misses, and detection delay where applicable.

The system should be evaluated not only for nominal correctness but also for how it behaves when components or communication paths fail.

## Repository Workflow

Render diagrams:

```bash
make diagrams
```

Run Go formatting and checks:

```bash
gofmt -w .
go vet ./...
go test ./...
```

Inspect recent commits and tags:

```bash
git log --oneline --decorate -n 10
```

## Project Philosophy

The project deliberately favors a small, explicit, well-tested architecture over unnecessary complexity.

The goal is not to maximize the number of services or the sophistication of the AI technique. The goal is to build a system whose behavior, failure modes, data flow, decisions, and tradeoffs can be clearly explained and evaluated end-to-end.
