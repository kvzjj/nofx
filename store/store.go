// Package store provides unified database storage layer
// All database operations should go through this package
package store

import (
	"database/sql"
	"fmt"
	"nofx/logger"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// dbTimeLayout matches SQLite's CURRENT_TIMESTAMP output, which is always
// UTC. Timestamps must therefore be parsed in time.UTC; using time.Parse
// would silently reinterpret them in the server's local zone.
const dbTimeLayout = "2006-01-02 15:04:05"

// parseDBTime parses a SQLite timestamp as UTC, returning the zero time on
// malformed input (mirroring the previous `t, _ = time.Parse(...)` behavior).
// dbTimeLayouts covers every timestamp shape the driver may return:
// CURRENT_TIMESTAMP emits "2006-01-02 15:04:05", while bound time.Time (and
// even pre-formatted strings in TIMESTAMP-affinity columns) may be normalized
// by modernc.org/sqlite to RFC3339 "2006-01-02T15:04:05Z".
var dbTimeLayouts = []string{
	dbTimeLayout,
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05",
}

func parseDBTime(value string) time.Time {
	for _, layout := range dbTimeLayouts {
		if t, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return t
		}
	}
	return time.Time{}
}

// Store unified data storage interface
type Store struct {
	db *sql.DB

	// Sub-stores (lazy initialization)
	user      *UserStore
	aiModel   *AIModelStore
	exchange  *ExchangeStore
	trader    *TraderStore
	decision  *DecisionStore
	backtest  *BacktestStore
	position  *PositionStore
	execution *ExecutionStore
	strategy  *StrategyStore
	paper     *PaperStore
	equity    *EquityStore
	audit     *AuditStore
	notification *NotificationStore
	tokenBlacklist *TokenBlacklistStore

	// Encryption functions
	encryptFunc func(string) string
	decryptFunc func(string) string

	mu sync.RWMutex
}

// New creates new Store instance
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// SQLite configuration
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Enable foreign key constraints
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Use DELETE mode (traditional mode) to ensure Docker bind mount compatibility
	// Note: WAL mode causes data sync issues on macOS Docker
	if _, err := db.Exec("PRAGMA journal_mode=DELETE"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set journal_mode: %w", err)
	}

	// Set synchronous=FULL
	if _, err := db.Exec("PRAGMA synchronous=FULL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set synchronous: %w", err)
	}

	// Set busy_timeout
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set busy_timeout: %w", err)
	}

	s := &Store{db: db}

	// Initialize all table structures
	if err := s.initTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize table structure: %w", err)
	}

	// Initialize default data
	if err := s.initDefaultData(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize default data: %w", err)
	}

	logger.Info("✅ Database enabled DELETE mode and FULL sync")
	return s, nil
}

// NewFromDB creates Store from existing database connection
func NewFromDB(db *sql.DB) *Store {
	return &Store{db: db}
}

// SetCryptoFuncs sets encryption/decryption functions
func (s *Store) SetCryptoFuncs(encrypt, decrypt func(string) string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.encryptFunc = encrypt
	s.decryptFunc = decrypt

	// Update already initialized sub-stores
	if s.aiModel != nil {
		s.aiModel.encryptFunc = encrypt
		s.aiModel.decryptFunc = decrypt
	}
	if s.exchange != nil {
		s.exchange.encryptFunc = encrypt
		s.exchange.decryptFunc = decrypt
	}
	if s.trader != nil {
		s.trader.decryptFunc = decrypt
	}
	if s.notification != nil {
		s.notification.SetCryptoFuncs(encrypt, decrypt)
	}
}

// initTables initializes all database tables
func (s *Store) initTables() error {
	// Initialize in dependency order
	if err := s.User().initTables(); err != nil {
		return fmt.Errorf("failed to initialize user tables: %w", err)
	}
	if err := s.AIModel().initTables(); err != nil {
		return fmt.Errorf("failed to initialize AI model tables: %w", err)
	}
	if err := s.Exchange().initTables(); err != nil {
		return fmt.Errorf("failed to initialize exchange tables: %w", err)
	}
	if err := s.Trader().initTables(); err != nil {
		return fmt.Errorf("failed to initialize trader tables: %w", err)
	}
	if err := s.Decision().initTables(); err != nil {
		return fmt.Errorf("failed to initialize decision log tables: %w", err)
	}
	if err := s.Backtest().initTables(); err != nil {
		return fmt.Errorf("failed to initialize backtest tables: %w", err)
	}
	if err := s.Position().InitTables(); err != nil {
		return fmt.Errorf("failed to initialize position tables: %w", err)
	}
	if err := s.Execution().InitTables(); err != nil {
		return fmt.Errorf("failed to initialize execution tables: %w", err)
	}
	if err := s.Strategy().initTables(); err != nil {
		return fmt.Errorf("failed to initialize strategy tables: %w", err)
	}
	if err := s.Paper().InitTables(); err != nil {
		return fmt.Errorf("failed to initialize paper trading tables: %w", err)
	}
	if err := s.Equity().initTables(); err != nil {
		return fmt.Errorf("failed to initialize equity tables: %w", err)
	}
	if err := s.Audit().initTables(); err != nil {
		return fmt.Errorf("failed to initialize audit tables: %w", err)
	}
	if err := s.Notification().initTables(); err != nil {
		return fmt.Errorf("failed to initialize notification tables: %w", err)
	}
	if err := s.TokenBlacklist().initTables(); err != nil {
		return fmt.Errorf("failed to initialize token blacklist tables: %w", err)
	}

	// Apply versioned migrations (records schema_migrations baseline).
	if err := RunMigrations(s.db); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}

