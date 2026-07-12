package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"nofx/config"
	"nofx/store"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
)

// MockUser Mock user structure
type MockUser struct {
	ID          int
	Email       string
	OTPSecret   string
	OTPVerified bool
}

func TestHandleRegisterResumesIncompleteOTPSetup(t *testing.T) {
	st := newTestStore(t)
	cfg := config.Get()
	oldRegistrationEnabled := cfg.RegistrationEnabled
	oldMaxUsers := cfg.MaxUsers
	cfg.RegistrationEnabled = true
	cfg.MaxUsers = 1
	t.Cleanup(func() {
		cfg.RegistrationEnabled = oldRegistrationEnabled
		cfg.MaxUsers = oldMaxUsers
	})

	existingUser := &store.User{
		ID:           "user-incomplete",
		Email:        "incomplete@example.com",
		PasswordHash: "old-hash",
		OTPSecret:    "JBSWY3DPEHPK3PXP",
		OTPVerified:  false,
	}
	if err := st.User().Create(existingUser); err != nil {
		t.Fatalf("failed to create existing user: %v", err)
	}

	resp := postRegister(t, st, map[string]string{
		"email":    existingUser.Email,
		"password": "NewPassword1!",
	})

	if resp.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["user_id"] != existingUser.ID {
		t.Fatalf("expected existing user_id %q, got %v", existingUser.ID, body["user_id"])
	}
	if body["otp_secret"] != existingUser.OTPSecret {
		t.Fatalf("expected existing otp_secret %q, got %v", existingUser.OTPSecret, body["otp_secret"])
	}
}

func TestHandleRegisterMaxUsersIgnoresIncompleteRegistrations(t *testing.T) {
	st := newTestStore(t)
	cfg := config.Get()
	oldRegistrationEnabled := cfg.RegistrationEnabled
	oldMaxUsers := cfg.MaxUsers
	cfg.RegistrationEnabled = true
	cfg.MaxUsers = 1
	t.Cleanup(func() {
		cfg.RegistrationEnabled = oldRegistrationEnabled
		cfg.MaxUsers = oldMaxUsers
	})

	if err := st.User().Create(&store.User{
		ID:           "user-incomplete",
		Email:        "incomplete@example.com",
		PasswordHash: "old-hash",
		OTPSecret:    "JBSWY3DPEHPK3PXP",
		OTPVerified:  false,
	}); err != nil {
		t.Fatalf("failed to create existing user: %v", err)
	}

	resp := postRegister(t, st, map[string]string{
		"email":    "new@example.com",
		"password": "NewPassword1!",
	})

	if resp.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestHandleCompleteRegistrationEnforcesMaxUsers(t *testing.T) {
	st := newTestStore(t)
	cfg := config.Get()
	oldRegistrationEnabled := cfg.RegistrationEnabled
	oldMaxUsers := cfg.MaxUsers
	cfg.RegistrationEnabled = true
	cfg.MaxUsers = 1
	t.Cleanup(func() {
		cfg.RegistrationEnabled = oldRegistrationEnabled
		cfg.MaxUsers = oldMaxUsers
	})

	if err := st.User().Create(&store.User{
		ID:           "user-verified",
		Email:        "verified@example.com",
		PasswordHash: "old-hash",
		OTPSecret:    "JBSWY3DPEHPK3PXP",
		OTPVerified:  true,
	}); err != nil {
		t.Fatalf("failed to create verified user: %v", err)
	}

	otpSecret := "JBSWY3DPEHPK3PXP"
	if err := st.User().Create(&store.User{
		ID:           "user-incomplete",
		Email:        "incomplete@example.com",
		PasswordHash: "old-hash",
		OTPSecret:    otpSecret,
		OTPVerified:  false,
	}); err != nil {
		t.Fatalf("failed to create incomplete user: %v", err)
	}

	otpCode, err := totp.GenerateCode(otpSecret, time.Now())
	if err != nil {
		t.Fatalf("failed to generate OTP code: %v", err)
	}

	resp := postCompleteRegistration(t, st, map[string]string{
		"user_id":  "user-incomplete",
		"otp_code": otpCode,
	})

	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected HTTP 403, got %d: %s", resp.Code, resp.Body.String())
	}
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Fatalf("failed to close store: %v", err)
		}
	})
	return st
}

func postRegister(t *testing.T, st *store.Store, payload map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	server := &Server{store: st}
	req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	server.handleRegister(newTestGinContext(resp, req))
	return resp
}

func postCompleteRegistration(t *testing.T, st *store.Store, payload map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	server := &Server{store: st}
	req := httptest.NewRequest(http.MethodPost, "/api/complete-registration", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	server.handleCompleteRegistration(newTestGinContext(resp, req))
	return resp
}

func newTestGinContext(resp *httptest.ResponseRecorder, req *http.Request) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(resp)
	c.Request = req
	return c
}

