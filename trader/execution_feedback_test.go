package trader

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"nofx/decision"
	"nofx/store"
)

func TestRememberExecutionFailureTruncatesAndCollapsesNewlines(t *testing.T) {
	at := &AutoTrader{}

	multiLine := "line1\nline2\n\nline3\n"
	at.rememberExecutionFailure("BTCUSDT", "open_long", errors.New(multiLine))

	got := at.recentExecutionFailuresForPrompt()
	if len(got) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(got))
	}
	if strings.Contains(got[0].Error, "\n") {
		t.Fatalf("error message should have newlines collapsed: %q", got[0].Error)
	}
	if got[0].Error != "line1 line2 line3" {
		t.Fatalf("unexpected collapsed message: %q", got[0].Error)
	}

	at = &AutoTrader{}
	long := strings.Repeat("x", maxExecutionFailureMessageLen+50)
	at.rememberExecutionFailure("BTCUSDT", "open_long", errors.New(long))
	got = at.recentExecutionFailuresForPrompt()
	if len(got[0].Error) > maxExecutionFailureMessageLen+3 {
		t.Fatalf("error message should be truncated, got length %d", len(got[0].Error))
	}
}

func TestRememberExecutionFailureIgnoresNilError(t *testing.T) {
	at := &AutoTrader{}
	at.rememberExecutionFailure("BTCUSDT", "open_long", nil)
	if failures := at.recentExecutionFailuresForPrompt(); len(failures) != 0 {
		t.Fatalf("nil error must not be recorded, got %d", len(failures))
	}
}

func TestRecentExecutionFailuresForPromptWindow(t *testing.T) {
	at := &AutoTrader{config: AutoTraderConfig{ScanInterval: 5 * time.Minute}}

	at.feedbackMutex.Lock()
	at.recentExecutionFailures = []decision.RecentExecutionFailure{
		{Symbol: "OLDUSDT", Action: "open_long", Error: "ancient failure", Timestamp: time.Now().Add(-3 * time.Hour)},
		{Symbol: "NEWUSDT", Action: "open_short", Error: "recent failure", Timestamp: time.Now().Add(-1 * time.Minute)},
	}
	at.feedbackMutex.Unlock()

	failures := at.recentExecutionFailuresForPrompt()
	if len(failures) != 1 {
		t.Fatalf("expected only the recent failure, got %d", len(failures))
	}
	if failures[0].Symbol != "NEWUSDT" {
		t.Fatalf("expected NEWUSDT, got %s", failures[0].Symbol)
	}
}

func TestRecentExecutionFailuresForPromptDedupesAndCaps(t *testing.T) {
	at := &AutoTrader{config: AutoTraderConfig{ScanInterval: time.Minute}}

	at.feedbackMutex.Lock()
	var buffered []decision.RecentExecutionFailure
	// Same symbol+action+error repeated across three cycles -> deduped to 1.
	for i := 0; i < 3; i++ {
		buffered = append(buffered, decision.RecentExecutionFailure{
			Symbol: "BTCUSDT", Action: "open_long", Error: "insufficient balance",
			Timestamp: time.Now().Add(-time.Duration(i) * time.Minute),
		})
	}
	// Nine distinct failures -> capped at 8 total.
	for i := 0; i < 9; i++ {
		buffered = append(buffered, decision.RecentExecutionFailure{
			Symbol: fmt.Sprintf("SYM%dUSDT", i), Action: "close_long", Error: fmt.Sprintf("err %d", i),
			Timestamp: time.Now(),
		})
	}
	at.recentExecutionFailures = buffered
	at.feedbackMutex.Unlock()

	failures := at.recentExecutionFailuresForPrompt()
	if len(failures) != 8 {
		t.Fatalf("expected cap of 8 failures, got %d", len(failures))
	}
	seen := map[string]int{}
	for _, f := range failures {
		seen[f.Symbol+"|"+f.Action+"|"+f.Error]++
	}
	for key, count := range seen {
		if count > 1 {
			t.Fatalf("duplicate failure leaked into prompt feedback: %s x%d", key, count)
		}
	}
}

func TestForgetExecutionFailureOnSuccess(t *testing.T) {
	at := &AutoTrader{}

	at.rememberExecutionFailure("BTCUSDT", "open_long", errors.New("insufficient balance"))
	at.rememberExecutionFailure("ETHUSDT", "open_short", errors.New("max positions"))

	at.forgetExecutionFailureOnSuccess("BTCUSDT", "open_long")

	failures := at.recentExecutionFailuresForPrompt()
	if len(failures) != 1 {
		t.Fatalf("expected the resolved BTCUSDT failure to be dropped, got %d", len(failures))
	}
	if failures[0].Symbol != "ETHUSDT" {
		t.Fatalf("expected ETHUSDT to remain, got %s", failures[0].Symbol)
	}
}

// TestExecuteDecisionWithRecordRecordsFailure verifies the dispatch funnel:
// a failing instruction is captured for the next AI cycle.
func TestExecuteDecisionWithRecordRecordsFailure(t *testing.T) {
	at := &AutoTrader{}

	d := &decision.Decision{Symbol: "BTCUSDT", Action: "bogus_action"}
	err := at.executeDecisionWithRecord(d, &store.DecisionAction{})
	if err == nil {
		t.Fatal("unknown action must return an error")
	}

	failures := at.recentExecutionFailuresForPrompt()
	if len(failures) != 1 {
		t.Fatalf("expected the failure to be recorded, got %d", len(failures))
	}
	if failures[0].Symbol != "BTCUSDT" || failures[0].Action != "bogus_action" {
		t.Fatalf("unexpected recorded failure: %+v", failures[0])
	}
	if failures[0].Error == "" {
		t.Fatal("recorded failure must carry the error message")
	}
}

// TestExecuteDecisionWithRecordHoldNotRecorded verifies hold/wait decisions
// never pollute the feedback buffer.
func TestExecuteDecisionWithRecordHoldNotRecorded(t *testing.T) {
	at := &AutoTrader{}

	d := &decision.Decision{Symbol: "BTCUSDT", Action: "hold"}
	if err := at.executeDecisionWithRecord(d, &store.DecisionAction{}); err != nil {
		t.Fatalf("hold must not fail: %v", err)
	}
	if failures := at.recentExecutionFailuresForPrompt(); len(failures) != 0 {
		t.Fatalf("hold must not be recorded as failure, got %d", len(failures))
	}
}
