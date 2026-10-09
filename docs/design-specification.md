# Design Specification

## 1. Purpose

This document defines the detailed behavioral and domain design of the Resilient Anomaly-Aware Access Control System.

Its purpose is to specify the system concepts, state ownership, authorization rules, anomaly-processing boundaries, decision logic, actuator behavior, and degraded-state handling at a level precise enough to guide implementation and testing.

The specification defines the meaning and relationships of the core domain entities, including users, access cards, roles, permissions, security zones, controlled doors, sensors, actuators, and simulated occupants.

It also defines the expected behavior of the autonomous decision path, including the separation between authorization and anomaly detection, the rules used to combine those signals, and the runtime constraints that must hold before an actuator command is issued.

This document does not define transport-level message schemas, deployment configuration, or test procedures in detail. Those concerns are documented separately in `data-contracts.md`, `deployment.md`, and `test-plan.md`.

The design is derived from the requirements in `requirements.md` and the system structure defined in `architecture.md`.

## 2. Domain Model

### 2.1 User

A User represents a person known to the access-control system.

A User:
- has a stable user identifier
- may have one or more roles
- may have zero or more access cards
- may have user-specific permission overrides

A User does not contain simulated movement behavior or schedule information. Those belong to the simulation domain.

### 2.2 AccessCard

An AccessCard represents a credential presented to a card reader.

Attributes include:
- card_id
- user_id
- status
- valid_from
- valid_until

Card status may include:
- ACTIVE
- DISABLED
- LOST
- EXPIRED

An AccessCard may be associated with only one active User at a time.

### 2.3 Role

A Role groups users for authorization purposes.

Examples:
- employee
- student
- manager
- facilities
- visitor

Roles may have permissions assigned to them.

### 2.4 Permission

A Permission describes whether a User or Role may cross a controlled boundary.

A permission may contain:
- subject type: USER or ROLE
- subject identifier
- controlled boundary
- direction
- effect: ALLOW or DENY
- valid days
- valid time range
- enabled state

### 2.5 SecurityZone

A SecurityZone groups one or more building locations that share a common access-control classification.

Initial zone types include:

- PUBLIC
- STUDENT
- EMPLOYEE
- RESTRICTED

A SecurityZone:
- has a stable zone identifier
- has a human-readable name
- contains one or more BuildSim locations
- may participate in one or more controlled boundaries

Security zones express security intent and are defined by the access-control system rather than inferred solely from BuildSim topology.

A room belongs to one security zone in the initial design.

Examples:

```text
ZONE-PUBLIC
ZONE-STUDENT
ZONE-EMPLOYEE
ZONE-RESTRICTED
```

Security-zone membership is used by the authorization system to determine the boundary being crossed.

---

### 2.6 ControlledDoor

A ControlledDoor represents a BuildSim door for which the access-control system controls passage.

A ControlledDoor:
- has a stable controlled-door identifier
- references a corresponding BuildSim door
- connects two BuildSim locations
- connects their associated security zones
- defines whether access control applies in one or both directions
- has an associated door actuator
- has observable door state

A controlled boundary is directional.

For example, movement from:

```text
PUBLIC -> EMPLOYEE
```

may require authorization while movement from:

```text
EMPLOYEE -> PUBLIC
```

may not require credential presentation.

The ControlledDoor is the access-control representation of the physical BuildSim door. It does not duplicate BuildSim's authoritative physical state.

---

### 2.7 CardReader

A CardReader represents a simulated credential-reading device positioned at a controlled boundary.

A CardReader:
- has a stable device identifier
- is associated with one ControlledDoor
- is associated with one controlled direction
- runs as an independently restartable sensor process
- observes presentation of an AccessCard
- emits an access-attempt observation

A CardReader does not:
- determine authorization
- calculate anomaly scores
- unlock a door

Its responsibility ends after observing and reporting the credential presentation and relevant context.

---

### 2.8 OccupancySensor

An OccupancySensor represents a simulated sensor that observes occupancy for a building location or security zone.

An OccupancySensor:
- has a stable device identifier
- is associated with a room or security zone
- runs as an independently restartable sensor process
- reports observed occupancy
- includes observation time and data-quality information

Occupancy observations are derived from simulated physical behavior but are treated as sensor observations rather than as direct access to simulator state.

This distinction allows sensor delay, stale data, missing observations, and sensor failures to be evaluated independently from the underlying simulated truth.

---

### 2.9 DoorStateSensor

A DoorStateSensor observes the physical state of a controlled door, reported
as the two independent fields defined in §9.1:

