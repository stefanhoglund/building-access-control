# Requirements

Requirements v0.1
Status: architecture baseline

## Requirement sources

Requirements in this document include:
- course-mandated architectural constraints,
- functional requirements of the access-control use case,
- non-functional requirements derived from the intended evaluation.


REQ-FUNC-001
The system shall deny an access attempt when no valid permission exists for the presented card, requested controlled boundary, and model time.

Acceptance:
- authorization decision is DENY
- door remains locked
- occupant does not cross the controlled boundary
- DENY decision and reason are recorded

REQ-FUNC-002
The system shall allow an access attempt when the presented card is active and a valid permission exists for the requested controlled boundary at model time.

Acceptance:
- authorization decision is ALLOW
- actuator receives an unlock command
- BuildSim reflects the unlocked state
- the occupant may cross the controlled boundary
- decision and outcome are recorded

REQ-FUNC-003
A door that was locked before an approved access attempt shall automatically return to the locked state no later than 10 model seconds after being unlocked if the door has not been opened.

Acceptance:
- initial state is LOCKED
- an ALLOW decision causes the door to unlock
- no door-open event occurs
- a DOOR_NOT_OPENED event is recorded
- door returns to LOCKED within 10 model seconds

REQ-FUNC-004
When an authorized occupant opens and passes through a controlled door, the system shall relock the door after passage is complete.

Acceptance:
- valid access is approved
- door opens
- occupant crosses boundary
- destination occupancy changes appropriately
- door returns to LOCKED


REQ-AUTH-001
Each access card shall be associated with exactly one active user at a time.

REQ-AUTH-002
Users shall be assigned one or more roles.

REQ-AUTH-003
Permissions shall be assignable at the role level.

REQ-AUTH-004
The system shall support user-specific permission overrides.

REQ-AUTH-005
Permissions may include time-of-day and day-of-week constraints evaluated against model time.

REQ-AUTH-006
A disabled, expired, or unknown card shall not authorize access.

REQ-AUTH-007
If no applicable permission exists, access shall default to DENY.

REQ-AUTH-008
The system shall define deterministic precedence when both role-level and user-specific permissions apply.

Acceptance:
- precedence rules are documented
- the same inputs always produce the same authorization result
- conflicting role/user permissions are covered by tests


REQ-TOPO-001
Rooms shall be assigned to security zones.

Initial security zones shall include:
- public/open
- employee
- student
- restricted

REQ-TOPO-002
A controlled door shall represent a directed or bidirectional transition between two BuildSim locations and their associated security zones.

REQ-TOPO-003
Each controlled direction shall be associated with:
- a card reader
- a door actuator
- observable door state

REQ-TOPO-004
Room identity shall use a level-qualified identifier such as level0/A109.


REQ-SIM-001
The simulator shall generate occupant movement and access behavior according to configured behavioral profiles, schedules, security-zone access patterns, and stochastic variation.

Acceptance:
- weekday arrival and departure distributions are configurable
- room-transition probabilities are configurable
- generated behavior is reproducible for a fixed seed
- generated behavior differs for different seeds

REQ-SIM-002
Normal weekday behavior shall include structured arrival, departure, lunch, and room-use patterns with controlled stochastic variation.

REQ-SIM-003
Simulation runs shall support a configurable random seed so that runs can be reproduced.

REQ-SIM-004
The simulator shall prevent an occupant from crossing a locked controlled door unless the active scenario explicitly models forced entry.

REQ-SIM-005
When an occupant successfully crosses from one room to another, the simulator shall update the physical occupancy state of both rooms and cause subsequent occupancy observations to reflect the change.

Acceptance:
- source-room occupancy decreases by one
- destination-room occupancy increases by one
- no occupancy transition occurs when movement is denied

REQ-SIM-006
Actuator actions shall influence subsequent simulated behavior and sensor readings, closing the cyber-physical feedback loop.

