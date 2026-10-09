// Package simulator implements the minimal Occupant & Movement Simulator
// needed by the first vertical slice: it owns model time (ADR-003) and
// simulated truth (ADR-006), and exposes both for observation rather than
// pushing into sensor processes (container.d2: "sensors -> simulator:
// observe simulated state / events").
//
// Full behavioral/schedule/stochastic generation (REQ-SIM-001/002/003/006)
// is out of scope for this slice.
package simulator

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/buildsim"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/contracts"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/modeltime"
)

// Server is the Simulator's HTTP handler. It holds no System Configuration
// (ADR-004) in this slice: the scenario-control caller supplies reader_id,
// door_id, and direction directly, keeping the Simulator config-free for
// now.
type Server struct {
	clock    *modeltime.Clock
	buildsim buildsim.Client
	mu       sync.Mutex
	pending  map[string][]contracts.PendingPresentation // keyed by reader_id, FIFO
}

// New creates a Simulator server. clock is the authoritative model clock;
// bs is shared with the Actuator so writes from one are visible to reads
// from the other.
func New(clock *modeltime.Clock, bs buildsim.Client) *Server {
	return &Server{
		clock:    clock,
		buildsim: bs,
		pending:  make(map[string][]contracts.PendingPresentation),
	}
}

// Handler returns the http.Handler for this server.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /pending-presentations", s.handlePendingPresentations)
	mux.HandleFunc("POST /scenario/present", s.handleScenarioPresent)
	mux.HandleFunc("GET /doors/{door_id}/state", s.handleDoorState)
	mux.HandleFunc("GET /modeltime", s.handleGetModelTime)
	mux.HandleFunc("POST /modeltime/advance", s.handleAdvanceModelTime)
	return mux
}

// handlePendingPresentations implements the Card Reader's observation
// interface: GET /pending-presentations?reader_id=... returns zero or one
// queued presentation, consumed on read (docs/data-contracts.md section 8).
func (s *Server) handlePendingPresentations(w http.ResponseWriter, r *http.Request) {
	readerID := r.URL.Query().Get("reader_id")
	if readerID == "" {
		http.Error(w, "reader_id is required", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	queue := s.pending[readerID]
	var next *contracts.PendingPresentation
	if len(queue) > 0 {
		next = &queue[0]
		s.pending[readerID] = queue[1:]
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if next == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	json.NewEncoder(w).Encode(next)
}

// handleScenarioPresent is the scenario-control input used by tests to
// enqueue a presentation. It belongs to the Simulator, not the Card
// Reader, so the Reader never receives an inbound push.
func (s *Server) handleScenarioPresent(w http.ResponseWriter, r *http.Request) {
	var req contracts.ScenarioPresentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.ReaderID == "" || req.CardID == "" || req.DoorID == "" || req.Direction == "" {
		http.Error(w, "reader_id, card_id, door_id, and direction are required", http.StatusBadRequest)
		return
	}

	presentation := contracts.PendingPresentation{
		CardID:    req.CardID,
		DoorID:    req.DoorID,
		Direction: req.Direction,
		ModelTime: modeltime.Format(s.clock.Now()),
	}

	s.mu.Lock()
	s.pending[req.ReaderID] = append(s.pending[req.ReaderID], presentation)
	s.mu.Unlock()

	w.WriteHeader(http.StatusAccepted)
}

// handleDoorState is the Simulator's own BuildSim read
// (container.d2: "simulator -> buildsim: REST GET"), exposed so a test
// harness can confirm "the simulator observes the resulting door state"
// without reaching into the Actuator's BuildSimClient directly.
func (s *Server) handleDoorState(w http.ResponseWriter, r *http.Request) {
	doorID := r.PathValue("door_id")
	state, err := s.buildsim.GetDoorState(doorID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"lock_state":    string(state.LockState),
		"door_position": string(state.DoorPosition),
	})
}

func (s *Server) handleGetModelTime(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"model_time": modeltime.Format(s.clock.Now()),
	})
}

// handleAdvanceModelTime is test/scenario-control only
// (design-specification.md section 4.3: model time is independent of
// wall-clock time, so tests advance it deterministically instead of
// sleeping).
func (s *Server) handleAdvanceModelTime(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Seconds float64 `json:"seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	now := s.clock.Advance(time.Duration(req.Seconds * float64(time.Second)))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"model_time": modeltime.Format(now),
	})
}
