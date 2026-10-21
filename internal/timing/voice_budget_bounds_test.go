package timing

import "testing"

func TestVoiceBudgetRejectsReservationOverflow(t *testing.T) {
	p := VoiceBudgetProfile{ID: "slow", CodecBitrateBPS: 1600, PacketPeriodMS: 80, GrossBitrateBPS: 1, FECNumerator: 1, FECDenominator: 1000}
	if _, err := p.Measure(make([]byte, 65535)); err == nil {
		t.Fatal("wrapped duration accepted")
	}
}
