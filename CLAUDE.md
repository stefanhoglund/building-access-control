# CLAUDE.md

## Project

This repository implements the **Resilient Anomaly-Aware Building Access Control System** for LTU course D7065E.

The project is a distributed cyber-physical system that integrates simulated occupants, sensor processes, an event/data pipeline, autonomous access-control decisions, actuator processes, persistent evidence, and BuildSim.

The architectural goal is not maximum complexity. The priority is a system that is:

- well specified
- explainable
- independently testable
- resilient to component and communication failures
- traceable from requirements to implementation and tests

The student must be able to explain the complete system architecture and behavior from memory.

---

# 1. Source of Truth

Before making implementation changes, read the relevant project documentation.

Primary sources of truth:

1. `docs/requirements.md`
   - defines what the system must do
   - requirements are identified by `REQ-*`

2. `docs/architecture.md`
   - defines system boundaries
   - defines major runtime components
   - defines responsibilities and major data/control flows

3. `docs/design-specification.md`
   - defines domain concepts
   - defines state ownership
   - defines authorization, anomaly, decision, and runtime behavior

4. `docs/data-contracts.md`
   - defines messages, records, APIs, timestamps, IDs, quality fields,
     retries, idempotency, and error behavior

5. `docs/scenarios.md`
   - defines normal simulation behavior
   - defines deterministic evaluation scenarios and fault cases

6. `docs/test-plan.md`
   - defines verification and requirement traceability

7. `docs/deployment.md`
   - defines runtime processes, containers, ports, health checks,
     startup/restart behavior, and persistence

8. `docs/decisions/`
   - contains Architecture Decision Records for important design choices

Diagrams under `docs/diagrams/` are architecture source artifacts.

Generated diagrams under `docs/figures/` are derived artifacts.

If code conflicts with the specification, do not silently treat the code as authoritative.

---

# 2. Required Workflow

Do not immediately implement vague or high-level requests.

For any non-trivial change, first identify:

- which `REQ-*` requirement(s) the change supports
- which architecture component owns the behavior
- which design specification section defines the behavior
- which interface or data contract is involved
- how the change will be verified

Use this workflow:

```text
EXPLORE
  -> SPECIFY
  -> PLAN
  -> IMPLEMENT
  -> VERIFY
  -> REVIEW
```

If requirements or design behavior are missing or ambiguous, stop before implementation and report the ambiguity.

Do not invent missing product behavior merely to continue coding.

---

# 3. Architecture Rules

The project is intentionally distributed.

Sensor, actuator, and autonomous decision-making components must remain independently deployable and restartable processes.

Do not collapse required independent processes into a monolithic application.

Do not introduce a new network service unless there is a clear architectural reason.

Within a process, prefer normal Go packages over unnecessary service boundaries.

Do not introduce Kubernetes or other orchestration complexity unless explicitly required by an experiment.

Docker Compose is the default deployment mechanism.

---

# 4. BuildSim Responsibilities

BuildSim is external to the student-developed system.

Treat BuildSim as the authoritative representation of current building state.

BuildSim may contain:

- rooms
- building topology
- doors
- routing information
- equipment
- current sensor values
- current actuator state
- visualization state

BuildSim is NOT:

- the authorization engine
- the anomaly detector
- the occupant behavior simulator
- the historical evidence store
- the source of persistent experiment history

Do not assume BuildSim state survives restart.

Device/equipment registration must therefore be reproducible and idempotent.

Do not invent BuildSim API behavior. Consult the local BuildSim documentation before implementing an integration.

---

# 5. Domain Separation

Keep the following concepts distinct.

## Authorization

Authorization answers:

> Is this action permitted?

Authorization is deterministic policy evaluation.

Inputs may include:

- access card
- user
- role
- permission
- controlled boundary
- direction
- model time

## Anomaly Detection

Anomaly detection answers:

> Is this behavior unusual?

Anomaly detection does not determine permission.

## Decision Policy

The decision policy combines authorization, anomaly information, sensor quality,
and system state.

## Runtime Assurance

Runtime assurance applies hard constraints before actuator commands are issued.