REQ-DEC-001
At least one independently deployable component shall autonomously make decisions based on sensor observations and influence actuator behavior.

REQ-DEC-002
Authorization and anomaly detection shall be treated as separate decision signals.

REQ-DEC-003
An anomaly score alone shall not grant access that is otherwise unauthorized.

REQ-DEC-004
Every access decision shall record:
- decision
- reason
- associated access event
- model time

REQ-DEC-005
The system shall identify access behavior that differs from defined normal behavior.

REQ-DEC-006
Anomalous but authorized behavior shall be distinguishable from unauthorized behavior.

REQ-DEC-007
Every autonomous decision shall be traceable to the observations and decision
signals used to produce it.

Acceptance:
- decision contains or references the triggering observation ID
- authorization result is identifiable
- anomaly result is identifiable where applicable
- decision-policy version is identifiable

REQ-DIST-001
Sensor, actuator, and autonomous decision components shall run as separate, independently deployable and restartable processes.

REQ-DIST-002
The system shall not be implemented as a single monolithic application.

REQ-DIST-003
Components shall communicate through documented network interfaces.

REQ-DIST-004
Network calls shall use bounded timeouts.

REQ-DIST-005
A restarted device process shall re-register with BuildSim and resume operation.

REQ-DIST-006
Repeated registration shall not create duplicate logical devices.

REQ-DIST-007
Repeated delivery of the same observation or command shall not cause duplicate physical side effects.

REQ-DATA-001
The system shall provide a defined data pipeline that collects sensor observations, stores them, and makes them available for monitoring, analysis, decision-making, training, or evaluation.

REQ-DATA-002
Observations, decisions, actuator commands, and relevant outcomes required for replay and evaluation shall survive application-process restarts.

REQ-DATA-003
Every retained observation shall include:
- unique event identifier
- source identifier
- model time
- source time
- receive time
- schema version

REQ-DATA-004
The pipeline shall detect and record duplicate observations.

REQ-DATA-005
The pipeline shall identify late or out-of-order observations.

REQ-DATA-006
Malformed observations shall be rejected from the validated data path, retained or logged as rejected evidence, and counted by pipeline telemetry.

REQ-RES-001
If the authorization or decision service is unavailable, a locked controlled door shall remain locked.

REQ-RES-002
If an actuator command cannot be confirmed, the resulting door state shall be marked as unverified and the failure shall be recorded.

REQ-RES-003
Stale sensor observations shall not be treated as current without an explicit quality flag.

REQ-RES-004
The system shall recover from restart of a sensor, actuator, or decision process without requiring restart of unrelated processes.

Acceptance:
- restarted process re-registers/reconnects
- normal processing resumes
- no duplicate logical devices are created
- recovery time is measured

REQ-RES-005
The system shall detect when a required sensor observation is unavailable or stale beyond its configured freshness limit.

Acceptance:
- stale/missing state is identifiable
- affected decisions record degraded-data status
- the system follows the documented degraded-state policy

REQ-VIS-001
The dashboard shall visualize, for the active scenario:
- current occupancy
- controlled-door state
- recent access decisions
- active anomaly/alert state
- degraded or failed component state where applicable

REQ-BS-001
BuildSim shall be treated as the authoritative source for current building topology, controlled-door state, and registered equipment state.

REQ-BS-002
Historical observations and decisions shall not rely solely on BuildSim persistence.

REQ-BS-003
An accepted actuator command shall produce an observable BuildSim state change that can affect subsequent simulator behavior or sensor observations.

REQ-PERF-001
Under nominal load, 95% of access decisions shall be produced within a defined latency threshold.

Acceptance threshold:
TBD after initial baseline measurement.

REQ-TEST-001
The system shall support reproducible end-to-end scenarios using a configured scenario identifier and random seed.

REQ-TEST-002
The system shall expose enough evidence to trace an access attempt from sensor observation through decision, actuator command, and resulting BuildSim state.
