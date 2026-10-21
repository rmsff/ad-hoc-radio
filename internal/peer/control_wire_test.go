package peer

import (
	"bytes"
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

func wireMessage(t testing.TB) message {
	t.Helper()
	_, s := peerFixture(t)
	p, err := makePermit(s, 1000, 80)
	if err != nil {
		t.Fatal(err)
	}
	return message{Kind: Proposal, From: 1, To: 2, Nonce: 1, Permit: p}
}

func TestCompactRoundTripAndSizes(t *testing.T) {
	m := wireMessage(t)
	for _, kind := range []Kind{Proposal, ACK, Commit, End, Sync} {
		m.Kind = kind
		raw, err := encodeControl(m, CompactV1)
		if err != nil {
			t.Fatal(err)
		}
		got, err := decodeControl(raw, CompactV1)
		if err != nil {
			t.Fatal(err)
		}
		if got.Kind != kind || got.From != m.From || got.To != m.To || got.Nonce != m.Nonce || got.Permit.Digest != m.Permit.Digest {
			t.Fatal("header differs")
		}
		if kind == Proposal {
			a, _ := timing.CanonicalOpportunityScheduleJSON(m.Permit.Schedule)
			b, _ := timing.CanonicalOpportunityScheduleJSON(got.Permit.Schedule)
			if !bytes.Equal(a, b) || got.Permit.EpochMS != 1000 || got.Permit.PeriodMS != 80 || got.Permit.ExpiresMS != m.Permit.ExpiresMS {
				t.Fatal("permit differs")
			}
			if len(raw) != 153 {
				t.Fatalf("proposal bytes=%d", len(raw))
			}
		} else if len(raw) != CompactReferenceBytes || len(got.Permit.Schedule.Opportunities) != 0 {
			t.Fatal("reference includes schedule")
		}
		old, err := encodeControl(m, JSONCRCv0)
		if err != nil || len(raw)*3 >= len(old) {
			t.Fatalf("no substantial reduction: %d/%d %v", len(raw), len(old), err)
		}
		again, err := encodeControl(got, CompactV1)
		if err != nil || !bytes.Equal(raw, again) {
			t.Fatal("noncanonical round trip")
		}
	}
}

func TestCompactHeaderGolden(t *testing.T) {
	m := wireMessage(t)
	m.Kind = Commit
	raw, err := encodeControl(m, CompactV1)
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(raw[:16]) != "41430103000100020000000000000001" || hex.EncodeToString(raw[48:60]) != "000100000000000003e80050" {
		t.Fatalf("header layout %x", raw)
	}
	if _, err := decodeControl(raw, JSONCRCv0); err == nil {
		t.Fatal("binary accepted as JSON")
	}
}

func TestCompactMalformed(t *testing.T) {
	raw, err := encodeControl(wireMessage(t), CompactV1)
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < len(raw); n++ {
		if _, err := decodeControl(raw[:n], CompactV1); err == nil {
			t.Fatalf("truncation %d accepted", n)
		}
		if n < len(raw)-4 {
			if _, err := decodeControl(sealControl(append([]byte(nil), raw[:n]...)), CompactV1); err == nil {
				t.Fatalf("valid-CRC truncation %d accepted", n)
			}
		}
	}
	for _, offset := range []int{0, 2, 3, 4, 5, 16, 48, 50, 58, 60, 110} {
		bad := append([]byte(nil), raw[:len(raw)-4]...)
		bad[offset] = 255
		if _, err := decodeControl(sealControl(bad), CompactV1); err == nil {
			t.Fatalf("invalid field %d accepted", offset)
		}
	}
	if _, err := decodeControl(sealControl(append(append([]byte(nil), raw[:len(raw)-4]...), 0)), CompactV1); err == nil {
		t.Fatal("suffix accepted")
	}
	if _, err := decodeControl(make([]byte, MaxCompactControlBytes+1), CompactV1); err == nil {
		t.Fatal("oversize accepted")
	}
	bad := append([]byte(nil), raw...)
	bad[16] ^= 1
	if _, err := decodeControl(bad, CompactV1); err == nil {
		t.Fatal("corruption accepted")
	}
}

func TestCompactReferenceRequiresLocalExactPermit(t *testing.T) {
	m := wireMessage(t)
	m.Kind = Commit
	raw, _ := encodeControl(m, CompactV1)
	ref, err := decodeControl(raw, CompactV1)
	if err != nil {
		t.Fatal(err)
	}
	l := &localState{State: State{ID: 2}, Pending: map[string]*Permit{}}
	if _, ok := resolveReference(l, ref); ok {
		t.Fatal("unknown proposal synthesized")
	}
	p := m.Permit
	l.Pending[p.Digest] = &p
	resolved, ok := resolveReference(l, ref)
	if !ok || !reflect.DeepEqual(resolved.Permit, p) {
		t.Fatal("local proposal not resolved")
	}
	for _, edit := range []func(*message){
		func(m *message) { m.Permit.EpochMS++ }, func(m *message) { m.Permit.PeriodMS++ },
		func(m *message) { m.Permit.Schedule.Binding.ScheduleVersion++ },
		func(m *message) { m.Permit.Digest = "unknown" }, func(m *message) { m.From = 3 },
	} {
		bad := ref
		edit(&bad)
		if _, ok := resolveReference(l, bad); ok {
			t.Fatal("foreign reference resolved")
		}
	}
	// The controller knows the proposal, but this destination never received it.
	cfg, s := peerFixture(t)
	q := sim.NewEventQueue()
	c := beginPeer(t, cfg, s, q)
	if err := c.send(m, 1, 0); err != nil {
		t.Fatal(err)
	}
	// Remove the queued local launch so no proposal can populate destination state.
	for id, event := range c.events {
		if event.Local == "launch" {
			delete(c.events, id)
		}
	}
	for {
		e, ok := q.Pop()
		if !ok {
			break
		}
		if c.HasEvent(e.EventID) {
			if _, err := c.Process(e); err != nil {
				t.Fatal(err)
			}
		}
	}
	if c.stats.Commits != 0 || c.locals[2].Active != nil {
		t.Fatal("commit without proposal authorized")
	}
}

func TestCompactMaximumInventory(t *testing.T) {
	edges := []routing.RelayTreeEdge{}
	members := []uint16{}
	for id := uint16(2); id <= 64; id++ {
		edges = append(edges, routing.RelayTreeEdge{ParentStationID: 1, ChildStationID: id})
		members = append(members, id)
	}
	s := peerSchedule(t, 1, 10, edges, members)
	s.Binding.ValidThroughSeq = 65535
	s.Binding.EffectiveSeq = 65534
	s.Binding.TopologyObservedAtMS = 1 << 33
	s.Binding.MembershipObservedAtMS = 1 << 33
	p, err := makePermit(s, 1<<34, 80)
	if err != nil {
		t.Fatal(err)
	}
	m := message{Kind: Proposal, From: 1, To: 2, Nonce: 1 << 40, Permit: p}
	raw, err := encodeControl(m, CompactV1)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeControl(raw, CompactV1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Permit.Digest != p.Digest || len(got.Permit.Schedule.Binding.ExpectedMemberIDs) != 63 {
		t.Fatal("boundary roundtrip")
	}
}

func FuzzCompactControl(f *testing.F) {
	f.Add([]byte("AC"))
	f.Add(make([]byte, CompactReferenceBytes))
	m := wireMessage(f)
	for _, kind := range []Kind{Proposal, ACK, Commit, End, Sync} {
		m.Kind = kind
		raw, err := encodeControl(m, CompactV1)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(raw[:len(raw)-4])
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > MaxCompactControlBytes {
			return
		}
		raw = sealControl(append([]byte(nil), raw...))
		m, err := decodeControl(raw, CompactV1)
		if err != nil {
			return
		}
		again, err := encodeControl(m, CompactV1)
		if err != nil || !bytes.Equal(raw, again) {
			t.Fatal("accepted noncanonical wire")
		}
	})
}
