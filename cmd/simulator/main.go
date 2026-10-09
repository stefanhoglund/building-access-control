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
	buildsimURL := flag.String("buildsim-url", "http://127.0.0.1:9090", "real BuildSim base URL")
	fromRoom := flag.String("from-room", "level0/1540", "source room for the simulated crossing (REQ-TOPO-004)")
	toRoom := flag.String("to-room", "level0/A109", "destination room for the simulated crossing")
	personID := flag.String("person-id", "sim-person-1", "the one simulated occupant's identifier")
	personName := flag.String("person-name", "Occupant", "the one simulated occupant's display name")
	disableCrossing := flag.Bool("disable-crossing", false, "disable successful-passage simulation (ALLOW/DENY/relock-timeout only)")
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

	bs := buildsim.NewHTTPClient(*buildsimURL)

	var crossing simulator.CrossingConfig
	initialOccupancy := map[string]int{}
	if !*disableCrossing {
		crossing = simulator.CrossingConfig{
			DoorID:     *doorID,
			FromRoom:   *fromRoom,
			ToRoom:     *toRoom,
			PersonID:   *personID,
			PersonName: *personName,
		}
		initialOccupancy[*fromRoom] = 1
		initialOccupancy[*toRoom] = 0
	}

	srv := simulator.New(clock, bs, crossing, initialOccupancy)
	srv.MirrorInitialOccupancy()

	log.Printf("simulator: listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, srv.Handler()))
}
