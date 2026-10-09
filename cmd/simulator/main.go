// Command simulator runs the Occupant & Movement Simulator process.
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/buildsim"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/modeltime"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/simulator"
)

func main() {
	addr := flag.String("addr", ":8081", "listen address")
	modelStart := flag.String("model-start", "", "initial model time (RFC3339); defaults to now")
	doorID := flag.String("door-id", "door-1", "the one controlled door for this slice")
	flag.Parse()

	start := time.Now().UTC()
	if *modelStart != "" {
		parsed, err := modeltime.Parse(*modelStart)
		if err != nil {
			log.Fatalf("simulator: invalid -model-start: %v", err)
		}
		start = parsed
	}
	clock := modeltime.NewClock(start)

	// This slice has no real BuildSim adapter yet (design-specification.md
	// section 12.3). A standalone run of this process has no shared fake
	// to talk to across processes; the acceptance tests instead wire
	// simulator.New directly with a shared *buildsim.FakeClient. This
	// main() exists so the process is independently runnable, not because
	// it is used by the tests.
	bs := buildsim.NewFakeClient(map[string]buildsim.DoorState{
		*doorID: {LockState: buildsim.LockStateLocked, DoorPosition: buildsim.DoorPositionClosed},
	})

	srv := simulator.New(clock, bs)
	log.Printf("simulator: listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, srv.Handler()))
}
