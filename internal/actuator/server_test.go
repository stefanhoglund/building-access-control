package actuator

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/buildsim"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/contracts"
)

// outcomeCounter is a minimal stand-in for the Event/Data Service's
// POST /outcomes endpoint. It counts how many times an outcome is posted
// — the observable side effect that proves (or disproves) REQ-DIST-007
// deduplication — and captures the most recent outcome body, so a test
// can also inspect *what* was reported (REQ-RES-002: was it honestly
// marked unverified, not falsely confirmed).
type outcomeCounter struct {
	mu    sync.Mutex
	count int
	last  contracts.ActuatorOutcome
}

func (o *outcomeCounter) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var outcome contracts.ActuatorOutcome
		_ = json.NewDecoder(r.Body).Decode(&outcome)
		o.mu.Lock()
		o.count++
		o.last = outcome
		o.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	})
}

func (o *outcomeCounter) value() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.count
}

func (o *outcomeCounter) lastOutcome() contracts.ActuatorOutcome {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.last
}

// postCommand POSTs cmd through the handler directly (no real network),
// synchronously: handleCommand posts the outcome to eventData, if any,
// before it writes its own response, so by the time this returns, that
// side effect has already happened or not.
func postCommand(t *testing.T, handler http.Handler, cmd contracts.ActuatorCommand) *http.Response {
	t.Helper()
	body, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal command: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/commands", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec.Result()
}

