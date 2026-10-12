package decision

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/config"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/contracts"
)

// These are the three required anomaly-policy scenarios
// (REQ-DEC-002/003/006, design-specification.md section 7.3), exercised
// through the real HTTP handler so the full policy — authorization,
// anomaly, and their combination into FinalAction/Alert — is proven
// together, not just the anomaly scorer in isolation (anomaly_test.go).

// decisionCapture is a minimal stand-in for the Event/Data Service and
// the Door Actuator, capturing what the Decision Service sends each.
type decisionCapture struct {
	mu            sync.Mutex
	lastDecision  contracts.AccessDecision
	commandsCount int
}

func (c *decisionCapture) eventDataHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /decisions", func(w http.ResponseWriter, r *http.Request) {
		var d contracts.AccessDecision
		_ = json.NewDecoder(r.Body).Decode(&d)
		c.mu.Lock()
		c.lastDecision = d
		c.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("POST /commands", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted) // evidence-return path, not counted here
	})
	return mux
}

func (c *decisionCapture) actuatorHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /commands", func(w http.ResponseWriter, r *http.Request) {
		c.mu.Lock()
		c.commandsCount++
		c.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

func (c *decisionCapture) snapshot() (contracts.AccessDecision, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastDecision, c.commandsCount
}

func postObservation(t *testing.T, srv *Server, obs contracts.AccessAttemptObservation) {
	t.Helper()
	body, err := json.Marshal(obs)
	if err != nil {
		t.Fatalf("marshal observation: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/decide", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/decide status = %d, want 200", rec.Code)
	}
}

func authorizedConfig() *config.Config {
	cfg := baseConfig()
	cfg.Permissions = []config.Permission{permission(config.SubjectRole, "employee", config.EffectAllow)}
	return cfg
}

func TestDecide_NormalAuthorized_AllowNoAlert(t *testing.T) {
	capture := &decisionCapture{}
	eventData := httptest.NewServer(capture.eventDataHandler())
	defer eventData.Close()
	actuatorSrv := httptest.NewServer(capture.actuatorHandler())
	defer actuatorSrv.Close()

	srv := New(authorizedConfig(), eventData.URL, actuatorSrv.URL, "http://unused")

	postObservation(t, srv, contracts.AccessAttemptObservation{
		EventID:   "event-1",
		SourceID:  "reader-1",
		ModelTime: "2026-10-08T10:00:00Z", // daytime, first attempt -> NORMAL
		CardID:    "card-1",
		DoorID:    testDoor,
		Direction: testDirection,
	})

	decision, commands := capture.snapshot()
	if decision.FinalAction != contracts.FinalActionAllow {
		t.Fatalf("final_action = %q, want ALLOW", decision.FinalAction)
	}
	if decision.AnomalyResult != contracts.AnomalyNormal {
		t.Fatalf("anomaly_result = %q, want NORMAL", decision.AnomalyResult)
	}
	if decision.Alert {
		t.Fatalf("alert = true, want false for a normal authorized attempt")
	}
	if commands != 1 {
		t.Fatalf("actuator commands received = %d, want 1 (UNLOCK issued)", commands)
	}
}

func TestDecide_AnomalousButAuthorized_AllowWithAlert(t *testing.T) {
	capture := &decisionCapture{}
	eventData := httptest.NewServer(capture.eventDataHandler())
	defer eventData.Close()
	actuatorSrv := httptest.NewServer(capture.actuatorHandler())
	defer actuatorSrv.Close()

	srv := New(authorizedConfig(), eventData.URL, actuatorSrv.URL, "http://unused")

	postObservation(t, srv, contracts.AccessAttemptObservation{
		EventID:   "event-2",
		SourceID:  "reader-1",
		ModelTime: "2026-10-08T03:00:00Z", // night -> ANOMALOUS, but still authorized
		CardID:    "card-1",
		DoorID:    testDoor,
		Direction: testDirection,
	})

	decision, commands := capture.snapshot()
	if decision.FinalAction != contracts.FinalActionAllow {
		t.Fatalf("final_action = %q, want ALLOW: anomalous behavior must not block an otherwise-authorized attempt", decision.FinalAction)
	}
	if decision.AnomalyResult != contracts.AnomalyAnomalous {
		t.Fatalf("anomaly_result = %q, want ANOMALOUS", decision.AnomalyResult)
	}
	if !decision.Alert {
		t.Fatalf("alert = false, want true for an anomalous-but-authorized attempt")
	}
	if commands != 1 {
		t.Fatalf("actuator commands received = %d, want 1: an alert must not suppress the UNLOCK", commands)
	}
}

func TestDecide_Unauthorized_DeniedRegardlessOfAnomaly(t *testing.T) {
	capture := &decisionCapture{}
	eventData := httptest.NewServer(capture.eventDataHandler())
	defer eventData.Close()
	actuatorSrv := httptest.NewServer(capture.actuatorHandler())
	defer actuatorSrv.Close()

	// No permission at all for this card's role -> REQ-AUTH-007 default
	// deny. The attempt is deliberately ALSO made at an anomalous time, to
	// prove the required invariant directly: an anomaly classification
	// must never override a DENY (design-specification.md section 11.2,
	// REQ-DEC-003).
	srv := New(baseConfig(), eventData.URL, actuatorSrv.URL, "http://unused")

	postObservation(t, srv, contracts.AccessAttemptObservation{
		EventID:   "event-3",
		SourceID:  "reader-1",
		ModelTime: "2026-10-08T03:00:00Z", // night -> ANOMALOUS
		CardID:    "card-1",
		DoorID:    testDoor,
		Direction: testDirection,
	})

	decision, commands := capture.snapshot()
	if decision.FinalAction != contracts.FinalActionDeny {
		t.Fatalf("final_action = %q, want DENY even though the attempt is also anomalous", decision.FinalAction)
	}
	if decision.AuthorizationReason != contracts.ReasonNoPermission {
		t.Fatalf("authorization_reason = %q, want %q", decision.AuthorizationReason, contracts.ReasonNoPermission)
	}
	if decision.AnomalyResult != contracts.AnomalyAnomalous {
		t.Fatalf("anomaly_result = %q, want ANOMALOUS: the anomaly classification itself must still be reported honestly", decision.AnomalyResult)
	}
	if !decision.Alert {
		t.Fatalf("alert = false, want true: DENY+ANOMALOUS still raises an alert (design-specification.md section 7.3)")
	}
	if commands != 0 {
		t.Fatalf("actuator commands received = %d, want 0: an anomaly alert must never cause an UNLOCK on a DENY", commands)
	}
}
