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

// DoorPosition mirrors BuildSim's door state field (open/closed/blocked),
// design-specification.md section 9.1. It is driven by the Simulator
// simulating the occupant's physical interaction with the door, not by
// the Door Actuator (which drives LockState only) — design-specification.md
// section 2.6/9.7.
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

// Person is the minimal subset of BuildSim's occupancy Person object this
// system uses (docs/internal/D7065E/buildingsim/pkg/model/person.go).
type Person struct {
	ID   string
	Name string
}

// RoomOccupancy is the minimal subset of BuildSim's RoomOccupancy object
// this system mirrors (docs/internal/D7065E/buildingsim/pkg/model/
// session.go). BuildSim's "aliens" field is not used and always written
// empty.
type RoomOccupancy struct {
	Persons []Person
}

// Client is the BuildSimClient abstraction. Callers never see BuildSim's
// read-modify-write requirement on PUT /api/doors or PUT /api/occupancy;
// an implementation is responsible for hiding that
// (design-specification.md section 12.3).
//
// There is deliberately no GetRoomOccupancy: BuildSim's mirrored occupancy
// is a one-way visualization copy, never a second source of truth
// (design-specification.md section 4.5, ADR-006). Nothing in this system
// reads occupancy back from BuildSim.
type Client interface {
	// GetDoorState returns the current state of doorID.
	GetDoorState(doorID string) (DoorState, error)
	// SetDoorState requests that doorID's lock_state become lockState.
	// door_position is not modified by this call. Used by the Door
	// Actuator only.
	SetDoorState(doorID string, lockState LockState) error
	// SetDoorPosition requests that doorID's door_position (open/closed)
	// become position. lock_state is not modified by this call. Used by
	// the Simulator only, to drive the physical open/close transition it
	// is simulating.
	SetDoorPosition(doorID string, position DoorPosition) error
	// SetRoomOccupancy mirrors the Simulator's occupancy truth for
	// roomKey (a level-qualified identifier such as "level0/A109") into
	// BuildSim for visualization. Used by the Simulator only.
	SetRoomOccupancy(roomKey string, occupancy RoomOccupancy) error
}

// ErrDoorNotFound is returned when doorID is not known to the client.
type ErrDoorNotFound struct{ DoorID string }

func (e ErrDoorNotFound) Error() string {
	return fmt.Sprintf("buildsim: door %q not found", e.DoorID)
}
