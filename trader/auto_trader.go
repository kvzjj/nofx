package trader

import (
	"encoding/json"
	"fmt"
	"math"
	"nofx/decision"
	"nofx/logger"
	"nofx/market"
	"nofx/mcp"
	"nofx/metrics"
	"nofx/notification"
	"nofx/store"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// AutoTraderConfig auto trading configuration (simplified version - AI makes all decisions)
type AutoTraderConfig struct {
	// Trader identification
	ID      string // Trader unique identifier (for log directory, etc.)
	Name    string // Trader display name
	AIModel string // AI model: "qwen" or "deepseek"

	// Trading platform selection
	Exchange   string // Exchange type: "binance"
	ExchangeID string // Exchange account UUID (for multi-account support)

	// Binance API configuration
	BinanceAPIKey    string
	BinanceSecretKey string
	BinanceTestnet   bool

	// AI configuration
	UseQwen     bool
	DeepSeekKey string
	QwenKey     string

	// Custom AI API configuration
	CustomAPIURL    string
	CustomAPIKey    string
	CustomModelName string

	// Scan configuration
	ScanInterval time.Duration // Scan interval (recommended 3 minutes)

	// Account configuration
	InitialBalance float64 // Initial balance (for P&L calculation, must be set manually)

	// Account-level hard circuit breakers. Strategy risk-control values take precedence.
	MaxDailyLoss    float64       // Maximum daily loss percentage
	MaxDrawdown     float64       // Maximum equity high-water drawdown percentage
	StopTradingTime time.Duration // Pause duration after risk control triggers

	// Position mode
	IsCrossMargin bool // true=cross margin mode, false=isolated margin mode

	// Order execution
	OrderType                  OrderType
	LimitPriceOffsetPct        float64
	ProtectionRetries          int
	ProtectionRetryDelay       time.Duration
	ProtectionFailureAction    string
	ProtectionFailureReducePct float64

	// Pending limit entry order monitoring
	PendingOrderPollInterval time.Duration // Status polling interval
	PendingOrderTimeout      time.Duration // Monitoring window before the order is canceled

	// Competition visibility
	ShowInCompetition bool // Whether to show in competition page

	// Strategy configuration (use complete strategy config)
	StrategyConfig *store.StrategyConfig // Strategy configuration (includes coin sources, indicators, risk control, prompts, etc.)
}

// AutoTrader automatic trader
type AutoTrader struct {
	id                      string // Trader unique identifier
	name                    string // Trader display name
	aiModel                 string // AI model name
	exchange                string // Trading platform type (binance/bybit/etc)
	exchangeID              string // Exchange account UUID
	showInCompetition       bool   // Whether to show in competition page
	config                  AutoTraderConfig
	trader                  Trader // Use Trader interface (supports multiple platforms)
	mcpClient               mcp.AIClient
	store                   *store.Store             // Data storage (decision records, etc.)
	strategyEngine          *decision.StrategyEngine // Strategy engine (uses strategy configuration)
	cycleNumber             int                      // Current cycle number
	initialBalance          float64
	dailyPnL                float64
	dayStartEquity          float64
	equityHighWater         float64
	consecutiveFailures     int
	customPrompt            string // Custom trading strategy prompt
	overrideBasePrompt      bool   // Whether to override base prompt
	lastResetTime           time.Time
	stopUntil               time.Time
	manualReviewRequired    bool
	lifecycleMutex          sync.Mutex
	isRunning               bool
	isStopping              bool
	startTime               time.Time                         // System start time
	callCount               atomic.Int64                      // AI call count (read by API goroutines)
	positionFirstSeenTime   map[string]int64                  // Position first seen time (symbol_side -> timestamp in milliseconds)
	stopMonitorCh           chan struct{}                     // Used to stop monitoring goroutine
	monitorWg               sync.WaitGroup                    // Used to wait for monitoring goroutine to finish
	peakPnLCache            map[string]peakPnLEntry           // Peak profit cache (symbol_side -> peak record)
	peakPnLCacheMutex       sync.RWMutex                      // Cache read-write lock
	entryMutex              sync.Mutex                        // Serializes risk snapshot and entry submission
	riskMutex               sync.Mutex                        // Protects account-level circuit-breaker state
	feedbackMutex           sync.Mutex                        // Protects recentExecutionFailures
	recentExecutionFailures []decision.RecentExecutionFailure // Recent failed instructions fed back to the AI
	lastBalanceSyncTime     time.Time                         // Last balance sync time
	userID                  string                            // User ID
}

// NewAutoTrader creates an automatic trader
// st parameter is used to store decision records to database
func NewAutoTrader(config AutoTraderConfig, st *store.Store, userID string) (*AutoTrader, error) {
	// Set default values
	if config.ID == "" {
		config.ID = "default_trader"
	}
	if config.Name == "" {
		config.Name = "Default Trader"
	}
	if config.AIModel == "" {
		if config.UseQwen {
			config.AIModel = "qwen"
		} else {
			config.AIModel = "deepseek"
		}
	}

	// Initialize AI client based on provider
	var mcpClient mcp.AIClient
	aiModel := config.AIModel
	if config.UseQwen && aiModel == "" {
		aiModel = "qwen"
	}

	switch aiModel {
	case "claude":
		mcpClient = mcp.NewClaudeClient()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using Claude AI", config.Name)

	case "kimi":
		mcpClient = mcp.NewKimiClient()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using Kimi (Moonshot) AI", config.Name)

	case "gemini":
		mcpClient = mcp.NewGeminiClient()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using Google Gemini AI", config.Name)

	case "grok":
		mcpClient = mcp.NewGrokClient()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using xAI Grok AI", config.Name)

	case "openai":
		mcpClient = mcp.NewOpenAIClient()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using OpenAI", config.Name)

	case "qwen":
		mcpClient = mcp.NewQwenClient()
		apiKey := config.QwenKey
		if apiKey == "" {
			apiKey = config.CustomAPIKey
		}
		mcpClient.SetAPIKey(apiKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using Alibaba Cloud Qwen AI", config.Name)

	case "custom":
		mcpClient = mcp.New()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using custom AI API: %s (model: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)

	default: // deepseek or empty
		mcpClient = mcp.NewDeepSeekClient()
		apiKey := config.DeepSeekKey
		if apiKey == "" {
			apiKey = config.CustomAPIKey
		}
		mcpClient.SetAPIKey(apiKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using DeepSeek AI", config.Name)
	}

	if config.CustomAPIURL != "" || config.CustomModelName != "" {
		logger.Infof("🔧 [%s] Custom config - URL: %s, Model: %s", config.Name, config.CustomAPIURL, config.CustomModelName)
	}

	// Set default trading platform
	if config.Exchange == "" {
		config.Exchange = "binance"
	}
	if config.OrderType == "" {
		config.OrderType = OrderTypeMarket
	}
	if config.OrderType != OrderTypeMarket && config.OrderType != OrderTypeLimit {
		logger.Infof("⚠️ [%s] Unsupported order type %q, falling back to market", config.Name, config.OrderType)
		config.OrderType = OrderTypeMarket
	}
	if config.LimitPriceOffsetPct <= 0 {
		config.LimitPriceOffsetPct = 0.05
	}
	if config.ProtectionRetries <= 0 {
		config.ProtectionRetries = 3
	}
	if config.ProtectionRetryDelay <= 0 {
		config.ProtectionRetryDelay = time.Second
	}
	if config.ProtectionFailureAction == "" {
		config.ProtectionFailureAction = "close"
	}
	if !strings.EqualFold(config.ProtectionFailureAction, "close") {
		logger.Warnf("[%s] protection failure action %q is unsafe; enforcing immediate close", config.Name, config.ProtectionFailureAction)
		config.ProtectionFailureAction = "close"
	}
	if config.ProtectionFailureReducePct <= 0 || config.ProtectionFailureReducePct > 100 {
		config.ProtectionFailureReducePct = 50
	}
	if config.PendingOrderPollInterval <= 0 {
		config.PendingOrderPollInterval = 2 * time.Second
	}
	if config.PendingOrderTimeout <= 0 {
		config.PendingOrderTimeout = 30 * time.Minute
	}

	// Create corresponding trader based on configuration
	var trader Trader

	// Record position mode (general)
	marginModeStr := "Cross Margin"
	if !config.IsCrossMargin {
		marginModeStr = "Isolated Margin"
	}
	logger.Infof("📊 [%s] Position mode: %s", config.Name, marginModeStr)

	switch config.Exchange {
	case "binance":
		logger.Infof("🏦 [%s] Using Binance Futures trading (testnet=%v)", config.Name, config.BinanceTestnet)
		trader = NewFuturesTraderWithTestnet(config.BinanceAPIKey, config.BinanceSecretKey, userID, config.BinanceTestnet)
	case "paper":
		logger.Infof("📒 [%s] Using paper trading (simulated fills at real market prices)", config.Name)
		paper, perr := NewPaperTrader(config.ID, config.InitialBalance, st)
		if perr != nil {
			return nil, fmt.Errorf("failed to create paper trading account: %w", perr)
		}
		trader = paper
	default:
		return nil, fmt.Errorf("unsupported trading platform: %s", config.Exchange)
	}

	// Simulated exchanges run a background matcher for resting orders
	// (limit entries, stop-loss/take-profit triggers).
	if engine, ok := trader.(MatchEngine); ok {
		engine.Start()
	}

	// Validate initial balance configuration, auto-fetch from exchange if 0
	if config.InitialBalance <= 0 {
		logger.Infof("📊 [%s] Initial balance not set, attempting to fetch current balance from exchange...", config.Name)
		account, err := trader.GetBalance()
		if err != nil {
			return nil, fmt.Errorf("initial balance not set and unable to fetch balance from exchange: %w", err)
		}
		// Try multiple balance field names (different exchanges return different formats)
		balanceKeys := []string{"total_equity", "totalWalletBalance", "wallet_balance", "totalEq", "balance"}
		var foundBalance float64
		for _, key := range balanceKeys {
			if balance, ok := account[key].(float64); ok && balance > 0 {
				foundBalance = balance
				break
			}
		}
		if foundBalance > 0 {
			config.InitialBalance = foundBalance
			logger.Infof("✓ [%s] Auto-fetched initial balance: %.2f USDT", config.Name, foundBalance)
			// Save to database so it persists across restarts
			if st != nil {
				if err := st.Trader().UpdateInitialBalance(userID, config.ID, foundBalance); err != nil {
					logger.Infof("⚠️  [%s] Failed to save initial balance to database: %v", config.Name, err)
				} else {
					logger.Infof("✓ [%s] Initial balance saved to database", config.Name)
				}
			}
		} else {
			return nil, fmt.Errorf("initial balance must be greater than 0, please set InitialBalance in config or ensure exchange account has balance")
		}
	}

	// Get last cycle number (for recovery)
	var cycleNumber int
	if st != nil {
		cycleNumber, _ = st.Decision().GetLastCycleNumber(config.ID)
		logger.Infof("📊 [%s] Decision records will be stored to database", config.Name)
	}

	// Create strategy engine (must have strategy config)
	if config.StrategyConfig == nil {
		return nil, fmt.Errorf("[%s] strategy not configured", config.Name)
	}
	strategyEngine := decision.NewStrategyEngine(config.StrategyConfig)
	logger.Infof("✓ [%s] Using strategy engine (strategy configuration loaded)", config.Name)

	return &AutoTrader{
		id:                    config.ID,
		name:                  config.Name,
		aiModel:               config.AIModel,
		exchange:              config.Exchange,
		exchangeID:            config.ExchangeID,
		showInCompetition:     config.ShowInCompetition,
		config:                config,
		trader:                trader,
		mcpClient:             mcpClient,
		store:                 st,
		strategyEngine:        strategyEngine,
		cycleNumber:           cycleNumber,
		initialBalance:        config.InitialBalance,
		dayStartEquity:        config.InitialBalance,
		equityHighWater:       config.InitialBalance,
		lastResetTime:         time.Now(),
		startTime:             time.Now(),
		isRunning:             false,
		positionFirstSeenTime: make(map[string]int64),
		stopMonitorCh:         make(chan struct{}),
		monitorWg:             sync.WaitGroup{},
		peakPnLCache:          make(map[string]peakPnLEntry),
		peakPnLCacheMutex:     sync.RWMutex{},
		lastBalanceSyncTime:   time.Now(),
		userID:                userID,
	}, nil
}

// Start atomically marks the trader as running before launching its main loop.
func (at *AutoTrader) Start() error {
	stopCh, err := at.beginRun()
	if err != nil {
		return err
	}

	go func() {
		if err := at.run(stopCh); err != nil {
			logger.Infof("❌ Trader %s runtime error: %v", at.GetName(), err)
		}
	}()
	return nil
}

// Run runs the automatic trading main loop and blocks until it stops.
func (at *AutoTrader) Run() error {
	stopCh, err := at.beginRun()
	if err != nil {
		return err
	}
	return at.run(stopCh)
}

func (at *AutoTrader) beginRun() (chan struct{}, error) {
	at.lifecycleMutex.Lock()
	defer at.lifecycleMutex.Unlock()

	if at.isRunning {
		return nil, fmt.Errorf("trader %s is already running", at.id)
	}

	at.isRunning = true
	at.isStopping = false
	at.stopMonitorCh = make(chan struct{})
	at.startTime = time.Now()
	at.monitorWg.Add(1)
	return at.stopMonitorCh, nil
}

func (at *AutoTrader) run(stopCh <-chan struct{}) error {
	defer at.monitorWg.Done()

	logger.Info("🚀 AI-driven automatic trading system started")
	logger.Infof("💰 Initial balance: %.2f USDT", at.initialBalance)
	logger.Infof("⚙️  Scan interval: %v", at.config.ScanInterval)
	logger.Info("🤖 AI proposes direction/invalidation/target; backend determines permission, size, and leverage")
	at.loadPeakPnLs()
	at.resumeEntryOrderSagas()
	// Start drawdown monitoring
	at.startDrawdownMonitor(stopCh)

	ticker := time.NewTicker(at.config.ScanInterval)
	defer ticker.Stop()

	// Execute immediately on first run
	if err := at.runCycle(); err != nil {
		logger.Infof("❌ Execution failed: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := at.runCycle(); err != nil {
				logger.Infof("❌ Execution failed: %v", err)
			}
		case <-stopCh:
			logger.Infof("[%s] ⏹ Stop signal received, exiting automatic trading main loop", at.name)
			return nil
		}
	}
}

// Stop stops the automatic trading
func (at *AutoTrader) Stop() {
	at.lifecycleMutex.Lock()
	if !at.isRunning {
		at.lifecycleMutex.Unlock()
		return
	}
	if at.isStopping {
		at.lifecycleMutex.Unlock()
		at.monitorWg.Wait()
		return
	}
	at.isStopping = true
	close(at.stopMonitorCh) // Notify monitoring goroutine to stop
	// Simulated exchanges must also stop their background matcher so trader
	// reloads do not leak goroutines.
	if engine, ok := at.trader.(MatchEngine); ok {
		engine.Stop()
	}
	at.lifecycleMutex.Unlock()
	at.monitorWg.Wait() // Wait for monitoring goroutine to finish
	at.lifecycleMutex.Lock()
	at.isRunning = false
	at.isStopping = false
	at.lifecycleMutex.Unlock()
	logger.Info("⏹ Automatic trading system stopped")
}

// IsRunning reports whether the trader has started and not yet been stopped.
func (at *AutoTrader) IsRunning() bool {
	at.lifecycleMutex.Lock()
	defer at.lifecycleMutex.Unlock()
	return at.isRunning
}

// runCycle runs one trading cycle (using AI full decision-making)
func (at *AutoTrader) runCycle() error {
	cycle := at.callCount.Add(1)

	logger.Info("\n" + strings.Repeat("=", 70) + "\n")
	logger.Infof("⏰ %s - AI decision cycle #%d", time.Now().Format("2006-01-02 15:04:05"), cycle)
	logger.Info(strings.Repeat("=", 70))

	// Create decision record
	record := &store.DecisionRecord{
		ExecutionLog: []string{},
		Success:      true,
	}

	// A circuit breaker never prevents closing risk. Entry checks below enforce
	// the pause while this cycle continues to process close decisions.
	at.riskMutex.Lock()
	stopUntil := at.stopUntil
	at.riskMutex.Unlock()
	if time.Now().Before(stopUntil) {
		remaining := stopUntil.Sub(time.Now())
		logger.Infof("⏸ Risk control: Trading paused, remaining %.0f minutes", remaining.Minutes())
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("Risk control paused, remaining %.0f minutes", remaining.Minutes())
		record.ExecutionLog = append(record.ExecutionLog, record.ErrorMessage)
	}

	// 2. Collect trading context
	ctx, err := at.buildTradingContext()
	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("Failed to build trading context: %v", err)
		at.saveDecision(record)
		return fmt.Errorf("failed to build trading context: %w", err)
	}

	// Save equity snapshot independently (decoupled from AI decision, used for drawing profit curve)
	at.saveEquitySnapshot(ctx)
	if at.evaluateAccountRisk(ctx) {
		record.ExecutionLog = append(record.ExecutionLog, "Account circuit breaker active: new entries are blocked")
	}

	logger.Info(strings.Repeat("=", 70))
	for _, coin := range ctx.CandidateCoins {
		record.CandidateCoins = append(record.CandidateCoins, coin.Symbol)
	}

	logger.Infof("📊 Account equity: %.2f USDT | Available: %.2f USDT | Positions: %d",
		ctx.Account.TotalEquity, ctx.Account.AvailableBalance, ctx.Account.PositionCount)

	// 5. Use strategy engine to call AI for decision
	logger.Infof("🤖 Requesting AI analysis and decision... [Strategy Engine]")
	aiDecision, err := decision.GetFullDecisionWithStrategy(ctx, at.mcpClient, at.strategyEngine, "balanced")

	if aiDecision != nil && aiDecision.AIRequestDurationMs > 0 {
		record.AIRequestDurationMs = aiDecision.AIRequestDurationMs
		logger.Infof("⏱️ AI call duration: %.2f seconds", float64(record.AIRequestDurationMs)/1000)
		record.ExecutionLog = append(record.ExecutionLog,
			fmt.Sprintf("AI call duration: %d ms", record.AIRequestDurationMs))
	}

	// Save chain of thought, decisions, and input prompt even if there's an error (for debugging)
	if aiDecision != nil {
		record.SystemPrompt = aiDecision.SystemPrompt // Save system prompt
		record.InputPrompt = aiDecision.UserPrompt
		record.CoTTrace = aiDecision.CoTTrace
		record.RawResponse = aiDecision.RawResponse // Save raw AI response for debugging
		if len(aiDecision.Decisions) > 0 {
			decisionJSON, _ := json.MarshalIndent(aiDecision.Decisions, "", "  ")
			record.DecisionJSON = string(decisionJSON)
		}
	}

	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("Failed to get AI decision: %v", err)

		// Print system prompt and AI chain of thought (output even with errors for debugging)
		if aiDecision != nil {
			logger.Info("\n" + strings.Repeat("=", 70) + "\n")
			logger.Infof("📋 System prompt (error case)")
			logger.Info(strings.Repeat("=", 70))
			logger.Info(aiDecision.SystemPrompt)
			logger.Info(strings.Repeat("=", 70))

			if aiDecision.CoTTrace != "" {
				logger.Info("\n" + strings.Repeat("-", 70) + "\n")
				logger.Info("💭 AI chain of thought analysis (error case):")
				logger.Info(strings.Repeat("-", 70))
				logger.Info(aiDecision.CoTTrace)
				logger.Info(strings.Repeat("-", 70))
			}
		}

		at.saveDecision(record)
		return fmt.Errorf("failed to get AI decision: %w", err)
	}

	for _, validationError := range aiDecision.ValidationErrors {
		message := fmt.Sprintf("⚠ Skipped invalid AI decision: %s", validationError)
		logger.Info(message)
		record.ExecutionLog = append(record.ExecutionLog, message)
	}

	// // 5. Print system prompt
	// logger.Infof("\n" + strings.Repeat("=", 70))
	// logger.Infof("📋 System prompt [template: %s]", at.systemPromptTemplate)
	// logger.Info(strings.Repeat("=", 70))
	// logger.Info(decision.SystemPrompt)
	// logger.Infof(strings.Repeat("=", 70) + "\n")

	// 6. Print AI chain of thought
	// logger.Infof("\n" + strings.Repeat("-", 70))
	// logger.Info("💭 AI chain of thought analysis:")
	// logger.Info(strings.Repeat("-", 70))
	// logger.Info(decision.CoTTrace)
	// logger.Infof(strings.Repeat("-", 70) + "\n")

	// 7. Print AI decisions
	// logger.Infof("📋 AI decision list (%d items):\n", len(decision.Decisions))
	// for i, d := range decision.Decisions {
	//     logger.Infof("  [%d] %s: %s - %s", i+1, d.Symbol, d.Action, d.Reasoning)
	//     if d.Action == "open_long" || d.Action == "open_short" {
	//        logger.Infof("      Leverage: %dx | Position: %.2f USDT | Stop loss: %.4f | Take profit: %.4f",
	//           d.Leverage, d.PositionSizeUSD, d.StopLoss, d.TakeProfit)
	//     }
	// }
	logger.Info()
	logger.Info(strings.Repeat("-", 70))
	// 8. Sort decisions: ensure close positions first, then open positions (prevent position stacking overflow)
	sortedDecisions := sortDecisionsByPriority(aiDecision.Decisions)

	logger.Info("🔄 Execution order (optimized): Close positions first → Open positions later")
	for i, d := range sortedDecisions {
		logger.Infof("  [%d] %s %s", i+1, d.Symbol, d.Action)
	}
	logger.Info()

	// Execute decisions and record results
	for _, d := range sortedDecisions {
		actionRecord := store.DecisionAction{
			Action:    d.Action,
			Symbol:    d.Symbol,
			Quantity:  0,
			Leverage:  d.Leverage,
			Price:     0,
			Timestamp: time.Now(),
			Success:   false,
		}

		if err := at.executeDecisionWithRecord(&d, &actionRecord); err != nil {
			at.recordExecutionFailure(err)
			logger.Infof("❌ Failed to execute decision (%s %s): %v", d.Symbol, d.Action, err)
			actionRecord.Error = err.Error()
			record.Success = false
			if record.ErrorMessage == "" {
				record.ErrorMessage = fmt.Sprintf("%s %s failed: %v", d.Symbol, d.Action, err)
			} else {
				record.ErrorMessage += fmt.Sprintf("; %s %s failed: %v", d.Symbol, d.Action, err)
			}
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("❌ %s %s failed: %v", d.Symbol, d.Action, err))
		} else {
			// Passive decisions did not test the exchange and must not erase a
			// previous execution failure in this or an earlier cycle.
			if d.Action != "hold" && d.Action != "wait" {
				at.recordExecutionSuccess()
			}
			actionRecord.Success = true
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("✓ %s %s succeeded", d.Symbol, d.Action))
			// Brief delay after successful execution
			time.Sleep(1 * time.Second)
		}

		record.Decisions = append(record.Decisions, actionRecord)
	}

	// 9. Save decision record
	if err := at.saveDecision(record); err != nil {
		logger.Infof("⚠ Failed to save decision record: %v", err)
	}

	return nil
}

