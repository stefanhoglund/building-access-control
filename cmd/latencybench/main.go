// Command latencybench measures access-decision latency (REQ-PERF-001)
// and end-to-end actuation-confirmation latency, both wall-clock, by
// driving N real presentations through the real five-component pipeline
// (in-process, FakeBuildSimClient — BuildSim's own responsiveness is not
// what REQ-PERF-001 is about) and then computing statistics directly from
// the resulting evidence records' recorded_at timestamps, which already
// carry exactly the measurement points needed:
//
//	access decision latency =
//	    decision.recorded_at - observation.recorded_at
//	    (wall-clock time from Event/Data receiving the access observation
//	    to Event/Data receiving the resulting AccessDecision)
//
//	end-to-end actuation-confirmation latency =
//	    outcome.recorded_at - observation.recorded_at
//	    (wall-clock time from the same observation to the Door Actuator's
//	    confirmed outcome being received by Event/Data)
//
// No production code is instrumented or modified to produce these
// numbers: both are already implicit in the evidence envelope every
// record carries (docs/data-contracts.md section 8).
package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/actuator"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/buildsim"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/cardreader"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/config"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/contracts"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/decision"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/eventdata"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/evidence"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/modeltime"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/simulator"
)

const (
	doorID    = "door-1"
	readerID  = "reader-1"
	direction = "PUBLIC_TO_EMPLOYEE"
	cardID    = "card-allow"
)

func main() {
	n := flag.Int("n", 200, "number of access attempts to drive")
	interval := flag.Duration("interval", 20*time.Millisecond, "wall-clock gap between presentations (nominal load, not a stress test)")
	outDir := flag.String("out-dir", "evidence/latency", "directory for evidence.jsonl, results.json, samples.csv")
	configPath := flag.String("config", "config/system-config.json", "path to the System Configuration file")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("latencybench: create output directory: %v", err)
	}
	evidencePath := filepath.Join(*outDir, "evidence.jsonl")
	_ = os.Remove(evidencePath) // one clean run, not an accumulating log

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("latencybench: load config: %v", err)
	}

	clock := modeltime.NewClock(time.Now().UTC())
	bs := buildsim.NewFakeClient(map[string]buildsim.DoorState{
		doorID: {LockState: buildsim.LockStateLocked, DoorPosition: buildsim.DoorPositionClosed},
	})

	store, err := evidence.Open(evidencePath)
	if err != nil {
		log.Fatalf("latencybench: open evidence store: %v", err)
	}
	defer store.Close()

	simulatorLn := mustListen()
	eventDataLn := mustListen()
	decisionLn := mustListen()
	actuatorLn := mustListen()

	simulatorURL := "http://" + simulatorLn.Addr().String()
	eventDataURL := "http://" + eventDataLn.Addr().String()
	decisionURL := "http://" + decisionLn.Addr().String()
	actuatorURL := "http://" + actuatorLn.Addr().String()

	// Crossing disabled: this benchmark measures decision/actuation
	// latency, not the successful-passage sequence, and disabling it
	// keeps each attempt's evidence to exactly one UNLOCK command, making
	// the per-attempt chain unambiguous to parse.
	simServer := simulator.New(clock, bs, simulator.CrossingConfig{}, nil)
	eventDataServer := eventdata.New(store, decisionURL)
	decisionServer := decision.New(cfg, eventDataURL, actuatorURL, simulatorURL)
	actuatorServer := actuator.New(bs, eventDataURL)

	serve(simulatorLn, simServer.Handler())
	serve(eventDataLn, eventDataServer.Handler())
	serve(decisionLn, decisionServer.Handler())
	serve(actuatorLn, actuatorServer.Handler())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go decisionServer.RunRelockTicker(ctx, 20*time.Millisecond)

	reader := &cardreader.Reader{
		ReaderID:         readerID,
		SimulatorBaseURL: simulatorURL,
		EventDataBaseURL: eventDataURL,
		PollInterval:     5 * time.Millisecond,
	}
	go reader.Run(ctx)

	log.Printf("latencybench: driving %d presentations, %s apart", *n, *interval)
	for i := 0; i < *n; i++ {
		present(simulatorURL)
		time.Sleep(*interval)
	}

	// Bounded settle time for the last few attempts' reader poll + full
	// pipeline to finish, before reading final results.
	time.Sleep(500 * time.Millisecond)

	records, err := evidence.ReadAll(evidencePath)
	if err != nil {
		log.Fatalf("latencybench: read evidence: %v", err)
	}

	decisionSamples, actuationSamples, matched := computeLatencies(records)
	log.Printf("latencybench: %d attempts driven, %d decisions matched, %d actuation outcomes matched", *n, matched.decisions, matched.outcomes)

	report := report{
		Definition: definition{
			DecisionLatency:  "wall-clock time from Event/Data receiving the access observation (observation.recorded_at) to Event/Data receiving the resulting AccessDecision (decision.recorded_at)",
			ActuationLatency: "wall-clock time from the same observation (observation.recorded_at) to Event/Data receiving the Door Actuator's outcome (outcome.recorded_at), end-to-end through decision and command issuance",
			Clock:            "wall-clock (not model time)",
			Unit:             "milliseconds",
		},
		DecisionLatencyMs:  summarize(decisionSamples),
		ActuationLatencyMs: summarize(actuationSamples),
	}

	writeJSON(filepath.Join(*outDir, "results.json"), report)
	writeCSV(filepath.Join(*outDir, "samples.csv"), decisionSamples, actuationSamples)

	fmt.Printf("\n%-32s %8s %8s %8s %8s %8s\n", "metric", "n", "min", "p50", "p95", "max")
	printRow("decision latency (ms)", report.DecisionLatencyMs)
	printRow("actuation confirm latency (ms)", report.ActuationLatencyMs)
	fmt.Printf("\nraw results: %s\n", filepath.Join(*outDir, "results.json"))
	fmt.Printf("raw samples: %s\n", filepath.Join(*outDir, "samples.csv"))
	fmt.Printf("evidence:    %s\n", evidencePath)
}

