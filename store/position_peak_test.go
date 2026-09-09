package store

import (
	"database/sql"
	"testing"
)

func newPeakPnLTestStore(t *testing.T) *PositionStore {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := NewFromDB(db)
	if err := st.Position().InitTables(); err != nil {
		t.Fatalf("init position tables: %v", err)
	}
	return st.Position()
}

func TestPeakPnLRoundtrip(t *testing.T) {
	s := newPeakPnLTestStore(t)

	if err := s.UpsertPeakPnL("trader-1", "BTCUSDT", "long", 12.5); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// A lower peak must not overwrite the recorded maximum via upsert callers;
	// the store itself stores whatever it is given, so simulate an increase.
	if err := s.UpsertPeakPnL("trader-1", "BTCUSDT", "long", 18.0); err != nil {
		t.Fatalf("upsert higher: %v", err)
	}
	if err := s.UpsertPeakPnL("trader-1", "ETHUSDT", "short", 7.25); err != nil {
		t.Fatalf("upsert second position: %v", err)
	}

	peaks, err := s.GetPeakPnLs("trader-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(peaks) != 2 {
		t.Fatalf("peaks = %v, want 2 entries", peaks)
	}
	if peaks["BTCUSDT_long"] != 18.0 {
		t.Fatalf("BTCUSDT_long = %v, want 18.0", peaks["BTCUSDT_long"])
	}
	if peaks["ETHUSDT_short"] != 7.25 {
		t.Fatalf("ETHUSDT_short = %v, want 7.25", peaks["ETHUSDT_short"])
	}

	if err := s.DeletePeakPnL("trader-1", "BTCUSDT", "long"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	peaks, err = s.GetPeakPnLs("trader-1")
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if len(peaks) != 1 {
		t.Fatalf("peaks after delete = %v, want 1 entry", peaks)
	}

	// Other traders must be isolated.
	other, err := s.GetPeakPnLs("trader-2")
	if err != nil {
		t.Fatalf("get other trader: %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("trader-2 peaks = %v, want empty", other)
	}
}
