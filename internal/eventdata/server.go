// Package eventdata implements the Event & Data Service
// (architecture.md section 5.3, ADR-005). It validates sensor
// observations, stamps receive_time, forwards validated observations to
// the Autonomous Decision Service, and is the sole writer to the
// Historical Evidence Store. Duplicate/late/malformed-data handling
// (REQ-DATA-004/005/006) is deferred to a fault-injection slice.
package eventdata

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/contracts"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/evidence"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/httpx"
)

// Server is the Event & Data Service's HTTP handler.
type Server struct {
	store           *evidence.Store
	decisionBaseURL string
}

// New creates an Event & Data Service server.
func New(store *evidence.Store, decisionBaseURL string) *Server {
	return &Server{store: store, decisionBaseURL: decisionBaseURL}
}

// Handler returns the http.Handler for this server.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /observations", s.handleObservation)
	mux.HandleFunc("POST /decisions", s.handleDecision)
	mux.HandleFunc("POST /commands", s.handleCommand)
	mux.HandleFunc("POST /outcomes", s.handleOutcome)
	mux.HandleFunc("POST /events", s.handleDoorEvent)
	return mux
}

// handleObservation accepts an AccessAttemptObservation from a Card
// Reader, stamps receive_time (REQ-DATA-003), writes it to the Historical
// Evidence Store, and forwards it to the Autonomous Decision Service.
func (s *Server) handleObservation(w http.ResponseWriter, r *http.Request) {
	var obs contracts.AccessAttemptObservation
	if err := json.NewDecoder(r.Body).Decode(&obs); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	obs.ReceiveTime = time.Now().UTC().Format(time.RFC3339Nano)

	if err := s.store.Append(contracts.RecordTypeObservation, obs); err != nil {
		log.Printf("eventdata: append observation evidence: %v", err)
		http.Error(w, "evidence write failed", http.StatusInternalServerError)
		return
	}

	// REQ-RES-001: if the decision service is unavailable, the attempt
	// fails closed by simply not producing a decision or command — no
	// further action is taken here.
	if err := httpx.PostJSON(s.decisionBaseURL+"/decide", obs, nil); err != nil {
		log.Printf("eventdata: forward observation to decision service: %v", err)
		http.Error(w, "decision service unavailable", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// handleDecision accepts an AccessDecision from the Autonomous Decision
// Service for persistence (ADR-005 evidence-return path).
func (s *Server) handleDecision(w http.ResponseWriter, r *http.Request) {
	var decision contracts.AccessDecision
	if err := json.NewDecoder(r.Body).Decode(&decision); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := s.store.Append(contracts.RecordTypeDecision, decision); err != nil {
		log.Printf("eventdata: append decision evidence: %v", err)
		http.Error(w, "evidence write failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// handleCommand accepts an ActuatorCommand from the Autonomous Decision
// Service for persistence (ADR-005 evidence-return path; this is distinct
// from the Actuator's own POST /commands, which executes the command).
// Retaining the command itself, not just its outcome, is required for
// REQ-TEST-002: the outcome alone does not carry decision_id.
func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	var cmd contracts.ActuatorCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := s.store.Append(contracts.RecordTypeCommand, cmd); err != nil {
		log.Printf("eventdata: append command evidence: %v", err)
		http.Error(w, "evidence write failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// handleOutcome accepts an ActuatorOutcome from the Door Actuator for
// persistence (ADR-005 evidence-return path).
func (s *Server) handleOutcome(w http.ResponseWriter, r *http.Request) {
	var outcome contracts.ActuatorOutcome
	if err := json.NewDecoder(r.Body).Decode(&outcome); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := s.store.Append(contracts.RecordTypeOutcome, outcome); err != nil {
		log.Printf("eventdata: append outcome evidence: %v", err)
		http.Error(w, "evidence write failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// handleDoorEvent accepts a DoorEvent (this slice: DOOR_NOT_OPENED only)
// from the Autonomous Decision Service for persistence.
func (s *Server) handleDoorEvent(w http.ResponseWriter, r *http.Request) {
	var event contracts.DoorEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := s.store.Append(contracts.RecordTypeDoorEvent, event); err != nil {
		log.Printf("eventdata: append door event evidence: %v", err)
		http.Error(w, "evidence write failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
