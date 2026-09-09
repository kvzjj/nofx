package notification

import "nofx/metrics"

// metricsIncSent bumps the delivered-notification counter. Kept in its own
// file to avoid importing metrics from the hot path files.
func metricsIncSent() { metrics.NotificationsSent.Inc() }
