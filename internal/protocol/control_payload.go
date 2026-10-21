package protocol

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
)

// CallStartTimingStrategy identifies the simulator timing authority announced
// by a scheduled call_start payload.
type CallStartTimingStrategy string

const (
	CallStartTimingStrategySourceLed   CallStartTimingStrategy = "source_led"
	CallStartTimingStrategyGroupLeader CallStartTimingStrategy = "group_leader"
)

// CallEndReason is the controlled reason enum for call_end payloads
// per frame-contract.md §8.3.
type CallEndReason string

const (
	CallEndReasonNormal  CallEndReason = "normal"
	CallEndReasonTimeout CallEndReason = "timeout"
	CallEndReasonAborted CallEndReason = "aborted"
)

// MaxFramePeriodMs is the inclusive upper bound of call_start
// frame_period_ms (frame-contract.md §8.1). compact_v1 and the peer layer
// carry the period in a u16 and keep their tighter 1..10000 range.
const MaxFramePeriodMs int64 = 60000

// MaxSafeCreatedAtMs is the largest created_at_ms a station accepts
// (frame-contract.md §8.1, §11). For 0 <= created_at_ms <= MaxSafeCreatedAtMs
// and a period in 1..MaxFramePeriodMs, created_at_ms + seq*frame_period_ms
// fits int64 for every 32-bit seq.
const MaxSafeCreatedAtMs = math.MaxInt64 - int64(math.MaxUint32)*MaxFramePeriodMs

// CallStartPayload is the minimum call_start control payload per
// frame-contract.md §8.1. JSON is used for the simulator control
// payload in Story 2.0; the outer Frame binary layout remains the
// canonical protocol envelope from §4.
type CallStartPayload struct {
	FramePeriodMs      int64                    `json:"frame_period_ms"`
	PayloadMode        string                   `json:"payload_mode"`
	ExpectedFrameCount *uint32                  `json:"expected_frame_count,omitempty"`
	SlotModeEnabled    bool                     `json:"slot_mode_enabled"`
	SlotDurationMs     *int64                   `json:"slot_duration_ms,omitempty"`
	TTLDefault         uint8                    `json:"ttl_default"`
	ScheduleID         *uint32                  `json:"schedule_id,omitempty"`
	ScheduleVersion    *uint16                  `json:"schedule_version,omitempty"`
	ScheduleDigest     *string                  `json:"schedule_digest,omitempty"`
	TimingStrategy     *CallStartTimingStrategy `json:"timing_strategy,omitempty"`
}

// Validate checks the Story 2.0 minimum call_start payload shape.
func (p CallStartPayload) Validate() error {
	if p.FramePeriodMs <= 0 || p.FramePeriodMs > MaxFramePeriodMs {
		return fmt.Errorf("protocol: call_start frame_period_ms=%d must be in 1..%d", p.FramePeriodMs, MaxFramePeriodMs)
	}
	if strings.TrimSpace(p.PayloadMode) == "" {
		return fmt.Errorf("protocol: call_start payload_mode must be non-empty")
	}
	if p.ExpectedFrameCount != nil && *p.ExpectedFrameCount == 0 {
		return fmt.Errorf("protocol: call_start expected_frame_count must be > 0 when present")
	}
	if p.SlotModeEnabled && p.SlotDurationMs == nil {
		return fmt.Errorf("protocol: call_start slot_duration_ms is required when slot_mode_enabled=true")
	}
	if p.SlotDurationMs != nil && *p.SlotDurationMs <= 0 {
		return fmt.Errorf("protocol: call_start slot_duration_ms must be > 0 when present")
	}
	if p.TTLDefault == 0 {
		return fmt.Errorf("protocol: call_start ttl_default must be > 0 for MVP relay scenarios")
	}
	if err := p.validateScheduleContext(); err != nil {
		return err
	}
	return nil
}

func (p CallStartPayload) validateScheduleContext() error {
	present := 0
	for _, ok := range []bool{
		p.ScheduleID != nil,
		p.ScheduleVersion != nil,
		p.ScheduleDigest != nil,
		p.TimingStrategy != nil,
	} {
		if ok {
			present++
		}
	}
	if present == 0 {
		return nil
	}
	if present != 4 {
		return fmt.Errorf("protocol: call_start schedule context fields must be all present or all absent")
	}
	if *p.ScheduleID == 0 {
		return fmt.Errorf("protocol: call_start schedule_id must be > 0")
	}
	if *p.ScheduleVersion == 0 {
		return fmt.Errorf("protocol: call_start schedule_version must be > 0")
	}
	if len(*p.ScheduleDigest) != 64 || strings.ToLower(*p.ScheduleDigest) != *p.ScheduleDigest {
		return fmt.Errorf("protocol: call_start schedule_digest must be 64 lowercase hexadecimal characters")
	}
	if _, err := hex.DecodeString(*p.ScheduleDigest); err != nil {
		return fmt.Errorf("protocol: call_start schedule_digest must be 64 lowercase hexadecimal characters")
	}
	switch *p.TimingStrategy {
	case CallStartTimingStrategySourceLed, CallStartTimingStrategyGroupLeader:
		return nil
	default:
		return fmt.Errorf("protocol: call_start timing_strategy %q is invalid", *p.TimingStrategy)
	}
}