- `door_position`: `OPEN` | `CLOSED` | `BLOCKED`
- `lock_state`: `UNLOCKED` | `LOCKED` | `JAMMED`

A `verification` value is not itself observed by this sensor; it is computed
by whichever component compares a commanded `lock_state` against this
sensor's reported value (§9.5, §9.6).

A DoorStateSensor:
- has a stable device identifier
- is associated with one ControlledDoor
- runs as an independently restartable sensor process
- reports observed door state
- provides evidence of whether an actuator command had the intended physical effect

The observed state is distinct from the requested actuator state.

For example:

```text
commanded lock_state = UNLOCKED
observed lock_state  = LOCKED
```

represents an actuator or communication failure rather than a successful unlock.

---

### 2.10 DoorActuator

A DoorActuator represents the independently running component responsible for applying access-control commands to a BuildSim door.

A DoorActuator:
- has a stable device identifier
- is associated with one ControlledDoor
- receives validated actuator commands
- interacts with BuildSim to change door or lock state
- reports the outcome of the command
- prevents repeated delivery of the same command from causing duplicate physical effects

The actuator does not determine whether access should be granted.

It executes commands that have already passed through the autonomous decision and runtime-assurance process.

A requested actuator state and an observed physical state are treated as distinct concepts.

---

## 3. Simulation Model

The simulation model represents the behavior of people in the building.

Simulation concepts are intentionally separated from authorization-domain concepts.

A User represents a person known to the access-control system.

A SimulatedPerson represents that person's behavior within an experiment.

This separation prevents expected behavior from becoming an implicit authorization rule.

### 3.1 SimulatedPerson

A SimulatedPerson represents a User participating in a simulation run.

A SimulatedPerson:
- has a stable simulation identifier
- references a User
- has a current simulated location
- has a Schedule
- has a BehaviorProfile
- has a MovementState

A SimulatedPerson may attempt actions that are:
- authorized and normal
- authorized and unusual
- unauthorized but behaviorally plausible
- unauthorized and anomalous

The simulator generates intent and movement attempts but does not decide whether controlled access is permitted.

---

### 3.2 Schedule

A Schedule describes expected time-dependent activities for a SimulatedPerson.

A schedule may define:

- expected arrival period
- expected departure period
- lunch period
- recurring meetings
- expected room or zone usage
- weekday/weekend differences

Schedule times are evaluated using model time.

Schedules describe typical behavior.

They do not grant access permission.

For example:

```text
Schedule:
    employee normally arrives between 07:30 and 09:00

Permission:
    employee may enter between 06:00 and 20:00
```

An arrival at 19:00 may therefore be authorized but behaviorally unusual.

---

### 3.3 BehaviorProfile

A BehaviorProfile defines the probabilistic behavioral tendencies of a SimulatedPerson or class of simulated persons.

A profile may include:

- likely destinations
- transition probabilities
- dwell-time distributions
- arrival/departure variation
- likelihood of attending scheduled activities
- role-dependent movement behavior

Behavior profiles introduce controlled stochastic variation into normal simulation runs.

Stochastic behavior shall use the configured simulation random seed so that an experiment can be reproduced.

---

### 3.4 MovementState

MovementState represents the current movement-related state of a SimulatedPerson.

It may include:

- current location
- intended destination
- current route
- waiting state
- pending controlled-door crossing
- most recent successful movement
- movement blocked status

The simulator owns MovementState.

The access-control system may observe movement-related evidence but does not directly modify a person's MovementState.

Movement across a controlled boundary occurs only after the simulator observes that the physical door state permits passage.

---

### 3.5 Scenario

A Scenario defines an explicit experimental condition or intervention.

A Scenario may specify:

- scenario identifier
- A Scenario defines the initial model time from which the simulator-controlled model clock begins.
- participating simulated persons
- random seed
- initial building state
- injected behavior
- injected faults
- expected high-level outcome

Scenarios support deterministic requirement verification and controlled evaluation.

Examples include:

- normal authorized access
- authorized access at an unusual time
- unauthorized restricted-zone access
- tailgating
- card-reader failure
- stale occupancy observation
- actuator failure
- duplicated observation
- delayed or out-of-order observation

The Scenario does not define the canonical system configuration. Users, doors, zones, and devices are configured separately and referenced by the scenario.

---

## 4. State Ownership

The design assigns authoritative ownership of state to avoid conflicting
copies of system truth.

