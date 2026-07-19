package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestExecutionStorePersistsLifecycleAndProtections(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "execution.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	order := TradeOrder{
		TraderID: "trader-1", ExchangeID: "account-1", ExchangeType: "binance",
		OrderID: "42", Symbol: "BTCUSDT", PositionSide: "LONG", Action: "open_long",
		RequestedQty: 1, Status: "SUBMITTED", Leverage: 5, StopLoss: 49000,
		TakeProfit: 52000, ProtectionStatus: "UNPROTECTED",
	}
	if err := st.Execution().UpsertOrder(order); err != nil {
		t.Fatal(err)
	}
	active, err := st.Execution().HasActiveEntry("trader-1", "account-1", "BTCUSDT", "LONG")
	if err != nil || !active {
		t.Fatalf("expected active submitted entry, active=%v err=%v", active, err)
	}
	order.Status = "PARTIAL"
	order.ExecutedQty = 0.4
	order.AvgPrice = 50000
	if err := st.Execution().UpsertOrder(order); err != nil {
		t.Fatal(err)
	}
	if err := st.Execution().RecordFill(order); err != nil {
		t.Fatal(err)
	}
	activeOrders, err := st.Execution().ListActiveOrders("trader-1", "account-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(activeOrders) != 1 || activeOrders[0].OrderID != "42" || activeOrders[0].ExecutedQty != 0.4 {
		t.Fatalf("unexpected active orders: %#v", activeOrders)
	}
	if activeOrders[0].StopLoss != 49000 || activeOrders[0].TakeProfit != 52000 || activeOrders[0].Leverage != 5 {
		t.Fatalf("entry protection intent was not persisted: %#v", activeOrders[0])
	}
	recoverable, err := st.Execution().ListRecoverableEntryOrders("trader-1", "account-1")
	if err != nil || len(recoverable) != 1 {
		t.Fatalf("expected partial order recovery, orders=%#v err=%v", recoverable, err)
	}
	if err := st.Execution().UpdateOrderProtection("trader-1", "account-1", "42", 0.4, "PROTECTED"); err != nil {
		t.Fatal(err)
	}
	if err := st.Execution().RecordFill(order); err != nil {
		t.Fatal(err)
	}

	var status string
	var executed float64
	if err := st.db.QueryRow(`SELECT status, executed_qty FROM trade_orders WHERE order_id = '42'`).Scan(&status, &executed); err != nil {
		t.Fatal(err)
	}
	if status != "PARTIAL" || executed != 0.4 {
		t.Fatalf("unexpected order state: status=%s executed=%v", status, executed)
	}
	order.Status = "FILLED"
	if err := st.Execution().UpsertOrder(order); err != nil {
		t.Fatal(err)
	}
	active, err = st.Execution().HasActiveEntry("trader-1", "account-1", "BTCUSDT", "LONG")
	if err != nil || active {
		t.Fatalf("expected terminal entry, active=%v err=%v", active, err)
	}
	activeOrders, err = st.Execution().ListActiveOrders("trader-1", "account-1")
	if err != nil || len(activeOrders) != 0 {
		t.Fatalf("expected no reconcilable terminal orders, orders=%#v err=%v", activeOrders, err)
	}
	unprotected := TradeOrder{
		TraderID: "trader-1", ExchangeID: "account-1", ExchangeType: "binance",
		OrderID: "43", Symbol: "ETHUSDT", PositionSide: "SHORT", Action: "open_short",
		RequestedQty: 0.2, ExecutedQty: 0.2, AvgPrice: 3000, Status: "FILLED",
		Leverage: 3, StopLoss: 3100, TakeProfit: 2800, ProtectionStatus: "UNPROTECTED",
	}
	if err := st.Execution().UpsertOrder(unprotected); err != nil {
		t.Fatal(err)
	}
	recoverable, err = st.Execution().ListRecoverableEntryOrders("trader-1", "account-1")
	if err != nil || len(recoverable) != 1 || recoverable[0].OrderID != "43" {
		t.Fatalf("expected terminal unprotected order recovery, orders=%#v err=%v", recoverable, err)
	}
	var fillCount int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM trade_fills WHERE order_id = '42'`).Scan(&fillCount); err != nil {
		t.Fatal(err)
	}
	if fillCount != 1 {
		t.Fatalf("expected idempotent cumulative fill, got %d rows", fillCount)
	}

	if err := st.Execution().UpsertProtection("trader-1", "account-1", "42", "BTCUSDT", "LONG", "STOP_LOSS", 0.4, 49000, "FAILED", 1, "timeout"); err != nil {
		t.Fatal(err)
	}
	if err := st.Execution().UpsertProtection("trader-1", "account-1", "42", "BTCUSDT", "LONG", "STOP_LOSS", 0.4, 49000, "ACTIVE", 2, ""); err != nil {
		t.Fatal(err)
	}
	var protectionStatus string
	var attempts int
	if err := st.db.QueryRow(`SELECT status, attempt_count FROM protection_orders WHERE entry_order_id = '42' AND kind = 'STOP_LOSS'`).Scan(&protectionStatus, &attempts); err != nil {
		t.Fatal(err)
	}
	if protectionStatus != "ACTIVE" || attempts != 2 {
		t.Fatalf("unexpected protection state: status=%s attempts=%d", protectionStatus, attempts)
	}
	if err := st.Execution().MarkProtectionsMissing("trader-1", "account-1", "BTCUSDT", "LONG"); err != nil {
		t.Fatal(err)
	}
	if err := st.db.QueryRow(`SELECT status FROM protection_orders WHERE entry_order_id = '42' AND kind = 'STOP_LOSS'`).Scan(&protectionStatus); err != nil {
		t.Fatal(err)
	}
	if protectionStatus != "MISSING" {
		t.Fatalf("expected exchange snapshot to invalidate stale protection, got %s", protectionStatus)
	}
	if err := st.Execution().UpsertReconciledProtection("trader-1", "account-1", "42", "stop-99", "BTCUSDT", "LONG", "STOP_LOSS", 0.4, 49000); err != nil {
		t.Fatal(err)
	}
	var exchangeProtectionID string
	if err := st.db.QueryRow(`SELECT exchange_order_id, status FROM protection_orders WHERE entry_order_id = '42' AND kind = 'STOP_LOSS'`).Scan(&exchangeProtectionID, &protectionStatus); err != nil {
		t.Fatal(err)
	}
	if exchangeProtectionID != "stop-99" || protectionStatus != "ACTIVE" {
		t.Fatalf("unexpected reconciled protection: id=%s status=%s", exchangeProtectionID, protectionStatus)
	}
	fillTime := time.Now().UTC().Truncate(time.Millisecond)
	exchangeFill := ExchangeFill{
		TraderID: "trader-1", ExchangeID: "account-1", TradeID: "trade-7", OrderID: "42",
		Symbol: "BTCUSDT", PositionSide: "LONG", Side: "BUY", Quantity: 0.4,
		Price: 50000, Fee: 0.1, ExecutedAt: fillTime,
	}
	if err := st.Execution().UpsertExchangeFill(exchangeFill); err != nil {
		t.Fatal(err)
	}
	if err := st.Execution().UpsertExchangeFill(exchangeFill); err != nil {
		t.Fatal(err)
	}
	var exchangeFillCount int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM exchange_fills WHERE trade_id = 'trade-7'`).Scan(&exchangeFillCount); err != nil {
		t.Fatal(err)
	}
	if exchangeFillCount != 1 {
		t.Fatalf("expected trade-id idempotency, got %d fills", exchangeFillCount)
	}
	lastFillTime, err := st.Execution().LastExchangeFillTime("trader-1", "account-1")
	if err != nil || !lastFillTime.Equal(fillTime) {
		t.Fatalf("unexpected fill cursor: time=%s err=%v", lastFillTime, err)
	}
}

