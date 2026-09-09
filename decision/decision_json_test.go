package decision

import (
	"strings"
	"testing"
)

func TestExtractDecisionsAcceptsCommonModelFormattingDrift(t *testing.T) {
	response := `<reasoning>valid setup</reasoning><decision>{"decisions":[{
		"symbol":" eth/usdt ", "action":"OPEN-SHORT", "leverage":"5x",
		"position_size_usd":"$1,000 USDT", "stop_loss":"4,100",
		"take_profit":"3,800", "confidence":"85%", "risk_usd":"25 usd",
		"reasoning":"breakdown with {nested} note"
	}]}</decision>`

	decisions, err := extractDecisions(response)
	if err != nil {
		t.Fatalf("extractDecisions() error = %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("got %d decisions, want 1", len(decisions))
	}
	d := decisions[0]
	if d.Symbol != "ETHUSDT" || d.Action != "open_short" || d.StopLoss != 4100 || d.TakeProfit != 3800 {
		t.Fatalf("unexpected normalized decision: %#v", d)
	}
	if d.Leverage != 0 || d.PositionSizeUSD != 0 || d.Confidence != 0 || d.RiskUSD != 0 {
		t.Fatalf("AI-owned sizing fields were not ignored: %#v", d)
	}
}

func TestExtractDecisionsAcceptsSingleObjectAndTextBeforeJSON(t *testing.T) {
	response := `Analysis mentions [RSI] first. Final: {"symbol":"BTC-USDT","action":"close long","reasoning":"protect profit"}`
	decisions, err := extractDecisions(response)
	if err != nil {
		t.Fatalf("extractDecisions() error = %v", err)
	}
	if len(decisions) != 1 || decisions[0].Symbol != "BTCUSDT" || decisions[0].Action != "close_long" {
		t.Fatalf("unexpected decisions: %#v", decisions)
	}
}

func TestExtractDecisionsIgnoresLegacySizingExpression(t *testing.T) {
	response := `<decision>[{"symbol":"BTCUSDT","action":"open_long","position_size_usd":"100 * 2","stop_loss":90,"take_profit":130}]</decision>`
	decisions, err := extractDecisions(response)
	if err != nil || len(decisions) != 1 || decisions[0].PositionSizeUSD != 0 {
		t.Fatalf("legacy sizing field should be ignored, decisions=%#v err=%v", decisions, err)
	}
}

func TestExtractDecisionsSalvagesValidCloseFromMalformedBatch(t *testing.T) {
	response := `<decision>[
		{"symbol":"ETHUSDT","action":"open_long","position_size_usd":"100 * 2"},
		{"symbol":"BTCUSDT","action":"close_long"},
	]</decision>`
	decisions, err := extractDecisions(response)
	if err != nil {
		t.Fatalf("extractDecisions() error = %v", err)
	}
	if len(decisions) != 2 || decisions[0].PositionSizeUSD != 0 || decisions[1].Symbol != "BTCUSDT" || decisions[1].Action != "close_long" {
		t.Fatalf("unexpected recovered decisions: %#v", decisions)
	}
}

func TestExtractDecisionsTreatsEmptyArrayAsWait(t *testing.T) {
	decisions, err := extractDecisions(`<decision>[]</decision>`)
	if err != nil {
		t.Fatalf("extractDecisions() error = %v", err)
	}
	if len(decisions) != 1 || decisions[0].Action != "wait" {
		t.Fatalf("unexpected decisions: %#v", decisions)
	}
}

func TestExtractDecisionsIgnoresBracketedProse(t *testing.T) {
	decisions, err := extractDecisions(`RSI is [high], so no structured output is available.`)
	if err != nil {
		t.Fatalf("extractDecisions() error = %v", err)
	}
	if len(decisions) != 1 || decisions[0].Action != "wait" {
		t.Fatalf("unexpected decisions: %#v", decisions)
	}
}

func TestExtractCoTTraceKeepsBracketedProse(t *testing.T) {
	response := "RSI [14] is high and momentum [1h] faded.\n[{\"symbol\":\"BTCUSDT\",\"action\":\"wait\"}]"
	trace := extractCoTTrace(response)
	if !strings.Contains(trace, "RSI [14]") || !strings.Contains(trace, "momentum [1h]") {
		t.Fatalf("reasoning trace was truncated at bracketed prose: %q", trace)
	}
	if strings.Contains(trace, "BTCUSDT") {
		t.Fatalf("reasoning trace must not include the decision JSON: %q", trace)
	}
}
