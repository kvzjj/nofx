package store

import "testing"

func TestApplyStrategyDefaultsPositionLimits(t *testing.T) {
	config := StrategyConfig{}
	config.ApplyDefaults()

	if config.RiskControl.MaxPositionSize != 1000 {
		t.Fatalf("MaxPositionSize = %v, want 1000", config.RiskControl.MaxPositionSize)
	}
	if config.RiskControl.MaxTotalPositionSize != 3000 {
		t.Fatalf("MaxTotalPositionSize = %v, want 3000", config.RiskControl.MaxTotalPositionSize)
	}
}

func TestApplyStrategyDefaultsPreservesPositionLimits(t *testing.T) {
	config := StrategyConfig{RiskControl: RiskControlConfig{
		MaxPositionSize:      250,
		MaxTotalPositionSize: 750,
	}}
	config.ApplyDefaults()

	if config.RiskControl.MaxPositionSize != 250 || config.RiskControl.MaxTotalPositionSize != 750 {
		t.Fatalf("position limits changed: %+v", config.RiskControl)
	}
}
