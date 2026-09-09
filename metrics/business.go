package metrics

import "time"

// Business (trading domain) metrics. All gauges are updated by the traders /
// decision engine; keeping them in one place makes the scrape endpoint easy to
// reason about.

// ActiveTraders number of currently running AI traders.
var ActiveTraders = Default().Gauge("nofx_traders_active", "Number of running AI traders")

// OpenPositions total open positions across all traders.
var OpenPositions = Default().Gauge("nofx_positions_open", "Number of open positions")

// AICallsTotal counts AI model invocations by outcome.
var AICallsTotal = Default().Counter("nofx_ai_calls_total", "Total AI model invocations")

// AICallFailures counts failed AI model invocations.
var AICallFailures = Default().Counter("nofx_ai_call_failures_total", "Failed AI model invocations")

// AICallDuration tracks AI model latency in seconds.
var AICallDuration = Default().Histogram("nofx_ai_call_duration_seconds", "AI model invocation latency")

// DecisionsTotal counts AI decisions processed.
var DecisionsTotal = Default().Counter("nofx_decisions_total", "Total AI decisions processed")

// OrdersTotal counts submitted orders (all outcomes).
var OrdersTotal = Default().Counter("nofx_orders_total", "Total orders submitted to exchanges")

// OrderFailures counts failed order submissions.
var OrderFailures = Default().Counter("nofx_order_failures_total", "Failed order submissions")

// AuditEventsTotal counts recorded audit events.
var AuditEventsTotal = Default().Counter("nofx_audit_events_total", "Total audit events recorded")

// NotificationsSent counts delivered notifications by channel (in title only).
var NotificationsSent = Default().Counter("nofx_notifications_sent_total", "Total notifications delivered")

// NotifyAIFailure records a failed AI invocation.
func NotifyAICall(duration time.Duration, failed bool) {
	AICallsTotal.Inc()
	AICallDuration.Observe(duration.Seconds())
	if failed {
		AICallFailures.Inc()
	}
}
