// Command doorstatesensor runs one DoorStateSensor process.
package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/buildsim"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/doorstatesensor"
)

func main() {
	sourceID := flag.String("source-id", "doorstatesensor-1", "this device's identifier")
	doorID := flag.String("door-id", "door-1", "the one controlled door this instance watches")
	buildsimURL := flag.String("buildsim-url", "http://127.0.0.1:9090", "real BuildSim base URL")
	simulatorURL := flag.String("simulator-url", "http://localhost:8081", "Simulator base URL (model time only)")
	eventDataURL := flag.String("eventdata-url", "http://localhost:8082", "Event/Data Service base URL")
	pollInterval := flag.Duration("poll-interval", 50*time.Millisecond, "polling cadence against BuildSim")
	flag.Parse()

	sensor := &doorstatesensor.Sensor{
		SourceID:         *sourceID,
		DoorID:           *doorID,
		BuildSim:         buildsim.NewHTTPClient(*buildsimURL),
		SimulatorBaseURL: *simulatorURL,
		EventDataBaseURL: *eventDataURL,
		PollInterval:     *pollInterval,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	log.Printf("doorstatesensor %s: watching %s via %s every %s", *sourceID, *doorID, *buildsimURL, *pollInterval)
	sensor.Run(ctx)
}
