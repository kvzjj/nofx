package decision

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// UnmarshalJSON tolerates harmless formatting drift commonly produced by LLMs,
// such as "5x", "85%", "$100", or numeric values encoded as JSON strings.
// Expressions are intentionally rejected rather than evaluated.
func (d *Decision) UnmarshalJSON(data []byte) error {
	type rawDecision struct {
		Symbol          string          `json:"symbol"`
		Action          string          `json:"action"`
		Leverage        json.RawMessage `json:"leverage"`
		PositionSizeUSD json.RawMessage `json:"position_size_usd"`
		StopLoss        json.RawMessage `json:"stop_loss"`
		TakeProfit      json.RawMessage `json:"take_profit"`
		Confidence      json.RawMessage `json:"confidence"`
		RiskUSD         json.RawMessage `json:"risk_usd"`
		Reasoning       string          `json:"reasoning"`
	}

	var raw rawDecision
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	var err error
	d.Symbol, d.Action, d.Reasoning = raw.Symbol, raw.Action, raw.Reasoning
	if d.Leverage, err = parseFlexibleInt(raw.Leverage, "leverage"); err != nil {
		return err
	}
	if d.PositionSizeUSD, err = parseFlexibleFloat(raw.PositionSizeUSD, "position_size_usd"); err != nil {
		return err
	}
	if d.StopLoss, err = parseFlexibleFloat(raw.StopLoss, "stop_loss"); err != nil {
		return err
	}
	if d.TakeProfit, err = parseFlexibleFloat(raw.TakeProfit, "take_profit"); err != nil {
		return err
	}
	if d.Confidence, err = parseFlexibleInt(raw.Confidence, "confidence"); err != nil {
		return err
	}
	if d.RiskUSD, err = parseFlexibleFloat(raw.RiskUSD, "risk_usd"); err != nil {
		return err
	}
	return nil
}

func parseFlexibleInt(raw json.RawMessage, field string) (int, error) {
	v, err := parseFlexibleFloat(raw, field)
	if err != nil {
		return 0, err
	}
	if math.Trunc(v) != v || v > math.MaxInt || v < math.MinInt {
		return 0, fmt.Errorf("%s must be an integer, got %v", field, v)
	}
	return int(v), nil
}

func parseFlexibleFloat(raw json.RawMessage, field string) (float64, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return 0, nil
	}

	value := strings.TrimSpace(string(raw))
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		if err := json.Unmarshal(raw, &value); err != nil {
			return 0, fmt.Errorf("%s has invalid numeric string: %w", field, err)
		}
	}
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	lower = strings.TrimSpace(strings.TrimSuffix(lower, "usdt"))
	lower = strings.TrimSpace(strings.TrimSuffix(lower, "usd"))
	lower = strings.TrimSuffix(lower, "%")
	lower = strings.TrimSuffix(lower, "x")
	lower = strings.TrimPrefix(lower, "$")
	lower = strings.ReplaceAll(lower, ",", "")

	parsed, err := strconv.ParseFloat(strings.TrimSpace(lower), 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, fmt.Errorf("%s must be a finite number, got %q", field, value)
	}
	return parsed, nil
}

// decodeDecisionsFromText scans balanced JSON values instead of using a
// regular expression, so nested objects and braces inside strings are safe.
// It accepts the documented array plus two frequent model variants:
// {"decisions":[...]} and a single decision object.
func decodeDecisionsFromText(text string) ([]Decision, string, error) {
	candidates := balancedJSONValues(text)
	var lastCandidate string
	var lastErr error
	var recovered []Decision
	for _, candidate := range candidates {
		candidate = fixMissingQuotes(strings.TrimSpace(candidate))
		candidateLooksRelevant := strings.Contains(candidate, `"action"`) ||
			strings.Contains(candidate, `"symbol"`) || strings.Contains(candidate, `"decisions"`)
		if !json.Valid([]byte(candidate)) {
			if candidateLooksRelevant {
				lastCandidate = candidate
				lastErr = fmt.Errorf("invalid decision JSON")
			}
			continue
		}
		if !looksLikeDecisionJSON(candidate) {
			continue
		}
		lastCandidate = candidate
		decisions, err := decodeDecisionValue(candidate)
		if err == nil {
			if strings.HasPrefix(candidate, "[") || strings.Contains(candidate, `"decisions"`) {
				return decisions, candidate, nil
			}
			recovered = append(recovered, decisions...)
			continue
		}
		lastErr = err
	}
	if len(recovered) > 0 {
		return recovered, lastCandidate, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no JSON array or object found")
	}
	return nil, lastCandidate, lastErr
}

func looksLikeDecisionJSON(candidate string) bool {
	var value any
	if err := json.Unmarshal([]byte(candidate), &value); err != nil {
		return false
	}
	switch typed := value.(type) {
	case []any:
		if len(typed) == 0 {
			return true
		}
		first, ok := typed[0].(map[string]any)
		return ok && (first["action"] != nil || first["symbol"] != nil)
	case map[string]any:
		return typed["decisions"] != nil || typed["action"] != nil || typed["symbol"] != nil
	default:
		return false
	}
}

func decodeDecisionValue(candidate string) ([]Decision, error) {
	candidate = fixMissingQuotes(strings.TrimSpace(candidate))
	if strings.HasPrefix(candidate, "[") {
		var decisions []Decision
		if err := json.Unmarshal([]byte(candidate), &decisions); err != nil {
			return nil, err
		}
		return decisions, nil
	}

	var wrapper struct {
		Decisions json.RawMessage `json:"decisions"`
	}
	if err := json.Unmarshal([]byte(candidate), &wrapper); err != nil {
		return nil, err
	}
	if len(wrapper.Decisions) > 0 {
		var decisions []Decision
		if err := json.Unmarshal(wrapper.Decisions, &decisions); err != nil {
			return nil, err
		}
		return decisions, nil
	}

	var single Decision
	if err := json.Unmarshal([]byte(candidate), &single); err != nil {
		return nil, err
	}
	if single.Symbol == "" && single.Action == "" {
		return nil, fmt.Errorf("object is not a decision")
	}
	return []Decision{single}, nil
}

func balancedJSONValues(text string) []string {
	values := make([]string, 0, 2)
	for start := 0; start < len(text); start++ {
		if text[start] != '[' && text[start] != '{' {
			continue
		}
		stack := make([]byte, 0, 4)
		inString, escaped := false, false
		for i := start; i < len(text); i++ {
			c := text[i]
			if inString {
				if escaped {
					escaped = false
				} else if c == '\\' {
					escaped = true
				} else if c == '"' {
					inString = false
				}
				continue
			}
			if c == '"' {
				inString = true
				continue
			}
			switch c {
			case '[', '{':
				stack = append(stack, c)
			case ']', '}':
				if len(stack) == 0 || (c == ']' && stack[len(stack)-1] != '[') || (c == '}' && stack[len(stack)-1] != '{') {
					i = len(text)
					continue
				}
				stack = stack[:len(stack)-1]
				if len(stack) == 0 {
					values = append(values, text[start:i+1])
					i = len(text)
				}
			}
		}
	}
	return values
}
