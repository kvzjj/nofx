package trader

import (
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"nofx/decision"
	"nofx/market"
	"nofx/store"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/suite"
)

// ============================================================
// AutoTraderTestSuite - Structured testing using testify/suite
// ============================================================

// AutoTraderTestSuite tests the AutoTrader execution pipeline against a mock
// exchange. Opening tests exercise the backend sizing path, so the mocked
// market data must provide ATR and volume.
type AutoTraderTestSuite struct {
	suite.Suite

	autoTrader  *AutoTrader
	mockTrader  *MockTrader
	patches     *gomonkey.Patches
	strategyCfg *store.StrategyConfig
}

func (s *AutoTraderTestSuite) SetupTest() {
	s.patches = gomonkey.NewPatches()

	s.mockTrader = &MockTrader{
		balance: map[string]interface{}{
			"totalWalletBalance":    10000.0,
			"availableBalance":      8000.0,
			"totalUnrealizedProfit": 100.0,
		},
		positions: []map[string]interface{}{},
		orderStatus: map[string]interface{}{
			"status":      "FILLED",
			"avgPrice":    50000.0,
			"executedQty": 0.05,
			"commission":  0.5,
		},
	}

	s.strategyCfg = &store.StrategyConfig{
		CoinSource: store.CoinSourceConfig{
			SourceType:  "static",
			StaticCoins: []string{"BTC", "ETH"},
		},
		RiskControl: store.RiskControlConfig{
			MaxPositions:         3,
			BTCETHMaxLeverage:    10,
			AltcoinMaxLeverage:   5,
			MinRiskRewardRatio:   0,
			MinPositionSize:      0,
			MaxPositionSize:      0,
			MaxTotalPositionSize: 0,
		},
	}

	s.autoTrader = &AutoTrader{
		id:                    "test_trader",
		name:                  "Test Trader",
		aiModel:               "deepseek",
		exchange:              "binance",
		config:                AutoTraderConfig{ID: "test_trader", Name: "Test Trader", StrategyConfig: s.strategyCfg},
		trader:                s.mockTrader,
		store:                 nil,
		initialBalance:        10000.0,
		strategyEngine:        decision.NewStrategyEngine(s.strategyCfg),
		lastResetTime:         time.Now(),
		startTime:             time.Now(),
		positionFirstSeenTime: make(map[string]int64),
		stopMonitorCh:         make(chan struct{}),
		peakPnLCache:          make(map[string]peakPnLEntry),
		userID:                "test_user",
	}
}

func (s *AutoTraderTestSuite) TearDownTest() {
	if s.patches != nil {
		s.patches.Reset()
	}
}

// mockMarketData returns market data rich enough for backend entry sizing:
// price 50000, ATR 400, latest base volume 1000.
func (s *AutoTraderTestSuite) mockMarketData(price float64) {
	s.patches.ApplyFunc(market.Get, func(symbol string) (*market.Data, error) {
		return &market.Data{
			Symbol:       symbol,
			CurrentPrice: price,
			IntradaySeries: &market.IntradayData{
				ATR14:  400.0,
				Volume: []float64{900.0, 1000.0},
			},
		}, nil
	})
}

// ============================================================
// Utility function tests
// ============================================================

func (s *AutoTraderTestSuite) TestSortDecisionsByPriority() {
	input := []decision.Decision{
		{Action: "open_long", Symbol: "BTCUSDT"},
		{Action: "close_short", Symbol: "ETHUSDT"},
		{Action: "hold", Symbol: "BNBUSDT"},
		{Action: "open_short", Symbol: "ADAUSDT"},
		{Action: "close_long", Symbol: "DOGEUSDT"},
	}

	result := sortDecisionsByPriority(input)
	s.Equal(len(input), len(result))

	getActionPriority := func(action string) int {
		switch action {
		case "close_long", "close_short":
			return 1
		case "open_long", "open_short":
			return 2
		case "hold", "wait":
			return 3
		default:
			return 999
		}
	}
	for i := 0; i < len(result)-1; i++ {
		s.LessOrEqual(getActionPriority(result[i].Action), getActionPriority(result[i+1].Action),
			"close actions must execute before open actions")
	}
}

// ============================================================
// Getter/Setter tests
// ============================================================

