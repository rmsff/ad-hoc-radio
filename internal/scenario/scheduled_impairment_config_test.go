package scenario

import (
	"github.com/rmsff/ad-hoc-radio/internal/transport"
	"testing"
)

func TestVoiceLinkOverridesRejectAmbiguity(t *testing.T) {
	for _, mode := range []string{"duplicate", "unselected", "delay", "disabled_invalid"} {
		t.Run(mode, func(t *testing.T) {
			cfg := peerAcceptanceConfig(1, 1, 19200, 42)
			l := transport.Link{From: 1, To: 2, Enabled: true, FixedDelayMs: 1}
			switch mode {
			case "duplicate":
				cfg.VoiceLinks = []transport.Link{l, l}
			case "unselected":
				l.To = 99
				cfg.VoiceLinks = []transport.Link{l}
			case "delay":
				l.FixedDelayMs = 2
				cfg.VoiceLinks = []transport.Link{l}
			case "disabled_invalid":
				l.Enabled = false
				l.LossPct = -1
				cfg.VoiceLinks = []transport.Link{l}
			}
			if _, err := RunScheduledRelayTreeScenario(cfg); err == nil {
				t.Fatal("invalid override accepted")
			}
		})
	}
}
