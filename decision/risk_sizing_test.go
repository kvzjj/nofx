package decision

import (
	"math"
	"testing"
)

func TestCalculateEntryPlanUsesBackendRiskBudget(t *testing.T) {
	plan, err := CalculateEntryPlan(EntrySizingInput{
		Action:                "open_long",
		Equity:                10_000,
		AvailableBalance:      10_000,
		EntryPrice:            100,
		StopLoss:              95,
		ATR:                   2,
		LatestBaseVolume:      1_000_000,
		ExistingPositionCount: 0,
		MaxLeverage:           5,
		MinPositionSize:       12,
		MaxPositionSize:       10_000,
		RemainingNotional:     10_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.RiskBudgetUSD != 50 || plan.ActualRiskUSD != 50 {
		t.Fatalf("risk budget/actual = %.2f/%.2f, want 50/50", plan.RiskBudgetUSD, plan.ActualRiskUSD)
	}
	if plan.Quantity != 10 || plan.PositionSizeUSD != 1000 || plan.Leverage != 1 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
}

func TestCalculateEntryPlanATRAndLiquidityOnlyReduce(t *testing.T) {
	base := EntrySizingInput{
		Action: "open_long", Equity: 10_000, AvailableBalance: 10_000,
		EntryPrice: 100, StopLoss: 99, ATR: 5, LatestBaseVolume: 1_000_000,
		MaxLeverage: 5, MinPositionSize: 1, MaxPositionSize: 100_000, RemainingNotional: 100_000,
	}
	plan, err := CalculateEntryPlan(base)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(plan.Quantity-10) > 1e-9 { // 50 USD / 5 ATR, not / 1 USD stop
		t.Fatalf("ATR-sized quantity = %.8f, want 10", plan.Quantity)
	}

	base.LatestBaseVolume = 100 // liquidity cap = 10 USDT
	liquidPlan, err := CalculateEntryPlan(base)
	if err != nil {
		t.Fatal(err)
	}
	if liquidPlan.PositionSizeUSD != 10 || liquidPlan.PositionSizeUSD >= plan.PositionSizeUSD {
		t.Fatalf("liquidity cap did not reduce size: base=%+v capped=%+v", plan, liquidPlan)
	}
}

func TestCalculateEntryPlanReservesPortfolioRisk(t *testing.T) {
	in := EntrySizingInput{
		Action: "open_short", Equity: 10_000, AvailableBalance: 10_000,
		EntryPrice: 100, StopLoss: 105, ATR: 2, LatestBaseVolume: 1_000_000,
		ExistingPositionCount: 3, MaxLeverage: 5, MinPositionSize: 1,
		MaxPositionSize: 100_000, RemainingNotional: 100_000,
	}
	if _, err := CalculateEntryPlan(in); err == nil {
		t.Fatal("expected exhausted portfolio risk budget")
	}
}

func TestCalculateEntryPlanIgnoresAIAmountsByConstruction(t *testing.T) {
	// EntrySizingInput has no AI position-size, leverage request, confidence, or
	// risk_usd fields; identical backend inputs necessarily produce one plan.
	in := EntrySizingInput{
		Action: "open_long", Equity: 10_000, AvailableBalance: 500,
		EntryPrice: 100, StopLoss: 95, ATR: 2, LatestBaseVolume: 1_000_000,
		MaxLeverage: 5, MinPositionSize: 1, MaxPositionSize: 100_000, RemainingNotional: 100_000,
	}
	a, err := CalculateEntryPlan(in)
	if err != nil {
		t.Fatal(err)
	}
	b, err := CalculateEntryPlan(in)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("deterministic plans differ: %+v vs %+v", a, b)
	}
}
