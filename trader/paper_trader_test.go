package trader

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"

	"nofx/market"
	"nofx/store"
)

// newTestPaperTrader builds a paper account with a deterministic price source.
func newTestPaperTrader(t *testing.T, initialBalance float64, price float64, withStore bool) (*PaperTrader, *gomonkey.Patches) {
	t.Helper()
	patches := gomonkey.NewPatches()
	patches.ApplyFunc(market.Get, func(symbol string) (*market.Data, error) {
		return &market.Data{Symbol: symbol, CurrentPrice: price}, nil
	})

	var st *store.Store
	if withStore {
		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatalf("open sqlite: %v", err)
		}
		t.Cleanup(func() { db.Close() })
		st = store.NewFromDB(db)
		if err := st.Paper().InitTables(); err != nil {
			t.Fatalf("init paper tables: %v", err)
		}
	}

	pt, err := NewPaperTrader("paper-test", initialBalance, st)
	if err != nil {
		t.Fatalf("NewPaperTrader: %v", err)
	}
	return pt, patches
}

func TestPaperMarketOpenClosePnLAndFees(t *testing.T) {
	pt, patches := newTestPaperTrader(t, 10000, 50000, false)
	defer patches.Reset()

	// Open 0.01 BTC long at ~50000 (+2bps slippage) with 10x leverage.
	order, err := pt.OpenLong("BTCUSDT", 0.01, 10)
	if err != nil {
		t.Fatalf("OpenLong: %v", err)
	}
	fillPrice := 50000 * (1 + paperSlippageBps/10000)
	if p := order["avgPrice"].(float64); p < fillPrice-1e-9 || p > fillPrice+1e-9 {
		t.Fatalf("fill price = %v, want %v", p, fillPrice)
	}
	if !pt.HasOpenPosition("BTCUSDT", "long") {
		t.Fatalf("position must exist after open")
	}

	positions, err := pt.GetPositions()
	if err != nil || len(positions) != 1 {
		t.Fatalf("GetPositions = %v, %v", positions, err)
	}
	if positions[0]["leverage"].(float64) != 10 {
		t.Fatalf("leverage = %v, want 10", positions[0]["leverage"])
	}

	// Close all at the same market price: realized PnL should equal the
	// slippage loss on entry + slippage loss on exit minus two taker fees.
	if _, err := pt.CloseLong("BTCUSDT", 0); err != nil {
		t.Fatalf("CloseLong: %v", err)
	}
	if pt.HasOpenPosition("BTCUSDT", "long") {
		t.Fatalf("position must be gone after full close")
	}

	// Absolute expectation: start 10000, minus entry fee, exit fee, and the
	// round-trip slippage loss.
	balanceAfter, _ := pt.GetBalance()
	walletAfter := balanceAfter["totalWalletBalance"].(float64)
	exitPrice := 50000 * (1 - paperSlippageBps/10000)
	entryFee := fillPrice * 0.01 * paperTakerFeeBps / 10000
	exitFee := exitPrice * 0.01 * paperTakerFeeBps / 10000
	roundTripPnL := (exitPrice - fillPrice) * 0.01
	want := 10000 - entryFee - exitFee + roundTripPnL
	if walletAfter < want-1e-9 || walletAfter > want+1e-9 {
		t.Fatalf("wallet after round trip = %.6f, want %.6f", walletAfter, want)
	}

	// The closed trade must be reported for position reconciliation.
	records, err := pt.GetClosedPnL(time.Time{}, 10)
	if err != nil || len(records) != 1 {
		t.Fatalf("GetClosedPnL = %v, %v", records, err)
	}
	if records[0].CloseType != "manual" || records[0].Symbol != "BTCUSDT" {
		t.Fatalf("closed record = %+v", records[0])
	}
}