func (s *AutoTraderTestSuite) TestGettersAndSetters() {
	s.Equal("test_trader", s.autoTrader.GetID())
	s.Equal("Test Trader", s.autoTrader.GetName())
	s.Equal("deepseek", s.autoTrader.GetAIModel())
	s.Equal("binance", s.autoTrader.GetExchange())

	s.autoTrader.SetCustomPrompt("custom prompt")
	s.Equal("custom prompt", s.autoTrader.customPrompt)
}

// ============================================================
// PeakPnL cache tests
// ============================================================

func (s *AutoTraderTestSuite) TestPeakPnLCache() {
	s.autoTrader.UpdatePeakPnL("BTCUSDT", "long", 10.5)
	s.Equal(10.5, s.autoTrader.GetPeakPnLCache()["BTCUSDT_long"])

	s.autoTrader.UpdatePeakPnL("BTCUSDT", "long", 15.0)
	s.Equal(15.0, s.autoTrader.GetPeakPnLCache()["BTCUSDT_long"])

	s.autoTrader.UpdatePeakPnL("BTCUSDT", "long", 12.0)
	s.Equal(15.0, s.autoTrader.GetPeakPnLCache()["BTCUSDT_long"], "peak must not decrease")

	s.autoTrader.ClearPeakPnLCache("BTCUSDT", "long")
	_, exists := s.autoTrader.GetPeakPnLCache()["BTCUSDT_long"]
	s.False(exists, "cleared key must not exist")
}

func (s *AutoTraderTestSuite) TestPeakPnLCacheKeyNormalization() {
	// Mixed-case symbol/side must land on the same cache key.
	s.autoTrader.UpdatePeakPnL(" btcusdt ", "LONG", 9.0)
	s.Equal(9.0, s.autoTrader.GetPeakPnLCache()["BTCUSDT_long"])

	s.autoTrader.ClearPeakPnLCache("BTCUSDT", "Long")
	_, exists := s.autoTrader.GetPeakPnLCache()["BTCUSDT_long"]
	s.False(exists, "normalized clear must remove the entry")
}

func (s *AutoTraderTestSuite) TestCheckPositionDrawdownDisabledByStrategy() {
	off := false
	original := s.strategyCfg.RiskControl.EnableTrailingProfitExit
	s.strategyCfg.RiskControl.EnableTrailingProfitExit = &off
	defer func() { s.strategyCfg.RiskControl.EnableTrailingProfitExit = original }()

	// This position/peak combination would trigger an emergency close when
	// the monitor is enabled (profit 6%% ≥ 5%%, drawdown 40%%).
	s.mockTrader.positions = []map[string]interface{}{
		{"symbol": "BTCUSDT", "side": "long", "positionAmt": 0.1, "entryPrice": 50000.0, "markPrice": 50300.0, "leverage": 10.0},
	}
	s.autoTrader.UpdatePeakPnL("BTCUSDT", "long", 10.0)

	s.autoTrader.checkPositionDrawdown()

	_, exists := s.autoTrader.GetPeakPnLCache()["BTCUSDT_long"]
	s.True(exists, "disabled monitor must not close positions or touch peak state")
}

func (s *AutoTraderTestSuite) TestCheckPositionDrawdownMissingLeverageDefaultsTo1x() {
	// No leverage field: PnL%% must be computed at 1x (0.6%%), far below the
	// 5%% trigger, so no emergency close happens even with a high cached peak.
	s.mockTrader.positions = []map[string]interface{}{
		{"symbol": "BTCUSDT", "side": "long", "positionAmt": 0.1, "entryPrice": 50000.0, "markPrice": 50300.0},
	}
	s.autoTrader.UpdatePeakPnL("BTCUSDT", "long", 10.0)

	s.autoTrader.checkPositionDrawdown()

	_, exists := s.autoTrader.GetPeakPnLCache()["BTCUSDT_long"]
	s.True(exists, "1x fallback must not trigger an emergency close")
}

// ============================================================
// GetStatus tests
// ============================================================

func (s *AutoTraderTestSuite) TestGetStatus() {
	s.autoTrader.lifecycleMutex.Lock()
	s.autoTrader.isRunning = true
	s.autoTrader.lifecycleMutex.Unlock()
	s.autoTrader.callCount.Store(15)

	status := s.autoTrader.GetStatus()

	s.Equal("test_trader", status["trader_id"])
	s.Equal("Test Trader", status["trader_name"])
	s.Equal("deepseek", status["ai_model"])
	s.Equal("binance", status["exchange"])
	s.True(status["is_running"].(bool))
	s.Equal(int64(15), status["call_count"])
	s.Equal(10000.0, status["initial_balance"])
}