func present(simulatorURL string) {
	body := map[string]string{
		"reader_id": readerID,
		"card_id":   cardID,
		"door_id":   doorID,
		"direction": direction,
	}
	payload, _ := json.Marshal(body)
	resp, err := http.Post(simulatorURL+"/scenario/present", "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Printf("latencybench: present: %v", err)
		return
	}
	resp.Body.Close()
}

type matchCounts struct {
	decisions int
	outcomes  int
}

// computeLatencies pairs each observation's event_id with its decision
// (by event_id) and, if one exists, the UNLOCK command issued for that
// decision_id and that command's outcome.
func computeLatencies(records []evidence.Record) (decisionMs, actuationMs []float64, matched matchCounts) {
	type obsInfo struct{ recordedAt time.Time }
	observations := map[string]obsInfo{}
	decisionRecordedAt := map[string]time.Time{} // event_id -> decision recorded_at
	decisionToCommand := map[string]string{}     // decision_id -> command_id (UNLOCK only)
	outcomeRecordedAt := map[string]time.Time{}  // command_id -> outcome recorded_at

	for _, r := range records {
		recordedAt, err := time.Parse(time.RFC3339Nano, r.RecordedAt)
		if err != nil {
			continue
		}
		switch r.RecordType {
		case contracts.RecordTypeObservation:
			var v contracts.AccessAttemptObservation
			if json.Unmarshal(r.Payload, &v) == nil {
				observations[v.EventID] = obsInfo{recordedAt: recordedAt}
			}
		case contracts.RecordTypeDecision:
			var v contracts.AccessDecision
			if json.Unmarshal(r.Payload, &v) == nil {
				decisionRecordedAt[v.EventID] = recordedAt
			}
		case contracts.RecordTypeCommand:
			var v contracts.ActuatorCommand
			if json.Unmarshal(r.Payload, &v) == nil && v.Action == contracts.ActionUnlock {
				decisionToCommand[v.DecisionID] = v.CommandID
			}
		case contracts.RecordTypeOutcome:
			var v contracts.ActuatorOutcome
			if json.Unmarshal(r.Payload, &v) == nil {
				outcomeRecordedAt[v.CommandID] = recordedAt
			}
		}
	}

	// Need decision_id per event_id to chase the command/outcome chain;
	// re-scan decisions for that mapping (kept separate from the
	// recorded_at map above for clarity).
	eventToDecisionID := map[string]string{}
	for _, r := range records {
		if r.RecordType != contracts.RecordTypeDecision {
			continue
		}
		var v contracts.AccessDecision
		if json.Unmarshal(r.Payload, &v) == nil {
			eventToDecisionID[v.EventID] = v.DecisionID
		}
	}

	for eventID, obs := range observations {
		decAt, ok := decisionRecordedAt[eventID]
		if !ok {
			continue
		}
		matched.decisions++
		decisionMs = append(decisionMs, decAt.Sub(obs.recordedAt).Seconds()*1000)

		decisionID := eventToDecisionID[eventID]
		commandID, ok := decisionToCommand[decisionID]
		if !ok {
			continue
		}
		outAt, ok := outcomeRecordedAt[commandID]
		if !ok {
			continue
		}
		matched.outcomes++
		actuationMs = append(actuationMs, outAt.Sub(obs.recordedAt).Seconds()*1000)
	}

	sort.Float64s(decisionMs)
	sort.Float64s(actuationMs)
	return decisionMs, actuationMs, matched
}

