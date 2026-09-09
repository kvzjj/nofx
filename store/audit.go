package store

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// AuditStore persists security-relevant audit events (logins, credential
// access, trader lifecycle, configuration changes).
type AuditStore struct {
	db *sql.DB
}

// AuditEvent is one auditable action.
type AuditEvent struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Email        string    `json:"email"`
	Action       string    `json:"action"`        // e.g. "login", "exchange.update", "crypto.decrypt"
	ResourceType string    `json:"resource_type"` // e.g. "exchange", "trader", "user"
	ResourceID   string    `json:"resource_id"`
	Detail       string    `json:"detail"`
	Status       string    `json:"status"` // "success" | "failure"
	IP           string    `json:"ip"`
	UserAgent    string    `json:"user_agent"`
	CreatedAt    time.Time `json:"created_at"`
}

// Well-known audit actions.
const (
	AuditActionLogin            = "auth.login"
	AuditActionLoginFailed      = "auth.login_failed"
	AuditActionLogout           = "auth.logout"
	AuditActionRegister         = "auth.register"
	AuditActionRegisterFailed   = "auth.register_failed"
	AuditActionOTPVerify        = "auth.otp_verify"
	AuditActionOTPFailed        = "auth.otp_failed"
	AuditActionPasswordReset    = "auth.password_reset"
	AuditActionCryptoDecrypt    = "crypto.decrypt"
	AuditActionExchangeCreate   = "exchange.create"
	AuditActionExchangeUpdate   = "exchange.update"
	AuditActionExchangeDelete   = "exchange.delete"
	AuditActionModelUpdate      = "model.update"
	AuditActionTraderCreate     = "trader.create"
	AuditActionTraderUpdate     = "trader.update"
	AuditActionTraderDelete     = "trader.delete"
	AuditActionTraderStart      = "trader.start"
	AuditActionTraderStop       = "trader.stop"
	AuditActionClosePosition    = "trader.close_position"
	AuditActionSyncBalance      = "trader.sync_balance"
	AuditActionResetPaper       = "trader.reset_paper"
	AuditActionNotificationTest = "notification.test"
	AuditActionNotificationUpdate = "notification.settings_update"
	AuditActionBackupRun        = "backup.run"
)

func (s *AuditStore) initTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS audit_logs (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT '',
			email TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL,
			resource_type TEXT NOT NULL DEFAULT '',
			resource_id TEXT NOT NULL DEFAULT '',
			detail TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'success',
			ip TEXT NOT NULL DEFAULT '',
			user_agent TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_audit_logs_user_time ON audit_logs(user_id, created_at DESC)`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action)`)
	return nil
}

// Record inserts an audit event. Failures are returned so the caller can log
// them, but they must never block the primary operation.
func (s *AuditStore) Record(e *AuditEvent) error {
	if e.ID == "" {
		e.ID = newAuditID()
	}
	if e.Status == "" {
		e.Status = "success"
	}
	_, err := s.db.Exec(`
		INSERT INTO audit_logs (id, user_id, email, action, resource_type, resource_id, detail, status, ip, user_agent, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		e.ID, e.UserID, e.Email, e.Action, e.ResourceType, e.ResourceID, e.Detail, e.Status, e.IP, e.UserAgent, time.Now().UTC().Format(dbTimeLayout))
	return err
}

// AuditQuery filters for listing audit events.
type AuditQuery struct {
	UserID  string
	Action  string
	Limit   int
	Offset  int
}

// List returns audit events matching the query, newest first.
func (s *AuditStore) List(q AuditQuery) ([]*AuditEvent, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	if q.UserID != "" {
		where += " AND user_id = ?"
		args = append(args, q.UserID)
	}
	if q.Action != "" {
		where += " AND action = ?"
		args = append(args, q.Action)
	}
	limit := q.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	args = append(args, limit, q.Offset)

	rows, err := s.db.Query(`
		SELECT id, user_id, email, action, resource_type, resource_id, detail, status, ip, user_agent, created_at
		FROM audit_logs `+where+`
		ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*AuditEvent
	for rows.Next() {
		e := &AuditEvent{}
		var createdAt string
		if err := rows.Scan(&e.ID, &e.UserID, &e.Email, &e.Action, &e.ResourceType, &e.ResourceID, &e.Detail, &e.Status, &e.IP, &e.UserAgent, &createdAt); err != nil {
			return nil, err
		}
		e.CreatedAt = parseDBTime(createdAt)
		events = append(events, e)
	}
	return events, rows.Err()
}

// PruneBefore deletes audit events older than the given time and returns the
// number of removed rows (retention policy).
func (s *AuditStore) PruneBefore(cutoff time.Time) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM audit_logs WHERE created_at < ?`, cutoff.UTC().Format(dbTimeLayout))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func newAuditID() string {
	return uuid.New().String()
}