func TestPaperPositionAveragingOnSecondEntry(t *testing.T) {
	pt, patches := newTestPaperTrader(t, 10000, 100, false)
	defer patches.Reset()

	if _, err := pt.OpenLong("ETHUSDT", 1, 2); err != nil {
		t.Fatalf("first open: %v", err)
	}
	positions, _ := pt.GetPositions()
	firstEntry := positions[0]["entryPrice"].(float64)

	if _, err := pt.OpenLong("ETHUSDT", 1, 2); err != nil {
		t.Fatalf("second open: %v", err)
	}
	positions, _ = pt.GetPositions()
	if positions[0]["positionAmt"].(float64) != 2 {
		t.Fatalf("qty = %v, want 2", positions[0]["positionAmt"])
	}
	// Same price both times -> average equals the first entry.
	if positions[0]["entryPrice"].(float64) != firstEntry {
		t.Fatalf("averaged entry = %v, want %v", positions[0]["entryPrice"], firstEntry)
	}
}

func TestPaperLimitEntryFillsWhenPriceCrosses(t *testing.T) {
	pt, patches := newTestPaperTrader(t, 10000, 100, false)
	defer patches.Reset()

	order, err := pt.OpenLongWithOptions("SOLUSDT", 5, 2, OrderOptions{Type: OrderTypeLimit, LimitPrice: 95})
	if err != nil {
		t.Fatalf("limit entry: %v", err)
	}
	orderID := strconv.FormatInt(order["orderId"].(int64), 10)

	// Price at 100 > 95: order rests.
	pt.matchRestingOrders()
	if pt.HasOpenPosition("SOLUSDT", "long") {
		t.Fatalf("limit must not fill above the limit price for a long")
	}
	status, _ := pt.GetOrderStatus("SOLUSDT", orderID)
	if status["status"] != "NEW" {
		t.Fatalf("status = %v, want NEW", status["status"])
	}

	// Price drops to 94: order fills at the limit price.
	patches.Reset()
	patches.ApplyFunc(market.Get, func(symbol string) (*market.Data, error) {
		return &market.Data{Symbol: symbol, CurrentPrice: 94}, nil
	})
	pt.matchRestingOrders()
	if !pt.HasOpenPosition("SOLUSDT", "long") {
		t.Fatalf("limit must fill once price crosses")
	}
	status, _ = pt.GetOrderStatus("SOLUSDT", orderID)
	if status["status"] != "FILLED" {
		t.Fatalf("status = %v, want FILLED", status["status"])
	}
	if status["avgPrice"].(float64) != 95 {
		t.Fatalf("fill price = %v, want limit 95", status["avgPrice"])
	}
}

func TestPaperStopLossTriggerClosesPosition(t *testing.T) {
	pt, patches := newTestPaperTrader(t, 10000, 100, false)
	defer patches.Reset()

	if _, err := pt.OpenShort("ETHUSDT", 1, 5); err != nil {
		t.Fatalf("open short: %v", err)
	}
	// Protective stop for a SHORT sits above entry.
	if err := pt.SetStopLoss("ETHUSDT", "SHORT", 1, 105); err != nil {
		t.Fatalf("SetStopLoss: %v", err)
	}

	patches.Reset()
	patches.ApplyFunc(market.Get, func(symbol string) (*market.Data, error) {
		return &market.Data{Symbol: symbol, CurrentPrice: 106}, nil
	})
	pt.matchRestingOrders()

	if pt.HasOpenPosition("ETHUSDT", "short") {
		t.Fatalf("stop-loss must close the short")
	}
	records, _ := pt.GetClosedPnL(time.Time{}, 10)
	if len(records) != 1 || records[0].CloseType != "stop_loss" {
		t.Fatalf("closed records = %+v", records)
	}
}

func TestPaperCancelProtectionOrders(t *testing.T) {
	pt, patches := newTestPaperTrader(t, 10000, 100, false)
	defer patches.Reset()

	if _, err := pt.OpenLong("BTCUSDT", 0.01, 5); err != nil {
		t.Fatalf("open: %v", err)
	}
	_ = pt.SetStopLoss("BTCUSDT", "LONG", 0.01, 90)
	_ = pt.SetTakeProfit("BTCUSDT", "LONG", 0.01, 120)

	if err := pt.CancelProtectionOrders("BTCUSDT", "LONG", "STOP_LOSS"); err != nil {
		t.Fatalf("CancelProtectionOrders: %v", err)
	}
	open, _ := pt.ListOpenOrders()
	if len(open) != 1 || open[0].Kind != "TAKE_PROFIT" {
		t.Fatalf("open orders after cancel = %+v", open)
	}

	_ = pt.CancelPositionOrders("BTCUSDT", "LONG")
	open, _ = pt.ListOpenOrders()
	if len(open) != 0 {
		t.Fatalf("all orders must be canceled, got %+v", open)
	}
}

