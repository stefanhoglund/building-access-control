package e2e

import (
	"bytes"
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

// scenarioSlice is startScenarioSlice's handle: the same five wired-up
// components as startSlice/TestSuccessfulPassage, but with the configured
// Population and named Scenarios (config/population.json,
// config/scenarios.json) attached, so a test can drive the system by
// invoking a named scenario exactly as scripts/run-local.sh or an
// operator would (POST /scenario/run), rather than reconstructing its
// steps by hand.
type scenarioSlice struct {
	t *testing.T

	simulatorURL string
	evidencePath string
}

func startScenarioSlice(t *testing.T) *scenarioSlice {
	t.Helper()

	// Overwritten by Clock.Set on the first RunScenario call — a named
	// scenario always defines its own initial model time
	// (design-specification.md section 3.5).
	clock := modeltime.NewClock(time.Now().UTC())

	bs := buildsim.NewFakeClient(map[string]buildsim.DoorState{
		doorID: {LockState: buildsim.LockStateLocked, DoorPosition: buildsim.DoorPositionClosed},
	})

	cfg, err := config.Load(filepath.Join("..", "..", "config", "system-config.json"))
	if err != nil {
		t.Fatalf("load system configuration: %v", err)
	}
	pop, err := simulator.LoadPopulation(filepath.Join("..", "..", "config", "population.json"))
	if err != nil {
		t.Fatalf("load population: %v", err)
	}
	scenarios, err := simulator.LoadScenarios(filepath.Join("..", "..", "config", "scenarios.json"))
	if err != nil {
		t.Fatalf("load scenarios: %v", err)
	}

	evidencePath := filepath.Join(t.TempDir(), "evidence.jsonl")
	store, err := evidence.Open(evidencePath)
	if err != nil {
		t.Fatalf("open evidence store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	runLog, err := evidence.Open(filepath.Join(t.TempDir(), "scenario-runs.jsonl"))
	if err != nil {
		t.Fatalf("open scenario run log: %v", err)
	}
	t.Cleanup(func() { runLog.Close() })

	simulatorLn := listen(t)
	eventDataLn := listen(t)
	decisionLn := listen(t)
	actuatorLn := listen(t)

	simulatorURL := "http://" + simulatorLn.Addr().String()
	eventDataURL := "http://" + eventDataLn.Addr().String()
	decisionURL := "http://" + decisionLn.Addr().String()
	actuatorURL := "http://" + actuatorLn.Addr().String()

	crossing := simulator.CrossingConfig{DoorID: doorID, FromRoom: fromRoom, ToRoom: toRoom}
	initialOccupancy := map[string]int{fromRoom: len(pop.People), toRoom: 0}

	simServer := simulator.New(clock, bs, crossing, initialOccupancy)
	simServer.SetPopulation(pop)
	simServer.SetScenarios(scenarios)
	simServer.SetScenarioRunLog(runLog)
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

	doorSensor := &doorstatesensor.Sensor{
		SourceID:         "doorstatesensor-1",
		DoorID:           doorID,
		BuildSim:         bs,
		SimulatorBaseURL: simulatorURL,
		EventDataBaseURL: eventDataURL,
		PollInterval:     pollInterval,
	}
	go doorSensor.Run(ctx)

	return &scenarioSlice{t: t, simulatorURL: simulatorURL, evidencePath: evidencePath}
}

// runScenario invokes POST /scenario/run, the same production interface
// an operator or scripts/run-local.sh uses, and blocks until the
// Simulator confirms every presentation the scenario specifies has been
// enqueued.
func (s *scenarioSlice) runScenario(name string, seed int64, count, intervalMillis int) contracts.ScenarioRunResult {
	s.t.Helper()
	payload, err := json.Marshal(contracts.ScenarioRunRequest{
		Scenario:       name,
		Seed:           seed,
		Count:          count,
		IntervalMillis: intervalMillis,
	})
	if err != nil {
		s.t.Fatalf("marshal scenario run request: %v", err)
	}
	resp, err := http.Post(s.simulatorURL+"/scenario/run", "application/json", bytes.NewReader(payload))
	if err != nil {
		s.t.Fatalf("POST /scenario/run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		s.t.Fatalf("POST /scenario/run %q: status = %d, want 200", name, resp.StatusCode)
	}
	var result contracts.ScenarioRunResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		s.t.Fatalf("decode scenario run result: %v", err)
	}
	return result
}

func findObservationsForCard(t *testing.T, records []evidence.Record, cardID string) []contracts.AccessAttemptObservation {
	t.Helper()
	var matches []contracts.AccessAttemptObservation
	for _, r := range records {
		if r.RecordType != contracts.RecordTypeObservation {
			continue
		}
		var v contracts.AccessAttemptObservation
		mustUnmarshal(t, r.Payload, &v)
		if v.CardID == cardID {
			matches = append(matches, v)
		}
	}
	return matches
}

func findDecisionForEventID(t *testing.T, records []evidence.Record, eventID string) contracts.AccessDecision {
	t.Helper()
	for _, r := range records {
		if r.RecordType != contracts.RecordTypeDecision {
			continue
		}
		var v contracts.AccessDecision
		mustUnmarshal(t, r.Payload, &v)
		if v.EventID == eventID {
			return v
		}
	}
	return contracts.AccessDecision{}
}

// TestScenarioAuthorizedPassageCrosses covers Phase 1's "normal authorized
// user crosses" requirement using the named "authorized-passage" scenario
// (config/scenarios.json), invoked exactly as an operator would via
// POST /scenario/run.
func TestScenarioAuthorizedPassageCrosses(t *testing.T) {
	slice := startScenarioSlice(t)

	result := slice.runScenario("authorized-passage", 1, 0, 0)
	if result.StepsExecuted != 1 {
		t.Fatalf("steps_executed = %d, want 1", result.StepsExecuted)
	}
	if len(result.CardIDs) != 1 || result.CardIDs[0] != "card-alice" {
		t.Fatalf("card_ids = %v, want [card-alice]", result.CardIDs)
	}

	var decisionRecord contracts.AccessDecision
	eventually(t, 2*time.Second, func() bool {
		obs := findObservationsForCard(t, readEvidenceRecords(t, slice.evidencePath), "card-alice")
		if len(obs) == 0 {
			return false
		}
		decisionRecord = findDecisionForEventID(t, readEvidenceRecords(t, slice.evidencePath), obs[0].EventID)
		return decisionRecord.DecisionID != ""
	})
	if decisionRecord.AuthorizationResult != contracts.AuthResultAllow {
		t.Fatalf("authorization_result = %q, want ALLOW", decisionRecord.AuthorizationResult)
	}
	if decisionRecord.FinalAction != contracts.FinalActionAllow {
		t.Fatalf("final_action = %q, want ALLOW", decisionRecord.FinalAction)
	}

	// REQ-SIM-005: the authorized user actually crosses.
	eventually(t, 2*time.Second, func() bool {
		return queryOccupancy(t, slice.simulatorURL, toRoom) >= 1
	})
}

// TestScenarioUnauthorizedAccessDenied covers "denied user does not
// cross" using the named "unauthorized-access" scenario: erin (visitor
// role, no permission on door-1) attempts entry.
func TestScenarioUnauthorizedAccessDenied(t *testing.T) {
	slice := startScenarioSlice(t)

	result := slice.runScenario("unauthorized-access", 1, 0, 0)
	if len(result.CardIDs) != 1 || result.CardIDs[0] != "card-erin" {
		t.Fatalf("card_ids = %v, want [card-erin]", result.CardIDs)
	}

	var decisionRecord contracts.AccessDecision
	eventually(t, 2*time.Second, func() bool {
		obs := findObservationsForCard(t, readEvidenceRecords(t, slice.evidencePath), "card-erin")
		if len(obs) == 0 {
			return false
		}
		decisionRecord = findDecisionForEventID(t, readEvidenceRecords(t, slice.evidencePath), obs[0].EventID)
		return decisionRecord.DecisionID != ""
	})
	if decisionRecord.AuthorizationResult != contracts.AuthResultDeny {
		t.Fatalf("authorization_result = %q, want DENY", decisionRecord.AuthorizationResult)
	}
	if decisionRecord.FinalAction != contracts.FinalActionDeny {
		t.Fatalf("final_action = %q, want DENY", decisionRecord.FinalAction)
	}

	// REQ-SIM-004/005: a denied user never crosses, and the door never
	// unlocks. Checked immediately: nothing in a DENY path ever mutates
	// occupancy or door state (watchForUnlockAndCross only proceeds past
	// an UNLOCKED observation, which a DENY never produces), so there is
	// no race to poll for.
	if got := queryOccupancy(t, slice.simulatorURL, toRoom); got != 0 {
		t.Fatalf("destination occupancy = %d, want 0 (denied user must not cross)", got)
	}
	if got := simDoorLockState(t, slice.simulatorURL); got != string(buildsim.LockStateLocked) {
		t.Fatalf("door lock_state = %q, want LOCKED", got)
	}
}

// TestScenarioAfterHoursAccessAnomalous covers "after-hours authorized
// event remains authorized but is classified anomalous": alice (normally
// authorized) presents at 03:00 model time, outside the anomaly module's
// normal-hour window (internal/decision/anomaly.go).
func TestScenarioAfterHoursAccessAnomalous(t *testing.T) {
	slice := startScenarioSlice(t)

	slice.runScenario("after-hours-access", 1, 0, 0)

	var decisionRecord contracts.AccessDecision
	eventually(t, 2*time.Second, func() bool {
		obs := findObservationsForCard(t, readEvidenceRecords(t, slice.evidencePath), "card-alice")
		if len(obs) == 0 {
			return false
		}
		decisionRecord = findDecisionForEventID(t, readEvidenceRecords(t, slice.evidencePath), obs[0].EventID)
		return decisionRecord.DecisionID != ""
	})

	// Authorization is unaffected by anomaly detection (REQ-DEC-002/003).
	if decisionRecord.AuthorizationResult != contracts.AuthResultAllow {
		t.Fatalf("authorization_result = %q, want ALLOW", decisionRecord.AuthorizationResult)
	}
	if decisionRecord.FinalAction != contracts.FinalActionAllow {
		t.Fatalf("final_action = %q, want ALLOW", decisionRecord.FinalAction)
	}
	if decisionRecord.AnomalyResult != contracts.AnomalyAnomalous {
		t.Fatalf("anomaly_result = %q, want ANOMALOUS", decisionRecord.AnomalyResult)
	}
	if !decisionRecord.Alert {
		t.Fatalf("alert = false, want true for an anomalous access")
	}
}

// TestScenarioRapidRepeatAccessAnomalous covers "rapid-repeat scenario
// produces the expected anomaly behavior": alice presents three times,
// 1 model-second apart, well inside the anomaly module's
// repeatedAttemptWindow (5s). The first presentation has no prior
// attempt on record and stays NORMAL; the second and third are each
// within the window of their immediately preceding attempt and must be
// classified ANOMALOUS.
func TestScenarioRapidRepeatAccessAnomalous(t *testing.T) {
	slice := startScenarioSlice(t)

	result := slice.runScenario("rapid-repeat-access", 1, 0, 0)
	if result.StepsExecuted != 3 {
		t.Fatalf("steps_executed = %d, want 3", result.StepsExecuted)
	}

	var observations []contracts.AccessAttemptObservation
	eventually(t, 2*time.Second, func() bool {
		observations = findObservationsForCard(t, readEvidenceRecords(t, slice.evidencePath), "card-alice")
		return len(observations) == 3
	})

	records := readEvidenceRecords(t, slice.evidencePath)
	decisions := make([]contracts.AccessDecision, 3)
	for i, obs := range observations {
		decisions[i] = findDecisionForEventID(t, records, obs.EventID)
		if decisions[i].DecisionID == "" {
			t.Fatalf("no decision found for presentation %d (event_id %s)", i, obs.EventID)
		}
	}

	if decisions[0].AnomalyResult != contracts.AnomalyNormal {
		t.Fatalf("decision[0].anomaly_result = %q, want NORMAL (first attempt, no prior)", decisions[0].AnomalyResult)
	}
	for i := 1; i < 3; i++ {
		if decisions[i].AnomalyResult != contracts.AnomalyAnomalous {
			t.Fatalf("decision[%d].anomaly_result = %q, want ANOMALOUS (repeated attempt)", i, decisions[i].AnomalyResult)
		}
		if decisions[i].AnomalyReason != "REPEATED_ATTEMPT" {
			t.Fatalf("decision[%d].anomaly_reason = %q, want REPEATED_ATTEMPT", i, decisions[i].AnomalyReason)
		}
	}
	// Authorization/final_action are unaffected throughout.
	for i, d := range decisions {
		if d.AuthorizationResult != contracts.AuthResultAllow || d.FinalAction != contracts.FinalActionAllow {
			t.Fatalf("decision[%d]: authorization_result=%q final_action=%q, want ALLOW/ALLOW", i, d.AuthorizationResult, d.FinalAction)
		}
	}
}

// TestScenarioBurstEmitsRequestedCount covers "burst scenario emits the
// requested number of events": a small, fast override of
// "burst-access"'s count/interval_ms keeps this test quick while still
// exercising the real burst code path (internal/simulator/scenarios.go).
func TestScenarioBurstEmitsRequestedCount(t *testing.T) {
	slice := startScenarioSlice(t)

	const want = 15
	result := slice.runScenario("burst-access", 1, want, 5)
	if result.StepsExecuted != want {
		t.Fatalf("steps_executed = %d, want %d", result.StepsExecuted, want)
	}

	eventually(t, 5*time.Second, func() bool {
		obs := findObservationsForCard(t, readEvidenceRecords(t, slice.evidencePath), "card-alice")
		return len(obs) == want
	})
}
