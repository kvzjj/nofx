package backtest

import (
	"math"
	"testing"

	"nofx/decision"
	"nofx/market"
	"nofx/store"
)

func testFeed(bars []market.Kline) *DataFeed {
	closeTimes := make([]int64, len(bars))
	for i := range bars {
		closeTimes[i] = bars[i].CloseTime
	}
	return &DataFeed{
		primaryTF:    "1m",
		symbols:      []string{"BTCUSDT"},
		timeframes:   []string{"1m"},
		symbolSeries: map[string]*symbolSeries{"BTCUSDT": {byTF: map[string]*timeframeSeries{"1m": {klines: bars, closeTimes: closeTimes}}}},
	}
}

func testStrategyEngine() *decision.StrategyEngine {
	return decision.NewStrategyEngine(&store.StrategyConfig{RiskControl: store.RiskControlConfig{
		MaxPositions: 3, BTCETHMaxLeverage: 5, AltcoinMaxLeverage: 5,
		MinPositionSize: 12, MaxPositionSize: 1000, MaxTotalPositionSize: 3000,
		MinRiskRewardRatio: 3,
	}})
}

func TestSignalQueuesWithoutBookingFutureOpen(t *testing.T) {
	bars := make([]market.Kline, 0, 31)
	for i := 0; i < 30; i++ {
		closePrice := 97.1 + float64(i)*0.1
		bars = append(bars, market.Kline{Open: closePrice - 0.2, High: closePrice + 2, Low: closePrice - 2, Close: closePrice, Volume: 10_000, QuoteVolume: 1_000_000, CloseTime: int64(i+1) * 1000})
	}
	bars = append(bars, market.Kline{Open: 95, High: 112, Low: 94, Close: 111, Volume: 10_000, QuoteVolume: 1_000_000, CloseTime: 31_000})
	account := NewBacktestAccount(1000, 0, 0)
	r := &Runner{cfg: BacktestConfig{FillPolicy: FillPolicyNextOpen}, feed: testFeed(bars), account: account, strategyEngine: testStrategyEngine(), state: &BacktestState{Equity: 1000, Positions: map[string]PositionSnapshot{}}}
	dec := decision.Decision{Symbol: "BTCUSDT", Action: "open_long", Leverage: 2, PositionSizeUSD: 110, StopLoss: 90, TakeProfit: 130, RiskUSD: 20}

	_, trades, _, err := r.executeDecision(dec, map[string]float64{"BTCUSDT": 100}, 30_000, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(trades) != 0 || len(account.Positions()) != 0 || account.Cash() != 1000 {
		t.Fatalf("signal close must only queue: trades=%d positions=%d cash=%.2f", len(trades), len(account.Positions()), account.Cash())
	}

	trades, err = r.executePendingAtOpen(31_000, map[string]float64{"BTCUSDT": 111}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(trades) != 1 {
		t.Fatalf("expected one fill, got %+v", trades)
	}
	if math.Abs(trades[0].Price-95) > 1e-9 {
		t.Fatalf("fill price %.2f, want next bar open 95", trades[0].Price)
	}
}

func TestSameBarStopAndTakeProfitUsesStopFirst(t *testing.T) {
	bar := market.Kline{Open: 100, High: 120, Low: 90, Close: 110, CloseTime: 2000, QuoteVolume: 1_000_000}
	account := NewBacktestAccount(1000, 0, 0)
	if _, _, _, err := account.Open("BTCUSDT", "long", 1, 2, 100, 1000, 95, 115); err != nil {
		t.Fatal(err)
	}
	r := &Runner{cfg: BacktestConfig{FillPolicy: FillPolicyNextOpen}, feed: testFeed([]market.Kline{bar}), account: account, state: &BacktestState{Positions: map[string]PositionSnapshot{}}}

	events, _, err := r.checkBarExits(2000, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one exit, got %+v", events)
	}
	if events[0].Action != "stop_loss" || math.Abs(events[0].Price-95) > 1e-9 {
		t.Fatalf("expected conservative stop-loss fill, got %+v", events[0])
	}
}

func TestFundingAndTieredMarginAffectAccount(t *testing.T) {
	account := NewBacktestAccount(1000, 0, 0)
	account.SetMarginTiers([]MarginTier{
		{NotionalCap: 0, InitialMarginRate: 0.25, MaintenanceMarginRate: 0.05},
		{NotionalCap: 500, InitialMarginRate: 0.10, MaintenanceMarginRate: 0.02},
	})
	pos, _, _, err := account.Open("BTCUSDT", "long", 4, 20, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(pos.Margin-40) > 1e-9 {
		t.Fatalf("tier initial margin %.2f, want 40", pos.Margin)
	}
	before := account.Cash()
	account.ApplyFunding(map[string]float64{"BTCUSDT": 100}, 0.001)
	if math.Abs(account.Cash()-(before-0.4)) > 1e-9 {
		t.Fatalf("funding cash %.4f, want %.4f", account.Cash(), before-0.4)
	}
}
