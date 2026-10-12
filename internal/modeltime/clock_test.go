package modeltime

import (
	"testing"
	"time"
)

func TestFormatParse_RoundTrip(t *testing.T) {
	in := time.Date(2026, 10, 8, 8, 15, 30, 0, time.UTC)

	s := Format(in)
	got, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	if !got.Equal(in) {
		t.Fatalf("round-trip = %v, want %v", got, in)
	}
}

func TestFormat_AlwaysUTC(t *testing.T) {
	// design-specification.md section 4.3: model time is an absolute
	// simulated date/time, not tied to any process's local wall-clock
	// timezone — formatting must normalize to UTC regardless of the
	// input time.Time's location.
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("tzdata not available in this environment: %v", err)
	}
	in := time.Date(2026, 10, 8, 4, 15, 30, 0, loc) // 08:15:30 UTC

	got := Format(in)
	want := "2026-10-08T08:15:30Z"
	if got != want {
		t.Fatalf("Format = %q, want %q", got, want)
	}
}

func TestClock_AdvanceIsCumulativeAndDoesNotDependOnWallClock(t *testing.T) {
	start := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
	c := NewClock(start)

	if got := c.Now(); !got.Equal(start) {
		t.Fatalf("Now() before any advance = %v, want %v", got, start)
	}

	c.Advance(10 * time.Second)
	c.Advance(5 * time.Second)

	want := start.Add(15 * time.Second)
	if got := c.Now(); !got.Equal(want) {
		t.Fatalf("Now() after two advances = %v, want %v (REQ-FUNC-003's timer must be driven by this, not by real elapsed time)", got, want)
	}
}

func TestClock_AdvanceReturnsNewValue(t *testing.T) {
	c := NewClock(time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC))
	returned := c.Advance(3 * time.Second)
	if !returned.Equal(c.Now()) {
		t.Fatalf("Advance's return value %v != Now() %v", returned, c.Now())
	}
}
