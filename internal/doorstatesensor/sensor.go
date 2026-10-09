// Package doorstatesensor implements one DoorStateSensor process
// (design-specification.md section 2.9). It observes a controlled door's
// physical state by polling BuildSimClient.GetDoorState directly — not by
// trusting the Simulator's account of what it caused — preserving the
// distinction between simulated/physical truth and sensor observation
// (section 4.1) even though, in this slice, the Simulator happens to be
// the one causing the transition the sensor observes.
package doorstatesensor

import (
	"context"
	"log"
	"time"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/buildsim"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/contracts"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/httpx"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/idgen"
)

// Sensor polls one door's physical state directly from BuildSim.
type Sensor struct {
	SourceID         string // this device's identifier
	DoorID           string
	BuildSim         buildsim.Client
	SimulatorBaseURL string // model-time source only (ADR-003); never polled for door/occupancy truth
	EventDataBaseURL string
	PollInterval     time.Duration

	lastPosition string
	haveLast     bool
}

// Run polls until ctx is canceled, reporting a DoorStateObservation each
// time the observed door_position changes.
func (s *Sensor) Run(ctx context.Context) {
	ticker := time.NewTicker(s.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pollOnce()
		}
	}
}

func (s *Sensor) pollOnce() {
	state, err := s.BuildSim.GetDoorState(s.DoorID)
	if err != nil {
		log.Printf("doorstatesensor %s: get door state: %v", s.SourceID, err)
		return
	}
	position := string(state.DoorPosition)

	if s.haveLast && position == s.lastPosition {
		return // unchanged: report on change, not continuously
	}
	s.lastPosition = position
	s.haveLast = true

	var modelTimeResp struct {
		ModelTime string `json:"model_time"`
	}
	if err := httpx.GetJSON(s.SimulatorBaseURL+"/modeltime", &modelTimeResp); err != nil {
		log.Printf("doorstatesensor %s: read model time: %v", s.SourceID, err)
		return
	}

	observation := contracts.DoorStateObservation{
		EventID:       idgen.New(),
		SchemaVersion: "1",
		SourceID:      s.SourceID,
		ModelTime:     modelTimeResp.ModelTime,
		SourceTime:    time.Now().UTC().Format(time.RFC3339Nano),
		DoorID:        s.DoorID,
		DoorPosition:  position,
		Quality:       contracts.ObservationQualityOK,
	}

	if err := httpx.PostJSON(s.EventDataBaseURL+"/door-state-observations", observation, nil); err != nil {
		log.Printf("doorstatesensor %s: post observation: %v", s.SourceID, err)
	}
}
