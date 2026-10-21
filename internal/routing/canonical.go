package routing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type canonicalRelayTree struct {
	CoveredMemberIDs     []uint16                 `json:"covered_member_ids"`
	Edges                []canonicalRelayTreeEdge `json:"edges"`
	MaxRelayDepth        uint8                    `json:"max_relay_depth"`
	MembershipSnapshotID uint32                   `json:"membership_snapshot_id"`
	SourceStationID      uint16                   `json:"source_station_id"`
	TopologyEpoch        uint32                   `json:"topology_epoch"`
	UncoveredMemberIDs   []uint16                 `json:"uncovered_member_ids"`
}

type canonicalRelayTreeEdge struct {
	ChildStationID  uint16 `json:"child_station_id"`
	Hop             uint8  `json:"hop"`
	ParentStationID uint16 `json:"parent_station_id"`
}

func CanonicalRelayTreeJSON(tree RelayTree) ([]byte, error) {
	if _, err := validateRelayTreeStructure(tree); err != nil {
		return nil, err
	}

	edges := make([]canonicalRelayTreeEdge, len(tree.Edges))
	for i, edge := range tree.Edges {
		edges[i] = canonicalRelayTreeEdge{
			ChildStationID:  edge.ChildStationID,
			Hop:             edge.Hop,
			ParentStationID: edge.ParentStationID,
		}
	}
	canonical := canonicalRelayTree{
		CoveredMemberIDs:     nonNilStationIDs(tree.Coverage.CoveredMemberIDs),
		Edges:                edges,
		MaxRelayDepth:        tree.MaxRelayDepth,
		MembershipSnapshotID: tree.MembershipSnapshotID,
		SourceStationID:      tree.SourceStationID,
		TopologyEpoch:        tree.TopologyEpoch,
		UncoveredMemberIDs:   nonNilStationIDs(tree.Coverage.UncoveredMemberIDs),
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return nil, fmt.Errorf("relay tree: encode canonical JSON: %w", err)
	}
	return encoded, nil
}

func RelayTreeHash(tree RelayTree) (string, error) {
	encoded, err := CanonicalRelayTreeJSON(tree)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func nonNilStationIDs(ids []uint16) []uint16 {
	if len(ids) == 0 {
		return []uint16{}
	}
	return ids
}
