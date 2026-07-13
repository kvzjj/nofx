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
		RequestedQty: 1, Status: "SUBMITTED",
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
	if err := st.Position().ApplyPartialClose(position.ID, 1.25, 75, 0.2); err != nil {
		t.Fatal(err)
	}

	var quantity, pnl, fee float64
	var lifecycle, protection string
	if err := st.db.QueryRow(`SELECT quantity, realized_pnl, fee, status, protection_status FROM trader_positions WHERE id = ?`, position.ID).
		Scan(&quantity, &pnl, &fee, &lifecycle, &protection); err != nil {
		t.Fatal(err)
	}
	if quantity != 1.25 || pnl != 75 || fee != 0.2 || lifecycle != "OPEN" || protection != "UNPROTECTED" {
		t.Fatalf("unexpected partial position: qty=%v pnl=%v fee=%v lifecycle=%s protection=%s", quantity, pnl, fee, lifecycle, protection)
	}
}
