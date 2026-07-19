package backtest

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

const epsilon = 1e-8

type position struct {
	Symbol           string
	Side             string
	Quantity         float64
	EntryPrice       float64
	Leverage         int
	Margin           float64
	Notional         float64
	LiquidationPrice float64
	OpenTime         int64
	StopLoss         float64
	TakeProfit       float64
	MaintenanceRate  float64
}

type MarginTier struct {
	NotionalCap           float64 `json:"notional_cap"`
	InitialMarginRate     float64 `json:"initial_margin_rate"`
	MaintenanceMarginRate float64 `json:"maintenance_margin_rate"`
}

type ExecutionCost struct {
	FeeRate      float64
	SlippageRate float64
}

type BacktestAccount struct {
	initialBalance float64
	cash           float64
	feeRate        float64
	slippageRate   float64
	positions      map[string]*position
	realizedPnL    float64
	marginTiers    []MarginTier
}

func NewBacktestAccount(initialBalance, feeBps, slippageBps float64) *BacktestAccount {
	return &BacktestAccount{
		initialBalance: initialBalance,
		cash:           initialBalance,
		feeRate:        feeBps / 10000.0,
		slippageRate:   slippageBps / 10000.0,
		positions:      make(map[string]*position),
	}
}

func (acc *BacktestAccount) SetMarginTiers(tiers []MarginTier) {
	acc.marginTiers = append([]MarginTier(nil), tiers...)
	sort.Slice(acc.marginTiers, func(i, j int) bool {
		left, right := acc.marginTiers[i].NotionalCap, acc.marginTiers[j].NotionalCap
		if left <= 0 {
			return false
		}
		if right <= 0 {
			return true
		}
		return left < right
	})
}

func positionKey(symbol, side string) string {
	return strings.ToUpper(symbol) + ":" + side
}

func (acc *BacktestAccount) ensurePosition(symbol, side string) *position {
	key := positionKey(symbol, side)
	if pos, ok := acc.positions[key]; ok {
		return pos
	}
	pos := &position{Symbol: strings.ToUpper(symbol), Side: side}
	acc.positions[key] = pos
	return pos
}

func (acc *BacktestAccount) removePosition(pos *position) {
	key := positionKey(pos.Symbol, pos.Side)
	delete(acc.positions, key)
}

func (acc *BacktestAccount) Open(symbol, side string, quantity float64, leverage int, price float64, ts int64, protection ...float64) (*position, float64, float64, error) {
	return acc.OpenWithCost(symbol, side, quantity, leverage, price, ts, ExecutionCost{FeeRate: acc.feeRate, SlippageRate: acc.slippageRate}, protection...)
}

func (acc *BacktestAccount) OpenWithCost(symbol, side string, quantity float64, leverage int, price float64, ts int64, cost ExecutionCost, protection ...float64) (*position, float64, float64, error) {
	if quantity <= 0 {
		return nil, 0, 0, fmt.Errorf("quantity must be positive")
	}
	if leverage <= 0 {
		return nil, 0, 0, fmt.Errorf("leverage must be positive")
	}

	execPrice := applySlippage(price, cost.SlippageRate, side, true)
	notional := execPrice * quantity
	initialRate, maintenanceRate := acc.marginRates(notional, leverage)
	margin := notional * initialRate
	fee := notional * cost.FeeRate

	if margin+fee > acc.cash+epsilon {
		return nil, 0, 0, fmt.Errorf("insufficient cash: need %.2f", margin+fee)
	}

	acc.cash -= margin + fee

	pos := acc.ensurePosition(symbol, side)

	if pos.Quantity < epsilon {
		pos.Quantity = quantity
		pos.EntryPrice = execPrice
		pos.Leverage = leverage
		pos.Margin = margin
		pos.Notional = notional
		pos.OpenTime = ts
		pos.MaintenanceRate = maintenanceRate
		pos.LiquidationPrice = computeTieredLiquidation(execPrice, initialRate, maintenanceRate, side)
		if len(protection) > 0 {
			pos.StopLoss = protection[0]
		}
		if len(protection) > 1 {
			pos.TakeProfit = protection[1]
		}
	} else {
		if leverage != pos.Leverage {
			// Use weighted average leverage (approximate)
			weightedMargin := pos.Margin + margin
			pos.Leverage = int(math.Round((pos.Notional + notional) / weightedMargin))
		}
		pos.Notional += notional
		pos.Margin += margin
		pos.EntryPrice = ((pos.EntryPrice * pos.Quantity) + execPrice*quantity) / (pos.Quantity + quantity)
		pos.Quantity += quantity
		_, pos.MaintenanceRate = acc.marginRates(pos.Notional, pos.Leverage)
		pos.LiquidationPrice = computeTieredLiquidation(pos.EntryPrice, pos.Margin/pos.Notional, pos.MaintenanceRate, side)
		if len(protection) > 0 {
			pos.StopLoss = protection[0]
		}
		if len(protection) > 1 {
			pos.TakeProfit = protection[1]
		}
	}

	return pos, fee, execPrice, nil
}

func (acc *BacktestAccount) Close(symbol, side string, quantity float64, price float64) (float64, float64, float64, error) {
	return acc.CloseWithCost(symbol, side, quantity, price, ExecutionCost{FeeRate: acc.feeRate, SlippageRate: acc.slippageRate})
}

