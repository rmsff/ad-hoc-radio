package peer

import (
	"encoding/binary"
	"fmt"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

type wireWriter struct{ raw []byte }

func (w *wireWriter) u8(v byte)    { w.raw = append(w.raw, v) }
func (w *wireWriter) u16(v uint16) { w.raw = binary.BigEndian.AppendUint16(w.raw, v) }
func (w *wireWriter) u32(v uint32) { w.raw = binary.BigEndian.AppendUint32(w.raw, v) }
func (w *wireWriter) u64(v uint64) { w.raw = binary.BigEndian.AppendUint64(w.raw, v) }
func (w *wireWriter) ids(ids []uint16) {
	w.u8(byte(len(ids)))
	for _, id := range ids {
		w.u16(id)
	}
}
func (w *wireWriter) schedule(s timing.OpportunitySchedule) {
	b := s.Binding
	w.u32(b.ScheduleID)
	w.u16(b.NetID)
	w.u16(b.GroupID)
	w.u16(b.SourceStationID)
	w.u32(b.CallID)
	w.u32(b.TopologyEpoch)
	w.u64(uint64(b.TopologyObservedAtMS))
	w.u32(b.MembershipSnapshotID)
	w.u64(uint64(b.MembershipObservedAtMS))
	w.u32(b.MembershipFreshnessMS)
	w.u16(uint16(b.EffectiveSeq))
	w.u16(uint16(b.ValidThroughSeq))
	w.u8(s.MaxRelayDepth)
	w.ids(b.ExpectedMemberIDs)
	w.ids(s.CoveredMemberIDs)
	w.ids(s.UncoveredMemberIDs)
	w.u8(byte(len(s.Opportunities)))
	for _, op := range s.Opportunities {
		w.u16(op.Index)
		w.u16(op.OwnerStationID)
		w.u16(op.ParentStationID)
		w.u8(op.Hop)
		w.u32(op.OffsetMS)
		w.u16(op.DurationMS)
		w.u16(op.GuardAfterMS)
	}
}

type wireReader struct {
	raw []byte
	err error
}

func (r *wireReader) take(n int) []byte {
	if r.err != nil || n < 0 || n > len(r.raw) {
		r.err = fmt.Errorf("peer: truncated compact field")
		return nil
	}
	p := r.raw[:n]
	r.raw = r.raw[n:]
	return p
}
func (r *wireReader) u8() byte {
	p := r.take(1)
	if p == nil {
		return 0
	}
	return p[0]
}
func (r *wireReader) u16() uint16 {
	p := r.take(2)
	if p == nil {
		return 0
	}
	return binary.BigEndian.Uint16(p)
}
func (r *wireReader) u32() uint32 {
	p := r.take(4)
	if p == nil {
		return 0
	}
	return binary.BigEndian.Uint32(p)
}
func (r *wireReader) u64() uint64 {
	p := r.take(8)
	if p == nil {
		return 0
	}
	return binary.BigEndian.Uint64(p)
}
func (r *wireReader) count() int {
	n := int(r.u8())
	if n > MaxPeers {
		r.err = fmt.Errorf("peer: compact inventory bound")
		return 0
	}
	return n
}
func (r *wireReader) ids() []uint16 {
	ids := make([]uint16, r.count())
	for i := range ids {
		ids[i] = r.u16()
	}
	return ids
}
func (r *wireReader) schedule(version uint16) timing.OpportunitySchedule {
	b := timing.ScheduleBinding{ScheduleVersion: version, TimingStrategy: timing.TimingStrategySourceLed}
	b.ScheduleID = r.u32()
	b.NetID = r.u16()
	b.GroupID = r.u16()
	b.SourceStationID = r.u16()
	b.ScheduleAuthorityID = b.SourceStationID
	b.TimingAuthorityID = b.SourceStationID
	b.CallID = r.u32()
	b.TopologyEpoch = r.u32()
	b.TopologyObservedAtMS = int64(r.u64())
	b.MembershipSnapshotID = r.u32()
	b.MembershipObservedAtMS = int64(r.u64())
	b.MembershipFreshnessMS = r.u32()
	b.EffectiveSeq = uint32(r.u16())
	b.ValidThroughSeq = uint32(r.u16())
	s := timing.OpportunitySchedule{MaxRelayDepth: r.u8()}
	b.ExpectedMemberIDs = r.ids()
	s.Binding = b
	s.CoveredMemberIDs = r.ids()
	s.UncoveredMemberIDs = r.ids()
	s.Opportunities = make([]timing.TxOpportunity, r.count())
	for i := range s.Opportunities {
		s.Opportunities[i] = timing.TxOpportunity{Index: r.u16(), OwnerStationID: r.u16(), ParentStationID: r.u16(), Hop: r.u8(), OffsetMS: r.u32(), DurationMS: r.u16(), GuardAfterMS: r.u16()}
	}
	return s
}
