// Command dashboard runs the Dashboard Service (REQ-VIS-001).
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/dashboard"
)

func main() {
	addr := flag.String("addr", ":8085", "listen address")
	evidencePath := flag.String("evidence-path", "data/evidence.jsonl", "path to the append-only evidence file")
	historyLimit := flag.Int("history-limit", 25, "number of recent evidence records to show")
	flag.Parse()

	srv := dashboard.New(*evidencePath, *historyLimit)
	log.Printf("dashboard: listening on %s (open http://localhost%s/)", *addr, *addr)
	log.Fatal(http.ListenAndServe(*addr, srv.Handler()))
}