// buildTradingContext builds trading context
func (at *AutoTrader) buildTradingContext() (*decision.Context, error) {
	// 1. Get account information
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("failed to get account balance: %w", err)
	}

	// Normalize account fields across exchanges. Some adapters expose camelCase
	// Binance-compatible keys while others (notably Lighter) use snake_case.
	totalUnrealizedProfit, _ := accountNumber(balance,
		"totalUnrealizedProfit", "unrealized_profit", "unrealized_pnl")
	totalWalletBalance, hasWalletBalance := accountNumber(balance,
		"totalWalletBalance", "wallet_balance", "walletBalance")
	totalEquity, hasTotalEquity := accountNumber(balance,
		"total_equity", "totalEquity", "totalEq", "accountEquity")
	if !hasTotalEquity {
		if !hasWalletBalance {
			return nil, fmt.Errorf("balance response does not contain total equity or wallet balance")
		}
		totalEquity = totalWalletBalance + totalUnrealizedProfit
	}
	if !hasWalletBalance {
		totalWalletBalance = totalEquity - totalUnrealizedProfit
	}
	availableBalance, ok := accountNumber(balance,
		"availableBalance", "available_balance", "available", "availBal")
	if !ok {
		return nil, fmt.Errorf("balance response does not contain available balance")
	}

	// 2. Get position information
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var positionInfos []decision.PositionInfo
	totalMarginUsed := 0.0

	// Current position key set (for cleaning up closed position records)
	currentPositionKeys := make(map[string]bool)

	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		symbol = strings.ToUpper(strings.TrimSpace(symbol))
		side, _ := pos["side"].(string)
		side = strings.ToLower(strings.TrimSpace(side))
		entryPrice, _ := accountNumber(pos, "entryPrice", "entry_price")
		markPrice, _ := accountNumber(pos, "markPrice", "mark_price")
		quantity, _ := accountNumber(pos, "positionAmt", "size", "quantity")
		if quantity < 0 {
			quantity = -quantity // Short position quantity is negative, convert to positive
		}

		// Skip closed positions (quantity = 0), prevent "ghost positions" from being passed to AI
		if quantity == 0 {
			continue
		}

		if symbol == "" || (side != "long" && side != "short") || entryPrice <= 0 || markPrice <= 0 {
			logger.Warnf("[%s] skipping malformed exchange position: symbol=%q side=%q entry=%.8f mark=%.8f", at.name, symbol, side, entryPrice, markPrice)
			continue
		}

		unrealizedPnl, _ := accountNumber(pos, "unRealizedProfit", "unrealized_pnl", "unrealizedPnl")
		liquidationPrice, _ := accountNumber(pos, "liquidationPrice", "liquidation_price")

		// Calculate margin used (estimated)
		leverage := 10
		if lev, ok := accountNumber(pos, "leverage"); ok && lev > 0 {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed

		// Calculate P&L percentage (based on margin, considering leverage)
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)

		// Get position open time from exchange (preferred) or fallback to local tracking
		posKey := symbol + "_" + side
		currentPositionKeys[posKey] = true

		var updateTime int64
		// Priority 1: Get from database (trader_positions table) - most accurate
		if at.store != nil {
			if dbPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, symbol, side); err == nil && dbPos != nil {
				if !dbPos.EntryTime.IsZero() {
					updateTime = dbPos.EntryTime.UnixMilli()
				}
			}
		}
		// Priority 2: Get from exchange API (Bybit: createdTime, OKX: createdTime)
		if updateTime == 0 {
			if createdTime, ok := pos["createdTime"].(int64); ok && createdTime > 0 {
				updateTime = createdTime
			}
		}
		// Priority 3: Fallback to local tracking
		if updateTime == 0 {
			if _, exists := at.positionFirstSeenTime[posKey]; !exists {
				at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()
				// A position seen for the first time must not inherit the peak
				// recorded for a previous position on the same symbol+side (the
				// old one closed without going through our close handlers, e.g.
				// via an exchange-side stop-loss). Peaks restored from the DB are
				// exempt: loadPeakPnLs seeds positionFirstSeenTime for them.
				at.ClearPeakPnLCache(symbol, side)
			}
			updateTime = at.positionFirstSeenTime[posKey]
		}

		// Get peak profit rate for this position
		at.peakPnLCacheMutex.RLock()
		peakPnlPct := at.peakPnLCache[posKey].Pct
		at.peakPnLCacheMutex.RUnlock()

		positionInfos = append(positionInfos, decision.PositionInfo{
			Symbol:           symbol,
			Side:             side,
			EntryPrice:       entryPrice,
			MarkPrice:        markPrice,
			Quantity:         quantity,
			Leverage:         leverage,
			UnrealizedPnL:    unrealizedPnl,
			UnrealizedPnLPct: pnlPct,
			PeakPnLPct:       peakPnlPct,
			LiquidationPrice: liquidationPrice,
			MarginUsed:       marginUsed,
			UpdateTime:       updateTime,
		})
	}

	// Clean up closed position records
	for key := range at.positionFirstSeenTime {
		if !currentPositionKeys[key] {
			delete(at.positionFirstSeenTime, key)
		}
	}

	// 3. Use strategy engine to get candidate coins (must have strategy engine)
	if at.strategyEngine == nil {
		return nil, fmt.Errorf("trader has no strategy engine configured")
	}
	candidateCoins, err := at.strategyEngine.GetCandidateCoins()
	if err != nil {
		return nil, fmt.Errorf("failed to get candidate coins: %w", err)
	}
	logger.Infof("📋 [%s] Strategy engine fetched candidate coins: %d", at.name, len(candidateCoins))

	// 4. Calculate total P&L
	totalPnL := totalEquity - at.initialBalance
	totalPnLPct := 0.0
	if at.initialBalance > 0 {
		totalPnLPct = (totalPnL / at.initialBalance) * 100
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	// 5. Get leverage from strategy config
	strategyConfig := at.strategyEngine.GetConfig()
	btcEthLeverage := strategyConfig.RiskControl.BTCETHMaxLeverage
	altcoinLeverage := strategyConfig.RiskControl.AltcoinMaxLeverage
	logger.Infof("📋 [%s] Strategy leverage config: BTC/ETH=%dx, Altcoin=%dx", at.name, btcEthLeverage, altcoinLeverage)

	// 6. Build context
	ctx := &decision.Context{
		CurrentTime:     time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		RuntimeMinutes:  int(time.Since(at.startTime).Minutes()),
		CallCount:       int(at.callCount.Load()),
		BTCETHLeverage:  btcEthLeverage,
		AltcoinLeverage: altcoinLeverage,
		Account: decision.AccountInfo{
			TotalEquity:      totalEquity,
			AvailableBalance: availableBalance,
			UnrealizedPnL:    totalUnrealizedProfit,
			TotalPnL:         totalPnL,
			TotalPnLPct:      totalPnLPct,
			MarginUsed:       totalMarginUsed,
			MarginUsedPct:    marginUsedPct,
			PositionCount:    len(positionInfos),
		},
		Positions:      positionInfos,
		CandidateCoins: candidateCoins,
	}

	// 7. Add recent closed trades (if store is available)
	if at.store != nil {
		// Get recent 10 closed trades for AI context
		recentTrades, err := at.store.Position().GetRecentTrades(at.id, 10)
		if err != nil {
			logger.Infof("⚠️ [%s] Failed to get recent trades: %v", at.name, err)
		} else {
			logger.Infof("📊 [%s] Found %d recent closed trades for AI context", at.name, len(recentTrades))
			for _, trade := range recentTrades {
				ctx.RecentOrders = append(ctx.RecentOrders, decision.RecentOrder{
					Symbol:       trade.Symbol,
					Side:         trade.Side,
					EntryPrice:   trade.EntryPrice,
					ExitPrice:    trade.ExitPrice,
					RealizedPnL:  trade.RealizedPnL,
					PnLPct:       trade.PnLPct,
					EntryTime:    trade.EntryTime,
					ExitTime:     trade.ExitTime,
					HoldDuration: trade.HoldDuration,
				})
			}
		}
	} else {
		logger.Infof("⚠️ [%s] Store is nil, cannot get recent trades", at.name)
	}

	// 7b. Feed recent execution failures back to the AI so it can correct
	// itself instead of repeating instructions that keep failing.
	if failures := at.recentExecutionFailuresForPrompt(); len(failures) > 0 {
		ctx.RecentFailures = failures
		logger.Infof("🔁 [%s] Feeding back %d recent execution failure(s) to AI", at.name, len(failures))
	}

	// 8. Get quantitative data (if enabled in strategy config)
	if strategyConfig.Indicators.EnableQuantData && strategyConfig.Indicators.QuantDataAPIURL != "" {
		// Collect symbols to query (candidate coins + position coins)
		symbolsToQuery := make(map[string]bool)
		for _, coin := range candidateCoins {
			symbolsToQuery[coin.Symbol] = true
		}
		for _, pos := range positionInfos {
			symbolsToQuery[pos.Symbol] = true
		}

		symbols := make([]string, 0, len(symbolsToQuery))
		for sym := range symbolsToQuery {
			symbols = append(symbols, sym)
		}

		logger.Infof("📊 [%s] Fetching quantitative data for %d symbols...", at.name, len(symbols))
		ctx.QuantDataMap = at.strategyEngine.FetchQuantDataBatch(symbols)
		logger.Infof("📊 [%s] Successfully fetched quantitative data for %d symbols", at.name, len(ctx.QuantDataMap))
	}

	// 9. Get OI ranking data (market-wide position changes)
	if strategyConfig.Indicators.EnableOIRanking {
		logger.Infof("📊 [%s] Fetching OI ranking data...", at.name)
		ctx.OIRankingData = at.strategyEngine.FetchOIRankingData()
		if ctx.OIRankingData != nil {
			logger.Infof("📊 [%s] OI ranking data ready: %d top, %d low positions",
				at.name, len(ctx.OIRankingData.TopPositions), len(ctx.OIRankingData.LowPositions))
		}
	}

	return ctx, nil
}