func TestPaperBalanceMath(t *testing.T) {
	pt, patches := newTestPaperTrader(t, 10000, 100, false)
	defer patches.Reset()

	// Long 1 @ ~100, 10x: margin ~10, no PnL at entry price.
	if _, err := pt.OpenLong("ETHUSDT", 1, 10); err != nil {
		t.Fatalf("open: %v", err)
	}
	balance, _ := pt.GetBalance()
	entryFill := 100 * (1 + paperSlippageBps/10000)
	entryFee := entryFill * paperTakerFeeBps / 10000
	// Equity = 10000 - entry fee + (mark - slipped entry) at mark 100.
	wantEquity := 10000 - entryFee + (100 - entryFill)
	if eq := balance["totalWalletBalance"].(float64); eq < wantEquity-1e-9 || eq > wantEquity+1e-9 {
		t.Fatalf("equity = %v, want ~%v", eq, wantEquity)
	}
	margin := 100.0 / 10
	walletOnly := 10000 - entryFee // balance without unrealized PnL
	available := balance["availableBalance"].(float64)
	if available < walletOnly-margin-1e-9 || available > walletOnly-margin+1e-9 {
		t.Fatalf("available = %v, want ~%v", available, walletOnly-margin)
	}

	// Price doubles: unrealized +100.
	patches.Reset()
	patches.ApplyFunc(market.Get, func(symbol string) (*market.Data, error) {
		return &market.Data{Symbol: symbol, CurrentPrice: 200}, nil
	})
	balance, _ = pt.GetBalance()
	wantUnrealized := 200 - entryFill
	if u := balance["totalUnrealizedProfit"].(float64); u < wantUnrealized-1e-9 || u > wantUnrealized+1e-9 {
		t.Fatalf("unrealized = %v, want ~%v", u, wantUnrealized)
	}
}

func TestPaperStatePersistenceRoundtrip(t *testing.T) {
	pt, patches := newTestPaperTrader(t, 5000, 100, true)
	defer patches.Reset()

	if _, err := pt.OpenLong("BTCUSDT", 0.1, 5); err != nil {
		t.Fatalf("open: %v", err)
	}
	_ = pt.SetTakeProfit("BTCUSDT", "LONG", 0.1, 150)

	raw, err := pt.store.Paper().LoadPaperState("paper-test")
	if err != nil || raw == "" {
		t.Fatalf("state not persisted: %q, %v", raw, err)
	}
	var saved paperState
	if err := json.Unmarshal([]byte(raw), &saved); err != nil {
		t.Fatalf("state corrupt: %v", err)
	}
	if len(saved.Positions) != 1 || len(saved.Resting()) != 1 {
		t.Fatalf("saved state = %+v", saved)
	}

	// A new instance resumes from the persisted state.
	patches.Reset()
	patches.ApplyFunc(market.Get, func(symbol string) (*market.Data, error) {
		return &market.Data{Symbol: symbol, CurrentPrice: 100}, nil
	})
	resumed, err := NewPaperTrader("paper-test", 9999, pt.store)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if !resumed.HasOpenPosition("BTCUSDT", "long") {
		t.Fatalf("resumed account must retain the open position")
	}
	balance, _ := resumed.GetBalance()
	if eq := balance["totalWalletBalance"].(float64); eq <= 0 || eq >= 5000 {
		t.Fatalf("resumed equity = %v, want slightly below 5000 after fees", eq)
	}
}

func TestPaperGetOrderStatusUnknownOrder(t *testing.T) {
	pt, patches := newTestPaperTrader(t, 10000, 100, false)
	defer patches.Reset()

	if _, err := pt.GetOrderStatus("BTCUSDT", "999999"); err == nil {
		t.Fatalf("unknown order must return an error")
	}
}