// ============================================================
// GetAccountInfo tests
// ============================================================

func (s *AutoTraderTestSuite) TestGetAccountInfo() {
	accountInfo, err := s.autoTrader.GetAccountInfo()

	s.NoError(err)
	s.NotNil(accountInfo)
	s.Equal(10100.0, accountInfo["total_equity"]) // 10000 + 100
	s.Equal(8000.0, accountInfo["available_balance"])
	s.Equal(100.0, accountInfo["total_pnl"]) // 10100 - 10000
}

func (s *AutoTraderTestSuite) TestGetAccountInfoUsesAuthoritativeEquityAndSnakeCaseFields() {
	s.mockTrader.balance = map[string]interface{}{
		"total_equity":      "10250.50",
		"available_balance": 7000.25,
		"unrealized_pnl":    50.5,
		"margin_used":       3250.25,
	}
	s.mockTrader.positions = []map[string]interface{}{
		{"symbol": "BTCUSDT", "unrealized_pnl": "50.5"},
	}

	accountInfo, err := s.autoTrader.GetAccountInfo()

	s.NoError(err)
	s.Equal(10250.5, accountInfo["total_equity"])
	s.Equal(10200.0, accountInfo["wallet_balance"])
	s.Equal(7000.25, accountInfo["available_balance"])
	s.Equal(250.5, accountInfo["total_pnl"])
	s.Equal(3250.25, accountInfo["margin_used"])
}

func (s *AutoTraderTestSuite) TestGetAccountInfoRejectsMissingBalanceTotals() {
	s.mockTrader.balance = map[string]interface{}{"available_balance": 100.0}

	accountInfo, err := s.autoTrader.GetAccountInfo()

	s.Error(err)
	s.Nil(accountInfo)
}

// ============================================================
// GetPositions tests
// ============================================================

func (s *AutoTraderTestSuite) TestGetPositions() {
	s.Run("No positions", func() {
		positions, err := s.autoTrader.GetPositions()
		s.NoError(err)
		if positions != nil {
			s.Equal(0, len(positions))
		}
	})

	s.Run("Has positions", func() {
		s.mockTrader.positions = []map[string]interface{}{
			{
				"symbol":           "BTCUSDT",
				"side":             "long",
				"entryPrice":       50000.0,
				"markPrice":        51000.0,
				"positionAmt":      0.1,
				"unRealizedProfit": 100.0,
				"liquidationPrice": 45000.0,
				"leverage":         10.0,
			},
		}

		positions, err := s.autoTrader.GetPositions()

		s.NoError(err)
		s.Equal(1, len(positions))

		pos := positions[0]
		s.Equal("BTCUSDT", pos["symbol"])
		s.Equal("long", pos["side"])
		s.Equal(0.1, pos["quantity"])
		s.Equal(50000.0, pos["entry_price"])
	})
}

// ============================================================
// buildTradingContext tests
// ============================================================

func (s *AutoTraderTestSuite) TestBuildTradingContext() {
	ctx, err := s.autoTrader.buildTradingContext()

	s.NoError(err)
	s.NotNil(ctx)
	s.Equal(10100.0, ctx.Account.TotalEquity) // 10000 + 100
	s.Equal(8000.0, ctx.Account.AvailableBalance)
	s.Equal(10, ctx.BTCETHLeverage)
	s.Equal(5, ctx.AltcoinLeverage)
	s.Equal(2, len(ctx.CandidateCoins))
}

// ============================================================
// Trade execution tests (backend sizing pipeline)
// ============================================================