// executeDecisionWithRecord executes AI decision and records detailed information.
// Every execution path (cycle loop and external calls) funnels through here,
// so failures are captured uniformly and fed back to the next AI cycle.
func (at *AutoTrader) executeDecisionWithRecord(decision *decision.Decision, actionRecord *store.DecisionAction) error {
	if decision.Action == "open_long" || decision.Action == "open_short" {
		if err := at.enforceEntryDecision(decision); err != nil {
			at.rememberExecutionFailure(decision.Symbol, decision.Action, err)
			return err
		}
	}
	err := at.dispatchDecisionWithRecord(decision, actionRecord)
	if err != nil {
		at.rememberExecutionFailure(decision.Symbol, decision.Action, err)
	} else if decision.Action != "hold" && decision.Action != "wait" {
		// A successful execution supersedes earlier failures of the same
		// instruction; only still-ineffective intents are fed back.
		at.forgetExecutionFailureOnSuccess(decision.Symbol, decision.Action)
	}
	return err
}

func (at *AutoTrader) dispatchDecisionWithRecord(decision *decision.Decision, actionRecord *store.DecisionAction) error {
	switch decision.Action {
	case "open_long":
		return at.executeOpenLongWithRecord(decision, actionRecord)
	case "open_short":
		return at.executeOpenShortWithRecord(decision, actionRecord)
	case "close_long":
		return at.executeCloseLongWithRecord(decision, actionRecord)
	case "close_short":
		return at.executeCloseShortWithRecord(decision, actionRecord)
	case "hold", "wait":
		// No execution needed, just record
		return nil
	default:
		return fmt.Errorf("unknown action: %s", decision.Action)
	}
}

func (at *AutoTrader) enforceEntryDecision(d *decision.Decision) error {
	at.riskMutex.Lock()
	paused := time.Now().Before(at.stopUntil)
	until := at.stopUntil
	manualReviewRequired := at.manualReviewRequired
	at.riskMutex.Unlock()
	if manualReviewRequired {
		return fmt.Errorf("❌ [RISK CONTROL] account drawdown halt requires manual review")
	}
	if paused {
		return fmt.Errorf("❌ [RISK CONTROL] new entries paused until %s", until.Format(time.RFC3339))
	}
	if at.config.StrategyConfig == nil {
		return nil
	}
	// AI-supplied confidence, amount, risk, and leverage are intentionally not
	// consulted. The submit path overwrites them with a backend EntryPlan.
	return nil
}

func isBTCETHSymbol(symbol string) bool {
	symbol = strings.ToUpper(symbol)
	return strings.HasPrefix(symbol, "BTC") || strings.HasPrefix(symbol, "ETH")
}

func validateEntryRiskBeforeSubmit(d *decision.Decision, entryPrice, quantity float64, strategyConfig *store.StrategyConfig) error {
	minRiskRewardRatio := 0.0
	if strategyConfig != nil {
		minRiskRewardRatio = strategyConfig.RiskControl.MinRiskRewardRatio
	}
	if err := decision.ValidateEntryRisk(d, entryPrice, quantity, minRiskRewardRatio); err != nil {
		return fmt.Errorf("❌ [RISK CONTROL] %w", err)
	}
	return nil
}

// ExecuteDecision executes a trading decision from external sources.
// This is a public method that can be called by other modules
func (at *AutoTrader) ExecuteDecision(d *decision.Decision) error {
	logger.Infof("[%s] Executing external decision: %s %s", at.name, d.Action, d.Symbol)

	// Create a minimal action record for tracking
	actionRecord := &store.DecisionAction{
		Symbol:   d.Symbol,
		Action:   d.Action,
		Leverage: d.Leverage,
	}

	// Execute the decision
	err := at.executeDecisionWithRecord(d, actionRecord)
	if err != nil {
		logger.Errorf("[%s] External decision execution failed: %v", at.name, err)
		return err
	}

	logger.Infof("[%s] External decision executed successfully: %s %s", at.name, d.Action, d.Symbol)
	return nil
}

func (at *AutoTrader) buildOpenOrderOptions(side string, currentPrice float64) OrderOptions {
	orderType := at.config.OrderType
	if orderType == "" {
		orderType = OrderTypeMarket
	}

	options := OrderOptions{Type: orderType}
	if orderType != OrderTypeLimit {
		return options
	}

	offset := at.config.LimitPriceOffsetPct
	if offset <= 0 {
		offset = 0.05
	}
	offsetRatio := offset / 100
	if side == "long" {
		options.LimitPrice = currentPrice * (1 - offsetRatio)
	} else {
		options.LimitPrice = currentPrice * (1 + offsetRatio)
	}
	logger.Infof("  🧾 Using limit order: side=%s current=%.8f offset=%.4f%% limit=%.8f", side, currentPrice, offset, options.LimitPrice)
	return options
}

func (at *AutoTrader) openLong(symbol string, quantity float64, leverage int, options OrderOptions) (map[string]interface{}, error) {
	if advanced, ok := at.trader.(AdvancedOrderTrader); ok {
		order, err := advanced.OpenLongWithOptions(symbol, quantity, leverage, options)
		at.recordOrderMetrics(err)
		return order, err
	}
	if options.Type == OrderTypeLimit {
		logger.Infof("  ⚠️ %s does not support configurable order type, falling back to market order", at.exchange)
	}
	order, err := at.trader.OpenLong(symbol, quantity, leverage)
	at.recordOrderMetrics(err)
	return order, err
}

func (at *AutoTrader) openShort(symbol string, quantity float64, leverage int, options OrderOptions) (map[string]interface{}, error) {
	if advanced, ok := at.trader.(AdvancedOrderTrader); ok {
		order, err := advanced.OpenShortWithOptions(symbol, quantity, leverage, options)
		at.recordOrderMetrics(err)
		return order, err
	}
	if options.Type == OrderTypeLimit {
		logger.Infof("  ⚠️ %s does not support configurable order type, falling back to market order", at.exchange)
	}
	order, err := at.trader.OpenShort(symbol, quantity, leverage)
	at.recordOrderMetrics(err)
	return order, err
}

// recordOrderMetrics tracks exchange order submission outcomes.
func (at *AutoTrader) recordOrderMetrics(err error) {
	metrics.OrdersTotal.Inc()
	if err != nil {
		metrics.OrderFailures.Inc()
	}
}

// executeOpenLongWithRecord executes open long position and records detailed information
func (at *AutoTrader) executeOpenLongWithRecord(decision *decision.Decision, actionRecord *store.DecisionAction) error {
	at.entryMutex.Lock()
	defer at.entryMutex.Unlock()
	logger.Infof("  📈 Open long: %s", decision.Symbol)

	// ⚠️ Get current positions for multiple checks
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	// [CODE ENFORCED] Check max positions limit
	if err := at.enforceMaxPositions(len(positions)); err != nil {
		return err
	}

	// Check if there's already a position in the same symbol and direction
	for _, pos := range positions {
		if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
			return fmt.Errorf("❌ %s already has long position, close it first", decision.Symbol)
		}
	}
	if pending, err := at.hasPendingEntry(decision.Symbol, "LONG"); err != nil {
		return err
	} else if pending {
		return fmt.Errorf("%s already has a pending long entry transaction", decision.Symbol)
	}
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		at.riskMutex.Lock()
		at.triggerRiskLocked("market data unavailable")
		at.riskMutex.Unlock()
		return fmt.Errorf("❌ [RISK CONTROL] cannot verify market conditions for %s: %w", decision.Symbol, err)
	}

	// Get balance (needed for multiple checks)
	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("failed to get account balance: %w", err)
	}
	// Fetch directly from the exchange immediately before submission. Limit
	// orders use their intended fill price; market orders use the latest price.
	latestPrice, err := at.trader.GetMarketPrice(decision.Symbol)
	if err != nil {
		return fmt.Errorf("failed to refresh market price before opening %s: %w", decision.Symbol, err)
	}
	if latestPrice <= 0 {
		return fmt.Errorf("invalid latest market price for %s: %.8f", decision.Symbol, latestPrice)
	}
	orderOptions := at.buildOpenOrderOptions("long", latestPrice)
	expectedEntryPrice := latestPrice
	if orderOptions.Type == OrderTypeLimit {
		expectedEntryPrice = orderOptions.LimitPrice
	}
	plan, err := at.buildBackendEntryPlan(decision, expectedEntryPrice, positions, balance, marketData)
	if err != nil {
		return fmt.Errorf("❌ [RISK CONTROL] %w", err)
	}
	decision.PositionSizeUSD = plan.PositionSizeUSD
	decision.Leverage = plan.Leverage
	decision.RiskUSD = plan.ActualRiskUSD
	// Keep the execution record consistent with the backend-owned sizing
	// (the AI-supplied leverage was ignored at parse time and stays 0).
	actionRecord.Leverage = plan.Leverage
	if err := at.enforceAccountEntryRisk(positions, balance, plan.PositionSizeUSD, plan.Leverage, marketData); err != nil {
		return err
	}
	quantity := plan.Quantity
	if err := validateEntryRiskBeforeSubmit(decision, expectedEntryPrice, quantity, at.config.StrategyConfig); err != nil {
		return err
	}
	logger.Infof("  🛡 Backend sizing: risk budget %.2f USD, actual risk %.2f USD, notional %.2f USDT, leverage %dx",
		plan.RiskBudgetUSD, plan.ActualRiskUSD, plan.PositionSizeUSD, plan.Leverage)
	actionRecord.Quantity = quantity
	actionRecord.Price = expectedEntryPrice

	// Set margin mode
	if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
		logger.Infof("  ⚠️ Failed to set margin mode: %v", err)
		// Continue execution, doesn't affect trading
	}

	// Cancel protective orders left over from a previous position on this
	// symbol+side (e.g. the un-triggered TP leg after an SL exit, before the
	// reconciliation cycle cleans it up). A stale closePosition order would
	// immediately act on the new position, potentially closing it right away.
	// Reaching this point implies no exchange position exists for this side,
	// so any such order is stale by definition.
	at.cancelPositionOrders(decision.Symbol, "LONG")

	// Open position
	order, err := at.openLong(decision.Symbol, quantity, decision.Leverage, orderOptions)
	if err != nil {
		return err
	}

	// Record order ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	logger.Infof("  ✓ Open order submitted, order ID: %v, quantity: %.4f", order["orderId"], quantity)
	at.persistEntryOrderState(getOrderIDString(order), decision.Symbol, "open_long", quantity,
		numberValue(order["executedQty"]), numberValue(order["avgPrice"]), numberValue(order["commission"]),
		normalizeOrderState(fmt.Sprint(order["status"])), decision.Leverage, decision.StopLoss, decision.TakeProfit, 0, "UNPROTECTED", nil)

	// Record order to database and poll for confirmation
	if !at.recordAndConfirmOrder(order, decision.Symbol, "open_long", quantity, expectedEntryPrice, decision.Leverage, 0) {
		if isLimitOrderResult(order) {
			logger.Infof("  ⏳ Limit order is pending; every observed partial fill will be protected immediately")
			at.monitorPendingEntryOrder(order, decision.Symbol, "open_long", "LONG", quantity, expectedEntryPrice, decision.Leverage, decision.StopLoss, decision.TakeProfit)
			// A resting limit order is an accepted execution, not a failure. The
			// background monitor owns fill recording and protection placement.
			return nil
		}
		if executedQty, avgPrice := numberValue(order["executedQty"]), numberValue(order["avgPrice"]); executedQty > 0 {
			orderID := getOrderIDString(order)
			if avgPrice > 0 {
				at.recordPositionChange(orderID, decision.Symbol, "LONG", "open_long", executedQty, avgPrice, decision.Leverage, 0, numberValue(order["commission"]))
			}
			if err := at.protectOpenedPosition(decision.Symbol, "LONG", executedQty, decision.StopLoss, decision.TakeProfit, orderID); err != nil {
				return err
			}
			return fmt.Errorf("open long order %s only partially filled %.8f", orderID, executedQty)
		}
		return fmt.Errorf("open long order %s was not confirmed filled", getOrderIDString(order))
	}
	if filledQty := numberValue(order["executedQty"]); filledQty > 0 {
		quantity = filledQty
	}

	// Record position opening time
	posKey := decision.Symbol + "_long"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

	return at.protectOpenedPosition(decision.Symbol, "LONG", quantity, decision.StopLoss, decision.TakeProfit, getOrderIDString(order))
}

// executeOpenShortWithRecord executes open short position and records detailed information
func (at *AutoTrader) executeOpenShortWithRecord(decision *decision.Decision, actionRecord *store.DecisionAction) error {
	at.entryMutex.Lock()
	defer at.entryMutex.Unlock()
	logger.Infof("  📉 Open short: %s", decision.Symbol)

	// ⚠️ Get current positions for multiple checks
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	// [CODE ENFORCED] Check max positions limit
	if err := at.enforceMaxPositions(len(positions)); err != nil {
		return err
	}

	// Check if there's already a position in the same symbol and direction
	for _, pos := range positions {
		if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
			return fmt.Errorf("❌ %s already has short position, close it first", decision.Symbol)
		}
	}
	if pending, err := at.hasPendingEntry(decision.Symbol, "SHORT"); err != nil {
		return err
	} else if pending {
		return fmt.Errorf("%s already has a pending short entry transaction", decision.Symbol)
	}
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		at.riskMutex.Lock()
		at.triggerRiskLocked("market data unavailable")
		at.riskMutex.Unlock()
		return fmt.Errorf("❌ [RISK CONTROL] cannot verify market conditions for %s: %w", decision.Symbol, err)
	}

	// Get balance (needed for multiple checks)
	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("failed to get account balance: %w", err)
	}
	// Fetch directly from the exchange immediately before submission. Limit
	// orders use their intended fill price; market orders use the latest price.
	latestPrice, err := at.trader.GetMarketPrice(decision.Symbol)
	if err != nil {
		return fmt.Errorf("failed to refresh market price before opening %s: %w", decision.Symbol, err)
	}
	if latestPrice <= 0 {
		return fmt.Errorf("invalid latest market price for %s: %.8f", decision.Symbol, latestPrice)
	}
	orderOptions := at.buildOpenOrderOptions("short", latestPrice)
	expectedEntryPrice := latestPrice
	if orderOptions.Type == OrderTypeLimit {
		expectedEntryPrice = orderOptions.LimitPrice
	}
	plan, err := at.buildBackendEntryPlan(decision, expectedEntryPrice, positions, balance, marketData)
	if err != nil {
		return fmt.Errorf("❌ [RISK CONTROL] %w", err)
	}
	decision.PositionSizeUSD = plan.PositionSizeUSD
	decision.Leverage = plan.Leverage
	decision.RiskUSD = plan.ActualRiskUSD
	// Keep the execution record consistent with the backend-owned sizing
	// (the AI-supplied leverage was ignored at parse time and stays 0).
	actionRecord.Leverage = plan.Leverage
	if err := at.enforceAccountEntryRisk(positions, balance, plan.PositionSizeUSD, plan.Leverage, marketData); err != nil {
		return err
	}
	quantity := plan.Quantity
	if err := validateEntryRiskBeforeSubmit(decision, expectedEntryPrice, quantity, at.config.StrategyConfig); err != nil {
		return err
	}
	logger.Infof("  🛡 Backend sizing: risk budget %.2f USD, actual risk %.2f USD, notional %.2f USDT, leverage %dx",
		plan.RiskBudgetUSD, plan.ActualRiskUSD, plan.PositionSizeUSD, plan.Leverage)
	actionRecord.Quantity = quantity
	actionRecord.Price = expectedEntryPrice

	// Set margin mode
	if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
		logger.Infof("  ⚠️ Failed to set margin mode: %v", err)
		// Continue execution, doesn't affect trading
	}

	// Cancel protective orders left over from a previous position on this
	// symbol+side before opening (see the long path for rationale).
	at.cancelPositionOrders(decision.Symbol, "SHORT")

	// Open position
	order, err := at.openShort(decision.Symbol, quantity, decision.Leverage, orderOptions)
	if err != nil {
		return err
	}

	// Record order ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	logger.Infof("  ✓ Open order submitted, order ID: %v, quantity: %.4f", order["orderId"], quantity)
	at.persistEntryOrderState(getOrderIDString(order), decision.Symbol, "open_short", quantity,
		numberValue(order["executedQty"]), numberValue(order["avgPrice"]), numberValue(order["commission"]),
		normalizeOrderState(fmt.Sprint(order["status"])), decision.Leverage, decision.StopLoss, decision.TakeProfit, 0, "UNPROTECTED", nil)

	// Record order to database and poll for confirmation
	if !at.recordAndConfirmOrder(order, decision.Symbol, "open_short", quantity, expectedEntryPrice, decision.Leverage, 0) {
		if isLimitOrderResult(order) {
			logger.Infof("  ⏳ Limit order is pending; every observed partial fill will be protected immediately")
			at.monitorPendingEntryOrder(order, decision.Symbol, "open_short", "SHORT", quantity, expectedEntryPrice, decision.Leverage, decision.StopLoss, decision.TakeProfit)
			// A resting limit order is an accepted execution, not a failure. The
			// background monitor owns fill recording and protection placement.
			return nil
		}
		if executedQty, avgPrice := numberValue(order["executedQty"]), numberValue(order["avgPrice"]); executedQty > 0 {
			orderID := getOrderIDString(order)
			if avgPrice > 0 {
				at.recordPositionChange(orderID, decision.Symbol, "SHORT", "open_short", executedQty, avgPrice, decision.Leverage, 0, numberValue(order["commission"]))
			}
			if err := at.protectOpenedPosition(decision.Symbol, "SHORT", executedQty, decision.StopLoss, decision.TakeProfit, orderID); err != nil {
				return err
			}
			return fmt.Errorf("open short order %s only partially filled %.8f", orderID, executedQty)
		}
		return fmt.Errorf("open short order %s was not confirmed filled", getOrderIDString(order))
	}
	if filledQty := numberValue(order["executedQty"]); filledQty > 0 {
		quantity = filledQty
	}

	// Record position opening time
	posKey := decision.Symbol + "_short"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

	return at.protectOpenedPosition(decision.Symbol, "SHORT", quantity, decision.StopLoss, decision.TakeProfit, getOrderIDString(order))
}

