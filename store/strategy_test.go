package store

import (
	"strings"
	"testing"
)

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

func TestDefaultStrategyPromptSupportsEquityLinkedInstruments(t *testing.T) {
	tests := []struct {
		name string
		lang string
		want []string
	}{
		{
			name: "Chinese",
			lang: "zh",
			want: []string{"代币化美股", "不要默认它们跟随BTC", "不得臆测实时美股现货价", "美国夏令时", "集中风险"},
		},
		{
			name: "English",
			lang: "en",
			want: []string{"tokenized US equities", "Do not assume these instruments follow BTC", "Never invent a live cash-equity price", "US daylight-saving rules", "concentrated exposure"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := GetDefaultStrategyConfig(tt.lang)
			prompt := config.PromptSections.RoleDefinition + "\n" +
				config.PromptSections.EntryStandards + "\n" +
				config.PromptSections.DecisionProcess

			for _, want := range tt.want {
				if !strings.Contains(prompt, want) {
					t.Errorf("default %s prompt missing %q", tt.lang, want)
				}
			}
		})
	}
}
