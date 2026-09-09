package store

import (
	"database/sql"
	"fmt"
	"time"
)

// PaperStore persists paper-trading (simulated) account state so simulated
// balances, positions, and resting orders survive restarts.
type PaperStore struct {
	db *sql.DB
}

func (s *PaperStore) InitTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS paper_accounts (
			trader_id TEXT PRIMARY KEY,
			state TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create paper_accounts table: %w", err)
	}
	return nil
}

// LoadPaperState returns the persisted JSON state for a trader, or "" when the
// trader has never traded on paper.
func (s *PaperStore) LoadPaperState(traderID string) (string, error) {
	var state string
	err := s.db.QueryRow(`SELECT state FROM paper_accounts WHERE trader_id = ?`, traderID).Scan(&state)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to load paper state for %s: %w", traderID, err)
	}
	return state, nil
}

// SavePaperState upserts the JSON state snapshot for a trader.
func (s *PaperStore) SavePaperState(traderID, state string) error {
	_, err := s.db.Exec(`
		INSERT INTO paper_accounts (trader_id, state, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(trader_id) DO UPDATE SET
			state = excluded.state,
			updated_at = excluded.updated_at
	`, traderID, state, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to save paper state for %s: %w", traderID, err)
	}
	return nil
}

// ResetPaperState deletes the persisted state (used when a user wants to
// restart a simulation from the initial balance).
func (s *PaperStore) ResetPaperState(traderID string) error {
	_, err := s.db.Exec(`DELETE FROM paper_accounts WHERE trader_id = ?`, traderID)
	if err != nil {
		return fmt.Errorf("failed to reset paper state for %s: %w", traderID, err)
	}
	return nil
}