type definition struct {
	DecisionLatency  string `json:"decision_latency"`
	ActuationLatency string `json:"actuation_confirmation_latency"`
	Clock            string `json:"clock"`
	Unit             string `json:"unit"`
}

type stats struct {
	SampleCount int       `json:"sample_count"`
	Min         float64   `json:"min"`
	P50         float64   `json:"p50"`
	P95         float64   `json:"p95"`
	Max         float64   `json:"max"`
	Samples     []float64 `json:"samples_ms"`
}

type report struct {
	Definition         definition `json:"metric_definition"`
	DecisionLatencyMs  stats      `json:"decision_latency_ms"`
	ActuationLatencyMs stats      `json:"actuation_confirmation_latency_ms"`
}

func summarize(sorted []float64) stats {
	if len(sorted) == 0 {
		return stats{}
	}
	return stats{
		SampleCount: len(sorted),
		Min:         sorted[0],
		P50:         percentile(sorted, 0.50),
		P95:         percentile(sorted, 0.95),
		Max:         sorted[len(sorted)-1],
		Samples:     sorted,
	}
}

// percentile uses the nearest-rank method on an already-sorted slice.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p*float64(len(sorted)-1) + 0.5)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func printRow(name string, s stats) {
	fmt.Printf("%-32s %8d %8.2f %8.2f %8.2f %8.2f\n", name, s.SampleCount, s.Min, s.P50, s.P95, s.Max)
}

func writeJSON(path string, v any) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatalf("latencybench: write %s: %v", path, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Fatalf("latencybench: encode %s: %v", path, err)
	}
}

func writeCSV(path string, decisionMs, actuationMs []float64) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatalf("latencybench: write %s: %v", path, err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"index", "decision_latency_ms", "actuation_confirmation_latency_ms"})
	maxLen := len(decisionMs)
	if len(actuationMs) > maxLen {
		maxLen = len(actuationMs)
	}
	for i := 0; i < maxLen; i++ {
		row := []string{fmt.Sprint(i)}
		if i < len(decisionMs) {
			row = append(row, fmt.Sprintf("%.3f", decisionMs[i]))
		} else {
			row = append(row, "")
		}
		if i < len(actuationMs) {
			row = append(row, fmt.Sprintf("%.3f", actuationMs[i]))
		} else {
			row = append(row, "")
		}
		_ = w.Write(row)
	}
}

func mustListen() net.Listener {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("latencybench: listen: %v", err)
	}
	return ln
}

func serve(ln net.Listener, handler http.Handler) {
	srv := &http.Server{Handler: handler}
	go srv.Serve(ln) //nolint:errcheck
}
