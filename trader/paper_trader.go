package trader

import (
	"encoding/json"
	"fmt"

	"strconv"
	"strings"
	"sync"
	"time"

	"nofx/logger"
	"nofx/market"
	"nofx/store"
)

// PaperTrader simulates a futures exchange account locally: real market
// prices drive fills, but no order ever leaves the process. It implements the
// full Trader interface plus the optional interfaces AutoTrader discovers at
// runtime (advanced orders, per-position cancel, protection orders, open
// order listing), so the live trading loop, risk controls, and audit trail
// run unchanged on top of it.
//
// Simulation model (deliberately simple but conservative):
//   - Market orders fill at the current price plus a fixed slippage buffer.
//   - Limit entries fill when the market price crosses the limit.
//   - Stop-loss / take-profit orders trigger when the price crosses the
//     trigger and fill at the trigger price plus slippage.
//   - Taker fee is charged on market fills, maker fee on limit fills.
//   - Wallet balance moves only by realized PnL and fees (cross-margin
//     accounting); margin usage is derived from position notional / leverage.
//   - Funding rates are NOT simulated; simulated PnL is therefore slightly
//     optimistic for positions held across funding timestamps.
//
// A background matcher goroutine evaluates resting orders every few seconds
// (prices come from the shared websocket-backed market monitor), so limit
// entries and protections fill between AI cycles just like on a real venue.
type PaperTrader struct {
	mu       sync.Mutex
	state    paperState
	store    *store.Store
	traderID string
	ticker   *time.Ticker
	stopCh   chan struct{}
	stopOnce sync.Once
	started  sync.Once
}

// Simulation parameters. Fees mirror typical futures tiers; slippage is a
// small fixed buffer so simulated fills are not unfairly perfect.
const (
	paperTakerFeeBps   = 5.0 // 0.05% per market fill
	paperMakerFeeBps   = 2.0 // 0.02% per limit fill
	paperSlippageBps   = 2.0 // 0.02% adverse buffer on market fills
	paperMatchInterval = 3 * time.Second
	paperInitBalance   = 10000.0
)

type paperState struct {
	Balance     float64         `json:"balance"` // wallet balance in USDT (realized PnL & fees applied)
	Positions   []paperPosition `json:"positions"`
	Orders      []paperOrder    `json:"orders"` // resting + recently filled/canceled (kept for polling)
	Trades      []paperTrade    `json:"trades"` // closed-trade records for GetClosedPnL
	NextOrderID int64           `json:"next_order_id"`
}

type paperPosition struct {
	Symbol     string    `json:"symbol"`
	Side       string    `json:"side"` // "long" | "short"
	Quantity   float64   `json:"quantity"`
	EntryPrice float64   `json:"entry_price"` // average entry
	Leverage   int       `json:"leverage"`
	OpenedAt   time.Time `json:"opened_at"`
}

// paperOrder kinds. LIMIT_ENTRY directions reuse Kind with Side ("long"/
// "short"); protections use the exchange position side ("LONG"/"SHORT").
const (
	paperKindLimitEntry = "LIMIT_ENTRY"
	paperKindStopLoss   = "STOP_LOSS"
	paperKindTakeProfit = "TAKE_PROFIT"
)

