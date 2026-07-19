package decision

import "testing"

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
	if d.Symbol != "ETHUSDT" || d.Action != "open_short" || d.Leverage != 5 || d.PositionSizeUSD != 1000 || d.Confidence != 85 {
		t.Fatalf("unexpected normalized decision: %#v", d)
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

func TestExtractDecisionsRejectsNumericExpression(t *testing.T) {
	response := `<decision>[{"symbol":"BTCUSDT","action":"open_long","leverage":5,"position_size_usd":"100 * 2"}]</decision>`
	if _, err := extractDecisions(response); err == nil {
		t.Fatal("extractDecisions() error = nil, want malformed numeric expression error")
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
	if len(decisions) != 1 || decisions[0].Symbol != "BTCUSDT" || decisions[0].Action != "close_long" {
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
