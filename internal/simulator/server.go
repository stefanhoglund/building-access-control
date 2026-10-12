// Package simulator implements the Occupant & Movement Simulator: it owns
// model time (ADR-003) and simulated truth — occupant intent, door
// position, and occupancy (ADR-006) — and exposes it for observation
// rather than pushing into sensor processes (container.d2: "sensors ->
// simulator: observe simulated state / events").
//
// It simulates exactly one controlled door (REQ-FUNC-004, REQ-SIM-005,
// REQ-SIM-006) but a small, configured population of simulated people can
// cross it (population.go), driven either by ad hoc scenario-control
// presentations (POST /scenario/present) or by a named, seeded scenario
// (POST /scenario/run, scenarios.go — REQ-SIM-001/002/003). Full
// probabilistic BehaviorProfile generation (design-specification.md
// section 3.3) remains out of scope: scenario generation here is
// deliberately small and data-driven, not a general crowd simulator.
package simulator

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/buildsim"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/contracts"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/evidence"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/modeltime"
)

// unlockWaitTimeout bounds how long the Simulator waits
// (design-specification.md section 11.6: bounded waits, not indefinite
// blocking) for a door to become UNLOCKED before giving up on a crossing.
// A DENY attempt never unlocks the door, so this bound is also what makes
// "no occupancy transition occurs when movement is denied" (REQ-SIM-005)
// hold without any separate denial-handling code path.
const unlockWaitTimeout = 5 * time.Second
const unlockPollInterval = 20 * time.Millisecond

// passageDwell is how long the door is held OPEN during a simulated
// crossing before closing again. This is not just simulation realism: the
// door-open/door-close transition is now only knowable to the rest of the
// system through an independent DoorStateSensor polling BuildSim
// (design-specification.md section 2.9 — the Simulator causes the
// transition but is not the one who reports observing it). Without a
// deliberate dwell, a poller could miss the OPEN state entirely between
// two poll ticks. This value must stay comfortably larger than any
// DoorStateSensor's configured poll interval.
const passageDwell = 150 * time.Millisecond

// CrossingConfig describes the one controlled-door crossing this slice
// simulates: the door and the two rooms it separates. A zero-value
// CrossingConfig (DoorID == "") disables crossing simulation entirely —
// existing scenarios that only need ALLOW/DENY/relock-timeout behavior
// are unaffected. The identity of whoever is currently crossing is
// deliberately NOT part of this config (see crossNow/mirrorOccupancy): it
// now comes from whichever presentation triggered the crossing, so
// multiple different simulated people (section 3.1 SimulatedPerson) can
// cross the same door over the course of a scenario.
type CrossingConfig struct {
	DoorID   string
	FromRoom string // level-qualified, e.g. "level0/1540" (REQ-TOPO-004)
	ToRoom   string // level-qualified, e.g. "level0/A109"
}

// Server is the Simulator's HTTP handler. It holds no System Configuration
// (ADR-004): the scenario-control caller supplies reader_id, door_id, and
// direction directly, and CrossingConfig supplies the one crossing this
// slice knows how to simulate. population and scenarios are optional
// (REQ-SIM-001/002/003, design-specification.md section 3): a Server with
// neither set behaves exactly as it did before named scenarios existed.
type Server struct {
	clock    *modeltime.Clock
	buildsim buildsim.Client
	crossing CrossingConfig

	population     *Population
	scenarios      *ScenarioSet
	scenarioRunLog *evidence.Store

	mu              sync.Mutex
	pending         map[string][]contracts.PendingPresentation // keyed by reader_id, FIFO
	occupancy       map[string]int                             // keyed by level-qualified room
	inFlight        map[string]bool                            // doorID -> crossing goroutine active
	lastCrosserID   string                                     // most recent crosser's identity, for BuildSim mirroring only
	lastCrosserName string
}

