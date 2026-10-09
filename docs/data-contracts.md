# Data Contracts

Status: partial — covers the first vertical slice (one controlled door,
one card reader, ALLOW/DENY, no anomaly detection), the real BuildSim
adapter, and the successful-passage CPS slice (REQ-FUNC-004, REQ-SIM-005,
REQ-SIM-006: door-state sensing, occupant crossing, occupancy sensing).

This document will grow as later slices add anomaly detection, fault
injection, and additional doors. It does not yet cover those.

All timestamps are RFC3339 strings. `model_time` is owned by the Simulator
(ADR-003) and is never computed locally by another component. `source_time`
and `receive_time` are wall-clock.

## 1. AccessAttemptObservation

Emitted by a Card Reader after polling the Simulator and observing a
pending credential presentation. Carries the REQ-DATA-003 required fields.

```json
{
  "event_id": "uuid",
  "schema_version": "1",
  "source_id": "reader-1",
  "model_time": "2026-10-08T08:15:30Z",
  "source_time": "2026-10-08T10:00:00.000Z",
  "receive_time": "2026-10-08T10:00:00.050Z",
  "card_id": "card-allow",
  "door_id": "door-1",
  "direction": "PUBLIC_TO_EMPLOYEE"
}
```

`receive_time` is empty when sent by the Card Reader and filled in by the
Event/Data Service on ingestion (REQ-DATA-003).

## 2. AccessDecision

Produced by the Autonomous Decision Service and returned to the Event/Data
Service for persistence (ADR-005).

```json
{
  "decision_id": "uuid",
  "event_id": "uuid",
  "door_id": "door-1",
  "direction": "PUBLIC_TO_EMPLOYEE",
  "authorization_result": "ALLOW",
  "authorization_reason": "ROLE_ALLOW",
  "anomaly_result": "NOT_EVALUATED",
  "final_action": "ALLOW",
  "model_time": "2026-10-08T08:15:30Z",
  "policy_version": "v0"
}
```

`authorization_reason` is one of: `ROLE_ALLOW`, `NO_PERMISSION`,
`CARD_UNKNOWN`, `CARD_DISABLED`. Others listed in
`design-specification.md` §5.5 are not reachable with this slice's
configuration and are added when exercised.

`anomaly_result` is always `NOT_EVALUATED` until anomaly detection exists
(§7.4 degraded-anomaly behavior covers this case already).

## 3. ActuatorCommand

Sent by the Autonomous Decision Service to the Door Actuator.

```json
{
  "command_id": "uuid",
  "decision_id": "uuid",
  "door_id": "door-1",
  "action": "UNLOCK",
  "reason": "ALLOW_UNLOCK",
  "model_time": "2026-10-08T08:15:30Z"
}
```

`action` is `UNLOCK` or `LOCK`. `reason` is one of `ALLOW_UNLOCK`,
`RELOCK_TIMEOUT` (REQ-FUNC-003: no door-open observation within 10 model
seconds), or `RELOCK_AFTER_PASSAGE` (REQ-FUNC-004: a door-close
observation arrived for a door that was observed open). `command_id` is
the idempotency key (REQ-DIST-007): the Actuator must not apply the same
`command_id` twice.

## 4. ActuatorOutcome

Returned by the Door Actuator to the Event/Data Service (ADR-005).

```json
{
  "command_id": "uuid",
  "door_id": "door-1",
  "action": "UNLOCK",
  "result": "CONFIRMED",
  "observed_state": "UNLOCKED",
  "model_time": "2026-10-08T08:15:30Z"
}
```

`result` is `CONFIRMED` when the BuildSim client's reported state matches
the requested state, `UNVERIFIED` otherwise. `UNVERIFIED` handling beyond
recording it is deferred (REQ-RES-002 is out of scope for this slice).

## 5. DoorStateObservation

Emitted by a DoorStateSensor, which polls `BuildSimClient.GetDoorState`
and reports only `door_position` (REQ-DATA-003 fields plus the observed
value). This is the one authoritative report of a door physically opening
or closing — the Simulator causes the physical transition
(`SetDoorPosition`) but is never treated as having observed it
(design-specification.md §2.9/§4.1: simulated/physical truth and sensor
observation are always distinct, even when the same component could
technically report both).