| State | Authoritative Owner |
|---|---|
| Model time | Occupant and Movement Simulator |
| Current building topology | BuildSim |
| Current physical door/lock state | BuildSim |
| Simulated occupant location and occupancy truth | Occupant and Movement Simulator |
| Simulated occupant intent, schedule, behavior, and movement state | Occupant and Movement Simulator |
| Users, cards, roles, and permissions | Versioned System Configuration |
| Security-zone definitions | Versioned System Configuration |
| Controlled-door and device bindings | Versioned System Configuration |
| Validated observations | Event and Data Service |
| Historical observations, decisions, commands, and outcomes | Historical Evidence Store |
| Logical per-door control state | Autonomous Decision Service |
| Device-local runtime state | Individual device process |
| Component last-seen / stream-health state | Event and Data Service |

The same information may be cached or referenced by another component, but this does not change authoritative ownership.

When cached state conflicts with authoritative state, the system shall follow the documented freshness and degraded-state policy rather than silently treating the cached value as current.

### 4.1 Physical State Versus Observation

The design distinguishes between physical or simulated truth and sensor observation.

For example:

```text
Simulator/BuildSim state:
Room A contains 3 occupants

Occupancy sensor observation:
Room A reports 2 occupants
```

These values may intentionally differ because of:

- sensor delay
- missing data
- faults
- injected anomalies
- stale observations

This distinction is necessary for meaningful resilience and data-quality evaluation.

### 4.2 Commanded State Versus Observed State

An actuator command represents an intended physical action.

It does not prove the action occurred.

For example:

```text
AccessDecision: ALLOW
ActuatorCommand: UNLOCK
ActuatorResult: command accepted
DoorObservation: LOCKED
```

The final physical state must therefore be observed independently where required by the decision or evaluation process.


### 4.3 Model Time

Model time represents the simulated date and time within an experiment.

Model time is distinct from wall-clock time.

The simulator is the authoritative owner of model time.

Model time is represented as an absolute simulated date and time so that time-dependent behavior can be evaluated consistently, including:

- day-of-week permission constraints
- time-of-day permission constraints
- simulated schedules
- behavioral expectations
- scenario timing
- door-control behavior defined in model time

Example:

```text
2026-10-08T08:15:30
```

The relationship between model time and wall-clock time may be accelerated, slowed, or paused by the simulator.Therefore, components shall not infer model time from the local operating-system clock.

### 4.3.1 Distribution

The simulator shall include `model_time` on observations or simulation events
that originate from simulated behavior.

Downstream records derived from an observation shall preserve the model time
associated with the triggering event.

For example:

```text
CardPresented
    model_time = 2026-10-08T08:15:30
        |
        v
AuthorizationDecision
    model_time = 2026-10-08T08:15:30
        |
        v
AccessDecision
    model_time = 2026-10-08T08:15:30
```

Components shall not independently calculate model time from wall-clock time.

If a component requires the current model time outside the context of an
existing event, the mechanism for obtaining it shall be defined by the
corresponding interface contract.

### 4.3.2 Wall-Clock Time

Wall-clock time represents real execution time on the host system.

Wall-clock time is used for operational concerns including:

- network timeouts
- health checks
- process liveness
- communication latency
- recovery-time measurement
- performance measurement

Model time and wall-clock time shall not be used interchangeably.

### 4.3.3 Timer Semantics

Simulation behavior and policy conditions expressed in simulated time use
model time.

Operational safety and distributed-system failure detection may use wall-clock
time.

Where a safety behavior depends on model time, the design may additionally
define a wall-clock safety backstop so that failure or suspension of the
simulator cannot indefinitely prevent a safety action.

The exact backstop behavior for door relocking remains an open design decision.

### 4.4 System Configuration

Static access-control and topology configuration is owned by a versioned
System Configuration.

The System Configuration contains:

- Users
- AccessCards
- Roles
- Permissions
- SecurityZones
- ControlledDoors
- device identities
- controlled directions
- device-to-door bindings
- BuildSim identifiers required by project components

The configuration is stored as version-controlled machine-readable data.

Runtime components load only the configuration required for their responsibilities.

The System Configuration is not a separately deployed runtime service.

BuildSim describes physical building topology and runtime state, but it does not define access-control security intent.

A configuration version or identifier shall be retained with experiment evidence so that a run can be reproduced.

### 4.5 Occupancy Truth and Observation

The Occupant and Movement Simulator is authoritative for simulated occupant location and physical occupancy truth.

When a SimulatedPerson successfully moves from one location to another, the simulator updates its occupancy state.