type paperOrder struct {
	ID        int64     `json:"id"`
	Kind      string    `json:"kind"`
	Symbol    string    `json:"symbol"`
	Side      string    `json:"side"` // long/short for entries, LONG/SHORT for protections
	Quantity  float64   `json:"quantity"`
	Price     float64   `json:"price"`     // limit or trigger price
	Leverage  int       `json:"leverage"`  // for entries
	Status    string    `json:"status"`    // NEW / FILLED / CANCELED
	AvgPrice  float64   `json:"avg_price"` // fill price once filled
	Fee       float64   `json:"fee"`       // fill fee once filled
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type paperTrade struct {
	Symbol      string    `json:"symbol"`
	Side        string    `json:"side"`
	EntryPrice  float64   `json:"entry_price"`
	ExitPrice   float64   `json:"exit_price"`
	Quantity    float64   `json:"quantity"`
	RealizedPnL float64   `json:"realized_pnl"`
	Fee         float64   `json:"fee"`
	Leverage    int       `json:"leverage"`
	EntryTime   time.Time `json:"entry_time"`
	ExitTime    time.Time `json:"exit_time"`
	OrderID     string    `json:"order_id"`
	CloseType   string    `json:"close_type"` // manual / stop_loss / take_profit
}

// NewPaperTrader creates (or resumes) a simulated account. State is restored
// from the store when the trader has traded before; otherwise the account
// starts with the given initial balance.
func NewPaperTrader(traderID string, initialBalance float64, st *store.Store) (*PaperTrader, error) {
	if initialBalance <= 0 {
		initialBalance = paperInitBalance
	}
	pt := &PaperTrader{
		traderID: traderID,
		state: paperState{
			Balance:     initialBalance,
			NextOrderID: 100000,
		},
		store:  st,
		stopCh: make(chan struct{}),
	}

	if st != nil {
		if raw, err := st.Paper().LoadPaperState(traderID); err != nil {
			return nil, fmt.Errorf("failed to load paper account state: %w", err)
		} else if raw != "" {
			if err := json.Unmarshal([]byte(raw), &pt.state); err != nil {
				return nil, fmt.Errorf("corrupt paper account state: %w", err)
			}
			logger.Infof("📒 Paper account resumed: balance %.2f USDT, %d open position(s), %d resting order(s)",
				pt.state.Balance, len(pt.state.Positions), len(pt.state.Resting()))
		}
	}
	return pt, nil
}

// Resting returns the orders still live in the simulation.
func (s *paperState) Resting() []paperOrder {
	var out []paperOrder
	for _, o := range s.Orders {
		if o.Status == "NEW" {
			out = append(out, o)
		}
	}
	return out
}

// persist snapshots the state. Called with the lock held.
func (pt *PaperTrader) persistLocked() {
	if pt.store == nil {
		return
	}
	data, err := json.Marshal(pt.state)
	if err != nil {
		logger.Warnf("📒 [paper] failed to serialize state: %v", err)
		return
	}
	if err := pt.store.Paper().SavePaperState(pt.traderID, string(data)); err != nil {
		logger.Warnf("📒 [paper] failed to persist state: %v", err)
	}
}

// Start begins the background matcher. Safe to call multiple times.
func (pt *PaperTrader) Start() {
	pt.started.Do(func() {
		pt.ticker = time.NewTicker(paperMatchInterval)
		go func() {
			for {
				select {
				case <-pt.ticker.C:
					pt.matchRestingOrders()
				case <-pt.stopCh:
					pt.ticker.Stop()
					return
				}
			}
		}()
		logger.Infof("📒 Paper matcher started (interval %s)", paperMatchInterval)
	})
}

// Stop terminates the background matcher.
func (pt *PaperTrader) Stop() {
	pt.stopOnce.Do(func() {
		close(pt.stopCh)
	})
}

// matchRestingOrders fills limit entries and protection orders whose trigger
// has been crossed by the live market price. Prices are fetched without the
// lock (network reads); fills then apply atomically under it.
func (pt *PaperTrader) matchRestingOrders() {
	pt.mu.Lock()
	resting := pt.state.Resting()
	symbols := make([]string, 0, len(resting))
	for _, o := range resting {
		symbols = append(symbols, o.Symbol)
	}
	pt.mu.Unlock()

	prices := make(map[string]float64, len(symbols))
	for _, symbol := range symbols {
		if _, seen := prices[symbol]; seen {
			continue
		}
		if data, err := market.Get(symbol); err == nil && data != nil && data.CurrentPrice > 0 {
			prices[symbol] = data.CurrentPrice
		}
	}
	if len(prices) == 0 {
		return
	}

	pt.mu.Lock()
	defer pt.mu.Unlock()
	changed := false
	for i := range pt.state.Orders {
		order := &pt.state.Orders[i]
		if order.Status != "NEW" {
			continue
		}
		price, ok := prices[order.Symbol]
		if !ok {
			continue
		}
		switch order.Kind {
		case paperKindLimitEntry:
			longBelow := order.Side == "long" && price <= order.Price
			shortAbove := order.Side == "short" && price >= order.Price
			if longBelow || shortAbove {
				pt.fillLimitEntryLocked(order, order.Price)
				changed = true
			}
		case paperKindStopLoss:
			longHit := order.Side == "LONG" && price <= order.Price
			shortHit := order.Side == "SHORT" && price >= order.Price
			if longHit || shortHit {
				pt.triggerProtectionLocked(order, "stop_loss")
				changed = true
			}
		case paperKindTakeProfit:
			longHit := order.Side == "LONG" && price >= order.Price
			shortHit := order.Side == "SHORT" && price <= order.Price
			if longHit || shortHit {
				pt.triggerProtectionLocked(order, "take_profit")
				changed = true
			}
		}
	}
	if changed {
		pt.persistLocked()
	}
}

func (pt *PaperTrader) triggerProtectionLocked(order *paperOrder, closeType string) {
	pt.closeAtLocked(order.Symbol, strings.ToLower(order.Side), order.Quantity, order.Price, closeType)
	order.Status = "FILLED"
	order.AvgPrice = order.Price
	order.UpdatedAt = time.Now()
}

func (pt *PaperTrader) fillLimitEntryLocked(order *paperOrder, fillPrice float64) {
	fee := fillPrice * order.Quantity * paperMakerFeeBps / 10000
	order.Status = "FILLED"
	order.AvgPrice = fillPrice
	order.Fee = fee
	order.UpdatedAt = time.Now()
	side := order.Side // "long"/"short"
	pt.applyFillLocked(order.Symbol, side, order.Quantity, fillPrice, order.Leverage)
	pt.state.Balance -= fee
}

// applyFillLocked merges an entry fill into positions (weighted average).
func (pt *PaperTrader) applyFillLocked(symbol, side string, quantity, price float64, leverage int) {
	if leverage <= 0 {
		leverage = 1
	}
	for i := range pt.state.Positions {
		p := &pt.state.Positions[i]
		if p.Symbol == symbol && p.Side == side {
			total := p.Quantity + quantity
			p.EntryPrice = (p.EntryPrice*p.Quantity + price*quantity) / total
			p.Quantity = total
			if leverage > p.Leverage {
				p.Leverage = leverage
			}
			return
		}
	}
	pt.state.Positions = append(pt.state.Positions, paperPosition{
		Symbol: symbol, Side: side, Quantity: quantity, EntryPrice: price,
		Leverage: leverage, OpenedAt: time.Now(),
	})
}

// closeAtLocked reduces a position at the given price, realizes PnL and fee,
// and records a closed trade. Missing/oversized quantities are clamped.
func (pt *PaperTrader) closeAtLocked(symbol, side string, quantity, price float64, closeType string) {
	for i := range pt.state.Positions {
		p := &pt.state.Positions[i]
		if p.Symbol != symbol || p.Side != side {
			continue
		}
		if quantity <= 0 || quantity > p.Quantity {
			quantity = p.Quantity
		}
		var pnl float64
		if side == "long" {
			pnl = (price - p.EntryPrice) * quantity
		} else {
			pnl = (p.EntryPrice - price) * quantity
		}
		fee := price * quantity * paperTakerFeeBps / 10000
		pt.state.Balance += pnl - fee

		pt.state.Trades = append(pt.state.Trades, paperTrade{
			Symbol: symbol, Side: side, EntryPrice: p.EntryPrice, ExitPrice: price,
			Quantity: quantity, RealizedPnL: pnl, Fee: fee, Leverage: p.Leverage,
			EntryTime: p.OpenedAt, ExitTime: time.Now(), CloseType: closeType,
		})
		if len(pt.state.Trades) > 500 {
			pt.state.Trades = pt.state.Trades[len(pt.state.Trades)-500:]
		}

		p.Quantity -= quantity
		if p.Quantity <= 1e-12 {
			pt.state.Positions = append(pt.state.Positions[:i], pt.state.Positions[i+1:]...)
		}
		return
	}
}

func (pt *PaperTrader) price(symbol string) (float64, error) {
	data, err := market.Get(symbol)
	if err != nil || data == nil || data.CurrentPrice <= 0 {
		return 0, fmt.Errorf("no market price available for %s: %v", symbol, err)
	}
	return data.CurrentPrice, nil
}

func (pt *PaperTrader) nextOrderIDLocked() int64 {
	pt.state.NextOrderID++
	return pt.state.NextOrderID
}

// newMarketEntry executes an immediate entry fill at price ± slippage.
func (pt *PaperTrader) newMarketEntry(symbol, side string, quantity float64, leverage int) (map[string]interface{}, error) {
	price, err := pt.price(symbol)
	if err != nil {
		return nil, err
	}
	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}
	slippage := price * paperSlippageBps / 10000
	fillPrice := price + slippage
	if side == "short" {
		fillPrice = price - slippage
	}
	fee := fillPrice * quantity * paperTakerFeeBps / 10000

	pt.mu.Lock()
	defer pt.mu.Unlock()
	id := pt.nextOrderIDLocked()
	feeCap := fee
	pt.applyFillLocked(symbol, side, quantity, fillPrice, leverage)
	pt.state.Balance -= fee
	pt.state.Orders = append(pt.state.Orders, paperOrder{
		ID: id, Kind: "MARKET", Symbol: symbol, Side: side, Quantity: quantity,
		Price: fillPrice, Leverage: leverage, Status: "FILLED",
		AvgPrice: fillPrice, Fee: feeCap, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	pt.persistLocked()
	return pt.orderResult(id, symbol, "MARKET", fillPrice, quantity, feeCap), nil
}

func (pt *PaperTrader) orderResult(id int64, symbol, orderType string, avgPrice, executedQty, fee float64) map[string]interface{} {
	return map[string]interface{}{
		"orderId":     id,
		"symbol":      symbol,
		"orderType":   orderType,
		"status":      "FILLED",
		"avgPrice":    avgPrice,
		"executedQty": executedQty,
		"commission":  fee,
	}
}

// newLimitEntry registers a resting limit order.
func (pt *PaperTrader) newLimitEntry(symbol, side string, quantity float64, leverage int, limitPrice float64) (map[string]interface{}, error) {
	if limitPrice <= 0 || quantity <= 0 {
		return nil, fmt.Errorf("limit price and quantity must be positive")
	}
	pt.mu.Lock()
	defer pt.mu.Unlock()
	id := pt.nextOrderIDLocked()
	pt.state.Orders = append(pt.state.Orders, paperOrder{
		ID: id, Kind: paperKindLimitEntry, Symbol: symbol, Side: side,
		Quantity: quantity, Price: limitPrice, Leverage: leverage,
		Status: "NEW", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	pt.persistLocked()
	return map[string]interface{}{
		"orderId": id, "symbol": symbol, "orderType": "LIMIT", "status": "NEW",
		"avgPrice": 0.0, "executedQty": 0.0, "commission": 0.0,
	}, nil
}

// ============================================================
// Trader interface
// ============================================================

func (pt *PaperTrader) GetBalance() (map[string]interface{}, error) {
	pt.mu.Lock()
	positions := append([]paperPosition(nil), pt.state.Positions...)
	balance := pt.state.Balance
	pt.mu.Unlock()

	unrealized := 0.0
	marginUsed := 0.0
	for _, p := range positions {
		price, err := pt.price(p.Symbol)
		if err != nil {
			continue
		}
		if p.Side == "long" {
			unrealized += (price - p.EntryPrice) * p.Quantity
		} else {
			unrealized += (p.EntryPrice - price) * p.Quantity
		}
		marginUsed += p.Quantity * price / float64(p.Leverage)
	}
	available := balance - marginUsed
	if available < 0 {
		available = 0
	}
	return map[string]interface{}{
		"totalWalletBalance":    balance + unrealized,
		"availableBalance":      available,
		"totalUnrealizedProfit": unrealized,
		"totalMarginBalance":    balance + unrealized,
	}, nil
}

func (pt *PaperTrader) GetPositions() ([]map[string]interface{}, error) {
	pt.mu.Lock()
	positions := append([]paperPosition(nil), pt.state.Positions...)
	pt.mu.Unlock()

	out := make([]map[string]interface{}, 0, len(positions))
	for _, p := range positions {
		price, err := pt.price(p.Symbol)
		if err != nil {
			continue
		}
		qty := p.Quantity
		if p.Side == "short" {
			qty = -qty
		}
		var unrealized float64
		if p.Side == "long" {
			unrealized = (price - p.EntryPrice) * p.Quantity
		} else {
			unrealized = (p.EntryPrice - price) * p.Quantity
		}
		// Rough isolated-margin liquidation estimate (maintenance ~0.5%).
		liq := p.EntryPrice * (1 - 0.95/float64(p.Leverage))
		if p.Side == "short" {
			liq = p.EntryPrice * (1 + 0.95/float64(p.Leverage))
		}
		out = append(out, map[string]interface{}{
			"symbol":           p.Symbol,
			"side":             p.Side,
			"positionAmt":      qty,
			"entryPrice":       p.EntryPrice,
			"markPrice":        price,
			"unRealizedProfit": unrealized,
			"leverage":         float64(p.Leverage),
			"liquidationPrice": liq,
			"updateTime":       p.OpenedAt.UnixMilli(),
		})
	}
	return out, nil
}

func (pt *PaperTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return pt.newMarketEntry(symbol, "long", quantity, leverage)
}

func (pt *PaperTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return pt.newMarketEntry(symbol, "short", quantity, leverage)
}

func (pt *PaperTrader) OpenLongWithOptions(symbol string, quantity float64, leverage int, options OrderOptions) (map[string]interface{}, error) {
	if options.Type == OrderTypeLimit {
		return pt.newLimitEntry(symbol, "long", quantity, leverage, options.LimitPrice)
	}
	return pt.OpenLong(symbol, quantity, leverage)
}

func (pt *PaperTrader) OpenShortWithOptions(symbol string, quantity float64, leverage int, options OrderOptions) (map[string]interface{}, error) {
	if options.Type == OrderTypeLimit {
		return pt.newLimitEntry(symbol, "short", quantity, leverage, options.LimitPrice)
	}
	return pt.OpenShort(symbol, quantity, leverage)
}

func (pt *PaperTrader) close(symbol, side string, quantity float64) (map[string]interface{}, error) {
	price, err := pt.price(symbol)
	if err != nil {
		return nil, err
	}
	slippage := price * paperSlippageBps / 10000
	fillPrice := price - slippage
	if side == "short" {
		fillPrice = price + slippage
	}

	pt.mu.Lock()
	defer pt.mu.Unlock()
	var posQty float64
	for _, p := range pt.state.Positions {
		if p.Symbol == symbol && p.Side == side {
			posQty = p.Quantity
			break
		}
	}
	if posQty <= 0 {
		return nil, fmt.Errorf("no open %s position on %s (paper)", side, symbol)
	}
	id := pt.nextOrderIDLocked()
	// Compute fee at the actual close quantity.
	effective := quantity
	if effective <= 0 || effective > posQty {
		effective = posQty
	}
	fee := fillPrice * effective * paperTakerFeeBps / 10000
	pt.closeAtLocked(symbol, side, quantity, fillPrice, "manual")
	pt.state.Orders = append(pt.state.Orders, paperOrder{
		ID: id, Kind: "MARKET", Symbol: symbol, Side: side, Quantity: effective,
		Price: fillPrice, Status: "FILLED", AvgPrice: fillPrice, Fee: fee,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	pt.persistLocked()
	return pt.orderResult(id, symbol, "MARKET", fillPrice, effective, fee), nil
}

func (pt *PaperTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	return pt.close(symbol, "long", quantity)
}

func (pt *PaperTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return pt.close(symbol, "short", quantity)
}

func (pt *PaperTrader) SetLeverage(symbol string, leverage int) error { return nil }

func (pt *PaperTrader) SetMarginMode(symbol string, isCrossMargin bool) error { return nil }

func (pt *PaperTrader) GetMarketPrice(symbol string) (float64, error) { return pt.price(symbol) }

func (pt *PaperTrader) addProtection(symbol, positionSide, kind string, quantity, trigger float64) error {
	if quantity <= 0 || trigger <= 0 {
		return fmt.Errorf("protection quantity and trigger price must be positive")
	}
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.state.Orders = append(pt.state.Orders, paperOrder{
		ID: pt.nextOrderIDLocked(), Kind: kind, Symbol: symbol, Side: positionSide,
		Quantity: quantity, Price: trigger, Status: "NEW",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	pt.persistLocked()
	return nil
}

func (pt *PaperTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	return pt.addProtection(symbol, positionSide, paperKindStopLoss, quantity, stopPrice)
}

func (pt *PaperTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return pt.addProtection(symbol, positionSide, paperKindTakeProfit, quantity, takeProfitPrice)
}

func (pt *PaperTrader) cancelWhere(predicate func(o paperOrder) bool) error {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	for i := range pt.state.Orders {
		if pt.state.Orders[i].Status == "NEW" && predicate(pt.state.Orders[i]) {
			pt.state.Orders[i].Status = "CANCELED"
			pt.state.Orders[i].UpdatedAt = time.Now()
		}
	}
	pt.persistLocked()
	return nil
}

func (pt *PaperTrader) CancelStopLossOrders(symbol string) error {
	return pt.cancelWhere(func(o paperOrder) bool {
		return o.Symbol == symbol && o.Kind == paperKindStopLoss
	})
}

func (pt *PaperTrader) CancelTakeProfitOrders(symbol string) error {
	return pt.cancelWhere(func(o paperOrder) bool {
		return o.Symbol == symbol && o.Kind == paperKindTakeProfit
	})
}

func (pt *PaperTrader) CancelAllOrders(symbol string) error {
	return pt.cancelWhere(func(o paperOrder) bool { return o.Symbol == symbol })
}

func (pt *PaperTrader) CancelStopOrders(symbol string) error {
	return pt.cancelWhere(func(o paperOrder) bool {
		return o.Symbol == symbol && (o.Kind == paperKindStopLoss || o.Kind == paperKindTakeProfit)
	})
}

// CancelPositionOrders implements PositionOrderCanceler.
func (pt *PaperTrader) CancelPositionOrders(symbol, positionSide string) error {
	return pt.cancelWhere(func(o paperOrder) bool {
		return o.Symbol == symbol && (o.Side == positionSide || o.Side == strings.ToLower(positionSide))
	})
}

// CancelProtectionOrders implements ProtectionOrderCanceler.
func (pt *PaperTrader) CancelProtectionOrders(symbol, positionSide, kind string) error {
	paperKind := ""
	switch kind {
	case "STOP_LOSS":
		paperKind = paperKindStopLoss
	case "TAKE_PROFIT":
		paperKind = paperKindTakeProfit
	default:
		return fmt.Errorf("unknown protection kind: %s", kind)
	}
	return pt.cancelWhere(func(o paperOrder) bool {
		return o.Symbol == symbol && o.Kind == paperKind && o.Side == positionSide
	})
}

// CancelOrder implements SingleOrderCanceler.
func (pt *PaperTrader) CancelOrder(symbol, orderID string) error {
	return pt.cancelWhere(func(o paperOrder) bool {
		return o.Symbol == symbol && strconv.FormatInt(o.ID, 10) == orderID
	})
}

// ListOpenOrders implements OpenOrderLister.
func (pt *PaperTrader) ListOpenOrders() ([]ExchangeOpenOrder, error) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	out := make([]ExchangeOpenOrder, 0)
	for _, o := range pt.state.Resting() {
		kind := ""
		if o.Kind == paperKindStopLoss || o.Kind == paperKindTakeProfit {
			kind = o.Kind
		}
		out = append(out, ExchangeOpenOrder{
			OrderID:      strconv.FormatInt(o.ID, 10),
			Symbol:       o.Symbol,
			PositionSide: o.Side,
			Kind:         kind,
			Status:       OrderStateSubmitted,
			Quantity:     o.Quantity,
			TriggerPrice: o.Price,
		})
	}
	return out, nil
}

func (pt *PaperTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return strconv.FormatFloat(quantity, 'f', 6, 64), nil
}

func (pt *PaperTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	for _, o := range pt.state.Orders {
		if strconv.FormatInt(o.ID, 10) != orderID || o.Symbol != symbol {
			continue
		}
		switch o.Status {
		case "FILLED":
			return map[string]interface{}{
				"status": "FILLED", "avgPrice": o.AvgPrice,
				"executedQty": o.Quantity, "commission": o.Fee,
			}, nil
		case "CANCELED":
			return map[string]interface{}{
				"status": "CANCELED", "avgPrice": o.AvgPrice,
				"executedQty": 0.0, "commission": 0.0,
			}, nil
		default:
			return map[string]interface{}{
				"status": "NEW", "avgPrice": 0.0,
				"executedQty": 0.0, "commission": 0.0,
			}, nil
		}
	}
	return nil, fmt.Errorf("order %s not found on %s (paper)", orderID, symbol)
}

func (pt *PaperTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if limit <= 0 {
		limit = 100
	}
	var out []ClosedPnLRecord
	for i := len(pt.state.Trades) - 1; i >= 0 && len(out) < limit; i-- {
		t := pt.state.Trades[i]
		if t.ExitTime.Before(startTime) {
			continue
		}
		out = append(out, ClosedPnLRecord{
			Symbol: t.Symbol, Side: t.Side, EntryPrice: t.EntryPrice, ExitPrice: t.ExitPrice,
			Quantity: t.Quantity, RealizedPnL: t.RealizedPnL, Fee: t.Fee, Leverage: t.Leverage,
			EntryTime: t.EntryTime, ExitTime: t.ExitTime, OrderID: t.OrderID, CloseType: t.CloseType,
		})
	}
	return out, nil
}

// HasOpenPosition reports whether a simulated position exists (test helper).
func (pt *PaperTrader) HasOpenPosition(symbol, side string) bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	for _, p := range pt.state.Positions {
		if p.Symbol == symbol && p.Side == side {
			return true
		}
	}
	return false
}

var _ Trader = (*PaperTrader)(nil)
var _ AdvancedOrderTrader = (*PaperTrader)(nil)
var _ PositionOrderCanceler = (*PaperTrader)(nil)
var _ ProtectionOrderCanceler = (*PaperTrader)(nil)
var _ SingleOrderCanceler = (*PaperTrader)(nil)
var _ OpenOrderLister = (*PaperTrader)(nil)
