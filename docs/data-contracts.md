# Data Contracts

Status: partial — covers only the messages and records needed by the first
vertical slice (one controlled door, one card reader, ALLOW/DENY, no
anomaly detection, fake BuildSim client).

This document will grow as later slices add occupancy sensing, door-state
sensing, anomaly detection, fault injection, and the real BuildSim adapter.
It does not yet cover those.

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

`action` is `UNLOCK` or `LOCK`. `reason` is `ALLOW_UNLOCK` or
`RELOCK_TIMEOUT` in this slice. `command_id` is the idempotency key
(REQ-DIST-007): the Actuator must not apply the same `command_id` twice.

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

## 5. DoorEvent

A minimal evidence-only record for door-lifecycle events that are not
themselves an AccessDecision or ActuatorOutcome. This slice uses exactly
one: `DOOR_NOT_OPENED` (REQ-FUNC-003).

```json
{
  "event_type": "DOOR_NOT_OPENED",
  "door_id": "door-1",
  "decision_id": "uuid",
  "model_time": "2026-10-08T08:15:40Z"
}
```

## 6. Evidence Record Envelope

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
`door_event`. `recorded_at` is wall-clock receive time at the Event/Data
Service. This satisfies REQ-DATA-002 (survives restart, file-backed) and
REQ-TEST-002 (an attempt is traceable by following `event_id` →
`decision_id` → `command_id`).

## 7. System Configuration (file)

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

Day/time constraints (REQ-AUTH-005) are omitted fields, meaning
unconstrained; the precedence evaluator (ADR-001) still runs its full
precedence chain even though only one role-level permission exists.

## 8. Internal-only interfaces (not persisted, not evidence)

These exist only to coordinate the five processes and the test harness.
They carry no REQ-DATA-003 obligations because they are not retained
observations.

- Simulator `GET /pending-presentations?reader_id=`: returns zero or one
  queued `{card_id, door_id, direction, model_time}`, consumed on read.
- Simulator `POST /scenario/present`: test/scenario-control input that
  enqueues a presentation. `{reader_id, card_id, door_id, direction}`.
- Simulator `GET /doors/{door_id}/state`: passthrough read of the
  BuildSimClient's current state for that door.
- Simulator `GET /modeltime`: current model time.
- Simulator `POST /modeltime/advance`: `{seconds}` — test-only deterministic
  model-time advance, avoiding real-time waits in tests (§4.3 model time is
  independent of wall-clock time).

## 9. BuildSim contract

Not defined here. `BuildSimClient` (Go interface:
`GetDoorState`/`SetDoorState`) is the integration seam. This slice uses
only `FakeBuildSimClient`, an in-memory implementation with no network
protocol of its own and no claim about BuildSim's real API
(`design-specification.md` §12.3). The real adapter is implemented once
local BuildSim door API documentation is available.