// New creates a Simulator server. clock is the authoritative model clock;
// bs is shared with the Actuator so writes from one are visible to reads
// from the other. The Simulator never reports having observed a door
// transition or an occupancy change — see DoorStateObservation and
// OccupancyObservation (docs/data-contracts.md sections 5/7) — so it has
// no need of the Event/Data Service's address. initialOccupancy seeds
// room occupancy truth, keyed by level-qualified room.
func New(clock *modeltime.Clock, bs buildsim.Client, crossing CrossingConfig, initialOccupancy map[string]int) *Server {
	occupancy := make(map[string]int, len(initialOccupancy))
	for room, count := range initialOccupancy {
		occupancy[room] = count
	}
	return &Server{
		clock:     clock,
		buildsim:  bs,
		crossing:  crossing,
		pending:   make(map[string][]contracts.PendingPresentation),
		occupancy: occupancy,
		inFlight:  make(map[string]bool),
	}
}

// Handler returns the http.Handler for this server.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /pending-presentations", s.handlePendingPresentations)
	mux.HandleFunc("POST /scenario/present", s.handleScenarioPresent)
	mux.HandleFunc("POST /scenario/run", s.handleScenarioRun)
	mux.HandleFunc("GET /doors/{door_id}/state", s.handleDoorState)
	mux.HandleFunc("GET /occupancy", s.handleGetOccupancy)
	mux.HandleFunc("GET /modeltime", s.handleGetModelTime)
	mux.HandleFunc("POST /modeltime/advance", s.handleAdvanceModelTime)
	return mux
}

// MirrorInitialOccupancy writes the current occupancy truth for every room
// named in CrossingConfig to BuildSim, so BuildSim's visualization starts
// consistent with simulator truth rather than empty. It is a no-op if
// crossing is disabled.
func (s *Server) MirrorInitialOccupancy() {
	if s.crossing.DoorID == "" {
		return
	}
	s.mirrorOccupancy(s.crossing.FromRoom)
	s.mirrorOccupancy(s.crossing.ToRoom)
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
// Reader, so the Reader never receives an inbound push. If the presented
// door matches CrossingConfig, it also starts watching for the door to
// unlock so the occupant can cross (REQ-FUNC-004/REQ-SIM-005). The
// mirrored occupant identity falls back to the card_id itself: this
// single-presentation path predates the configured Population and has no
// obligation to know a nicer display name for whoever is presenting.
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

	s.enqueuePresentation(req.ReaderID, req.CardID, req.DoorID, req.Direction, req.CardID)

	w.WriteHeader(http.StatusAccepted)
}

