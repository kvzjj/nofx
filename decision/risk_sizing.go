package decision

import (
	"fmt"
	"math"

	"nofx/market"
)

// BackendRiskPolicy is intentionally not supplied by the AI. These limits are
// the final authority for every opening order.
const (
	PerTradeRiskPct        = 0.50
	MaxPortfolioRiskPct    = 1.50
	DailyLossHaltPct       = 1.50
	AccountDrawdownHaltPct = 8.00
	MaxVolumeParticipation = 0.001 // 0.10% of the latest bar's estimated notional
	MarginSafetyFactor     = 0.80
)

type EntrySizingInput struct {
	Action                string
	Equity                float64
	AvailableBalance      float64
	EntryPrice            float64
	StopLoss              float64
	ATR                   float64
	LatestBaseVolume      float64
	ExistingPositionCount int
	MaxLeverage           int
	MinPositionSize       float64
	MaxPositionSize       float64
	RemainingNotional     float64
}

type EntryPlan struct {
	Quantity        float64
	PositionSizeUSD float64
	Leverage        int
	RiskBudgetUSD   float64
	ActualRiskUSD   float64
}

// ConservativeATRAndVolume derives the portable volatility/liquidity inputs
// used by live trading and backtesting. Volume is a proxy until historical L2
// depth is available across all exchange adapters.
func ConservativeATRAndVolume(data *market.Data) (float64, float64) {
	if data == nil {
		return 0, 0
	}
	atr, volume := 0.0, math.Inf(1)
	consider := func(candidateATR, candidateVolume float64) {
		if candidateATR > atr {
			atr = candidateATR
		}
		if candidateVolume > 0 && candidateVolume < volume {
			volume = candidateVolume
		}
	}
	if data.IntradaySeries != nil {
		latest := 0.0
		if n := len(data.IntradaySeries.Volume); n > 0 {
			latest = data.IntradaySeries.Volume[n-1]
		}
		consider(data.IntradaySeries.ATR14, latest)
	}
	if data.LongerTermContext != nil {
		consider(math.Max(data.LongerTermContext.ATR3, data.LongerTermContext.ATR14), data.LongerTermContext.CurrentVolume)
	}
	for _, series := range data.TimeframeData {
		latest := 0.0
		if n := len(series.Klines); n > 0 {
			latest = series.Klines[n-1].Volume
		} else if n := len(series.Volume); n > 0 {
			latest = series.Volume[n-1]
		}
		consider(series.ATR14, latest)
	}
	if math.IsInf(volume, 1) {
		volume = 0
	}
	return atr, volume
}

// CalculateEntryPlan derives quantity and leverage exclusively from backend
// inputs. Existing positions are conservatively charged the full per-trade
// budget because exchange position snapshots do not expose their stop prices.
func CalculateEntryPlan(in EntrySizingInput) (EntryPlan, error) {
	if in.Equity <= 0 || in.AvailableBalance <= 0 {
		return EntryPlan{}, fmt.Errorf("account equity and available balance must be greater than 0")
	}
	if in.EntryPrice <= 0 || in.StopLoss <= 0 {
		return EntryPlan{}, fmt.Errorf("entry price and stop loss must be greater than 0")
	}
	if (in.Action == "open_long" && in.StopLoss >= in.EntryPrice) ||
		(in.Action == "open_short" && in.StopLoss <= in.EntryPrice) {
		return EntryPlan{}, fmt.Errorf("stop loss is on the wrong side of entry price")
	}
	if in.Action != "open_long" && in.Action != "open_short" {
		return EntryPlan{}, fmt.Errorf("entry sizing requires an opening action: %s", in.Action)
	}
	if in.ATR <= 0 || in.LatestBaseVolume <= 0 {
		return EntryPlan{}, fmt.Errorf("ATR and recent volume are required for deterministic sizing")
	}
	if in.MaxLeverage <= 0 {
		return EntryPlan{}, fmt.Errorf("maximum leverage must be greater than 0")
	}

	perTradeBudget := in.Equity * PerTradeRiskPct / 100
	portfolioBudget := in.Equity * MaxPortfolioRiskPct / 100
	existingRiskReserve := float64(in.ExistingPositionCount) * perTradeBudget
	riskBudget := math.Min(perTradeBudget, portfolioBudget-existingRiskReserve)
	if riskBudget <= 0 {
		return EntryPlan{}, fmt.Errorf("portfolio risk budget exhausted")
	}

	// A stop tighter than one ATR must not manufacture an oversized position.
	// The submitted stop remains unchanged; ATR is only a sizing floor.
	stopDistance := math.Abs(in.EntryPrice - in.StopLoss)
	effectiveDistance := math.Max(stopDistance, in.ATR)
	quantity := riskBudget / effectiveDistance
	notional := quantity * in.EntryPrice

	liquidityCap := in.LatestBaseVolume * in.EntryPrice * MaxVolumeParticipation
	notional = math.Min(notional, liquidityCap)
	if in.MaxPositionSize > 0 {
		notional = math.Min(notional, in.MaxPositionSize)
	}
	if in.RemainingNotional >= 0 {
		notional = math.Min(notional, in.RemainingNotional)
	}
	maxNotionalByMargin := in.AvailableBalance * MarginSafetyFactor * float64(in.MaxLeverage)
	notional = math.Min(notional, maxNotionalByMargin)
	if notional <= 0 || (in.MinPositionSize > 0 && notional < in.MinPositionSize) {
		return EntryPlan{}, fmt.Errorf("backend-sized position %.2f USDT is below minimum %.2f USDT", notional, in.MinPositionSize)
	}

	leverage := int(math.Ceil(notional / (in.AvailableBalance * MarginSafetyFactor)))
	if leverage < 1 {
		leverage = 1
	}
	if leverage > in.MaxLeverage {
		leverage = in.MaxLeverage
	}
	quantity = notional / in.EntryPrice
	actualRisk := stopDistance * quantity
	if actualRisk > riskBudget+1e-9 {
		return EntryPlan{}, fmt.Errorf("calculated risk %.2f exceeds backend budget %.2f", actualRisk, riskBudget)
	}

	return EntryPlan{
		Quantity:        quantity,
		PositionSizeUSD: notional,
		Leverage:        leverage,
		RiskBudgetUSD:   riskBudget,
		ActualRiskUSD:   actualRisk,
	}, nil
}