BuildSim may receive a mirrored representation of occupancy for current-state visualization. This mirrored representation does not make BuildSim the authoritative source of simulated occupancy truth.

OccupancySensor processes observe simulated occupancy truth and produce OccupancyObservation records.

Sensor observations may intentionally differ from simulation truth because of:

- delay
- stale data
- missing observations
- injected sensor error
- sensor-process failure

Therefore the system distinguishes:

```text
Simulator occupancy truth
       |
       +------> BuildSim representation
       |
       +------> Occupancy Sensor
                     |
                     v
              OccupancyObservation
```
---

## 5. Authorization Model

Authorization determines whether a presented credential permits passage across a controlled boundary at a specific model time.

Authorization is deterministic.

Given the same:
- card state
- user
- role membership
- permissions
- controlled boundary
- direction
- model time

the authorization service shall produce the same result.

Authorization does not use anomaly score as an input when determining whether permission exists.

Authorization uses the `model_time` associated with the triggering access attempt and does not evaluate permission windows using wall-clock time.

### 5.1 Authorization Inputs

An authorization evaluation requires:

- AccessCard
- associated User
- User roles
- applicable user-specific permissions
- applicable role permissions
- ControlledDoor
- requested direction
- model time

### 5.2 Credential Validation

Before permissions are evaluated, the presented card is validated.

Authorization returns DENY if the card is:

- unknown
- disabled
- lost
- expired
- not yet valid
- not associated with an active User

### 5.3 Permission Matching

A permission matches an access attempt when:

- the permission is enabled
- the subject matches the User or one of the User's roles
- the controlled boundary matches
- the requested direction matches
- the model day satisfies the permission constraint
- the model time satisfies the permission constraint

Permissions that do not satisfy all applicable constraints are ignored.

### 5.4 Permission Precedence

When multiple applicable permissions exist, the initial precedence rule is:

```text
1. invalid credential        -> DENY
2. explicit user DENY        -> DENY
3. explicit user ALLOW       -> ALLOW
4. matching role DENY        -> DENY
5. matching role ALLOW       -> ALLOW
6. no applicable permission  -> DENY
```

User-specific permissions therefore override role permissions.

Within the same subject level, DENY takes precedence over ALLOW.

The policy is fail-closed: absence of a matching permission results in DENY.

### 5.5 Authorization Result

Authorization produces a result containing conceptually:

- ALLOW or DENY
- reason
- matched permission where applicable
- User
- AccessCard
- controlled boundary
- direction
- model time

The exact transport representation is defined in `data-contracts.md`.

Examples of authorization reasons may include:

- `CARD_UNKNOWN`
- `CARD_DISABLED`
- `CARD_EXPIRED`
- `USER_DENY`
- `USER_ALLOW`
- `ROLE_DENY`
- `ROLE_ALLOW`
- `NO_PERMISSION`

### 5.6 Authorization and Simulation Behavior

Authorization describes what a User is permitted to do.

Simulation behavior describes what a SimulatedPerson is likely to do.

These concepts shall remain separate.

For example:

```text
User:
    permitted to enter employee zone from 06:00 to 20:00

BehaviorProfile:
    normally enters between 07:30 and 09:00

Observed attempt:
    19:15
```

The attempt may therefore be:

```text
Authorization: ALLOW
Behavior: anomalous
```

This distinction is fundamental to the anomaly-aware access-control design.

## 6. Anomaly Model

Anomaly detection evaluates whether behavior is unusual.

It does not determine whether access is authorized.

Possible anomaly inputs include:
- model time
- user role
- current location
- requested destination
- recent movement history
- typical access times
- occupancy context

The anomaly detector returns:
- anomaly score
- anomaly classification
- reason/features where available

## 7. Decision Policy

The Decision Policy combines authorization, anomaly information, sensor quality, and current system state into a final access decision.

Authorization and anomaly detection remain separate inputs.

The Decision Policy does not redefine authorization rules. It decides what action the system should take after authorization and anomaly evaluation have completed.

### 7.1 Decision Inputs

A decision may use:

- Authorization result
- Anomaly result
- Access attempt context
- ControlledDoor
- requested direction
- model time
- relevant sensor-quality state
- relevant degraded-system state

The exact transport representation is defined in `data-contracts.md`.

### 7.2 Decision Outputs

The Decision Policy produces an AccessDecision.

Conceptually, an AccessDecision contains:

- decision identifier
- associated access-attempt identifier
- authorization result
- anomaly result where applicable
- final action
- reason
- model time
- quality/degraded-state information
- policy version

The initial final actions are:

