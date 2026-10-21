package scenario

import (
	"slices"
	"testing"
)

// With several pending activations for one effective_seq (the invariant allows
// at most one), the lowest event id is replaced regardless of map order.
func TestScheduleActivationReplacesLowestEventIDForSameEffectiveSeq(t *testing.T) {
	for run := 0; run < 100; run++ {
		runtime := &scheduledRuntime{activationEvents: map[uint64]scheduledActivation{
			7:  {Noop: true, EffectiveSeq: 5},
			3:  {Noop: true, EffectiveSeq: 5},
			11: {Noop: true, EffectiveSeq: 5},
			2:  {Noop: true, EffectiveSeq: 6},
			9:  {Noop: true, EffectiveSeq: 5},
		}}

		runtime.scheduleActivation(0, scheduledActivation{Block: true, EffectiveSeq: 5})

		if got := runtime.activationEvents[3]; !got.Block || got.Noop || got.EffectiveSeq != 5 {
			t.Fatalf("run %d: event 3 = %+v, want replaced Block activation", run, got)
		}
		for _, id := range []uint64{7, 9, 11} {
			if got := runtime.activationEvents[id]; !got.Noop || got.Block {
				t.Fatalf("run %d: event %d = %+v, want untouched Noop", run, id, got)
			}
		}
		if got := runtime.activationEvents[2]; !got.Noop || got.EffectiveSeq != 6 {
			t.Fatalf("run %d: event 2 = %+v, want untouched other effective_seq", run, got)
		}
		if len(runtime.activationEvents) != 5 {
			t.Fatalf("run %d: activation events = %d, want 5 (replace, not add)", run, len(runtime.activationEvents))
		}
	}
}

func TestActivationEventIDsAreAscending(t *testing.T) {
	runtime := &scheduledRuntime{activationEvents: map[uint64]scheduledActivation{}}
	for _, id := range []uint64{42, 7, 19, 3, 100, 1} {
		runtime.activationEvents[id] = scheduledActivation{}
	}
	want := []uint64{1, 3, 7, 19, 42, 100}
	for run := 0; run < 100; run++ {
		if got := runtime.activationEventIDs(); !slices.Equal(got, want) {
			t.Fatalf("run %d: ids = %v, want %v", run, got, want)
		}
	}
}
