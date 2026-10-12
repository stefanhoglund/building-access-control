// Command stressbench is the one grade-5 breaking-point experiment
// (concurrent access attempts against the single Decision Service
// pipeline): it sweeps increasing concurrency levels, firing access
// observations directly at the Event/Data Service from N concurrent
// goroutines for a fixed duration per level, and reports offered load,
// throughput, p95 decision latency, and error/drop rate at each level —
// looking for the point where the single in-process pipeline (one mutex-
// guarded evidence file, one mutex-guarded per-door pending map in the
// Decision Service) visibly degrades.
//
// This bypasses the Card Reader and the Simulator's single-reader poll
// queue on purpose: that queue is a known, separate bottleneck (one
// presentation drained per poll tick) that would dominate any
// measurement taken through it, masking the pipeline's own concurrency
// behavior. Posting directly to Event/Data Service's /observations
// endpoint is what "concurrent access attempts against the single
// decision pipeline" (the task's own suggested experiment) means here.
package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/actuator"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/buildsim"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/config"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/contracts"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/decision"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/eventdata"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/evidence"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/idgen"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/modeltime"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/simulator"
)

const (
	doorID    = "door-1"
	direction = "PUBLIC_TO_EMPLOYEE"
	cardID    = "card-allow"
)

func main() {
	levelsFlag := flag.String("levels", "1,5,10,25,50,100,200", "comma-separated concurrency levels to sweep")
	perLevelDuration := flag.Duration("duration", 2*time.Second, "how long to sustain each concurrency level")
	outDir := flag.String("out-dir", "evidence/stress", "directory for per-level evidence and the summary")
	configPath := flag.String("config", "config/system-config.json", "path to the System Configuration file")
	flag.Parse()

	var levels []int
	for _, s := range strings.Split(*levelsFlag, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			log.Fatalf("stressbench: invalid level %q: %v", s, err)
		}
		levels = append(levels, n)
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("stressbench: create output directory: %v", err)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("stressbench: load config: %v", err)
	}

	var summary []levelResult
	for _, c := range levels {
		log.Printf("stressbench: concurrency=%d, duration=%s", c, *perLevelDuration)
		r := runLevel(cfg, c, *perLevelDuration, filepath.Join(*outDir, fmt.Sprintf("level-%d", c)))
		summary = append(summary, r)
		fmt.Printf("%-12d %-14d %-14.1f %-10d %-10.2f %-10.2f %-8.2f%%\n",
			r.Concurrency, r.Sent, r.OfferedLoadRPS, r.Matched, r.ThroughputRPS, r.P95Ms, r.ErrorRatePct)
	}

	writeJSON(filepath.Join(*outDir, "summary.json"), summary)
	writeSummaryCSV(filepath.Join(*outDir, "summary.csv"), summary)

	fmt.Println("\nconcurrency  sent          offered(req/s)  matched   through(req/s) p95(ms)   error%")
	for _, r := range summary {
		fmt.Printf("%-12d %-14d %-14.1f %-10d %-10.2f %-10.2f %-8.2f%%\n",
			r.Concurrency, r.Sent, r.OfferedLoadRPS, r.Matched, r.ThroughputRPS, r.P95Ms, r.ErrorRatePct)
	}
	fmt.Printf("\nsummary: %s\n", filepath.Join(*outDir, "summary.json"))
}

type levelResult struct {
	Concurrency    int     `json:"concurrency"`
	DurationSec    float64 `json:"duration_sec"`
	Sent           int     `json:"sent"`
	Errors         int     `json:"send_errors"`
	Matched        int     `json:"matched_decisions"`
	OfferedLoadRPS float64 `json:"offered_load_rps"`
	ThroughputRPS  float64 `json:"throughput_rps"`
	P50Ms          float64 `json:"p50_ms"`
	P95Ms          float64 `json:"p95_ms"`
	MaxMs          float64 `json:"max_ms"`
	ErrorRatePct   float64 `json:"error_rate_pct"`
}

