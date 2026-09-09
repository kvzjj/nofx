package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// NotificationStore persists notification history, per-user delivery
// preferences, user-configured channels (Telegram / Webhook / Email) and
// delivery logs per user.
type NotificationStore struct {
	db            *sql.DB
	encryptFunc   func(string) string
	decryptFunc   func(string) string
}

// NotificationRecord is one delivered (or in-app) notification.
type NotificationRecord struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	TraderID   string    `json:"trader_id,omitempty"`
	TraderName string    `json:"trader_name,omitempty"`
	EventType  string    `json:"event_type"`
	Severity   string    `json:"severity"` // info | warning | critical
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	IsRead     bool      `json:"is_read"`
	CreatedAt  time.Time `json:"created_at"`
}

// NotificationChannel is a user-configured delivery channel.
type NotificationChannel struct {
	ID        string    `json:"id"`
	UserID    string    `json:"-"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // "telegram" | "webhook" | "email"
	Config    string    `json:"-"`   // JSON blob (secrets masked at API layer)
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// Populated on read (Config parsed into a map, secrets intact for owner).
	ConfigMap map[string]interface{} `json:"config,omitempty"`
}

// NotificationLog is one delivery attempt.
type NotificationLog struct {
	ID          int64     `json:"id"`
	UserID      string    `json:"-"`
	ChannelID   string    `json:"channel_id"`
	ChannelName string    `json:"channel_name"`
	ChannelType string    `json:"channel_type"`
	Event       string    `json:"event"`
	Title       string    `json:"title"`
	Message     string    `json:"message"`
	Success     bool      `json:"success"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Delivery channels kept in sync with notification.Settings.
const (
	ChannelInApp    = "in_app"
	ChannelTelegram = "telegram"
	ChannelWebhook  = "webhook"
	ChannelEmail    = "email"
)

// NotificationSettings are per-user notification preferences.
type NotificationSettings struct {
	UserID string `json:"user_id"`

	Enabled bool `json:"enabled"` // master switch (all channels)

	TelegramEnabled  bool   `json:"telegram_enabled"`
	TelegramBotToken string `json:"telegram_bot_token,omitempty"` // encrypted at rest
	TelegramChatID   string `json:"telegram_chat_id,omitempty"`

	WebhookEnabled bool   `json:"webhook_enabled"`
	WebhookURL     string `json:"webhook_url,omitempty"`
	WebhookSecret  string `json:"webhook_secret,omitempty"`

	EmailEnabled bool   `json:"email_enabled"`
	EmailTo      string `json:"email_to,omitempty"`
	SMTPHost     string `json:"smtp_host,omitempty"`
	SMTPPort     int    `json:"smtp_port,omitempty"`
	SMTPUsername string `json:"smtp_username,omitempty"`
	SMTPPassword string `json:"smtp_password,omitempty"` // encrypted at rest
	SMTPFrom     string `json:"smtp_from,omitempty"`
	UseTLS       bool   `json:"smtp_use_tls"`

	// EventSubscriptions maps event type -> enabled. Empty means all enabled.
	EventSubscriptions map[string]bool `json:"event_subscriptions,omitempty"`

	// QuietHours: suppress non-critical delivery inside this UTC window.
	QuietHoursStart int `json:"quiet_hours_start_utc,omitempty"` // 0-23, -1 disables
	QuietHoursEnd   int `json:"quiet_hours_end_utc,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DefaultNotificationSettings returns sensible defaults (in-app only).
func DefaultNotificationSettings(userID string) *NotificationSettings {
	return &NotificationSettings{
		UserID:             userID,
		Enabled:            true,
		EventSubscriptions: map[string]bool{},
		QuietHoursStart:    -1,
		QuietHoursEnd:      -1,
	}
}

// InitTables creates the notification tables (records, settings, channels, logs).
func (s *NotificationStore) InitTables() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS notification_records (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			trader_id TEXT NOT NULL DEFAULT '',
			trader_name TEXT NOT NULL DEFAULT '',
			event_type TEXT NOT NULL,
			severity TEXT NOT NULL DEFAULT 'info',
			title TEXT NOT NULL,
			body TEXT NOT NULL DEFAULT '',
			is_read INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_records_user_time ON notification_records(user_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS notification_settings (
			user_id TEXT PRIMARY KEY,
			enabled INTEGER NOT NULL DEFAULT 1,
			telegram_enabled INTEGER NOT NULL DEFAULT 0,
			telegram_bot_token TEXT NOT NULL DEFAULT '',
			telegram_chat_id TEXT NOT NULL DEFAULT '',
			webhook_enabled INTEGER NOT NULL DEFAULT 0,
			webhook_url TEXT NOT NULL DEFAULT '',
			webhook_secret TEXT NOT NULL DEFAULT '',
			email_enabled INTEGER NOT NULL DEFAULT 0,
			email_to TEXT NOT NULL DEFAULT '',
			smtp_host TEXT NOT NULL DEFAULT '',
			smtp_port INTEGER NOT NULL DEFAULT 587,
			smtp_username TEXT NOT NULL DEFAULT '',
			smtp_password TEXT NOT NULL DEFAULT '',
			smtp_from TEXT NOT NULL DEFAULT '',
			smtp_use_tls INTEGER NOT NULL DEFAULT 1,
			event_subscriptions TEXT NOT NULL DEFAULT '{}',
			quiet_hours_start INTEGER NOT NULL DEFAULT -1,
			quiet_hours_end INTEGER NOT NULL DEFAULT -1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
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

// SetCryptoFuncs wires optional at-rest encryption for credentials.
func (s *NotificationStore) SetCryptoFuncs(encrypt, decrypt func(string) string) {
	s.encryptFunc = encrypt
	s.decryptFunc = decrypt
}

func (s *NotificationStore) encrypt(v string) string {
	if s.encryptFunc != nil && v != "" {
		return s.encryptFunc(v)
	}
	return v
}

func (s *NotificationStore) decrypt(v string) string {
	if s.decryptFunc != nil && v != "" {
		return s.decryptFunc(v)
	}
	return v
}

// ---------------------------------------------------------------------------
// In-app notification records (notification_records)
// ---------------------------------------------------------------------------

// Insert stores one notification record.
func (s *NotificationStore) Insert(n *NotificationRecord) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}
	read := 0
	if n.IsRead {
		read = 1
	}
	_, err := s.db.Exec(`
		INSERT INTO notification_records (id, user_id, trader_id, trader_name, event_type, severity, title, body, is_read, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, n.ID, n.UserID, n.TraderID, n.TraderName, n.EventType, n.Severity, n.Title, n.Body, read, n.CreatedAt.UTC().Format(dbTimeLayout))
	return err
}

// List returns notifications for a user, newest first.
func (s *NotificationStore) List(userID string, limit, offset int) ([]*NotificationRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT id, user_id, trader_id, trader_name, event_type, severity, title, body, is_read, created_at
		FROM notification_records WHERE user_id = ?
		ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*NotificationRecord
	for rows.Next() {
		n := &NotificationRecord{}
		var createdAt string
		var read int
		if err := rows.Scan(&n.ID, &n.UserID, &n.TraderID, &n.TraderName, &n.EventType, &n.Severity, &n.Title, &n.Body, &read, &createdAt); err != nil {
			return nil, err
		}
		n.IsRead = read == 1
		n.CreatedAt = parseDBTime(createdAt)
		list = append(list, n)
	}
	return list, rows.Err()
}

// CountUnread returns the number of unread notifications.
func (s *NotificationStore) CountUnread(userID string) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM notification_records WHERE user_id = ? AND is_read = 0`, userID).Scan(&count)
	return count, err
}

// MarkRead marks one notification as read (ownership enforced).
func (s *NotificationStore) MarkRead(userID, id string) error {
	_, err := s.db.Exec(`UPDATE notification_records SET is_read = 1 WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// MarkAllRead marks every notification of the user as read.
func (s *NotificationStore) MarkAllRead(userID string) error {
	_, err := s.db.Exec(`UPDATE notification_records SET is_read = 1 WHERE user_id = ?`, userID)
	return err
}

// PruneBefore deletes notifications older than the cutoff (retention).
func (s *NotificationStore) PruneBefore(cutoff time.Time) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM notification_records WHERE created_at < ?`, cutoff.UTC().Format(dbTimeLayout))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ---------------------------------------------------------------------------
// Per-user delivery settings (notification_settings)
// ---------------------------------------------------------------------------

// GetSettings loads the user's notification settings, creating defaults when
// absent.
func (s *NotificationStore) GetSettings(userID string) (*NotificationSettings, error) {
	row := s.db.QueryRow(`
		SELECT user_id, enabled, telegram_enabled, telegram_bot_token, telegram_chat_id,
		       webhook_enabled, webhook_url, webhook_secret, email_enabled, email_to,
		       smtp_host, smtp_port, smtp_username, smtp_password, smtp_from, smtp_use_tls,
		       event_subscriptions, quiet_hours_start, quiet_hours_end, created_at, updated_at
		FROM notification_settings WHERE user_id = ?
	`, userID)

	st := &NotificationSettings{}
	var subs string
	var enabled, tgEnabled, whEnabled, emEnabled, useTLS int
	var createdAt, updatedAt string
	err := row.Scan(&st.UserID, &enabled, &tgEnabled, &st.TelegramBotToken, &st.TelegramChatID,
		&whEnabled, &st.WebhookURL, &st.WebhookSecret, &emEnabled, &st.EmailTo,
		&st.SMTPHost, &st.SMTPPort, &st.SMTPUsername, &st.SMTPPassword, &st.SMTPFrom, &useTLS,
		&subs, &st.QuietHoursStart, &st.QuietHoursEnd, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		def := DefaultNotificationSettings(userID)
		if err := s.SaveSettings(def); err != nil {
			return nil, err
		}
		return def, nil
	}
	if err != nil {
		return nil, err
	}

	st.Enabled = enabled == 1
	st.TelegramEnabled = tgEnabled == 1
	st.WebhookEnabled = whEnabled == 1
	st.EmailEnabled = emEnabled == 1
	st.UseTLS = useTLS == 1
	st.TelegramBotToken = s.decrypt(st.TelegramBotToken)
	st.SMTPPassword = s.decrypt(st.SMTPPassword)
	if err := json.Unmarshal([]byte(subs), &st.EventSubscriptions); err != nil {
		st.EventSubscriptions = map[string]bool{}
	}
	st.CreatedAt = parseDBTime(createdAt)
	st.UpdatedAt = parseDBTime(updatedAt)
	return st, nil
}

// SaveSettings persists the user's notification settings (credentials are
// encrypted at rest when crypto funcs are configured).
func (s *NotificationStore) SaveSettings(st *NotificationSettings) error {
	subs, err := json.Marshal(st.EventSubscriptions)
	if err != nil {
		subs = []byte("{}")
	}
	enabled, tgEnabled, whEnabled, emEnabled, useTLS := 0, 0, 0, 0, 0
	if st.Enabled {
		enabled = 1
	}
	if st.TelegramEnabled {
		tgEnabled = 1
	}
	if st.WebhookEnabled {
		whEnabled = 1
	}
	if st.EmailEnabled {
		emEnabled = 1
	}
	if st.UseTLS {
		useTLS = 1
	}
	if st.SMTPPort <= 0 {
		st.SMTPPort = 587
	}

	_, err = s.db.Exec(`
		INSERT INTO notification_settings (
			user_id, enabled, telegram_enabled, telegram_bot_token, telegram_chat_id,
			webhook_enabled, webhook_url, webhook_secret, email_enabled, email_to,
			smtp_host, smtp_port, smtp_username, smtp_password, smtp_from, smtp_use_tls,
			event_subscriptions, quiet_hours_start, quiet_hours_end, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			enabled = excluded.enabled,
			telegram_enabled = excluded.telegram_enabled,
			telegram_bot_token = excluded.telegram_bot_token,
			telegram_chat_id = excluded.telegram_chat_id,
			webhook_enabled = excluded.webhook_enabled,
			webhook_url = excluded.webhook_url,
			webhook_secret = excluded.webhook_secret,
			email_enabled = excluded.email_enabled,
			email_to = excluded.email_to,
			smtp_host = excluded.smtp_host,
			smtp_port = excluded.smtp_port,
			smtp_username = excluded.smtp_username,
			smtp_password = excluded.smtp_password,
			smtp_from = excluded.smtp_from,
			smtp_use_tls = excluded.smtp_use_tls,
			event_subscriptions = excluded.event_subscriptions,
			quiet_hours_start = excluded.quiet_hours_start,
			quiet_hours_end = excluded.quiet_hours_end,
			updated_at = excluded.updated_at
	`,
		st.UserID, enabled, tgEnabled, s.encrypt(st.TelegramBotToken), st.TelegramChatID,
		whEnabled, st.WebhookURL, st.WebhookSecret, emEnabled, st.EmailTo,
		st.SMTPHost, st.SMTPPort, st.SMTPUsername, s.encrypt(st.SMTPPassword), st.SMTPFrom, useTLS,
		string(subs), st.QuietHoursStart, st.QuietHoursEnd, time.Now().UTC().Format(dbTimeLayout), time.Now().UTC().Format(dbTimeLayout))
	return err
}

// ListUsersWithChannel returns user IDs that enabled the given channel. Used
// by system-level events.
func (s *NotificationStore) ListUsersWithChannel(channel string) ([]string, error) {
	col := ""
	switch channel {
	case ChannelTelegram:
		col = "telegram_enabled"
	case ChannelWebhook:
		col = "webhook_enabled"
	case ChannelEmail:
		col = "email_enabled"
	default:
		return nil, nil
	}
	rows, err := s.db.Query(`SELECT user_id FROM notification_settings WHERE enabled = 1 AND ` + col + ` = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ---------------------------------------------------------------------------
// User-configured channels (notification_channels)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Delivery logs (notification_logs)
// ---------------------------------------------------------------------------

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
