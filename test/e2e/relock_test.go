package e2e

import (
	"testing"
	"time"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/buildsim"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/contracts"
)

// TestRelockTimeout exercises REQ-FUNC-003: a door unlocked by an ALLOW
// decision and never opened returns to LOCKED no later than 10 model
// seconds later, with a DOOR_NOT_OPENED event and a LOCK command/outcome
// retained as evidence. Model time is advanced deterministically rather
// than slept (design-specification.md section 4.3), and the relock timer
// itself is driven by the Autonomous Decision Service's wall-clock polling
// ticker against that model time (design-specification.md section 9.3/9.7).
func TestRelockTimeout(t *testing.T) {
	s := startSlice(t)

	s.present(cardAllow)

	var decisionRecord contracts.AccessDecision
	eventually(t, 2*time.Second, func() bool {
		_, decisionRecord, _, _ = findAllowChain(t, s.evidenceRecords())
		return decisionRecord.DecisionID != ""
	})

	eventually(t, 2*time.Second, func() bool {
		return s.doorLockState() == string(buildsim.LockStateUnlocked)
	})

	// No door-open observation is produced by this slice (the
	// DoorStateSensor is out of scope), so the 10-model-second no-open
	// window always elapses.
	s.advanceModelTime(11 * time.Second)

	eventually(t, 2*time.Second, func() bool {
		return s.doorLockState() == string(buildsim.LockStateLocked)
	})

	var notOpened contracts.DoorEvent
	var lockCommand contracts.ActuatorCommand
	var lockOutcome contracts.ActuatorOutcome
	eventually(t, 2*time.Second, func() bool {
		notOpened, lockCommand, lockOutcome = findRelockChain(t, s.evidenceRecords(), decisionRecord.DecisionID)
		return lockOutcome.CommandID != ""
	})

	if notOpened.EventType != contracts.DoorEventNotOpened {
		t.Fatalf("door event type = %q, want DOOR_NOT_OPENED", notOpened.EventType)
	}
	if notOpened.DecisionID != decisionRecord.DecisionID {
		t.Fatalf("door_event.decision_id = %q, want %q", notOpened.DecisionID, decisionRecord.DecisionID)
	}
	if notOpened.DoorID != doorID {
		t.Fatalf("door_event.door_id = %q, want %q", notOpened.DoorID, doorID)
	}
	if lockCommand.Action != contracts.ActionLock || lockCommand.Reason != contracts.ReasonRelockTimeout {
		t.Fatalf("relock command = %+v, want action=LOCK reason=RELOCK_TIMEOUT", lockCommand)
	}
	if lockCommand.DecisionID != decisionRecord.DecisionID {
		t.Fatalf("relock command.decision_id = %q, want %q", lockCommand.DecisionID, decisionRecord.DecisionID)
	}
	if lockOutcome.Result != contracts.OutcomeConfirmed {
		t.Fatalf("relock outcome.result = %q, want CONFIRMED", lockOutcome.Result)
	}
	if lockOutcome.ObservedState != string(buildsim.LockStateLocked) {
		t.Fatalf("relock outcome.observed_state = %q, want LOCKED", lockOutcome.ObservedState)
	}
}
