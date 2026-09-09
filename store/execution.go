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
	TraderID         string
	ExchangeID       string
	ExchangeType     string
	OrderID          string
	Symbol           string
	PositionSide     string
	Action           string
	RequestedQty     float64
	ExecutedQty      float64
	AvgPrice         float64
	Fee              float64
	Status           string
	LastError        string
	Leverage         int
	StopLoss         float64
	TakeProfit       float64
	ProtectedQty     float64
	ProtectionStatus string
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

// OrderRecord is a persisted trade order enriched with timestamps for display.
type OrderRecord struct {
	TradeOrder
	CreatedAt time.Time
	UpdatedAt time.Time
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
			leverage INTEGER NOT NULL DEFAULT 0,
			stop_loss REAL NOT NULL DEFAULT 0,
			take_profit REAL NOT NULL DEFAULT 0,
			protected_qty REAL NOT NULL DEFAULT 0,
			protection_status TEXT NOT NULL DEFAULT 'UNPROTECTED',
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
	_, _ = s.db.Exec(`ALTER TABLE trade_orders ADD COLUMN leverage INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.Exec(`ALTER TABLE trade_orders ADD COLUMN stop_loss REAL NOT NULL DEFAULT 0`)
	_, _ = s.db.Exec(`ALTER TABLE trade_orders ADD COLUMN take_profit REAL NOT NULL DEFAULT 0`)
	_, _ = s.db.Exec(`ALTER TABLE trade_orders ADD COLUMN protected_qty REAL NOT NULL DEFAULT 0`)
	_, _ = s.db.Exec(`ALTER TABLE trade_orders ADD COLUMN protection_status TEXT NOT NULL DEFAULT 'UNPROTECTED'`)
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
			leverage, stop_loss, take_profit, protected_qty, protection_status,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(trader_id, exchange_id, order_id) DO UPDATE SET
			executed_qty = excluded.executed_qty, avg_price = excluded.avg_price,
			fee = excluded.fee, status = excluded.status, last_error = excluded.last_error,
			leverage = CASE WHEN excluded.leverage > 0 THEN excluded.leverage ELSE trade_orders.leverage END,
			stop_loss = CASE WHEN excluded.stop_loss > 0 THEN excluded.stop_loss ELSE trade_orders.stop_loss END,
			take_profit = CASE WHEN excluded.take_profit > 0 THEN excluded.take_profit ELSE trade_orders.take_profit END,
			protected_qty = MAX(trade_orders.protected_qty, excluded.protected_qty),
			protection_status = CASE WHEN excluded.protection_status != '' THEN excluded.protection_status ELSE trade_orders.protection_status END,
			updated_at = excluded.updated_at
	`, order.TraderID, order.ExchangeID, order.ExchangeType, order.OrderID, order.Symbol,
		order.PositionSide, order.Action, order.RequestedQty, order.ExecutedQty, order.AvgPrice,
		order.Fee, order.Status, order.LastError, order.Leverage, order.StopLoss,
		order.TakeProfit, order.ProtectedQty, order.ProtectionStatus, now, now)
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
			fee, status, last_error, leverage, stop_loss, take_profit,
			protected_qty, protection_status
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
			&order.Fee, &order.Status, &order.LastError, &order.Leverage,
			&order.StopLoss, &order.TakeProfit, &order.ProtectedQty,
			&order.ProtectionStatus,
		); err != nil {
			return nil, fmt.Errorf("failed to scan active order: %w", err)
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

// ListRecoverableEntryOrders returns entry sagas which still require exchange
// monitoring or protection repair after a process restart.
func (s *ExecutionStore) ListRecoverableEntryOrders(traderID, exchangeID string) ([]TradeOrder, error) {
	rows, err := s.db.Query(`
		SELECT trader_id, exchange_id, exchange_type, order_id, symbol,
			position_side, action, requested_qty, executed_qty, avg_price,
			fee, status, last_error, leverage, stop_loss, take_profit,
			protected_qty, protection_status
		FROM trade_orders
		WHERE trader_id = ? AND exchange_id = ?
			AND action IN ('open_long', 'open_short')
			AND (status IN ('INTENT', 'SUBMITTED', 'PARTIAL')
				OR (executed_qty > protected_qty AND protection_status != 'PROTECTED'))
		ORDER BY created_at
	`, traderID, exchangeID)
	if err != nil {
		return nil, fmt.Errorf("failed to list recoverable entry orders: %w", err)
	}
	defer rows.Close()
	var orders []TradeOrder
	for rows.Next() {
		var order TradeOrder
		if err := rows.Scan(&order.TraderID, &order.ExchangeID, &order.ExchangeType,
			&order.OrderID, &order.Symbol, &order.PositionSide, &order.Action,
			&order.RequestedQty, &order.ExecutedQty, &order.AvgPrice, &order.Fee,
			&order.Status, &order.LastError, &order.Leverage, &order.StopLoss,
			&order.TakeProfit, &order.ProtectedQty, &order.ProtectionStatus); err != nil {
			return nil, fmt.Errorf("failed to scan recoverable entry order: %w", err)
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (s *ExecutionStore) FindEntryOrderForPosition(traderID, exchangeID, symbol, positionSide string) (*TradeOrder, error) {
	var order TradeOrder
	err := s.db.QueryRow(`
		SELECT trader_id, exchange_id, exchange_type, order_id, symbol,
			position_side, action, requested_qty, executed_qty, avg_price,
			fee, status, last_error, leverage, stop_loss, take_profit,
			protected_qty, protection_status
		FROM trade_orders
		WHERE trader_id = ? AND exchange_id = ? AND symbol = ? AND position_side = ?
			AND action IN ('open_long', 'open_short') AND executed_qty > 0
		ORDER BY updated_at DESC LIMIT 1
	`, traderID, exchangeID, symbol, positionSide).Scan(
		&order.TraderID, &order.ExchangeID, &order.ExchangeType, &order.OrderID,
		&order.Symbol, &order.PositionSide, &order.Action, &order.RequestedQty,
		&order.ExecutedQty, &order.AvgPrice, &order.Fee, &order.Status,
		&order.LastError, &order.Leverage, &order.StopLoss, &order.TakeProfit,
		&order.ProtectedQty, &order.ProtectionStatus)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (s *ExecutionStore) UpdateOrderProtection(traderID, exchangeID, orderID string, protectedQty float64, status string) error {
	_, err := s.db.Exec(`
		UPDATE trade_orders SET protected_qty = ?, protection_status = ?, updated_at = ?
		WHERE trader_id = ? AND exchange_id = ? AND order_id = ?
	`, protectedQty, status, time.Now().Format(time.RFC3339Nano), traderID, exchangeID, orderID)
	return err
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

// ListOrderHistory returns the most recent orders (newest first) with
// pagination, scoped to one trader + exchange account.
func (s *ExecutionStore) ListOrderHistory(traderID, exchangeID string, limit, offset int) ([]OrderRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.Query(`
		SELECT trader_id, exchange_id, exchange_type, order_id, symbol,
			position_side, action, requested_qty, executed_qty, avg_price,
			fee, status, last_error, leverage, stop_loss, take_profit,
			protected_qty, protection_status, created_at, updated_at
		FROM trade_orders
		WHERE trader_id = ? AND exchange_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, traderID, exchangeID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list order history: %w", err)
	}
	defer rows.Close()

	var orders []OrderRecord
	for rows.Next() {
		var rec OrderRecord
		var createdAt, updatedAt string
		if err := rows.Scan(
			&rec.TraderID, &rec.ExchangeID, &rec.ExchangeType,
			&rec.OrderID, &rec.Symbol, &rec.PositionSide, &rec.Action,
			&rec.RequestedQty, &rec.ExecutedQty, &rec.AvgPrice,
			&rec.Fee, &rec.Status, &rec.LastError, &rec.Leverage,
			&rec.StopLoss, &rec.TakeProfit, &rec.ProtectedQty,
			&rec.ProtectionStatus, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan order history row: %w", err)
		}
		rec.CreatedAt = parseStoredTime(createdAt)
		rec.UpdatedAt = parseStoredTime(updatedAt)
		orders = append(orders, rec)
	}
	return orders, rows.Err()
}

// CountOrders returns the total number of persisted orders for pagination.
func (s *ExecutionStore) CountOrders(traderID, exchangeID string) (int, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM trade_orders WHERE trader_id = ? AND exchange_id = ?
	`, traderID, exchangeID).Scan(&count)
	return count, err
}

// ListFillHistory returns the most recent exchange fills (newest first) with
// pagination. Fills carry realized PnL, fee and execution time.
func (s *ExecutionStore) ListFillHistory(traderID, exchangeID string, limit, offset int) ([]ExchangeFill, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.Query(`
		SELECT trade_id, order_id, symbol, position_side, side,
			quantity, price, fee, realized_pnl, executed_at
		FROM exchange_fills
		WHERE trader_id = ? AND exchange_id = ?
		ORDER BY executed_at DESC
		LIMIT ? OFFSET ?
	`, traderID, exchangeID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list fill history: %w", err)
	}
	defer rows.Close()

	fills := make([]ExchangeFill, 0, limit)
	for rows.Next() {
		var fill ExchangeFill
		var executedAt string
		if err := rows.Scan(
			&fill.TradeID, &fill.OrderID, &fill.Symbol, &fill.PositionSide,
			&fill.Side, &fill.Quantity, &fill.Price, &fill.Fee,
			&fill.RealizedPnL, &executedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan fill history row: %w", err)
		}
		fill.TraderID = traderID
		fill.ExchangeID = exchangeID
		fill.ExecutedAt = parseStoredTime(executedAt)
		fills = append(fills, fill)
	}
	return fills, rows.Err()
}

// CountFills returns the total number of persisted exchange fills for pagination.
func (s *ExecutionStore) CountFills(traderID, exchangeID string) (int, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM exchange_fills WHERE trader_id = ? AND exchange_id = ?
	`, traderID, exchangeID).Scan(&count)
	return count, err
}

// parseStoredTime parses a stored RFC3339 timestamp, falling back to now on
// malformed values so listings never break on legacy rows.
func parseStoredTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, value)
		if err != nil {
			return time.Time{}
		}
	}
	return parsed
}

// SymbolPnL aggregates realized PnL per symbol.
type SymbolPnL struct {
	Symbol       string  `json:"symbol"`
	RealizedPnL  float64 `json:"realized_pnl"`
	Fee          float64 `json:"fee"`
	ClosedTrades int     `json:"closed_trades"`
}

// ExecutionStats holds closed-trade performance metrics derived from fills.
type ExecutionStats struct {
	ClosedTrades     int         `json:"closed_trades"`
	WinningTrades    int         `json:"winning_trades"`
	LosingTrades     int         `json:"losing_trades"`
	WinRate          float64     `json:"win_rate"`
	TotalRealizedPnL float64     `json:"total_realized_pnl"`
	TotalFee         float64     `json:"total_fee"`
	GrossProfit      float64     `json:"gross_profit"`
	GrossLoss        float64     `json:"gross_loss"`
	ProfitFactor     float64     `json:"profit_factor"`
	AvgWin           float64     `json:"avg_win"`
	AvgLoss          float64     `json:"avg_loss"`
	PnLBySymbol      []SymbolPnL `json:"pnl_by_symbol"`
}

// GetExecutionStats computes win rate / profit factor / PnL breakdown from
// realized fills (fills with non-zero realized PnL are treated as closes).
func (s *ExecutionStore) GetExecutionStats(traderID, exchangeID string) (*ExecutionStats, error) {
	stats := &ExecutionStats{PnLBySymbol: []SymbolPnL{}}

	// Per-symbol aggregation
	rows, err := s.db.Query(`
		SELECT symbol,
			SUM(CASE WHEN realized_pnl != 0 THEN 1 ELSE 0 END) AS closed_trades,
			SUM(realized_pnl) AS realized_pnl,
			SUM(fee) AS fee
		FROM exchange_fills
		WHERE trader_id = ? AND exchange_id = ?
		GROUP BY symbol
		ORDER BY realized_pnl DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate symbol pnl: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item SymbolPnL
		if err := rows.Scan(&item.Symbol, &item.ClosedTrades, &item.RealizedPnL, &item.Fee); err != nil {
			return nil, fmt.Errorf("failed to scan symbol pnl: %w", err)
		}
		stats.PnLBySymbol = append(stats.PnLBySymbol, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Per-close aggregation for win rate / profit factor
	closeRows, err := s.db.Query(`
		SELECT realized_pnl, fee
		FROM exchange_fills
		WHERE trader_id = ? AND exchange_id = ? AND realized_pnl != 0
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query realized fills: %w", err)
	}
	defer closeRows.Close()

	for closeRows.Next() {
		var pnl, fee float64
		if err := closeRows.Scan(&pnl, &fee); err != nil {
			return nil, fmt.Errorf("failed to scan realized fill: %w", err)
		}
		stats.ClosedTrades++
		stats.TotalRealizedPnL += pnl
		stats.TotalFee += fee
		if pnl > 0 {
			stats.WinningTrades++
			stats.GrossProfit += pnl
		} else {
			stats.LosingTrades++
			stats.GrossLoss += -pnl
		}
	}
	if err := closeRows.Err(); err != nil {
		return nil, err
	}

	if stats.ClosedTrades > 0 {
		stats.WinRate = float64(stats.WinningTrades) / float64(stats.ClosedTrades) * 100
	}
	if stats.WinningTrades > 0 {
		stats.AvgWin = stats.GrossProfit / float64(stats.WinningTrades)
	}
	if stats.LosingTrades > 0 {
		stats.AvgLoss = stats.GrossLoss / float64(stats.LosingTrades)
	}
	if stats.GrossLoss > 0 {
		stats.ProfitFactor = stats.GrossProfit / stats.GrossLoss
	}
	return stats, nil
}