func (s *AutoTraderTestSuite) TestExecuteOpenPosition() {
	tests := []struct {
		name          string
		action        string
		stopLoss      float64 // long: below entry, short: above entry
		takeProfit    float64 // long: above entry, short: below entry
		expectedOrder int64
		existingSide  string
		availBalance  float64
		expectedErr   string
		executeFn     func(*decision.Decision, *store.DecisionAction) error
	}{
		{
			name:          "Successfully open long",
			action:        "open_long",
			stopLoss:      49000,
			takeProfit:    52000,
			expectedOrder: 123456,
			availBalance:  8000.0,
			executeFn: func(d *decision.Decision, a *store.DecisionAction) error {
				return s.autoTrader.executeOpenLongWithRecord(d, a)
			},
		},
		{
			name:          "Successfully open short",
			action:        "open_short",
			stopLoss:      51000,
			takeProfit:    48000,
			expectedOrder: 123457,
			availBalance:  8000.0,
			executeFn: func(d *decision.Decision, a *store.DecisionAction) error {
				return s.autoTrader.executeOpenShortWithRecord(d, a)
			},
		},
		{
			name:         "Long - zero available balance",
			action:       "open_long",
			stopLoss:     49000,
			takeProfit:   52000,
			availBalance: 0.0,
			expectedErr:  "must be greater than 0",
			executeFn: func(d *decision.Decision, a *store.DecisionAction) error {
				return s.autoTrader.executeOpenLongWithRecord(d, a)
			},
		},
		{
			name:         "Short - zero available balance",
			action:       "open_short",
			stopLoss:     51000,
			takeProfit:   48000,
			availBalance: 0.0,
			expectedErr:  "must be greater than 0",
			executeFn: func(d *decision.Decision, a *store.DecisionAction) error {
				return s.autoTrader.executeOpenShortWithRecord(d, a)
			},
		},
		{
			name:         "Long - already has same side position",
			action:       "open_long",
			stopLoss:     49000,
			takeProfit:   52000,
			existingSide: "long",
			availBalance: 8000.0,
			expectedErr:  "already has long position",
			executeFn: func(d *decision.Decision, a *store.DecisionAction) error {
				return s.autoTrader.executeOpenLongWithRecord(d, a)
			},
		},
		{
			name:         "Short - already has same side position",
			action:       "open_short",
			stopLoss:     51000,
			takeProfit:   48000,
			existingSide: "short",
			availBalance: 8000.0,
			expectedErr:  "already has short position",
			executeFn: func(d *decision.Decision, a *store.DecisionAction) error {
				return s.autoTrader.executeOpenShortWithRecord(d, a)
			},
		},
	}

	for _, tt := range tests {
		time.Sleep(time.Millisecond)
		s.Run(tt.name, func() {
			s.mockMarketData(50000.0)

			s.mockTrader.balance["availableBalance"] = tt.availBalance
			if tt.existingSide != "" {
				s.mockTrader.positions = []map[string]interface{}{{"symbol": "BTCUSDT", "side": tt.existingSide}}
			} else {
				s.mockTrader.positions = []map[string]interface{}{}
			}

			// Backend owns sizing: only direction, invalidation, and target
			// come from the decision.
			d := &decision.Decision{Action: tt.action, Symbol: "BTCUSDT", StopLoss: tt.stopLoss, TakeProfit: tt.takeProfit}
			actionRecord := &store.DecisionAction{Action: tt.action, Symbol: "BTCUSDT"}

			err := tt.executeFn(d, actionRecord)

			if tt.expectedErr != "" {
				s.Error(err)
				s.Contains(err.Error(), tt.expectedErr)
			} else {
				s.NoError(err)
				s.Equal(tt.expectedOrder, actionRecord.OrderID)
				s.Greater(actionRecord.Quantity, 0.0)
				s.Equal(50000.0, actionRecord.Price)
				// Backend sizing must overwrite the ignored AI-supplied fields.
				s.Greater(d.Leverage, 0)
				s.Greater(d.PositionSizeUSD, 0.0)
				s.Greater(d.RiskUSD, 0.0)
				s.Equal(d.Leverage, actionRecord.Leverage)
			}

			// Restore default state
			s.mockTrader.balance["availableBalance"] = 8000.0
			s.mockTrader.positions = []map[string]interface{}{}
		})
	}
}

func (s *AutoTraderTestSuite) TestExecuteClosePosition() {
	tests := []struct {
		name         string
		action       string
		currentPrice float64
		orderID      int64
		executeFn    func(*decision.Decision, *store.DecisionAction) error
	}{
		{
			name:         "Successfully close long",
			action:       "close_long",
			currentPrice: 51000.0,
			orderID:      123458,
			executeFn: func(d *decision.Decision, a *store.DecisionAction) error {
				return s.autoTrader.executeCloseLongWithRecord(d, a)
			},
		},
		{
			name:         "Successfully close short",
			action:       "close_short",
			currentPrice: 49000.0,
			orderID:      123459,
			executeFn: func(d *decision.Decision, a *store.DecisionAction) error {
				return s.autoTrader.executeCloseShortWithRecord(d, a)
			},
		},
	}

	for _, tt := range tests {
		time.Sleep(time.Millisecond)
		s.Run(tt.name, func() {
			s.mockMarketData(tt.currentPrice)

			d := &decision.Decision{Action: tt.action, Symbol: "BTCUSDT"}
			actionRecord := &store.DecisionAction{Action: tt.action, Symbol: "BTCUSDT"}

			err := tt.executeFn(d, actionRecord)

			s.NoError(err)
			s.Equal(tt.orderID, actionRecord.OrderID)
			s.Equal(tt.currentPrice, actionRecord.Price)
		})
	}
}