// initDefaultData initializes default data
func (s *Store) initDefaultData() error {
	if err := s.AIModel().initDefaultData(); err != nil {
		return err
	}
	if err := s.Exchange().initDefaultData(); err != nil {
		return err
	}
	if err := s.Strategy().initDefaultData(); err != nil {
		return err
	}
	// Migrate old decision_account_snapshots data to new trader_equity_snapshots table
	if migrated, err := s.Equity().MigrateFromDecision(); err != nil {
		logger.Warnf("failed to migrate equity data: %v", err)
	} else if migrated > 0 {
		logger.Infof("✅ Migrated %d equity records to new table", migrated)
	}
	return nil
}

// User gets user storage
func (s *Store) User() *UserStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.user == nil {
		s.user = &UserStore{db: s.db}
	}
	return s.user
}

// AIModel gets AI model storage
func (s *Store) AIModel() *AIModelStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.aiModel == nil {
		s.aiModel = &AIModelStore{
			db:          s.db,
			encryptFunc: s.encryptFunc,
			decryptFunc: s.decryptFunc,
		}
	}
	return s.aiModel
}

// Exchange gets exchange storage
func (s *Store) Exchange() *ExchangeStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.exchange == nil {
		s.exchange = &ExchangeStore{
			db:          s.db,
			encryptFunc: s.encryptFunc,
			decryptFunc: s.decryptFunc,
		}
	}
	return s.exchange
}

// Trader gets trader storage
func (s *Store) Trader() *TraderStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.trader == nil {
		s.trader = &TraderStore{
			db:          s.db,
			decryptFunc: s.decryptFunc,
		}
	}
	return s.trader
}

// Decision gets decision log storage
func (s *Store) Decision() *DecisionStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.decision == nil {
		s.decision = &DecisionStore{db: s.db}
	}
	return s.decision
}

// Backtest gets backtest data storage
func (s *Store) Backtest() *BacktestStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.backtest == nil {
		s.backtest = &BacktestStore{db: s.db}
	}
	return s.backtest
}

// Position gets position storage
func (s *Store) Position() *PositionStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.position == nil {
		s.position = NewPositionStore(s.db)
	}
	return s.position
}

// Execution gets order, fill, and protection storage.
func (s *Store) Execution() *ExecutionStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.execution == nil {
		s.execution = NewExecutionStore(s.db)
	}
	return s.execution
}

// Strategy gets strategy storage
func (s *Store) Strategy() *StrategyStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.strategy == nil {
		s.strategy = &StrategyStore{db: s.db}
	}
	return s.strategy
}

// Paper gets paper-trading (simulated account) storage
func (s *Store) Paper() *PaperStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.paper == nil {
		s.paper = &PaperStore{db: s.db}
	}
	return s.paper
}

// Equity gets equity storage
func (s *Store) Equity() *EquityStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.equity == nil {
		s.equity = &EquityStore{db: s.db}
	}
	return s.equity
}

// Audit gets audit log storage
func (s *Store) Audit() *AuditStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.audit == nil {
		s.audit = &AuditStore{db: s.db}
	}
	return s.audit
}

// Notification gets notification storage
func (s *Store) Notification() *NotificationStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.notification == nil {
		s.notification = &NotificationStore{db: s.db, encryptFunc: s.encryptFunc, decryptFunc: s.decryptFunc}
	}
	return s.notification
}

// TokenBlacklist gets persistent token blacklist storage
func (s *Store) TokenBlacklist() *TokenBlacklistStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tokenBlacklist == nil {
		s.tokenBlacklist = &TokenBlacklistStore{db: s.db}
	}
	return s.tokenBlacklist
}

// Close closes database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// DB gets underlying database connection (for legacy code compatibility, gradually deprecated)
// Deprecated: use Store methods instead
func (s *Store) DB() *sql.DB {
	return s.db
}

// Transaction executes transaction
func (s *Store) Transaction(fn func(tx *sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}
