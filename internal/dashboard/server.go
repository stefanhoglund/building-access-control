// Package dashboard implements the Dashboard Service (REQ-VIS-001,
// architecture.md section 5.7). It is a read-only view over the
// Historical Evidence Store — the same file the Event/Data Service is the
// sole writer of (ADR-005) — rendered as plain server-side HTML with a
// periodic auto-refresh. No frontend framework, no WebSocket, no
// database: the evidence file already holds everything this view needs,
// and re-reading it on each request is simple and sufficient at this
// project's scale.
//
// It does not query BuildSim, the Simulator, or the Decision Service
// directly: container.d2 only ever drew the Dashboard as a consumer of
// the Event/Data Service and the Historical Evidence Store, and reading
// the evidence file directly is a faithful, minimal implementation of
// exactly that edge, not a new architectural path.
package dashboard

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/contracts"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/evidence"
)

// Server is the Dashboard's HTTP handler.
type Server struct {
	evidencePath string
	historyLimit int
}

// New creates a Dashboard server reading evidencePath on every request.
// historyLimit bounds how many recent evidence records are listed.
func New(evidencePath string, historyLimit int) *Server {
	return &Server{evidencePath: evidencePath, historyLimit: historyLimit}
}

// Handler returns the http.Handler for this server.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleIndex)
	return mux
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	records, err := evidence.ReadAll(s.evidencePath)
	if err != nil {
		http.Error(w, "reading evidence: "+err.Error(), http.StatusInternalServerError)
		return
	}

	vm := buildViewModel(records, s.historyLimit)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pageTemplate.Execute(w, vm); err != nil {
		http.Error(w, "rendering dashboard: "+err.Error(), http.StatusInternalServerError)
	}
}

// --- view model ------------------------------------------------------------

type viewModel struct {
	GeneratedAt string
	HasRecords  bool
	Door        doorView
	Decision    decisionView
	Occupancy   occupancyView
	Health      []healthView
	Recent      []recentView
}

type doorView struct {
	DoorID           string
	LockState        string
	LockStateAsOf    string
	Verification     string
	DoorPosition     string
	DoorPositionAsOf string
}

type decisionView struct {
	FinalAction         string
	AuthorizationReason string
	AnomalyResult       string
	DoorID              string
	ModelTime           string
	RecordedAt          string
}

type occupancyView struct {
	RoomKey string
	Count   int
	AsOf    string
}

type healthView struct {
	Component string
	LastSeen  string
	Note      string
}

type recentView struct {
	RecordType string
	RecordedAt string
	Summary    string
}

