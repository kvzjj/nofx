package store

import (
	"database/sql"
	"testing"
)

func TestTraderStoreUpdatePersistsFullConfiguration(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	store := NewFromDB(db)
	traderStore := store.Trader()
	if err := traderStore.initTables(); err != nil {
		t.Fatalf("init trader tables: %v", err)
	}

	original := &Trader{
		ID:                   "trader-1",
		UserID:               "user-1",
		Name:                 "original",
		AIModelID:            "model-old",
		ExchangeID:           "exchange-old",
		StrategyID:           "strategy-old",
		InitialBalance:       1000,
		ScanIntervalMinutes:  3,
		IsRunning:            true,
		IsCrossMargin:        true,
		ShowInCompetition:    true,
		BTCETHLeverage:       5,
		AltcoinLeverage:      3,
		TradingSymbols:       "BTCUSDT,ETHUSDT",
		UseCoinPool:          false,
		UseOITop:             false,
		CustomPrompt:         "old prompt",
		OverrideBasePrompt:   false,
		SystemPromptTemplate: "default",
	}
	if err := traderStore.Create(original); err != nil {
		t.Fatalf("create trader: %v", err)
	}

	updated := &Trader{
		ID:                   original.ID,
		UserID:               original.UserID,
		Name:                 "updated",
		AIModelID:            "model-new",
		ExchangeID:           "exchange-new",
		StrategyID:           "strategy-new",
		InitialBalance:       2000,
		ScanIntervalMinutes:  15,
		IsCrossMargin:        false,
		ShowInCompetition:    false,
		BTCETHLeverage:       12,
		AltcoinLeverage:      8,
		TradingSymbols:       "SOLUSDT,DOGEUSDT",
		UseCoinPool:          true,
		UseOITop:             true,
		CustomPrompt:         "new prompt",
		OverrideBasePrompt:   true,
		SystemPromptTemplate: "nof1",
	}
	if err := traderStore.Update(updated); err != nil {
		t.Fatalf("update trader: %v", err)
	}

	traders, err := traderStore.List(original.UserID)
	if err != nil {
		t.Fatalf("list traders: %v", err)
	}
	if len(traders) != 1 {
		t.Fatalf("expected 1 trader, got %d", len(traders))
	}

	got := traders[0]
	if got.Name != updated.Name ||
		got.AIModelID != updated.AIModelID ||
		got.ExchangeID != updated.ExchangeID ||
		got.StrategyID != updated.StrategyID ||
		got.InitialBalance != updated.InitialBalance ||
		got.ScanIntervalMinutes != updated.ScanIntervalMinutes ||
		got.IsCrossMargin != updated.IsCrossMargin ||
		got.ShowInCompetition != updated.ShowInCompetition ||
		got.BTCETHLeverage != updated.BTCETHLeverage ||
		got.AltcoinLeverage != updated.AltcoinLeverage ||
		got.TradingSymbols != updated.TradingSymbols ||
		got.UseCoinPool != updated.UseCoinPool ||
		got.UseOITop != updated.UseOITop ||
		got.CustomPrompt != updated.CustomPrompt ||
		got.OverrideBasePrompt != updated.OverrideBasePrompt ||
		got.SystemPromptTemplate != updated.SystemPromptTemplate {
		t.Fatalf("updated trader was not fully persisted: got %+v, want %+v", got, updated)
	}
}
