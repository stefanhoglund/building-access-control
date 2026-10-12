// Package contracts defines the Go representations of the messages and
// records described in docs/data-contracts.md. It covers the first
// vertical slice (one controlled door, ALLOW/DENY authorization, no
// anomaly detection) and the successful-passage CPS slice (REQ-FUNC-004,
// REQ-SIM-005, REQ-SIM-006): door-state observation, occupant crossing,
// and occupancy observation.
package contracts

// AccessAttemptObservation is emitted by a Card Reader. See
// docs/data-contracts.md section 1. receive_time is empty until the
// Event/Data Service fills it in on ingestion (REQ-DATA-003).
type AccessAttemptObservation struct {
	EventID       string `json:"event_id"`
	SchemaVersion string `json:"schema_version"`
	SourceID      string `json:"source_id"`
	ModelTime     string `json:"model_time"`
	SourceTime    string `json:"source_time"`
	ReceiveTime   string `json:"receive_time,omitempty"`
	CardID        string `json:"card_id"`
	DoorID        string `json:"door_id"`
	Direction     string `json:"direction"`
}

// Authorization results (design-specification.md section 5.5).
const (
	AuthResultAllow = "ALLOW"
	AuthResultDeny  = "DENY"
)

// Authorization reasons reachable by this slice's configuration
// (design-specification.md section 5.5).
const (
	ReasonUserDeny     = "USER_DENY"
	ReasonUserAllow    = "USER_ALLOW"
	ReasonRoleDeny     = "ROLE_DENY"
	ReasonRoleAllow    = "ROLE_ALLOW"
	ReasonNoPermission = "NO_PERMISSION"
	ReasonCardUnknown  = "CARD_UNKNOWN"
	ReasonCardDisabled = "CARD_DISABLED"
)

// AnomalyNotEvaluated is the degraded-state value (design-specification.md
// section 7.4: the anomaly subsystem is unavailable, not that it returned
// a normal result). See the AnomalyNormal/AnomalyAnomalous constants below
// for the classifications the scorer actually produces.
const AnomalyNotEvaluated = "NOT_EVALUATED"

// Final decision actions (design-specification.md section 9.7). Alerts are
// a separate concern and not used by this slice.
const (
	FinalActionAllow = "ALLOW"
	FinalActionDeny  = "DENY"
)

// PolicyVersion identifies the decision-policy logic version
// (REQ-DEC-007). It is a placeholder constant until the policy evolves.
const PolicyVersion = "v0"

// Anomaly classifications (design-specification.md section 6).
// AnomalyNotEvaluated remains a possible value for the degraded-state
// case in section 7.4 (anomaly subsystem unavailable); the deterministic
// scorer implemented in internal/decision/anomaly.go cannot itself fail,
// so it never produces this value today.
const (
	AnomalyNormal    = "NORMAL"
	AnomalyAnomalous = "ANOMALOUS"
)

// AccessDecision is produced by the Autonomous Decision Service. See
// docs/data-contracts.md section 2. FinalAction is determined solely by
// AuthorizationResult (design-specification.md section 11.2: "A DENY
// authorization cannot result in an ALLOW access decision") — Alert is a
// separate field precisely so anomaly information can never be folded
// into, or mistaken for, the authorization outcome itself (section 9.7).
type AccessDecision struct {
	DecisionID          string  `json:"decision_id"`
	EventID             string  `json:"event_id"`
	DoorID              string  `json:"door_id"`
	Direction           string  `json:"direction"`
	AuthorizationResult string  `json:"authorization_result"`
	AuthorizationReason string  `json:"authorization_reason"`
	AnomalyResult       string  `json:"anomaly_result"`
	AnomalyReason       string  `json:"anomaly_reason"`
	AnomalyScore        float64 `json:"anomaly_score"`
	FinalAction         string  `json:"final_action"`
	Alert               bool    `json:"alert"`
	ModelTime           string  `json:"model_time"`
	PolicyVersion       string  `json:"policy_version"`
}

// Actuator command actions and reasons (design-specification.md section 9.1/9.7).
const (
	ActionUnlock = "UNLOCK"
	ActionLock   = "LOCK"

	ReasonAllowUnlock     = "ALLOW_UNLOCK"
	ReasonRelockTimeout   = "RELOCK_TIMEOUT"
	ReasonPassageComplete = "RELOCK_AFTER_PASSAGE"
)

// ActuatorCommand is sent by the Autonomous Decision Service to the Door
// Actuator. CommandID is the REQ-DIST-007 idempotency key. See
// docs/data-contracts.md section 3.
type ActuatorCommand struct {
	CommandID  string `json:"command_id"`
	DecisionID string `json:"decision_id"`
	DoorID     string `json:"door_id"`
	Action     string `json:"action"`
	Reason     string `json:"reason"`
	ModelTime  string `json:"model_time"`
}

// Actuator outcome results (design-specification.md section 9.1, "verification").
const (
	OutcomeConfirmed  = "CONFIRMED"
	OutcomeUnverified = "UNVERIFIED"
)

