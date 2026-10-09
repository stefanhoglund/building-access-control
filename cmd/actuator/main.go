// Command actuator runs one Door Actuator process.
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/actuator"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/buildsim"
)

func main() {
	addr := flag.String("addr", ":8084", "listen address")
	eventDataURL := flag.String("eventdata-url", "http://localhost:8082", "Event/Data Service base URL")
	doorID := flag.String("door-id", "door-1", "the one controlled door this actuator applies commands to")
	flag.Parse()

	// No real BuildSim adapter yet (design-specification.md section
	// 12.3); see cmd/simulator's main.go comment about standalone runs.
	bs := buildsim.NewFakeClient(map[string]buildsim.DoorState{
		*doorID: {LockState: buildsim.LockStateLocked, DoorPosition: buildsim.DoorPositionClosed},
	})

	srv := actuator.New(bs, *eventDataURL)
	log.Printf("actuator: listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, srv.Handler()))
}
