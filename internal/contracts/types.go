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
	ReasonRoleAllow    = "ROLE_ALLOW"
	ReasonNoPermission = "NO_PERMISSION"
	ReasonCardUnknown  = "CARD_UNKNOWN"
	ReasonCardDisabled = "CARD_DISABLED"
)

// AnomalyNotEvaluated is the only anomaly_result value used until anomaly
// detection is implemented (design-specification.md section 7.4).
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

// AccessDecision is produced by the Autonomous Decision Service. See
// docs/data-contracts.md section 2.
type AccessDecision struct {
	DecisionID          string `json:"decision_id"`
	EventID             string `json:"event_id"`
	DoorID              string `json:"door_id"`
	Direction           string `json:"direction"`
	AuthorizationResult string `json:"authorization_result"`
	AuthorizationReason string `json:"authorization_reason"`
	AnomalyResult       string `json:"anomaly_result"`
	FinalAction         string `json:"final_action"`
	ModelTime           string `json:"model_time"`
	PolicyVersion       string `json:"policy_version"`
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
