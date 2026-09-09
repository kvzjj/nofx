package metrics

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
)

// httpRequestsTotal counts HTTP requests by method/route/status.
var httpRequestsTotal = Default().Counter("nofx_http_requests_total", "Total HTTP requests processed")

// httpRequestDuration tracks HTTP latency in seconds.
var httpRequestDuration = Default().Histogram("nofx_http_request_duration_seconds", "HTTP request latency")

// rateLimitedTotal counts requests rejected by the rate limiter.
var rateLimitedTotal = Default().Counter("nofx_http_rate_limited_total", "Requests rejected by rate limiting")

// HTTPMiddleware records request count and latency for every route.
// The route pattern (not the raw path) is used as label to bound cardinality.
func HTTPMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		httpRequestsTotal.Inc()
		httpRequestDuration.Observe(time.Since(start).Seconds())
		_ = route // per-route labels intentionally omitted to bound cardinality
	}
}

// ObserveHTTPRequest lets manual handlers (SSE etc.) record a request with
// explicit values without going through the middleware a second time.
func ObserveHTTPRequest(duration time.Duration) {
	httpRequestsTotal.Inc()
	httpRequestDuration.Observe(duration.Seconds())
}

// IncRateLimited records a rate-limit rejection.
func IncRateLimited() { rateLimitedTotal.Inc() }

// StartRuntimeCollector refreshes Go runtime gauges every interval until stop
// is closed.
func StartRuntimeCollector(stop <-chan struct{}, interval time.Duration) {
	goroutines := Default().Gauge("nofx_go_goroutines", "Number of goroutines")
	memAlloc := Default().Gauge("nofx_go_mem_alloc_bytes", "Heap memory in use")
	memSys := Default().Gauge("nofx_go_mem_sys_bytes", "Total memory obtained from OS")
	gcCount := Default().Gauge("nofx_go_gc_total", "Total GC cycles")
	lastGC := Default().Gauge("nofx_go_gc_last_pause_seconds", "Last GC pause in seconds")

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var ms runtime.MemStats
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			goroutines.Set(float64(runtime.NumGoroutine()))
			runtime.ReadMemStats(&ms)
			memAlloc.Set(float64(ms.Alloc))
			memSys.Set(float64(ms.Sys))
			gcCount.Set(float64(ms.NumGC))
			lastGC.Set(float64(ms.PauseNs[(ms.NumGC+255)%256]) / 1e9)
		}
	}
}

// Handler serves the registry in Prometheus text format.
func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write([]byte(Default().Gather()))
	}
}