func TestHandleCommand_Unlock_ConfirmedAndPostsOneOutcome(t *testing.T) {
	bs := buildsim.NewFakeClient(map[string]buildsim.DoorState{
		"door-1": {LockState: buildsim.LockStateLocked, DoorPosition: buildsim.DoorPositionClosed},
	})
	outcomes := &outcomeCounter{}
	eventData := httptest.NewServer(outcomes.handler())
	defer eventData.Close()

	srv := New(bs, eventData.URL)

	resp := postCommand(t, srv.Handler(), contracts.ActuatorCommand{
		CommandID: "cmd-1",
		DoorID:    "door-1",
		Action:    contracts.ActionUnlock,
		Reason:    contracts.ReasonAllowUnlock,
		ModelTime: "2026-10-08T08:00:00Z",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	state, err := bs.GetDoorState("door-1")
	if err != nil {
		t.Fatalf("get door state: %v", err)
	}
	if state.LockState != buildsim.LockStateUnlocked {
		t.Fatalf("lock_state = %q, want UNLOCKED", state.LockState)
	}
	if got := outcomes.value(); got != 1 {
		t.Fatalf("outcomes posted = %d, want 1", got)
	}
}

func TestHandleCommand_DuplicateCommandID_NoSecondPhysicalEffect(t *testing.T) {
	bs := buildsim.NewFakeClient(map[string]buildsim.DoorState{
		"door-1": {LockState: buildsim.LockStateLocked, DoorPosition: buildsim.DoorPositionClosed},
	})
	outcomes := &outcomeCounter{}
	eventData := httptest.NewServer(outcomes.handler())
	defer eventData.Close()

	srv := New(bs, eventData.URL)

	first := contracts.ActuatorCommand{
		CommandID: "cmd-dup",
		DoorID:    "door-1",
		Action:    contracts.ActionUnlock,
		Reason:    contracts.ReasonAllowUnlock,
		ModelTime: "2026-10-08T08:00:00Z",
	}
	postCommand(t, srv.Handler(), first)
	if got := outcomes.value(); got != 1 {
		t.Fatalf("outcomes posted after first delivery = %d, want 1", got)
	}

	// Deliberately redeliver the SAME command_id with a DIFFERENT action
	// (LOCK instead of UNLOCK). If deduplication only compared actions,
	// or applied the action again regardless of command_id, this would
	// relock the door. REQ-DIST-007 requires it to do nothing at all.
	redelivered := first
	redelivered.Action = contracts.ActionLock

	resp := postCommand(t, srv.Handler(), redelivered)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("redelivered command status = %d, want 200 (acknowledged, not reapplied)", resp.StatusCode)
	}

	state, err := bs.GetDoorState("door-1")
	if err != nil {
		t.Fatalf("get door state: %v", err)
	}
	if state.LockState != buildsim.LockStateUnlocked {
		t.Fatalf("lock_state after redelivered duplicate = %q, want still UNLOCKED (REQ-DIST-007: no second physical effect)", state.LockState)
	}
	if got := outcomes.value(); got != 1 {
		t.Fatalf("outcomes posted = %d, want 1 (the duplicate must not produce a second outcome either)", got)
	}
}

func TestHandleCommand_UnknownAction_Rejected(t *testing.T) {
	bs := buildsim.NewFakeClient(map[string]buildsim.DoorState{
		"door-1": {LockState: buildsim.LockStateLocked, DoorPosition: buildsim.DoorPositionClosed},
	})
	outcomes := &outcomeCounter{}
	eventData := httptest.NewServer(outcomes.handler())
	defer eventData.Close()

	srv := New(bs, eventData.URL)

	resp := postCommand(t, srv.Handler(), contracts.ActuatorCommand{
		CommandID: "cmd-bad",
		DoorID:    "door-1",
		Action:    "BOGUS",
		ModelTime: "2026-10-08T08:00:00Z",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an unknown action", resp.StatusCode)
	}

	state, err := bs.GetDoorState("door-1")
	if err != nil {
		t.Fatalf("get door state: %v", err)
	}
	if state.LockState != buildsim.LockStateLocked {
		t.Fatalf("lock_state = %q, want unchanged LOCKED: an unknown action must not reach BuildSim", state.LockState)
	}
}

// failingBuildSimClient wraps a real FakeClient but forces SetDoorState to
// fail, standing in for "BuildSim unavailable or timed out" without
// depending on a real BuildSim instance being unavailable (fault
// experiment, REQ-RES-002). GetDoorState/SetDoorPosition/SetRoomOccupancy
// are delegated unchanged.
type failingBuildSimClient struct {
	*buildsim.FakeClient
}

func (f *failingBuildSimClient) SetDoorState(doorID string, lockState buildsim.LockState) error {
	return errors.New("simulated: BuildSim unavailable")
}

// TestHandleCommand_BuildSimUnavailable_RecordsUnverifiedNotFalseSuccess is
// the fault experiment required alongside the Decision Service restart
// experiment: ALLOW -> actuator/BuildSim unavailable -> physical state
// cannot be confirmed -> no false success -> result recorded as
// failed/unverified (REQ-RES-002).
func TestHandleCommand_BuildSimUnavailable_RecordsUnverifiedNotFalseSuccess(t *testing.T) {
	bs := &failingBuildSimClient{FakeClient: buildsim.NewFakeClient(map[string]buildsim.DoorState{
		"door-1": {LockState: buildsim.LockStateLocked, DoorPosition: buildsim.DoorPositionClosed},
	})}
	outcomes := &outcomeCounter{}
	eventData := httptest.NewServer(outcomes.handler())
	defer eventData.Close()

	srv := New(bs, eventData.URL)

	// This is the ALLOW -> UNLOCK command the Decision Service would
	// issue after a valid ALLOW decision; BuildSim is unreachable when
	// the Actuator tries to apply it.
	resp := postCommand(t, srv.Handler(), contracts.ActuatorCommand{
		CommandID: "cmd-buildsim-down",
		DoorID:    "door-1",
		Action:    contracts.ActionUnlock,
		Reason:    contracts.ReasonAllowUnlock,
		ModelTime: "2026-10-08T08:00:00Z",
	})

	// The command itself is still accepted: whether BuildSim could be
	// reached is an outcome to record, not a reason to reject the
	// command (design-specification.md section 4.2/9.5 — commanded state
	// and observed state are separate concepts).
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (command accepted; failure is reported as an outcome, not an HTTP error)", resp.StatusCode)
	}

	outcome := outcomes.lastOutcome()
	if outcome.Result != contracts.OutcomeUnverified {
		t.Fatalf("outcome.result = %q, want UNVERIFIED (REQ-RES-002)", outcome.Result)
	}
	if outcome.ObservedState != "" {
		t.Fatalf("outcome.observed_state = %q, want empty: no false success may be claimed when BuildSim could not be reached", outcome.ObservedState)
	}
	if outcome.CommandID != "cmd-buildsim-down" {
		t.Fatalf("outcome.command_id = %q, want traceable back to the issuing command", outcome.CommandID)
	}

	// No false success in BuildSim either: the underlying (fake) BuildSim
	// state was never actually mutated, since SetDoorState itself failed
	// before any write could happen.
	state, err := bs.GetDoorState("door-1")
	if err != nil {
		t.Fatalf("get door state: %v", err)
	}
	if state.LockState != buildsim.LockStateLocked {
		t.Fatalf("BuildSim lock_state = %q, want still LOCKED: a failed SetDoorState must not be reported as if it succeeded", state.LockState)
	}
}