```json
{
  "event_id": "uuid",
  "schema_version": "1",
  "source_id": "doorstatesensor-1",
  "model_time": "2026-10-08T08:15:31Z",
  "source_time": "2026-10-08T10:00:01.000Z",
  "receive_time": "2026-10-08T10:00:01.050Z",
  "door_id": "door-1",
  "door_position": "OPEN",
  "quality": "OK"
}
```

`door_position` is `OPEN`, `CLOSED`, or `BLOCKED`
(design-specification.md §9.1). `model_time` is not inferred by the sensor
from its own observation — it is read from the Simulator at the moment of
observation (§4.3.1: "If a component requires the current model time
outside the context of an existing event, the mechanism for obtaining it
shall be defined by the corresponding interface contract"), since a
physical door transition is not itself a simulation-originated event
carrying `model_time` already. `quality` is always `OK` in this slice; full
staleness/freshness handling (REQ-RES-003) is deferred to a
fault-injection slice. The sensor reports only on a change in
`door_position`, not on every poll.

**Correlation.** This observation carries no `decision_id`: the sensor has
no way to know one, and is not supposed to — attaching a decision to a
physical observation would blur the authorization/observation boundary
this specification maintains elsewhere. The Autonomous Decision Service
correlates an incoming `door_id` to its own per-door control state
(design-specification.md §9.7): an `OPEN` observation for a door with a
pending unlock marks it opened (disarming the REQ-FUNC-003 no-open
timeout); a subsequent `CLOSED` observation for that same door, now marked
opened, triggers `RELOCK_AFTER_PASSAGE` (REQ-FUNC-004). A `CLOSED`
observation for a door with no pending state, or not yet marked opened, is
not a passage-complete signal and is ignored by the Decision Service
(though still retained as evidence).

## 6. DoorEvent

A minimal evidence-only record for door-lifecycle events that are not
themselves an AccessDecision, ActuatorOutcome, or DoorStateObservation.
This slice uses exactly one: `DOOR_NOT_OPENED` (REQ-FUNC-003), produced by
the Autonomous Decision Service itself, not by a sensor.

```json
{
  "event_type": "DOOR_NOT_OPENED",
  "door_id": "door-1",
  "decision_id": "uuid",
  "model_time": "2026-10-08T08:15:40Z"
}
```

## 7. OccupancyObservation

Emitted by an OccupancySensor, which polls the Simulator's occupancy truth
for one room (design-specification.md §2.8/§4.5) — never BuildSim's
mirrored copy of it, which is visualization-only (ADR-006).

```json
{
  "event_id": "uuid",
  "schema_version": "1",
  "source_id": "occupancysensor-1",
  "model_time": "2026-10-08T08:15:31Z",
  "source_time": "2026-10-08T10:00:01.000Z",
  "receive_time": "2026-10-08T10:00:01.050Z",
  "room_key": "level0/A109",
  "count": 1,
  "quality": "OK"
}
```

`room_key` is a level-qualified room identifier (REQ-TOPO-004). `count` is
the occupancy truth at the moment of observation. `quality` is always `OK`
in this slice (see DoorStateObservation above — the same REQ-RES-003
deferral applies). The sensor reports only on a change in `count`.

## 8. Evidence Record Envelope

The Historical Evidence Store is an append-only JSONL file. The Event/Data
Service is the only writer (ADR-005). Every line is one envelope:

```json
{
  "record_type": "observation",
  "recorded_at": "2026-10-08T10:00:00.050Z",
  "payload": { "...": "one of the message types above" }
}
```

`record_type` is one of `observation`, `decision`, `command`, `outcome`,
`door_event`, `door_state_observation`, `occupancy_observation`.
`recorded_at` is wall-clock receive time at the Event/Data Service. This
satisfies REQ-DATA-002 (survives restart, file-backed) and REQ-TEST-002
(an attempt is traceable by following `event_id` → `decision_id` →
`command_id`).

## 9. System Configuration (file)

Loaded only by the Autonomous Decision Service in this slice (ADR-004).
JSON, not YAML, to avoid adding a parsing dependency.

```json
{
  "zones": [
    { "zone_id": "ZONE-PUBLIC", "name": "Public" },
    { "zone_id": "ZONE-STUDENT", "name": "Student" },
    { "zone_id": "ZONE-EMPLOYEE", "name": "Employee" },
    { "zone_id": "ZONE-RESTRICTED", "name": "Restricted" }
  ],
  "controlled_doors": [
    {
      "door_id": "door-1",
      "from_zone": "ZONE-PUBLIC",
      "to_zone": "ZONE-EMPLOYEE",
      "from_room": "level0/1540",
      "to_room": "level0/A109",
      "direction": "PUBLIC_TO_EMPLOYEE",
      "reader_id": "reader-1",
      "actuator_id": "actuator-1"
    }
  ],
  "roles": [
    { "role_id": "employee" },
    { "role_id": "visitor" }
  ],
  "users": [
    { "user_id": "user-allow", "roles": ["employee"] },
    { "user_id": "user-deny", "roles": ["visitor"] }
  ],
  "access_cards": [
    { "card_id": "card-allow", "user_id": "user-allow", "status": "ACTIVE" },
    { "card_id": "card-deny", "user_id": "user-deny", "status": "ACTIVE" }
  ],
  "permissions": [
    {
      "subject_type": "ROLE",
      "subject_id": "employee",
      "door_id": "door-1",
      "direction": "PUBLIC_TO_EMPLOYEE",
      "effect": "ALLOW",
      "enabled": true
    }
  ]
}
```

`from_room`/`to_room` are level-qualified room identifiers (REQ-TOPO-004)
— the two rooms occupancy truth moves between on a successful crossing —
distinct from `from_zone`/`to_zone`'s security classification. Day/time
constraints (REQ-AUTH-005) are omitted fields, meaning unconstrained; the
precedence evaluator (ADR-001) still runs its full precedence chain even
though only one role-level permission exists.

## 10. Internal-only interfaces (not persisted, not evidence)

These exist only to coordinate the processes and the test harness. They
carry no REQ-DATA-003 obligations because they are not retained
observations.

- Simulator `GET /pending-presentations?reader_id=`: returns zero or one
  queued `{card_id, door_id, direction, model_time}`, consumed on read.
- Simulator `POST /scenario/present`: test/scenario-control input that
  enqueues a presentation. `{reader_id, card_id, door_id, direction}`.
- Simulator `GET /doors/{door_id}/state`: passthrough read of the
  BuildSimClient's current state for that door.
- Simulator `GET /occupancy?room=`: returns `{room_key, count}` — the
  Simulator's current occupancy truth for that room, polled by
  OccupancySensor.
- Simulator `GET /modeltime`: current model time. Also polled by
  DoorStateSensor and OccupancySensor to stamp their observations
  (see §5/§7).
- Simulator `POST /modeltime/advance`: `{seconds}` — test-only deterministic
  model-time advance, avoiding real-time waits in tests (§4.3 model time is
  independent of wall-clock time).

## 11. BuildSim contract

`BuildSimClient` (Go interface) is the integration seam:

- `GetDoorState(doorID)` — read `lock_state`/`door_position`. Used by the
  Door Actuator (to verify its own commands) and by DoorStateSensor (to
  observe passage).
- `SetDoorState(doorID, lockState)` — used by the Door Actuator only.
- `SetDoorPosition(doorID, position)` — used by the Simulator only, to
  cause the physical open/close transition it is simulating.
- `SetRoomOccupancy(roomKey, occupancy)` — used by the Simulator only, to
  mirror occupancy truth for visualization. There is no corresponding
  `GetRoomOccupancy`: nothing in this system reads occupancy back from
  BuildSim (ADR-006).

Two implementations exist: `FakeBuildSimClient` (in-memory, used by all
automated tests) and `HTTPClient` (the real adapter, verified against
`docs/internal/D7065E/buildingsim`). The real adapter hides BuildSim's
whole-collection replace semantics on both `PUT /api/doors` and
`PUT /api/occupancy` behind a read-modify-write
(design-specification.md §12.3) — callers never see it.