// ============================================================
// executeDecisionWithRecord routing tests
// ============================================================

func (s *AutoTraderTestSuite) TestExecuteDecisionWithRecord() {
	s.mockMarketData(50000.0)

	s.Run("Route to open_long", func() {
		d := &decision.Decision{Action: "open_long", Symbol: "BTCUSDT", StopLoss: 49000, TakeProfit: 52000}
		actionRecord := &store.DecisionAction{}

		err := s.autoTrader.executeDecisionWithRecord(d, actionRecord)
		s.NoError(err)
	})

	s.Run("Route to close_long", func() {
		d := &decision.Decision{Action: "close_long", Symbol: "BTCUSDT"}
		actionRecord := &store.DecisionAction{}

		err := s.autoTrader.executeDecisionWithRecord(d, actionRecord)
		s.NoError(err)
	})

	s.Run("Route to hold - no execution", func() {
		d := &decision.Decision{Action: "hold", Symbol: "BTCUSDT"}
		actionRecord := &store.DecisionAction{}

		err := s.autoTrader.executeDecisionWithRecord(d, actionRecord)
		s.NoError(err)
	})

	s.Run("Unknown action returns error", func() {
		d := &decision.Decision{Action: "unknown_action", Symbol: "BTCUSDT"}
		actionRecord := &store.DecisionAction{}

		err := s.autoTrader.executeDecisionWithRecord(d, actionRecord)
		s.Error(err)
		s.Contains(err.Error(), "unknown action")
	})
}

// ============================================================
// Drawdown monitor tests
// ============================================================

