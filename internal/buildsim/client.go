// Package buildsim defines the integration seam to BuildSim
// (design-specification.md section 12.3). Only a fake, in-memory
// implementation exists so far. The real adapter is not implemented until
// the local BuildSim door API documentation's remaining open questions
// (entry_node_id mapping, level/room translation) are resolved against an
// actual running instance; see design-specification.md section 12.3 for
// the read-modify-write requirement the real adapter must hide.
package buildsim

import "fmt"

// LockState mirrors BuildSim's door lock_state field
// (docs/internal/D7065E/buildingsim/docs/api/doors.md).
type LockState string

const (
	LockStateLocked   LockState = "LOCKED"
	LockStateUnlocked LockState = "UNLOCKED"
	LockStateJammed   LockState = "JAMMED"
)

// DoorPosition mirrors BuildSim's door state field (open/closed/blocked).
// This slice does not drive door_position directly (no door-open
// simulation yet); it is carried here for shape-completeness with
// design-specification.md section 9.1 and is not exercised by the
// vertical-slice tests.
type DoorPosition string

const (
	DoorPositionClosed  DoorPosition = "CLOSED"
	DoorPositionOpen    DoorPosition = "OPEN"
	DoorPositionBlocked DoorPosition = "BLOCKED"
)

// DoorState is the pair of independent fields BuildSim actually reports
// (design-specification.md section 9.1). Verification is not part of
// DoorState: it is computed by the caller (Actuator/Simulator), not
// observed from BuildSim (design-specification.md section 9.5).
type DoorState struct {
	LockState    LockState
	DoorPosition DoorPosition
}

// Client is the BuildSimClient abstraction. Callers never see BuildSim's
// read-modify-write requirement on PUT /api/doors; an implementation is
// responsible for hiding that (design-specification.md section 12.3).
type Client interface {
	// GetDoorState returns the current state of doorID.
	GetDoorState(doorID string) (DoorState, error)
	// SetDoorState requests that doorID's lock_state become lockState.
	// door_position is not modified by this call.
	SetDoorState(doorID string, lockState LockState) error
}

// ErrDoorNotFound is returned when doorID is not known to the client.
type ErrDoorNotFound struct{ DoorID string }

func (e ErrDoorNotFound) Error() string {
	return fmt.Sprintf("buildsim: door %q not found", e.DoorID)
}
