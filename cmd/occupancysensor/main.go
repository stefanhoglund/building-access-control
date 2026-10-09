// Command occupancysensor runs one OccupancySensor process.
package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/occupancysensor"
)

func main() {
	sourceID := flag.String("source-id", "occupancysensor-1", "this device's identifier")
	room := flag.String("room", "level0/A109", "level-qualified room this instance watches (REQ-TOPO-004)")
	simulatorURL := flag.String("simulator-url", "http://localhost:8081", "Simulator base URL")
	eventDataURL := flag.String("eventdata-url", "http://localhost:8082", "Event/Data Service base URL")
	pollInterval := flag.Duration("poll-interval", 200*time.Millisecond, "polling cadence against the Simulator")
	flag.Parse()

	sensor := &occupancysensor.Sensor{
		SourceID:         *sourceID,
		RoomKey:          *room,
		SimulatorBaseURL: *simulatorURL,
		EventDataBaseURL: *eventDataURL,
		PollInterval:     *pollInterval,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	log.Printf("occupancysensor %s: watching %s via %s every %s", *sourceID, *room, *simulatorURL, *pollInterval)
	sensor.Run(ctx)
}
