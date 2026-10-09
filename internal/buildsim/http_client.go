package buildsim

import (
	"strings"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/httpx"
)

// HTTPClient is the real BuildSimClient adapter, verified against
// docs/internal/D7065E/buildingsim/docs/api/doors.md and its
// implementation (pkg/server/handlers/visualization.go, pkg/store/
// memory.go). It implements Client.
//
// Two things about BuildSim's actual API are deliberately kept out of the
// Client interface and out of every caller:
//
//   - PUT /api/doors replaces BuildSim's entire door collection
//     (confirmed in pkg/store/memory.go: SetDoors does
//     `s.doors = cloneDoors(doors)`, not an upsert by ID). SetDoorState
//     therefore reads the full collection, replaces only its own door's
//     entry, and writes the full collection back. A caller of
//     SetDoorState never sees this.
//   - BuildSim's wire values are lowercase ("locked", "closed", ...);
//     Client's LockState/DoorPosition constants are uppercase. The
//     translation happens only in this file.
//
// design-specification.md section 12.3 records both requirements.
type HTTPClient struct {
	baseURL string
}

// NewHTTPClient creates a real BuildSimClient against the BuildSim
// instance at baseURL (e.g. "http://127.0.0.1:9090").
func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{baseURL: strings.TrimRight(baseURL, "/")}
}

// wireDoor mirrors BuildSim's Door JSON shape
// (pkg/model/visualization.go). Only the fields this adapter reads or
// writes are included; others round-trip through json.RawMessage-free
// zero values and are simply omitted on write, which is safe because
// SetDoorState only ever mutates a door object it just read back from
// BuildSim.
type wireDoor struct {
	ID          string      `json:"id"`
	Name        string      `json:"name,omitempty"`
	Kind        string      `json:"kind,omitempty"`
	Level       string      `json:"level"`
	Room        string      `json:"room,omitempty"`
	EntryNodeID *int        `json:"entry_node_id,omitempty"`
	Position    *[2]float64 `json:"position,omitempty"`
	State       string      `json:"state"`
	LockState   string      `json:"lock_state"`
}

// GetDoorState reads doorID's current state via GET /api/doors
// (docs/internal/D7065E/buildingsim/docs/api/doors.md).
func (c *HTTPClient) GetDoorState(doorID string) (DoorState, error) {
	doors, err := c.getDoors()
	if err != nil {
		return DoorState{}, err
	}
	for _, d := range doors {
		if d.ID == doorID {
			return DoorState{
				LockState:    parseLockState(d.LockState),
				DoorPosition: parseDoorPosition(d.State),
			}, nil
		}
	}
	return DoorState{}, ErrDoorNotFound{DoorID: doorID}
}

// SetDoorState requests lockState for doorID. It performs the
// GET-modify-PUT sequence PUT /api/doors requires, hidden from the
// caller: read the full door collection, replace only doorID's lock_state,
// write the full collection back. door_position is preserved unchanged.
func (c *HTTPClient) SetDoorState(doorID string, lockState LockState) error {
	return c.mutateDoor(doorID, func(d *wireDoor) {
		d.LockState = formatLockState(lockState)
	})
}

// SetDoorPosition requests position for doorID, via the same hidden
// GET-modify-PUT sequence. lock_state is preserved unchanged.
func (c *HTTPClient) SetDoorPosition(doorID string, position DoorPosition) error {
	return c.mutateDoor(doorID, func(d *wireDoor) {
		d.State = formatDoorPosition(position)
	})
}

// mutateDoor hides BuildSim's whole-collection PUT /api/doors replace
// behavior from both SetDoorState and SetDoorPosition: read the full
// collection, apply mutate to only doorID's entry, write the full
// collection back unchanged otherwise.
func (c *HTTPClient) mutateDoor(doorID string, mutate func(*wireDoor)) error {
	doors, err := c.getDoors()
	if err != nil {
		return err
	}

	found := false
	for i := range doors {
		if doors[i].ID == doorID {
			mutate(&doors[i])
			found = true
			break
		}
	}
	if !found {
		return ErrDoorNotFound{DoorID: doorID}
	}

	return httpx.PutJSON(c.baseURL+"/api/doors", doors, nil)
}

func (c *HTTPClient) getDoors() ([]wireDoor, error) {
	var doors []wireDoor
	if err := httpx.GetJSON(c.baseURL+"/api/doors", &doors); err != nil {
		return nil, err
	}
	return doors, nil
}

// wireRoomOccupancy mirrors BuildSim's RoomOccupancy JSON shape
// (pkg/model/session.go). Aliens is modeled (not just dropped) so that
// GET-modify-PUT round-trips any other room's alien entries unchanged;
// this system itself never populates aliens for the room it writes.
type wireRoomOccupancy struct {
	Persons []wirePerson `json:"persons"`
	Aliens  []wireAlien  `json:"aliens"`
}

type wirePerson struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type wireAlien struct {
	ID string `json:"id"`
}

// SetRoomOccupancy mirrors occupancy for roomKey (e.g. "level0/A109") via
// the same hidden GET-modify-PUT sequence PUT /api/occupancy requires
// (pkg/store/memory.go: SetOccupancy also replaces the whole map, not an
// upsert by key — confirmed alongside SetDoors). Only roomKey's entry is
// replaced; every other room's entry, aliens included, round-trips
// unchanged.
func (c *HTTPClient) SetRoomOccupancy(roomKey string, occupancy RoomOccupancy) error {
	rooms := make(map[string]wireRoomOccupancy)
	if err := httpx.GetJSON(c.baseURL+"/api/occupancy", &rooms); err != nil {
		return err
	}

	persons := make([]wirePerson, 0, len(occupancy.Persons))
	for _, p := range occupancy.Persons {
		persons = append(persons, wirePerson{ID: p.ID, Name: p.Name})
	}
	rooms[roomKey] = wireRoomOccupancy{Persons: persons, Aliens: []wireAlien{}}

	return httpx.PutJSON(c.baseURL+"/api/occupancy", rooms, nil)
}

func parseLockState(s string) LockState {
	switch strings.ToLower(s) {
	case "locked":
		return LockStateLocked
	case "unlocked":
		return LockStateUnlocked
	case "jammed":
		return LockStateJammed
	default:
		return LockState(strings.ToUpper(s))
	}
}

func formatLockState(s LockState) string {
	return strings.ToLower(string(s))
}

func formatDoorPosition(s DoorPosition) string {
	return strings.ToLower(string(s))
}

func parseDoorPosition(s string) DoorPosition {
	switch strings.ToLower(s) {
	case "open":
		return DoorPositionOpen
	case "closed":
		return DoorPositionClosed
	case "blocked":
		return DoorPositionBlocked
	default:
		return DoorPosition(strings.ToUpper(s))
	}
}