func runLevel(cfg *config.Config, concurrency int, duration time.Duration, outDir string) levelResult {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("stressbench: create %s: %v", outDir, err)
	}
	evidencePath := filepath.Join(outDir, "evidence.jsonl")
	_ = os.Remove(evidencePath)

	clock := modeltime.NewClock(time.Now().UTC())
	bs := buildsim.NewFakeClient(map[string]buildsim.DoorState{
		doorID: {LockState: buildsim.LockStateLocked, DoorPosition: buildsim.DoorPositionClosed},
	})
	store, err := evidence.Open(evidencePath)
	if err != nil {
		log.Fatalf("stressbench: open evidence store: %v", err)
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

	var sent, sendErrors int64
	var mu sync.Mutex
	stop := make(chan struct{})
	var wg sync.WaitGroup
	// A generous connection pool so the load-generating client itself is
	// never the bottleneck being measured; Go's http.DefaultTransport
	// defaults to only 2 idle connections per host, which would otherwise
	// make this benchmark client, not the system under test, the first
	// thing to degrade under concurrency.
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        2000,
			MaxIdleConnsPerHost: 2000,
			MaxConnsPerHost:     0, // unlimited
		},
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				now := time.Now().UTC()
				obs := contracts.AccessAttemptObservation{
					EventID:       idgen.New(),
					SchemaVersion: "1",
					SourceID:      "stressbench",
					ModelTime:     modeltime.Format(now),
					SourceTime:    now.Format(time.RFC3339Nano),
					CardID:        cardID,
					DoorID:        doorID,
					Direction:     direction,
				}
				payload, _ := json.Marshal(obs)
				resp, err := client.Post(eventDataURL+"/observations", "application/json", bytes.NewReader(payload))
				mu.Lock()
				sent++
				if err != nil || resp.StatusCode >= 300 {
					sendErrors++
				}
				mu.Unlock()
				if resp != nil {
					// Drain before closing: an undrained body prevents Go's
					// http.Transport from reusing the underlying
					// connection, which at high concurrency exhausts
					// ephemeral loopback ports — a benchmark-client
					// artifact, not a property of the system under test
					// (internal/httpx's PostJSON, used by every real
					// inter-service call, already drains correctly).
					_, _ = io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}
			}
		}()
	}

	time.Sleep(duration)
	close(stop)
	wg.Wait()
	time.Sleep(300 * time.Millisecond) // bounded settle for in-flight requests

	records, err := evidence.ReadAll(evidencePath)
	if err != nil {
		log.Fatalf("stressbench: read evidence: %v", err)
	}
	decisionMs, matched := computeDecisionLatencies(records)

	result := levelResult{
		Concurrency:    concurrency,
		DurationSec:    duration.Seconds(),
		Sent:           int(sent),
		Errors:         int(sendErrors),
		Matched:        matched,
		OfferedLoadRPS: float64(sent) / duration.Seconds(),
		ThroughputRPS:  float64(matched) / duration.Seconds(),
	}
	if sent > 0 {
		result.ErrorRatePct = 100 * float64(sendErrors) / float64(sent)
	}
	if len(decisionMs) > 0 {
		result.P50Ms = percentile(decisionMs, 0.50)
		result.P95Ms = percentile(decisionMs, 0.95)
		result.MaxMs = decisionMs[len(decisionMs)-1]
	}
	return result
}

func computeDecisionLatencies(records []evidence.Record) ([]float64, int) {
	type obsInfo struct{ recordedAt time.Time }
	observations := map[string]obsInfo{}
	decisionRecordedAt := map[string]time.Time{}

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
		}
	}

	var samples []float64
	for eventID, obs := range observations {
		if decAt, ok := decisionRecordedAt[eventID]; ok {
			samples = append(samples, decAt.Sub(obs.recordedAt).Seconds()*1000)
		}
	}
	sort.Float64s(samples)
	return samples, len(samples)
}

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

func mustListen() net.Listener {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("stressbench: listen: %v", err)
	}
	return ln
}

func serve(ln net.Listener, handler http.Handler) {
	srv := &http.Server{Handler: handler}
	go srv.Serve(ln) //nolint:errcheck
}

func writeJSON(path string, v any) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatalf("stressbench: write %s: %v", path, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Fatalf("stressbench: encode %s: %v", path, err)
	}
}

func writeSummaryCSV(path string, rows []levelResult) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatalf("stressbench: write %s: %v", path, err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"concurrency", "sent", "send_errors", "matched_decisions", "offered_load_rps", "throughput_rps", "p50_ms", "p95_ms", "max_ms", "error_rate_pct"})
	for _, r := range rows {
		_ = w.Write([]string{
			strconv.Itoa(r.Concurrency),
			strconv.Itoa(r.Sent),
			strconv.Itoa(r.Errors),
			strconv.Itoa(r.Matched),
			fmt.Sprintf("%.2f", r.OfferedLoadRPS),
			fmt.Sprintf("%.2f", r.ThroughputRPS),
			fmt.Sprintf("%.3f", r.P50Ms),
			fmt.Sprintf("%.3f", r.P95Ms),
			fmt.Sprintf("%.3f", r.MaxMs),
			fmt.Sprintf("%.2f", r.ErrorRatePct),
		})
	}
}
