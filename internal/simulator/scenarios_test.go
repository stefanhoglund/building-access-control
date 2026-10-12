package simulator

import (
	"reflect"
	"testing"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/buildsim"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/modeltime"
)

func testPopulation() *Population {
	return &Population{People: []Person{
		{PersonID: "alice", CardID: "card-alice", NormalHourStart: 8, NormalHourEnd: 17},
		{PersonID: "bob", CardID: "card-bob", NormalHourStart: 7, NormalHourEnd: 16},
		{PersonID: "carol", CardID: "card-carol", NormalHourStart: 6, NormalHourEnd: 14},
		{PersonID: "dave", CardID: "card-dave", NormalHourStart: 8, NormalHourEnd: 18},
		{PersonID: "erin", CardID: "card-erin", NormalHourStart: 10, NormalHourEnd: 16},
		{PersonID: "frank", CardID: "card-frank", NormalHourStart: 10, NormalHourEnd: 16},
		{PersonID: "grace", CardID: "card-grace", NormalHourStart: 9, NormalHourEnd: 20},
		{PersonID: "henry", CardID: "card-henry", NormalHourStart: 8, NormalHourEnd: 17},
	}}
}

func testScenarios() *ScenarioSet {
	return &ScenarioSet{Scenarios: map[string]ScenarioDef{
		"normal-day": {
			ModelStart:     "2026-10-08T06:00:00Z",
			ReaderID:       "reader-1",
			DoorID:         "door-1",
			Direction:      "PUBLIC_TO_EMPLOYEE",
			Kind:           "population_sample",
			SampleSize:     5,
			SpacingSeconds: 900,
			JitterSeconds:  180,
		},
	}}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	start, err := modeltime.Parse("2026-10-08T00:00:00Z")
	if err != nil {
		t.Fatalf("parse model time: %v", err)
	}
	clock := modeltime.NewClock(start)
	bs := buildsim.NewFakeClient(map[string]buildsim.DoorState{
		"door-1": {LockState: buildsim.LockStateLocked, DoorPosition: buildsim.DoorPositionClosed},
	})
	// Crossing is disabled (zero-value CrossingConfig): this test only
	// cares about RunScenario's enqueue ordering/timing, not the
	// downstream unlock-and-cross behavior already covered by
	// test/e2e/scenario_test.go.
	srv := New(clock, bs, CrossingConfig{}, nil)
	srv.SetPopulation(testPopulation())
	srv.SetScenarios(testScenarios())
	return srv
}

// TestRunScenarioSeedReproducibility covers Phase 1's "fixed seed
// produces reproducible event ordering/timing" requirement
// (REQ-SIM-001/003): the same seed against the same population/scenario
// must select the same people in the same order every time, and a
// different seed must (for this population/scenario) select a
// different order.
func TestRunScenarioSeedReproducibility(t *testing.T) {
	srv1 := newTestServer(t)
	result1, err := srv1.RunScenario("normal-day", 42, 0, 0)
	if err != nil {
		t.Fatalf("RunScenario(seed=42) run 1: %v", err)
	}

	srv2 := newTestServer(t)
	result2, err := srv2.RunScenario("normal-day", 42, 0, 0)
	if err != nil {
		t.Fatalf("RunScenario(seed=42) run 2: %v", err)
	}

	if !reflect.DeepEqual(result1.CardIDs, result2.CardIDs) {
		t.Fatalf("same seed produced different orderings: %v vs %v", result1.CardIDs, result2.CardIDs)
	}
	if result1.StepsExecuted != 5 {
		t.Fatalf("steps_executed = %d, want 5 (sample_size)", result1.StepsExecuted)
	}

	srv3 := newTestServer(t)
	result3, err := srv3.RunScenario("normal-day", 7, 0, 0)
	if err != nil {
		t.Fatalf("RunScenario(seed=7): %v", err)
	}
	if reflect.DeepEqual(result1.CardIDs, result3.CardIDs) {
		t.Fatalf("different seeds (42, 7) produced the same ordering: %v", result1.CardIDs)
	}
}
