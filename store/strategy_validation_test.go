package store

import (
	"strings"
	"testing"
)

func validConfig() StrategyConfig {
	cfg := GetDefaultStrategyConfig("en")
	return cfg
}

func TestValidateStrategyConfigAcceptsDefaults(t *testing.T) {
	cfg := validConfig()
	cfg.ApplyDefaults()
	warnings, errs := ValidateStrategyConfig(&cfg)
	if len(errs) > 0 {
		t.Fatalf("default config rejected: %v", errs)
	}
	_ = warnings
}

func TestValidateStrategyConfigCrossFieldLimits(t *testing.T) {
	cfg := validConfig()
	cfg.ApplyDefaults()
	// min > max per order: every entry would be rejected at sizing time.
	cfg.RiskControl.MinPositionSize = cfg.RiskControl.MaxPositionSize + 1
	_, errs := ValidateStrategyConfig(&cfg)
	if !hasErrorContaining(errs, "min_position_size") {
		t.Fatalf("expected min>max error, got %v", errs)
	}

	cfg = validConfig()
	cfg.ApplyDefaults()
	cfg.RiskControl.MaxPositionSize = cfg.RiskControl.MaxTotalPositionSize + 1
	_, errs = ValidateStrategyConfig(&cfg)
	if !hasErrorContaining(errs, "max_position_size") {
		t.Fatalf("expected per-order>total error, got %v", errs)
	}
}

func TestValidateStrategyConfigLeverageCaps(t *testing.T) {
	cfg := validConfig()
	cfg.ApplyDefaults()
	cfg.RiskControl.AltcoinMaxLeverage = 500
	_, errs := ValidateStrategyConfig(&cfg)
	if !hasErrorContaining(errs, "altcoin_max_leverage") {
		t.Fatalf("expected leverage cap error, got %v", errs)
	}

	// High-but-legal leverage must warn, not reject.
	cfg = validConfig()
	cfg.ApplyDefaults()
	cfg.RiskControl.BTCETHMaxLeverage = 50
	warnings, errs := ValidateStrategyConfig(&cfg)
	if !hasErrorContaining(warnings, "btc_eth_max_leverage") || len(errs) != 0 {
		t.Fatalf("expected warning only, got warnings=%v errs=%v", warnings, errs)
	}
}

func TestValidateStrategyConfigTimeframeWhitelist(t *testing.T) {
	cfg := validConfig()
	cfg.ApplyDefaults()
	cfg.Indicators.Klines.PrimaryTimeframe = "7m"
	cfg.Indicators.Klines.SelectedTimeframes = []string{"5m", "7m"}
	_, errs := ValidateStrategyConfig(&cfg)
	if !hasErrorContaining(errs, "primary timeframe") || !hasErrorContaining(errs, "selected timeframe") {
		t.Fatalf("expected timeframe errors, got %v", errs)
	}
}

func TestValidateStrategyConfigKlineCountBounds(t *testing.T) {
	cfg := validConfig()
	cfg.ApplyDefaults()
	cfg.Indicators.Klines.PrimaryCount = 100000
	_, errs := ValidateStrategyConfig(&cfg)
	if !hasErrorContaining(errs, "K-line count") {
		t.Fatalf("expected kline count error, got %v", errs)
	}
}

func TestValidateStrategyConfigCoinSymbols(t *testing.T) {
	cfg := validConfig()
	cfg.ApplyDefaults()
	cfg.CoinSource.StaticCoins = append(cfg.CoinSource.StaticCoins, "BTC/USDT!", "not a symbol!")
	_, errs := ValidateStrategyConfig(&cfg)
	if !hasErrorContaining(errs, "invalid coin symbol") {
		t.Fatalf("expected coin symbol errors, got %v", errs)
	}
}

func TestValidateStrategyConfigQuantURLScheme(t *testing.T) {
	cfg := validConfig()
	cfg.ApplyDefaults()
	cfg.Indicators.EnableQuantData = true
	cfg.Indicators.QuantDataAPIURL = "file:///etc/passwd?symbol={symbol}"
	_, errs := ValidateStrategyConfig(&cfg)
	if !hasErrorContaining(errs, "quant data URL") {
		t.Fatalf("expected quant URL error, got %v", errs)
	}
}

func TestApplyDefaultsDedupesAndNormalizesCoins(t *testing.T) {
	cfg := StrategyConfig{}
	cfg.ApplyDefaults()
	cfg.CoinSource.StaticCoins = []string{" btcusdt ", "BTCUSDT", "ethusdt", ""}
	cfg.ApplyDefaults()
	got := strings.Join(cfg.CoinSource.StaticCoins, ",")
	if got != "BTCUSDT,ETHUSDT" {
		t.Fatalf("dedupe result = %q, want BTCUSDT,ETHUSDT", got)
	}
}

func TestTrailingProfitExitDefaultsPreserveHistoricalBehavior(t *testing.T) {
	// A config saved before the fields existed (JSON keys absent) must keep
	// the always-on 5%%/40%% behavior and accept explicit opt-out.
	legacy := StrategyConfig{}
	legacy.ApplyDefaults()
	if !legacy.RiskControl.TrailingProfitExitEnabled() {
		t.Fatalf("legacy config must default to enabled")
	}
	trigger, giveback := legacy.RiskControl.TrailingProfitExitLevels()
	if trigger != 5 || giveback != 40 {
		t.Fatalf("legacy defaults = %.1f/%.0f, want 5/40", trigger, giveback)
	}

	off := false
	disabled := StrategyConfig{RiskControl: RiskControlConfig{EnableTrailingProfitExit: &off}}
	disabled.ApplyDefaults()
	if disabled.RiskControl.TrailingProfitExitEnabled() {
		t.Fatalf("explicit disable must be preserved")
	}
}

func TestValidateStrategyConfigTrailingProfitBounds(t *testing.T) {
	cfg := validConfig()
	cfg.ApplyDefaults()
	cfg.RiskControl.TrailingProfitTriggerPct = 5000
	cfg.RiskControl.TrailingProfitGivebackPct = 0.1
	_, errs := ValidateStrategyConfig(&cfg)
	if !hasErrorContaining(errs, "trailing_profit_trigger_pct") || !hasErrorContaining(errs, "trailing_profit_giveback_pct") {
		t.Fatalf("expected trailing profit bound errors, got %v", errs)
	}
}

func hasErrorContaining(list []string, substr string) bool {
	for _, item := range list {
		if strings.Contains(item, substr) {
			return true
		}
	}
	return false
}