- `ALLOW`
- `DENY`
- `ALLOW_WITH_ALERT`
- `DENY_WITH_ALERT`

Additional operational states such as actuator failure or unverified physical state are recorded separately from the access authorization decision.

### 7.3 Initial Decision Matrix

The initial policy is:

```text
Authorization    Anomaly        Final Decision
-------------    -----------    ----------------
ALLOW            NORMAL         ALLOW
ALLOW            ANOMALOUS      ALLOW_WITH_ALERT
DENY             NORMAL         DENY
DENY             ANOMALOUS      DENY_WITH_ALERT
```

The anomaly result does not change whether an otherwise unauthorized access attempt is permitted.

An authorized but anomalous access attempt remains authorized under the initial policy but generates an alert.

An unauthorized access attempt remains denied regardless of anomaly classification.

### 7.4 Missing or Unavailable Anomaly Result

Authorization shall not depend on successful anomaly evaluation.

If authorization succeeds but the anomaly detector is unavailable or unable to produce a valid result, the decision shall follow the documented degraded-state policy.

The initial degraded behavior is:

```text
Authorization = ALLOW
Anomaly result unavailable
-> ALLOW with degraded-anomaly status recorded
```

provided that no runtime-assurance constraint prevents actuation.

This behavior allows deterministic authorization to remain operational when the anomaly subsystem is unavailable.

### 7.5 Decision Determinism

For identical:

- authorization result
- anomaly result
- sensor-quality state
- degraded-state condition
- decision-policy version

the Decision Policy shall produce the same result.

This makes decisions reproducible and suitable for controlled evaluation.

### 7.6 Decision Evidence

Every final access decision shall be traceable to:

- the triggering access attempt
- authorization result
- anomaly result or anomaly-unavailable condition
- relevant quality/degraded-state information
- decision-policy version

This evidence shall be retained for replay and evaluation.

---

## 8. Runtime Assurance

Runtime Assurance is the final safety layer between the Decision Policy and physical actuation.

Its purpose is to prevent an otherwise valid software decision from producing an unsafe or invalid actuator command.

Runtime Assurance is intentionally simpler than the anomaly or decision logic.

It enforces hard constraints that must always hold.

### 8.1 Runtime-Assurance Inputs

Runtime Assurance may evaluate:

- final AccessDecision
- credential validity state
- authorization result
- observation freshness
- target ControlledDoor
- requested actuator action
- current known door state
- duplicate command state
- decision-service health
- actuator availability where known

### 8.2 Hard Safety Constraints

The following constraints shall apply:

```text
DENY decision
-> must never produce an UNLOCK command

Invalid, disabled, lost, or expired card
-> must never produce an UNLOCK command

Unauthorized access
-> must never be converted to ALLOW by anomaly information

Required stale authorization input
-> must not silently result in UNLOCK

Duplicate actuator command
-> must not create an additional physical side effect

Unknown or invalid target door
-> actuator command must be rejected

Unconfirmed physical actuation
-> physical state must not be represented as confirmed
```

### 8.3 Runtime-Assurance Outcomes

Runtime Assurance may produce:

- `COMMAND_APPROVED`
- `COMMAND_REJECTED`
- `COMMAND_SUPPRESSED_DUPLICATE`
- `COMMAND_BLOCKED_STALE_INPUT`
- `COMMAND_BLOCKED_INVALID_TARGET`
- `COMMAND_BLOCKED_DEGRADED_STATE`

The exact set of machine-readable values will be defined in `data-contracts.md`.

### 8.4 Fail-Closed Behavior

Where authorization cannot be safely evaluated, the system shall fail closed.

Examples include:

- authorization service unavailable
- required credential data unavailable
- permission data unavailable or invalid
- access decision cannot be validated

A failure of anomaly detection alone does not necessarily require fail-closed behavior because anomaly detection does not determine permission.

### 8.5 Separation of Decision and Assurance

The Decision Policy answers:

> What action should the system take?

Runtime Assurance answers:

> Is that action safe and valid to execute?

This separation permits more complex autonomous behavior while keeping the final actuator boundary governed by simple, explicit constraints.

---

## 9. Door State Machine

The access-control design distinguishes between logical access decisions, actuator commands, and observed physical door state.

A controlled door begins in a known safe state.

The normal initial state is:

```text
lock_state    = LOCKED
door_position = CLOSED
verification  = CONFIRMED
```

### 9.1 Logical Door States

door_position:
    OPEN | CLOSED | BLOCKED

lock_state:
    UNLOCKED | LOCKED | JAMMED

