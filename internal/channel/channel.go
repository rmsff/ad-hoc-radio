package channel

import (
	"fmt"
	"math"
	"math/big"
	"math/rand/v2"
	"sort"
)

const (
	DefaultChannelBitrateBps int64 = 9600
	DefaultSlotDurationMs    int64 = 40
	duplicateDelayMs         int64 = 5
)

type AirtimeConfig struct {
	ChannelBitrateBps int64
	SlotDurationMs    int64
}

type Measurement struct {
	FrameBytes        int
	FrameBits         int64
	ChannelBitrateBps int64
	AirtimeMs         int64
	SlotDurationMs    int64
	FitsSlot          bool
}

type Config struct {
	ChannelBitrateBps int64
	SlotDurationMs    int64
	Seed              uint64
	FixedDelayMs      int64
	JitterMs          int64
	LossPct           float64
	DuplicatePct      float64
	CorruptionPct     float64
}

type Frame struct {
	ID       string
	Bytes    []byte
	SentAtMs int64
}

type Delivery struct {
	FrameID            string
	InputIndex         int
	DeliveryID         int
	Bytes              []byte
	SentAtMs           int64
	ReceivedAtMs       int64
	Measurement        Measurement
	Corrupted          bool
	IsDuplicate        bool
	OriginalDeliveryID int
}

type Counters struct {
	InputFrames    int
	Delivered      int
	Lost           int
	Corrupted      int
	Duplicated     int
	SlotMisses     int
	TotalAirtimeMs int64
}

type Report struct {
	Config       Config
	Measurements []Measurement
	Deliveries   []Delivery
	Counters     Counters
}

func MeasureFrame(frame []byte, cfg AirtimeConfig) (Measurement, error) {
	return MeasureFrameBytes(len(frame), cfg)
}

func MeasureFrameBytes(frameBytes int, cfg AirtimeConfig) (Measurement, error) {
	cfg = normalizeAirtimeConfig(cfg)
	if err := validateAirtimeConfig(cfg); err != nil {
		return Measurement{}, err
	}

	if frameBytes < 0 {
		return Measurement{}, fmt.Errorf("channel: frame_bytes=%d must be >= 0", frameBytes)
	}
	if frameBytes > math.MaxInt64/8 {
		return Measurement{}, fmt.Errorf("channel: frame_bytes=%d overflows bit count", frameBytes)
	}
	frameBits := int64(frameBytes) * 8
	airtimeMs, err := airtimeMs(frameBits, cfg.ChannelBitrateBps)
	if err != nil {
		return Measurement{}, err
	}

	return Measurement{
		FrameBytes:        frameBytes,
		FrameBits:         frameBits,
		ChannelBitrateBps: cfg.ChannelBitrateBps,
		AirtimeMs:         airtimeMs,
		SlotDurationMs:    cfg.SlotDurationMs,
		FitsSlot:          airtimeMs <= cfg.SlotDurationMs,
	}, nil
}

func RunLoopback(frames []Frame, cfg Config) (Report, error) {
	cfg = normalizeConfig(cfg)
	if err := validateConfig(cfg); err != nil {
		return Report{}, err
	}

	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed))
	report := Report{
		Config:       cfg,
		Measurements: make([]Measurement, 0, len(frames)),
		Counters:     Counters{InputFrames: len(frames)},
	}

	for i, f := range frames {
		if f.SentAtMs < 0 {
			return Report{}, fmt.Errorf("channel: frame %d sent_at_ms=%d must be >= 0", i, f.SentAtMs)
		}
		m, err := MeasureFrame(f.Bytes, AirtimeConfig{
			ChannelBitrateBps: cfg.ChannelBitrateBps,
			SlotDurationMs:    cfg.SlotDurationMs,
		})
		if err != nil {
			return Report{}, err
		}
		report.Measurements = append(report.Measurements, m)
		totalAirtime, err := addMs(report.Counters.TotalAirtimeMs, m.AirtimeMs)
		if err != nil {
			return Report{}, fmt.Errorf("channel: total airtime after frame %d: %w", i, err)
		}
		report.Counters.TotalAirtimeMs = totalAirtime
		if !m.FitsSlot {
			report.Counters.SlotMisses++
		}

		if drawPct(rng, cfg.LossPct) {
			report.Counters.Lost++
			continue
		}

		corrupted := len(f.Bytes) > 0 && drawPct(rng, cfg.CorruptionPct)
		if corrupted {
			report.Counters.Corrupted++
		}
		receivedAt, err := receiveTimeMs(f.SentAtMs, m.AirtimeMs, cfg.FixedDelayMs, drawJitter(rng, cfg.JitterMs))
		if err != nil {
			return Report{}, fmt.Errorf("channel: frame %d receive time: %w", i, err)
		}
		originalDeliveryID := len(report.Deliveries)
		report.Deliveries = append(report.Deliveries, makeDelivery(f, i, originalDeliveryID, receivedAt, m, corrupted, false, -1))
		report.Counters.Delivered++

		if drawPct(rng, cfg.DuplicatePct) {
			report.Counters.Duplicated++
			dupAt, err := addMs(receivedAt, duplicateDelayMs)
			if err != nil {
				return Report{}, fmt.Errorf("channel: frame %d duplicate time: %w", i, err)
			}
			report.Deliveries = append(report.Deliveries, makeDelivery(f, i, len(report.Deliveries), dupAt, m, corrupted, true, originalDeliveryID))
			report.Counters.Delivered++
		}
	}

	sort.SliceStable(report.Deliveries, func(i, j int) bool {
		left := report.Deliveries[i]
		right := report.Deliveries[j]
		if left.ReceivedAtMs != right.ReceivedAtMs {
			return left.ReceivedAtMs < right.ReceivedAtMs
		}
		if left.InputIndex != right.InputIndex {
			return left.InputIndex < right.InputIndex
		}
		if left.IsDuplicate != right.IsDuplicate {
			return !left.IsDuplicate
		}
		return left.DeliveryID < right.DeliveryID
	})

	return report, nil
}

