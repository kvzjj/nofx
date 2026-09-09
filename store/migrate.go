package store

import (
	"database/sql"
	"fmt"
	"nofx/logger"
	"sort"
)

// Migration is one versioned, idempotent schema change. Migrations run inside
// a transaction; the applied version is recorded in schema_migrations so an
// upgrade never re-applies old steps and a failed step aborts safely.
//
// NOTE: the historical CREATE TABLE / ALTER TABLE calls embedded in each
// sub-store's initTables() are kept and remain the source of truth for fresh
// databases. This framework governs FUTURE schema changes: new migrations
// must be registered here (not as loose ALTERs) so production upgrades are
// auditable and ordered.
type Migration struct {
	Version int    // monotonically increasing; 1..N, no gaps
	Name    string // short slug, shown in logs
	Up      func(tx *sql.Tx) error
}

// migrations registers all versioned migrations. Append only — never edit or
// reorder published entries.
var migrations = []Migration{
	{
		Version: 1,
		Name:    "baseline-audit-notification-metrics",
		Up: func(tx *sql.Tx) error {
			// Baseline for the new audit / notification tables. The CREATE IF
			// NOT EXISTS statements mirror the sub-store initTables and are a
			// no-op on databases that already created them at boot.
			if _, err := tx.Exec(`
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
				)`); err != nil {
				return err
			}
			if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_audit_logs_user_time ON audit_logs(user_id, created_at DESC)`); err != nil {
				return err
			}
			if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action)`); err != nil {
				return err
			}
			if _, err := tx.Exec(`
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
				)`); err != nil {
				return err
			}
			if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_notification_records_user_time ON notification_records(user_id, created_at DESC)`); err != nil {
				return err
			}
			if _, err := tx.Exec(`
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
				)`); err != nil {
				return err
			}
			if _, err := tx.Exec(`
				CREATE TABLE IF NOT EXISTS token_blacklist (
					token_hash TEXT PRIMARY KEY,
					expires_at TIMESTAMP NOT NULL
				)`); err != nil {
				return err
			}
			if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS backup_history (
				id TEXT PRIMARY KEY,
				path TEXT NOT NULL,
				size_bytes INTEGER NOT NULL DEFAULT 0,
				status TEXT NOT NULL DEFAULT 'ok',
				error TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`); err != nil {
				return err
			}
			return nil
		},
	},
}

// RunMigrations applies all pending migrations in order. It is called once at
// startup, after initTables (which keeps fresh installs working) — existing
// databases get any missing versioned steps recorded.
func RunMigrations(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied := map[int]bool{}
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("read schema_migrations: %w", err)
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	rows.Close()

	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })
	for i, m := range migrations {
		if i > 0 && m.Version != migrations[i-1].Version+1 {
			return fmt.Errorf("migration version gap at v%d", m.Version)
		}
		if applied[m.Version] {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if err := m.Up(tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration v%d (%s) failed: %w", m.Version, m.Name, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, name) VALUES (?, ?)`, m.Version, m.Name); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration v%d: %w", m.Version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration v%d: %w", m.Version, err)
		}
		logger.Infof("🗃️  Migration applied: v%d %s", m.Version, m.Name)
	}
	return nil
}

// MigrationVersions returns the applied migration versions (for /api/health).
func MigrationVersions(db *sql.DB) []int {
	rows, err := db.Query(`SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var vs []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err == nil {
			vs = append(vs, v)
		}
	}
	return vs
}
