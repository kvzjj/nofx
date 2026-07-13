package store

import (
	"database/sql"
	"fmt"
	"time"
)

// ExecutionStore persists exchange orders, cumulative fills, and protection
// legs independently from the position projection.
type ExecutionStore struct {
	db *sql.DB
}

type TradeOrder struct {
	TraderID     string
	ExchangeID   string
	ExchangeType string
	OrderID      string
	Symbol       string
	PositionSide string
	Action       string
	RequestedQty float64
	ExecutedQty  float64
	AvgPrice     float64
	Fee          float64
	Status       string
	LastError    string
}

type ExchangeFill struct {
	TraderID     string
	ExchangeID   string
	TradeID      string
	OrderID      string
	Symbol       string
	PositionSide string
	Side         string
	Quantity     float64
	Price        float64
	Fee          float64
	RealizedPnL  float64
	ExecutedAt   time.Time
}

type ProtectionOrder struct {
	Kind         string
	Quantity     float64
	TriggerPrice float64
	AttemptCount int
	Status       string
}

func NewExecutionStore(db *sql.DB) *ExecutionStore {
	return &ExecutionStore{db: db}
}

func (s *ExecutionStore) InitTables() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS trade_orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trader_id TEXT NOT NULL,
			exchange_id TEXT NOT NULL DEFAULT '',
			exchange_type TEXT NOT NULL DEFAULT '',
			order_id TEXT NOT NULL,
			symbol TEXT NOT NULL,
			position_side TEXT NOT NULL,
			action TEXT NOT NULL,
			requested_qty REAL NOT NULL DEFAULT 0,
			executed_qty REAL NOT NULL DEFAULT 0,
			avg_price REAL NOT NULL DEFAULT 0,
			fee REAL NOT NULL DEFAULT 0,
			status TEXT NOT NULL,
			last_error TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			UNIQUE(trader_id, exchange_id, order_id)
		)`,
		`CREATE TABLE IF NOT EXISTS trade_fills (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trader_id TEXT NOT NULL,
			exchange_id TEXT NOT NULL DEFAULT '',
			order_id TEXT NOT NULL,
			symbol TEXT NOT NULL,
			position_side TEXT NOT NULL,
			executed_qty REAL NOT NULL,
			avg_price REAL NOT NULL,
			fee REAL NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL,
			UNIQUE(trader_id, exchange_id, order_id, executed_qty)
		)`,
		`CREATE TABLE IF NOT EXISTS protection_orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trader_id TEXT NOT NULL,
			exchange_id TEXT NOT NULL DEFAULT '',
			entry_order_id TEXT NOT NULL,
			symbol TEXT NOT NULL,
			position_side TEXT NOT NULL,
			kind TEXT NOT NULL,
			exchange_order_id TEXT NOT NULL DEFAULT '',
			quantity REAL NOT NULL,
			trigger_price REAL NOT NULL,
			status TEXT NOT NULL,
			attempt_count INTEGER NOT NULL DEFAULT 0,
			last_error TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			UNIQUE(trader_id, exchange_id, entry_order_id, kind)
		)`,
		`CREATE TABLE IF NOT EXISTS exchange_fills (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trader_id TEXT NOT NULL,
			exchange_id TEXT NOT NULL DEFAULT '',
			trade_id TEXT NOT NULL,
			order_id TEXT NOT NULL DEFAULT '',
			symbol TEXT NOT NULL,
			position_side TEXT NOT NULL DEFAULT '',
			side TEXT NOT NULL DEFAULT '',
			quantity REAL NOT NULL,
			price REAL NOT NULL,
			fee REAL NOT NULL DEFAULT 0,
			realized_pnl REAL NOT NULL DEFAULT 0,
			executed_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL,
			UNIQUE(trader_id, exchange_id, trade_id)
		)`,
	}
	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("failed to initialize execution tables: %w", err)
		}
	}
	// Migration for databases created before exchange-side protection IDs were tracked.
	_, _ = s.db.Exec(`ALTER TABLE protection_orders ADD COLUMN exchange_order_id TEXT NOT NULL DEFAULT ''`)
	return nil
}

// UpsertExchangeFill persists an immutable exchange fill by exchange trade ID.
func (s *ExecutionStore) UpsertExchangeFill(fill ExchangeFill) error {
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO exchange_fills (
			trader_id, exchange_id, trade_id, order_id, symbol, position_side,
			side, quantity, price, fee, realized_pnl, executed_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, fill.TraderID, fill.ExchangeID, fill.TradeID, fill.OrderID, fill.Symbol,
		fill.PositionSide, fill.Side, fill.Quantity, fill.Price, fill.Fee,
		fill.RealizedPnL, fill.ExecutedAt.Format(time.RFC3339Nano), time.Now().Format(time.RFC3339Nano))
	return err
}

func (s *ExecutionStore) LastExchangeFillTime(traderID, exchangeID string) (time.Time, error) {
	var value sql.NullString
	err := s.db.QueryRow(`
		SELECT MAX(executed_at) FROM exchange_fills WHERE trader_id = ? AND exchange_id = ?
	`, traderID, exchangeID).Scan(&value)
	if err != nil {
		return time.Time{}, err
	}
	if !value.Valid || value.String == "" {
		return time.Now().Add(-24 * time.Hour), nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse last exchange fill time: %w", err)
	}
	return parsed, nil
}

func (s *ExecutionStore) UpsertOrder(order TradeOrder) error {
	now := time.Now().Format(time.RFC3339Nano)
	_, err := s.db.Exec(`
		INSERT INTO trade_orders (
			trader_id, exchange_id, exchange_type, order_id, symbol, position_side,
			action, requested_qty, executed_qty, avg_price, fee, status, last_error,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(trader_id, exchange_id, order_id) DO UPDATE SET
			executed_qty = excluded.executed_qty, avg_price = excluded.avg_price,
			fee = excluded.fee, status = excluded.status, last_error = excluded.last_error,
			updated_at = excluded.updated_at
	`, order.TraderID, order.ExchangeID, order.ExchangeType, order.OrderID, order.Symbol,
		order.PositionSide, order.Action, order.RequestedQty, order.ExecutedQty, order.AvgPrice,
		order.Fee, order.Status, order.LastError, now, now)
	return err
}

func (s *ExecutionStore) HasActiveEntry(traderID, exchangeID, symbol, positionSide string) (bool, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM trade_orders
		WHERE trader_id = ? AND exchange_id = ? AND symbol = ? AND position_side = ?
			AND action IN ('open_long', 'open_short') AND status IN ('INTENT', 'SUBMITTED', 'PARTIAL')
	`, traderID, exchangeID, symbol, positionSide).Scan(&count)
	return count > 0, err
}

