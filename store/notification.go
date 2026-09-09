package store

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// NotificationStore persists notification history and per-user delivery
// preferences.
type NotificationStore struct {
	db            *sql.DB
	encryptFunc   func(string) string
	decryptFunc   func(string) string
}

// NotificationRecord is one delivered (or in-app) notification.
type NotificationRecord struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TraderID  string    `json:"trader_id,omitempty"`
	TraderName string   `json:"trader_name,omitempty"`
	EventType string    `json:"event_type"`
	Severity  string    `json:"severity"` // info | warning | critical
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

// Delivery channels kept in sync with notification.Settings.
const (
	ChannelInApp     = "in_app"
	ChannelTelegram  = "telegram"
	ChannelWebhook   = "webhook"
	ChannelEmail     = "email"
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

func (s *NotificationStore) initTables() error {
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS notification_records (
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
		)
	`); err != nil {
		return err
	}
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_notification_records_user_time ON notification_records(user_id, created_at DESC)`)

	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS notification_settings (
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
		)
	`)
	return err
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
