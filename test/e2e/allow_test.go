package e2e

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/buildsim"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/contracts"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/evidence"
)

// TestAllow exercises REQ-FUNC-002 (ALLOW unlocks, BuildSim reflects it,
// evidence is retained), using card-allow, whose role (employee) holds a
// role-level ALLOW permission for door-1's PUBLIC_TO_EMPLOYEE direction
// (config/system-config.json). REQ-FUNC-003 (relock timeout) is a
// separate test: TestRelockTimeout.
func TestAllow(t *testing.T) {
	s := startSlice(t)

	s.present(cardAllow)

	// REQ-FUNC-002 / REQ-BS-003: the actuator's UNLOCK is visible in
	// BuildSim, and the Simulator's own independent read confirms it
	// ("simulator observing the resulting door state").
	eventually(t, 2*time.Second, func() bool {
		return s.doorLockState() == string(buildsim.LockStateUnlocked)
	})

	var observation contracts.AccessAttemptObservation
	var decisionRecord contracts.AccessDecision
	var unlockCommand contracts.ActuatorCommand
	var unlockOutcome contracts.ActuatorOutcome

	eventually(t, 2*time.Second, func() bool {
		observation, decisionRecord, unlockCommand, unlockOutcome = findAllowChain(t, s.evidenceRecords())
		return unlockOutcome.CommandID != ""
	})

	// REQ-DEC-004/007 and REQ-TEST-002: the whole chain is linked by ID
	// and traceable from the observation through to the confirmed
	// physical outcome.
	if observation.CardID != cardAllow {
		t.Fatalf("observation.card_id = %q, want %q", observation.CardID, cardAllow)
	}
	if decisionRecord.EventID != observation.EventID {
		t.Fatalf("decision.event_id = %q, want %q", decisionRecord.EventID, observation.EventID)
	}
	if decisionRecord.AuthorizationResult != contracts.AuthResultAllow {
		t.Fatalf("authorization_result = %q, want ALLOW", decisionRecord.AuthorizationResult)
	}
	if decisionRecord.AuthorizationReason != contracts.ReasonRoleAllow {
		t.Fatalf("authorization_reason = %q, want %q", decisionRecord.AuthorizationReason, contracts.ReasonRoleAllow)
	}
	if decisionRecord.FinalAction != contracts.FinalActionAllow {
		t.Fatalf("final_action = %q, want ALLOW", decisionRecord.FinalAction)
	}
	if unlockCommand.DecisionID != decisionRecord.DecisionID {
		t.Fatalf("command.decision_id = %q, want %q", unlockCommand.DecisionID, decisionRecord.DecisionID)
	}
	if unlockCommand.Action != contracts.ActionUnlock {
		t.Fatalf("command.action = %q, want UNLOCK", unlockCommand.Action)
	}
	if unlockOutcome.CommandID != unlockCommand.CommandID {
		t.Fatalf("outcome.command_id = %q, want %q", unlockOutcome.CommandID, unlockCommand.CommandID)
	}
	if unlockOutcome.Result != contracts.OutcomeConfirmed {
		t.Fatalf("outcome.result = %q, want CONFIRMED", unlockOutcome.Result)
	}
}

func findAllowChain(t *testing.T, records []evidence.Record) (
	contracts.AccessAttemptObservation,
	contracts.AccessDecision,
	contracts.ActuatorCommand,
	contracts.ActuatorOutcome,
) {
	t.Helper()
	var obs contracts.AccessAttemptObservation
	var dec contracts.AccessDecision
	var cmd contracts.ActuatorCommand
	var out contracts.ActuatorOutcome
	for _, r := range records {
		switch r.RecordType {
		case contracts.RecordTypeObservation:
			var v contracts.AccessAttemptObservation
			mustUnmarshal(t, r.Payload, &v)
			if v.CardID == cardAllow {
				obs = v
			}
		case contracts.RecordTypeDecision:
			var v contracts.AccessDecision
			mustUnmarshal(t, r.Payload, &v)
			if obs.EventID != "" && v.EventID == obs.EventID {
				dec = v
			}
		case contracts.RecordTypeCommand:
			var v contracts.ActuatorCommand
			mustUnmarshal(t, r.Payload, &v)
			if dec.DecisionID != "" && v.DecisionID == dec.DecisionID && v.Action == contracts.ActionUnlock {
				cmd = v
			}
		case contracts.RecordTypeOutcome:
			var v contracts.ActuatorOutcome
			mustUnmarshal(t, r.Payload, &v)
			if cmd.CommandID != "" && v.CommandID == cmd.CommandID {
				out = v
			}
		}
	}
	return obs, dec, cmd, out
}

func findRelockChain(t *testing.T, records []evidence.Record, decisionID string) (
	contracts.DoorEvent,
	contracts.ActuatorCommand,
	contracts.ActuatorOutcome,
) {
	t.Helper()
	var evt contracts.DoorEvent
	var cmd contracts.ActuatorCommand
	var out contracts.ActuatorOutcome
	for _, r := range records {
		switch r.RecordType {
		case contracts.RecordTypeDoorEvent:
			var v contracts.DoorEvent
			mustUnmarshal(t, r.Payload, &v)
			if v.DecisionID == decisionID {
				evt = v
			}
		case contracts.RecordTypeCommand:
			var v contracts.ActuatorCommand
			mustUnmarshal(t, r.Payload, &v)
			if v.DecisionID == decisionID && v.Action == contracts.ActionLock {
				cmd = v
			}
		case contracts.RecordTypeOutcome:
			var v contracts.ActuatorOutcome
			mustUnmarshal(t, r.Payload, &v)
			if cmd.CommandID != "" && v.CommandID == cmd.CommandID {
				out = v
			}
		}
	}
	return evt, cmd, out
}

func mustUnmarshal(t *testing.T, data json.RawMessage, out any) {
	t.Helper()
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("unmarshal evidence payload: %v", err)
	}
}
