package api

import (
	"net/http"
	"nofx/config"
	"nofx/metrics"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// rateLimiter implements a sliding-window counter keyed by an arbitrary key
// (typically client IP or user ID). It is safe for concurrent use.
type rateLimiter struct {
	mu      sync.Mutex
	window  time.Duration
	limit   int
	hits    map[string][]time.Time
	lastSwept time.Time
}

// newRateLimiter creates a limiter allowing `limit` requests per `window`.
func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		window: window,
		limit:  limit,
		hits:   make(map[string][]time.Time),
	}
}

// allow reports whether the key is within the limit, recording the attempt.
func (r *rateLimiter) allow(key string) bool {
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()

	// Periodic sweep to keep the map bounded (limit: 10k keys).
	if now.Sub(r.lastSwept) > time.Minute {
		for k, ts := range r.hits {
			if len(ts) == 0 || now.Sub(ts[len(ts)-1]) > r.window {
				delete(r.hits, k)
			}
		}
		r.lastSwept = now
	}

	// Drop entries outside the window.
	ts := r.hits[key]
	keep := ts[:0]
	for _, t := range ts {
		if now.Sub(t) <= r.window {
			keep = append(keep, t)
		}
	}
	if len(keep) >= r.limit {
		r.hits[key] = keep
		return false
	}
	r.hits[key] = append(keep, now)
	return true
}

// retryAfter returns how long until the oldest hit in the window expires.
func (r *rateLimiter) retryAfter(key string) time.Duration {
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	ts := r.hits[key]
	if len(ts) == 0 {
		return 0
	}
	d := r.window - now.Sub(ts[0])
	if d < 0 {
		return 0
	}
	return d
}

// rateLimitConfig groups the limits applied to different endpoint classes.
// All values are requests per minute and can be overridden via environment.
type rateLimitConfig struct {
	enabled     bool
	authPerMin  int // /api/login, /api/register, /api/verify-otp, /api/reset-password ...
	publicPerMin int // other unauthenticated endpoints
	userPerMin  int // authenticated endpoints
	sensitivePerMin int // /api/crypto/decrypt etc.
}

func loadRateLimitConfig() rateLimitConfig {
	return rateLimitConfig{
		enabled:     config.Get().RateLimitEnabled,
		authPerMin:  config.Get().RateLimitAuthPerMin,
		publicPerMin: config.Get().RateLimitPublicPerMin,
		userPerMin:  config.Get().RateLimitUserPerMin,
		sensitivePerMin: config.Get().RateLimitSensitivePerMin,
	}
}

// rateLimiters bundles the limiters used by the server.
type rateLimiters struct {
	cfg       rateLimitConfig
	auth      *rateLimiter
	public    *rateLimiter
	user      *rateLimiter
	sensitive *rateLimiter
}

func newRateLimiters() *rateLimiters {
	cfg := loadRateLimitConfig()
	return &rateLimiters{
		cfg:       cfg,
		auth:      newRateLimiter(cfg.authPerMin, time.Minute),
		public:    newRateLimiter(cfg.publicPerMin, time.Minute),
		user:      newRateLimiter(cfg.userPerMin, time.Minute),
		sensitive: newRateLimiter(cfg.sensitivePerMin, time.Minute),
	}
}

// clientIP resolves the requester IP. gin already honors X-Forwarded-For when
// configured (trusted proxies); fall back to RemoteAddr otherwise.
func clientIP(c *gin.Context) string {
	if ip := c.ClientIP(); ip != "" {
		return ip
	}
	return c.Request.RemoteAddr
}

// reject aborts the request with 429 and standard hints.
func (rl *rateLimiters) reject(c *gin.Context, limiter *rateLimiter, key string) {
	metrics.IncRateLimited()
	retry := limiter.retryAfter(key)
	c.Header("Retry-After", retry.Truncate(time.Second).String())
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"error":      "Too many requests, please try again later",
		"retry_after": int(retry.Seconds()) + 1,
	})
}

// authRateLimit protects credential endpoints (login / register / OTP /
// password reset) against brute-force. Keyed by client IP only: the user is
// not authenticated yet.
func (rl *rateLimiters) authRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.cfg.enabled {
			c.Next()
			return
		}
		key := clientIP(c)
		if !rl.auth.allow(key) {
			rl.reject(c, rl.auth, key)
			return
		}
		c.Next()
	}
}

// publicRateLimit protects unauthenticated endpoints (leaderboard, config,
// crypto endpoints ...). Keyed by client IP.
func (rl *rateLimiters) publicRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.cfg.enabled {
			c.Next()
			return
		}
		key := "ip:" + clientIP(c)
		if !rl.public.allow(key) {
			rl.reject(c, rl.public, key)
			return
		}
		// Sensitive endpoints get their own, stricter bucket.
		if c.Request.URL.Path == "/api/crypto/decrypt" {
			if !rl.sensitive.allow(key) {
				rl.reject(c, rl.sensitive, key)
				return
			}
		}
		c.Next()
	}
}

// userRateLimit protects authenticated endpoints. Keyed by user ID (falls
// back to IP when the identity is still unknown, e.g. invalid token).
func (rl *rateLimiters) userRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.cfg.enabled {
			c.Next()
			return
		}
		var key string
		if uid, ok := c.Get("user_id"); ok {
			key = "user:" + uid.(string)
		} else {
			key = "ip:" + clientIP(c)
		}
		if !rl.user.allow(key) {
			rl.reject(c, rl.user, key)
			return
		}
		c.Next()
	}
}
