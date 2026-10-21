package scenario

import (
	"fmt"
	"sort"
)

func fieldRealismRowID(driftPpm int64, slotOffsetMs int64, beaconLossPct float64, beaconJitterMs int64) string {
	return fmt.Sprintf("fri_drift_%04d_offset_%03d_beacon_loss_%06.2f_jitter_%05d",
		driftPpm, slotOffsetMs, beaconLossPct, beaconJitterMs)
}

func fieldRealismRowCount(cfg FieldRealismImpairmentConfig) int {
	return len(cfg.ClockDriftPpmCandidates) *
		len(cfg.SlotOffsetMsCandidates) *
		len(cfg.BeaconLossPctCandidates) *
		len(cfg.BeaconJitterMsCandidates)
}

func cloneFieldRealismNodes(in []FieldRealismNode) []FieldRealismNode {
	out := make([]FieldRealismNode, len(in))
	for i, node := range in {
		out[i] = node
		out[i].HealthTimeline = cloneFieldRealismHealthPoints(node.HealthTimeline)
	}
	return out
}

func cloneFieldRealismHealthPoints(in []FieldRealismNodeHealthPoint) []FieldRealismNodeHealthPoint {
	out := make([]FieldRealismNodeHealthPoint, len(in))
	copy(out, in)
	return out
}

func cloneFieldRealismDirectedLinks(in []FieldRealismDirectedLinkTimeline) []FieldRealismDirectedLinkTimeline {
	out := make([]FieldRealismDirectedLinkTimeline, len(in))
	for i, link := range in {
		out[i] = link
		out[i].Points = cloneFieldRealismQualityPoints(link.Points)
	}
	return out
}

func cloneFieldRealismQualityPoints(in []FieldRealismQualityPoint) []FieldRealismQualityPoint {
	out := make([]FieldRealismQualityPoint, len(in))
	copy(out, in)
	return out
}

func cloneFieldRealismWindows(in []FieldRealismWindow) []FieldRealismWindow {
	out := make([]FieldRealismWindow, len(in))
	copy(out, in)
	return out
}

func cloneFieldRealismHiddenWindows(in []FieldRealismHiddenNodeWindow) []FieldRealismHiddenNodeWindow {
	out := make([]FieldRealismHiddenNodeWindow, len(in))
	copy(out, in)
	return out
}

func cloneFieldRealismBusyWindows(in []FieldRealismLocalBusyWindow) []FieldRealismLocalBusyWindow {
	out := make([]FieldRealismLocalBusyWindow, len(in))
	copy(out, in)
	return out
}

func cloneFieldRealismFloat64s(in []float64) []float64 {
	out := make([]float64, len(in))
	copy(out, in)
	return out
}

func sortedFieldRealismFloat64s(values []float64) []float64 {
	out := cloneFieldRealismFloat64s(values)
	sort.Float64s(out)
	return out
}
