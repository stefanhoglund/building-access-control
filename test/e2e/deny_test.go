package e2e

import (
	"testing"
	"time"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/buildsim"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/contracts"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/evidence"
)

// TestDeny exercises REQ-FUNC-001 (DENY, door stays locked, no crossing)
// via REQ-AUTH-007's default-deny path, using card-deny, whose role
// (visitor) has no permission entry at all for door-1
// (config/system-config.json).
func TestDeny(t *testing.T) {
	s := startSlice(t)

	s.present(cardDeny)

	var observation contracts.AccessAttemptObservation
	var decisionRecord contracts.AccessDecision
	eventually(t, 2*time.Second, func() bool {
		observation, decisionRecord = findDenyChain(t, s.evidenceRecords())
		return decisionRecord.DecisionID != ""
	})

	if observation.CardID != cardDeny {
		t.Fatalf("observation.card_id = %q, want %q", observation.CardID, cardDeny)
	}
	if decisionRecord.EventID != observation.EventID {
		t.Fatalf("decision.event_id = %q, want %q", decisionRecord.EventID, observation.EventID)
	}
	if decisionRecord.AuthorizationResult != contracts.AuthResultDeny {
		t.Fatalf("authorization_result = %q, want DENY", decisionRecord.AuthorizationResult)
	}
	if decisionRecord.AuthorizationReason != contracts.ReasonNoPermission {
		t.Fatalf("authorization_reason = %q, want %q", decisionRecord.AuthorizationReason, contracts.ReasonNoPermission)
	}
	if decisionRecord.FinalAction != contracts.FinalActionDeny {
		t.Fatalf("final_action = %q, want DENY", decisionRecord.FinalAction)
	}

	// REQ-FUNC-001: the door remains locked throughout — not merely at
	// this instant, but because no command was ever issued for this
	// decision.
	if got := s.doorLockState(); got != string(buildsim.LockStateLocked) {
		t.Fatalf("door lock_state = %q, want LOCKED", got)
	}
	for _, r := range s.evidenceRecords() {
		if r.RecordType != contracts.RecordTypeCommand {
			continue
		}
		var cmd contracts.ActuatorCommand
		mustUnmarshal(t, r.Payload, &cmd)
		if cmd.DecisionID == decisionRecord.DecisionID {
			t.Fatalf("an ActuatorCommand was issued for a DENY decision: %+v", cmd)
		}
	}
}

func findDenyChain(t *testing.T, records []evidence.Record) (
	contracts.AccessAttemptObservation,
	contracts.AccessDecision,
) {
	t.Helper()
	var obs contracts.AccessAttemptObservation
	var dec contracts.AccessDecision
	for _, r := range records {
		switch r.RecordType {
		case contracts.RecordTypeObservation:
			var v contracts.AccessAttemptObservation
			mustUnmarshal(t, r.Payload, &v)
			if v.CardID == cardDeny {
				obs = v
			}
		case contracts.RecordTypeDecision:
			var v contracts.AccessDecision
			mustUnmarshal(t, r.Payload, &v)
			if obs.EventID != "" && v.EventID == obs.EventID {
				dec = v
			}
		}
	}
	return obs, dec
}
