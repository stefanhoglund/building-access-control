// Package decision implements the Autonomous Decision Service
// (architecture.md section 5.5). This slice implements authorization and
// decision policy fully; the anomaly module is a structural pass-through
// (anomaly_result is always NOT_EVALUATED, design-specification.md
// section 7.4); runtime assurance is the minimal hard-constraint check
// reachable by this slice's paths. It also owns the per-door logical
// control state and relock timer (design-specification.md section 9.7,
// architecture.md section 11).
package decision

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/config"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/contracts"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/httpx"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/idgen"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/modeltime"
)

// relockTimeout is REQ-FUNC-003's 10-model-second no-open window.
const relockTimeout = 10 * time.Second

// doorControlState is the per-door logical control state owned by this
// service while an access cycle is active (design-specification.md
// section 9.7). opened is set once a DOOR_OPENED event arrives for this
// door, which disarms the no-open timeout (REQ-FUNC-003 only applies
// while the door has not been opened) in favor of relock-after-passage
// (REQ-FUNC-004), triggered by the subsequent DOOR_CLOSED event.
type doorControlState struct {
	decisionID      string
	unlockModelTime time.Time
	opened          bool
}

// Server is the Autonomous Decision Service's HTTP handler plus its
// background relock-timeout ticker.
type Server struct {
	cfg              *config.Config
	eventDataBaseURL string
	actuatorBaseURL  string
	simulatorBaseURL string
	anomaly          *anomalyTracker

	mu      sync.Mutex
	pending map[string]doorControlState // keyed by door_id
}

// New creates an Autonomous Decision Service server.
func New(cfg *config.Config, eventDataBaseURL, actuatorBaseURL, simulatorBaseURL string) *Server {
	return &Server{
		cfg:              cfg,
		eventDataBaseURL: eventDataBaseURL,
		actuatorBaseURL:  actuatorBaseURL,
		simulatorBaseURL: simulatorBaseURL,
		anomaly:          newAnomalyTracker(),
		pending:          make(map[string]doorControlState),
	}
}

// Handler returns the http.Handler for this server.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /decide", s.handleDecide)
	mux.HandleFunc("POST /door-state-observations", s.handleDoorStateObservation)
	return mux
}

// RunRelockTicker polls model time every pollInterval (a wall-clock
// operational cadence, design-specification.md section 4.3.3) and relocks
// any door whose relock timer has expired in model time. It runs until ctx
// is canceled.
func (s *Server) RunRelockTicker(ctx context.Context, pollInterval time.Duration) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkRelockTimers()
		}
	}
}

func (s *Server) checkRelockTimers() {
	var modelTimeResp struct {
		ModelTime string `json:"model_time"`
	}
	if err := httpx.GetJSON(s.simulatorBaseURL+"/modeltime", &modelTimeResp); err != nil {
		log.Printf("decision: read model time: %v", err)
		return
	}
	now, err := modeltime.Parse(modelTimeResp.ModelTime)
	if err != nil {
		log.Printf("decision: parse model time: %v", err)
		return
	}

	s.mu.Lock()
	var expired []string
	for doorID, state := range s.pending {
		// A door that was opened is no longer subject to the no-open
		// timeout; relock-after-passage (REQ-FUNC-004) owns it instead,
		// triggered by handleDoorStateObservation on a CLOSED observation.
		if !state.opened && !now.Before(state.unlockModelTime.Add(relockTimeout)) {
			expired = append(expired, doorID)
		}
	}
	s.mu.Unlock()

	for _, doorID := range expired {
		s.relockOnTimeout(doorID, now)
	}
}

// relockOnTimeout implements design-specification.md section 9.3: no
// door-open observation within 10 model seconds produces a
// DOOR_NOT_OPENED event and a LOCK command.
func (s *Server) relockOnTimeout(doorID string, now time.Time) {
	state, ok := s.takePending(doorID)
	if !ok {
		return // already relocked by a concurrent tick
	}

	nowStr := modeltime.Format(now)

	event := contracts.DoorEvent{
		EventType:  contracts.DoorEventNotOpened,
		DoorID:     doorID,
		DecisionID: state.decisionID,
		ModelTime:  nowStr,
	}
	if err := httpx.PostJSON(s.eventDataBaseURL+"/events", event, nil); err != nil {
		log.Printf("decision: post DOOR_NOT_OPENED event: %v", err)
	}

	s.relock(doorID, state.decisionID, contracts.ReasonRelockTimeout, nowStr)
}

// takePending atomically removes and returns doorID's pending control
// state, if any. Used by both relock paths so a door is relocked exactly
// once regardless of which path wins the race.
func (s *Server) takePending(doorID string) (doorControlState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.pending[doorID]
	if ok {
		delete(s.pending, doorID)
	}
	return state, ok
}

// relock issues the LOCK command common to both relock paths (no-open
// timeout and after-passage).
func (s *Server) relock(doorID, decisionID, reason, modelTime string) {
	command := contracts.ActuatorCommand{
		CommandID:  idgen.New(),
		DecisionID: decisionID,
		DoorID:     doorID,
		Action:     contracts.ActionLock,
		Reason:     reason,
		ModelTime:  modelTime,
	}
	s.issueCommand(command)
}