func TestPositionStoreTracksProtectionAndPartialClose(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "position.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	position := &TraderPosition{
		TraderID: "trader-1", ExchangeID: "account-1", ExchangeType: "binance",
		Symbol: "ETHUSDT", Side: "LONG", Quantity: 2, EntryPrice: 3000,
		EntryOrderID: "entry-1", EntryTime: time.Now(), Leverage: 3,
	}
	if err := st.Position().Create(position); err != nil {
		t.Fatal(err)
	}
	exists, err := st.Position().HasOpenPositionForExchange("account-1", "ETHUSDT", "LONG")
	if err != nil || !exists {
		t.Fatalf("expected account-level open projection, exists=%v err=%v", exists, err)
	}
	if err := st.Position().UpdateProtectionStatus("trader-1", "entry-1", "UNPROTECTED"); err != nil {
		t.Fatal(err)
	}
	loaded, err := st.Position().GetOpenPositionBySymbol("trader-1", "ETHUSDT", "LONG")
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil || loaded.ProtectionStatus != "UNPROTECTED" {
		t.Fatalf("expected visible UNPROTECTED state, got %#v", loaded)
	}
	if err := st.Position().UpdateOpenPositionProjection(position.ID, 1.5, 3100, 5); err != nil {
		t.Fatal(err)
	}
	loaded, err = st.Position().GetOpenPositionBySymbol("trader-1", "ETHUSDT", "LONG")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Quantity != 1.5 || loaded.EntryPrice != 3100 || loaded.Leverage != 5 {
		t.Fatalf("exchange projection was not applied: %#v", loaded)
	}
	if err := st.Position().SetOpenPositionProtectionStatus("trader-1", "ETHUSDT", "LONG", "PROTECTED"); err != nil {
		t.Fatal(err)
	}
	if err := st.Position().ApplyPartialClose(position.ID, 1.25, 75, 0.2); err != nil {
		t.Fatal(err)
	}

	var quantity, pnl, fee float64
	var lifecycle, protection string
	if err := st.db.QueryRow(`SELECT quantity, realized_pnl, fee, status, protection_status FROM trader_positions WHERE id = ?`, position.ID).
		Scan(&quantity, &pnl, &fee, &lifecycle, &protection); err != nil {
		t.Fatal(err)
	}
	if quantity != 1.25 || pnl != 75 || fee != 0.2 || lifecycle != "OPEN" || protection != "PROTECTED" {
		t.Fatalf("unexpected partial position: qty=%v pnl=%v fee=%v lifecycle=%s protection=%s", quantity, pnl, fee, lifecycle, protection)
	}
}
