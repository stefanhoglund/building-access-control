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
	doors, err := c.getDoors()
	if err != nil {
		return err
	}

	found := false
	for i := range doors {
		if doors[i].ID == doorID {
			doors[i].LockState = formatLockState(lockState)
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
