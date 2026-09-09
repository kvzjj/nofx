package store

import (
	"database/sql"
	"testing"
)

func newStrategyTestStore(t *testing.T) *StrategyStore {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := NewFromDB(db)
	if err := st.Strategy().initTables(); err != nil {
		t.Fatalf("init strategy tables: %v", err)
	}
	return st.Strategy()
}

func TestInitDefaultDataSeedsDefaultStrategyIdempotently(t *testing.T) {
	s := newStrategyTestStore(t)
	if err := s.initDefaultData(); err != nil {
		t.Fatalf("initDefaultData: %v", err)
	}
	def, err := s.GetDefault()
	if err != nil {
		t.Fatalf("GetDefault after seed: %v", err)
	}
	if def.ID != "default" || !def.IsDefault {
		t.Fatalf("unexpected default strategy: %+v", def)
	}
	if def.IsActive {
		t.Fatalf("default strategy must not carry an activation flag")
	}

	// Second run must not duplicate or clobber the row.
	if err := s.initDefaultData(); err != nil {
		t.Fatalf("second initDefaultData: %v", err)
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM strategies WHERE is_default = 1`).Scan(&count); err != nil {
		t.Fatalf("count defaults: %v", err)
	}
	if count != 1 {
		t.Fatalf("default strategy count = %d, want 1", count)
	}
}

func TestSetActiveOnlyActivatesUserOwnedStrategies(t *testing.T) {
	s := newStrategyTestStore(t)
	if err := s.initDefaultData(); err != nil {
		t.Fatalf("initDefaultData: %v", err)
	}

	userStrategy := &Strategy{ID: "s1", UserID: "user-1", Name: "mine", Config: "{}"}
	if err := s.Create(userStrategy); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Activating the shared default row must fail instead of writing
	// per-user state onto a global row.
	def, err := s.GetDefault()
	if err != nil {
		t.Fatalf("GetDefault: %v", err)
	}
	if err := s.SetActive("user-1", def.ID); err == nil {
		t.Fatalf("activating the shared default strategy should fail")
	}
	var defaultActive bool
	if err := s.db.QueryRow(`SELECT is_active FROM strategies WHERE id = ?`, def.ID).Scan(&defaultActive); err != nil {
		t.Fatalf("read default is_active: %v", err)
	}
	if defaultActive {
		t.Fatalf("default row must remain is_active=0")
	}

	// Nonexistent strategy must fail loudly instead of silently succeeding.
	if err := s.SetActive("user-1", "does-not-exist"); err == nil {
		t.Fatalf("activating a nonexistent strategy should fail")
	}

	// A user-owned strategy activates and deactivates prior ones.
	if err := s.SetActive("user-1", "s1"); err != nil {
		t.Fatalf("SetActive own strategy: %v", err)
	}
	active, err := s.GetActive("user-1")
	if err != nil {
		t.Fatalf("GetActive: %v", err)
	}
	if active.ID != "s1" {
		t.Fatalf("active strategy = %s, want s1", active.ID)
	}

	// Another user's strategy cannot be activated.
	if err := s.Create(&Strategy{ID: "s2", UserID: "user-2", Name: "theirs", Config: "{}"}); err != nil {
		t.Fatalf("create s2: %v", err)
	}
	if err := s.SetActive("user-1", "s2"); err == nil {
		t.Fatalf("activating another user's strategy should fail")
	}
}

func TestGetActiveFallsBackToSeededDefault(t *testing.T) {
	s := newStrategyTestStore(t)
	if err := s.initDefaultData(); err != nil {
		t.Fatalf("initDefaultData: %v", err)
	}
	active, err := s.GetActive("user-without-strategies")
	if err != nil {
		t.Fatalf("GetActive fallback failed: %v", err)
	}
	if !active.IsDefault {
		t.Fatalf("expected seeded default, got %+v", active)
	}
}

func TestSetActiveClearsStaleDefaultActivationFlag(t *testing.T) {
	s := newStrategyTestStore(t)
	if err := s.initDefaultData(); err != nil {
		t.Fatalf("initDefaultData: %v", err)
	}
	// Simulate the dirty state written by older versions.
	if _, err := s.db.Exec(`UPDATE strategies SET is_active = 1 WHERE is_default = 1`); err != nil {
		t.Fatalf("seed dirty flag: %v", err)
	}
	if err := s.Create(&Strategy{ID: "s1", UserID: "user-1", Name: "mine", Config: "{}"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.SetActive("user-1", "s1"); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	var defaultActive bool
	if err := s.db.QueryRow(`SELECT is_active FROM strategies WHERE is_default = 1`).Scan(&defaultActive); err != nil {
		t.Fatalf("read default is_active: %v", err)
	}
	if defaultActive {
		t.Fatalf("stale is_active=1 on shared default row was not cleared")
	}
}