func (acc *BacktestAccount) CloseWithCost(symbol, side string, quantity float64, price float64, cost ExecutionCost) (float64, float64, float64, error) {
	key := positionKey(symbol, side)
	pos, ok := acc.positions[key]
	if !ok || pos.Quantity <= epsilon {
		return 0, 0, 0, fmt.Errorf("no active %s position for %s", side, symbol)
	}

	if quantity <= 0 || quantity > pos.Quantity+epsilon {
		if math.Abs(quantity) <= epsilon {
			quantity = pos.Quantity
		} else {
			return 0, 0, 0, fmt.Errorf("invalid close quantity")
		}
	}

	execPrice := applySlippage(price, cost.SlippageRate, side, false)
	notional := execPrice * quantity
	fee := notional * cost.FeeRate

	realized := realizedPnL(pos, quantity, execPrice)

	closeRatio := quantity / pos.Quantity
	marginPortion := pos.Margin * closeRatio
	notionalPortion := pos.Notional * closeRatio
	acc.cash += marginPortion + realized - fee
	acc.realizedPnL += realized - fee

	pos.Quantity -= quantity
	pos.Notional -= notionalPortion
	pos.Margin -= marginPortion

	if pos.Quantity <= epsilon {
		acc.removePosition(pos)
	}

	return realized, fee, execPrice, nil
}

func (acc *BacktestAccount) ApplyFunding(markPrices map[string]float64, rate float64) float64 {
	total := 0.0
	for _, pos := range acc.positions {
		mark := markPrices[pos.Symbol]
		if mark <= 0 {
			mark = pos.EntryPrice
		}
		payment := mark * pos.Quantity * rate
		if pos.Side == "short" {
			payment = -payment
		}
		acc.cash -= payment
		acc.realizedPnL -= payment
		total += payment
	}
	return total
}

func (acc *BacktestAccount) marginRates(notional float64, leverage int) (float64, float64) {
	initial := 1 / float64(leverage)
	maintenance := 0.005
	for _, tier := range acc.marginTiers {
		if tier.NotionalCap <= 0 || notional <= tier.NotionalCap {
			if tier.InitialMarginRate > initial {
				initial = tier.InitialMarginRate
			}
			if tier.MaintenanceMarginRate > 0 {
				maintenance = tier.MaintenanceMarginRate
			}
			break
		}
	}
	return initial, maintenance
}

func (acc *BacktestAccount) TotalEquity(priceMap map[string]float64) (float64, float64, map[string]float64) {
	unrealized := 0.0
	margin := 0.0
	perSymbol := make(map[string]float64)
	for _, pos := range acc.positions {
		price := priceMap[pos.Symbol]
		pnl := unrealizedPnL(pos, price)
		unrealized += pnl
		margin += pos.Margin
		perSymbol[pos.Symbol+":"+pos.Side] = pnl
	}
	return acc.cash + margin + unrealized, unrealized, perSymbol
}

func applySlippage(price float64, rate float64, side string, isOpen bool) float64 {
	if rate <= 0 {
		return price
	}
	adjust := 1.0
	if side == "long" {
		if isOpen {
			adjust += rate
		} else {
			adjust -= rate
		}
	} else {
		if isOpen {
			adjust -= rate
		} else {
			adjust += rate
		}
	}
	return price * adjust
}

func computeTieredLiquidation(entry, initialRate, maintenanceRate float64, side string) float64 {
	if side == "long" {
		return entry * (1 - initialRate + maintenanceRate)
	}
	return entry * (1 + initialRate - maintenanceRate)
}

func realizedPnL(pos *position, qty, price float64) float64 {
	if pos.Side == "long" {
		return (price - pos.EntryPrice) * qty
	}
	return (pos.EntryPrice - price) * qty
}

func unrealizedPnL(pos *position, price float64) float64 {
	if pos.Side == "long" {
		return (price - pos.EntryPrice) * pos.Quantity
	}
	return (pos.EntryPrice - price) * pos.Quantity
}

func (acc *BacktestAccount) Positions() []*position {
	list := make([]*position, 0, len(acc.positions))
	for _, pos := range acc.positions {
		list = append(list, pos)
	}
	return list
}

func (acc *BacktestAccount) positionLeverage(symbol, side string) int {
	key := positionKey(symbol, side)
	if pos, ok := acc.positions[key]; ok && pos.Quantity > epsilon {
		return pos.Leverage
	}
	return 0
}

func (acc *BacktestAccount) Cash() float64 {
	return acc.cash
}

func (acc *BacktestAccount) InitialBalance() float64 {
	return acc.initialBalance
}

func (acc *BacktestAccount) RealizedPnL() float64 {
	return acc.realizedPnL
}

// RestoreFromSnapshots restores account state from checkpoint.
func (acc *BacktestAccount) RestoreFromSnapshots(cash float64, realized float64, snaps []PositionSnapshot) {
	acc.cash = cash
	acc.realizedPnL = realized
	acc.positions = make(map[string]*position)
	for _, snap := range snaps {
		pos := &position{
			Symbol:           snap.Symbol,
			Side:             snap.Side,
			Quantity:         snap.Quantity,
			EntryPrice:       snap.AvgPrice,
			Leverage:         snap.Leverage,
			Margin:           snap.MarginUsed,
			Notional:         snap.Quantity * snap.AvgPrice,
			LiquidationPrice: snap.LiquidationPrice,
			OpenTime:         snap.OpenTime,
			StopLoss:         snap.StopLoss,
			TakeProfit:       snap.TakeProfit,
			MaintenanceRate:  snap.MaintenanceRate,
		}
		key := positionKey(pos.Symbol, pos.Side)
		acc.positions[key] = pos
	}
}