verification:
    CONFIRMED | UNVERIFIED | UNKNOWN

### 9.2 Normal Authorized Transition

The normal successful sequence, tracked as the three independent fields
defined in §9.1, is:

```text
lock_state=LOCKED,   door_position=CLOSED, verification=CONFIRMED
  |
  | valid ALLOW decision -> UNLOCK command confirmed
  v
lock_state=UNLOCKED, door_position=CLOSED, verification=CONFIRMED
  |
  | occupant opens door
  v
lock_state=UNLOCKED, door_position=OPEN,   verification=CONFIRMED
  |
  | passage completes / door closes
  v
lock_state=UNLOCKED, door_position=CLOSED, verification=CONFIRMED
  |
  | relock command confirmed
  v
lock_state=LOCKED,   door_position=CLOSED, verification=CONFIRMED
```

`door_position` and `lock_state` vary independently; a door may, for
example, be observed `OPEN` while `lock_state` is still `UNLOCKED` from an
earlier command. The two intermediate rows above are the same
`(UNLOCKED, CLOSED)` pair occurring before and after passage — there is no
separate named state for it, which is why this specification no longer uses
a single flattened enum value for door state (see §9.1).

### 9.3 Unlock Timeout

When an approved access attempt causes a previously locked door to become unlocked, the Autonomous Decision Service starts a per-door relock timer.

If no door-open observation is received within 10 model seconds:

1. a `DOOR_NOT_OPENED` event is produced;
2. the Autonomous Decision Service issues a `LOCK` command;
3. the DoorActuator applies the command to BuildSim;
4. physical door state is subsequently verified.

The timer uses model time as required by REQ-FUNC-003.

### 9.4 Denied Access

A denied access attempt shall not change the door from its locked state.

```text
lock_state=LOCKED, door_position=CLOSED, verification=CONFIRMED
  |
  | DENY
  v
lock_state=LOCKED, door_position=CLOSED, verification=CONFIRMED
```

No occupant crossing shall occur.

### 9.5 Unverified Actuation

If an actuator command is issued but the resulting physical state cannot be confirmed, `verification` becomes `UNVERIFIED` for that door. `door_position` is unaffected by a lock/unlock command and keeps its last-confirmed value; only `lock_state` is uncertain.

For example:

```text
lock_state=LOCKED,              door_position=CLOSED, verification=CONFIRMED
  |
  | UNLOCK command sent
  | confirmation unavailable
  v
lock_state=UNLOCKED (requested, not confirmed), door_position=CLOSED, verification=UNVERIFIED
```

`verification=UNVERIFIED` means the system does not know whether the requested `lock_state` transition occurred.

The system shall not represent the requested command state as confirmed physical truth.

### 9.6 Door State Ownership

BuildSim remains authoritative for the current physical door state.

The access-control system may maintain logical control state needed for:

- pending commands
- timeout handling
- command correlation
- verification
- degraded-state handling

Such logical state does not replace BuildSim as the source of current physical state.

### 9.7 Door-Control Ownership

The Autonomous Decision Service owns the logical per-door control state used to manage an active access cycle.

This state may include:

- associated access-decision ID
- most recent actuator-command ID
- unlock model time
- pending relock state
- verification state
- most recently issued command sequence

An AccessDecision and an ActuatorCommand are separate concepts.

Access decisions are:

```text
ALLOW
DENY
```

An alert is recorded separately from the access action.

Door actuator commands are:

```text
UNLOCK
LOCK
```

An ALLOW decision may cause an UNLOCK command.

Relocking does not require a new authorization decision. It is part of the door-control lifecycle owned by the Autonomous Decision Service.

---

## 10. Degraded-State Behavior

The system is expected to continue operating safely when individual distributed components fail or data quality degrades.

A degraded state is a condition in which the system remains partially operational but one or more expected capabilities or observations are unavailable.

Degraded behavior shall be explicit and observable.

### 10.1 Authorization or Decision Service Unavailable

If authorization or final decision-making is unavailable:

```text
new access attempts -> DENY / no unlock
```

A previously locked door shall remain locked.

The failure shall be recorded and exposed through system monitoring.

### 10.2 Anomaly Detector Unavailable

If authorization remains available but anomaly evaluation is unavailable:

```text
authorized access -> may proceed
anomaly status -> UNKNOWN / UNAVAILABLE
```

The decision shall record that anomaly evaluation was unavailable.

Anomaly-system failure shall not silently be interpreted as `NORMAL`.

### 10.3 Card Reader Unavailable

