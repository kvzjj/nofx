package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

// JWTSecret is the JWT secret key, will be dynamically set from config
var JWTSecret []byte

// tokenBlacklist for logged out tokens (memory only, cleaned by expiration time)
var tokenBlacklist = struct {
	sync.RWMutex
	items map[string]time.Time
}{items: make(map[string]time.Time)}

// maxBlacklistEntries is the maximum capacity threshold for blacklist
const maxBlacklistEntries = 100_000

// TokenStore is an optional persistent backend for the token blacklist.
// When set, blacklisted tokens survive process restarts.
type TokenStore interface {
	// BlacklistToken persists a revoked token hash until exp.
	BlacklistToken(tokenHash string, exp time.Time) error
	// IsTokenBlacklisted reports whether the token hash is revoked.
	IsTokenBlacklisted(tokenHash string) (bool, error)
}

// persistentTokens is the optional persistent blacklist backend.
var persistentTokens TokenStore

// SetTokenStore installs a persistent blacklist backend (write-through with
// read fallback). Passing nil reverts to memory-only operation.
func SetTokenStore(ts TokenStore) {
	persistentTokens = ts
}

// OTPIssuer is the OTP issuer name
const OTPIssuer = "nofxAI"

// SetJWTSecret sets the JWT secret key
func SetJWTSecret(secret string) {
	JWTSecret = []byte(secret)
}

// hashToken derives the storage key for a raw token. Only the hash is
// persisted so the blacklist table never contains usable credentials.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// BlacklistToken adds token to blacklist until expiration
func BlacklistToken(token string, exp time.Time) {
	tokenBlacklist.Lock()
	defer tokenBlacklist.Unlock()
	tokenBlacklist.items[token] = exp

	// If exceeds capacity threshold, perform expired cleanup; if still over limit, log warning
	if len(tokenBlacklist.items) > maxBlacklistEntries {
		now := time.Now()
		for t, e := range tokenBlacklist.items {
			if now.After(e) {
				delete(tokenBlacklist.items, t)
			}
		}
		if len(tokenBlacklist.items) > maxBlacklistEntries {
			log.Printf("auth: token blacklist size (%d) exceeds limit (%d) after sweep; consider reducing JWT TTL or using a shared persistent store",
				len(tokenBlacklist.items), maxBlacklistEntries)
		}
	}

	// Write-through to the persistent store so revocation survives restarts.
	if persistentTokens != nil {
		if err := persistentTokens.BlacklistToken(hashToken(token), exp); err != nil {
			log.Printf("auth: persist blacklist entry failed: %v", err)
		}
	}
}

// IsTokenBlacklisted checks if token is in blacklist (auto cleanup on expiration)
func IsTokenBlacklisted(token string) bool {
	tokenBlacklist.Lock()
	defer tokenBlacklist.Unlock()
	if exp, ok := tokenBlacklist.items[token]; ok {
		if time.Now().After(exp) {
			delete(tokenBlacklist.items, token)
			return false
		}
		return true
	}

	// Memory miss: consult the persistent store (restart scenario).
	if persistentTokens != nil {
		revoked, err := persistentTokens.IsTokenBlacklisted(hashToken(token))
		if err == nil && revoked {
			return true
		}
	}
	return false
}

// Claims represents JWT claims
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// HashPassword hashes the password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword verifies the password
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateOTPSecret generates OTP secret
func GenerateOTPSecret() (string, error) {
	secret := make([]byte, 20)
	_, err := rand.Read(secret)
	if err != nil {
		return "", err
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      OTPIssuer,
		AccountName: uuid.New().String(),
	})
	if err != nil {
		return "", err
	}

	return key.Secret(), nil
}

// VerifyOTP verifies OTP code
func VerifyOTP(secret, code string) bool {
	return totp.Validate(code, secret)
}

// GenerateJWT generates JWT token
func GenerateJWT(userID, email string) (string, error) {
	claims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), // Expires in 24 hours
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "nofxAI",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}

// ValidateJWT validates JWT token
func ValidateJWT(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return JWTSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// GetOTPQRCodeURL gets OTP QR code URL
func GetOTPQRCodeURL(secret, email string) string {
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s", OTPIssuer, email, secret, OTPIssuer)
}