// executeCloseLongWithRecord executes close long position and records detailed information
func (at *AutoTrader) executeCloseLongWithRecord(decision *decision.Decision, actionRecord *store.DecisionAction) error {
	logger.Infof("  🔄 Close long: %s", decision.Symbol)

	// Get current price
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// Get entry price and quantity from exchange API (most accurate).
	// Use the same tolerant accountNumber parsing as the opening path so
	// adapters returning string-typed numbers do not silently yield quantity=0
	// (which would misclassify a partial close as fully closed and cancel the
	// remaining position's protective orders).
	var entryPrice float64
	var quantity float64
	positionFound := false
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
				positionFound = true
				entryPrice, _ = accountNumber(pos, "entryPrice", "entry_price")
				if amt, _ := accountNumber(pos, "positionAmt", "size", "quantity"); amt > 0 {
					quantity = amt
				}
				break
			}
		}
	}

	// Cancel resting entry orders for this side BEFORE closing, so a pending
	// limit entry cannot re-open the position right after the close fill (the
	// entry monitor exits cleanly once the exchange reports the order
	// CANCELED).
	pendingCanceled := at.cancelPendingEntries(decision.Symbol, "LONG")

	// Close position
	order, err := at.trader.CloseLong(decision.Symbol, 0) // 0 = close all
	if err != nil {
		// No exchange position left (e.g. SL/TP already closed it): the close
		// intent is satisfied as long as we stopped any pending entry.
		if !positionFound && pendingCanceled > 0 {
			logger.Infof("  ✓ No open long on exchange; canceled %d pending long entry order(s) for %s", pendingCanceled, decision.Symbol)
			at.ClearPeakPnLCache(decision.Symbol, "long")
			return nil
		}
		return err
	}

	// Record order ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	// Record only after the exchange confirms the close fill.
	if !at.recordAndConfirmOrder(order, decision.Symbol, "close_long", quantity, marketData.CurrentPrice, 0, entryPrice) {
		if executedQty, avgPrice := numberValue(order["executedQty"]), numberValue(order["avgPrice"]); executedQty > 0 && avgPrice > 0 {
			at.recordPositionChange(getOrderIDString(order), decision.Symbol, "LONG", "close_long", executedQty, avgPrice, 0, entryPrice, numberValue(order["commission"]))
			return fmt.Errorf("close long order %s only partially filled %.8f", getOrderIDString(order), executedQty)
		}
		return fmt.Errorf("close long order %s was not confirmed filled", getOrderIDString(order))
	}
	if filledQty := numberValue(order["executedQty"]); quantity <= 0 || filledQty+1e-9 >= quantity {
		at.cancelPositionOrders(decision.Symbol, "LONG")
		// Trailing profit-protection state belongs to the closed position.
		at.ClearPeakPnLCache(decision.Symbol, "long")
	} else {
		return fmt.Errorf("close long order %s only filled %.8f of %.8f", getOrderIDString(order), filledQty, quantity)
	}

	logger.Infof("  ✓ Position closed successfully")
	return nil
}

// executeCloseShortWithRecord executes close short position and records detailed information
func (at *AutoTrader) executeCloseShortWithRecord(decision *decision.Decision, actionRecord *store.DecisionAction) error {
	logger.Infof("  🔄 Close short: %s", decision.Symbol)

	// Get current price
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// Get entry price and quantity from exchange API (most accurate).
	// Same tolerant parsing as the close-long path; positionAmt is negative for
	// shorts, so normalize with math.Abs.
	var entryPrice float64
	var quantity float64
	positionFound := false
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
				positionFound = true
				entryPrice, _ = accountNumber(pos, "entryPrice", "entry_price")
				if amt, _ := accountNumber(pos, "positionAmt", "size", "quantity"); amt != 0 {
					quantity = math.Abs(amt)
				}
				break
			}
		}
	}

	// Cancel resting entry orders for this side BEFORE closing (see the long
	// close path for rationale).
	pendingCanceled := at.cancelPendingEntries(decision.Symbol, "SHORT")

	// Close position
	order, err := at.trader.CloseShort(decision.Symbol, 0) // 0 = close all
	if err != nil {
		// No exchange position left (e.g. SL/TP already closed it): the close
		// intent is satisfied as long as we stopped any pending entry.
		if !positionFound && pendingCanceled > 0 {
			logger.Infof("  ✓ No open short on exchange; canceled %d pending short entry order(s) for %s", pendingCanceled, decision.Symbol)
			return nil
		}
		return err
	}

	// Record order ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	// Record only after the exchange confirms the close fill.
	if !at.recordAndConfirmOrder(order, decision.Symbol, "close_short", quantity, marketData.CurrentPrice, 0, entryPrice) {
		if executedQty, avgPrice := numberValue(order["executedQty"]), numberValue(order["avgPrice"]); executedQty > 0 && avgPrice > 0 {
			at.recordPositionChange(getOrderIDString(order), decision.Symbol, "SHORT", "close_short", executedQty, avgPrice, 0, entryPrice, numberValue(order["commission"]))
			return fmt.Errorf("close short order %s only partially filled %.8f", getOrderIDString(order), executedQty)
		}
		return fmt.Errorf("close short order %s was not confirmed filled", getOrderIDString(order))
	}
	if filledQty := numberValue(order["executedQty"]); quantity <= 0 || filledQty+1e-9 >= quantity {
		at.cancelPositionOrders(decision.Symbol, "SHORT")
		// Trailing profit-protection state belongs to the closed position.
		at.ClearPeakPnLCache(decision.Symbol, "short")
	} else {
		return fmt.Errorf("close short order %s only filled %.8f of %.8f", getOrderIDString(order), filledQty, quantity)
	}

	logger.Infof("  ✓ Position closed successfully")
	return nil
}

// GetID gets trader ID
func (at *AutoTrader) GetID() string {
	return at.id
}

// GetName gets trader name
func (at *AutoTrader) GetName() string {
	return at.name
}

// GetAIModel gets AI model
func (at *AutoTrader) GetAIModel() string {
	return at.aiModel
}

// GetExchange gets exchange
func (at *AutoTrader) GetExchange() string {
	return at.exchange
}

// GetShowInCompetition returns whether trader should be shown in competition
func (at *AutoTrader) GetShowInCompetition() bool {
	return at.showInCompetition
}

// SetShowInCompetition sets whether trader should be shown in competition
func (at *AutoTrader) SetShowInCompetition(show bool) {
	at.showInCompetition = show
}

// SetCustomPrompt sets custom trading strategy prompt
func (at *AutoTrader) SetCustomPrompt(prompt string) {
	at.customPrompt = prompt
}

// SetOverrideBasePrompt sets whether to override base prompt
func (at *AutoTrader) SetOverrideBasePrompt(override bool) {
	at.overrideBasePrompt = override
}

// GetSystemPromptTemplate gets current system prompt template name (from strategy config)
func (at *AutoTrader) GetSystemPromptTemplate() string {
	if at.strategyEngine != nil {
		config := at.strategyEngine.GetConfig()
		if config.CustomPrompt != "" {
			return "custom"
		}
	}
	return "strategy"
}

// saveEquitySnapshot saves equity snapshot independently (for drawing profit curve, decoupled from AI decision)
func (at *AutoTrader) saveEquitySnapshot(ctx *decision.Context) {
	if at.store == nil || ctx == nil {
		return
	}

	snapshot := &store.EquitySnapshot{
		TraderID:      at.id,
		Timestamp:     time.Now().UTC(),
		TotalEquity:   ctx.Account.TotalEquity,
		Balance:       ctx.Account.TotalEquity - ctx.Account.UnrealizedPnL,
		UnrealizedPnL: ctx.Account.UnrealizedPnL,
		PositionCount: ctx.Account.PositionCount,
		MarginUsedPct: ctx.Account.MarginUsedPct,
	}

	if err := at.store.Equity().Save(snapshot); err != nil {
		logger.Infof("⚠️ Failed to save equity snapshot: %v", err)
	}
}

// saveDecision saves AI decision log to database (only records AI input/output, for debugging)
func (at *AutoTrader) saveDecision(record *store.DecisionRecord) error {
	if at.store == nil {
		return nil
	}

	at.cycleNumber++
	record.CycleNumber = at.cycleNumber
	record.TraderID = at.id

	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	}

	if err := at.store.Decision().LogDecision(record); err != nil {
		logger.Infof("⚠️ Failed to save decision record: %v", err)
		return err
	}

	logger.Infof("📝 Decision record saved: trader=%s, cycle=%d", at.id, at.cycleNumber)
	return nil
}

// GetStore gets data store (for external access to decision records, etc.)
func (at *AutoTrader) GetStore() *store.Store {
	return at.store
}

// GetStatus gets system status (for API)
func (at *AutoTrader) GetStatus() map[string]interface{} {
	at.lifecycleMutex.Lock()
	isRunning := at.isRunning
	startTime := at.startTime
	at.lifecycleMutex.Unlock()
	at.riskMutex.Lock()
	stopUntil := at.stopUntil
	lastResetTime := at.lastResetTime
	at.riskMutex.Unlock()
	callCount := at.callCount.Load()

	aiProvider := "DeepSeek"
	if at.config.UseQwen {
		aiProvider = "Qwen"
	}

	return map[string]interface{}{
		"trader_id":       at.id,
		"trader_name":     at.name,
		"ai_model":        at.aiModel,
		"exchange":        at.exchange,
		"is_running":      isRunning,
		"start_time":      startTime.Format(time.RFC3339),
		"runtime_minutes": int(time.Since(startTime).Minutes()),
		"call_count":      callCount,
		"initial_balance": at.initialBalance,
		"scan_interval":   at.config.ScanInterval.String(),
		"stop_until":      stopUntil.Format(time.RFC3339),
		"last_reset_time": lastResetTime.Format(time.RFC3339),
		"ai_provider":     aiProvider,
	}
}

func accountNumber(values map[string]interface{}, keys ...string) (float64, bool) {
	for _, key := range keys {
		value, exists := values[key]
		if !exists || value == nil {
			continue
		}

		switch number := value.(type) {
		case float64:
			if !math.IsNaN(number) && !math.IsInf(number, 0) {
				return number, true
			}
		case float32:
			parsed := float64(number)
			if !math.IsNaN(parsed) && !math.IsInf(parsed, 0) {
				return parsed, true
			}
		case int:
			return float64(number), true
		case int64:
			return float64(number), true
		case string:
			parsed, err := strconv.ParseFloat(number, 64)
			if err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0) {
				return parsed, true
			}
		}
	}
	return 0, false
}

// GetAccountInfo gets account information (for API)
func (at *AutoTrader) GetAccountInfo() (map[string]interface{}, error) {
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	// Prefer the exchange's account-level equity. Reconstruct it only for
	// exchanges that expose wallet balance and unrealized PnL separately.
	totalUnrealizedProfit, _ := accountNumber(balance,
		"totalUnrealizedProfit", "unrealized_profit", "unrealized_pnl")
	totalEquity, hasTotalEquity := accountNumber(balance,
		"total_equity", "totalEquity", "totalEq", "accountEquity")
	totalWalletBalance, hasWalletBalance := accountNumber(balance,
		"totalWalletBalance", "wallet_balance", "walletBalance")
	if !hasTotalEquity {
		if !hasWalletBalance {
			return nil, fmt.Errorf("balance response does not contain total equity or wallet balance")
		}
		totalEquity = totalWalletBalance + totalUnrealizedProfit
	}
	if !hasWalletBalance {
		totalWalletBalance = totalEquity - totalUnrealizedProfit
	}
	availableBalance, _ := accountNumber(balance,
		"availableBalance", "available_balance", "available", "availBal")

	// Get positions to calculate total margin
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	totalMarginUsed, hasAccountMargin := accountNumber(balance, "margin_used", "marginUsed")
	totalUnrealizedPnLCalculated := 0.0
	for _, pos := range positions {
		markPrice, hasMarkPrice := accountNumber(pos, "markPrice", "mark_price")
		quantity, hasQuantity := accountNumber(pos, "positionAmt", "size", "quantity")
		if quantity < 0 {
			quantity = -quantity
		}
		if unrealizedPnl, ok := accountNumber(pos, "unRealizedProfit", "unrealized_pnl", "unrealizedPnl"); ok {
			totalUnrealizedPnLCalculated += unrealizedPnl
		}

		if hasAccountMargin {
			continue
		}
		if positionMargin, ok := accountNumber(pos, "margin_used", "marginUsed", "positionInitialMargin"); ok {
			totalMarginUsed += positionMargin
			continue
		}
		leverage, hasLeverage := accountNumber(pos, "leverage")
		if hasMarkPrice && hasQuantity && hasLeverage && leverage > 0 {
			totalMarginUsed += (quantity * markPrice) / leverage
		}
	}

	// Verify unrealized P&L consistency (API value vs calculated from positions)
	diff := math.Abs(totalUnrealizedProfit - totalUnrealizedPnLCalculated)
	if diff > 0.1 { // Allow 0.01 USDT error margin
		logger.Infof("⚠️ Unrealized P&L inconsistency: API=%.4f, Calculated=%.4f, Diff=%.4f",
			totalUnrealizedProfit, totalUnrealizedPnLCalculated, diff)
	}

	totalPnL := totalEquity - at.initialBalance
	totalPnLPct := 0.0
	if at.initialBalance > 0 {
		totalPnLPct = (totalPnL / at.initialBalance) * 100
	} else {
		logger.Infof("⚠️ Initial Balance abnormal: %.2f, cannot calculate P&L percentage", at.initialBalance)
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	at.riskMutex.Lock()
	dailyPnL := at.dailyPnL
	at.riskMutex.Unlock()
	return map[string]interface{}{
		// Core fields
		"total_equity":      totalEquity,           // Account equity = wallet + unrealized
		"wallet_balance":    totalWalletBalance,    // Wallet balance (excluding unrealized P&L)
		"unrealized_profit": totalUnrealizedProfit, // Unrealized P&L (official value from exchange API)
		"available_balance": availableBalance,      // Available balance

		// P&L statistics
		"total_pnl":       totalPnL,          // Total P&L = equity - initial
		"total_pnl_pct":   totalPnLPct,       // Total P&L percentage
		"initial_balance": at.initialBalance, // Initial balance
		"daily_pnl":       dailyPnL,          // Daily P&L

		// Position information
		"position_count":  len(positions),  // Position count
		"margin_used":     totalMarginUsed, // Margin used
		"margin_used_pct": marginUsedPct,   // Margin usage rate
	}, nil
}

// GetPositions gets position list (for API)
func (at *AutoTrader) GetPositions() ([]map[string]interface{}, error) {
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		// Defensive parsing consistent with buildTradingContext: skip malformed
		// positions instead of panicking on missing or mistyped fields.
		symbol, _ := pos["symbol"].(string)
		symbol = strings.ToUpper(strings.TrimSpace(symbol))
		side, _ := pos["side"].(string)
		side = strings.ToLower(strings.TrimSpace(side))
		entryPrice, _ := accountNumber(pos, "entryPrice", "entry_price")
		markPrice, _ := accountNumber(pos, "markPrice", "mark_price")
		quantity, _ := accountNumber(pos, "positionAmt", "size", "quantity")
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl, _ := accountNumber(pos, "unRealizedProfit", "unrealized_pnl", "unrealizedPnl")
		liquidationPrice, _ := accountNumber(pos, "liquidationPrice", "liquidation_price")
		if symbol == "" || (side != "long" && side != "short") || entryPrice <= 0 || markPrice <= 0 {
			logger.Warnf("[%s] skipping malformed exchange position in API view: symbol=%q side=%q entry=%.8f mark=%.8f", at.name, symbol, side, entryPrice, markPrice)
			continue
		}

		leverage := 10
		if lev, ok := accountNumber(pos, "leverage"); ok && lev > 0 {
			leverage = int(lev)
		}

		// Calculate margin used
		marginUsed := (quantity * markPrice) / float64(leverage)

		// Calculate P&L percentage (based on margin)
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)

		result = append(result, map[string]interface{}{
			"symbol":             symbol,
			"side":               side,
			"entry_price":        entryPrice,
			"mark_price":         markPrice,
			"quantity":           quantity,
			"leverage":           leverage,
			"unrealized_pnl":     unrealizedPnl,
			"unrealized_pnl_pct": pnlPct,
			"liquidation_price":  liquidationPrice,
			"margin_used":        marginUsed,
		})
	}

	return result, nil
}

