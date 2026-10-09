// Command eventdata runs the Event & Data Service process.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/eventdata"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/evidence"
)

func main() {
	addr := flag.String("addr", ":8082", "listen address")
	decisionURL := flag.String("decision-url", "http://localhost:8083", "Autonomous Decision Service base URL")
	evidencePath := flag.String("evidence-path", "data/evidence.jsonl", "path to the append-only evidence file")
	flag.Parse()

	if dir := filepath.Dir(*evidencePath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("eventdata: create evidence directory: %v", err)
		}
	}

	store, err := evidence.Open(*evidencePath)
	if err != nil {
		log.Fatalf("eventdata: %v", err)
	}
	defer store.Close()

	srv := eventdata.New(store, *decisionURL)
	log.Printf("eventdata: listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, srv.Handler()))
}