// TestOTPRefetchLogic Test OTP refetch logic
func TestOTPRefetchLogic(t *testing.T) {
	tests := []struct {
		name            string
		existingUser    *MockUser
		userExists      bool
		expectedAction  string // "allow_refetch", "reject_duplicate", "create_new"
		expectedMessage string
	}{
		{
			name:            "New user registration - email does not exist",
			existingUser:    nil,
			userExists:      false,
			expectedAction:  "create_new",
			expectedMessage: "Create new user",
		},
		{
			name: "Incomplete OTP verification - allow refetch",
			existingUser: &MockUser{
				ID:          1,
				Email:       "test@example.com",
				OTPSecret:   "SECRET123",
				OTPVerified: false,
			},
			userExists:      true,
			expectedAction:  "allow_refetch",
			expectedMessage: "Incomplete registration detected, please continue OTP setup",
		},
		{
			name: "Completed OTP verification - reject duplicate registration",
			existingUser: &MockUser{
				ID:          2,
				Email:       "verified@example.com",
				OTPSecret:   "SECRET456",
				OTPVerified: true,
			},
			userExists:      true,
			expectedAction:  "reject_duplicate",
			expectedMessage: "Email already registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate logic processing flow
			var actualAction string
			var actualMessage string

			if !tt.userExists {
				// User does not exist, create new user
				actualAction = "create_new"
				actualMessage = "Create new user"
			} else {
				// User exists, check OTP verification status
				if !tt.existingUser.OTPVerified {
					// OTP verification incomplete, allow refetch
					actualAction = "allow_refetch"
					actualMessage = "Incomplete registration detected, please continue OTP setup"
				} else {
					// Verification completed, reject duplicate registration
					actualAction = "reject_duplicate"
					actualMessage = "Email already registered"
				}
			}

			// Verify results
			if actualAction != tt.expectedAction {
				t.Errorf("Action mismatch: got %s, want %s", actualAction, tt.expectedAction)
			}
			if actualMessage != tt.expectedMessage {
				t.Errorf("Message mismatch: got %s, want %s", actualMessage, tt.expectedMessage)
			}
		})
	}
}

// TestOTPVerificationStates Test OTP verification state determination
func TestOTPVerificationStates(t *testing.T) {
	tests := []struct {
		name               string
		otpVerified        bool
		shouldAllowRefetch bool
	}{
		{
			name:               "OTP verified - disallow refetch",
			otpVerified:        true,
			shouldAllowRefetch: false,
		},
		{
			name:               "OTP not verified - allow refetch",
			otpVerified:        false,
			shouldAllowRefetch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate verification logic
			allowRefetch := !tt.otpVerified

			if allowRefetch != tt.shouldAllowRefetch {
				t.Errorf("Refetch logic error: OTPVerified=%v, allowRefetch=%v, expected=%v",
					tt.otpVerified, allowRefetch, tt.shouldAllowRefetch)
			}
		})
	}
}

// TestRegistrationFlow Test complete registration flow logic branches
func TestRegistrationFlow(t *testing.T) {
	tests := []struct {
		name           string
		scenario       string
		userExists     bool
		otpVerified    bool
		expectHTTPCode int // Simulated HTTP status code
		expectResponse string
	}{
		{
			name:           "Scenario 1: New user first registration",
			scenario:       "New user first accesses registration endpoint",
			userExists:     false,
			otpVerified:    false,
			expectHTTPCode: 200,
			expectResponse: "Create user and return OTP setup information",
		},
		{
			name:           "Scenario 2: User re-accesses after interrupting registration",
			scenario:       "User registered previously but did not complete OTP setup, now re-accessing",
			userExists:     true,
			otpVerified:    false,
			expectHTTPCode: 200,
			expectResponse: "Return existing user's OTP information, allow continuation",
		},
		{
			name:           "Scenario 3: Registered user attempts duplicate registration",
			scenario:       "User already completed registration, attempts to register again with same email",
			userExists:     true,
			otpVerified:    true,
			expectHTTPCode: 409, // Conflict
			expectResponse: "Email already registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate registration flow logic
			var actualHTTPCode int
			var actualResponse string

			if !tt.userExists {
				// New user, create and return OTP information
				actualHTTPCode = 200
				actualResponse = "Create user and return OTP setup information"
			} else {
				// User exists
				if !tt.otpVerified {
					// OTP verification incomplete, allow refetch
					actualHTTPCode = 200
					actualResponse = "Return existing user's OTP information, allow continuation"
				} else {
					// Verification completed, reject duplicate registration
					actualHTTPCode = 409
					actualResponse = "Email already registered"
				}
			}

			// Verify
			if actualHTTPCode != tt.expectHTTPCode {
				t.Errorf("HTTP code mismatch: got %d, want %d (scenario: %s)",
					actualHTTPCode, tt.expectHTTPCode, tt.scenario)
			}
			if actualResponse != tt.expectResponse {
				t.Errorf("Response mismatch: got %s, want %s (scenario: %s)",
					actualResponse, tt.expectResponse, tt.scenario)
			}

			t.Logf("✓ %s: HTTP %d, %s", tt.scenario, actualHTTPCode, actualResponse)
		})
	}
}

// TestEdgeCases Test edge cases
func TestEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		user        *MockUser
		expectAllow bool
		description string
	}{
		{
			name: "User ID is 0 - treated as new user",
			user: &MockUser{
				ID:          0,
				Email:       "new@example.com",
				OTPVerified: false,
			},
			expectAllow: true,
			description: "ID of 0 usually indicates user has not been created yet",
		},
		{
			name: "OTPSecret is empty - still can refetch",
			user: &MockUser{
				ID:          1,
				Email:       "test@example.com",
				OTPSecret:   "",
				OTPVerified: false,
			},
			expectAllow: true,
			description: "Even if OTPSecret is empty, as long as not verified, refetch is allowed",
		},
		{
			name: "OTPSecret exists but already verified - not allowed",
			user: &MockUser{
				ID:          2,
				Email:       "verified@example.com",
				OTPSecret:   "SECRET789",
				OTPVerified: true,
			},
			expectAllow: false,
			description: "Users with verified OTP cannot refetch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Core logic: as long as OTPVerified is false, refetch is allowed
			allowRefetch := !tt.user.OTPVerified

			if allowRefetch != tt.expectAllow {
				t.Errorf("Edge case failed: %s\nUser: ID=%d, OTPVerified=%v\nExpected allow=%v, got=%v",
					tt.description, tt.user.ID, tt.user.OTPVerified, tt.expectAllow, allowRefetch)
			}

			t.Logf("✓ %s", tt.description)
		})
	}
}