// handleScenarioRun invokes a named, seeded scenario (scenarios.go,
// design-specification.md section 3.5). It blocks until every
// presentation the scenario specifies has been enqueued, so a caller
// (test or operator) can rely on the evidence pipeline having everything
// it needs to process as soon as this call returns.
func (s *Server) handleScenarioRun(w http.ResponseWriter, r *http.Request) {
	var req contracts.ScenarioRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Scenario == "" {
		http.Error(w, "scenario is required", http.StatusBadRequest)
		return
	}
	result, err := s.RunScenario(req.Scenario, req.Seed, req.Count, req.IntervalMillis)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// enqueuePresentation is the one place a presentation is queued for a
// reader and, for the crossing-configured door, a crossing is started to
// watch for. personID/personName identify whoever is presenting for
// BuildSim mirroring purposes only (mirrorOccupancy) — never for
// authorization, which depends solely on cardID (decision.authorize).
func (s *Server) enqueuePresentation(readerID, cardID, doorID, direction, personID string) {
	presentation := contracts.PendingPresentation{
		CardID:    cardID,
		DoorID:    doorID,
		Direction: direction,
		ModelTime: modeltime.Format(s.clock.Now()),
	}

	s.mu.Lock()
	s.pending[readerID] = append(s.pending[readerID], presentation)
	s.mu.Unlock()

	go s.watchForUnlockAndCross(doorID, personID, personID)
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

// handleGetOccupancy implements the OccupancySensor's observation
// interface: GET /occupancy?room=<level-qualified room> returns the
// Simulator's current occupancy truth for that room
// (design-specification.md section 2.8: sensors observe simulated truth,
// not BuildSim's mirrored copy of it).
func (s *Server) handleGetOccupancy(w http.ResponseWriter, r *http.Request) {
	room := r.URL.Query().Get("room")
	if room == "" {
		http.Error(w, "room is required", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	count := s.occupancy[room]
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"room_key": room,
		"count":    count,
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

// watchForUnlockAndCross waits (bounded) for doorID to become UNLOCKED and
// then simulates personID/personName crossing it. It is a no-op for any
// door other than the one CrossingConfig names, and it will not start a
// second concurrent crossing for the same door.
func (s *Server) watchForUnlockAndCross(doorID, personID, personName string) {
	if s.crossing.DoorID == "" || s.crossing.DoorID != doorID {
		return
	}
	if !s.beginCrossing(doorID) {
		return
	}
	defer s.endCrossing(doorID)

	deadline := time.Now().Add(unlockWaitTimeout)
	for {
		state, err := s.buildsim.GetDoorState(doorID)
		if err == nil && state.LockState == buildsim.LockStateUnlocked {
			break
		}
		if time.Now().After(deadline) {
			// REQ-SIM-004/REQ-SIM-005: the door never unlocked (e.g. a
			// DENY decision) — the occupant does not cross, and no
			// occupancy transition occurs.
			return
		}
		time.Sleep(unlockPollInterval)
	}

	s.crossNow(doorID, personID, personName)
}

func (s *Server) beginCrossing(doorID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inFlight[doorID] {
		return false
	}
	s.inFlight[doorID] = true
	return true
}

func (s *Server) endCrossing(doorID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.inFlight, doorID)
}

// crossNow simulates the occupant opening the door, crossing, and closing
// it: ALLOW -> UNLOCK (already done by the caller/Actuator) -> OPEN ->
// occupant crosses -> source occupancy decreases, destination occupancy
// increases -> CLOSE. The Simulator only causes these physical
// transitions in BuildSim; it does not report having observed them, and
// it does not lock or unlock the door itself. An independent
// DoorStateSensor reports the OPEN/CLOSE transitions
// (design-specification.md section 2.9), and the Autonomous Decision
// Service, informed by that sensor's observations, issues the relock
// command (section 9.7).
func (s *Server) crossNow(doorID, personID, personName string) {
	cfg := s.crossing

	if err := s.buildsim.SetDoorPosition(doorID, buildsim.DoorPositionOpen); err != nil {
		log.Printf("simulator: set door_position OPEN: %v", err)
	}

	time.Sleep(passageDwell)

	s.mu.Lock()
	if s.occupancy[cfg.FromRoom] > 0 {
		s.occupancy[cfg.FromRoom]--
	}
	s.occupancy[cfg.ToRoom]++
	s.lastCrosserID = personID
	s.lastCrosserName = personName
	s.mu.Unlock()

	s.mirrorOccupancy(cfg.FromRoom)
	s.mirrorOccupancy(cfg.ToRoom)

	if err := s.buildsim.SetDoorPosition(doorID, buildsim.DoorPositionClosed); err != nil {
		log.Printf("simulator: set door_position CLOSED: %v", err)
	}
}

// mirrorOccupancy writes roomKey's current occupancy truth to BuildSim
// for visualization only (design-specification.md section 4.5, ADR-006 —
// one-way, never read back). A named scenario's population_sample kind
// can send several different simulated people through the same door over
// a run; this mirror deliberately does not attempt to list everyone who
// has ever accumulated in a room (that would need real occupant tracking,
// not a visualization mirror) — it represents a nonzero count as the most
// recent crosser, which is enough for "BuildSim shows someone is there"
// without the mirror claiming to be authoritative (it never is, ADR-006).
// The Simulator's own /occupancy endpoint and the OccupancyObservation
// pipeline carry the real count regardless of what the mirror shows.
func (s *Server) mirrorOccupancy(roomKey string) {
	s.mu.Lock()
	count := s.occupancy[roomKey]
	cfg := s.crossing
	crosserID, crosserName := s.lastCrosserID, s.lastCrosserName
	s.mu.Unlock()

	var persons []buildsim.Person
	if count > 0 && (roomKey == cfg.FromRoom || roomKey == cfg.ToRoom) && crosserID != "" {
		persons = []buildsim.Person{{ID: crosserID, Name: crosserName}}
	}
	if err := s.buildsim.SetRoomOccupancy(roomKey, buildsim.RoomOccupancy{Persons: persons}); err != nil {
		log.Printf("simulator: mirror occupancy for %s: %v", roomKey, err)
	}
}
