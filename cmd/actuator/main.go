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
	buildsimURL := flag.String("buildsim-url", "http://127.0.0.1:9090", "real BuildSim base URL")
	flag.Parse()

	bs := buildsim.NewHTTPClient(*buildsimURL)

	srv := actuator.New(bs, *eventDataURL)
	log.Printf("actuator: listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, srv.Handler()))
}