// calculatePnLPercentage calculates P&L percentage (based on margin, automatically considers leverage)
// Return rate = Unrealized P&L / Margin × 100%
func calculatePnLPercentage(unrealizedPnl, marginUsed float64) float64 {
	if marginUsed > 0 {
		return (unrealizedPnl / marginUsed) * 100
	}
	return 0.0
}

// sortDecisionsByPriority sorts decisions: close positions first, then open positions, finally hold/wait
// This avoids position stacking overflow when changing positions
func sortDecisionsByPriority(decisions []decision.Decision) []decision.Decision {
	if len(decisions) <= 1 {
		return decisions
	}

	// Define priority
	getActionPriority := func(action string) int {
		switch action {
		case "close_long", "close_short":
			return 1 // Highest priority: close positions first
		case "open_long", "open_short":
			return 2 // Second priority: open positions later
		case "hold", "wait":
			return 3 // Lowest priority: wait
		default:
			return 999 // Unknown actions at the end
		}
	}

	// Copy decision list
	sorted := make([]decision.Decision, len(decisions))
	copy(sorted, decisions)

	// Sort by priority
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if getActionPriority(sorted[i].Action) > getActionPriority(sorted[j].Action) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// startDrawdownMonitor starts drawdown monitoring
func (at *AutoTrader) startDrawdownMonitor(stopCh <-chan struct{}) {
	at.monitorWg.Add(1)
	go func() {
		defer at.monitorWg.Done()

		ticker := time.NewTicker(1 * time.Minute) // Check every minute
		defer ticker.Stop()

		logger.Info("📊 Started position drawdown monitoring (check every minute)")

		for {
			select {
			case <-ticker.C:
				at.checkPositionDrawdown()
			case <-stopCh:
				logger.Info("⏹ Stopped position drawdown monitoring")
				return
			}
		}
	}()
}

// checkPositionDrawdown checks position drawdown situation
func (at *AutoTrader) checkPositionDrawdown() {
	// The trailing profit-protection policy is strategy-owned. Older
	// configs (or a nil strategy config) keep the historical always-on
	// behavior via the defaults in RiskControlConfig.
	var risk store.RiskControlConfig
	if at.config.StrategyConfig != nil {
		risk = at.config.StrategyConfig.RiskControl
	}
	triggerPct, givebackPct := risk.TrailingProfitExitLevels()
	if !risk.TrailingProfitExitEnabled() {
		return
	}

	// Get current positions
	positions, err := at.trader.GetPositions()
	if err != nil {
		logger.Infof("❌ Drawdown monitoring: failed to get positions: %v", err)
		return
	}

	for _, pos := range positions {
		// Defensive parsing: an adapter that returns a malformed or differently
		// typed position must never panic this monitor goroutine (a panic here
		// would kill the whole process).
		symbol, _ := pos["symbol"].(string)
		symbol = strings.ToUpper(strings.TrimSpace(symbol))
		side, _ := pos["side"].(string)
		side = strings.ToLower(strings.TrimSpace(side))
		entryPrice, _ := accountNumber(pos, "entryPrice", "entry_price")
		markPrice, _ := accountNumber(pos, "markPrice", "mark_price")
		quantity, _ := accountNumber(pos, "positionAmt", "size", "quantity")
		if quantity < 0 {
			quantity = -quantity // Short position quantity is negative, convert to positive
		}
		if symbol == "" || (side != "long" && side != "short") || entryPrice <= 0 || markPrice <= 0 {
			logger.Warnf("[%s] drawdown monitor skipping malformed exchange position: symbol=%q side=%q entry=%.8f mark=%.8f", at.name, symbol, side, entryPrice, markPrice)
			continue
		}

		// Calculate current P&L percentage. A missing leverage field must
		// default to 1x: defaulting to a higher value would inflate PnL% and
		// could trigger an unjustified emergency close.
		leverage := 1
		if lev, ok := accountNumber(pos, "leverage"); ok && lev > 0 {
			leverage = int(lev)
		}

		var currentPnLPct float64
		if side == "long" {
			currentPnLPct = ((markPrice - entryPrice) / entryPrice) * float64(leverage) * 100
		} else {
			currentPnLPct = ((entryPrice - markPrice) / entryPrice) * float64(leverage) * 100
		}

		// Construct unique position identifier (distinguish long/short)
		posKey := peakCacheKey(symbol, side)

		// Get historical peak profit for this position
		at.peakPnLCacheMutex.RLock()
		peakEntry, exists := at.peakPnLCache[posKey]
		at.peakPnLCacheMutex.RUnlock()

		peakPnLPct := currentPnLPct
		if exists {
			peakPnLPct = peakEntry.Pct
		}
		// Update peak cache (persists when a new peak is recorded)
		at.UpdatePeakPnL(symbol, side, currentPnLPct)

		// Calculate drawdown (magnitude of decline from peak)
		var drawdownPct float64
		if peakPnLPct > 0 && currentPnLPct < peakPnLPct {
			drawdownPct = ((peakPnLPct - currentPnLPct) / peakPnLPct) * 100
		}

		// Check close position condition: profit above the trigger threshold
		// and retraced by the configured giveback fraction from its peak.
		if currentPnLPct > triggerPct && drawdownPct >= givebackPct {
			logger.Infof("🚨 Drawdown close position condition triggered: %s %s | Current profit: %.2f%% | Peak profit: %.2f%% | Drawdown: %.2f%%",
				symbol, side, currentPnLPct, peakPnLPct, drawdownPct)

			// Execute close position
			if err := at.emergencyClosePosition(symbol, side); err != nil {
				logger.Infof("❌ Drawdown close position failed (%s %s): %v", symbol, side, err)
			} else {
				logger.Infof("✅ Drawdown close position succeeded: %s %s", symbol, side)
				// Clear cache for this position after closing
				at.ClearPeakPnLCache(symbol, side)
			}
		} else if currentPnLPct > triggerPct {
			// Record situations close to close position condition (for debugging)
			logger.Infof("📊 Drawdown monitoring: %s %s | Profit: %.2f%% | Peak: %.2f%% | Drawdown: %.2f%%",
				symbol, side, currentPnLPct, peakPnLPct, drawdownPct)
		}
	}
}

// emergencyClosePosition emergency close position function
func (at *AutoTrader) emergencyClosePosition(symbol, side string) error {
	var order map[string]interface{}
	var err error
	var action string
	switch side {
	case "long":
		order, err = at.trader.CloseLong(symbol, 0) // 0 = close all
		action = "close_long"
	case "short":
		order, err = at.trader.CloseShort(symbol, 0) // 0 = close all
		action = "close_short"
	default:
		return fmt.Errorf("unknown position direction: %s", side)
	}
	if err != nil {
		at.logBackendForcedAction(symbol, action, "", err)
		return err
	}
	if !at.recordAndConfirmOrder(order, symbol, action, 0, 0, 0, 0) {
		confirmErr := fmt.Errorf("emergency close order %s was not confirmed filled", getOrderIDString(order))
		at.logBackendForcedAction(symbol, action, getOrderIDString(order), confirmErr)
		return confirmErr
	}
	at.cancelPositionOrders(symbol, strings.ToUpper(side))
	logger.Infof("✅ Emergency close %s position confirmed, order ID: %v", side, order["orderId"])
	at.logBackendForcedAction(symbol, action, getOrderIDString(order), nil)

	return nil
}

// logBackendForcedAction persists a decision record for backend-initiated
// position actions so the audit trail explains WHY a position was closed
// outside the AI decision cycle.
func (at *AutoTrader) logBackendForcedAction(symbol, action, orderID string, actionErr error) {
	if at.store == nil {
		return
	}
	var risk store.RiskControlConfig
	if at.config.StrategyConfig != nil {
		risk = at.config.StrategyConfig.RiskControl
	}
	trigger, giveback := risk.TrailingProfitExitLevels()

	reason := fmt.Sprintf("backend trailing profit protection: profit retraced %.0f%%+ from peak (trigger %.1f%%, giveback %.0f%%)", giveback, trigger, giveback)
	if actionErr != nil {
		reason = fmt.Sprintf("backend trailing profit protection attempted close but failed: %v", actionErr)
	}

	record := &store.DecisionRecord{
		Timestamp: time.Now().UTC(),
		ExecutionLog: []string{
			fmt.Sprintf("🚨 %s %s: %s", symbol, action, reason),
		},
		Success:   actionErr == nil,
		Decisions: []store.DecisionAction{{Action: action, Symbol: symbol, Timestamp: time.Now(), Success: actionErr == nil, OrderID: 0, Error: errString(actionErr)}},
	}
	// DecisionAction has no free-text field; keep the reason in ExecutionLog
	// and the decision log's error column.
	if actionErr != nil {
		record.ErrorMessage = reason
	}
	if err := at.saveDecision(record); err != nil {
		logger.Warnf("⚠️ [%s] failed to save backend-forced action record: %v", at.name, err)
	}

	// Trading notification: trailing-profit protection acts on the user's
	// position without AI involvement, so it must always be surfaced.
	severity := notification.SeverityWarning
	if actionErr != nil {
		severity = notification.SeverityCritical
	}
	at.publishPositionEvent(notification.EventTakeProfitHit, severity, symbol,
		fmt.Sprintf("🛡 Trailing profit protection: %s", symbol), reason)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// peakPnLEntry records a position's peak unrealized PnL together with the
// moment it was observed. The timestamp lets the drawdown monitor detect
// peaks recorded for a *previous* position on the same symbol+side (the
// position was closed and reopened after the peak was stored).
type peakPnLEntry struct {
	Pct       float64
	UpdatedAt time.Time
}

// peakCacheKey builds the cache key for a position. Side is normalized to
// lowercase to match exchange position payloads.
func peakCacheKey(symbol, side string) string {
	return strings.ToUpper(strings.TrimSpace(symbol)) + "_" + strings.ToLower(strings.TrimSpace(side))
}

// GetPeakPnLCache gets peak profit cache
func (at *AutoTrader) GetPeakPnLCache() map[string]float64 {
	at.peakPnLCacheMutex.RLock()
	defer at.peakPnLCacheMutex.RUnlock()

	// Return a copy of the cache
	cache := make(map[string]float64)
	for k, v := range at.peakPnLCache {
		cache[k] = v.Pct
	}
	return cache
}

// UpdatePeakPnL updates peak profit cache and persists new peaks so the
// trailing profit protection survives restarts.
func (at *AutoTrader) UpdatePeakPnL(symbol, side string, currentPnLPct float64) {
	posKey := peakCacheKey(symbol, side)

	at.peakPnLCacheMutex.Lock()
	peak, exists := at.peakPnLCache[posKey]
	if exists && currentPnLPct <= peak.Pct {
		at.peakPnLCacheMutex.Unlock()
		return
	}
	at.peakPnLCache[posKey] = peakPnLEntry{Pct: currentPnLPct, UpdatedAt: time.Now()}
	at.peakPnLCacheMutex.Unlock()

	// Persist only when a new peak was recorded (avoids a DB write per tick).
	if at.store != nil {
		if err := at.store.Position().UpsertPeakPnL(at.id, strings.ToUpper(strings.TrimSpace(symbol)), strings.ToLower(strings.TrimSpace(side)), currentPnLPct); err != nil {
			logger.Warnf("⚠️ [%s] failed to persist peak PnL for %s: %v", at.name, posKey, err)
		}
	}
}

// ClearPeakPnLCache clears peak cache for specified position
func (at *AutoTrader) ClearPeakPnLCache(symbol, side string) {
	posKey := peakCacheKey(symbol, side)

	at.peakPnLCacheMutex.Lock()
	delete(at.peakPnLCache, posKey)
	at.peakPnLCacheMutex.Unlock()

	if at.store != nil {
		if err := at.store.Position().DeletePeakPnL(at.id, strings.ToUpper(strings.TrimSpace(symbol)), strings.ToLower(strings.TrimSpace(side))); err != nil {
			logger.Warnf("⚠️ [%s] failed to delete persisted peak PnL for %s: %v", at.name, posKey, err)
		}
	}
}

// loadPeakPnLs restores persisted peak PnL state after a restart. Rows for
// positions that no longer exist on the exchange (closed while the trader
// was down, e.g. by exchange-side stop-loss) are removed.
func (at *AutoTrader) loadPeakPnLs() {
	if at.store == nil {
		return
	}
	peaks, err := at.store.Position().GetPeakPnLs(at.id)
	if err != nil {
		logger.Warnf("⚠️ [%s] failed to load peak PnL state: %v", at.name, err)
		return
	}
	if len(peaks) == 0 {
		return
	}

	live := make(map[string]bool)
	positions, posErr := at.trader.GetPositions()
	if posErr != nil {
		// Without a position snapshot, keep persisted peaks as-is: a stale
		// entry is guarded by the monitor's first-seen timestamp check.
		logger.Warnf("⚠️ [%s] could not verify open positions while loading peaks: %v", at.name, posErr)
	} else {
		for _, pos := range positions {
			symbol, _ := pos["symbol"].(string)
			side, _ := pos["side"].(string)
			if symbol != "" && side != "" {
				live[peakCacheKey(symbol, side)] = true
			}
		}
	}

	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()
	for key, pct := range peaks {
		if len(live) > 0 && !live[key] {
			if idx := strings.LastIndex(key, "_"); idx > 0 {
				_ = at.store.Position().DeletePeakPnL(at.id, key[:idx], key[idx+1:])
			}
			continue
		}
		at.peakPnLCache[key] = peakPnLEntry{Pct: pct, UpdatedAt: time.Now()}
		// Seed first-seen so the next buildTradingContext cycle does not
		// treat this live position as brand-new and wipe the restored peak.
		if _, exists := at.positionFirstSeenTime[key]; !exists {
			at.positionFirstSeenTime[key] = time.Now().UnixMilli()
		}
	}
	logger.Infof("📊 [%s] restored peak PnL state for %d position(s)", at.name, len(at.peakPnLCache))
}

func isLimitOrderResult(orderResult map[string]interface{}) bool {
	orderType, ok := orderResult["orderType"].(string)
	return ok && orderType == string(OrderTypeLimit)
}

func getOrderIDString(orderResult map[string]interface{}) string {
	switch v := orderResult["orderId"].(type) {
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%.0f", v)
	case string:
		return v
	case json.Number:
		return v.String()
	default:
		// Missing or unexpected orderId types must yield "" so callers treat
		// the result as unrecordable instead of persisting junk IDs like "<nil>".
		return ""
	}
}

func normalizeOrderState(status string) OrderState {
	switch strings.ToUpper(status) {
	case "FILLED":
		return OrderStateFilled
	case "PARTIALLY_FILLED", "PARTIAL":
		return OrderStatePartial
	case "CANCELED", "CANCELLED", "EXPIRED":
		return OrderStateCanceled
	case "REJECTED", "FAILED":
		return OrderStateRejected
	case "NEW", "OPEN", "LIVE", "PENDING", "SUBMITTED":
		return OrderStateSubmitted
	default:
		return OrderStateSubmitted
	}
}

func (at *AutoTrader) updateProtectionStatus(entryOrderID, status string) {
	if at.store == nil || entryOrderID == "" || entryOrderID == "0" {
		return
	}
	if err := at.store.Position().UpdateProtectionStatus(at.id, entryOrderID, status); err != nil {
		logger.Errorf("[%s] failed to persist protection status %s for order %s: %v", at.name, status, entryOrderID, err)
	}
}

func orderPositionSide(action string) string {
	if strings.HasSuffix(action, "long") {
		return "LONG"
	}
	if strings.HasSuffix(action, "short") {
		return "SHORT"
	}
	return ""
}

func (at *AutoTrader) hasPendingEntry(symbol, positionSide string) (bool, error) {
	if at.store == nil {
		return false, nil
	}
	pending, err := at.store.Execution().HasActiveEntry(at.id, at.exchangeID, symbol, positionSide)
	if err != nil {
		return false, fmt.Errorf("failed to check pending entry orders: %w", err)
	}
	return pending, nil
}

func (at *AutoTrader) persistOrderState(orderID, symbol, action string, requestedQty, executedQty, avgPrice, fee float64, state OrderState, lastErr error) {
	if at.store == nil {
		return
	}
	errText := ""
	if lastErr != nil {
		errText = lastErr.Error()
	}
	order := store.TradeOrder{
		TraderID: at.id, ExchangeID: at.exchangeID, ExchangeType: at.exchange,
		OrderID: orderID, Symbol: symbol, PositionSide: orderPositionSide(action), Action: action,
		RequestedQty: requestedQty, ExecutedQty: executedQty, AvgPrice: avgPrice, Fee: fee,
		Status: string(state), LastError: errText,
	}
	if err := at.store.Execution().UpsertOrder(order); err != nil {
		logger.Errorf("[%s] failed to persist order %s state %s: %v", at.name, orderID, state, err)
		return
	}
	if executedQty > 0 && avgPrice > 0 {
		if err := at.store.Execution().RecordFill(order); err != nil {
			logger.Errorf("[%s] failed to persist fill for order %s: %v", at.name, orderID, err)
		}
	}
}

func (at *AutoTrader) persistEntryOrderState(orderID, symbol, action string, requestedQty, executedQty, avgPrice, fee float64, state OrderState, leverage int, stopLoss, takeProfit, protectedQty float64, protectionStatus string, lastErr error) {
	if at.store == nil {
		return
	}
	errText := ""
	if lastErr != nil {
		errText = lastErr.Error()
	}
	order := store.TradeOrder{
		TraderID: at.id, ExchangeID: at.exchangeID, ExchangeType: at.exchange,
		OrderID: orderID, Symbol: symbol, PositionSide: orderPositionSide(action), Action: action,
		RequestedQty: requestedQty, ExecutedQty: executedQty, AvgPrice: avgPrice, Fee: fee,
		Status: string(state), LastError: errText, Leverage: leverage, StopLoss: stopLoss,
		TakeProfit: takeProfit, ProtectedQty: protectedQty, ProtectionStatus: protectionStatus,
	}
	if err := at.store.Execution().UpsertOrder(order); err != nil {
		logger.Errorf("[%s] failed to persist entry order %s state %s: %v", at.name, orderID, state, err)
		return
	}
	if executedQty > 0 && avgPrice > 0 {
		if err := at.store.Execution().RecordFill(order); err != nil {
			logger.Errorf("[%s] failed to persist fill for entry order %s: %v", at.name, orderID, err)
		}
	}
}

func (at *AutoTrader) updateOrderProtection(entryOrderID string, protectedQty float64, status string) {
	if at.store == nil || entryOrderID == "" || entryOrderID == "0" {
		return
	}
	if err := at.store.Execution().UpdateOrderProtection(at.id, at.exchangeID, entryOrderID, protectedQty, status); err != nil {
		logger.Errorf("[%s] failed to update order %s protection state: %v", at.name, entryOrderID, err)
	}
}

func (at *AutoTrader) persistProtection(entryOrderID, symbol, positionSide, kind string, quantity, triggerPrice float64, status string, attempt int, lastErr error) {
	if at.store == nil {
		return
	}
	errText := ""
	if lastErr != nil {
		errText = lastErr.Error()
	}
	if err := at.store.Execution().UpsertProtection(at.id, at.exchangeID, entryOrderID, symbol, positionSide, kind, quantity, triggerPrice, status, attempt, errText); err != nil {
		logger.Errorf("[%s] failed to persist %s protection for order %s: %v", at.name, kind, entryOrderID, err)
	}
}

func (at *AutoTrader) cancelPositionOrders(symbol, positionSide string) {
	canceler, ok := at.trader.(PositionOrderCanceler)
	if !ok {
		return
	}
	if err := canceler.CancelPositionOrders(symbol, positionSide); err != nil {
		logger.Errorf("[%s] failed to cancel %s %s owned orders: %v", at.name, symbol, positionSide, err)
	}
}

func (at *AutoTrader) cancelPendingOrder(symbol, orderID string) bool {
	canceler, ok := at.trader.(SingleOrderCanceler)
	if !ok {
		logger.Errorf("[%s] exchange %s cannot cancel timed-out order %s individually", at.name, at.exchange, orderID)
		return false
	}
	if err := canceler.CancelOrder(symbol, orderID); err != nil {
		logger.Errorf("[%s] failed to cancel pending order %s: %v", at.name, orderID, err)
		return false
	}
	return true
}

// cancelPendingEntries cancels resting local entry orders for one symbol and
// position side. It is used by the close paths so a pending limit entry
// cannot silently re-open a position that was just closed (the entry
// monitor exits cleanly once the exchange reports the order CANCELED).
// It returns the number of orders whose cancellation was accepted.
func (at *AutoTrader) cancelPendingEntries(symbol, positionSide string) int {
	if at.store == nil {
		return 0
	}
	action := "open_" + strings.ToLower(positionSide)
	orders, err := at.store.Execution().ListActiveOrders(at.id, at.exchangeID)
	if err != nil {
		logger.Errorf("[%s] failed to list pending entry orders for %s %s: %v", at.name, symbol, positionSide, err)
		return 0
	}
	canceled := 0
	for _, order := range orders {
		if order.Symbol != symbol || order.Action != action || order.OrderID == "" {
			continue
		}
		if at.cancelPendingOrder(order.Symbol, order.OrderID) {
			canceled++
		}
	}
	return canceled
}

func (at *AutoTrader) resumeEntryOrderSagas() {
	if at.store == nil {
		return
	}
	orders, err := at.store.Execution().ListRecoverableEntryOrders(at.id, at.exchangeID)
	if err != nil {
		logger.Errorf("[%s] failed to load recoverable entry orders: %v", at.name, err)
		return
	}
	for _, order := range orders {
		if order.StopLoss <= 0 || order.TakeProfit <= 0 {
			logger.Errorf("[%s] recoverable order %s has incomplete protection intent; closing any fill", at.name, order.OrderID)
			at.cancelPendingOrder(order.Symbol, order.OrderID)
			if status, statusErr := at.trader.GetOrderStatus(order.Symbol, order.OrderID); statusErr == nil {
				order.Status = string(normalizeOrderState(fmt.Sprint(status["status"])))
				order.ExecutedQty = numberValue(status["executedQty"])
				order.AvgPrice = numberValue(status["avgPrice"])
				order.Fee = numberValue(status["commission"])
			}
			if order.ExecutedQty > order.ProtectedQty {
				_ = at.protectPositionIncrement(order.Symbol, order.PositionSide, order.ExecutedQty-order.ProtectedQty, order.ExecutedQty, order.StopLoss, order.TakeProfit, order.OrderID)
			}
			continue
		}
		status, statusErr := at.trader.GetOrderStatus(order.Symbol, order.OrderID)
		if statusErr == nil {
			order.Status = string(normalizeOrderState(fmt.Sprint(status["status"])))
			order.ExecutedQty = numberValue(status["executedQty"])
			order.AvgPrice = numberValue(status["avgPrice"])
			order.Fee = numberValue(status["commission"])
		} else {
			logger.Warnf("[%s] resuming order %s from persisted state because exchange status failed: %v", at.name, order.OrderID, statusErr)
		}
		state := normalizeOrderState(order.Status)
		result := map[string]interface{}{
			"orderId": order.OrderID, "status": string(state), "executedQty": order.ExecutedQty,
			"avgPrice": order.AvgPrice, "commission": order.Fee, "type": "LIMIT",
		}
		if state == OrderStateSubmitted || state == OrderStatePartial {
			logger.Infof("[%s] resuming pending entry order %s (%s)", at.name, order.OrderID, state)
			at.monitorPendingEntryOrderState(result, order.Symbol, order.Action, order.PositionSide,
				order.RequestedQty, order.AvgPrice, order.Leverage, order.StopLoss, order.TakeProfit, order.ProtectedQty)
			continue
		}
		if order.ExecutedQty > order.ProtectedQty+1e-9 {
			if _, err := at.projectAndProtectEntryFill(order.OrderID, order.Symbol, order.Action, order.PositionSide,
				order.RequestedQty, order.ExecutedQty, order.AvgPrice, order.Fee, order.Leverage,
				order.StopLoss, order.TakeProfit, order.ProtectedQty, state); err != nil {
				logger.Errorf("[%s] failed to recover protection for order %s: %v", at.name, order.OrderID, err)
			}
		}
	}
}

// protectOpenedPosition completes the opening saga. A position is not a
// successful business operation until both protective legs are installed.
func (at *AutoTrader) protectOpenedPosition(symbol, positionSide string, quantity, stopLoss, takeProfit float64, entryOrderID string) error {
	return at.protectPositionIncrement(symbol, positionSide, quantity, quantity, stopLoss, takeProfit, entryOrderID)
}

func (at *AutoTrader) protectPositionIncrement(symbol, positionSide string, installQuantity, cumulativeQuantity, stopLoss, takeProfit float64, entryOrderID string) error {
	at.updateProtectionStatus(entryOrderID, "PROTECTING")
	at.updateOrderProtection(entryOrderID, cumulativeQuantity-installQuantity, "PROTECTING")
	retries := at.config.ProtectionRetries
	if retries <= 0 {
		retries = 3
	}
	retryDelay := at.config.ProtectionRetryDelay
	if retryDelay <= 0 {
		retryDelay = time.Second
	}

	var stopErr, takeErr error
	if stopLoss <= 0 || takeProfit <= 0 {
		stopErr = fmt.Errorf("invalid persisted stop-loss %.8f", stopLoss)
		takeErr = fmt.Errorf("invalid persisted take-profit %.8f", takeProfit)
		retries = 0
	}
	stopInstalled := false
	takeInstalled := false
	for attempt := 1; attempt <= retries; attempt++ {
		if !stopInstalled {
			stopErr = at.trader.SetStopLoss(symbol, positionSide, installQuantity, stopLoss)
			stopInstalled = stopErr == nil
			status := "FAILED"
			if stopInstalled {
				status = "ACTIVE"
			}
			at.persistProtection(entryOrderID, symbol, positionSide, "STOP_LOSS", cumulativeQuantity, stopLoss, status, attempt, stopErr)
		}
		if !takeInstalled {
			takeErr = at.trader.SetTakeProfit(symbol, positionSide, installQuantity, takeProfit)
			takeInstalled = takeErr == nil
			status := "FAILED"
			if takeInstalled {
				status = "ACTIVE"
			}
			at.persistProtection(entryOrderID, symbol, positionSide, "TAKE_PROFIT", cumulativeQuantity, takeProfit, status, attempt, takeErr)
		}
		if stopInstalled && takeInstalled {
			at.updateProtectionStatus(entryOrderID, "PROTECTED")
			at.updateOrderProtection(entryOrderID, cumulativeQuantity, "PROTECTED")
			return nil
		}
		// Validation failures (precision, price filters, already-triggered
		// stops) are deterministic: resubmitting the identical request can
		// never succeed and only delays the fail-closed close below.
		if isDeterministicProtectionRejection(stopErr) || isDeterministicProtectionRejection(takeErr) {
			logger.Errorf("[%s] protection request for %s %s was deterministically rejected; skipping further retries (stop=%v, take=%v)",
				at.name, symbol, positionSide, stopErr, takeErr)
			break
		}
		if attempt < retries {
			logger.Warnf("[%s] protection attempt %d/%d failed for %s %s (stop=%v, take=%v)", at.name, attempt, retries, symbol, positionSide, stopErr, takeErr)
			time.Sleep(retryDelay)
		}
	}

	at.updateProtectionStatus(entryOrderID, "UNPROTECTED")
	at.updateOrderProtection(entryOrderID, cumulativeQuantity-installQuantity, "UNPROTECTED")
	protectionErr := fmt.Errorf("position %s %s is UNPROTECTED after %d attempts (stop=%v, take=%v)", symbol, positionSide, retries, stopErr, takeErr)
	logger.Errorf("[%s] CRITICAL: %v", at.name, protectionErr)

	{
		closeQuantity := cumulativeQuantity
		var order map[string]interface{}
		var err error
		if positionSide == "LONG" {
			order, err = at.trader.CloseLong(symbol, 0)
		} else {
			order, err = at.trader.CloseShort(symbol, 0)
		}
		if err != nil {
			return fmt.Errorf("%w; automatic close submission failed: %v", protectionErr, err)
		}
		if !at.recordAndConfirmOrder(order, symbol, "close_"+strings.ToLower(positionSide), closeQuantity, 0, 0, 0) {
			if executedQty, avgPrice := numberValue(order["executedQty"]), numberValue(order["avgPrice"]); executedQty > 0 && avgPrice > 0 {
				at.recordPositionChange(getOrderIDString(order), symbol, positionSide, "close_"+strings.ToLower(positionSide), executedQty, avgPrice, 0, 0, numberValue(order["commission"]))
				return fmt.Errorf("%w; automatic close order %s only partially filled %.8f", protectionErr, getOrderIDString(order), executedQty)
			}
			return fmt.Errorf("%w; automatic close order %s was not confirmed filled", protectionErr, getOrderIDString(order))
		}
		if filledQty := numberValue(order["executedQty"]); filledQty+1e-9 < closeQuantity {
			return fmt.Errorf("%w; automatic close order %s only filled %.8f of %.8f", protectionErr, getOrderIDString(order), filledQty, closeQuantity)
		}
		at.cancelPositionOrders(symbol, positionSide)
		at.updateOrderProtection(entryOrderID, cumulativeQuantity, "CLOSED")
		logger.Errorf("[%s] unprotected %s %s was automatically closed", at.name, symbol, positionSide)
	}
	return protectionErr
}

func (at *AutoTrader) projectAndProtectEntryFill(orderID, symbol, action, positionSide string, requestedQty, executedQty, avgPrice, fee float64, leverage int, stopLoss, takeProfit, protectedQty float64, state OrderState) (float64, error) {
	if executedQty <= 0 {
		return protectedQty, nil
	}
	at.persistEntryOrderState(orderID, symbol, action, requestedQty, executedQty, avgPrice, fee, state, leverage, stopLoss, takeProfit, protectedQty, "UNPROTECTED", nil)
	if at.store != nil && avgPrice > 0 {
		pos := &store.TraderPosition{
			TraderID: at.id, ExchangeID: at.exchangeID, ExchangeType: at.exchange,
			Symbol: symbol, Side: positionSide, Quantity: executedQty, EntryPrice: avgPrice,
			EntryOrderID: orderID, EntryTime: time.Now(), Leverage: leverage, Fee: fee,
		}
		if err := at.store.Position().UpsertEntryFill(pos); err != nil {
			return protectedQty, fmt.Errorf("failed to project entry fill: %w", err)
		}
	}
	if executedQty <= protectedQty+1e-9 {
		return protectedQty, nil
	}
	if coverage, ok := at.trader.(FutureFillProtection); ok && coverage.ProtectionCoversFutureFills() && protectedQty > 0 {
		at.persistProtection(orderID, symbol, positionSide, "STOP_LOSS", executedQty, stopLoss, "ACTIVE", 0, nil)
		at.persistProtection(orderID, symbol, positionSide, "TAKE_PROFIT", executedQty, takeProfit, "ACTIVE", 0, nil)
		at.updateProtectionStatus(orderID, "PROTECTED")
		at.updateOrderProtection(orderID, executedQty, "PROTECTED")
		return executedQty, nil
	}
	delta := executedQty - protectedQty
	if err := at.protectPositionIncrement(symbol, positionSide, delta, executedQty, stopLoss, takeProfit, orderID); err != nil {
		return protectedQty, err
	}
	return executedQty, nil
}

func (at *AutoTrader) monitorPendingEntryOrder(orderResult map[string]interface{}, symbol, action, positionSide string, quantity, fallbackPrice float64, leverage int, stopLoss, takeProfit float64) {
	at.monitorPendingEntryOrderState(orderResult, symbol, action, positionSide, quantity, fallbackPrice, leverage, stopLoss, takeProfit, 0)
}

func (at *AutoTrader) monitorPendingEntryOrderState(orderResult map[string]interface{}, symbol, action, positionSide string, quantity, fallbackPrice float64, leverage int, stopLoss, takeProfit, initialProtectedQty float64) {
	orderID := getOrderIDString(orderResult)
	if orderID == "" || orderID == "0" {
		return
	}
	initialQty := numberValue(orderResult["executedQty"])
	initialPrice := numberValue(orderResult["avgPrice"])
	initialFee := numberValue(orderResult["commission"])
	initialState := normalizeOrderState(fmt.Sprint(orderResult["status"]))
	at.persistEntryOrderState(orderID, symbol, action, quantity, initialQty, initialPrice, initialFee, initialState, leverage, stopLoss, takeProfit, initialProtectedQty, "UNPROTECTED", nil)
	at.lifecycleMutex.Lock()
	stopCh := at.stopMonitorCh
	at.lifecycleMutex.Unlock()

	at.monitorWg.Add(1)
	go func() {
		defer at.monitorWg.Done()
		logger.Infof("  ⏳ Monitoring pending limit order %s for %s %s", orderID, symbol, action)

		// Real-time order updates replace high-frequency polling whenever the
		// exchange supports a user-data stream; the poller degrades to a slow
		// safety net that repairs any event the websocket may have missed.
		pollInterval := at.config.PendingOrderPollInterval
		var events <-chan OrderUpdateEvent
		if streamer, ok := at.trader.(OrderUpdateStreamer); ok {
			if ch, unsubscribe := streamer.SubscribeOrderUpdates(); ch != nil {
				events = ch
				defer unsubscribe()
				fallback := pollInterval * 10
				if fallback < 15*time.Second {
					fallback = 15 * time.Second
				}
				if fallback > 60*time.Second {
					fallback = 60 * time.Second
				}
				pollInterval = fallback
				logger.Infof("  📡 Real-time order updates enabled for %s (fallback poll every %s)", orderID, fallback)
			}
		}
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		timeout := time.NewTimer(at.config.PendingOrderTimeout)
		defer timeout.Stop()

		partialQty := initialQty
		partialPrice := initialPrice
		partialFee := initialFee
		protectedQty := initialProtectedQty
		consecutiveErrors := 0
		processFill := func(state OrderState, executedQty, avgPrice, fee float64) bool {
			if executedQty <= protectedQty+1e-9 {
				return true
			}
			var err error
			protectedQty, err = at.projectAndProtectEntryFill(orderID, symbol, action, positionSide, quantity, executedQty, avgPrice, fee, leverage, stopLoss, takeProfit, protectedQty, state)
			if err != nil {
				logger.Errorf("[%s] entry order %s incremental protection failed: %v", at.name, orderID, err)
				at.cancelPendingOrder(symbol, orderID)
				return false
			}
			return true
		}
		if partialQty > 0 && !processFill(initialState, partialQty, partialPrice, partialFee) {
			return
		}
		finalizePending := func(reason string) bool {
			at.cancelPendingOrder(symbol, orderID)
			status, err := at.trader.GetOrderStatus(symbol, orderID)
			if err != nil {
				logger.Warnf("[%s] %s order %s cancellation is not confirmed: %v", at.name, reason, orderID, err)
				return false
			}
			state := normalizeOrderState(fmt.Sprint(status["status"]))
			if qty := numberValue(status["executedQty"]); qty > partialQty {
				partialQty = qty
				partialPrice = numberValue(status["avgPrice"])
				partialFee = numberValue(status["commission"])
			}
			protectionState := "UNPROTECTED"
			if partialQty > 0 && partialQty <= protectedQty+1e-9 {
				protectionState = "PROTECTED"
			}
			at.persistEntryOrderState(orderID, symbol, action, quantity, partialQty, partialPrice, partialFee, state, leverage, stopLoss, takeProfit, protectedQty, protectionState, nil)
			if state != OrderStateFilled && state != OrderStateCanceled && state != OrderStateRejected {
				logger.Warnf("[%s] %s order %s remains %s after cancel request; monitoring continues", at.name, reason, orderID, state)
				return false
			}
			return processFill(state, partialQty, partialPrice, partialFee)
		}
		// handleObservation processes one order-state observation (real-time
		// event or poll) and reports whether monitoring should stop.
		handleObservation := func(source string, state OrderState, executedQty, avgPrice, fee float64) bool {
			if executedQty > partialQty {
				partialQty, partialPrice, partialFee = executedQty, avgPrice, fee
			}
			protectionState := "UNPROTECTED"
			if executedQty > 0 && executedQty <= protectedQty+1e-9 {
				protectionState = "PROTECTED"
			}
			at.persistEntryOrderState(orderID, symbol, action, quantity, executedQty, avgPrice, fee, state, leverage, stopLoss, takeProfit, protectedQty, protectionState, nil)
			if executedQty > protectedQty+1e-9 && !processFill(state, executedQty, avgPrice, fee) {
				return true
			}
			switch state {
			case OrderStateFilled:
				if avgPrice <= 0 || executedQty <= 0 {
					logger.Errorf("[%s] pending order %s reported FILLED without valid fill data (%s)", at.name, orderID, source)
					return true
				}
				logger.Infof("  ✅ Pending limit order filled and protected: %s %s order=%s", symbol, positionSide, orderID)
				return true
			case OrderStatePartial:
				logger.Warnf("  Partial fill detected for pending order %s (%s): executedQty=%.8f", orderID, source, executedQty)
			case OrderStateCanceled, OrderStateRejected:
				logger.Infof("  ⚠️ Pending limit order %s ended with status %s (%s)", orderID, state, source)
				return true
			}
			return false
		}
		for {
			select {
			case <-stopCh:
				if !finalizePending("stopped") {
					logger.Errorf("[%s] pending order %s could not be finalized before stop; it may remain live at the exchange until reconciliation", at.name, orderID)
				}
				logger.Infof("  ⏹ Stop monitoring pending order %s because trader stopped", orderID)
				return
			case <-timeout.C:
				if finalizePending("timed-out") {
					logger.Infof("  ⏰ Pending limit order %s reached a terminal exchange state", orderID)
					return
				}
				timeout.Reset(15 * time.Second)
			case ev := <-events:
				if ev.OrderID != orderID {
					continue
				}
				if ev.ExecutedQty <= 0 && ev.Status != OrderStateCanceled && ev.Status != OrderStateRejected {
					continue // lifecycle noise (e.g. NEW) without fill information
				}
				// Terminal states are confirmed with one authoritative REST query:
				// it repairs anything the stream may have missed and yields exact
				// cumulative fee accounting.
				if ev.Status == OrderStateFilled || ev.Status == OrderStateCanceled || ev.Status == OrderStateRejected {
					status, confirmErr := at.trader.GetOrderStatus(symbol, orderID)
					if confirmErr == nil {
						if handleObservation("confirmed", normalizeOrderState(fmt.Sprint(status["status"])), numberValue(status["executedQty"]), numberValue(status["avgPrice"]), numberValue(status["commission"])) {
							return
						}
						continue
					}
					logger.Infof("  ⚠️ Could not confirm terminal state of order %s via REST: %v", orderID, confirmErr)
				}
				if handleObservation("stream", ev.Status, ev.ExecutedQty, ev.AvgPrice, ev.Fee) {
					return
				}
			case <-ticker.C:
				status, err := at.trader.GetOrderStatus(symbol, orderID)
				if err != nil {
					// Back off progressively so a rate-limited or unreachable
					// exchange is not hammered every poll tick.
					consecutiveErrors++
					backoff := time.Duration(consecutiveErrors) * pollInterval
					if backoff > 30*time.Second {
						backoff = 30 * time.Second
					}
					logger.Infof("  ⚠️ Failed to check pending order %s (%d in a row): %v; backing off %s", orderID, consecutiveErrors, err, backoff)
					select {
					case <-stopCh:
					case <-time.After(backoff):
					}
					continue
				}
				consecutiveErrors = 0

				statusStr, _ := status["status"].(string)
				if handleObservation("poll", normalizeOrderState(statusStr), numberValue(status["executedQty"]), numberValue(status["avgPrice"]), numberValue(status["commission"])) {
					return
				}
			}
		}
	}()
}

// recordAndConfirmOrder polls order status for actual fill data and records position.
// It returns true only when the order can be treated as filled/recorded locally.
// action: open_long, open_short, close_long, close_short
// entryPrice: entry price when closing (0 when opening)
func (at *AutoTrader) recordAndConfirmOrder(orderResult map[string]interface{}, symbol, action string, quantity float64, price float64, leverage int, entryPrice float64) bool {
	orderID := getOrderIDString(orderResult)
	if orderID == "" || orderID == "0" {
		logger.Infof("  ⚠️ Order ID is empty, skipping record")
		return false
	}
	at.persistOrderState(orderID, symbol, action, quantity, 0, 0, 0, OrderStateSubmitted, nil)

	// Determine positionSide
	var positionSide string
	switch action {
	case "open_long", "close_long":
		positionSide = "LONG"
	case "open_short", "close_short":
		positionSide = "SHORT"
	}

	// Poll order status to get actual fill price, quantity and fee
	var actualPrice float64
	var actualQty float64
	var fee float64
	orderFilled := false
	lastState := OrderStateSubmitted

	// Wait for order to be filled and get actual fill data
	time.Sleep(500 * time.Millisecond)
	for i := 0; i < 5; i++ {
		status, err := at.trader.GetOrderStatus(symbol, orderID)
		if err == nil {
			statusStr, _ := status["status"].(string)
			lastState = normalizeOrderState(statusStr)
			observedQty := numberValue(status["executedQty"])
			observedPrice := numberValue(status["avgPrice"])
			observedFee := numberValue(status["commission"])
			orderResult["status"] = statusStr
			orderResult["executedQty"] = observedQty
			orderResult["avgPrice"] = observedPrice
			orderResult["commission"] = observedFee
			at.persistOrderState(orderID, symbol, action, quantity, observedQty, observedPrice, observedFee, lastState, nil)
			if lastState == OrderStateFilled {
				// Get actual fill price
				actualPrice = numberValue(status["avgPrice"])
				// Get actual executed quantity
				actualQty = numberValue(status["executedQty"])
				// Get commission/fee
				if commission, ok := status["commission"].(float64); ok {
					fee = commission
				}
				if actualPrice <= 0 || actualQty <= 0 {
					logger.Errorf("  Order %s reported FILLED without valid fill data (price=%.8f qty=%.8f)", orderID, actualPrice, actualQty)
					return false
				}
				logger.Infof("  ✅ Order filled: avgPrice=%.6f, qty=%.6f, fee=%.6f", actualPrice, actualQty, fee)
				orderFilled = true
				break
			} else if lastState == OrderStateCanceled || lastState == OrderStateRejected {
				executed := numberValue(status["executedQty"])
				if executed > 0 {
					logger.Errorf("  Order %s ended %s after partial execution %.8f; position reconciliation required", orderID, statusStr, executed)
				} else {
					logger.Infof("  ⚠️ Order %s, skipping position record", statusStr)
				}
				return false
			} else if lastState == OrderStatePartial {
				logger.Warnf("  Order %s is partially filled: executedQty=%.8f", orderID, numberValue(status["executedQty"]))
			}
		} else {
			at.persistOrderState(orderID, symbol, action, quantity, 0, 0, 0, lastState, err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	if !orderFilled {
		logger.Errorf("  Order %s was not confirmed filled (last state: %s); local position will not be mutated", orderID, lastState)
		return false
	}

	logger.Infof("  📝 Recording position (ID: %s, action: %s, price: %.6f, qty: %.6f, fee: %.4f)",
		orderID, action, actualPrice, actualQty, fee)
	orderResult["status"] = string(OrderStateFilled)
	orderResult["avgPrice"] = actualPrice
	orderResult["executedQty"] = actualQty
	orderResult["commission"] = fee

	// Record position change with actual fill data
	at.recordPositionChange(orderID, symbol, positionSide, action, actualQty, actualPrice, leverage, entryPrice, fee)
	return true
}

// publishPositionEvent emits a trading notification event to the global bus.
// It never blocks or panics the trading loop; a nil bus target is tolerated.
func (at *AutoTrader) publishPositionEvent(eventType string, severity notification.Severity, symbol, title, body string) {
	e := &notification.Event{
		EventType:  eventType,
		Severity:   severity,
		UserID:     at.userID,
		TraderID:   at.id,
		TraderName: at.name,
		Symbol:     symbol,
		Title:      title,
		Body:       body,
		Time:       time.Now().UTC(),
	}
	if e.Title != "" {
		e.Title = at.name + ": " + e.Title
	}
	notification.Publish(e)
}

// recordPositionChange records position change (create record on open, update record on close)
func (at *AutoTrader) recordPositionChange(orderID, symbol, side, action string, quantity, price float64, leverage int, entryPrice float64, fee float64) {
	if at.store == nil {
		return
	}

	switch action {
	case "open_long", "open_short":
		// Open position: create new position record
		pos := &store.TraderPosition{
			TraderID:     at.id,
			ExchangeID:   at.exchangeID, // Exchange account UUID
			ExchangeType: at.exchange,   // Exchange type: binance/bybit/okx/etc
			Symbol:       symbol,
			Side:         side, // LONG or SHORT
			Quantity:     quantity,
			EntryPrice:   price,
			EntryOrderID: orderID,
			EntryTime:    time.Now(),
			Leverage:     leverage,
			Fee:          fee,
			Status:       "OPEN",
		}
		if err := at.store.Position().UpsertEntryFill(pos); err != nil {
			logger.Infof("  ⚠️ Failed to record position: %v", err)
		} else {
			logger.Infof("  📊 Position recorded [%s] %s %s @ %.4f", at.id[:8], symbol, side, price)
			at.publishPositionEvent(notification.EventPositionOpened, notification.SeverityInfo, symbol,
				fmt.Sprintf("📈 Opened %s %s", strings.ToLower(side), symbol),
				fmt.Sprintf("Quantity: %.4f @ %.4f, Leverage: %dx", quantity, price, leverage))
		}

	case "close_long", "close_short":
		// Close position: find corresponding open position record and update
		openPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, symbol, side)
		if err != nil || openPos == nil {
			logger.Infof("  ⚠️ Cannot find corresponding open position record (%s %s)", symbol, side)
			return
		}

		closedQuantity := math.Min(quantity, openPos.Quantity)
		if closedQuantity <= 0 {
			logger.Errorf("  Invalid close fill quantity %.8f for %s %s", quantity, symbol, side)
			return
		}

		// Calculate P&L for the confirmed fill only.
		var realizedPnL float64
		if side == "LONG" {
			realizedPnL = (price - openPos.EntryPrice) * closedQuantity
		} else {
			realizedPnL = (openPos.EntryPrice - price) * closedQuantity
		}

		remainingQuantity := openPos.Quantity - closedQuantity
		if remainingQuantity > 1e-9 {
			if err := at.store.Position().ApplyPartialClose(openPos.ID, remainingQuantity, realizedPnL, fee); err != nil {
				logger.Errorf("  Failed to record partial close: %v", err)
			} else {
				logger.Warnf("  Position partially closed [%s] %s %s: closed=%.8f remaining=%.8f", at.id, symbol, side, closedQuantity, remainingQuantity)
				at.publishPositionEvent(notification.EventPartialClose, notification.SeverityInfo, symbol,
					fmt.Sprintf("〰️ Partially closed %s %s (%+.2f USDT)", strings.ToLower(side), symbol, realizedPnL),
					fmt.Sprintf("Closed: %.4f, Remaining: %.4f, P&L: %+.2f USDT", closedQuantity, remainingQuantity, realizedPnL))
			}
			return
		}

		// Update position record
		err = at.store.Position().ClosePosition(
			openPos.ID,
			price,   // exitPrice
			orderID, // exitOrderID
			realizedPnL,
			fee, // fee from exchange API
			"ai_decision",
		)
		if err != nil {
			logger.Infof("  ⚠️ Failed to update position: %v", err)
		} else {
			logger.Infof("  📊 Position closed [%s] %s %s @ %.4f → %.4f, P&L: %.2f, Fee: %.4f",
				at.id[:8], symbol, side, openPos.EntryPrice, price, realizedPnL, fee)
			severity := notification.SeverityInfo
			emoji := "✅"
			if realizedPnL < 0 {
				severity = notification.SeverityWarning
				emoji = "🔻"
			}
			pnlPct := 0.0
			if openPos.EntryPrice > 0 && openPos.Leverage > 0 {
				pnlPct = ((price - openPos.EntryPrice) / openPos.EntryPrice) * float64(openPos.Leverage)
				if side == "SHORT" {
					pnlPct = -pnlPct
				}
			}
			at.publishPositionEvent(notification.EventPositionClosed, severity, symbol,
				fmt.Sprintf("%s Closed %s %s (%+.2f USDT)", emoji, strings.ToLower(side), symbol, realizedPnL),
				fmt.Sprintf("Entry: %.4f → Exit: %.4f, P&L: %+.2f USDT (%+.2f%%), Fee: %.4f",
					openPos.EntryPrice, price, realizedPnL, pnlPct, fee))
		}
	}
}

// ============================================================================
// Risk Control Helpers
// ============================================================================

func (at *AutoTrader) buildBackendEntryPlan(d *decision.Decision, entryPrice float64, positions []map[string]interface{}, balance map[string]interface{}, data *market.Data) (decision.EntryPlan, error) {
	equity, ok := accountNumber(balance, "total_equity", "totalEquity", "totalEq", "accountEquity")
	if !ok {
		wallet, walletOK := accountNumber(balance, "totalWalletBalance", "wallet_balance", "walletBalance")
		unrealized, _ := accountNumber(balance, "totalUnrealizedProfit", "unrealized_profit", "unrealized_pnl")
		if walletOK {
			equity = wallet + unrealized
		}
	}
	available, ok := accountNumber(balance, "availableBalance", "available_balance", "available", "availBal")
	if !ok {
		return decision.EntryPlan{}, fmt.Errorf("failed to determine available balance from exchange response")
	}
	if equity <= 0 {
		return decision.EntryPlan{}, fmt.Errorf("failed to determine positive account equity from exchange response")
	}

	maxLeverage := 1
	minPositionSize := 0.0
	maxPositionSize := 0.0
	remainingNotional := math.Inf(1)
	if at.config.StrategyConfig != nil {
		risk := at.config.StrategyConfig.RiskControl
		maxLeverage = risk.AltcoinMaxLeverage
		if isBTCETHSymbol(d.Symbol) {
			maxLeverage = risk.BTCETHMaxLeverage
		}
		minPositionSize = risk.MinPositionSize
		maxPositionSize = risk.MaxPositionSize
		if risk.MaxTotalPositionSize > 0 {
			remainingNotional = math.Max(risk.MaxTotalPositionSize-totalPositionNotional(positions), 0)
		}
	}
	atr, latestVolume := decision.ConservativeATRAndVolume(data)
	pendingEntries := 0
	if at.store != nil {
		orders, err := at.store.Execution().ListActiveOrders(at.id, at.exchangeID)
		if err != nil {
			return decision.EntryPlan{}, fmt.Errorf("cannot verify pending-entry risk: %w", err)
		}
		for _, order := range orders {
			if order.Action == "open_long" || order.Action == "open_short" {
				pendingEntries++
			}
		}
	}
	return decision.CalculateEntryPlan(decision.EntrySizingInput{
		Action:                d.Action,
		Equity:                equity,
		AvailableBalance:      available,
		EntryPrice:            entryPrice,
		StopLoss:              d.StopLoss,
		ATR:                   atr,
		LatestBaseVolume:      latestVolume,
		ExistingPositionCount: len(positions) + pendingEntries,
		MaxLeverage:           maxLeverage,
		MinPositionSize:       minPositionSize,
		MaxPositionSize:       maxPositionSize,
		RemainingNotional:     remainingNotional,
	})
}

// conservativeATRAndVolume uses the largest available ATR and the smallest
// positive recent volume. K-line volume is only a liquidity proxy; the common
// trader interface currently exposes no portable order-book depth snapshot.
func conservativeATRAndVolume(data *market.Data) (float64, float64) {
	return decision.ConservativeATRAndVolume(data)
}

// evaluateAccountRisk updates the mark-to-market account state once per cycle.
// It deliberately blocks only future entries; close orders remain available.
func (at *AutoTrader) evaluateAccountRisk(ctx *decision.Context) bool {
	at.riskMutex.Lock()
	defer at.riskMutex.Unlock()

	now := time.Now().UTC()
	if at.lastResetTime.IsZero() || now.Format("2006-01-02") != at.lastResetTime.UTC().Format("2006-01-02") {
		at.dayStartEquity = ctx.Account.TotalEquity
		at.dailyPnL = 0
		at.lastResetTime = now
	}
	if at.dayStartEquity <= 0 {
		at.dayStartEquity = ctx.Account.TotalEquity
	}
	if at.equityHighWater < ctx.Account.TotalEquity {
		at.equityHighWater = ctx.Account.TotalEquity
	}
	at.dailyPnL = ctx.Account.TotalEquity - at.dayStartEquity

	limits := at.accountRiskLimits()
	if at.dayStartEquity > 0 && limits.maxDailyLoss > 0 && at.dailyPnL/at.dayStartEquity*100 <= -limits.maxDailyLoss {
		at.triggerRiskLocked("daily loss limit")
	}
	if at.equityHighWater > 0 && limits.maxDrawdown > 0 && (at.equityHighWater-ctx.Account.TotalEquity)/at.equityHighWater*100 >= limits.maxDrawdown {
		at.manualReviewRequired = true
		at.triggerRiskLocked("equity drawdown limit")
	}
	if limits.maxMarginUsage > 0 && ctx.Account.MarginUsedPct >= limits.maxMarginUsage {
		at.triggerRiskLocked("margin usage limit")
	}
	for _, position := range ctx.Positions {
		if liquidationDistancePct(position.MarkPrice, position.LiquidationPrice) <= limits.minLiquidationDistance && position.LiquidationPrice > 0 {
			at.triggerRiskLocked("liquidation distance limit")
			break
		}
	}
	return at.manualReviewRequired || time.Now().Before(at.stopUntil)
}

// ClearManualReviewHalt is intentionally explicit: an account drawdown halt
// never expires on a timer and must be acknowledged by an operator.
func (at *AutoTrader) ClearManualReviewHalt() {
	at.riskMutex.Lock()
	at.manualReviewRequired = false
	at.equityHighWater = 0
	at.riskMutex.Unlock()
}

type accountRiskLimits struct {
	maxDailyLoss, maxDrawdown, maxMarginUsage, minLiquidationDistance, maxMarketMove float64
	maxFailures                                                                      int
	stopFor                                                                          time.Duration
}

func (at *AutoTrader) accountRiskLimits() accountRiskLimits {
	limits := accountRiskLimits{maxDailyLoss: decision.DailyLossHaltPct, maxDrawdown: decision.AccountDrawdownHaltPct, stopFor: at.config.StopTradingTime}
	if at.config.MaxDailyLoss > 0 && at.config.MaxDailyLoss < limits.maxDailyLoss {
		limits.maxDailyLoss = at.config.MaxDailyLoss
	}
	if at.config.MaxDrawdown > 0 && at.config.MaxDrawdown < limits.maxDrawdown {
		limits.maxDrawdown = at.config.MaxDrawdown
	}
	if at.config.StrategyConfig != nil {
		r := at.config.StrategyConfig.RiskControl
		if r.MaxDailyLossPct > 0 && r.MaxDailyLossPct < limits.maxDailyLoss {
			limits.maxDailyLoss = r.MaxDailyLossPct
		}
		if r.MaxDrawdownPct > 0 && r.MaxDrawdownPct < limits.maxDrawdown {
			limits.maxDrawdown = r.MaxDrawdownPct
		}
		limits.maxMarginUsage, limits.minLiquidationDistance = r.MaxMarginUsagePct, r.MinLiquidationDistancePct
		limits.maxFailures, limits.maxMarketMove = r.MaxConsecutiveFailures, r.MaxMarketMovePct
		limits.stopFor = time.Duration(r.StopTradingMinutes) * time.Minute
	}
	return limits
}

func (at *AutoTrader) triggerRiskLocked(reason string) {
	duration := at.accountRiskLimits().stopFor
	if duration <= 0 {
		duration = time.Hour
	}
	until := time.Now().Add(duration)
	if until.After(at.stopUntil) {
		at.stopUntil = until
		logger.Warnf("🚨 [RISK CONTROL] %s triggered; new entries paused until %s", reason, until.Format(time.RFC3339))
		at.publishPositionEvent(notification.EventRiskTriggered, notification.SeverityCritical, "",
			fmt.Sprintf("🚨 Risk control triggered: %s", reason),
			fmt.Sprintf("New entries paused until %s", until.Format("2006-01-02 15:04 MST")))
	}
}

func liquidationDistancePct(markPrice, liquidationPrice float64) float64 {
	if markPrice <= 0 || liquidationPrice <= 0 {
		return math.Inf(1)
	}
	return math.Abs(markPrice-liquidationPrice) / markPrice * 100
}

// enforceAccountEntryRisk is called while entryMutex is held, immediately
// before order sizing, so a stale cycle snapshot cannot bypass a breaker.
func (at *AutoTrader) enforceAccountEntryRisk(positions []map[string]interface{}, balance map[string]interface{}, positionSize float64, leverage int, data *market.Data) error {
	limits := at.accountRiskLimits()
	if data != nil && limits.maxMarketMove > 0 && (math.Abs(data.PriceChange1h) >= limits.maxMarketMove || math.Abs(data.PriceChange4h) >= limits.maxMarketMove) {
		at.riskMutex.Lock()
		at.triggerRiskLocked("abnormal market move")
		at.riskMutex.Unlock()
		return fmt.Errorf("❌ [RISK CONTROL] abnormal market move (1h %.2f%%, 4h %.2f%%)", data.PriceChange1h, data.PriceChange4h)
	}

	equity, ok := accountNumber(balance, "total_equity", "totalEquity", "totalEq", "accountEquity")
	if !ok {
		wallet, walletOK := accountNumber(balance, "totalWalletBalance", "wallet_balance", "walletBalance")
		unrealized, _ := accountNumber(balance, "totalUnrealizedProfit", "unrealized_profit", "unrealized_pnl")
		if walletOK {
			equity = wallet + unrealized
		}
	}
	marginUsed := 0.0
	for _, position := range positions {
		mark, _ := accountNumber(position, "markPrice", "mark_price")
		liq, _ := accountNumber(position, "liquidationPrice", "liquidation_price")
		if liq > 0 && liquidationDistancePct(mark, liq) <= limits.minLiquidationDistance {
			at.riskMutex.Lock()
			at.triggerRiskLocked("liquidation distance limit")
			at.riskMutex.Unlock()
			return fmt.Errorf("❌ [RISK CONTROL] an open position is too close to liquidation")
		}
		if used, ok := accountNumber(position, "margin_used", "marginUsed", "positionInitialMargin"); ok {
			marginUsed += used
			continue
		}
		quantity, _ := accountNumber(position, "positionAmt", "size", "quantity")
		lev, hasLev := accountNumber(position, "leverage")
		if mark > 0 && hasLev && lev > 0 {
			marginUsed += math.Abs(quantity) * mark / lev
		}
	}
	if leverage > 0 && equity > 0 && limits.maxMarginUsage > 0 && (marginUsed+positionSize/float64(leverage))/equity*100 > limits.maxMarginUsage {
		return fmt.Errorf("❌ [RISK CONTROL] projected margin usage exceeds %.2f%%", limits.maxMarginUsage)
	}
	at.riskMutex.Lock()
	paused := time.Now().Before(at.stopUntil)
	manualReviewRequired := at.manualReviewRequired
	at.riskMutex.Unlock()
	if manualReviewRequired {
		return fmt.Errorf("❌ [RISK CONTROL] account drawdown halt requires manual review")
	}
	if paused {
		return fmt.Errorf("❌ [RISK CONTROL] new entries are paused")
	}
	return nil
}

func (at *AutoTrader) recordExecutionFailure(err error) {
	if err == nil || isPolicyRejection(err) || isPendingReconciliation(err) {
		return
	}
	at.riskMutex.Lock()
	defer at.riskMutex.Unlock()
	at.consecutiveFailures++
	limits := at.accountRiskLimits()
	if limits.maxFailures > 0 && at.consecutiveFailures >= limits.maxFailures {
		at.triggerRiskLocked(fmt.Sprintf("%d consecutive execution failures", at.consecutiveFailures))
	}
}

// isPolicyRejection distinguishes an intentional strategy/precondition block
// from an exchange execution failure. Policy rejections remain visible in the
// decision record, but should not poison the exchange-health circuit breaker.
func isPolicyRejection(err error) bool {
	message := strings.ToLower(err.Error())
	markers := []string{
		"[risk control]",
		"already has long position",
		"already has short position",
		"already has a pending",
		"already at max positions",
		"no long position found",
		"no short position found",
		"below minimum",
		"confidence ",
	}
	for _, marker := range markers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func (at *AutoTrader) recordExecutionSuccess() {
	at.riskMutex.Lock()
	at.consecutiveFailures = 0
	at.riskMutex.Unlock()
}

// maxRecentExecutionFailures bounds the in-memory failure buffer.
const maxRecentExecutionFailures = 16

// maxExecutionFailureMessageLen truncates verbose exchange errors so the
// feedback injected into the AI prompt stays compact.
const maxExecutionFailureMessageLen = 220

// rememberExecutionFailure records a failed instruction so the next AI cycle
// can see it and avoid repeating an ineffective decision. Policy rejections
// (e.g. "already has long position") are included on purpose: they tell the
// AI why its intent was blocked.
func (at *AutoTrader) rememberExecutionFailure(symbol, action string, err error) {
	if err == nil {
		return
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		message = "unknown execution error"
	}
	// Collapse embedded newlines so a multi-line error cannot break the
	// numbered list format rendered into the prompt.
	message = strings.Join(strings.Fields(message), " ")
	if len(message) > maxExecutionFailureMessageLen {
		message = message[:maxExecutionFailureMessageLen] + "..."
	}

	at.feedbackMutex.Lock()
	defer at.feedbackMutex.Unlock()
	at.recentExecutionFailures = append(at.recentExecutionFailures, decision.RecentExecutionFailure{
		Symbol:    symbol,
		Action:    action,
		Error:     message,
		Timestamp: time.Now(),
	})
	if overflow := len(at.recentExecutionFailures) - maxRecentExecutionFailures; overflow > 0 {
		at.recentExecutionFailures = at.recentExecutionFailures[overflow:]
	}
}

// recentExecutionFailuresForPrompt returns recent failures within the
// feedback window (2x scan interval, so at least the previous cycle is
// covered), deduplicated by symbol+action+error, newest first, capped at 8.
// A failed instruction that later succeeds is removed from the feedback.
func (at *AutoTrader) recentExecutionFailuresForPrompt() []decision.RecentExecutionFailure {
	window := 10 * time.Minute
	if at.config.ScanInterval > 0 {
		if w := 2 * at.config.ScanInterval; w > window {
			window = w
		}
	}
	cutoff := time.Now().Add(-window)

	at.feedbackMutex.Lock()
	defer at.feedbackMutex.Unlock()

	seen := make(map[string]bool)
	var failures []decision.RecentExecutionFailure
	for i := len(at.recentExecutionFailures) - 1; i >= 0; i-- {
		f := at.recentExecutionFailures[i]
		if f.Timestamp.Before(cutoff) {
			continue
		}
		key := f.Symbol + "|" + f.Action + "|" + f.Error
		if seen[key] {
			continue
		}
		seen[key] = true
		failures = append(failures, f)
		if len(failures) >= 8 {
			break
		}
	}
	return failures
}

// forgetExecutionFailureOnSuccess drops matching failure entries once the
// same symbol+action executes successfully, so the prompt only reflects
// instructions that are still not effective.
func (at *AutoTrader) forgetExecutionFailureOnSuccess(symbol, action string) {
	at.feedbackMutex.Lock()
	defer at.feedbackMutex.Unlock()
	kept := at.recentExecutionFailures[:0]
	for _, f := range at.recentExecutionFailures {
		if f.Symbol == symbol && f.Action == action {
			continue
		}
		kept = append(kept, f)
	}
	at.recentExecutionFailures = kept
}

// isPendingReconciliation reports an error whose outcome is not yet known:
// the exchange accepted the order but confirmation polling timed out, or only
// a partial fill was observed. The position sync manager reconciles these
// orders asynchronously, so they must not poison the exchange-health circuit
// breaker (which would pause all future entries).
func isPendingReconciliation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	markers := []string{
		"was not confirmed filled",
		"only partially filled",
		"only filled",
	}
	for _, marker := range markers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

// isDeterministicProtectionRejection reports whether a protective-order
// failure is a validation error that resubmitting the identical request can
// never fix (price precision, exchange filters, or an already-triggered
// stop). Retrying these only delays the fail-closed close.
func isDeterministicProtectionRejection(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	markers := []string{
		"precision is over the maximum",
		"would immediately trigger",
		"filter failure",
		"price not increased",
		"reduceonly rejected",
		"invalid stopprice",
		"mandatory param",
	}
	for _, marker := range markers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

// totalPositionNotional sums the notional value of all open positions; used
// to enforce the aggregate position limit during backend sizing.
func totalPositionNotional(positions []map[string]interface{}) float64 {
	total := 0.0
	for _, position := range positions {
		quantity, _ := accountNumber(position, "positionAmt", "size", "quantity")
		quantity = math.Abs(quantity)
		price, _ := accountNumber(position, "markPrice", "mark_price", "entryPrice", "entry_price")
		if quantity > 0 && price > 0 {
			total += quantity * price
		}
	}
	return total
}

func numberValue(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		parsed, _ := strconv.ParseFloat(v, 64)
		return parsed
	default:
		return 0
	}
}

// enforceMaxPositions checks maximum positions count (CODE ENFORCED)
func (at *AutoTrader) enforceMaxPositions(currentPositionCount int) error {
	if at.config.StrategyConfig == nil {
		return nil
	}

	if err := decision.ValidatePositionCount(currentPositionCount, at.config.StrategyConfig.RiskControl); err != nil {
		return fmt.Errorf("❌ [RISK CONTROL] %w", err)
	}
	return nil
}
