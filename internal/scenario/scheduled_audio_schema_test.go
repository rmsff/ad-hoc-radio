package scenario

import (
	"encoding/json"
	"testing"
)

func TestPeerAcceptanceMetricsSchemaAndClone(t *testing.T) {
	for _, loss := range []float64{0, 1} {
		cfg := peerAcceptanceConfig(1, 1, 19200, 42)
		for i := range cfg.InitialPlan.Graph.Links {
			cfg.InitialPlan.Graph.Links[i].Loss = loss
		}
		m, err := RunScheduledRelayTreeScenario(cfg)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := m.MarshalReportJSON()
		if err != nil {
			t.Fatal(err)
		}
		validateMetricsJSONAgainstSchema(t, raw)
		budget := m.ScheduledRelayTree.Wire.Budget
		if budget.WireBytes != 36 || budget.ProtectedBits != 480 || budget.AirtimeUS != 25000 || budget.GuardUS != 4000 {
			t.Fatalf("wire version binding omitted from cost: %+v", budget)
		}
		root := decodeMetricsJSONMap(t, raw)
		tree := root["scheduled_relay_tree"].(map[string]interface{})
		audio := tree["audio"].(map[string]interface{})
		audio["playout_latency_ms"] = "not a latency or null"
		bad, err := json.Marshal(root)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateMetricsJSONValue(bad, loadMetricsSchema(t)); err == nil {
			t.Fatal("anyOf silently ignored")
		}
		copy := cloneScheduledRelayTreeReport(m.ScheduledRelayTree)
		copy.Wire.Profile.ID = "changed"
		copy.Control.Commits++
		if copy.Wire.Profile.ID == m.ScheduledRelayTree.Wire.Profile.ID || copy.Control.Commits == m.ScheduledRelayTree.Control.Commits {
			t.Fatal("nested report alias")
		}
	}
}
