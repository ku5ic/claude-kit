package claudecli

import (
	"testing"
	"time"
)

// A key a run failed on waits out CooldownPeriod; a cleared key, another
// caller's, or another key doesn't.
func TestCooldownHoldsAFailedKeyForItsPeriod(t *testing.T) {
	t.Parallel()
	dir, now := t.TempDir(), time.Now()
	c := NewCooldown(dir, "claims")
	if err := c.Record(now, map[string]string{"verifier": "timed out"}, nil); err != nil {
		t.Fatal(err)
	}
	f, ok := c.Cooling("verifier", now.Add(time.Minute))
	if !ok || f.Why != "timed out" || !f.Until().Equal(now.Add(CooldownPeriod)) {
		t.Errorf("Cooling = %+v, %v", f, ok)
	}
	if _, ok := c.Cooling("verifier", now.Add(CooldownPeriod)); ok {
		t.Error("held past its period")
	}
	if _, ok := c.Cooling("other", now); ok {
		t.Error("another key held")
	}
	if _, ok := NewCooldown(dir, "gapfill").Cooling("verifier", now); ok {
		t.Error("another caller's key held")
	}
	if err := c.Record(now, nil, []string{"verifier"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Cooling("verifier", now); ok {
		t.Error("a cleared key held")
	}
}

// A failure stamped ahead of the clock can't say when it ends, so it holds
// nothing back, and the next record drops it.
func TestCooldownDropsAFailureAheadOfTheClock(t *testing.T) {
	t.Parallel()
	now := time.Now()
	c := NewCooldown(t.TempDir(), "gapfill")
	if err := c.Record(now.Add(48*time.Hour), map[string]string{"a": "x"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Cooling("a", now); ok {
		t.Error("a future stamp held the key")
	}
	if err := c.Record(now, map[string]string{"b": "y"}, nil); err != nil {
		t.Fatal(err)
	}
	if got := c.failures(); len(got) != 1 {
		t.Errorf("failures after the record = %+v, want only b", got)
	}
}

// The zero Cooldown, with no file, holds nothing and records nothing.
func TestTheZeroCooldownHoldsNothing(t *testing.T) {
	t.Parallel()
	var c Cooldown
	if err := c.Record(time.Now(), map[string]string{"a": "x"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Cooling("a", time.Now()); ok {
		t.Error("held a key")
	}
}
