package buildsim

import "sync"

// FakeClient is an in-memory Client used by the vertical-slice tests in
// place of a real BuildSim adapter (approved: "do not implement the real
// BuildSim adapter until the local BuildSim door API documentation is
// available" — a decision now resolved, but the real adapter itself is
// still out of scope for this slice).
//
// It holds a single map of doors directly, so it needs none of the real
// BuildSim GET-modify-PUT read-modify-write behavior documented in
// design-specification.md section 12.3 — that requirement is specific to
// an HTTP adapter driving BuildSim's whole-collection PUT /api/doors, not
// to this abstraction.
//
// It is shared (the same *FakeClient instance) between the Actuator and
// the Simulator in tests, so a write from one is immediately visible to a
// read from the other — exactly what "simulator observing the resulting
// door state" requires.
type FakeClient struct {
	mu    sync.Mutex
	doors map[string]DoorState
}

// NewFakeClient creates a FakeClient seeded with the given initial door
// states, keyed by door ID.
func NewFakeClient(initial map[string]DoorState) *FakeClient {
	doors := make(map[string]DoorState, len(initial))
	for id, state := range initial {
		doors[id] = state
	}
	return &FakeClient{doors: doors}
}

func (f *FakeClient) GetDoorState(doorID string) (DoorState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	state, ok := f.doors[doorID]
	if !ok {
		return DoorState{}, ErrDoorNotFound{DoorID: doorID}
	}
	return state, nil
}

func (f *FakeClient) SetDoorState(doorID string, lockState LockState) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	state, ok := f.doors[doorID]
	if !ok {
		return ErrDoorNotFound{DoorID: doorID}
	}
	state.LockState = lockState
	f.doors[doorID] = state
	return nil
}
