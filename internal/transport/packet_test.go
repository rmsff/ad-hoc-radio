package transport

import (
	"math"
	"reflect"
	"testing"
)

func TestSamplePacketImpairmentBounds(t *testing.T) {
	l := Link{From: 1, To: 2, Enabled: true, FixedDelayMs: 1, JitterMs: 10, DuplicatePct: 100, CorruptionPct: 100}
	raw := []byte{1, 2, 3}
	a, err := SamplePacket(l, raw, 100, 42)
	if err != nil {
		t.Fatal(err)
	}
	b, err := SamplePacket(l, raw, 100, 42)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatalf("nondeterministic: %v", err)
	}
	if len(a) != 2 || !a[0].Corrupted || !a[1].IsDuplicate || a[1].ReceivedAt-a[0].ReceivedAt != 5 || raw[2] != 3 {
		t.Fatalf("invalid delivery/caller mutation: %+v", a)
	}
	l.LossPct = 100
	lost, err := SamplePacket(l, raw, 100, 42)
	if err != nil || len(lost) != 0 {
		t.Fatalf("100 percent loss: %v %v", lost, err)
	}
	l.LossPct = 0
	if _, err := SamplePacket(l, raw, math.MaxInt64, 42); err == nil {
		t.Fatal("overflow accepted")
	}
	l.LossPct = math.NaN()
	if _, err := SamplePacket(l, raw, 0, 42); err == nil {
		t.Fatal("NaN accepted")
	}
}
