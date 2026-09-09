package store

import (
	"database/sql"
	"time"
)

// TokenBlacklistStore persists revoked JWT hashes so logout survives restarts.
// Implements auth.TokenStore.
type TokenBlacklistStore struct {
	db *sql.DB
}

func (s *TokenBlacklistStore) initTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS token_blacklist (
			token_hash TEXT PRIMARY KEY,
			expires_at TIMESTAMP NOT NULL
		)
	`)
	if err != nil {
		return err
	}
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_token_blacklist_expires ON token_blacklist(expires_at)`)
	return nil
}

// BlacklistToken stores a revoked token hash until exp.
func (s *TokenBlacklistStore) BlacklistToken(tokenHash string, exp time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO token_blacklist (token_hash, expires_at) VALUES (?, ?)
		ON CONFLICT(token_hash) DO UPDATE SET expires_at = excluded.expires_at
	`, tokenHash, exp.UTC().Format(dbTimeLayout))
	if err != nil {
		return err
	}
	s.pruneExpired()
	return nil
}

// IsTokenBlacklisted reports whether the hash is revoked and unexpired.
func (s *TokenBlacklistStore) IsTokenBlacklisted(tokenHash string) (bool, error) {
	var expiresAt string
	err := s.db.QueryRow(`SELECT expires_at FROM token_blacklist WHERE token_hash = ?`, tokenHash).Scan(&expiresAt)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	exp := parseDBTime(expiresAt)
	if time.Now().After(exp) {
		// Lazily drop the expired entry.
		_, _ = s.db.Exec(`DELETE FROM token_blacklist WHERE token_hash = ?`, tokenHash)
		return false, nil
	}
	return true, nil
}

// pruneExpired removes expired rows (cheap, called on writes only).
func (s *TokenBlacklistStore) pruneExpired() {
	_, _ = s.db.Exec(`DELETE FROM token_blacklist WHERE expires_at < ?`, time.Now().UTC().Format(dbTimeLayout))
}
