// Package occupancysensor implements one OccupancySensor process
// (design-specification.md section 2.8). It observes simulated occupancy
// truth by polling the Simulator — not BuildSim's mirrored copy of it,
// which would make the mirror a second source of truth (ADR-006) — and
// reports an OccupancyObservation to the Event/Data Service whenever the
// observed value changes.
package occupancysensor

import (
	"context"
	"log"
	"net/url"
	"time"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/contracts"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/httpx"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/idgen"
)

// Sensor polls one room's occupancy truth from the Simulator.
type Sensor struct {
	SourceID         string // this device's identifier
	RoomKey          string // level-qualified room this instance watches (REQ-TOPO-004)
	SimulatorBaseURL string
	EventDataBaseURL string
	PollInterval     time.Duration

	lastCount int
	haveLast  bool
}

// Run polls until ctx is canceled, reporting an OccupancyObservation each
// time the observed count changes.
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
	reqURL := s.SimulatorBaseURL + "/occupancy?room=" + url.QueryEscape(s.RoomKey)
	var resp struct {
		RoomKey string `json:"room_key"`
		Count   int    `json:"count"`
	}
	if err := httpx.GetJSON(reqURL, &resp); err != nil {
		log.Printf("occupancysensor %s: poll simulator: %v", s.SourceID, err)
		return
	}

	if s.haveLast && resp.Count == s.lastCount {
		return // unchanged: this slice reports on change, not continuously
	}
	s.lastCount = resp.Count
	s.haveLast = true

	now := time.Now().UTC().Format(time.RFC3339Nano)
	observation := contracts.OccupancyObservation{
		EventID:       idgen.New(),
		SchemaVersion: "1",
		SourceID:      s.SourceID,
		ModelTime:     "", // filled from the simulator's own model_time below
		SourceTime:    now,
		RoomKey:       s.RoomKey,
		Count:         resp.Count,
		Quality:       contracts.ObservationQualityOK,
	}

	var modelTimeResp struct {
		ModelTime string `json:"model_time"`
	}
	if err := httpx.GetJSON(s.SimulatorBaseURL+"/modeltime", &modelTimeResp); err != nil {
		log.Printf("occupancysensor %s: read model time: %v", s.SourceID, err)
		return
	}
	observation.ModelTime = modelTimeResp.ModelTime

	if err := httpx.PostJSON(s.EventDataBaseURL+"/occupancy-observations", observation, nil); err != nil {
		log.Printf("occupancysensor %s: post observation: %v", s.SourceID, err)
	}
}