If the required card reader is unavailable:

- no new credential-based access attempt can be trusted from that reader
- the controlled boundary shall not unlock based on missing credential evidence
- the device failure shall be observable

### 10.4 Occupancy Sensor Unavailable or Stale

If occupancy information is unavailable or stale:

- the observation shall be marked degraded
- stale occupancy shall not silently be treated as current
- authorization may continue if occupancy is not required for permission evaluation
- anomaly confidence or classification may be degraded
- the degraded condition shall be retained in decision evidence where relevant

### 10.5 Door-State Sensor Unavailable

If the physical door state cannot be observed:

- actuation may become `UNVERIFIED`
- the system shall not assume that a command succeeded
- affected access events shall retain the unverified state
- the dashboard shall expose the degraded condition

### 10.6 Door Actuator Unavailable

If the actuator process is unavailable or cannot execute the command:

- the system shall record actuation failure
- the simulator shall not treat the door as successfully unlocked
- access shall not be considered physically completed
- the failure shall be visible for evaluation

### 10.7 Event/Data Service Unavailable

If the event/data service is unavailable:

- producers shall not silently treat delivery as successful
- failure shall be visible through bounded timeout/error behavior
- the system shall follow the interface-specific retry or failure policy
- duplicate retries must remain safe

Exact buffering and retry behavior remains an open implementation decision until communication contracts are defined.

### 10.8 BuildSim Unavailable

If BuildSim is unavailable:

- physical state changes cannot be confirmed
- simulator movement that depends on controlled-door state shall not proceed based on assumed state
- affected components shall report degraded operation
- components shall recover independently when BuildSim becomes available

### 10.9 Process Restart

Individual components shall support restart without requiring restart of unrelated components.

After restart, a device process shall:

- reconnect to required dependencies
- re-register with BuildSim if necessary
- avoid creating duplicate logical equipment
- resume normal processing
- preserve externally persisted evidence

### 10.10 Decision-Service Restart During Active Door Control

When the Autonomous Decision Service starts or restarts, it shall reconcile controlled-door state before assuming normal operation.

For each controlled door:

1. obtain the current observed physical state;
2. determine whether a valid active access cycle exists;
3. if the door is observed UNLOCKED with no valid active access cycle,
   issue a LOCK command;
4. record the reconciliation action.

A restart shall therefore not allow an orphaned unlocked door to remain indefinitely unlocked.

---

## 11. Design Invariants

Design invariants are conditions that must remain true regardless of implementation details.

### 11.1 Authorization Invariants

- Access without a valid applicable permission defaults to DENY.
- An invalid credential cannot authorize access.
- Authorization is deterministic for equivalent inputs.
- User-specific permissions take precedence over role permissions.
- Within the same subject level, DENY takes precedence over ALLOW.
- Authorization is evaluated against model time.
- Anomaly detection does not define authorization.

### 11.2 Decision Invariants

- A DENY authorization cannot result in an ALLOW access decision.
- An anomaly score alone cannot grant access.
- An anomalous authorized access attempt remains distinguishable from an unauthorized attempt.
- Every final decision is traceable to its triggering access attempt and decision inputs.

### 11.3 Actuation Invariants

- Only a validated command may be sent to a door actuator.
- Duplicate command delivery must not cause duplicate physical side effects.
- Commanded state and observed physical state are separate.
- An actuator request does not prove that the physical transition occurred.
- A locked door must not be represented as unlocked without observed or otherwise verified physical evidence.

### 11.4 Simulation Invariants

- The simulator generates intent and movement but does not authorize access.
- A SimulatedPerson cannot cross a locked controlled boundary unless the active scenario explicitly models forced entry.
- Successful movement changes subsequent occupancy state.
- Simulator behavior is reproducible for a fixed scenario and random seed.
- Schedule and BehaviorProfile describe expected behavior, not permission.

### 11.5 Data Invariants

- Retained events have stable identity.
- Duplicate observations remain detectable.
- Late or out-of-order data is not silently rewritten as on-time data.
- Malformed data does not silently enter the validated data path.
- Model time and wall-clock time remain semantically distinct.
- Historical evidence does not depend solely on BuildSim runtime state.

### 11.6 Distributed-System Invariants

- Required sensor, actuator, and autonomous decision components remain independently restartable.
- Restart of one component does not require restart of the complete system.
- Network operations use bounded waits rather than indefinite blocking.
- Failure of a remote dependency is handled as a normal distributed-system condition rather than assumed impossible.

### 11.7 State-Ownership Invariants

