package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

func classifyGroupVoiceCoverage(cfg GroupVoiceProfileScenarioConfig) groupVoiceCoverageClassification {
	recipients := groupVoiceExpectedRecipients(cfg)
	directSet := uint16Set(cfg.SourceDirectReachable)
	unreachableSet := uint16Set(cfg.UnreachableMembers)
	depthByMember := make(map[uint16]int, len(recipients))
	for _, coverage := range cfg.RelayCoverage {
		depth := len(coverage.RelayPath)
		for _, memberID := range coverage.Members {
			if existingDepth, ok := depthByMember[memberID]; ok && existingDepth <= depth {
				continue
			}
			depthByMember[memberID] = depth
		}
	}

	classification := groupVoiceCoverageClassification{
		recipients:     recipients,
		coveredByDepth: make(map[int][]uint16),
	}
	for _, memberID := range recipients {
		if _, ok := directSet[memberID]; ok {
			classification.coveredDirect = append(classification.coveredDirect, memberID)
			continue
		}
		if depth, ok := depthByMember[memberID]; ok {
			classification.coveredByDepth[depth] = append(classification.coveredByDepth[depth], memberID)
			if depth > classification.requiredDepth {
				classification.requiredDepth = depth
			}
			continue
		}
		if _, ok := unreachableSet[memberID]; ok {
			classification.uncovered = append(classification.uncovered, memberID)
			continue
		}
		classification.unknown = append(classification.unknown, memberID)
	}
	return classification
}

func groupVoiceExpectedRecipients(cfg GroupVoiceProfileScenarioConfig) []uint16 {
	recipients := make([]uint16, 0, len(cfg.ExpectedMembers))
	for _, memberID := range sortedUniqueUint16s(cfg.ExpectedMembers) {
		if memberID == cfg.SourceID {
			continue
		}
		recipients = append(recipients, memberID)
	}
	return recipients
}

func coverageDepthRows(coveredByDepth map[int][]uint16) []GroupVoiceCoverageDepth {
	depths := make([]int, 0, len(coveredByDepth))
	for depth := range coveredByDepth {
		depths = append(depths, depth)
	}
	sort.Ints(depths)
	rows := make([]GroupVoiceCoverageDepth, 0, len(depths))
	for _, depth := range depths {
		rows = append(rows, GroupVoiceCoverageDepth{
			Depth:   depth,
			Members: sortedUniqueUint16s(coveredByDepth[depth]),
		})
	}
	return rows
}

func coveredViaRelayFromDepths(rows []GroupVoiceCoverageDepth) []uint16 {
	var members []uint16
	for _, row := range rows {
		members = append(members, row.Members...)
	}
	return sortedUniqueUint16s(members)
}

func groupVoiceCoverageFraction(report GroupVoiceProfileReport) float64 {
	if len(report.ExpectedRecipients) == 0 {
		return 1
	}
	covered := len(report.CoveredDirect) + len(report.CoveredViaRelay)
	return float64(covered) / float64(len(report.ExpectedRecipients))
}

func cloneGroupVoiceProfiles(profiles []GroupVoiceProfile) []GroupVoiceProfile {
	out := make([]GroupVoiceProfile, len(profiles))
	copy(out, profiles)
	return out
}

func cloneGroupVoiceUint16s(in []uint16) []uint16 {
	if len(in) == 0 {
		return nil
	}
	out := make([]uint16, len(in))
	copy(out, in)
	return out
}

func groupVoiceProfileReportHash(report GroupVoiceProfileReport) (string, error) {
	hashable := report
	hashable.EventHash = ""
	b, err := json.Marshal(hashable)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
