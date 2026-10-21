package main

import (
	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
	"github.com/rmsff/ad-hoc-radio/internal/transport"
)

// Fleet is deliberately not accepted from JSON: each CLI run owns fresh peers.
type scheduledPeerControlDTO struct {
	WireFormat        peer.WireFormat             `json:"wire_format"`
	ReplanLeadMS      int64                       `json:"replan_lead_ms"`
	LinksObservedAtMS int64                       `json:"links_observed_at_ms"`
	LinksFreshnessMS  int64                       `json:"links_freshness_ms"`
	Faults            []scheduledControlFaultDTO  `json:"faults"`
	Peers             []peer.State                `json:"peers"`
	Links             []scheduledTransportLinkDTO `json:"links"`
	StartMS           int64                       `json:"start_ms"`
	RetryMS           int64                       `json:"retry_ms"`
	Attempts          int                         `json:"attempts"`
	Seed              uint64                      `json:"seed"`
	MinBattery        float64                     `json:"min_battery"`
	Physical          timing.VoiceBudgetProfile   `json:"physical"`
}

func (d scheduledPeerControlDTO) toConfig() *peer.Config {
	faults := make([]peer.ControlFault, len(d.Faults))
	for i, f := range d.Faults {
		faults[i] = peer.ControlFault{Kind: f.Kind, Version: f.Version, From: f.From, To: f.To, LossPct: f.LossPct, DelayMS: f.DelayMS}
	}
	return &peer.Config{WireFormat: d.WireFormat, ReplanLeadMS: d.ReplanLeadMS, LinksObservedAtMS: d.LinksObservedAtMS, LinksFreshnessMS: d.LinksFreshnessMS, Faults: faults, Peers: d.Peers, Links: decodeScheduledTransportLinks(d.Links), StartMS: d.StartMS, RetryMS: d.RetryMS, Attempts: d.Attempts, Seed: d.Seed, MinBattery: d.MinBattery, Physical: d.Physical}
}

type scheduledControlFaultDTO struct {
	Version uint16    `json:"version"`
	Kind    peer.Kind `json:"kind"`
	From    uint16    `json:"from_station_id"`
	To      uint16    `json:"to_station_id"`
	LossPct float64   `json:"loss_pct"`
	DelayMS int64     `json:"delay_ms"`
}

type scheduledTransportLinkDTO struct {
	From          uint16  `json:"from_station_id"`
	To            uint16  `json:"to_station_id"`
	Enabled       bool    `json:"enabled"`
	LossPct       float64 `json:"loss_pct"`
	FixedDelayMS  int64   `json:"fixed_delay_ms"`
	JitterMS      int64   `json:"jitter_ms"`
	DuplicatePct  float64 `json:"duplicate_pct"`
	CorruptionPct float64 `json:"corruption_pct"`
}

func decodeScheduledTransportLinks(in []scheduledTransportLinkDTO) []transport.Link {
	out := make([]transport.Link, len(in))
	for i, l := range in {
		out[i] = transport.Link{From: transport.StationID(l.From), To: transport.StationID(l.To), Enabled: l.Enabled, LossPct: l.LossPct, FixedDelayMs: l.FixedDelayMS, JitterMs: l.JitterMS, DuplicatePct: l.DuplicatePct, CorruptionPct: l.CorruptionPct}
	}
	return out
}
