package node

import (
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/protocol"
)

func decodeCallStartFrame(f protocol.Frame) (CallKey, protocol.CallStartPayload, error) {
	if err := f.VerifyIntegrity(); err != nil {
		return CallKey{}, protocol.CallStartPayload{}, err
	}
	if err := f.Validate(); err != nil {
		return CallKey{}, protocol.CallStartPayload{}, err
	}
	if f.FrameType != protocol.FrameTypeCallStart {
		return CallKey{}, protocol.CallStartPayload{}, fmt.Errorf("node: expected call_start frame, got frame_type=0x%02X", f.FrameType)
	}
	if f.Flags&protocol.FlagCallStart == 0 {
		return CallKey{}, protocol.CallStartPayload{}, fmt.Errorf("node: call_start frame missing call_start flag")
	}
	if f.Flags&protocol.FlagCallEnd != 0 {
		return CallKey{}, protocol.CallStartPayload{}, fmt.Errorf("node: call_start frame must not set call_end flag")
	}
	payload, err := protocol.DecodeCallStartPayload(f.Payload)
	if err != nil {
		return CallKey{}, protocol.CallStartPayload{}, err
	}
	return CallKeyFromFrame(f), payload, nil
}

func decodeCallEndFrame(f protocol.Frame) (CallKey, protocol.CallEndPayload, error) {
	if err := f.VerifyIntegrity(); err != nil {
		return CallKey{}, protocol.CallEndPayload{}, err
	}
	if err := f.Validate(); err != nil {
		return CallKey{}, protocol.CallEndPayload{}, err
	}
	if f.FrameType != protocol.FrameTypeCallEnd {
		return CallKey{}, protocol.CallEndPayload{}, fmt.Errorf("node: expected call_end frame, got frame_type=0x%02X", f.FrameType)
	}
	if f.Flags&protocol.FlagCallEnd == 0 {
		return CallKey{}, protocol.CallEndPayload{}, fmt.Errorf("node: call_end frame missing call_end flag")
	}
	if f.Flags&protocol.FlagCallStart != 0 {
		return CallKey{}, protocol.CallEndPayload{}, fmt.Errorf("node: call_end frame must not set call_start flag")
	}
	payload, err := protocol.DecodeCallEndPayload(f.Payload)
	if err != nil {
		return CallKey{}, protocol.CallEndPayload{}, err
	}
	return CallKeyFromFrame(f), payload, nil
}
