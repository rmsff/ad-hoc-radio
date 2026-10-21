package timing

import "testing"

func TestVoiceBudgetBindsBytesAndFractionalBurst(t *testing.T) {
	p := VoiceBudgetProfile{ID: "test", CodecBitrateBPS: 1600, PacketPeriodMS: 80, GrossBitrateBPS: 19200, FECNumerator: 2, FECDenominator: 3, FramingBits: 48, RXToTXUS: 1000, TXToRXUS: 1000, ProcessingUS: 1000, UncertaintyUS: 1000}
	n, err := p.PayloadBytes()
	if err != nil || n != 16 {
		t.Fatalf("payload=%d err=%v", n, err)
	}
	b, err := p.Measure(make([]byte, 14+n))
	if err != nil {
		t.Fatal(err)
	}
	if b.AirtimeUS != 21250 || b.OccupiedUS != 25250 || b.DurationMS != 22 || b.GuardMS != 4 {
		t.Fatalf("budget=%+v", b)
	}
	p.GrossBitrateBPS = 9600
	p.FECNumerator = 1
	p.FECDenominator = 1
	p.FramingBits = 0
	p.RXToTXUS = 0
	p.TXToRXUS = 0
	p.ProcessingUS = 0
	p.UncertaintyUS = 2500
	p.SlotUS = 30000
	p.BurstUS = 27500
	b, err = p.Measure(make([]byte, 33))
	if err != nil {
		t.Fatal(err)
	}
	if b.AirtimeUS != 27500 || b.DurationMS != 28 || b.GuardMS != 3 || !b.FitsBackend {
		t.Fatalf("fractional timing lost: %+v", b)
	}
	p.UncertaintyUS = 2501
	b, _ = p.Measure(make([]byte, 33))
	if b.FitsBackend {
		t.Fatal("guard beyond slot accepted")
	}
}
