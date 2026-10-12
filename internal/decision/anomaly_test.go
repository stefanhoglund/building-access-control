package decision

import (
	"testing"
	"time"
)

// These are the anomaly module's own unit tests (anomaly.go), independent
// of authorization — see server_decide_test.go for the three required
// end-to-end scenarios (normal authorized, anomalous-but-authorized,
// unauthorized) that prove the policy matrix, not just the scorer alone.

func TestAnomaly_NormalDaytimeFirstAttempt_Normal(t *testing.T) {
	tr := newAnomalyTracker()
	daytime := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC) // 10:00, within 07-19

	got := tr.score("card-1", daytime)

	if got.classification != "NORMAL" {
		t.Fatalf("classification = %q, want NORMAL: %+v", got.classification, got)
	}
	if got.score != 0 {
		t.Fatalf("score = %v, want 0", got.score)
	}
}

func TestAnomaly_NightTime_Anomalous(t *testing.T) {
	tr := newAnomalyTracker()
	night := time.Date(2026, 10, 8, 3, 0, 0, 0, time.UTC) // 03:00, outside 07-19

	got := tr.score("card-1", night)

	if got.classification != "ANOMALOUS" {
		t.Fatalf("classification = %q, want ANOMALOUS: %+v", got.classification, got)
	}
	if got.reason != anomalyReasonUnusualTime {
		t.Fatalf("reason = %q, want %q", got.reason, anomalyReasonUnusualTime)
	}
}

func TestAnomaly_RepeatedAttemptWithinWindow_Anomalous(t *testing.T) {
	tr := newAnomalyTracker()
	t1 := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(2 * time.Second) // within the 5s window

	first := tr.score("card-1", t1)
	if first.classification != "NORMAL" {
		t.Fatalf("first attempt classification = %q, want NORMAL", first.classification)
	}

	second := tr.score("card-1", t2)
	if second.classification != "ANOMALOUS" {
		t.Fatalf("second attempt classification = %q, want ANOMALOUS: %+v", second.classification, second)
	}
	if second.reason != anomalyReasonRepeatedAttempt {
		t.Fatalf("reason = %q, want %q", second.reason, anomalyReasonRepeatedAttempt)
	}
}

func TestAnomaly_RepeatedAttemptOutsideWindow_Normal(t *testing.T) {
	tr := newAnomalyTracker()
	t1 := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(30 * time.Second) // outside the 5s window

	tr.score("card-1", t1)
	second := tr.score("card-1", t2)

	if second.classification != "NORMAL" {
		t.Fatalf("classification = %q, want NORMAL: %+v", second.classification, second)
	}
}

func TestAnomaly_DifferentCards_DoNotAffectEachOther(t *testing.T) {
	tr := newAnomalyTracker()
	t1 := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)

	tr.score("card-1", t1)
	got := tr.score("card-2", t1.Add(time.Second)) // different card, same instant

	if got.classification != "NORMAL" {
		t.Fatalf("classification = %q, want NORMAL: a different card's attempt must not look repeated", got.classification)
	}
}

func TestAnomaly_BothFactors_ScoreAccumulatesAndReasonListsBoth(t *testing.T) {
	tr := newAnomalyTracker()
	t1 := time.Date(2026, 10, 8, 3, 0, 0, 0, time.UTC) // night
	t2 := t1.Add(time.Second)                          // also repeated

	tr.score("card-1", t1)
	got := tr.score("card-1", t2)

	if got.classification != "ANOMALOUS" {
		t.Fatalf("classification = %q, want ANOMALOUS", got.classification)
	}
	if got.score <= repeatedAttemptWeight || got.score <= unusualTimeWeight {
		t.Fatalf("score = %v, want both factors' weights summed (> either alone)", got.score)
	}
	if got.reason != anomalyReasonRepeatedAttempt+","+anomalyReasonUnusualTime {
		t.Fatalf("reason = %q, want both reasons listed in deterministic (sorted) order", got.reason)
	}
}