func buildViewModel(records []evidence.Record, historyLimit int) viewModel {
	vm := viewModel{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		HasRecords:  len(records) > 0,
		Door:        doorView{LockState: "UNKNOWN", DoorPosition: "UNKNOWN", Verification: "UNKNOWN"},
	}

	var readerLastSeen, doorSensorLastSeen, occSensorLastSeen, decisionLastSeen, actuatorLastSeen string

	for _, r := range records {
		switch r.RecordType {
		case contracts.RecordTypeObservation:
			var v contracts.AccessAttemptObservation
			if json.Unmarshal(r.Payload, &v) == nil {
				readerLastSeen = r.RecordedAt
			}
		case contracts.RecordTypeDecision:
			var v contracts.AccessDecision
			if json.Unmarshal(r.Payload, &v) == nil {
				decisionLastSeen = r.RecordedAt
				vm.Decision = decisionView{
					FinalAction:         v.FinalAction,
					AuthorizationReason: v.AuthorizationReason,
					AnomalyResult:       v.AnomalyResult,
					DoorID:              v.DoorID,
					ModelTime:           v.ModelTime,
					RecordedAt:          r.RecordedAt,
				}
			}
		case contracts.RecordTypeOutcome:
			var v contracts.ActuatorOutcome
			if json.Unmarshal(r.Payload, &v) == nil {
				actuatorLastSeen = r.RecordedAt
				if v.Result == contracts.OutcomeConfirmed {
					vm.Door.DoorID = v.DoorID
					vm.Door.LockState = v.ObservedState
					vm.Door.LockStateAsOf = r.RecordedAt
					vm.Door.Verification = v.Result
				} else {
					// An unverified outcome still updates verification
					// status (REQ-RES-002) without claiming a lock_state
					// that was never confirmed.
					vm.Door.Verification = v.Result
				}
			}
		case contracts.RecordTypeDoorStateObservation:
			var v contracts.DoorStateObservation
			if json.Unmarshal(r.Payload, &v) == nil {
				doorSensorLastSeen = r.RecordedAt
				vm.Door.DoorPosition = v.DoorPosition
				vm.Door.DoorPositionAsOf = r.RecordedAt
			}
		case contracts.RecordTypeOccupancyObservation:
			var v contracts.OccupancyObservation
			if json.Unmarshal(r.Payload, &v) == nil {
				occSensorLastSeen = r.RecordedAt
				vm.Occupancy = occupancyView{RoomKey: v.RoomKey, Count: v.Count, AsOf: r.RecordedAt}
			}
		}
	}

	addHealth := func(component, lastSeen, note string) {
		if lastSeen == "" {
			vm.Health = append(vm.Health, healthView{Component: component, LastSeen: "never", Note: note})
			return
		}
		vm.Health = append(vm.Health, healthView{Component: component, LastSeen: lastSeen, Note: note})
	}
	addHealth("Card Reader (reader-1)", readerLastSeen, "last access-attempt observation")
	addHealth("DoorStateSensor (doorstatesensor-1)", doorSensorLastSeen, "last door_position observation")
	addHealth("OccupancySensor (occupancysensor-1)", occSensorLastSeen, "last occupancy observation")
	addHealth("Autonomous Decision Service", decisionLastSeen, "last AccessDecision recorded")
	addHealth("Door Actuator", actuatorLastSeen, "last ActuatorOutcome recorded")
	// The Simulator and Event/Data Service have no direct health signal
	// here: the Simulator never itself appears as a record source (it
	// causes state, it does not report observing it — section 4.1), and
	// the Event/Data Service is the process doing this writing, so any
	// non-empty evidence file already implies it is, or recently was,
	// alive. Shown as a note rather than a fabricated timestamp.
	vm.Health = append(vm.Health,
		healthView{Component: "Simulator", LastSeen: "n/a", Note: "causes state, does not report observing it (design-specification.md §4.1) — not independently inferable from evidence"},
		healthView{Component: "Event/Data Service", LastSeen: "n/a", Note: "implied alive by this page having any data at all (it is the sole writer, ADR-005)"},
	)

	start := len(records) - historyLimit
	if start < 0 {
		start = 0
	}
	for i := len(records) - 1; i >= start; i-- {
		r := records[i]
		vm.Recent = append(vm.Recent, recentView{
			RecordType: r.RecordType,
			RecordedAt: r.RecordedAt,
			Summary:    summarize(r),
		})
	}

	return vm
}

func summarize(r evidence.Record) string {
	switch r.RecordType {
	case contracts.RecordTypeObservation:
		var v contracts.AccessAttemptObservation
		if json.Unmarshal(r.Payload, &v) == nil {
			return "card=" + v.CardID + " door=" + v.DoorID + " direction=" + v.Direction
		}
	case contracts.RecordTypeDecision:
		var v contracts.AccessDecision
		if json.Unmarshal(r.Payload, &v) == nil {
			s := v.FinalAction + " (" + v.AuthorizationReason + ")"
			if v.AnomalyResult != "" && v.AnomalyResult != "NOT_EVALUATED" {
				s += " anomaly=" + v.AnomalyResult
			}
			return s
		}
	case contracts.RecordTypeCommand:
		var v contracts.ActuatorCommand
		if json.Unmarshal(r.Payload, &v) == nil {
			return v.Action + " door=" + v.DoorID + " reason=" + v.Reason
		}
	case contracts.RecordTypeOutcome:
		var v contracts.ActuatorOutcome
		if json.Unmarshal(r.Payload, &v) == nil {
			return v.Action + " -> " + v.Result + " observed=" + v.ObservedState
		}
	case contracts.RecordTypeDoorEvent:
		var v contracts.DoorEvent
		if json.Unmarshal(r.Payload, &v) == nil {
			return v.EventType + " door=" + v.DoorID
		}
	case contracts.RecordTypeDoorStateObservation:
		var v contracts.DoorStateObservation
		if json.Unmarshal(r.Payload, &v) == nil {
			return "door=" + v.DoorID + " position=" + v.DoorPosition
		}
	case contracts.RecordTypeOccupancyObservation:
		var v contracts.OccupancyObservation
		if json.Unmarshal(r.Payload, &v) == nil {
			return v.RoomKey + " count=" + strconv.Itoa(v.Count)
		}
	}
	return string(r.Payload)
}

