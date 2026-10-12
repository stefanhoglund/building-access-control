package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/actuator"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/buildsim"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/cardreader"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/config"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/contracts"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/decision"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/doorstatesensor"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/eventdata"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/evidence"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/modeltime"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/occupancysensor"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/simulator"
)

const (
	fromRoom = "level0/1540"
	toRoom   = "level0/A109"
)

// TestSuccessfulPassage exercises REQ-FUNC-004, REQ-SIM-005, and
// REQ-SIM-006 end-to-end:
//
//	ALLOW -> UNLOCK -> OPEN -> occupant crosses -> source occupancy
//	decreases -> destination occupancy increases -> occupancy sensor
//	observes the new state -> CLOSE -> LOCK
//
// This is a distinct harness from startSlice (test/e2e/harness_test.go)
// because it enables CrossingConfig, which startSlice deliberately leaves
// disabled so TestAllow/TestDeny/TestRelockTimeout keep their original,
// passage-independent behavior.
func TestSuccessfulPassage(t *testing.T) {
	modelStart, err := modeltime.Parse("2026-10-08T08:00:00Z")
	if err != nil {
		t.Fatalf("parse fixed model start time: %v", err)
	}
	clock := modeltime.NewClock(modelStart)

	bs := buildsim.NewFakeClient(map[string]buildsim.DoorState{
		doorID: {LockState: buildsim.LockStateLocked, DoorPosition: buildsim.DoorPositionClosed},
	})

	cfg, err := config.Load(filepath.Join("..", "..", "config", "system-config.json"))
	if err != nil {
		t.Fatalf("load system configuration: %v", err)
	}

	evidencePath := filepath.Join(t.TempDir(), "evidence.jsonl")
	store, err := evidence.Open(evidencePath)
	if err != nil {
		t.Fatalf("open evidence store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	simulatorLn := listen(t)
	eventDataLn := listen(t)
	decisionLn := listen(t)
	actuatorLn := listen(t)

	simulatorURL := "http://" + simulatorLn.Addr().String()
	eventDataURL := "http://" + eventDataLn.Addr().String()
	decisionURL := "http://" + decisionLn.Addr().String()
	actuatorURL := "http://" + actuatorLn.Addr().String()

	crossing := simulator.CrossingConfig{
		DoorID:   doorID,
		FromRoom: fromRoom,
		ToRoom:   toRoom,
	}
	initialOccupancy := map[string]int{fromRoom: 1, toRoom: 0}

	simServer := simulator.New(clock, bs, crossing, initialOccupancy)
	simServer.MirrorInitialOccupancy()
	eventDataServer := eventdata.New(store, decisionURL)
	decisionServer := decision.New(cfg, eventDataURL, actuatorURL, simulatorURL)
	actuatorServer := actuator.New(bs, eventDataURL)

	serve(t, simulatorLn, simServer.Handler())
	serve(t, eventDataLn, eventDataServer.Handler())
	serve(t, decisionLn, decisionServer.Handler())
	serve(t, actuatorLn, actuatorServer.Handler())

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go decisionServer.RunRelockTicker(ctx, pollInterval)

	reader := &cardreader.Reader{
		ReaderID:         readerID,
		SimulatorBaseURL: simulatorURL,
		EventDataBaseURL: eventDataURL,
		PollInterval:     pollInterval,
	}
	go reader.Run(ctx)

	occSensor := &occupancysensor.Sensor{
		SourceID:         "occupancysensor-1",
		RoomKey:          toRoom,
		SimulatorBaseURL: simulatorURL,
		EventDataBaseURL: eventDataURL,
		PollInterval:     pollInterval,
	}
	go occSensor.Run(ctx)

	// Polls comfortably faster than the Simulator's passageDwell (150ms;
	// see internal/simulator/server.go) so the OPEN state is never missed
	// between ticks.
	doorSensor := &doorstatesensor.Sensor{
		SourceID:         "doorstatesensor-1",
		DoorID:           doorID,
		BuildSim:         bs,
		SimulatorBaseURL: simulatorURL,
		EventDataBaseURL: eventDataURL,
		PollInterval:     pollInterval,
	}
	go doorSensor.Run(ctx)

	// Initial occupancy, before anything happens: source=1, destination=0.
	if got := queryOccupancy(t, simulatorURL, fromRoom); got != 1 {
		t.Fatalf("initial source occupancy = %d, want 1", got)
	}
	if got := queryOccupancy(t, simulatorURL, toRoom); got != 0 {
		t.Fatalf("initial destination occupancy = %d, want 0", got)
	}

	// Drive the scenario: present card-allow.
	reqBody := map[string]string{
		"reader_id": readerID,
		"card_id":   cardAllow,
		"door_id":   doorID,
		"direction": direction,
	}
	postJSONExpect(t, simulatorURL+"/scenario/present", reqBody, http.StatusAccepted)

	// ALLOW -> UNLOCK.
	var decisionRecord contracts.AccessDecision
	eventually(t, 2*time.Second, func() bool {
		_, decisionRecord, _, _ = findAllowChain(t, readEvidenceRecords(t, evidencePath))
		return decisionRecord.DecisionID != ""
	})
	if decisionRecord.FinalAction != contracts.FinalActionAllow {
		t.Fatalf("final_action = %q, want ALLOW", decisionRecord.FinalAction)
	}

	// OPEN, then CLOSE — both are evidence (DoorStateObservation),
	// produced by the independent DoorStateSensor polling BuildSim, not
	// by the Simulator's own account of what it caused
	// (design-specification.md section 4.1).
	var opened, closed contracts.DoorStateObservation
	eventually(t, 2*time.Second, func() bool {
		opened, closed = findDoorStateObservations(readEvidenceRecords(t, evidencePath), doorID)
		return opened.DoorPosition != "" && closed.DoorPosition != ""
	})
	if opened.SourceID != "doorstatesensor-1" || closed.SourceID != "doorstatesensor-1" {
		t.Fatalf("door state observations not from the DoorStateSensor: opened=%+v closed=%+v", opened, closed)
	}

	// Occupant crosses: source occupancy decreases, destination occupancy
	// increases (REQ-SIM-005).
	eventually(t, 2*time.Second, func() bool {
		return queryOccupancy(t, simulatorURL, fromRoom) == 0
	})
	eventually(t, 2*time.Second, func() bool {
		return queryOccupancy(t, simulatorURL, toRoom) == 1
	})

	// The occupancy sensor observes the new state.
	var occObservation contracts.OccupancyObservation
	eventually(t, 2*time.Second, func() bool {
		occObservation = findOccupancyObservation(readEvidenceRecords(t, evidencePath), toRoom, 1)
		return occObservation.EventID != ""
	})
	if occObservation.RoomKey != toRoom {
		t.Fatalf("occupancy observation room_key = %q, want %q", occObservation.RoomKey, toRoom)
	}
	if occObservation.Count != 1 {
		t.Fatalf("occupancy observation count = %d, want 1", occObservation.Count)
	}
	if occObservation.Quality != contracts.ObservationQualityOK {
		t.Fatalf("occupancy observation quality = %q, want %q", occObservation.Quality, contracts.ObservationQualityOK)
	}

	// CLOSE already confirmed above triggers LOCK (relock after passage,
	// REQ-FUNC-004) — distinct from the no-open timeout path.
	eventually(t, 2*time.Second, func() bool {
		return simDoorLockState(t, simulatorURL) == string(buildsim.LockStateLocked)
	})

	var lockCommand contracts.ActuatorCommand
	var lockOutcome contracts.ActuatorOutcome
	eventually(t, 2*time.Second, func() bool {
		lockCommand, lockOutcome = findPassageRelockChain(readEvidenceRecords(t, evidencePath), decisionRecord.DecisionID)
		return lockOutcome.CommandID != ""
	})
	if lockCommand.Reason != contracts.ReasonPassageComplete {
		t.Fatalf("relock command.reason = %q, want %q (not a timeout relock)", lockCommand.Reason, contracts.ReasonPassageComplete)
	}
	if lockOutcome.Result != contracts.OutcomeConfirmed {
		t.Fatalf("relock outcome.result = %q, want CONFIRMED", lockOutcome.Result)
	}

	// No DOOR_NOT_OPENED event should exist for this decision: the door
	// was opened, so the no-open timeout never applied.
	for _, r := range readEvidenceRecords(t, evidencePath) {
		if r.RecordType != contracts.RecordTypeDoorEvent {
			continue
		}
		var evt contracts.DoorEvent
		mustUnmarshal(t, r.Payload, &evt)
		if evt.EventType == contracts.DoorEventNotOpened && evt.DecisionID == decisionRecord.DecisionID {
			t.Fatalf("unexpected DOOR_NOT_OPENED event for a door that was opened: %+v", evt)
		}
	}
}

func queryOccupancy(t *testing.T, simulatorURL, room string) int {
	t.Helper()
	var resp struct {
		Count int `json:"count"`
	}
	getJSONExpect(t, simulatorURL+"/occupancy?room="+room, &resp)
	return resp.Count
}

func simDoorLockState(t *testing.T, simulatorURL string) string {
	t.Helper()
	var resp struct {
		LockState string `json:"lock_state"`
	}
	getJSONExpect(t, simulatorURL+"/doors/"+doorID+"/state", &resp)
	return resp.LockState
}

func readEvidenceRecords(t *testing.T, path string) []evidence.Record {
	t.Helper()
	records, err := evidence.ReadAll(path)
	if err != nil {
		t.Fatalf("read evidence: %v", err)
	}
	return records
}

// findDoorStateObservations returns the passage's OPEN and CLOSED
// DoorStateObservation for doorID. The sensor's very first poll always
// reports the door's steady-state CLOSED, before any passage has
// happened — so "closed" is deliberately required to be a CLOSED record
// that appears *after* "opened" in the (append-ordered) evidence log;
// the pre-passage steady-state CLOSED is not mistaken for passage
// completion.
func findDoorStateObservations(records []evidence.Record, doorID string) (opened, closed contracts.DoorStateObservation) {
	for _, r := range records {
		if r.RecordType != contracts.RecordTypeDoorStateObservation {
			continue
		}
		var obs contracts.DoorStateObservation
		if err := json.Unmarshal(r.Payload, &obs); err != nil {
			continue
		}
		if obs.DoorID != doorID {
			continue
		}
		switch obs.DoorPosition {
		case contracts.DoorPositionOpen:
			if opened.DoorPosition == "" {
				opened = obs
			}
		case contracts.DoorPositionClosed:
			if opened.DoorPosition != "" {
				closed = obs // only once an OPEN has already been seen
			}
		}
	}
	return opened, closed
}

func findOccupancyObservation(records []evidence.Record, roomKey string, count int) contracts.OccupancyObservation {
	for _, r := range records {
		if r.RecordType != contracts.RecordTypeOccupancyObservation {
			continue
		}
		var obs contracts.OccupancyObservation
		if err := json.Unmarshal(r.Payload, &obs); err != nil {
			continue
		}
		if obs.RoomKey == roomKey && obs.Count == count {
			return obs
		}
	}
	return contracts.OccupancyObservation{}
}

func findPassageRelockChain(records []evidence.Record, decisionID string) (contracts.ActuatorCommand, contracts.ActuatorOutcome) {
	var cmd contracts.ActuatorCommand
	var out contracts.ActuatorOutcome
	for _, r := range records {
		switch r.RecordType {
		case contracts.RecordTypeCommand:
			var v contracts.ActuatorCommand
			if err := json.Unmarshal(r.Payload, &v); err != nil {
				continue
			}
			if v.DecisionID == decisionID && v.Action == contracts.ActionLock {
				cmd = v
			}
		case contracts.RecordTypeOutcome:
			var v contracts.ActuatorOutcome
			if err := json.Unmarshal(r.Payload, &v); err != nil {
				continue
			}
			if cmd.CommandID != "" && v.CommandID == cmd.CommandID {
				out = v
			}
		}
	}
	return cmd, out
}