// handleDoorStateObservation reacts to the DoorStateSensor's observed
// door_position (design-specification.md section 9.7, REQ-FUNC-004): an
// OPEN observation disarms the no-open timeout for this door; a CLOSED
// observation for a door already marked opened relocks it immediately,
// using the observation's own model_time rather than re-querying it. A
// CLOSED observation for a door not marked opened (e.g. the sensor's
// initial steady-state report, or a door with no pending decision at all)
// is not a passage-complete signal and is ignored — see
// docs/data-contracts.md section 5 ("Correlation").
func (s *Server) handleDoorStateObservation(w http.ResponseWriter, r *http.Request) {
	var obs contracts.DoorStateObservation
	if err := json.NewDecoder(r.Body).Decode(&obs); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	switch obs.DoorPosition {
	case contracts.DoorPositionOpen:
		s.mu.Lock()
		if state, ok := s.pending[obs.DoorID]; ok {
			state.opened = true
			s.pending[obs.DoorID] = state
		}
		s.mu.Unlock()
	case contracts.DoorPositionClosed:
		s.mu.Lock()
		state, ok := s.pending[obs.DoorID]
		shouldRelock := ok && state.opened
		s.mu.Unlock()
		if shouldRelock {
			if state, ok := s.takePending(obs.DoorID); ok {
				s.relock(obs.DoorID, state.decisionID, contracts.ReasonPassageComplete, obs.ModelTime)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

// handleDecide evaluates a validated AccessAttemptObservation and
// produces a final AccessDecision (design-specification.md sections 5-8).
func (s *Server) handleDecide(w http.ResponseWriter, r *http.Request) {
	var obs contracts.AccessAttemptObservation
	if err := json.NewDecoder(r.Body).Decode(&obs); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	auth := authorize(s.cfg, obs.CardID, obs.DoorID, obs.Direction)

	// Anomaly detection (anomaly.go) is evaluated entirely independently
	// of auth above: it is never given the authorization result, and its
	// result is never read by the two lines that set FinalAction below.
	// This is REQ-DEC-002/003 made structurally true, not just asserted.
	anomalyModelTime, err := modeltime.Parse(obs.ModelTime)
	if err != nil {
		log.Printf("decision: parse observation model time for anomaly scoring: %v", err)
	}
	anom := s.anomaly.score(obs.CardID, anomalyModelTime)

	decision := contracts.AccessDecision{
		DecisionID:          idgen.New(),
		EventID:             obs.EventID,
		DoorID:              obs.DoorID,
		Direction:           obs.Direction,
		AuthorizationResult: auth.result,
		AuthorizationReason: auth.reason,
		AnomalyResult:       anom.classification,
		AnomalyReason:       anom.reason,
		AnomalyScore:        anom.score,
		Alert:               anom.classification == contracts.AnomalyAnomalous,
		ModelTime:           obs.ModelTime,
		PolicyVersion:       contracts.PolicyVersion,
	}

	// Decision Policy (design-specification.md section 7.3): the final
	// action depends solely on auth.result. This is REQ-DEC-003/section
	// 8.2 made structurally true: nothing in this function, including the
	// anomaly outcome computed above, can turn a DENY into ALLOW — Alert
	// is the only place the anomaly result is expressed.
	if auth.result == contracts.AuthResultAllow {
		decision.FinalAction = contracts.FinalActionAllow
	} else {
		decision.FinalAction = contracts.FinalActionDeny
	}

	if err := httpx.PostJSON(s.eventDataBaseURL+"/decisions", decision, nil); err != nil {
		log.Printf("decision: post decision evidence: %v", err)
	}

	if decision.FinalAction == contracts.FinalActionAllow {
		// Runtime Assurance (design-specification.md section 8.2): the
		// only hard constraint reachable on this path is "valid,
		// authorized ALLOW may unlock" — a DENY never reaches here.
		s.issueUnlock(decision)
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) issueUnlock(decision contracts.AccessDecision) {
	unlockModelTime, err := modeltime.Parse(decision.ModelTime)
	if err != nil {
		log.Printf("decision: parse decision model time: %v", err)
		return
	}

	command := contracts.ActuatorCommand{
		CommandID:  idgen.New(),
		DecisionID: decision.DecisionID,
		DoorID:     decision.DoorID,
		Action:     contracts.ActionUnlock,
		Reason:     contracts.ReasonAllowUnlock,
		ModelTime:  decision.ModelTime,
	}
	if !s.issueCommand(command) {
		return
	}

	s.mu.Lock()
	s.pending[decision.DoorID] = doorControlState{
		decisionID:      decision.DecisionID,
		unlockModelTime: unlockModelTime,
	}
	s.mu.Unlock()
}

// issueCommand sends cmd to the Door Actuator for execution and, per
// ADR-005, separately returns it to the Event/Data Service for
// persistence (docs/data-contracts.md section 6: "command" is a retained
// evidence type, distinct from its eventual outcome). It reports whether
// the actuator accepted the command.
func (s *Server) issueCommand(cmd contracts.ActuatorCommand) bool {
	if err := httpx.PostJSON(s.eventDataBaseURL+"/commands", cmd, nil); err != nil {
		log.Printf("decision: post command evidence: %v", err)
	}
	if err := httpx.PostJSON(s.actuatorBaseURL+"/commands", cmd, nil); err != nil {
		log.Printf("decision: post command to actuator: %v", err)
		return false
	}
	return true
}
