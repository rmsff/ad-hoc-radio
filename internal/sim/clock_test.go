package sim

import "testing"

// TestClock_AdvanceMonotonic verifies basic advance and Now.
func TestClock_AdvanceMonotonic(t *testing.T) {
	c := NewSimulatedClock()
	if c.Now() != 0 {
		t.Errorf("initial Now = %d, want 0", c.Now())
	}
	c.Advance(100)
	if c.Now() != 100 {
		t.Errorf("Now = %d, want 100", c.Now())
	}
	c.Advance(50)
	if c.Now() != 150 {
		t.Errorf("Now = %d, want 150", c.Now())
	}
}

// TestClock_SetAbsolute verifies Set pins absolute time.
func TestClock_SetAbsolute(t *testing.T) {
	c := NewSimulatedClock()
	c.Set(1000)
	if c.Now() != 1000 {
		t.Errorf("Now = %d, want 1000", c.Now())
	}
}

// TestClock_AdvanceNegativePanics is Review Finding 4: negative
// advance must panic (monotonic clock).
func TestClock_AdvanceNegativePanics(t *testing.T) {
	c := NewSimulatedClock()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Advance(-1) did not panic")
		}
	}()
	c.Advance(-1)
}

// TestClock_SetBackwardsPanics verifies Set cannot move backwards.
func TestClock_SetBackwardsPanics(t *testing.T) {
	c := NewSimulatedClock()
	c.Set(100)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Set(50) after Set(100) did not panic")
		}
	}()
	c.Set(50)
}

// TestClock_AdvanceOverflowPanics is Review Finding 4: advancing past
// math.MaxInt64 must panic rather than wrap to a negative time.
func TestClock_AdvanceOverflowPanics(t *testing.T) {
	c := NewSimulatedClock()
	c.Set(1 << 62) // large but leaves room to overflow with a big advance
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Advance past MaxInt64 did not panic")
		}
	}()
	c.Advance(1 << 62)
}

// TestAddMs_OverflowPanics verifies the shared overflow-safe addition
// used by transport delay arithmetic panics on overflow.
func TestAddMs_OverflowPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("AddMs(MaxInt64, 1) did not panic")
		}
	}()
	AddMs(1<<63-1, 1)
}

// TestAddMs_NegativePanics verifies negative operands are rejected.
func TestAddMs_NegativePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("AddMs(-1, 1) did not panic")
		}
	}()
	AddMs(-1, 1)
}

// TestAddMs_Normal verifies the happy path.
func TestAddMs_Normal(t *testing.T) {
	if got := AddMs(100, 50); got != 150 {
		t.Errorf("AddMs(100,50) = %d, want 150", got)
	}
}
