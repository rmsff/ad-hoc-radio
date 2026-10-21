package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"github.com/rmsff/ad-hoc-radio/internal/scenario"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
)

func TestPeerControlSnakeCaseLinkContract(t *testing.T) {
	var d scheduledPeerControlDTO
	raw := `{"links":[{"from_station_id":1,"to_station_id":2,"enabled":true,"loss_pct":10,"fixed_delay_ms":2,"jitter_ms":3,"duplicate_pct":4,"corruption_pct":5}],"retry_ms":30,"attempts":2}`
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		t.Fatal(err)
	}
	c := d.toConfig()
	l := c.Links[0]
	if l.From != 1 || l.To != 2 || l.LossPct != 10 || l.FixedDelayMs != 2 || l.JitterMs != 3 || l.DuplicatePct != 4 || l.CorruptionPct != 5 {
		t.Fatalf("ignored fields: %+v", l)
	}
	dec = json.NewDecoder(strings.NewReader(`{"fleet":{}}`))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err == nil {
		t.Fatal("caller injected shared fleet")
	}
}

func TestControlWireFormatJSONContract(t *testing.T) {
	for _, format := range []peer.WireFormat{"", peer.CompactV1, peer.JSONCRCv0, "unknown"} {
		raw, err := json.Marshal(scheduledPeerControlDTO{WireFormat: format, RetryMS: 30, Attempts: 2, Physical: scenario.PeerAcceptanceVoiceProfile(19200)})
		if err != nil {
			t.Fatal(err)
		}
		var d scheduledPeerControlDTO
		if err = json.Unmarshal(raw, &d); err != nil {
			t.Fatal(err)
		}
		cfg := d.toConfig()
		cfg.Peers = []peer.State{{ID: 1, Mode: peer.ADHR}}
		if cfg.WireFormat != format {
			t.Fatal("wire_format not propagated")
		}
		_, err = peer.New(*cfg, sim.NewEventQueue())
		if (err == nil) != (format != "unknown") {
			t.Fatalf("format=%q err=%v", format, err)
		}
	}
}

func TestPeerControlLocalTimingAndFaultContract(t *testing.T) {
	var d scheduledPeerControlDTO
	raw := `{"replan_lead_ms":800,"links_observed_at_ms":200,"links_freshness_ms":1000,"faults":[{"kind":"commit","version":2,"from_station_id":1,"to_station_id":2,"loss_pct":50,"delay_ms":90}]}`
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		t.Fatal(err)
	}
	c := d.toConfig()
	if c.ReplanLeadMS != 800 || c.LinksObservedAtMS != 200 || c.LinksFreshnessMS != 1000 || len(c.Faults) != 1 {
		t.Fatalf("local control fields not propagated: %+v", c)
	}
	f := c.Faults[0]
	if f.Kind != peer.Commit || f.Version != 2 || f.From != 1 || f.To != 2 || f.LossPct != 50 || f.DelayMS != 90 {
		t.Fatalf("control fault fields not propagated: %+v", f)
	}
}