Do not allow anomaly scores to override an authorization denial unless the specification is explicitly changed.

A useful conceptual rule is:

```text
authorization = what may happen
anomaly       = what usually happens
```

---

# 6. Safety Invariants

The following invariants must not be violated unless the specification is explicitly changed:

- an invalid or disabled card must not unlock a controlled door
- a DENY authorization must not produce an unlock command
- anomaly detection alone must not grant unauthorized access
- stale required authorization input must not silently be treated as current
- duplicate messages must not produce duplicate physical side effects
- an unconfirmed actuator result must not be represented as confirmed physical state
- a failed decision service must not cause a locked door to become unlocked
- occupants must not cross locked controlled boundaries unless an explicit test scenario models forced entry

Changes affecting these rules require explicit review.

---

# 7. State Ownership

Do not duplicate authoritative state without a documented reason.

Expected ownership:

- BuildSim: current physical/building state
- Simulator: simulated occupant intent, schedule, behavior, and movement state
- Authorization configuration: users, cards, roles, permissions
- Event/Data service: validated event ingestion and data quality handling
- Historical Evidence Store: retained observations, decisions, commands, outcomes
- Decision service: transient decision state
- Device processes: local device/runtime state only

If ownership is unclear, do not guess. Report it before implementation.

---

# 8. Time Semantics

The project may use multiple time concepts.

Distinguish:

- model time
- source time
- receive time
- processing time
- wall-clock time

Use model time for simulated building behavior and permission windows unless otherwise specified.

Use wall-clock time for runtime concerns such as:

- network timeout
- process health
- real execution latency

Do not mix model time and wall-clock time implicitly.

Any timeout or freshness rule must state which clock it uses.

---

# 9. Data and Event Rules

Canonical records should have stable identity.

Where applicable, preserve:

- `event_id`
- source/device ID
- scenario ID
- experiment ID
- model time
- source time
- receive time
- schema version
- quality state
- related event IDs

Do not silently discard:

- malformed input
- duplicate input
- late input
- out-of-order input
- unconfirmed commands

Rejected or degraded data should remain observable for evaluation.

Do not silently "clean" data in ways that erase experiment evidence.

---

# 10. Simulation Rules

Simulation behavior belongs in the simulation domain, not in the access-control domain.

A `User` does not own:

- movement schedule
- stochastic behavior
- current simulated position
- scenario behavior

Those belong to `SimulatedPerson`, `Schedule`, `BehaviorProfile`,
`MovementState`, or equivalent simulation concepts.

Normal behavior should be structured rather than uniformly random.

Simulation runs should support reproducible seeds.

Use deterministic scenarios for:

- debugging
- requirement verification
- fault injection
- controlled evaluation

Use seeded stochastic scenarios for robustness evaluation.

---

# 11. Component Boundaries

The intended major runtime components are:

- occupant/movement simulator
- card-reader processes
- occupancy-sensor processes
- door-state-sensor processes
- event/data service
- historical evidence store
- autonomous decision service
- door-actuator processes
- dashboard service
- BuildSim as external system

The exact transport and storage technologies may evolve through documented architectural decisions.

Do not add a service simply because it seems architecturally "clean."

Prefer the fewest process boundaries that satisfy the requirements and failure experiments.

---

# 12. Implementation Planning

Before coding a non-trivial change, provide a short plan containing:

1. requirement IDs
2. affected components
3. affected files
4. affected interfaces/contracts
5. expected upstream dependencies
6. expected downstream dependencies
7. verification approach
8. implementation order

Prefer small vertical slices.

Do not implement multiple unrelated components in one change.

---

# 13. Testing

Tests must derive from requirements, contracts, and scenarios.

Do not only test the implementation that was written.

For each important behavior, ask:

> What observable evidence would prove the requirement was satisfied?

Testing should include, where applicable:

- unit tests
- contract tests
- component tests
- integration tests
- end-to-end tests
- restart tests
- fault injection
- duplicate-message tests
- malformed-data tests
- delayed-data tests
- out-of-order-data tests
- unavailable-service tests
- actuator-failure tests
- load/stress tests

Tests generated by an AI coding tool are not considered proof of correctness merely because they pass.