// --- template ----------------------------------------------------------

var pageTemplate = template.Must(template.New("dashboard").Parse(`<!doctype html>
<html>
<head>
<meta charset="utf-8">
<meta http-equiv="refresh" content="2">
<title>Access Control Dashboard</title>
<style>
  body { font-family: -apple-system, system-ui, sans-serif; margin: 2em; background: #0b0f14; color: #e6edf3; }
  h1 { font-size: 1.4em; }
  h2 { font-size: 1.1em; margin-top: 1.5em; border-bottom: 1px solid #30363d; padding-bottom: 0.3em; }
  .cards { display: flex; gap: 1em; flex-wrap: wrap; }
  .card { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 1em 1.4em; min-width: 220px; }
  .label { color: #8b949e; font-size: 0.8em; text-transform: uppercase; letter-spacing: 0.05em; }
  .value { font-size: 1.6em; font-weight: 600; margin: 0.2em 0; }
  .allow { color: #3fb950; }
  .deny { color: #f85149; }
  .locked { color: #f0883e; }
  .unlocked { color: #3fb950; }
  .confirmed { color: #3fb950; }
  .unverified { color: #f85149; }
  .sub { color: #8b949e; font-size: 0.85em; }
  table { border-collapse: collapse; width: 100%; font-size: 0.9em; }
  th, td { text-align: left; padding: 0.3em 0.6em; border-bottom: 1px solid #21262d; }
  th { color: #8b949e; font-weight: 500; }
  tr:hover { background: #161b22; }
  .mono { font-family: ui-monospace, monospace; }
</style>
</head>
<body>
<h1>Resilient Anomaly-Aware Access Control — Dashboard</h1>
<p class="sub">generated {{.GeneratedAt}} &middot; auto-refreshes every 2s &middot; source: evidence store (read-only)</p>

{{if not .HasRecords}}
<p>No evidence recorded yet. Present a card to generate activity.</p>
{{end}}

<h2>Current State</h2>
<div class="cards">
  <div class="card">
    <div class="label">Door {{.Door.DoorID}} &middot; lock_state</div>
    <div class="value {{if eq .Door.LockState "LOCKED"}}locked{{else if eq .Door.LockState "UNLOCKED"}}unlocked{{end}}">{{.Door.LockState}}</div>
    <div class="sub">as of {{.Door.LockStateAsOf}}</div>
  </div>
  <div class="card">
    <div class="label">door_position</div>
    <div class="value">{{.Door.DoorPosition}}</div>
    <div class="sub">as of {{.Door.DoorPositionAsOf}}</div>
  </div>
  <div class="card">
    <div class="label">verification</div>
    <div class="value {{if eq .Door.Verification "CONFIRMED"}}confirmed{{else if eq .Door.Verification "UNVERIFIED"}}unverified{{end}}">{{.Door.Verification}}</div>
  </div>
  <div class="card">
    <div class="label">latest decision</div>
    <div class="value {{if eq .Decision.FinalAction "ALLOW"}}allow{{else if eq .Decision.FinalAction "DENY"}}deny{{end}}">{{.Decision.FinalAction}}</div>
    <div class="sub">reason: {{.Decision.AuthorizationReason}}{{if and .Decision.AnomalyResult (ne .Decision.AnomalyResult "NOT_EVALUATED")}} &middot; anomaly: {{.Decision.AnomalyResult}}{{end}}</div>
    <div class="sub">{{.Decision.RecordedAt}}</div>
  </div>
  <div class="card">
    <div class="label">destination occupancy &middot; {{.Occupancy.RoomKey}}</div>
    <div class="value">{{.Occupancy.Count}}</div>
    <div class="sub">as of {{.Occupancy.AsOf}}</div>
  </div>
</div>

<h2>Component Health (last-seen, derived from evidence)</h2>
<table>
<tr><th>Component</th><th>Last seen</th><th>Note</th></tr>
{{range .Health}}
<tr><td>{{.Component}}</td><td class="mono">{{.LastSeen}}</td><td class="sub">{{.Note}}</td></tr>
{{end}}
</table>

<h2>Recent Evidence ({{len .Recent}} shown)</h2>
<table>
<tr><th>Recorded at</th><th>Type</th><th>Summary</th></tr>
{{range .Recent}}
<tr><td class="mono">{{.RecordedAt}}</td><td>{{.RecordType}}</td><td>{{.Summary}}</td></tr>
{{end}}
</table>

</body>
</html>
`))
