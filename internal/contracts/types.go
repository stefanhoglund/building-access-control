// Package contracts defines the Go representations of the messages and
// records described in docs/data-contracts.md. It covers only the first
// vertical slice: one controlled door, ALLOW/DENY authorization, no
// anomaly detection.
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

	ReasonAllowUnlock   = "ALLOW_UNLOCK"
	ReasonRelockTimeout = "RELOCK_TIMEOUT"
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

// DoorEventNotOpened is the one DoorEvent type used by this slice
// (REQ-FUNC-003). See docs/data-contracts.md section 5.
const DoorEventNotOpened = "DOOR_NOT_OPENED"

// DoorEvent is a minimal evidence-only record for door-lifecycle events
// that are not themselves an AccessDecision or ActuatorOutcome.
type DoorEvent struct {
	EventType  string `json:"event_type"`
	DoorID     string `json:"door_id"`
	DecisionID string `json:"decision_id"`
	ModelTime  string `json:"model_time"`
}

// Evidence record types (docs/data-contracts.md section 6).
const (
	RecordTypeObservation = "observation"
	RecordTypeDecision    = "decision"
	RecordTypeCommand     = "command"
	RecordTypeOutcome     = "outcome"
	RecordTypeDoorEvent   = "door_event"
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
