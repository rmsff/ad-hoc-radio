package scenario

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"

	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/transport"
)

func applyVoiceLinkOverrides(version *ScheduledVersion, links []transport.Link) error {
	seen := map[ScheduledLinkKey]bool{}
	for _, link := range links {
		check := link
		check.Enabled = true
		if err := (transport.Topology{Links: []transport.Link{check}}).Validate(); err != nil {
			return err
		}
		key := ScheduledLinkKey{uint16(link.From), uint16(link.To)}
		if seen[key] {
			return fmt.Errorf("voice link override: duplicate edge %d->%d", link.From, link.To)
		}
		seen[key] = true
		if _, ok := version.Links[key]; !ok {
			return fmt.Errorf("voice link override: unselected edge %d->%d", link.From, link.To)
		}
		if link.FixedDelayMs != version.LinkDelayMS[key] {
			return fmt.Errorf("voice link override: fixed delay must match planner graph")
		}
		version.Links[key] = link
	}
	return nil
}

func selectedTreeLinks(tree routing.RelayTree, graph routing.NeighborGraph) map[ScheduledLinkKey]transport.Link {
	links := map[ScheduledLinkKey]transport.Link{}
	for _, edge := range tree.Edges {
		for _, link := range graph.Links {
			if link.FromStationID == edge.ParentStationID && link.ToStationID == edge.ChildStationID {
				links[ScheduledLinkKey{link.FromStationID, link.ToStationID}] = transport.Link{From: transport.StationID(link.FromStationID), To: transport.StationID(link.ToStationID), Enabled: true, LossPct: link.Loss * 100, FixedDelayMs: link.DelayMS}
			}
		}
	}
	return links
}

func scheduledPacketSeed(seed uint64, callID, seq uint32, version, from, to uint16) uint64 {
	var data [22]byte
	binary.LittleEndian.PutUint64(data[:], seed)
	binary.LittleEndian.PutUint32(data[8:], callID)
	binary.LittleEndian.PutUint32(data[12:], seq)
	binary.LittleEndian.PutUint16(data[16:], version)
	binary.LittleEndian.PutUint16(data[18:], from)
	binary.LittleEndian.PutUint16(data[20:], to)
	hash := fnv.New64a()
	_, _ = hash.Write(data[:])
	return hash.Sum64()
}
