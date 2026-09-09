// Package notification implements the trading event notification system.
//
// Traders (and other subsystems) publish typed events to a global bus; the
// service fans them out to per-user channels (in-app, Telegram, webhook,
// email) according to each user's stored preferences, records an in-app
// history, and supports quiet hours plus per-event subscriptions.
package notification

import "time"

// Severity of an event. Critical events bypass quiet hours.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Event types published by the trading core.
const (
	EventPositionOpened  = "position.opened"
	EventPositionClosed  = "position.closed"
	EventPartialClose    = "position.partial_closed"
	EventStopLossTrigger = "position.stop_loss"
	EventTakeProfitHit   = "position.take_profit"
	EventLiquidationRisk = "position.liquidation_risk"
	EventDrawdownAlert   = "position.drawdown_alert"
	EventRiskTriggered   = "risk.control_triggered"
	EventTraderStopped   = "trader.stopped"
	EventTraderError     = "trader.error"
	EventDailySummary    = "report.daily_summary"
	EventSystem          = "system.notice"
)

// Event is a trading event emitted by the system.
type Event struct {
	EventType  string    `json:"event_type"`
	Severity   Severity  `json:"severity"`
	UserID     string    `json:"user_id,omitempty"` // empty = broadcast to all users
	TraderID   string    `json:"trader_id,omitempty"`
	TraderName string    `json:"trader_name,omitempty"`
	Symbol     string    `json:"symbol,omitempty"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	Time       time.Time `json:"time"`
}

// IsCritical reports whether the event bypasses quiet hours.
func (e *Event) IsCritical() bool { return e.Severity == SeverityCritical }