func (s *AutoTraderTestSuite) TestCheckPositionDrawdown() {
	tests := []struct {
		name             string
		setupPositions   func()
		setupPeakPnL     func()
		setupFailures    func()
		cleanupFailures  func()
		expectedCacheKey string
		shouldClearCache bool
		skipCacheCheck   bool
	}{
		{
			name:            "Get positions failed - no panic",
			setupFailures:   func() { s.mockTrader.shouldFailPositions = true },
			cleanupFailures: func() { s.mockTrader.shouldFailPositions = false },
			skipCacheCheck:  true,
		},
		{
			name:           "No positions - no panic",
			setupPositions: func() { s.mockTrader.positions = []map[string]interface{}{} },
			skipCacheCheck: true,
		},
		{
			name: "Malformed positions - no panic",
			setupPositions: func() {
				s.mockTrader.positions = []map[string]interface{}{
					{"symbol": "BTCUSDT"},               // missing side/prices
					{"side": "long", "entryPrice": 1.0}, // missing symbol
					{"symbol": "ETHUSDT", "side": "long", "entryPrice": "3000", "markPrice": "3010", "positionAmt": "0.5", "leverage": 10.0}, // string-typed prices
					{"symbol": "SOLUSDT", "side": "long", "entryPrice": 20.0, "markPrice": 20.4, "positionAmt": 10.0, "leverage": 10.0},
				}
			},
			skipCacheCheck: true,
		},
		{
			name: "Profit less than 5% - no close",
			setupPositions: func() {
				s.mockTrader.positions = []map[string]interface{}{
					{"symbol": "BTCUSDT", "side": "long", "positionAmt": 0.1, "entryPrice": 50000.0, "markPrice": 50150.0, "leverage": 10.0},
				}
			},
			setupPeakPnL:   func() { s.autoTrader.ClearPeakPnLCache("BTCUSDT", "long") },
			skipCacheCheck: true,
		},
		{
			name: "Drawdown less than 40% - no close",
			setupPositions: func() {
				s.mockTrader.positions = []map[string]interface{}{
					{"symbol": "BTCUSDT", "side": "long", "positionAmt": 0.1, "entryPrice": 50000.0, "markPrice": 50400.0, "leverage": 10.0},
				}
			},
			setupPeakPnL:   func() { s.autoTrader.UpdatePeakPnL("BTCUSDT", "long", 10.0) },
			skipCacheCheck: true,
		},
		{
			name: "Long - trigger drawdown close",
			setupPositions: func() {
				s.mockTrader.positions = []map[string]interface{}{
					{"symbol": "BTCUSDT", "side": "long", "positionAmt": 0.1, "entryPrice": 50000.0, "markPrice": 50300.0, "leverage": 10.0},
				}
			},
			setupPeakPnL:     func() { s.autoTrader.UpdatePeakPnL("BTCUSDT", "long", 10.0) },
			expectedCacheKey: "BTCUSDT_long",
			shouldClearCache: true,
		},
		{
			name: "Short - trigger drawdown close",
			setupPositions: func() {
				s.mockTrader.positions = []map[string]interface{}{
					{"symbol": "ETHUSDT", "side": "short", "positionAmt": -0.5, "entryPrice": 3000.0, "markPrice": 2982.0, "leverage": 10.0},
				}
			},
			setupPeakPnL:     func() { s.autoTrader.UpdatePeakPnL("ETHUSDT", "short", 10.0) },
			expectedCacheKey: "ETHUSDT_short",
			shouldClearCache: true,
		},
		{
			name: "Long - close failed - keep cache",
			setupPositions: func() {
				s.mockTrader.positions = []map[string]interface{}{
					{"symbol": "BTCUSDT", "side": "long", "positionAmt": 0.1, "entryPrice": 50000.0, "markPrice": 50300.0, "leverage": 10.0},
				}
			},
			setupPeakPnL:     func() { s.autoTrader.UpdatePeakPnL("BTCUSDT", "long", 10.0) },
			setupFailures:    func() { s.mockTrader.shouldFailCloseLong = true },
			cleanupFailures:  func() { s.mockTrader.shouldFailCloseLong = false },
			expectedCacheKey: "BTCUSDT_long",
			shouldClearCache: false,
		},
		{
			name: "Short - close failed - keep cache",
			setupPositions: func() {
				s.mockTrader.positions = []map[string]interface{}{
					{"symbol": "ETHUSDT", "side": "short", "positionAmt": -0.5, "entryPrice": 3000.0, "markPrice": 2982.0, "leverage": 10.0},
				}
			},
			setupPeakPnL:     func() { s.autoTrader.UpdatePeakPnL("ETHUSDT", "short", 10.0) },
			setupFailures:    func() { s.mockTrader.shouldFailCloseShort = true },
			cleanupFailures:  func() { s.mockTrader.shouldFailCloseShort = false },
			expectedCacheKey: "ETHUSDT_short",
			shouldClearCache: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			if tt.setupPositions != nil {
				tt.setupPositions()
			}
			if tt.setupPeakPnL != nil {
				tt.setupPeakPnL()
			}
			if tt.setupFailures != nil {
				tt.setupFailures()
			}
			if tt.cleanupFailures != nil {
				defer tt.cleanupFailures()
			}

			s.autoTrader.checkPositionDrawdown()

			if !tt.skipCacheCheck {
				cache := s.autoTrader.GetPeakPnLCache()
				_, exists := cache[tt.expectedCacheKey]
				if tt.shouldClearCache {
					s.False(exists, "Peak PnL cache should be cleared")
				} else {
					s.True(exists, "Peak PnL cache should not be cleared")
				}
			}

			s.mockTrader.positions = []map[string]interface{}{}
		})
	}
}

// ============================================================
// Mock exchange implementation
// ============================================================

// MockTrader implements the full trader.Trader interface with error control.
type MockTrader struct {
	balance              map[string]interface{}
	positions            []map[string]interface{}
	orderStatus          map[string]interface{}
	shouldFailBalance    bool
	shouldFailPositions  bool
	shouldFailOpenLong   bool
	shouldFailCloseLong  bool
	shouldFailCloseShort bool
}

func (m *MockTrader) GetBalance() (map[string]interface{}, error) {
	if m.shouldFailBalance {
		return nil, errors.New("failed to get balance")
	}
	if m.balance == nil {
		return map[string]interface{}{
			"totalWalletBalance":    10000.0,
			"availableBalance":      8000.0,
			"totalUnrealizedProfit": 100.0,
		}, nil
	}
	return m.balance, nil
}