// EncodeCallStartPayload validates and encodes p as JSON bytes for a
// call_start Frame.Payload.
func EncodeCallStartPayload(p CallStartPayload) ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	out, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("protocol: encode call_start payload: %w", err)
	}
	return out, nil
}

// DecodeCallStartPayload decodes a call_start JSON payload with
// unknown-field rejection, then validates the resulting shape.
func DecodeCallStartPayload(raw []byte) (CallStartPayload, error) {
	var decoded struct {
		FramePeriodMs      *int64                   `json:"frame_period_ms"`
		PayloadMode        *string                  `json:"payload_mode"`
		ExpectedFrameCount *uint32                  `json:"expected_frame_count,omitempty"`
		SlotModeEnabled    *bool                    `json:"slot_mode_enabled"`
		SlotDurationMs     *int64                   `json:"slot_duration_ms,omitempty"`
		TTLDefault         *uint8                   `json:"ttl_default"`
		ScheduleID         *uint32                  `json:"schedule_id,omitempty"`
		ScheduleVersion    *uint16                  `json:"schedule_version,omitempty"`
		ScheduleDigest     *string                  `json:"schedule_digest,omitempty"`
		TimingStrategy     *CallStartTimingStrategy `json:"timing_strategy,omitempty"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&decoded); err != nil {
		return CallStartPayload{}, fmt.Errorf("protocol: decode call_start payload: %w", err)
	}
	var trailing struct{}
	if err := dec.Decode(&trailing); err != io.EOF {
		return CallStartPayload{}, fmt.Errorf("protocol: decode call_start payload: trailing JSON value")
	}
	if decoded.FramePeriodMs == nil {
		return CallStartPayload{}, fmt.Errorf("protocol: call_start frame_period_ms is required")
	}
	if decoded.PayloadMode == nil {
		return CallStartPayload{}, fmt.Errorf("protocol: call_start payload_mode is required")
	}
	if decoded.SlotModeEnabled == nil {
		return CallStartPayload{}, fmt.Errorf("protocol: call_start slot_mode_enabled is required")
	}
	if decoded.TTLDefault == nil {
		return CallStartPayload{}, fmt.Errorf("protocol: call_start ttl_default is required")
	}
	p := CallStartPayload{
		FramePeriodMs:      *decoded.FramePeriodMs,
		PayloadMode:        *decoded.PayloadMode,
		ExpectedFrameCount: decoded.ExpectedFrameCount,
		SlotModeEnabled:    *decoded.SlotModeEnabled,
		SlotDurationMs:     decoded.SlotDurationMs,
		TTLDefault:         *decoded.TTLDefault,
		ScheduleID:         decoded.ScheduleID,
		ScheduleVersion:    decoded.ScheduleVersion,
		ScheduleDigest:     decoded.ScheduleDigest,
		TimingStrategy:     decoded.TimingStrategy,
	}
	if err := p.Validate(); err != nil {
		return CallStartPayload{}, err
	}
	return p, nil
}

// CallEndPayload is the minimum call_end control payload per
// frame-contract.md §8.3.
type CallEndPayload struct {
	LastSeq uint32        `json:"last_seq"`
	Reason  CallEndReason `json:"reason"`
}

// Validate checks the call_end reason enum. LastSeq is uint32 and may
// legitimately be 0 for a call that produced one voice frame.
func (p CallEndPayload) Validate() error {
	switch p.Reason {
	case CallEndReasonNormal, CallEndReasonTimeout, CallEndReasonAborted:
		return nil
	default:
		return fmt.Errorf("protocol: call_end reason %q is invalid", p.Reason)
	}
}

// EncodeCallEndPayload validates and encodes p as JSON bytes for a
// call_end Frame.Payload.
func EncodeCallEndPayload(p CallEndPayload) ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	out, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("protocol: encode call_end payload: %w", err)
	}
	return out, nil
}

// DecodeCallEndPayload decodes a call_end JSON payload with
// unknown-field rejection, then validates the reason enum.
func DecodeCallEndPayload(raw []byte) (CallEndPayload, error) {
	var decoded struct {
		LastSeq *uint32        `json:"last_seq"`
		Reason  *CallEndReason `json:"reason"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&decoded); err != nil {
		return CallEndPayload{}, fmt.Errorf("protocol: decode call_end payload: %w", err)
	}
	var trailing struct{}
	if err := dec.Decode(&trailing); err != io.EOF {
		return CallEndPayload{}, fmt.Errorf("protocol: decode call_end payload: trailing JSON value")
	}
	if decoded.LastSeq == nil {
		return CallEndPayload{}, fmt.Errorf("protocol: call_end last_seq is required")
	}
	if decoded.Reason == nil {
		return CallEndPayload{}, fmt.Errorf("protocol: call_end reason is required")
	}
	p := CallEndPayload{
		LastSeq: *decoded.LastSeq,
		Reason:  *decoded.Reason,
	}
	if err := p.Validate(); err != nil {
		return CallEndPayload{}, err
	}
	return p, nil
}
