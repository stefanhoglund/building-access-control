// Package cardreader implements the Card Reader sensor process
// (design-specification.md section 2.7). It observes the Simulator's
// truth by polling rather than being pushed to (container.d2:
// "sensors -> simulator: observe simulated state / events"), and its
// responsibility ends after reporting the presentation — it does not
// determine authorization, calculate anomaly scores, or unlock a door.
package cardreader

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/contracts"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/httpx"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/idgen"
)

// Reader polls one Simulator for pending presentations addressed to
// ReaderID and reports each as an AccessAttemptObservation to the
// Event/Data Service.
type Reader struct {
	ReaderID         string
	SimulatorBaseURL string
	EventDataBaseURL string
	PollInterval     time.Duration
}

// Run polls until ctx is canceled.
func (r *Reader) Run(ctx context.Context) {
	ticker := time.NewTicker(r.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.pollOnce()
		}
	}
}

func (r *Reader) pollOnce() {
	url := fmt.Sprintf("%s/pending-presentations?reader_id=%s", r.SimulatorBaseURL, r.ReaderID)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("cardreader %s: poll simulator: %v", r.ReaderID, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("cardreader %s: poll simulator: status %d", r.ReaderID, resp.StatusCode)
		return
	}

	var presentation contracts.PendingPresentation
	if err := json.NewDecoder(resp.Body).Decode(&presentation); err != nil {
		log.Printf("cardreader %s: decode presentation: %v", r.ReaderID, err)
		return
	}

	observation := contracts.AccessAttemptObservation{
		EventID:       idgen.New(),
		SchemaVersion: "1",
		SourceID:      r.ReaderID,
		ModelTime:     presentation.ModelTime,
		SourceTime:    time.Now().UTC().Format(time.RFC3339Nano),
		CardID:        presentation.CardID,
		DoorID:        presentation.DoorID,
		Direction:     presentation.Direction,
	}

	if err := httpx.PostJSON(r.EventDataBaseURL+"/observations", observation, nil); err != nil {
		log.Printf("cardreader %s: post observation: %v", r.ReaderID, err)
	}
}
