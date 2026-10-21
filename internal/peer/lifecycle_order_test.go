package peer

import (
	"maps"
	"slices"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/sim"
)

func expiredFixturePending() map[string]*Permit {
	return map[string]*Permit{
		"e": {Digest: "e", ExpiresMS: 100},
		"c": {Digest: "c", ExpiresMS: 400},
		"a": {Digest: "a", ExpiresMS: 500},
		"g": {Digest: "g", ExpiresMS: 501}, // not expired at 500
		"d": {Digest: "d", ExpiresMS: 200},
		"b": {Digest: "b", ExpiresMS: 300},
		"f": {Digest: "f", ExpiresMS: 10_000}, // not expired
	}
}

// Several expired pending permits are released in ascending digest order,
// independent of map iteration order.
func TestExpiredPendingDigestsAreAscendingAndDeterministic(t *testing.T) {
	want := []string{"a", "b", "c", "d", "e"}
	// Go randomizes map iteration per range; many fresh maps expose any
	// dependence on it.
	for i := 0; i < 100; i++ {
		l := &localState{Pending: expiredFixturePending()}
		if got := expiredPendingDigests(l, 500); !slices.Equal(got, want) {
			t.Fatalf("run %d: expired digests = %v, want %v", i, got, want)
		}
	}
	if got := expiredPendingDigests(&localState{Pending: map[string]*Permit{}}, 500); len(got) != 0 {
		t.Fatalf("empty pending produced digests %v", got)
	}
}

func TestAcceptProposalReleasesAllExpiredPending(t *testing.T) {
	cfg, schedule := peerFixture(t)
	c := beginPeer(t, cfg, schedule, sim.NewEventQueue())
	l := c.locals[3]
	l.Pending = expiredFixturePending()
	l.Closed = map[string]int64{}

	if !c.acceptProposal(l, c.permit, 500) {
		t.Fatal("proposal rejected")
	}
	want := slices.Sorted(slices.Values([]string{"f", "g", c.permit.Digest}))
	if got := slices.Sorted(maps.Keys(l.Pending)); !slices.Equal(got, want) {
		t.Fatalf("pending after accept = %v, want %v", got, want)
	}
	if len(l.Closed) != 0 {
		t.Fatalf("closed after accept = %v, want expired digests swept", l.Closed)
	}
}
