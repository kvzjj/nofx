package decision

import (
	"strings"
	"testing"
)

func TestValidateEntryRisk(t *testing.T) {
	tests := []struct {
		name       string
		decision   Decision
		entryPrice float64
		quantity   float64
		wantError  string
	}{
		{
			name: "long accepts risk at allowed limit",
			decision: Decision{
				Action: "open_long", StopLoss: 95, TakeProfit: 115, RiskUSD: 50,
			},
			entryPrice: 100,
			quantity:   10,
		},
		{
			name: "long rejects actual dollar risk above allowance",
			decision: Decision{
				Action: "open_long", StopLoss: 95, TakeProfit: 115, RiskUSD: 49.99,
			},
			entryPrice: 100,
			quantity:   10,
			wantError:  "position risk 50.00 USD exceeds allowed risk 49.99 USD",
		},
		{
			name: "short accepts sub-cent risk excess that rounds to allowance",
			decision: Decision{
				Action: "open_short", StopLoss: 4045.86, TakeProfit: 3985.86, RiskUSD: 3.72,
			},
			entryPrice: 4030.86,
			quantity:   0.24808602,
		},
		{
			name: "short rejects risk that rounds above allowance",
			decision: Decision{
				Action: "open_short", StopLoss: 4045.88, TakeProfit: 3985.80, RiskUSD: 3.72,
			},
			entryPrice: 4030.86,
			quantity:   0.24808602,
			wantError:  "position risk 3.73 USD exceeds allowed risk 3.72 USD",
		},
		{
			name: "short uses real entry for reward ratio",
			decision: Decision{
				Action: "open_short", StopLoss: 105, TakeProfit: 90, RiskUSD: 50,
			},
			entryPrice: 100,
			quantity:   10,
			wantError:  "risk/reward ratio too low (2.00:1)",
		},
		{
			name: "accepts decimal reward ratio exactly on boundary",
			decision: Decision{
				Action: "open_short", StopLoss: 204.65, TakeProfit: 199.21, RiskUSD: 100,
			},
			entryPrice: 203.29,
			quantity:   1,
		},
		{
			name: "rejects stop on wrong side of real entry",
			decision: Decision{
				Action: "open_long", StopLoss: 101, TakeProfit: 115, RiskUSD: 50,
			},
			entryPrice: 100,
			quantity:   10,
			wantError:  "stop loss < entry < take profit",
		},
		{
			name: "risk allowance is required",
			decision: Decision{
				Action: "open_short", StopLoss: 105, TakeProfit: 85,
			},
			entryPrice: 100,
			quantity:   10,
			wantError:  "risk_usd must be greater than 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEntryRisk(&tt.decision, tt.entryPrice, tt.quantity, 3)
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("ValidateEntryRisk() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("ValidateEntryRisk() error = %v, want substring %q", err, tt.wantError)
			}
		})
	}
}
