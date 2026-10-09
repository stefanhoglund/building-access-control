// Command decision runs the Autonomous Decision Service process.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/config"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/decision"
)

func main() {
	addr := flag.String("addr", ":8083", "listen address")
	configPath := flag.String("config", "config/system-config.json", "path to the System Configuration file")
	eventDataURL := flag.String("eventdata-url", "http://localhost:8082", "Event/Data Service base URL")
	actuatorURL := flag.String("actuator-url", "http://localhost:8084", "Door Actuator base URL")
	simulatorURL := flag.String("simulator-url", "http://localhost:8081", "Simulator base URL")
	relockPoll := flag.Duration("relock-poll-interval", 200*time.Millisecond, "wall-clock polling cadence for the relock timer")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("decision: %v", err)
	}

	srv := decision.New(cfg, *eventDataURL, *actuatorURL, *simulatorURL)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go srv.RunRelockTicker(ctx, *relockPoll)

	log.Printf("decision: listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, srv.Handler()))
}