- BuildSim is authoritative for current physical building state.
- The simulator is authoritative for simulated occupant intent and movement state.
- Authorization configuration is authoritative for users, cards, roles, and permissions.
- Historical evidence is owned by the evidence/data layer rather than BuildSim.
- Cached state does not silently replace authoritative state when freshness is unknown.

---

## 12. Open Design Decisions

The following decisions remain intentionally unresolved.

They shall be resolved before the affected implementation is considered stable.

### 12.1 Communication Pattern

Decision required:

- direct HTTP between components
- message broker such as MQTT
- hybrid approach

Evaluation criteria include:

- simplicity
- failure behavior
- retry semantics
- fan-out needs
- replay requirements
- observability
- operational complexity

This decision should be captured in an ADR.

### 12.2 Persistent Evidence Storage

The logical requirement for durable historical evidence is defined, but the storage implementation remains open.

Candidate approaches include:

- append-only JSONL
- Parquet files
- DuckDB-backed analytical storage
- combination of raw and processed stores

The selected design should prioritize:

- reproducibility
- replay
- low operational complexity
- experiment analysis
- persistence across process restarts

### 12.3 Exact BuildSim Door Integration

The exact BuildSim API representation of:

- door identity
- lock state
- open/closed state
- command operations
- observed state

must be verified against the local BuildSim documentation before implementation.

No API behavior shall be assumed from this specification alone.

Confirmed against the local BuildSim API documentation and source
(`PUT /api/doors`, `pkg/store/memory.go` `SetDoors`): BuildSim replaces its
entire door collection on every `PUT /api/doors` call; it does not upsert a
single door by ID. The real `BuildSimClient` adapter (not yet implemented)
must therefore perform `GET /api/doors`, replace only its own door's entry
in the returned collection, and `PUT` the full collection back, inside a
single `SetDoorState` call. This read-modify-write sequence is an adapter
implementation detail and shall not leak into the `BuildSimClient`
interface or into any caller of it. `BuildSimClient` also has no
asynchronous confirmation from BuildSim itself: a successful write is
immediately consistent, so `verification=UNVERIFIED` is produced only by
the adapter's own failure/timeout handling, never by a BuildSim response.
The `FakeBuildSimClient` used for automated tests needs neither behavior,
since it holds a single door directly in memory.

### 12.4 Anomaly Detection Method

The anomaly architecture is defined independently from the specific algorithm.

The initial method remains open.

Candidate approaches may include:

- deterministic behavioral rules
- statistical thresholds
- Isolation Forest
- another lightweight unsupervised method

The selected learned or statistical approach should be compared against a simple deterministic baseline using identical scenarios and seeds.

### 12.5 Anomaly Thresholds

The following remain to be determined experimentally:

- anomaly-score threshold
- minimum evidence window
- treatment of low-confidence results
- behavior when required features are unavailable

Thresholds should be selected from evaluation evidence rather than chosen arbitrarily.

### 12.6 Freshness Thresholds

Maximum acceptable age remains to be defined for:

- card-reader observations
- occupancy observations
- door-state observations
- cached authorization information

Each freshness rule must explicitly state whether it uses model time or wall-clock time.

### 12.7 Timeout and Retry Values

The architecture requires bounded network calls, but exact values remain open.

Values shall be chosen and justified for:

- service calls
- BuildSim calls
- actuator confirmation
- component startup
- retry backoff

Retries that may repeat side-effecting operations must use idempotent command identity.

### 12.8 Performance Threshold

The target p95 access-decision latency remains TBD.

The threshold will be set after collecting an initial nominal-system baseline.

The final value should be justified relative to the simulated access-control use case.

### 12.9 Door Passage Timing

The detailed timing between:

- unlock
- door open
- occupant passage
- door close
- relock

requires finalization after the simulator and BuildSim door behavior are verified.

The already accepted requirement remains:

```text
If an approved door is unlocked but is not opened,
it shall relock within 10 model seconds.
```

### 12.10 Dashboard Implementation

The required information to display is defined, but the exact implementation remains open.

Possible approaches include:

- extending BuildSim visualization/session capabilities
- a lightweight separate web dashboard
- a combination of both

The choice should avoid duplicating BuildSim functionality without a clear benefit.

### 12.11 Model-Time Safety Backstop

REQ-FUNC-003 currently defines the door unlock timeout in model seconds.

A model-time timer may stop progressing if the simulator pauses or fails.

The design must determine whether door relocking additionally uses a
wall-clock safety backstop and, if so, the relationship between the
model-time and wall-clock limits.
