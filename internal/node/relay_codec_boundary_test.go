package node

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/codec"
)

func TestUT012_RelayForwardsDummyCodecPayloadWithoutCodecCalls(t *testing.T) {
	spy := codec.NewSpy(codec.DummyCodec{})
	payload, err := spy.Encode(codec.Input{Seq: 7, CreatedAtMs: 1_560})
	if err != nil {
		t.Fatalf("spy Encode failed: %v", err)
	}
	beforeEncodeCalls := spy.EncodeCalls()
	beforeDecodeCalls := spy.DecodeCalls()

	relay := relayTestSetup(t)
	frame := makeVoiceFrameWithPayload(7, payload, nil)

	result := relay.ProcessVoiceFrame(frame, 1_560)
	if result.Dropped != nil {
		t.Fatalf("relay dropped frame: %s", result.Dropped.Reason)
	}
	if result.Forwarded == nil {
		t.Fatal("expected forwarded frame, got nil")
	}

	if spy.EncodeCalls() != beforeEncodeCalls {
		t.Fatalf("relay forwarding changed EncodeCalls: got %d, want %d", spy.EncodeCalls(), beforeEncodeCalls)
	}
	if spy.DecodeCalls() != beforeDecodeCalls {
		t.Fatalf("relay forwarding changed DecodeCalls: got %d, want %d", spy.DecodeCalls(), beforeDecodeCalls)
	}
	if !bytes.Equal(result.Forwarded.Payload, payload) {
		t.Fatalf("forwarded payload = %x, want %x", result.Forwarded.Payload, payload)
	}
}

func TestUT012_RelayPackageDoesNotDependOnCodec(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", ".")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list dependencies failed: %v\n%s", err, output)
	}

	target := "github.com/rmsff/ad-hoc-radio/internal/codec"
	for _, dep := range strings.Fields(string(output)) {
		if dep == target || strings.HasPrefix(dep, target+"/") {
			t.Fatalf("internal/node depends on internal/codec:\n%s", output)
		}
	}
}