// ListActiveOrders returns local non-terminal orders that must continue to be
// reconciled after an in-memory monitor exits or the process restarts.
func (s *ExecutionStore) ListActiveOrders(traderID, exchangeID string) ([]TradeOrder, error) {
	rows, err := s.db.Query(`
		SELECT trader_id, exchange_id, exchange_type, order_id, symbol,
			position_side, action, requested_qty, executed_qty, avg_price,
			fee, status, last_error
		FROM trade_orders
		WHERE trader_id = ? AND exchange_id = ?
			AND status IN ('INTENT', 'SUBMITTED', 'PARTIAL')
		ORDER BY created_at
	`, traderID, exchangeID)
	if err != nil {
		return nil, fmt.Errorf("failed to list active orders: %w", err)
	}
	defer rows.Close()

	var orders []TradeOrder
	for rows.Next() {
		var order TradeOrder
		if err := rows.Scan(
			&order.TraderID, &order.ExchangeID, &order.ExchangeType,
			&order.OrderID, &order.Symbol, &order.PositionSide, &order.Action,
			&order.RequestedQty, &order.ExecutedQty, &order.AvgPrice,
			&order.Fee, &order.Status, &order.LastError,
		); err != nil {
			return nil, fmt.Errorf("failed to scan active order: %w", err)
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (s *ExecutionStore) RecordFill(order TradeOrder) error {
	if order.ExecutedQty <= 0 || order.AvgPrice <= 0 {
		return nil
	}
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO trade_fills (
			trader_id, exchange_id, order_id, symbol, position_side,
			executed_qty, avg_price, fee, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, order.TraderID, order.ExchangeID, order.OrderID, order.Symbol, order.PositionSide,
		order.ExecutedQty, order.AvgPrice, order.Fee, time.Now().Format(time.RFC3339Nano))
	return err
}

func (s *ExecutionStore) UpsertProtection(traderID, exchangeID, entryOrderID, symbol, positionSide, kind string, quantity, triggerPrice float64, status string, attempt int, lastError string) error {
	now := time.Now().Format(time.RFC3339Nano)
	_, err := s.db.Exec(`
		INSERT INTO protection_orders (
			trader_id, exchange_id, entry_order_id, symbol, position_side, kind,
			quantity, trigger_price, status, attempt_count, last_error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(trader_id, exchange_id, entry_order_id, kind) DO UPDATE SET
			quantity = excluded.quantity, trigger_price = excluded.trigger_price,
			status = excluded.status, attempt_count = excluded.attempt_count,
			last_error = excluded.last_error, updated_at = excluded.updated_at
	`, traderID, exchangeID, entryOrderID, symbol, positionSide, kind, quantity,
		triggerPrice, status, attempt, lastError, now, now)
	return err
}

func (s *ExecutionStore) UpsertReconciledProtection(traderID, exchangeID, entryOrderID, exchangeOrderID, symbol, positionSide, kind string, quantity, triggerPrice float64) error {
	now := time.Now().Format(time.RFC3339Nano)
	_, err := s.db.Exec(`
		INSERT INTO protection_orders (
			trader_id, exchange_id, entry_order_id, exchange_order_id, symbol,
			position_side, kind, quantity, trigger_price, status, attempt_count,
			last_error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'ACTIVE', 0, '', ?, ?)
		ON CONFLICT(trader_id, exchange_id, entry_order_id, kind) DO UPDATE SET
			exchange_order_id = excluded.exchange_order_id, quantity = excluded.quantity,
			trigger_price = excluded.trigger_price, status = 'ACTIVE', last_error = '',
			updated_at = excluded.updated_at
	`, traderID, exchangeID, entryOrderID, exchangeOrderID, symbol, positionSide,
		kind, quantity, triggerPrice, now, now)
	return err
}

// MarkProtectionsMissing clears stale local ACTIVE state before the current
// exchange snapshot is applied.
func (s *ExecutionStore) MarkProtectionsMissing(traderID, exchangeID, symbol, positionSide string) error {
	_, err := s.db.Exec(`
		UPDATE protection_orders
		SET status = 'MISSING', last_error = 'not present in exchange snapshot', updated_at = ?
		WHERE trader_id = ? AND exchange_id = ? AND symbol = ? AND position_side = ?
			AND status = 'ACTIVE'
	`, time.Now().Format(time.RFC3339Nano), traderID, exchangeID, symbol, positionSide)
	return err
}

func (s *ExecutionStore) ListProtectionOrders(traderID, exchangeID, entryOrderID string) ([]ProtectionOrder, error) {
	rows, err := s.db.Query(`
		SELECT kind, quantity, trigger_price, attempt_count, status
		FROM protection_orders
		WHERE trader_id = ? AND exchange_id = ? AND entry_order_id = ?
	`, traderID, exchangeID, entryOrderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ProtectionOrder
	for rows.Next() {
		var order ProtectionOrder
		if err := rows.Scan(&order.Kind, &order.Quantity, &order.TriggerPrice, &order.AttemptCount, &order.Status); err != nil {
			return nil, err
		}
		result = append(result, order)
	}
	return result, rows.Err()
}
