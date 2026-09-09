package decision

import (
	"strings"
	"testing"

	"nofx/store"
)

func TestParseFullDecisionResponseKeepsValidCloseWhenOpenIsInvalid(t *testing.T) {
	riskControl := store.RiskControlConfig{
		BTCETHMaxLeverage:  10,
		AltcoinMaxLeverage: 5,
		MinPositionSize:    10,
		MinRiskRewardRatio: 2,
	}
	response := `<decision>[
		{"symbol":"ETHUSDT","action":"open_short","leverage":0,"position_size_usd":100,"stop_loss":110,"take_profit":90,"confidence":90,"risk_usd":10,"reasoning":"invalid leverage"},
		{"symbol":"BTCUSDT","action":"close_long","reasoning":"reduce risk"}
	]</decision>`

	result, err := parseFullDecisionResponse(response, 1000, riskControl, map[string]float64{"ETHUSDT": 100})
	if err != nil {
		t.Fatalf("parseFullDecisionResponse() error = %v", err)
	}
	if len(result.Decisions) != 1 {
		t.Fatalf("got %d executable decisions, want 1", len(result.Decisions))
	}
	if result.Decisions[0].Action != "close_long" {
		t.Fatalf("kept action = %q, want close_long", result.Decisions[0].Action)
	}
	if len(result.ValidationErrors) != 1 || !strings.Contains(result.ValidationErrors[0], "decision #1") {
		t.Fatalf("validation errors = %#v, want rejected decision #1", result.ValidationErrors)
	}
}

func TestParseFullDecisionResponseErrorsWhenAllDecisionsAreInvalid(t *testing.T) {
	riskControl := store.RiskControlConfig{
		BTCETHMaxLeverage:  10,
		AltcoinMaxLeverage: 5,
		MinPositionSize:    10,
		MinRiskRewardRatio: 2,
	}
	response := `<decision>[{"symbol":"ETHUSDT","action":"open_short","leverage":0,"reasoning":"invalid"}]</decision>`

	result, err := parseFullDecisionResponse(response, 1000, riskControl, map[string]float64{"ETHUSDT": 100})
	if err == nil {
		t.Fatal("parseFullDecisionResponse() error = nil, want all-invalid error")
	}
	if result == nil || len(result.Decisions) != 0 || len(result.ValidationErrors) != 1 {
		t.Fatalf("result = %#v, want no executable decisions and one validation error", result)
	}
}
