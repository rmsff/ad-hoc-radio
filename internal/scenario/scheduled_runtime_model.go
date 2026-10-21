package scenario

import (
	"fmt"
	"github.com/rmsff/ad-hoc-radio/internal/node"
	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
	"github.com/rmsff/ad-hoc-radio/internal/transport"
)

const DefaultScheduledMissedParentThreshold = 2

type ScheduledLinkKey struct {
	FromStationID uint16
	ToStationID   uint16
}

type ScheduledVersion struct {
	Links       map[ScheduledLinkKey]transport.Link
	Tree        routing.RelayTree
	Schedule    timing.OpportunitySchedule
	LinkDelayMS map[ScheduledLinkKey]int64
}

type ScheduledReplanReason string

const (
	ScheduledReplanTopologyEpochChanged      ScheduledReplanReason = "topology_epoch_changed"
	ScheduledReplanMembershipSnapshotChanged ScheduledReplanReason = "membership_snapshot_changed"
	ScheduledReplanSelectedRelayUnavailable  ScheduledReplanReason = "selected_relay_unavailable"
	ScheduledReplanTimingAuthorityLost       ScheduledReplanReason = "timing_authority_lost"
	ScheduledReplanMissedParentThreshold     ScheduledReplanReason = "consecutive_missed_parent_threshold"
)

type ScheduledReplanInput struct {
	AtMS        int64
	Reason      ScheduledReplanReason
	PlanRequest routing.PlanRequest
	Binding     timing.ScheduleBinding
	Profile     timing.OpportunityProfile
}

type ScheduledRuntimeConfig struct {
	Reacquisitions         []ScheduledReacquisition
	Interference           []ScheduledInterferenceWindow
	Control                *peer.Config
	VoiceProfile           *timing.VoiceBudgetProfile
	Seed                   uint64
	Versions               []ScheduledVersion
	ReplanInputs           []ScheduledReplanInput
	Source                 *SourceStation
	Relays                 map[uint16]*node.RelayStation
	Receivers              map[uint16]*node.ReceiverStation
	ForwardQueueMaxDepth   int
	FrameCount             uint32
	AllowCycleTimingStress bool
	MissedParentThreshold  int
}

type ScheduledReacquisition struct {
	PeerID uint16
	AtMS   int64
}

func (r *scheduledRuntime) push(event sim.Event) {
	r.queue.Push(event)
	r.ownedEvents[r.queue.LastPushedID()] = true
}

// RunConcurrentScheduledRuntimes shares simulated RF occupancy and peer-local
// permits. Each call retains its own audio buffers and result counters; event
// hashes describe the common timeline, not an isolated replay of one call.
func RunConcurrentScheduledRuntimes(configs []ScheduledRuntimeConfig) ([]ScheduledRuntimeResult, error) {
	if len(configs) == 0 || len(configs) > peer.MaxPeers {
		return nil, fmt.Errorf("scheduled runtime: concurrent call count out of bounds")
	}
	queue := sim.NewEventQueue()
	runtimes := make([]*scheduledRuntime, 0, len(configs))
	var fleet *peer.Fleet
	identities := map[[4]uint32]bool{}
	for _, config := range configs {
		if config.Control == nil || config.Control.Fleet == nil {
			return nil, fmt.Errorf("scheduled runtime: concurrent calls require a shared Fleet")
		}
		if fleet != nil && fleet != config.Control.Fleet {
			return nil, fmt.Errorf("scheduled runtime: concurrent calls use different Fleets")
		}
		fleet = config.Control.Fleet
		cfg, err := normalizeScheduledRuntimeConfig(config)
		if err != nil {
			return nil, err
		}
		b := cfg.Versions[0].Schedule.Binding
		key := [4]uint32{uint32(b.NetID), uint32(b.GroupID), uint32(b.SourceStationID), b.CallID}
		if identities[key] {
			return nil, fmt.Errorf("scheduled runtime: duplicate concurrent call identity")
		}
		identities[key] = true
		r := newScheduledRuntime(cfg)
		r.queue = queue
		runtimes = append(runtimes, r)
	}
	for _, r := range runtimes {
		if err := r.processCallStart(); err != nil {
			return nil, err
		}
		if err := r.scheduleInitialEvents(); err != nil {
			return nil, err
		}
	}
	for queue.Len() > 0 {
		e, _ := queue.Pop()
		var owner *scheduledRuntime
		for _, r := range runtimes {
			if r.ownedEvents[e.EventID] || r.control.HasEvent(e.EventID) {
				owner = r
				break
			}
		}
		if owner == nil {
			return nil, fmt.Errorf("scheduled runtime: unowned shared event %d", e.EventID)
		}
		owner.nowMS = e.TimeMs
		if err := owner.processEvent(e); err != nil {
			return nil, err
		}
	}
	results := make([]ScheduledRuntimeResult, len(runtimes))
	for i, r := range runtimes {
		result, err := r.result()
		if err != nil {
			return nil, err
		}
		results[i] = result
	}
	return results, nil
}

