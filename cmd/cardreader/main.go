// Command cardreader runs one Card Reader sensor process.
package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/cardreader"
)

func main() {
	readerID := flag.String("reader-id", "reader-1", "this reader's device identifier")
	simulatorURL := flag.String("simulator-url", "http://localhost:8081", "Simulator base URL")
	eventDataURL := flag.String("eventdata-url", "http://localhost:8082", "Event/Data Service base URL")
	pollInterval := flag.Duration("poll-interval", 200*time.Millisecond, "polling cadence against the Simulator")
	flag.Parse()

	r := &cardreader.Reader{
		ReaderID:         *readerID,
		SimulatorBaseURL: *simulatorURL,
		EventDataBaseURL: *eventDataURL,
		PollInterval:     *pollInterval,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	log.Printf("cardreader %s: polling %s every %s", *readerID, *simulatorURL, *pollInterval)
	r.Run(ctx)
}
