// Package actuator implements a Door Actuator process
// (design-specification.md section 2.10). It validates and applies
// actuator commands through a buildsim.Client, deduplicates by
// command_id (REQ-DIST-007), and reports the outcome to the Event/Data
// Service. It does not determine whether access should be granted.
package actuator

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/buildsim"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/contracts"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/httpx"
)

// Server is one Door Actuator's HTTP handler, bound to a single
// ControlledDoor for this slice.
type Server struct {
	buildsim         buildsim.Client
	eventDataBaseURL string

	mu   sync.Mutex
	seen map[string]bool // applied command_id values (REQ-DIST-007)
}

// New creates a Door Actuator server.
func New(bs buildsim.Client, eventDataBaseURL string) *Server {
	return &Server{
		buildsim:         bs,
		eventDataBaseURL: eventDataBaseURL,
		seen:             make(map[string]bool),
	}
}

// Handler returns the http.Handler for this server.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /commands", s.handleCommand)
	return mux
}

func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	var cmd contracts.ActuatorCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	if s.seen[cmd.CommandID] {
		s.mu.Unlock()
		// REQ-DIST-007: a repeated command_id must not cause a second
		// physical side effect. The first application already reported
		// its outcome; acknowledge and do nothing further.
		w.WriteHeader(http.StatusOK)
		return
	}
	s.seen[cmd.CommandID] = true
	s.mu.Unlock()

	var target buildsim.LockState
	switch cmd.Action {
	case contracts.ActionUnlock:
		target = buildsim.LockStateUnlocked
	case contracts.ActionLock:
		target = buildsim.LockStateLocked
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}

	outcome := contracts.ActuatorOutcome{
		CommandID: cmd.CommandID,
		DoorID:    cmd.DoorID,
		Action:    cmd.Action,
		ModelTime: cmd.ModelTime,
	}

	if err := s.buildsim.SetDoorState(cmd.DoorID, target); err != nil {
		log.Printf("actuator: set door state: %v", err)
		outcome.Result = contracts.OutcomeUnverified
	} else if state, err := s.buildsim.GetDoorState(cmd.DoorID); err != nil || state.LockState != target {
		// Commanded state is not proven merely by a successful write
		// (design-specification.md section 4.2/9.5): confirm by reading
		// it back.
		outcome.Result = contracts.OutcomeUnverified
	} else {
		outcome.Result = contracts.OutcomeConfirmed
		outcome.ObservedState = string(state.LockState)
	}

	if err := httpx.PostJSON(s.eventDataBaseURL+"/outcomes", outcome, nil); err != nil {
		log.Printf("actuator: post outcome evidence: %v", err)
	}

	w.WriteHeader(http.StatusOK)
}