type ScheduledRuntimeCounters struct {
	CollisionDropCount        int
	SourceFramesCreated       int
	SourceTransmissions       int
	RelayFramesReceived       int
	RelayFramesForwarded      int
	RelayTransmissions        int
	FrameDeliveries           int
	MissedOpportunityCount    int
	OutOfScheduleTxCount      int
	ScheduleMismatchDropCount int
	ScheduleExpiredDropCount  int
	NotScheduledDropCount     int
	RouteReplanCount          int
	VersionActivationCount    int
	ForwardQueueDrops         int
	ForwardQueueMaxDepth      int
	DuplicatePlayoutCount     int
	DroppedByReason           map[node.DropReason]int
	ReplanByReason            map[ScheduledReplanReason]int
}

type ScheduledLinkObservation struct {
	DelaySumMS    int64
	Lost          int
	Corrupted     int
	Duplicated    int
	FromStationID uint16
	ToStationID   uint16
	DelayMS       int64
	Deliveries    int
}

type ScheduledRelayDepthLatencySamples struct {
	RelayDepth uint8
	SamplesMS  []int64
}

type ScheduledRuntimeResult struct {
	Wire                    *ScheduledWireEvidence
	ControlCounters         *peer.Counters
	PlayoutEvents           []ScheduledPlayoutEvent
	MemberDelivery          []ScheduledMemberDeliveryCount
	ExpectedMemberIDs       []uint16
	CoveredMemberIDs        []uint16
	DeliveredMemberIDs      []uint16
	UncoveredMemberIDs      []uint16
	Counters                ScheduledRuntimeCounters
	ReceiverCounters        map[uint16]node.ReceiverCounters
	RelaySeenCacheEntries   int
	RelaySeenCacheEvictions int
	ActiveTree              routing.RelayTree
	ActiveSchedule          timing.OpportunitySchedule
	LinkObservations        []ScheduledLinkObservation
	LatencyByRelayDepth     []ScheduledRelayDepthLatencySamples
	TreeHash                string
	ScheduleHash            string
	EventHash               string
	EventCount              int
	SimDurationMS           int64
	ResultHash              string
	BlockedFromSeq          *uint32
}

type ScheduledPlayoutEvent struct {
	MemberID  uint16 `json:"member_id"`
	Seq       uint32 `json:"seq"`
	AtMS      int64  `json:"at_ms"`
	LatencyMS int64  `json:"latency_ms"`
}

type runtimeScheduleKey struct {
	NetID           uint16
	GroupID         uint16
	SourceStationID uint16
	CallID          uint32
	ScheduleID      uint32
}

type scheduledPendingDelivery struct {
	SerializedAtMS    int64
	PeerTransmission  *peer.AirTransmission
	TransmissionIndex int
	HasTransmission   bool
	ScheduleKey       runtimeScheduleKey
	ScheduleVersion   uint16
	ParentStationID   uint16
	OwnerStationID    uint16
	OpportunityIndex  uint16
	Edge              routing.RelayTreeEdge
	RawFrame          []byte
}

type scheduledReservationKey struct {
	ScheduleKey      runtimeScheduleKey
	ScheduleVersion  uint16
	SourceSeq        uint32
	ParentStationID  uint16
	OwnerStationID   uint16
	OpportunityIndex uint16
}

type scheduledReservation struct {
	Frame protocol.Frame
	Key   scheduledReservationKey
}