// ActuatorOutcome is returned by the Door Actuator to the Event/Data
// Service. See docs/data-contracts.md section 4.
type ActuatorOutcome struct {
	CommandID     string `json:"command_id"`
	DoorID        string `json:"door_id"`
	Action        string `json:"action"`
	Result        string `json:"result"`
	ObservedState string `json:"observed_state"`
	ModelTime     string `json:"model_time"`
}

// DoorPosition values an observation or BuildSim call may carry
// (design-specification.md section 9.1).
const (
	DoorPositionOpen    = "OPEN"
	DoorPositionClosed  = "CLOSED"
	DoorPositionBlocked = "BLOCKED"
)

// ObservationQualityOK is the only data-quality value this slice
// produces for sensor observations; no fault injection yet degrades it
// (REQ-RES-003 is deferred).
const ObservationQualityOK = "OK"

// DoorStateObservation is emitted by a DoorStateSensor, which polls
// BuildSimClient.GetDoorState and reports only door_position
// (design-specification.md section 2.9). It is the one authoritative
// report that a door physically opened or closed — the Simulator causes
// the transition (SetDoorPosition) but is never treated as having
// observed it (section 4.1). It carries no decision_id: the Autonomous
// Decision Service correlates door_id to its own per-door control state
// instead (section 9.7). See docs/data-contracts.md section 5.
type DoorStateObservation struct {
	EventID       string `json:"event_id"`
	SchemaVersion string `json:"schema_version"`
	SourceID      string `json:"source_id"`
	ModelTime     string `json:"model_time"`
	SourceTime    string `json:"source_time"`
	ReceiveTime   string `json:"receive_time,omitempty"`
	DoorID        string `json:"door_id"`
	DoorPosition  string `json:"door_position"`
	Quality       string `json:"quality"`
}

// DoorEventNotOpened is the one DoorEvent type used by this slice,
// produced by the Autonomous Decision Service itself (REQ-FUNC-003). See
// docs/data-contracts.md section 6.
const DoorEventNotOpened = "DOOR_NOT_OPENED"

// DoorEvent is a minimal evidence-only record for door-lifecycle events
// that are not themselves an AccessDecision, ActuatorOutcome, or
// DoorStateObservation.
type DoorEvent struct {
	EventType  string `json:"event_type"`
	DoorID     string `json:"door_id"`
	DecisionID string `json:"decision_id,omitempty"`
	ModelTime  string `json:"model_time"`
}

// OccupancyObservation is emitted by an OccupancySensor
// (design-specification.md section 2.8/4.5). See docs/data-contracts.md
// section 7.
type OccupancyObservation struct {
	EventID       string `json:"event_id"`
	SchemaVersion string `json:"schema_version"`
	SourceID      string `json:"source_id"`
	ModelTime     string `json:"model_time"`
	SourceTime    string `json:"source_time"`
	ReceiveTime   string `json:"receive_time,omitempty"`
	RoomKey       string `json:"room_key"`
	Count         int    `json:"count"`
	Quality       string `json:"quality"`
}

// Evidence record types (docs/data-contracts.md section 8).
const (
	RecordTypeObservation          = "observation"
	RecordTypeDecision             = "decision"
	RecordTypeCommand              = "command"
	RecordTypeOutcome              = "outcome"
	RecordTypeDoorEvent            = "door_event"
	RecordTypeDoorStateObservation = "door_state_observation"
	RecordTypeOccupancyObservation = "occupancy_observation"
)

// PendingPresentation is the Simulator's internal (non-evidence) record of
// a queued credential presentation, observed by a Card Reader via
// GET /pending-presentations. See docs/data-contracts.md section 8.
type PendingPresentation struct {
	CardID    string `json:"card_id"`
	DoorID    string `json:"door_id"`
	Direction string `json:"direction"`
	ModelTime string `json:"model_time"`
}

// ScenarioPresentRequest is the test/scenario-control input that enqueues a
// PendingPresentation for a specific reader.
type ScenarioPresentRequest struct {
	ReaderID  string `json:"reader_id"`
	CardID    string `json:"card_id"`
	DoorID    string `json:"door_id"`
	Direction string `json:"direction"`
}

// ScenarioRunRequest invokes one of the Simulator's named, seeded
// scenarios (design-specification.md section 3.5). Count/IntervalMillis
// only apply to the "burst" scenario kind and override its configured
// defaults when nonzero; they are ignored for every other kind.
type ScenarioRunRequest struct {
	Scenario       string `json:"scenario"`
	Seed           int64  `json:"seed"`
	Count          int    `json:"count,omitempty"`
	IntervalMillis int    `json:"interval_ms,omitempty"`
}

// ScenarioRunResult is returned once every presentation a scenario run
// specifies has been enqueued. It is also what gets appended to
// evidence/scenarios/runs.jsonl (reproducibility metadata, not part of the
// Historical Evidence Store proper — see internal/simulator/scenarios.go).
type ScenarioRunResult struct {
	RunID         string   `json:"run_id"`
	Scenario      string   `json:"scenario"`
	Seed          int64    `json:"seed"`
	ModelStart    string   `json:"model_start"`
	StepsExecuted int      `json:"steps_executed"`
	CardIDs       []string `json:"card_ids"`
}
