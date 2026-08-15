package observability

import (
	"errors"
	"testing"
	"time"
)

func TestObserveAndDrainRequests(t *testing.T) {
	m := NewMetrics()
	m.Observe(200, 10*time.Millisecond)
	m.Observe(500, 20*time.Millisecond)
	m.Observe(503, 30*time.Millisecond)

	snap := m.drain()
	if snap.requests != 3 {
		t.Errorf("requests = %d, want 3", snap.requests)
	}
	if snap.requestErrors != 2 {
		t.Errorf("requestErrors = %d, want 2 (5xx only)", snap.requestErrors)
	}
	if snap.latencyCount != 3 || snap.latencySumMs != 60 {
		t.Errorf("latency = %d/%d, want 60/3", snap.latencySumMs, snap.latencyCount)
	}
	if got := avgMs(snap.latencySumMs, snap.latencyCount); got != 20 {
		t.Errorf("avg latency = %v, want 20", got)
	}
	// The second drain must be empty (counters were reset).
	if snap2 := m.drain(); !snap2.empty() {
		t.Errorf("second drain not empty: %+v", snap2)
	}
}

func TestObserveDatabase(t *testing.T) {
	m := NewMetrics()
	// db1: two good pings then a failure -> avg over successes only, marked down.
	m.ObserveDatabase("db1", 5*time.Millisecond, nil)
	m.ObserveDatabase("db1", 15*time.Millisecond, nil)
	m.ObserveDatabase("db1", 2*time.Second, errors.New("down"))
	// db2: one good ping -> up.
	m.ObserveDatabase("db2", 8*time.Millisecond, nil)

	snap := m.drain()
	d1, ok := snap.dbs["db1"]
	if !ok {
		t.Fatal("db1 missing from snapshot")
	}
	if d1.latencyCount != 2 || d1.latencySumMs != 20 {
		t.Errorf("db1 latency = %d/%d, want 20/2 (failed pings excluded from the average)", d1.latencySumMs, d1.latencyCount)
	}
	if d1.errors != 1 {
		t.Errorf("db1 errors = %d, want 1", d1.errors)
	}
	if d1.up {
		t.Error("db1 up = true, want false (most recent ping failed)")
	}
	d2 := snap.dbs["db2"]
	if !d2.up || d2.errors != 0 || d2.latencyCount != 1 {
		t.Errorf("db2 = %+v, want up with one good sample", d2)
	}

	// After draining, the window holds no samples — but `up` must persist as the
	// last known state so DatabaseUp keeps reporting between windows.
	snap2 := m.drain()
	if snap2.dbs["db1"].up {
		t.Error("db1 up should still be false after drain")
	}
	if !snap2.empty() {
		t.Errorf("second window should hold no samples, got %+v", snap2.dbs)
	}
}
