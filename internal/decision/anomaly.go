// This file is the Anomaly module (architecture.md section 5.5,
// design-specification.md section 6): logically separate from
// authorization (authz.go), even though both run inside the same
// Autonomous Decision Service process (ADR-002). It answers "is this
// behavior unusual?", never "is this access permitted?" — nothing here
// reads or writes an authResult, and its output is folded into
// AccessDecision.AnomalyResult/AnomalyReason/AnomalyScore/Alert only,
// never into AuthorizationResult or FinalAction (handleDecide enforces
// that separation structurally; see server.go).
//
// This is the deterministic rule-based baseline named as a candidate in
// design-specification.md section 12.4, not a learned model. It combines
// two weighted factors, each contributing to a summed score:
//
//   - unusual access time: the model-time hour falls outside a fixed
//     normal window (design-specification.md section 3.2's own worked
//     example: "employee normally arrives between 07:30 and 09:00" —
//     generalized here to one fixed window rather than a per-role
//     schedule, which is the simplification this baseline makes).
//   - repeated attempts: the same card is presented again within a short
//     model-time window of its previous attempt.
//
// A third candidate factor — unusual destination/zone — is intentionally
// not implemented: this project's configuration has exactly one
// controlled door, so no such factor could ever vary or be falsified by
// a test (see docs/design-specification.md section 12.4).
package decision

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/contracts"
)

// Placeholder thresholds (design-specification.md section 12.5): a
// defensible starting baseline, not values derived from an evaluation
// data set, since none exists for this project yet.
const (
	normalAccessHourStart = 7  // 07:00 model time
	normalAccessHourEnd   = 19 // 19:00 model time

	repeatedAttemptWindow = 5 * time.Second // model time

	unusualTimeWeight     = 0.6
	repeatedAttemptWeight = 0.6
	anomalyScoreThreshold = 0.5
)

const (
	anomalyReasonUnusualTime     = "UNUSUAL_ACCESS_TIME"
	anomalyReasonRepeatedAttempt = "REPEATED_ATTEMPT"
	anomalyReasonNormal          = "NORMAL"
)

// anomalyOutcome is the Anomaly module's result
// (design-specification.md section 6: "anomaly score, anomaly
// classification, reason/features").
type anomalyOutcome struct {
	classification string // contracts.AnomalyNormal | AnomalyAnomalous
	score          float64
	reason         string
}

// anomalyTracker holds the only state the Anomaly module needs: each
// card's most recent attempt time, to detect repeated attempts. This is
// transient, in-memory, per-process state (design-specification.md
// section 4: "Current decision being evaluated | Autonomous Decision
// Service") — it is lost on restart, same as the per-door control state
// in server.go, and that is an accepted, documented trade-off, not an
// oversight.
type anomalyTracker struct {
	mu          sync.Mutex
	lastAttempt map[string]time.Time // card_id -> model time of last attempt
}

func newAnomalyTracker() *anomalyTracker {
	return &anomalyTracker{lastAttempt: make(map[string]time.Time)}
}

// score evaluates cardID's attempt at modelTime. It is not given, and
// does not need, the authorization result: anomaly detection answers a
// question entirely independent of permission (design-specification.md
// section 6, "It does not determine whether access is authorized").
func (t *anomalyTracker) score(cardID string, modelTime time.Time) anomalyOutcome {
	var score float64
	var reasons []string

	hour := modelTime.UTC().Hour()
	if hour < normalAccessHourStart || hour >= normalAccessHourEnd {
		score += unusualTimeWeight
		reasons = append(reasons, anomalyReasonUnusualTime)
	}

	t.mu.Lock()
	previous, seen := t.lastAttempt[cardID]
	t.lastAttempt[cardID] = modelTime
	t.mu.Unlock()

	if seen {
		delta := modelTime.Sub(previous)
		if delta >= 0 && delta < repeatedAttemptWindow {
			score += repeatedAttemptWeight
			reasons = append(reasons, anomalyReasonRepeatedAttempt)
		}
	}

	if score < anomalyScoreThreshold {
		return anomalyOutcome{classification: contracts.AnomalyNormal, score: score, reason: anomalyReasonNormal}
	}
	sort.Strings(reasons) // deterministic ordering (design-specification.md section 5/7.5: reproducible results)
	return anomalyOutcome{
		classification: contracts.AnomalyAnomalous,
		score:          score,
		reason:         strings.Join(reasons, ","),
	}
}