func (m *MockTrader) GetPositions() ([]map[string]interface{}, error) {
	if m.shouldFailPositions {
		return nil, errors.New("failed to get positions")
	}
	if m.positions == nil {
		return []map[string]interface{}{}, nil
	}
	return m.positions, nil
}

func (m *MockTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	if m.shouldFailOpenLong {
		return nil, errors.New("failed to open long")
	}
	return map[string]interface{}{
		"orderId":   int64(123456),
		"symbol":    symbol,
		"orderType": "MARKET",
	}, nil
}

func (m *MockTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return map[string]interface{}{
		"orderId":   int64(123457),
		"symbol":    symbol,
		"orderType": "MARKET",
	}, nil
}

func (m *MockTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	if m.shouldFailCloseLong {
		return nil, errors.New("failed to close long")
	}
	return map[string]interface{}{
		"orderId":   int64(123458),
		"symbol":    symbol,
		"orderType": "MARKET",
	}, nil
}

func (m *MockTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	if m.shouldFailCloseShort {
		return nil, errors.New("failed to close short")
	}
	return map[string]interface{}{
		"orderId":   int64(123459),
		"symbol":    symbol,
		"orderType": "MARKET",
	}, nil
}

func (m *MockTrader) SetLeverage(symbol string, leverage int) error { return nil }

func (m *MockTrader) SetMarginMode(symbol string, isCrossMargin bool) error { return nil }

func (m *MockTrader) GetMarketPrice(symbol string) (float64, error) { return 50000.0, nil }

func (m *MockTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	return nil
}

func (m *MockTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return nil
}

func (m *MockTrader) CancelStopLossOrders(symbol string) error   { return nil }
func (m *MockTrader) CancelTakeProfitOrders(symbol string) error { return nil }
func (m *MockTrader) CancelAllOrders(symbol string) error        { return nil }
func (m *MockTrader) CancelStopOrders(symbol string) error       { return nil }

func (m *MockTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return fmt.Sprintf("%.4f", quantity), nil
}

func (m *MockTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	if m.orderStatus == nil {
		return map[string]interface{}{
			"status":      "FILLED",
			"avgPrice":    50000.0,
			"executedQty": 0.05,
			"commission":  0.5,
		}, nil
	}
	return m.orderStatus, nil
}

func (m *MockTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
	return nil, nil
}

// ============================================================
// Test suite entry point
// ============================================================

func TestAutoTraderTestSuite(t *testing.T) {
	suite.Run(t, new(AutoTraderTestSuite))
}

// ============================================================
// Independent unit tests - calculatePnLPercentage
// ============================================================

func TestCalculatePnLPercentage(t *testing.T) {
	tests := []struct {
		name          string
		unrealizedPnl float64
		marginUsed    float64
		expected      float64
	}{
		{"Normal profit - 10x leverage", 100.0, 1000.0, 10.0},
		{"Normal loss - 10x leverage", -50.0, 1000.0, -5.0},
		{"High leverage profit", 200.0, 1000.0, 20.0},
		{"Zero margin - edge case", 100.0, 0.0, 0.0},
		{"Negative margin - edge case", 100.0, -1000.0, 0.0},
		{"Zero PnL", 0.0, 1000.0, 0.0},
		{"Small trade", 0.5, 10.0, 5.0},
		{"Large profit", 5000.0, 10000.0, 50.0},
		{"Tiny margin", 1.0, 0.01, 10000.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculatePnLPercentage(tt.unrealizedPnl, tt.marginUsed)
			if math.Abs(result-tt.expected) > 0.0001 {
				t.Errorf("calculatePnLPercentage(%v, %v) = %v, want %v",
					tt.unrealizedPnl, tt.marginUsed, result, tt.expected)
			}
		})
	}
}

func TestCalculatePnLPercentage_RealWorldScenarios(t *testing.T) {
	scenarios := []struct {
		name                  string
		pnl, margin, expected float64
	}{
		{"BTC 10x leverage, 2% price increase", 200.0, 1000.0, 20.0},
		{"ETH 5x leverage, 3% price decrease", -300.0, 2000.0, -15.0},
		{"SOL 20x leverage, 0.5% price increase", 50.0, 500.0, 10.0},
	}

	for _, tt := range scenarios {
		t.Run(tt.name, func(t *testing.T) {
			result := calculatePnLPercentage(tt.pnl, tt.margin)
			if math.Abs(result-tt.expected) > 0.0001 {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}
