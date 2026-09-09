package decision

import (
	"strings"
	"testing"
	"time"

	"nofx/store"
)

// TestBuildUserPromptIncludesExecutionFailures verifies that recent failed
// instructions are rendered into the user prompt with a corrective hint, so
// the AI knows its previous orders were NOT executed.
func TestBuildUserPromptIncludesExecutionFailures(t *testing.T) {
	engine := NewStrategyEngine(&store.StrategyConfig{
		CoinSource: store.CoinSourceConfig{SourceType: "static", StaticCoins: []string{"BTC"}},
	})

	ctx := &Context{
		CurrentTime: "2025-01-01 00:00:00 UTC",
		CallCount:   2,
		Account: AccountInfo{
			TotalEquity:      1000,
			AvailableBalance: 1000,
		},
		RecentFailures: []RecentExecutionFailure{
			{Symbol: "BTCUSDT", Action: "open_long", Error: "insufficient available balance", Timestamp: time.Now().Add(-1 * time.Minute)},
			{Symbol: "ETHUSDT", Action: "close_long", Error: "no long position found", Timestamp: time.Now().Add(-30 * time.Second)},
		},
	}

	prompt := engine.BuildUserPrompt(ctx)

	if !strings.Contains(prompt, "Recent Execution Failures") {
		t.Fatal("prompt should contain the execution-failures section header")
	}
	if !strings.Contains(prompt, "BTCUSDT open_long failed: insufficient available balance") {
		t.Fatal("prompt should list the BTCUSDT open_long failure verbatim")
	}
	if !strings.Contains(prompt, "ETHUSDT close_long failed: no long position found") {
		t.Fatal("prompt should list the ETHUSDT close_long failure verbatim")
	}
	if !strings.Contains(prompt, "NOT executed") {
		t.Fatal("prompt should emphasize the instructions were not executed")
	}
}

// TestBuildUserPromptOmitsFailureSectionWhenEmpty verifies no empty section
// is added when there are no failures (keeps the prompt clean).
func TestBuildUserPromptOmitsFailureSectionWhenEmpty(t *testing.T) {
	engine := NewStrategyEngine(&store.StrategyConfig{
		CoinSource: store.CoinSourceConfig{SourceType: "static", StaticCoins: []string{"BTC"}},
	})

	ctx := &Context{
		CurrentTime: "2025-01-01 00:00:00 UTC",
		CallCount:   1,
		Account:     AccountInfo{TotalEquity: 1000, AvailableBalance: 1000},
	}

	prompt := engine.BuildUserPrompt(ctx)

	if strings.Contains(prompt, "Recent Execution Failures") {
		t.Fatal("failure section must be omitted when there are no failures")
	}
}
