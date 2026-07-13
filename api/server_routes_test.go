package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResetPasswordRouteIsPubliclyRegistered(t *testing.T) {
	server := NewServer(nil, nil, nil, nil, 0)
	req := httptest.NewRequest(http.MethodPost, "/api/reset-password", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	server.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 from reset password validation, got %d: %s", resp.Code, resp.Body.String())
	}
}
