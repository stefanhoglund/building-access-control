//go:build integration

// Package integration contains the real-BuildSim demonstration for the
// approved vertical slice. It is excluded from `go test ./...` by the
// integration build tag — the default suite stays fast and has no
// external-process dependency — and run explicitly:
//
//	go test -tags=integration ./test/integration/...
//
// It requires a real BuildSim binary. By default it looks for the one
// already committed under docs/internal/D7065E/buildingsim/bin/buildsim;
// set BUILDSIM_BIN to point at a different one. If the binary is missing
// or cannot run on this platform, the test skips rather than fails, since
// that is an environment limitation, not a slice defect.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
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
	cardAllow = "card-allow"

	// A109/level0 is the exact room used by doors.md's own worked
	// example; confirmed to resolve against the embedded floor plan.
	doorLevel = "level0"
	doorRoom  = "A109"

	pollInterval = 20 * time.Millisecond
)

// TestRealBuildSimALLOWAndRelock is the end-to-end demonstration against
// a real, locally running BuildSim instance (not FakeBuildSimClient). It
// proves, in order:
//
//  1. BuildSim starts with the controlled door LOCKED
//  2. an authorized card presentation produces ALLOW
//  3. the Door Actuator causes the real BuildSim door to become UNLOCKED
//  4. the Simulator independently reads the changed BuildSim state
//  5. after 10 model seconds with no open event, LOCK is issued
//  6. the real BuildSim door returns to LOCKED
//  7. the evidence store contains the linked
//     observation -> decision -> command -> outcome chain
func TestRealBuildSimALLOWAndRelock(t *testing.T) {
	buildsimURL := startRealBuildSim(t)
	seedDoor(t, buildsimURL)

	// (1) BuildSim starts with the controlled door LOCKED.
	hc := buildsim.NewHTTPClient(buildsimURL)
	state, err := hc.GetDoorState(doorID)
	if err != nil {
		t.Fatalf("read initial door state from real BuildSim: %v", err)
	}
	if state.LockState != buildsim.LockStateLocked {
		t.Fatalf("initial lock_state = %q, want LOCKED", state.LockState)
	}

	cfg, err := config.Load(filepath.Join("..", "..", "config", "system-config.json"))
	if err != nil {
		t.Fatalf("load system configuration: %v", err)
	}

	evidencePath := filepath.Join(t.TempDir(), "evidence.jsonl")
	store, err := evidence.Open(evidencePath)
	if err != nil {
		t.Fatalf("open evidence store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	modelStart, err := modeltime.Parse("2026-10-08T08:00:00Z")
	if err != nil {
		t.Fatalf("parse fixed model start time: %v", err)
	}
	clock := modeltime.NewClock(modelStart)

	// Each component gets its own *buildsim.HTTPClient pointed at the
	// same running BuildSim instance. Unlike FakeBuildSimClient, no
	// shared Go object is needed: BuildSim itself is the shared state
	// (design-specification.md section 9.6).
	simulatorLn := listen(t)
	eventDataLn := listen(t)
	decisionLn := listen(t)
	actuatorLn := listen(t)

	simulatorURL := "http://" + simulatorLn.Addr().String()
	eventDataURL := "http://" + eventDataLn.Addr().String()
	decisionURL := "http://" + decisionLn.Addr().String()
	actuatorURL := "http://" + actuatorLn.Addr().String()

	simServer := simulator.New(clock, buildsim.NewHTTPClient(buildsimURL))
	eventDataServer := eventdata.New(store, decisionURL)
	decisionServer := decision.New(cfg, eventDataURL, actuatorURL, simulatorURL)
	actuatorServer := actuator.New(buildsim.NewHTTPClient(buildsimURL), eventDataURL)

	serve(t, simulatorLn, simServer.Handler())
	serve(t, eventDataLn, eventDataServer.Handler())
	serve(t, decisionLn, decisionServer.Handler())
	serve(t, actuatorLn, actuatorServer.Handler())

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go decisionServer.RunRelockTicker(ctx, pollInterval)

	reader := &cardreader.Reader{
		ReaderID:         readerID,
		SimulatorBaseURL: simulatorURL,
		EventDataBaseURL: eventDataURL,
		PollInterval:     pollInterval,
	}
	go reader.Run(ctx)

	// Drive the scenario: present card-allow.
	postJSON(t, simulatorURL+"/scenario/present", map[string]string{
		"reader_id": readerID,
		"card_id":   cardAllow,
		"door_id":   doorID,
		"direction": direction,
	})

	// (2) ALLOW, (3) real BuildSim UNLOCKED, (4) Simulator independently
	// observes it via its own GET against BuildSim.
	var decisionRecord contracts.AccessDecision
	eventually(t, 5*time.Second, func() bool {
		_, decisionRecord, _, _ = findAllowChain(readEvidence(t, evidencePath))
		return decisionRecord.DecisionID != ""
	})
	if decisionRecord.AuthorizationResult != contracts.AuthResultAllow {
		t.Fatalf("authorization_result = %q, want ALLOW", decisionRecord.AuthorizationResult)
	}
	if decisionRecord.FinalAction != contracts.FinalActionAllow {
		t.Fatalf("final_action = %q, want ALLOW", decisionRecord.FinalAction)
	}

	eventually(t, 5*time.Second, func() bool {
		return simulatorDoorLockState(t, simulatorURL) == string(buildsim.LockStateUnlocked)
	})
	// Cross-check directly against BuildSim, independent of the
	// Simulator's own read, to prove (3) and (4) are consistent.
	directState, err := hc.GetDoorState(doorID)
	if err != nil {
		t.Fatalf("read door state from real BuildSim after unlock: %v", err)
	}
	if directState.LockState != buildsim.LockStateUnlocked {
		t.Fatalf("real BuildSim lock_state = %q, want UNLOCKED", directState.LockState)
	}

	var unlockCommand contracts.ActuatorCommand
	var unlockOutcome contracts.ActuatorOutcome
	eventually(t, 5*time.Second, func() bool {
		_, _, unlockCommand, unlockOutcome = findAllowChain(readEvidence(t, evidencePath))
		return unlockOutcome.CommandID != ""
	})
	if unlockCommand.Action != contracts.ActionUnlock {
		t.Fatalf("command.action = %q, want UNLOCK", unlockCommand.Action)
	}
	if unlockOutcome.Result != contracts.OutcomeConfirmed {
		t.Fatalf("unlock outcome.result = %q, want CONFIRMED", unlockOutcome.Result)
	}

	// (5) 10 model seconds with no open event -> LOCK issued. (6) real
	// BuildSim returns to LOCKED.
	postJSON(t, simulatorURL+"/modeltime/advance", map[string]float64{"seconds": 11})

	eventually(t, 5*time.Second, func() bool {
		return simulatorDoorLockState(t, simulatorURL) == string(buildsim.LockStateLocked)
	})
	directState, err = hc.GetDoorState(doorID)
	if err != nil {
		t.Fatalf("read door state from real BuildSim after relock: %v", err)
	}
	if directState.LockState != buildsim.LockStateLocked {
		t.Fatalf("real BuildSim lock_state after timeout = %q, want LOCKED", directState.LockState)
	}

	var notOpened contracts.DoorEvent
	var lockCommand contracts.ActuatorCommand
	var lockOutcome contracts.ActuatorOutcome
	eventually(t, 5*time.Second, func() bool {
		notOpened, lockCommand, lockOutcome = findRelockChain(readEvidence(t, evidencePath), decisionRecord.DecisionID)
		return lockOutcome.CommandID != ""
	})
	if notOpened.EventType != contracts.DoorEventNotOpened {
		t.Fatalf("door event type = %q, want DOOR_NOT_OPENED", notOpened.EventType)
	}
	if lockCommand.Action != contracts.ActionLock || lockCommand.Reason != contracts.ReasonRelockTimeout {
		t.Fatalf("relock command = %+v, want action=LOCK reason=RELOCK_TIMEOUT", lockCommand)
	}
	if lockOutcome.Result != contracts.OutcomeConfirmed {
		t.Fatalf("relock outcome.result = %q, want CONFIRMED", lockOutcome.Result)
	}

	// (7) The evidence store contains the full linked chain.
	records := readEvidence(t, evidencePath)
	observation, decisionAgain, unlockCmdAgain, unlockOutAgain := findAllowChain(records)
	if observation.EventID == "" || decisionAgain.DecisionID == "" || unlockCmdAgain.CommandID == "" || unlockOutAgain.CommandID == "" {
		t.Fatalf("incomplete ALLOW evidence chain: observation=%+v decision=%+v command=%+v outcome=%+v",
			observation, decisionAgain, unlockCmdAgain, unlockOutAgain)
	}
	if decisionAgain.EventID != observation.EventID {
		t.Fatalf("decision.event_id = %q, want %q", decisionAgain.EventID, observation.EventID)
	}
	if unlockCmdAgain.DecisionID != decisionAgain.DecisionID {
		t.Fatalf("command.decision_id = %q, want %q", unlockCmdAgain.DecisionID, decisionAgain.DecisionID)
	}
	if unlockOutAgain.CommandID != unlockCmdAgain.CommandID {
		t.Fatalf("outcome.command_id = %q, want %q", unlockOutAgain.CommandID, unlockCmdAgain.CommandID)
	}

	t.Logf("real BuildSim end-to-end chain verified: event_id=%s decision_id=%s command_id=%s",
		observation.EventID, decisionAgain.DecisionID, unlockCmdAgain.CommandID)
}

// startRealBuildSim locates and starts the real BuildSim binary on a free
// local port, waits for it to become healthy, and returns its base URL.
// It skips the test if no usable binary is found.
func startRealBuildSim(t *testing.T) string {
	t.Helper()

	binPath := os.Getenv("BUILDSIM_BIN")
	if binPath == "" {
		binPath = filepath.Join("..", "..", "docs", "internal", "D7065E", "buildingsim", "bin", "buildsim")
	}
	if info, err := os.Stat(binPath); err != nil || info.IsDir() {
		t.Skipf("real BuildSim binary not found at %s (set BUILDSIM_BIN to override): %v", binPath, err)
	}

	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	baseURL := "http://" + addr

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, binPath, "start", "--port", fmt.Sprint(port), "--log-file", os.DevNull)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		t.Skipf("starting real BuildSim binary failed (environment limitation): %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := http.Get(baseURL + "/healthz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("real BuildSim did not become healthy on %s within 5s", baseURL)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return baseURL
}

// seedDoor registers door-1 against a real room (A109/level0, the exact
// room used by doors.md's own worked example) as LOCKED/CLOSED. This is
// scenario provisioning (docs/internal/D7065E/buildingsim/docs/
// lab-quickstart.md: "each pair should keep a Go seed program or script
// that can recreate its setup"), not part of the BuildSimClient
// abstraction itself.
func seedDoor(t *testing.T, buildsimURL string) {
	t.Helper()
	door := map[string]any{
		"id":         doorID,
		"name":       "PUBLIC to EMPLOYEE door",
		"kind":       "door",
		"level":      doorLevel,
		"room":       doorRoom,
		"state":      "closed",
		"lock_state": "locked",
	}
	postJSONTo(t, buildsimURL+"/api/doors", http.MethodPut, []any{door})
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	return ln
}

func serve(t *testing.T, ln net.Listener, handler http.Handler) {
	t.Helper()
	srv := &http.Server{Handler: handler}
	go srv.Serve(ln) //nolint:errcheck // listener Close() in cleanup causes the expected "closed" error
}

func postJSON(t *testing.T, url string, body any) {
	t.Helper()
	postJSONTo(t, url, http.MethodPost, body)
}

func postJSONTo(t *testing.T, url, method string, body any) {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build %s %s: %v", method, url, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		t.Fatalf("%s %s: status %d", method, url, resp.StatusCode)
	}
}

func simulatorDoorLockState(t *testing.T, simulatorURL string) string {
	t.Helper()
	resp, err := http.Get(simulatorURL + "/doors/" + doorID + "/state")
	if err != nil {
		t.Fatalf("GET simulator door state: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET simulator door state: status %d", resp.StatusCode)
	}
	var out struct {
		LockState string `json:"lock_state"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode simulator door state: %v", err)
	}
	return out.LockState
}

func readEvidence(t *testing.T, path string) []evidence.Record {
	t.Helper()
	records, err := evidence.ReadAll(path)
	if err != nil {
		t.Fatalf("read evidence: %v", err)
	}
	return records
}

func eventually(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if check() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("condition not met within %s", timeout)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func findAllowChain(records []evidence.Record) (
	contracts.AccessAttemptObservation,
	contracts.AccessDecision,
	contracts.ActuatorCommand,
	contracts.ActuatorOutcome,
) {
	var obs contracts.AccessAttemptObservation
	var dec contracts.AccessDecision
	var cmd contracts.ActuatorCommand
	var out contracts.ActuatorOutcome
	for _, r := range records {
		switch r.RecordType {
		case contracts.RecordTypeObservation:
			var v contracts.AccessAttemptObservation
			_ = json.Unmarshal(r.Payload, &v)
			if v.CardID == cardAllow {
				obs = v
			}
		case contracts.RecordTypeDecision:
			var v contracts.AccessDecision
			_ = json.Unmarshal(r.Payload, &v)
			if obs.EventID != "" && v.EventID == obs.EventID {
				dec = v
			}
		case contracts.RecordTypeCommand:
			var v contracts.ActuatorCommand
			_ = json.Unmarshal(r.Payload, &v)
			if dec.DecisionID != "" && v.DecisionID == dec.DecisionID && v.Action == contracts.ActionUnlock {
				cmd = v
			}
		case contracts.RecordTypeOutcome:
			var v contracts.ActuatorOutcome
			_ = json.Unmarshal(r.Payload, &v)
			if cmd.CommandID != "" && v.CommandID == cmd.CommandID {
				out = v
			}
		}
	}
	return obs, dec, cmd, out
}

func findRelockChain(records []evidence.Record, decisionID string) (
	contracts.DoorEvent,
	contracts.ActuatorCommand,
	contracts.ActuatorOutcome,
) {
	var evt contracts.DoorEvent
	var cmd contracts.ActuatorCommand
	var out contracts.ActuatorOutcome
	for _, r := range records {
		switch r.RecordType {
		case contracts.RecordTypeDoorEvent:
			var v contracts.DoorEvent
			_ = json.Unmarshal(r.Payload, &v)
			if v.DecisionID == decisionID {
				evt = v
			}
		case contracts.RecordTypeCommand:
			var v contracts.ActuatorCommand
			_ = json.Unmarshal(r.Payload, &v)
			if v.DecisionID == decisionID && v.Action == contracts.ActionLock {
				cmd = v
			}
		case contracts.RecordTypeOutcome:
			var v contracts.ActuatorOutcome
			_ = json.Unmarshal(r.Payload, &v)
			if cmd.CommandID != "" && v.CommandID == cmd.CommandID {
				out = v
			}
		}
	}
	return evt, cmd, out
}
