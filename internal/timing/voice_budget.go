package timing

import (
	"fmt"
	"math"
)

// VoiceBudgetProfile is a simulator capability assumption, not a measurement.
// Integer microseconds retain fractional-millisecond burst/guard distinctions.
type VoiceBudgetProfile struct {
	ID              string `json:"id"`
	CodecBitrateBPS int64  `json:"codec_bitrate_bps"`
	PacketPeriodMS  int64  `json:"packet_period_ms"`
	GrossBitrateBPS int64  `json:"gross_bitrate_bps"`
	FECNumerator    int64  `json:"fec_numerator"`
	FECDenominator  int64  `json:"fec_denominator"`
	FramingBits     int64  `json:"framing_bits"`
	RXToTXUS        int64  `json:"rx_to_tx_us"`
	TXToRXUS        int64  `json:"tx_to_rx_us"`
	ProcessingUS    int64  `json:"processing_us"`
	UncertaintyUS   int64  `json:"uncertainty_us"`
	SlotUS          int64  `json:"slot_us"`
	BurstUS         int64  `json:"burst_us"`
	MaxPayloadBytes int    `json:"max_payload_bytes"`
}

type WireBudget struct {
	WireBytes     int    `json:"wire_bytes"`
	ProtectedBits int64  `json:"protected_bits"`
	AirtimeUS     int64  `json:"airtime_us"`
	GuardUS       int64  `json:"guard_us"`
	OccupiedUS    int64  `json:"occupied_us"`
	DurationMS    uint32 `json:"duration_ms"`
	GuardMS       uint32 `json:"guard_ms"`
	FitsBackend   bool   `json:"fits_backend"`
}

func (p VoiceBudgetProfile) Validate() error {
	if p.ID == "" || p.CodecBitrateBPS <= 0 || p.CodecBitrateBPS > 128000 || p.PacketPeriodMS <= 0 || p.PacketPeriodMS > 10000 || p.GrossBitrateBPS <= 0 || p.GrossBitrateBPS > 1000000000 {
		return fmt.Errorf("voice budget: invalid identity/bitrate/period")
	}
	if p.FECNumerator <= 0 || p.FECDenominator < p.FECNumerator || p.FECDenominator > 1000 {
		return fmt.Errorf("voice budget: invalid FEC code rate")
	}
	for _, v := range []int64{p.FramingBits, p.RXToTXUS, p.TXToRXUS, p.ProcessingUS, p.UncertaintyUS, p.SlotUS, p.BurstUS} {
		if v < 0 || v > 10000000 {
			return fmt.Errorf("voice budget: invalid timing/framing value")
		}
	}
	if (p.SlotUS == 0) != (p.BurstUS == 0) || p.BurstUS > p.SlotUS || p.MaxPayloadBytes < 0 {
		return fmt.Errorf("voice budget: invalid burst/slot/payload constraint")
	}
	if p.CodecBitrateBPS*p.PacketPeriodMS%8000 != 0 {
		return fmt.Errorf("voice budget: packet does not contain an integer number of bytes")
	}
	return nil
}

func (p VoiceBudgetProfile) PayloadBytes() (int, error) {
	if err := p.Validate(); err != nil {
		return 0, err
	}
	return int(p.CodecBitrateBPS * p.PacketPeriodMS / 8000), nil
}

func (p VoiceBudgetProfile) Measure(raw []byte) (WireBudget, error) {
	if err := p.Validate(); err != nil {
		return WireBudget{}, err
	}
	if len(raw) == 0 || len(raw) > 65535 {
		return WireBudget{}, fmt.Errorf("voice budget: invalid wire length")
	}
	protected := ceilPositive(int64(len(raw))*8*p.FECDenominator, p.FECNumerator) + p.FramingBits
	airtime := ceilPositive(protected*1000000, p.GrossBitrateBPS)
	guard := p.RXToTXUS + p.TXToRXUS + p.ProcessingUS + p.UncertaintyUS
	fits := p.MaxPayloadBytes == 0 || len(raw) <= p.MaxPayloadBytes
	occupied := airtime + guard
	if p.SlotUS > 0 {
		fits = fits && airtime <= p.BurstUS && guard <= p.SlotUS-p.BurstUS
		if p.SlotUS > occupied {
			occupied = p.SlotUS
		}
	}
	// Keep exact microsecond evidence; the millisecond simulator rounds each
	// reserved component UP, never silently rounds 27.5 ms down to 27 ms.
	if ceilPositive(airtime, 1000) > math.MaxUint32 || ceilPositive(occupied-airtime, 1000) > math.MaxUint32 {
		return WireBudget{}, fmt.Errorf("voice budget: reservation exceeds uint32 milliseconds")
	}
	return WireBudget{WireBytes: len(raw), ProtectedBits: protected, AirtimeUS: airtime, GuardUS: occupied - airtime, OccupiedUS: occupied, DurationMS: uint32(ceilPositive(airtime, 1000)), GuardMS: uint32(ceilPositive(occupied-airtime, 1000)), FitsBackend: fits}, nil
}

func ceilPositive(n, d int64) int64 { return n/d + boolInt(n%d != 0) }
func boolInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
