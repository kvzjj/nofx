package decision

import (
	"testing"

	"nofx/store"
)

func TestValidateDecisionIgnoresAISizingFields(t *testing.T) {
	d := Decision{
		Symbol: "BTCUSDT", Action: "open_long",
		Leverage: 100, PositionSizeUSD: 1_000_000, RiskUSD: 1_000_000,
		Confidence: 0, StopLoss: 90, TakeProfit: 130,
	}
	risk := store.RiskControlConfig{MinRiskRewardRatio: 3, BTCETHMaxLeverage: 5}
	if err := validateDecision(&d, 10_000, risk, 100); err != nil {
		t.Fatalf("AI sizing fields must not authorize or reject the intent: %v", err)
	}
	if d.Leverage != 100 || d.PositionSizeUSD != 1_000_000 || d.RiskUSD != 1_000_000 {
		t.Fatalf("intent validation unexpectedly rewrote legacy fields: %+v", d)
	}
}

func TestValidateDecisionStillRejectsInvalidIntent(t *testing.T) {
	d := Decision{Symbol: "BTCUSDT", Action: "open_long", StopLoss: 101, TakeProfit: 130}
	if err := validateDecision(&d, 10_000, store.RiskControlConfig{MinRiskRewardRatio: 3}, 100); err == nil {
		t.Fatal("expected stop on wrong side of entry to be rejected")
	}
}
