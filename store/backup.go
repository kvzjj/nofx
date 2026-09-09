package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"nofx/logger"

	"github.com/google/uuid"
)

// BackupConfig controls the scheduled SQLite backup job.
type BackupConfig struct {
	Enabled        bool
	Dir            string        // target directory
	Interval       time.Duration // between backups
	RetentionCount int           // keep newest N backups
}

// BackupService takes consistent SQLite backups via VACUUM INTO and prunes
// old ones according to retention. VACUUM INTO produces a compact, defragmented
// copy while the database stays online (journal_mode=DELETE + single writer
// connection keep the snapshot consistent for our access pattern).
type BackupService struct {
	store *storeShim
	cfg   BackupConfig
	stop  chan struct{}
	done  chan struct{}
}

// storeShim gives BackupService access to the raw *sql.DB without exporting
// it from Store.
type storeShim struct{ db *sql.DB }

// NewBackupService creates the service. Call Start to schedule runs.
func NewBackupService(db *sql.DB, cfg BackupConfig) *BackupService {
	return &BackupService{
		store: &storeShim{db: db},
		cfg:   cfg,
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
}

// Start launches the scheduled backup loop. The first run happens after one
// interval; use RunNow for an immediate backup.
func (b *BackupService) Start() {
	go func() {
		defer close(b.done)
		if !b.cfg.Enabled {
			return
		}
		ticker := time.NewTicker(b.cfg.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-b.stop:
				return
			case <-ticker.C:
				if _, err := b.RunNow("scheduled"); err != nil {
					logger.Errorf("🗄️  Scheduled backup failed: %v", err)
				}
			}
		}
	}()
}

// Stop terminates the loop and waits for it.
func (b *BackupService) Stop() {
	close(b.stop)
	<-b.done
}

// RunNow performs one backup immediately and prunes old copies. Returns the
// backup path.
func (b *BackupService) RunNow(trigger string) (string, error) {
	if err := os.MkdirAll(b.cfg.Dir, 0755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	name := fmt.Sprintf("nofx-backup-%s.db", time.Now().UTC().Format("20060102-150405"))
	target := filepath.Join(b.cfg.Dir, name)

	start := time.Now()
	if _, err := b.store.db.Exec(`VACUUM INTO ?`, target); err != nil {
		// Record failure for observability.
		b.record(target, 0, err)
		return "", fmt.Errorf("VACUUM INTO %s: %w", target, err)
	}

	var size int64
	if fi, err := os.Stat(target); err == nil {
		size = fi.Size()
	}
	b.record(target, size, nil)
	b.prune()

	logger.Infof("🗄️  Backup complete (%s): %s (%.1f KB in %s)",
		trigger, target, float64(size)/1024, time.Since(start).Truncate(time.Millisecond))
	return target, nil
}

func (b *BackupService) record(path string, size int64, runErr error) {
	status, msg := "ok", ""
	if runErr != nil {
		status, msg = "failed", runErr.Error()
	}
	_, _ = b.store.db.Exec(`
		INSERT INTO backup_history (id, path, size_bytes, status, error, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, uuid.New().String(), path, size, status, msg, time.Now().UTC().Format(dbTimeLayout))
}

// prune removes the oldest backups beyond the retention count.
func (b *BackupService) prune() {
	if b.cfg.RetentionCount <= 0 {
		return
	}
	entries, err := os.ReadDir(b.cfg.Dir)
	if err != nil {
		return
	}
	var backups []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "nofx-backup-") && strings.HasSuffix(e.Name(), ".db") {
			backups = append(backups, e.Name())
		}
	}
	sort.Strings(backups) // timestamp prefix => chronological
	excess := len(backups) - b.cfg.RetentionCount
	for i := 0; i < excess; i++ {
		p := filepath.Join(b.cfg.Dir, backups[i])
		if err := os.Remove(p); err == nil {
			logger.Infof("🗄️  Pruned old backup %s", p)
		}
	}
}

// ListBackups returns recent backup history rows.
func (b *BackupService) ListBackups(limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := b.store.db.Query(`
		SELECT path, size_bytes, status, error, created_at FROM backup_history
		ORDER BY created_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]interface{}
	for rows.Next() {
		var path, status, errMsg, createdAt string
		var size int64
		if err := rows.Scan(&path, &size, &status, &errMsg, &createdAt); err != nil {
			continue
		}
		out = append(out, map[string]interface{}{
			"path": path, "size_bytes": size, "status": status,
			"error": errMsg, "created_at": createdAt,
		})
	}
	return out, nil
}
