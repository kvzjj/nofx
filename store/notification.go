package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// NotificationStore persists notification channels (Telegram / Webhook / Email)
// and delivery logs per user.
type NotificationStore struct {
	db *sql.DB
}

// NotificationChannel is a user-configured delivery channel.
type NotificationChannel struct {
	ID        string           `json:"id"`
	UserID    string           `json:"-"`
	Name      string           `json:"name"`
	Type      string           `json:"type"` // "telegram" | "webhook" | "email"
	Config    string           `json:"-"`   // JSON blob (secrets masked at API layer)
	Enabled   bool             `json:"enabled"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	// Populated on read (Config parsed into a map, secrets intact for owner).
	ConfigMap map[string]interface{} `json:"config,omitempty"`
}

// NotificationLog is one delivery attempt.
type NotificationLog struct {
	ID          int64      `json:"id"`
	UserID      string     `json:"-"`
	ChannelID   string     `json:"channel_id"`
	ChannelName string     `json:"channel_name"`
	ChannelType string     `json:"channel_type"`
	Event       string     `json:"event"`
	Title       string     `json:"title"`
	Message     string     `json:"message"`
	Success     bool       `json:"success"`
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

func NewNotificationStore(db *sql.DB) *NotificationStore {
	return &NotificationStore{db: db}
}

func (s *NotificationStore) InitTables() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS notification_channels (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			config TEXT NOT NULL DEFAULT '{}',
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS notification_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			channel_id TEXT NOT NULL,
			channel_name TEXT NOT NULL DEFAULT '',
			channel_type TEXT NOT NULL DEFAULT '',
			event TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT '',
			message TEXT NOT NULL DEFAULT '',
			success INTEGER NOT NULL DEFAULT 0,
			error TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_logs_user ON notification_logs(user_id, created_at DESC)`,
	}
	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("failed to initialize notification tables: %w", err)
		}
	}
	return nil
}

const channelColumns = `id, user_id, name, type, config, enabled, created_at, updated_at`

func scanChannel(row interface{ Scan(...interface{}) error }) (*NotificationChannel, error) {
	var ch NotificationChannel
	var config string
	var createdAt, updatedAt string
	var enabled int
	if err := row.Scan(&ch.ID, &ch.UserID, &ch.Name, &ch.Type, &config, &enabled, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	ch.Enabled = enabled != 0
	ch.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	ch.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	ch.Config = config
	return &ch, nil
}

// ListChannels returns all channels of a user.
func (s *NotificationStore) ListChannels(userID string) ([]*NotificationChannel, error) {
	rows, err := s.db.Query(`
		SELECT `+channelColumns+` FROM notification_channels
		WHERE user_id = ? ORDER BY created_at ASC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list notification channels: %w", err)
	}
	defer rows.Close()

	var channels []*NotificationChannel
	for rows.Next() {
		ch, err := scanChannel(rows)
		if err != nil {
			continue
		}
		channels = append(channels, ch)
	}
	return channels, rows.Err()
}

// GetChannel returns a single channel owned by userID.
func (s *NotificationStore) GetChannel(userID, channelID string) (*NotificationChannel, error) {
	row := s.db.QueryRow(`
		SELECT `+channelColumns+` FROM notification_channels
		WHERE user_id = ? AND id = ?
	`, userID, channelID)
	ch, err := scanChannel(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return ch, err
}

// CreateChannel inserts a new channel.
func (s *NotificationStore) CreateChannel(userID, name, channelType, config string, enabled bool) (*NotificationChannel, error) {
	now := time.Now().Format(time.RFC3339Nano)
	id := uuid.New().String()
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	_, err := s.db.Exec(`
		INSERT INTO notification_channels (id, user_id, name, type, config, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, userID, name, channelType, config, enabledInt, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create notification channel: %w", err)
	}
	return s.GetChannel(userID, id)
}

// UpdateChannel updates mutable fields of a channel.
func (s *NotificationStore) UpdateChannel(userID, channelID, name, channelType, config string, enabled bool) (*NotificationChannel, error) {
	now := time.Now().Format(time.RFC3339Nano)
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	result, err := s.db.Exec(`
		UPDATE notification_channels
		SET name = ?, type = ?, config = ?, enabled = ?, updated_at = ?
		WHERE user_id = ? AND id = ?
	`, name, channelType, config, enabledInt, now, userID, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to update notification channel: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return nil, sql.ErrNoRows
	}
	return s.GetChannel(userID, channelID)
}

// DeleteChannel removes a channel.
func (s *NotificationStore) DeleteChannel(userID, channelID string) error {
	_, err := s.db.Exec(`
		DELETE FROM notification_channels WHERE user_id = ? AND id = ?
	`, userID, channelID)
	return err
}

// InsertLog records a delivery attempt.
func (s *NotificationStore) InsertLog(logEntry *NotificationLog) error {
	_, err := s.db.Exec(`
		INSERT INTO notification_logs (
			user_id, channel_id, channel_name, channel_type, event, title,
			message, success, error, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, logEntry.UserID, logEntry.ChannelID, logEntry.ChannelName, logEntry.ChannelType,
		logEntry.Event, logEntry.Title, logEntry.Message, logEntry.Success,
		logEntry.Error, time.Now().Format(time.RFC3339Nano))
	return err
}

// ListLogs returns the most recent delivery logs of a user.
func (s *NotificationStore) ListLogs(userID string, limit int) ([]*NotificationLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT id, user_id, channel_id, channel_name, channel_type, event, title,
			message, success, error, created_at
		FROM notification_logs
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list notification logs: %w", err)
	}
	defer rows.Close()

	var logs []*NotificationLog
	for rows.Next() {
		var entry NotificationLog
		var successInt int
		var createdAt string
		if err := rows.Scan(&entry.ID, &entry.UserID, &entry.ChannelID, &entry.ChannelName,
			&entry.ChannelType, &entry.Event, &entry.Title, &entry.Message,
			&successInt, &entry.Error, &createdAt); err != nil {
			continue
		}
		entry.Success = successInt != 0
		entry.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		logs = append(logs, &entry)
	}
	return logs, rows.Err()
}

// CleanOldLogs removes delivery logs older than the given days.
func (s *NotificationStore) CleanOldLogs(days int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -days).Format(time.RFC3339Nano)
	result, err := s.db.Exec(`DELETE FROM notification_logs WHERE created_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
