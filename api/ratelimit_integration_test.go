package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// newRateLimitTestRouter builds a router with the same middleware wiring as
// production for the auth endpoint class, but with a tiny limit.
func newRateLimitTestRouter(limit int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	rl := newRateLimiters()
	rl.cfg.enabled = true
	rl.cfg.authPerMin = limit
	rl.auth = newRateLimiter(limit, rl.auth.window)

	r := gin.New()
	r.POST("/api/login", rl.authRateLimit(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func TestAuthRateLimitReturns429(t *testing.T) {
	r := newRateLimitTestRouter(3)

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/api/login", bytes.NewBufferString(`{}`)))
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: code = %d, want 200", i+1, w.Code)
		}
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/login", bytes.NewBufferString(`{}`)))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("4th request: code = %d, want 429", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not json: %v", err)
	}
	if body["error"] == nil {
		t.Fatal("429 body must contain error message")
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("429 must set Retry-After header")
	}
}

func TestRateLimitDisabledPassesThrough(t *testing.T) {
	rl := newRateLimiters()
	rl.cfg.enabled = false

	r := gin.New()
	handlerCalled := false
	r.POST("/api/login", rl.authRateLimit(), func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	for i := 0; i < 50; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/api/login", bytes.NewBufferString(`{}`)))
		if w.Code != http.StatusOK {
			t.Fatalf("request %d blocked while disabled", i)
		}
	}
	if !handlerCalled {
		t.Fatal("handler must run when limiter disabled")
	}
}