func makeDelivery(f Frame, inputIndex, deliveryIndex int, receivedAt int64, m Measurement, corrupted, duplicate bool, originalDelivery int) Delivery {
	bytes := make([]byte, len(f.Bytes))
	copy(bytes, f.Bytes)
	if corrupted && len(bytes) > 0 {
		bytes[0] ^= 0xFF
	}
	return Delivery{
		FrameID:            f.ID,
		InputIndex:         inputIndex,
		DeliveryID:         deliveryIndex,
		Bytes:              bytes,
		SentAtMs:           f.SentAtMs,
		ReceivedAtMs:       receivedAt,
		Measurement:        m,
		Corrupted:          corrupted,
		IsDuplicate:        duplicate,
		OriginalDeliveryID: originalDelivery,
	}
}

func receiveTimeMs(sentAt, airtime, fixedDelay, jitter int64) (int64, error) {
	withAirtime, err := addMs(sentAt, airtime)
	if err != nil {
		return 0, err
	}
	withDelay, err := addMs(withAirtime, fixedDelay)
	if err != nil {
		return 0, err
	}
	return addMs(withDelay, jitter)
}

func normalizeConfig(cfg Config) Config {
	airtime := normalizeAirtimeConfig(AirtimeConfig{
		ChannelBitrateBps: cfg.ChannelBitrateBps,
		SlotDurationMs:    cfg.SlotDurationMs,
	})
	cfg.ChannelBitrateBps = airtime.ChannelBitrateBps
	cfg.SlotDurationMs = airtime.SlotDurationMs
	return cfg
}

func normalizeAirtimeConfig(cfg AirtimeConfig) AirtimeConfig {
	if cfg.ChannelBitrateBps == 0 {
		cfg.ChannelBitrateBps = DefaultChannelBitrateBps
	}
	if cfg.SlotDurationMs == 0 {
		cfg.SlotDurationMs = DefaultSlotDurationMs
	}
	return cfg
}

func validateConfig(cfg Config) error {
	if err := validateAirtimeConfig(AirtimeConfig{
		ChannelBitrateBps: cfg.ChannelBitrateBps,
		SlotDurationMs:    cfg.SlotDurationMs,
	}); err != nil {
		return err
	}
	if cfg.FixedDelayMs < 0 {
		return fmt.Errorf("channel: fixed_delay_ms=%d must be >= 0", cfg.FixedDelayMs)
	}
	if cfg.JitterMs < 0 {
		return fmt.Errorf("channel: jitter_ms=%d must be >= 0", cfg.JitterMs)
	}
	if cfg.JitterMs >= math.MaxInt64 {
		return fmt.Errorf("channel: jitter_ms=%d too large (>= MaxInt64)", cfg.JitterMs)
	}
	if err := validatePct("loss_pct", cfg.LossPct); err != nil {
		return err
	}
	if err := validatePct("duplicate_pct", cfg.DuplicatePct); err != nil {
		return err
	}
	if err := validatePct("corruption_pct", cfg.CorruptionPct); err != nil {
		return err
	}
	return nil
}

func validateAirtimeConfig(cfg AirtimeConfig) error {
	if cfg.ChannelBitrateBps <= 0 {
		return fmt.Errorf("channel: channel_bitrate_bps=%d must be > 0", cfg.ChannelBitrateBps)
	}
	if cfg.SlotDurationMs <= 0 {
		return fmt.Errorf("channel: slot_duration_ms=%d must be > 0", cfg.SlotDurationMs)
	}
	return nil
}

func validatePct(name string, v float64) error {
	if math.IsNaN(v) {
		return fmt.Errorf("channel: %s is NaN", name)
	}
	if math.IsInf(v, 0) {
		return fmt.Errorf("channel: %s is Inf", name)
	}
	if v < 0 || v > 100 {
		return fmt.Errorf("channel: %s=%g is out of range [0, 100]", name, v)
	}
	return nil
}

func drawPct(rng *rand.Rand, pct float64) bool {
	if pct <= 0 {
		return false
	}
	if pct >= 100 {
		return true
	}
	return rng.Float64()*100 < pct
}

func drawJitter(rng *rand.Rand, jitterMs int64) int64 {
	if jitterMs <= 0 {
		return 0
	}
	return rng.Int64N(jitterMs + 1)
}

func addMs(a, b int64) (int64, error) {
	if a < 0 || b < 0 {
		return 0, fmt.Errorf("negative millisecond operand: %d + %d", a, b)
	}
	if a > math.MaxInt64-b {
		return 0, fmt.Errorf("int64 overflow: %d + %d", a, b)
	}
	return a + b, nil
}

func airtimeMs(frameBits int64, bitrateBps int64) (int64, error) {
	if frameBits < 0 {
		return 0, fmt.Errorf("channel: frame_bits=%d must be >= 0", frameBits)
	}
	numerator := new(big.Int).Mul(big.NewInt(frameBits), big.NewInt(1000))
	denominator := big.NewInt(bitrateBps)
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, denominator, remainder)
	if remainder.Sign() != 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, fmt.Errorf("channel: airtime overflows int64 for frame_bits=%d bitrate_bps=%d", frameBits, bitrateBps)
	}
	return quotient.Int64(), nil
}
