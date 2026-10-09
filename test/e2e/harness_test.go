// Package e2e contains the vertical-slice acceptance tests. It wires the
// five components (Simulator, Card Reader, Event & Data Service,
// Autonomous Decision Service, Door Actuator) together over real loopback
// HTTP, sharing one buildsim.FakeClient between the Actuator and the
// Simulator so a write from one is immediately visible to a read from the
// other — the fake stands in for the still-unimplemented real BuildSim
// adapter (design-specification.md section 12.3).
package e2e

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/actuator"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/buildsim"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/cardreader"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/config"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/decision"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/eventdata"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/evidence"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/modeltime"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/simulator"
)

const (
	doorID       = "door-1"
	readerID     = "reader-1"
	direction    = "PUBLIC_TO_EMPLOYEE"
	cardAllow    = "card-allow"
	cardDeny     = "card-deny"
	pollInterval = 20 * time.Millisecond
)

// slice holds everything an acceptance test needs to drive a scenario and
// assert on its outcome.
type slice struct {
	t *testing.T

	simulatorURL string
	eventDataURL string

	clock        *modeltime.Clock
	buildsimFake *buildsim.FakeClient
	evidencePath string
}

// startSlice wires the five components together and starts them, sharing
// one buildsim.FakeClient and returning handles for driving/asserting on
// the scenario. All goroutines and listeners are cleaned up when the test
// ends.
func startSlice(t *testing.T) *slice {
	t.Helper()

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

	// Bind all listeners up front so every component's peer URLs are
	// known before any handler is constructed — this is what breaks the
	// Event/Data <-> Decision circular reference (ADR-005): in a real
	// deployment both URLs come from static configuration, not from
	// discovering each other's startup order, and the test harness
	// mirrors that instead of working around it.
	simulatorLn := listen(t)
	eventDataLn := listen(t)
	decisionLn := listen(t)
	actuatorLn := listen(t)

	simulatorURL := "http://" + simulatorLn.Addr().String()
	eventDataURL := "http://" + eventDataLn.Addr().String()
	decisionURL := "http://" + decisionLn.Addr().String()
	actuatorURL := "http://" + actuatorLn.Addr().String()

	simServer := simulator.New(clock, bs)
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

	return &slice{
		t:            t,
		simulatorURL: simulatorURL,
		eventDataURL: eventDataURL,
		clock:        clock,
		buildsimFake: bs,
		evidencePath: evidencePath,
	}
}

func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	return ln
}

func serve(t *testing.T, ln net.Listener, handler http.Handler) {
	t.Helper()
	srv := &http.Server{Handler: handler}
	go srv.Serve(ln) //nolint:errcheck // listener Close() in cleanup causes the expected "closed" error
}

// present enqueues a card presentation on the Simulator, as the course
// scenario-control input (docs/data-contracts.md section 8), and returns
// the model_time it was recorded at.
func (s *slice) present(cardID string) {
	s.t.Helper()
	reqBody := map[string]string{
		"reader_id": readerID,
		"card_id":   cardID,
		"door_id":   doorID,
		"direction": direction,
	}
	postJSONExpect(s.t, s.simulatorURL+"/scenario/present", reqBody, http.StatusAccepted)
}

// advanceModelTime fast-forwards the Simulator's model clock, as the
// slice's tests use instead of sleeping real seconds
// (design-specification.md section 4.3).
func (s *slice) advanceModelTime(d time.Duration) {
	s.t.Helper()
	postJSONExpect(s.t, s.simulatorURL+"/modeltime/advance", map[string]float64{
		"seconds": d.Seconds(),
	}, http.StatusOK)
}

// doorLockState reads the door's lock_state as the Simulator observes it
// (container.d2: "simulator -> buildsim: REST GET"), not as the Actuator
// sees it — this is "the simulator observing the resulting door state".
func (s *slice) doorLockState() string {
	s.t.Helper()
	var resp struct {
		LockState string `json:"lock_state"`
	}
	getJSONExpect(s.t, s.simulatorURL+"/doors/"+doorID+"/state", &resp)
	return resp.LockState
}

// evidenceRecords returns every evidence record written so far.
func (s *slice) evidenceRecords() []evidence.Record {
	s.t.Helper()
	records, err := evidence.ReadAll(s.evidencePath)
	if err != nil {
		s.t.Fatalf("read evidence: %v", err)
	}
	return records
}