Review whether the test independently captures the intended requirement.

---

# 14. Fault Handling

Assume distributed components can fail independently.

Consider:

- process crash
- startup ordering
- network timeout
- unavailable dependency
- duplicate delivery
- delayed delivery
- out-of-order delivery
- malformed messages
- stale observations
- actuator timeout
- BuildSim restart
- partial recovery

Do not assume a remote call behaves like a local function call.

Network calls should have bounded timeouts.

Retries must account for idempotency.

---

# 15. Observability

Runtime behavior should be diagnosable.

Prefer structured logs containing relevant identifiers such as:

- event ID
- user/card ID where appropriate
- device ID
- door ID
- scenario ID
- experiment ID
- decision ID
- command ID

Do not log secrets or unnecessary sensitive data.

Important failures should be visible rather than silently recovered.

---

# 16. Code Quality

The project is primarily implemented in Go.

Prefer:

- simple packages
- explicit types
- small interfaces
- standard library where practical
- explicit error handling
- deterministic behavior where required
- minimal dependencies

Avoid:

- speculative abstractions
- framework-heavy solutions
- unnecessary generic infrastructure
- premature optimization
- hidden global state
- tightly coupled packages

After Go changes, run as applicable:

```bash
gofmt -w .
go vet ./...
go test ./...
```

Review the full diff before considering a task complete.

---

# 17. Dependency Policy

Do not add new dependencies without explaining why they are needed.

Before adding a dependency, consider whether the Go standard library is sufficient.

For significant dependencies, report:

- purpose
- alternatives considered
- operational impact
- whether the dependency changes deployment or architecture

Do not introduce infrastructure dependencies such as MQTT brokers, databases,
or external AI services without a documented architectural decision.

---

# 18. Documentation Changes

If implementation requires a behavior not currently specified, update the relevant specification before or with the implementation.

Examples:

- new requirement -> `requirements.md`
- changed responsibility -> `architecture.md`
- changed behavior -> `design-specification.md`
- changed event/API -> `data-contracts.md`
- changed deployment -> `deployment.md`
- significant technology choice -> ADR
- new verification behavior -> `test-plan.md`

Do not allow implementation to silently become the only documentation of system behavior.

---

# 19. Architecture Decision Records

Create or update an ADR for decisions with meaningful alternatives or long-term consequences.

Examples include:

- HTTP vs MQTT
- evidence storage technology
- authorization precedence
- learned anomaly model choice
- communication retry strategy
- BuildSim synchronization approach

ADRs should contain:

```text
Context
Decision
Alternatives Considered
Consequences
```

Do not create ADRs for trivial implementation choices.

---

# 20. Review Mode

When asked to review architecture or specifications:

- do not modify files unless explicitly requested
- do not immediately redesign the system
- identify contradictions and gaps first
- reference exact documents and sections
- distinguish requirement problems from implementation problems
- classify findings by severity

Useful severity levels:

- `BLOCKER` — must be resolved before implementation
- `IMPORTANT` — likely to cause ambiguity, failure, or evaluation weakness
- `MINOR` — clarity or maintainability improvement

Prefer the smallest correction that resolves the issue.

---

# 21. Oral Exam Constraint

All architecture and implementation choices should remain explainable by the student.

When proposing a solution, prefer one that can be clearly explained in terms of:

- requirement
- responsibility
- interface
- state
- failure behavior
- tradeoff
- verification

Avoid complexity that does not provide measurable value to the project.

A solution is not better merely because it uses more advanced technology.

---

# 22. Current Development Stage

The current priority is specification-first development.

Do not begin broad implementation until the following are sufficiently defined:

- requirements
- context architecture
- container architecture
- design specification
- primary data contracts
- core scenarios
- test approach

Existing or experimental code may be used as reference, but it does not override the specification.

Nothing should be retained solely because it already exists.

---

# 23. Default Behavior When Uncertain

If uncertain:

1. inspect the relevant documentation
2. identify the ambiguity
3. explain why it matters
4. propose the smallest set of options
5. ask for a decision before implementing behavior that would change system semantics

Do not silently invent policy.