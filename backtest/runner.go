package backtest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"nofx/logger"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"nofx/decision"
	"nofx/market"
	"nofx/mcp"
	"nofx/store"
)

var (
	errBacktestCompleted = errors.New("backtest completed")
	errLiquidated        = errors.New("account liquidated")
)

const (
	metricsWriteInterval = 5 * time.Second
	aiDecisionMaxRetries = 3
)

// Runner encapsulates the lifecycle of a single backtest run.
type Runner struct {
	cfg            BacktestConfig
	feed           *DataFeed
	account        *BacktestAccount
	strategyEngine *decision.StrategyEngine

	decisionLogDir string
	mcpClient      mcp.AIClient

	statusMu sync.RWMutex
	status   RunState

	stateMu sync.RWMutex
	state   *BacktestState

	pauseCh  chan struct{}
	resumeCh chan struct{}
	stopCh   chan struct{}
	doneCh   chan struct{}

	err              error
	errMu            sync.RWMutex
	lastError        string
	lastCheckpoint   time.Time
	createdAt        time.Time
	lastMetricsWrite time.Time

	aiCache   *AICache
	cachePath string

	lockInfo *RunLockInfo
	lockStop chan struct{}
}

// NewRunner constructs a backtest runner.
func NewRunner(cfg BacktestConfig, mcpClient mcp.AIClient) (*Runner, error) {
	if err := ensureRunDir(cfg.RunID); err != nil {
		return nil, err
	}

	client, err := configureMCPClient(cfg, mcpClient)
	if err != nil {
		return nil, err
	}

	feed, err := NewDataFeed(cfg)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(decisionLogDir(cfg.RunID), 0o755); err != nil {
		return nil, err
	}

	dLogDir := decisionLogDir(cfg.RunID)
	account := NewBacktestAccount(cfg.InitialBalance, cfg.FeeBps, cfg.SlippageBps)
	account.SetMarginTiers(cfg.MarginTiers)

	createdAt := time.Now().UTC()
	state := &BacktestState{
		Positions:      make(map[string]PositionSnapshot),
		Cash:           account.Cash(),
		Equity:         cfg.InitialBalance,
		UnrealizedPnL:  0,
		RealizedPnL:    0,
		MaxEquity:      cfg.InitialBalance,
		MinEquity:      cfg.InitialBalance,
		MaxDrawdownPct: 0,
		LastUpdate:     createdAt,
	}

	var (
		aiCache   *AICache
		cachePath string
	)
	if cfg.CacheAI || cfg.ReplayOnly || cfg.SharedAICachePath != "" {
		cachePath = cfg.SharedAICachePath
		if cachePath == "" {
			cachePath = filepath.Join(runDir(cfg.RunID), "ai_cache.json")
		}
		cache, err := LoadAICache(cachePath)
		if err != nil {
			return nil, fmt.Errorf("load ai cache: %w", err)
		}
		aiCache = cache
	}

	// Create strategy engine from backtest config for unified prompt generation
	strategyConfig := cfg.ToStrategyConfig()
	strategyEngine := decision.NewStrategyEngine(strategyConfig)

	r := &Runner{
		cfg:            cfg,
		feed:           feed,
		account:        account,
		strategyEngine: strategyEngine,
		decisionLogDir: dLogDir,
		mcpClient:      client,
		status:         RunStateCreated,
		state:          state,
		pauseCh:        make(chan struct{}, 1),
		resumeCh:       make(chan struct{}, 1),
		stopCh:         make(chan struct{}, 1),
		doneCh:         make(chan struct{}),
		createdAt:      createdAt,
		aiCache:        aiCache,
		cachePath:      cachePath,
	}

	if err := r.initLock(); err != nil {
		return nil, err
	}

	return r, nil
}

func (r *Runner) initLock() error {
	if r.cfg.RunID == "" {
		return fmt.Errorf("run_id required for lock")
	}
	info, err := acquireRunLock(r.cfg.RunID)
	if err != nil {
		return err
	}
	r.lockInfo = info
	r.lockStop = make(chan struct{})
	go r.lockHeartbeatLoop()
	return nil
}

func (r *Runner) lockHeartbeatLoop() {
	ticker := time.NewTicker(lockHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := updateRunLockHeartbeat(r.lockInfo); err != nil {
				logger.Infof("failed to update lock heartbeat for %s: %v", r.cfg.RunID, err)
			}
		case <-r.lockStop:
			return
		}
	}
}

func (r *Runner) releaseLock() {
	if r.lockStop != nil {
		close(r.lockStop)
		r.lockStop = nil
	}
	if err := deleteRunLock(r.cfg.RunID); err != nil {
		logger.Infof("failed to release lock for %s: %v", r.cfg.RunID, err)
	}
	r.lockInfo = nil
}

// Start launches the backtest loop.
func (r *Runner) Start(ctx context.Context) error {
	r.statusMu.Lock()
	if r.status != RunStateCreated && r.status != RunStatePaused {
		r.statusMu.Unlock()
		return fmt.Errorf("cannot start runner in state %s", r.status)
	}
	r.status = RunStateRunning
	r.statusMu.Unlock()

	go r.loop(ctx)
	return nil
}

// PersistMetadata writes the current snapshot to run.json.
func (r *Runner) PersistMetadata() {
	r.persistMetadata()
}

func (r *Runner) setLastError(err error) {
	r.errMu.Lock()
	defer r.errMu.Unlock()
	if err == nil {
		r.lastError = ""
		return
	}
	r.lastError = err.Error()
}

func (r *Runner) lastErrorString() string {
	r.errMu.RLock()
	defer r.errMu.RUnlock()
	return r.lastError
}

// CurrentMetadata returns the metadata corresponding to the current in-memory state.
func (r *Runner) CurrentMetadata() *RunMetadata {
	state := r.snapshotState()
	meta := r.buildMetadata(state, r.Status())
	meta.CreatedAt = r.createdAt
	meta.UpdatedAt = state.LastUpdate
	return meta
}

func (r *Runner) loop(ctx context.Context) {
	defer close(r.doneCh)

	for {
		select {
		case <-ctx.Done():
			r.handleStop(fmt.Errorf("context canceled: %w", ctx.Err()))
			return
		case <-r.stopCh:
			r.handleStop(nil)
			return
		case <-r.pauseCh:
			r.handlePause()
			<-r.resumeCh
			r.resumeFromPause()
		default:
		}

		err := r.stepOnce()
		if errors.Is(err, errBacktestCompleted) {
			r.handleCompletion()
			return
		}
		if errors.Is(err, errLiquidated) {
			r.handleLiquidation()
			return
		}
		if err != nil {
			r.handleFailure(err)
			return
		}
	}
}

func (r *Runner) stepOnce() error {
	state := r.snapshotState()
	if state.BarIndex >= r.feed.DecisionBarCount() {
		return errBacktestCompleted
	}

	ts := r.feed.DecisionTimestamp(state.BarIndex)

	marketData, multiTF, err := r.feed.BuildMarketData(ts)
	if err != nil {
		return err
	}

	priceMap := make(map[string]float64, len(marketData))
	for symbol, data := range marketData {
		priceMap[symbol] = data.CurrentPrice
	}

	// Event order: orders signalled on the prior close execute at this bar's
	// open, then intrabar protection/liquidation is evaluated, and only after
	// the close do we generate the next signal.
	tradeEvents, err := r.executePendingAtOpen(ts, priceMap, state.DecisionCycle)
	if err != nil {
		return err
	}
	fundingEvents := r.applyFunding(ts, state.DecisionCycle)
	tradeEvents = append(tradeEvents, fundingEvents...)
	protectionEvents, protectionNote, err := r.checkBarExits(ts, state.DecisionCycle)
	if err != nil {
		return err
	}
	tradeEvents = append(tradeEvents, protectionEvents...)

	callCount := state.DecisionCycle + 1
	shouldDecide := r.shouldTriggerDecision(state.BarIndex)

	var (
		record          *store.DecisionRecord
		decisionActions []store.DecisionAction
		execLog         []string
		hadError        bool
	)

	decisionAttempted := shouldDecide

	if shouldDecide {
		ctx, rec, err := r.buildDecisionContext(ts, marketData, multiTF, priceMap, callCount)
		if err != nil {
			rec.Success = false
			rec.ErrorMessage = fmt.Sprintf("failed to build trading context: %v", err)
			_ = r.logDecision(rec)
			return err
		}
		record = rec

		var (
			fullDecision *decision.FullDecision
			fromCache    bool
			cacheKey     string
		)
		if r.aiCache != nil {
			if key, err := computeCacheKey(ctx, r.cfg.PromptVariant, ts); err == nil {
				cacheKey = key
				if cached, ok := r.aiCache.Get(cacheKey); ok {
					fullDecision = cached
					fromCache = true
				} else if r.cfg.ReplayOnly {
					decisionErr := fmt.Errorf("replay_only enabled but cache miss at %d", ts)
					record.Success = false
					record.ErrorMessage = fmt.Sprintf("cached decision not found for ts=%d", ts)
					_ = r.logDecision(record)
					return decisionErr
				}
			} else {
				logger.Infof("failed to compute ai cache key: %v", err)
			}
		}

		if !fromCache {
			fd, err := r.invokeAIWithRetry(ctx)
			if err != nil {
				decisionAttempted = true
				hadError = true
				record.Success = false
				record.ErrorMessage = fmt.Sprintf("AI decision failed: %v", err)
				execLog = append(execLog, fmt.Sprintf("⚠️ AI decision failed: %v", err))
				r.setLastError(err)
			} else {
				fullDecision = fd
				if r.cfg.CacheAI && r.aiCache != nil && cacheKey != "" {
					if err := r.aiCache.Put(cacheKey, r.cfg.PromptVariant, ts, fullDecision); err != nil {
						logger.Infof("failed to persist ai cache for %s: %v", r.cfg.RunID, err)
					}
				}
			}
		}

		if fullDecision != nil {
			r.fillDecisionRecord(record, fullDecision)

			sorted := sortDecisionsByPriority(fullDecision.Decisions)

			prevLogs := execLog
			decisionActions = make([]store.DecisionAction, 0, len(sorted))
			execLog = make([]string, 0, len(sorted)+len(prevLogs))
			if len(prevLogs) > 0 {
				execLog = append(execLog, prevLogs...)
			}

			for _, dec := range sorted {
				actionRecord, trades, logEntry, execErr := r.executeDecision(dec, priceMap, ts, callCount)
				if execErr != nil {
					actionRecord.Success = false
					actionRecord.Error = execErr.Error()
					hadError = true
					execLog = append(execLog, fmt.Sprintf("❌ %s %s: %v", dec.Symbol, dec.Action, execErr))
				} else {
					actionRecord.Success = true
					execLog = append(execLog, fmt.Sprintf("✓ %s %s", dec.Symbol, dec.Action))
				}
				if len(trades) > 0 {
					tradeEvents = append(tradeEvents, trades...)
				}
				if logEntry != "" {
					execLog = append(execLog, logEntry)
				}
				decisionActions = append(decisionActions, actionRecord)
			}
		}
	}

	cycleForLog := state.DecisionCycle
	if decisionAttempted {
		cycleForLog = callCount
	}

	liquidationEvents, liquidationNote, err := r.checkLiquidation(ts, priceMap, cycleForLog)
	if err != nil {
		if record != nil {
			record.Success = false
			record.ErrorMessage = err.Error()
			_ = r.logDecision(record)
		}
		return err
	}
	if len(liquidationEvents) > 0 {
		hadError = true
		tradeEvents = append(tradeEvents, liquidationEvents...)
		if record != nil {
			execLog = append(execLog, fmt.Sprintf("⚠️ Forced liquidation: %s", liquidationNote))
		}
	}
	if protectionNote != "" {
		execLog = append(execLog, protectionNote)
	}

	if record != nil {
		record.Decisions = decisionActions
		record.ExecutionLog = execLog
		record.Success = !hadError && liquidationNote == ""
		if liquidationNote != "" {
			record.ErrorMessage = liquidationNote
		}
	}

	equity, unrealized, _ := r.account.TotalEquity(priceMap)
	marginUsed := r.totalMarginUsed()

	r.updateState(ts, equity, unrealized, marginUsed, priceMap, decisionAttempted)

	snapshot := r.snapshotState()
	drawdownPct := 0.0
	if snapshot.MaxEquity > 0 {
		drawdownPct = ((snapshot.MaxEquity - snapshot.Equity) / snapshot.MaxEquity) * 100
	}

	equityPoint := EquityPoint{
		Timestamp:   ts,
		Equity:      snapshot.Equity,
		Available:   snapshot.Cash,
		PnL:         snapshot.Equity - r.account.InitialBalance(),
		PnLPct:      ((snapshot.Equity - r.account.InitialBalance()) / r.account.InitialBalance()) * 100,
		DrawdownPct: drawdownPct,
		Cycle:       snapshot.DecisionCycle,
	}

	if err := appendEquityPoint(r.cfg.RunID, equityPoint); err != nil {
		return err
	}

	for _, evt := range tradeEvents {
		if err := appendTradeEvent(r.cfg.RunID, evt); err != nil {
			return err
		}
	}

	if record != nil {
		if err := r.logDecision(record); err != nil {
			return err
		}
	}

	if err := saveProgress(r.cfg.RunID, &snapshot, &r.cfg); err != nil {
		return err
	}

	if err := r.maybeCheckpoint(); err != nil {
		return err
	}

	r.persistMetadata()
	r.persistMetrics(false)

	if !hadError && liquidationNote == "" {
		r.setLastError(nil)
	}

	if snapshot.Liquidated {
		return errLiquidated
	}

	return nil
}

func (r *Runner) buildDecisionContext(ts int64, marketData map[string]*market.Data, multiTF map[string]map[string]*market.Data, priceMap map[string]float64, callCount int) (*decision.Context, *store.DecisionRecord, error) {
	equity, unrealized, _ := r.account.TotalEquity(priceMap)
	available := r.account.Cash()
	marginUsed := r.totalMarginUsed()
	marginPct := 0.0
	if equity > 0 {
		marginPct = (marginUsed / equity) * 100
	}

	accountInfo := decision.AccountInfo{
		TotalEquity:      equity,
		AvailableBalance: available,
		TotalPnL:         equity - r.account.InitialBalance(),
		TotalPnLPct:      ((equity - r.account.InitialBalance()) / r.account.InitialBalance()) * 100,
		MarginUsed:       marginUsed,
		MarginUsedPct:    marginPct,
		PositionCount:    len(r.account.Positions()),
	}

	positions := r.convertPositions(priceMap)

	candidateCoins := make([]decision.CandidateCoin, 0, len(r.cfg.Symbols))
	for _, sym := range r.cfg.Symbols {
		candidateCoins = append(candidateCoins, decision.CandidateCoin{Symbol: sym})
	}

	runtime := int((ts - int64(r.cfg.StartTS*1000)) / 60000)
	ctx := &decision.Context{
		CurrentTime:     time.UnixMilli(ts).UTC().Format("2006-01-02 15:04:05 UTC"),
		RuntimeMinutes:  runtime,
		CallCount:       callCount,
		Account:         accountInfo,
		Positions:       positions,
		CandidateCoins:  candidateCoins,
		PromptVariant:   r.cfg.PromptVariant,
		MarketDataMap:   marketData,
		MultiTFMarket:   multiTF,
		BTCETHLeverage:  r.cfg.Leverage.BTCETHLeverage,
		AltcoinLeverage: r.cfg.Leverage.AltcoinLeverage,
		Timeframes:      r.cfg.Timeframes,
	}

	record := &store.DecisionRecord{
		AccountState: store.AccountSnapshot{
			TotalBalance:          accountInfo.TotalEquity,
			AvailableBalance:      accountInfo.AvailableBalance,
			TotalUnrealizedProfit: unrealized,
			PositionCount:         accountInfo.PositionCount,
			MarginUsedPct:         accountInfo.MarginUsedPct,
		},
		CandidateCoins: make([]string, 0, len(candidateCoins)),
		Positions:      r.snapshotPositions(priceMap),
	}
	for _, coin := range candidateCoins {
		record.CandidateCoins = append(record.CandidateCoins, coin.Symbol)
	}
	record.Timestamp = time.UnixMilli(ts).UTC()

	return ctx, record, nil
}

func (r *Runner) fillDecisionRecord(record *store.DecisionRecord, full *decision.FullDecision) {
	record.InputPrompt = full.UserPrompt
	record.CoTTrace = full.CoTTrace
	if len(full.Decisions) > 0 {
		if data, err := json.MarshalIndent(full.Decisions, "", "  "); err == nil {
			record.DecisionJSON = string(data)
		}
	}
}

func (r *Runner) invokeAIWithRetry(ctx *decision.Context) (*decision.FullDecision, error) {
	var lastErr error
	for attempt := 0; attempt < aiDecisionMaxRetries; attempt++ {
		// Use GetFullDecisionWithStrategy with the pre-configured strategy engine
		// This ensures backtest uses the same unified prompt generation as live trading
		fd, err := decision.GetFullDecisionWithStrategy(
			ctx,
			r.mcpClient,
			r.strategyEngine,
			r.cfg.PromptVariant,
		)
		if err == nil {
			return fd, nil
		}
		lastErr = err
		delay := time.Duration(attempt+1) * 500 * time.Millisecond
		time.Sleep(delay)
	}
	return nil, lastErr
}

func (r *Runner) executeDecision(dec decision.Decision, priceMap map[string]float64, ts int64, cycle int) (store.DecisionAction, []TradeEvent, string, error) {
	action := store.DecisionAction{Action: dec.Action, Symbol: dec.Symbol, Leverage: r.resolveLeverage(dec.Leverage, dec.Symbol), Timestamp: time.UnixMilli(ts).UTC()}
	if dec.Action == "hold" || dec.Action == "wait" {
		return action, nil, fmt.Sprintf("hold position: %s", dec.Action), nil
	}
	if dec.Action != "open_long" && dec.Action != "open_short" && dec.Action != "close_long" && dec.Action != "close_short" {
		return action, nil, "", fmt.Errorf("unsupported action %s", dec.Action)
	}
	if dec.Action == "open_long" || dec.Action == "open_short" {
		if err := r.prepareEntryDecision(&dec, priceMap, ts); err != nil {
			return action, nil, "", err
		}
		action.Leverage = dec.Leverage
	}
	// Signals are created at the close and become executable events on the next
	// bar. This prevents future prices from being booked at the signal time.
	r.stateMu.Lock()
	r.state.PendingOrders = append(r.state.PendingOrders, PendingOrder{Decision: dec, SignalTimestamp: ts, Cycle: cycle})
	r.stateMu.Unlock()
	return action, nil, "queued for next bar open", nil
}

func (r *Runner) prepareEntryDecision(dec *decision.Decision, priceMap map[string]float64, ts int64) error {
	marketData, _, err := r.feed.BuildMarketData(ts)
	if err != nil {
		return fmt.Errorf("build sizing inputs: %w", err)
	}
	data := marketData[dec.Symbol]
	atr, latestVolume := decision.ConservativeATRAndVolume(data)
	equity, _, _ := r.account.TotalEquity(priceMap)
	currentNotional := 0.0
	for _, pos := range r.account.Positions() {
		currentNotional += pos.Quantity * pos.EntryPrice
	}
	risk := r.strategyEngine.GetRiskControlConfig()
	remaining := math.Inf(1)
	if risk.MaxTotalPositionSize > 0 {
		remaining = math.Max(risk.MaxTotalPositionSize-currentNotional, 0)
	}
	r.stateMu.RLock()
	pendingEntries := 0
	for _, order := range r.state.PendingOrders {
		if order.Decision.Action == "open_long" || order.Decision.Action == "open_short" {
			pendingEntries++
		}
	}
	r.stateMu.RUnlock()
	plan, err := decision.CalculateEntryPlan(decision.EntrySizingInput{
		Action: dec.Action, Equity: equity, AvailableBalance: r.account.Cash(), EntryPrice: priceMap[dec.Symbol], StopLoss: dec.StopLoss,
		ATR: atr, LatestBaseVolume: latestVolume, ExistingPositionCount: len(r.account.Positions()) + pendingEntries,
		MaxLeverage: r.resolveLeverage(0, dec.Symbol), MinPositionSize: risk.MinPositionSize, MaxPositionSize: risk.MaxPositionSize, RemainingNotional: remaining,
	})
	if err != nil {
		return fmt.Errorf("backend sizing: %w", err)
	}
	dec.PositionSizeUSD, dec.Leverage, dec.RiskUSD = plan.PositionSizeUSD, plan.Leverage, plan.ActualRiskUSD
	return nil
}

func (r *Runner) executeDecisionImmediate(dec decision.Decision, priceMap map[string]float64, ts int64, cycle int) (store.DecisionAction, []TradeEvent, string, error) {
	symbol := dec.Symbol
	usedLeverage := r.resolveLeverage(dec.Leverage, symbol)
	actionRecord := store.DecisionAction{
		Action:    dec.Action,
		Symbol:    symbol,
		Leverage:  usedLeverage,
		Timestamp: time.UnixMilli(ts).UTC(),
	}

	basePrice := priceMap[symbol]
	if basePrice <= 0 {
		return actionRecord, nil, "", fmt.Errorf("price unavailable for %s", symbol)
	}
	fillPrice := r.executionPrice(symbol, basePrice, ts)
	if dec.Action == "open_long" || dec.Action == "open_short" {
		risk := r.strategyEngine.GetRiskControlConfig()
		if err := decision.ValidatePositionCount(len(r.account.Positions()), risk); err != nil {
			return actionRecord, nil, "", err
		}
		if err := decision.ValidateMinimumPositionSize(dec.PositionSizeUSD, risk); err != nil {
			return actionRecord, nil, "", err
		}
	}

	switch dec.Action {
	case "open_long":
		qty := r.determineQuantity(dec, basePrice)
		if qty <= 0 {
			return actionRecord, nil, "", fmt.Errorf("invalid qty")
		}
		if err := decision.ValidateEntryRisk(&dec, fillPrice, qty, r.strategyEngine.GetRiskControlConfig().MinRiskRewardRatio); err != nil {
			return actionRecord, nil, "", err
		}
		cost := r.executionCost(symbol, ts, fillPrice*qty)
		pos, fee, execPrice, err := r.account.OpenWithCost(symbol, "long", qty, usedLeverage, fillPrice, ts, cost, dec.StopLoss, dec.TakeProfit)
		if err != nil {
			return actionRecord, nil, "", err
		}
		actionRecord.Quantity = qty
		actionRecord.Price = execPrice
		actionRecord.Leverage = pos.Leverage
		trade := TradeEvent{
			Timestamp:     ts,
			Symbol:        symbol,
			Action:        dec.Action,
			Side:          "long",
			Quantity:      qty,
			Price:         execPrice,
			Fee:           fee,
			Slippage:      execPrice - basePrice,
			OrderValue:    execPrice * qty,
			RealizedPnL:   0,
			Leverage:      pos.Leverage,
			Cycle:         cycle,
			PositionAfter: pos.Quantity,
		}
		return actionRecord, []TradeEvent{trade}, "", nil

	case "open_short":
		qty := r.determineQuantity(dec, basePrice)
		if qty <= 0 {
			return actionRecord, nil, "", fmt.Errorf("invalid qty")
		}
		if err := decision.ValidateEntryRisk(&dec, fillPrice, qty, r.strategyEngine.GetRiskControlConfig().MinRiskRewardRatio); err != nil {
			return actionRecord, nil, "", err
		}
		cost := r.executionCost(symbol, ts, fillPrice*qty)
		pos, fee, execPrice, err := r.account.OpenWithCost(symbol, "short", qty, usedLeverage, fillPrice, ts, cost, dec.StopLoss, dec.TakeProfit)
		if err != nil {
			return actionRecord, nil, "", err
		}
		actionRecord.Quantity = qty
		actionRecord.Price = execPrice
		actionRecord.Leverage = pos.Leverage
		trade := TradeEvent{
			Timestamp:     ts,
			Symbol:        symbol,
			Action:        dec.Action,
			Side:          "short",
			Quantity:      qty,
			Price:         execPrice,
			Fee:           fee,
			Slippage:      basePrice - execPrice,
			OrderValue:    execPrice * qty,
			RealizedPnL:   0,
			Leverage:      pos.Leverage,
			Cycle:         cycle,
			PositionAfter: pos.Quantity,
		}
		return actionRecord, []TradeEvent{trade}, "", nil

	case "close_long":
		qty := r.determineCloseQuantity(symbol, "long", dec)
		if qty <= 0 {
			return actionRecord, nil, "", fmt.Errorf("invalid close qty")
		}
		posLev := r.account.positionLeverage(symbol, "long")
		cost := r.executionCost(symbol, ts, fillPrice*qty)
		realized, fee, execPrice, err := r.account.CloseWithCost(symbol, "long", qty, fillPrice, cost)
		if err != nil {
			return actionRecord, nil, "", err
		}
		actionRecord.Quantity = qty
		actionRecord.Price = execPrice
		actionRecord.Leverage = posLev
		trade := TradeEvent{
			Timestamp:     ts,
			Symbol:        symbol,
			Action:        dec.Action,
			Side:          "long",
			Quantity:      qty,
			Price:         execPrice,
			Fee:           fee,
			Slippage:      basePrice - execPrice,
			OrderValue:    execPrice * qty,
			RealizedPnL:   realized - fee,
			Leverage:      posLev,
			Cycle:         cycle,
			PositionAfter: r.remainingPosition(symbol, "long"),
		}
		return actionRecord, []TradeEvent{trade}, "", nil

	case "close_short":
		qty := r.determineCloseQuantity(symbol, "short", dec)
		if qty <= 0 {
			return actionRecord, nil, "", fmt.Errorf("invalid close qty")
		}
		posLev := r.account.positionLeverage(symbol, "short")
		cost := r.executionCost(symbol, ts, fillPrice*qty)
		realized, fee, execPrice, err := r.account.CloseWithCost(symbol, "short", qty, fillPrice, cost)
		if err != nil {
			return actionRecord, nil, "", err
		}
		actionRecord.Quantity = qty
		actionRecord.Price = execPrice
		actionRecord.Leverage = posLev
		trade := TradeEvent{
			Timestamp:     ts,
			Symbol:        symbol,
			Action:        dec.Action,
			Side:          "short",
			Quantity:      qty,
			Price:         execPrice,
			Fee:           fee,
			Slippage:      execPrice - basePrice,
			OrderValue:    execPrice * qty,
			RealizedPnL:   realized - fee,
			Leverage:      posLev,
			Cycle:         cycle,
			PositionAfter: r.remainingPosition(symbol, "short"),
		}
		return actionRecord, []TradeEvent{trade}, "", nil

	case "hold", "wait":
		return actionRecord, nil, fmt.Sprintf("hold position: %s", dec.Action), nil
	default:
		return actionRecord, nil, "", fmt.Errorf("unsupported action %s", dec.Action)
	}
}

func (r *Runner) executePendingAtOpen(ts int64, closePrices map[string]float64, cycle int) ([]TradeEvent, error) {
	r.stateMu.Lock()
	pending := append([]PendingOrder(nil), r.state.PendingOrders...)
	r.state.PendingOrders = nil
	r.stateMu.Unlock()
	if len(pending) == 0 {
		return nil, nil
	}

	openPrices := make(map[string]float64, len(closePrices))
	for symbol, closePrice := range closePrices {
		openPrices[symbol] = closePrice
		if bar := r.feed.DecisionBar(symbol, ts); bar != nil && bar.Open > 0 {
			openPrices[symbol] = bar.Open
		}
	}
	events := make([]TradeEvent, 0, len(pending))
	for _, order := range pending {
		_, trades, _, err := r.executeDecisionImmediate(order.Decision, openPrices, ts, order.Cycle)
		if err != nil {
			events = append(events, TradeEvent{Timestamp: ts, Symbol: order.Decision.Symbol, Action: "rejected", Cycle: order.Cycle, Note: err.Error()})
			continue
		}
		events = append(events, trades...)
	}
	return events, nil
}

func (r *Runner) executionCost(symbol string, ts int64, notional float64) ExecutionCost {
	rate := r.cfg.SlippageBps / 10000
	if bar := r.feed.DecisionBar(symbol, ts); bar != nil {
		if bar.Open > 0 && r.cfg.VolatilitySlippage > 0 {
			rate += r.cfg.VolatilitySlippage * (bar.High - bar.Low) / bar.Open
		}
		volumeUSD := bar.QuoteVolume
		if volumeUSD <= 0 {
			volumeUSD = bar.Volume * bar.Open
		}
		if volumeUSD > 0 && r.cfg.ImpactBps > 0 && notional > 0 {
			rate += r.cfg.ImpactBps / 10000 * math.Sqrt(notional/volumeUSD)
		}
	}
	if rate > 0.05 {
		rate = 0.05
	}
	feeBps := r.cfg.TakerFeeBps
	if feeBps <= 0 {
		feeBps = r.cfg.FeeBps
	}
	return ExecutionCost{FeeRate: feeBps / 10000, SlippageRate: rate}
}

func (r *Runner) checkBarExits(ts int64, cycle int) ([]TradeEvent, string, error) {
	positions := append([]*position(nil), r.account.Positions()...)
	events := make([]TradeEvent, 0)
	notes := make([]string, 0)
	for _, pos := range positions {
		bar := r.feed.DecisionBar(pos.Symbol, ts)
		if bar == nil {
			continue
		}
		action, triggerPrice := "", 0.0
		liquidated := false
		if pos.Side == "long" {
			if pos.StopLoss > 0 && bar.Open <= pos.StopLoss {
				action, triggerPrice = "stop_loss", bar.Open
			} else if pos.TakeProfit > 0 && bar.Open >= pos.TakeProfit {
				action, triggerPrice = "take_profit", bar.Open
			} else {
				stopHit := pos.StopLoss > 0 && bar.Low <= pos.StopLoss
				takeHit := pos.TakeProfit > 0 && bar.High >= pos.TakeProfit
				if stopHit {
					action, triggerPrice = "stop_loss", pos.StopLoss // conservative when both hit
				} else if takeHit {
					action, triggerPrice = "take_profit", pos.TakeProfit
				} else if pos.LiquidationPrice > 0 && bar.Low <= pos.LiquidationPrice {
					action, triggerPrice, liquidated = "liquidated", pos.LiquidationPrice, true
				}
			}
		} else {
			if pos.StopLoss > 0 && bar.Open >= pos.StopLoss {
				action, triggerPrice = "stop_loss", bar.Open
			} else if pos.TakeProfit > 0 && bar.Open <= pos.TakeProfit {
				action, triggerPrice = "take_profit", bar.Open
			} else {
				stopHit := pos.StopLoss > 0 && bar.High >= pos.StopLoss
				takeHit := pos.TakeProfit > 0 && bar.Low <= pos.TakeProfit
				if stopHit {
					action, triggerPrice = "stop_loss", pos.StopLoss
				} else if takeHit {
					action, triggerPrice = "take_profit", pos.TakeProfit
				} else if pos.LiquidationPrice > 0 && bar.High >= pos.LiquidationPrice {
					action, triggerPrice, liquidated = "liquidated", pos.LiquidationPrice, true
				}
			}
		}
		if action == "" {
			continue
		}
		cost := r.executionCost(pos.Symbol, ts, triggerPrice*pos.Quantity)
		realized, fee, fill, err := r.account.CloseWithCost(pos.Symbol, pos.Side, pos.Quantity, triggerPrice, cost)
		if err != nil {
			return nil, "", err
		}
		events = append(events, TradeEvent{Timestamp: ts, Symbol: pos.Symbol, Action: action, Side: pos.Side, Quantity: pos.Quantity, Price: fill, Fee: fee, Slippage: math.Abs(fill - triggerPrice), OrderValue: fill * pos.Quantity, RealizedPnL: realized - fee, Leverage: pos.Leverage, Cycle: cycle, PositionAfter: 0, LiquidationFlag: liquidated, Note: fmt.Sprintf("%s triggered at %.8f", action, triggerPrice)})
		notes = append(notes, fmt.Sprintf("%s %s %s", pos.Symbol, pos.Side, action))
		if liquidated {
			r.stateMu.Lock()
			r.state.Liquidated = true
			r.state.LiquidationNote = notes[len(notes)-1]
			r.stateMu.Unlock()
		}
	}
	return events, strings.Join(notes, "; "), nil
}

func (r *Runner) applyFunding(ts int64, cycle int) []TradeEvent {
	interval := int64(time.Duration(r.cfg.FundingIntervalHours) * time.Hour / time.Millisecond)
	if interval <= 0 || r.cfg.FundingRateBps == 0 {
		return nil
	}
	r.stateMu.Lock()
	if r.state.NextFundingTS == 0 {
		start := r.cfg.StartTS * 1000
		r.state.NextFundingTS = ((start + interval - 1) / interval) * interval
	}
	next := r.state.NextFundingTS
	r.stateMu.Unlock()
	if next > ts {
		return nil
	}
	marks := make(map[string]float64)
	for _, pos := range r.account.Positions() {
		if bar := r.feed.DecisionBar(pos.Symbol, ts); bar != nil {
			marks[pos.Symbol] = bar.Open
		}
	}
	events := make([]TradeEvent, 0)
	for next <= ts {
		for _, pos := range append([]*position(nil), r.account.Positions()...) {
			mark := marks[pos.Symbol]
			if mark <= 0 {
				mark = pos.EntryPrice
			}
			payment := mark * pos.Quantity * r.cfg.FundingRateBps / 10000
			if pos.Side == "short" {
				payment = -payment
			}
			events = append(events, TradeEvent{Timestamp: next, Symbol: pos.Symbol, Action: "funding", Side: pos.Side, Quantity: pos.Quantity, Price: mark, RealizedPnL: -payment, Cycle: cycle, PositionAfter: pos.Quantity, Note: fmt.Sprintf("funding rate %.4f bps", r.cfg.FundingRateBps)})
		}
		r.account.ApplyFunding(marks, r.cfg.FundingRateBps/10000)
		next += interval
	}
	r.stateMu.Lock()
	r.state.NextFundingTS = next
	r.stateMu.Unlock()
	return events
}

func (r *Runner) determineQuantity(dec decision.Decision, price float64) float64 {
	snapshot := r.snapshotState()
	equity := snapshot.Equity
	if equity <= 0 {
		equity = r.account.InitialBalance()
	}
	sizeUSD := dec.PositionSizeUSD
	if sizeUSD <= 0 {
		sizeUSD = 0.05 * equity
	}
	currentNotional := 0.0
	for _, pos := range r.account.Positions() {
		currentNotional += pos.Quantity * pos.EntryPrice
	}
	sizeUSD = decision.CapPositionSize(sizeUSD, currentNotional, r.strategyEngine.GetRiskControlConfig())
	qty := sizeUSD / price
	if qty < 0 {
		qty = 0
	}
	return qty
}

func (r *Runner) determineCloseQuantity(symbol, side string, dec decision.Decision) float64 {
	for _, pos := range r.account.Positions() {
		if pos.Symbol == strings.ToUpper(symbol) && pos.Side == side {
			return pos.Quantity
		}
	}
	return 0
}

func (r *Runner) resolveLeverage(requested int, symbol string) int {
	if requested > 0 {
		return requested
	}
	sym := strings.ToUpper(symbol)
	if sym == "BTCUSDT" || sym == "ETHUSDT" {
		if r.cfg.Leverage.BTCETHLeverage > 0 {
			return r.cfg.Leverage.BTCETHLeverage
		}
	} else {
		if r.cfg.Leverage.AltcoinLeverage > 0 {
			return r.cfg.Leverage.AltcoinLeverage
		}
	}
	return 5
}

func (r *Runner) remainingPosition(symbol, side string) float64 {
	for _, pos := range r.account.Positions() {
		if pos.Symbol == strings.ToUpper(symbol) && pos.Side == side {
			return pos.Quantity
		}
	}
	return 0
}

func (r *Runner) snapshotPositions(priceMap map[string]float64) []store.PositionSnapshot {
	positions := r.account.Positions()
	list := make([]store.PositionSnapshot, 0, len(positions))
	for _, pos := range positions {
		price := priceMap[pos.Symbol]
		list = append(list, store.PositionSnapshot{
			Symbol:           pos.Symbol,
			Side:             pos.Side,
			PositionAmt:      pos.Quantity,
			EntryPrice:       pos.EntryPrice,
			MarkPrice:        price,
			UnrealizedProfit: unrealizedPnL(pos, price),
			Leverage:         float64(pos.Leverage),
			LiquidationPrice: pos.LiquidationPrice,
		})
	}
	return list
}

func (r *Runner) convertPositions(priceMap map[string]float64) []decision.PositionInfo {
	positions := r.account.Positions()
	list := make([]decision.PositionInfo, 0, len(positions))
	for _, pos := range positions {
		price := priceMap[pos.Symbol]
		list = append(list, decision.PositionInfo{
			Symbol:           pos.Symbol,
			Side:             pos.Side,
			EntryPrice:       pos.EntryPrice,
			MarkPrice:        price,
			Quantity:         pos.Quantity,
			Leverage:         pos.Leverage,
			UnrealizedPnL:    unrealizedPnL(pos, price),
			UnrealizedPnLPct: 0,
			LiquidationPrice: pos.LiquidationPrice,
			MarginUsed:       pos.Margin,
			UpdateTime:       time.Now().UnixMilli(),
		})
	}
	return list
}

func (r *Runner) executionPrice(symbol string, markPrice float64, ts int64) float64 {
	curr, _ := r.feed.decisionBarSnapshot(symbol, ts)
	if curr != nil && curr.Open > 0 {
		return curr.Open
	}
	return markPrice
}

func (r *Runner) totalMarginUsed() float64 {
	sum := 0.0
	for _, pos := range r.account.Positions() {
		sum += pos.Margin
	}
	return sum
}

func (r *Runner) updateState(ts int64, equity, unrealized, marginUsed float64, priceMap map[string]float64, advancedDecision bool) {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()

	if r.state.MaxEquity == 0 || equity > r.state.MaxEquity {
		r.state.MaxEquity = equity
	}
	if r.state.MinEquity == 0 || equity < r.state.MinEquity {
		r.state.MinEquity = equity
	}
	if r.state.MaxEquity > 0 {
		drawdown := ((r.state.MaxEquity - equity) / r.state.MaxEquity) * 100
		if drawdown > r.state.MaxDrawdownPct {
			r.state.MaxDrawdownPct = drawdown
		}
	}

	positions := make(map[string]PositionSnapshot)
	for _, pos := range r.account.Positions() {
		key := fmt.Sprintf("%s:%s", pos.Symbol, pos.Side)
		positions[key] = PositionSnapshot{
			Symbol:           pos.Symbol,
			Side:             pos.Side,
			Quantity:         pos.Quantity,
			AvgPrice:         pos.EntryPrice,
			Leverage:         pos.Leverage,
			LiquidationPrice: pos.LiquidationPrice,
			MarginUsed:       pos.Margin,
			OpenTime:         pos.OpenTime,
			StopLoss:         pos.StopLoss,
			TakeProfit:       pos.TakeProfit,
			MaintenanceRate:  pos.MaintenanceRate,
		}
	}

	r.state.BarTimestamp = ts
	r.state.BarIndex++
	if advancedDecision {
		r.state.DecisionCycle++
	}
	r.state.Cash = r.account.Cash()
	r.state.Equity = equity
	r.state.UnrealizedPnL = unrealized
	r.state.RealizedPnL = r.account.RealizedPnL()
	r.state.Positions = positions
	r.state.LastUpdate = time.Now().UTC()
}

func (r *Runner) maybeCheckpoint() error {
	state := r.snapshotState()
	shouldCheckpoint := false

	if r.cfg.CheckpointIntervalBars > 0 && state.BarIndex > 0 && state.BarIndex%r.cfg.CheckpointIntervalBars == 0 {
		shouldCheckpoint = true
	}

	interval := time.Duration(r.cfg.CheckpointIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if time.Since(r.lastCheckpoint) >= interval {
		shouldCheckpoint = true
	}

	if !shouldCheckpoint {
		return nil
	}

	if err := r.saveCheckpoint(state); err != nil {
		return err
	}

	return nil
}

func (r *Runner) snapshotForCheckpoint(state BacktestState) []PositionSnapshot {
	res := make([]PositionSnapshot, 0, len(state.Positions))
	for _, pos := range state.Positions {
		res = append(res, pos)
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].Symbol == res[j].Symbol {
			return res[i].Side < res[j].Side
		}
		return res[i].Symbol < res[j].Symbol
	})
	return res
}

func (r *Runner) checkLiquidation(ts int64, priceMap map[string]float64, cycle int) ([]TradeEvent, string, error) {
	positions := append([]*position(nil), r.account.Positions()...)
	events := make([]TradeEvent, 0)
	var noteBuilder strings.Builder

	for _, pos := range positions {
		price := priceMap[pos.Symbol]
		liqPrice := pos.LiquidationPrice
		trigger := false
		execPrice := price
		if pos.Side == "long" {
			if price <= liqPrice && liqPrice > 0 {
				trigger = true
				execPrice = liqPrice
			}
		} else {
			if price >= liqPrice && liqPrice > 0 {
				trigger = true
				execPrice = liqPrice
			}
		}
		if !trigger {
			continue
		}

		realized, fee, finalPrice, err := r.account.Close(pos.Symbol, pos.Side, pos.Quantity, execPrice)
		if err != nil {
			return nil, "", err
		}

		noteBuilder.WriteString(fmt.Sprintf("%s %s @ %.4f; ", pos.Symbol, pos.Side, finalPrice))

		evt := TradeEvent{
			Timestamp:       ts,
			Symbol:          pos.Symbol,
			Action:          "liquidated",
			Side:            pos.Side,
			Quantity:        pos.Quantity,
			Price:           finalPrice,
			Fee:             fee,
			Slippage:        0,
			OrderValue:      finalPrice * pos.Quantity,
			RealizedPnL:     realized - fee,
			Leverage:        pos.Leverage,
			Cycle:           cycle,
			PositionAfter:   0,
			LiquidationFlag: true,
			Note:            fmt.Sprintf("forced liquidation at %.4f", finalPrice),
		}
		events = append(events, evt)
	}

	if len(events) == 0 {
		return events, "", nil
	}

	note := strings.TrimSuffix(noteBuilder.String(), "; ")

	r.stateMu.Lock()
	r.state.Liquidated = true
	r.state.LiquidationNote = note
	r.stateMu.Unlock()

	return events, note, nil
}

func (r *Runner) shouldTriggerDecision(barIndex int) bool {
	if r.cfg.DecisionCadenceNBars <= 1 {
		return true
	}
	if barIndex < 0 {
		return true
	}
	return barIndex%r.cfg.DecisionCadenceNBars == 0
}

func (r *Runner) handleStop(reason error) {
	r.forceCheckpoint()
	if reason != nil {
		r.setLastError(reason)
	} else {
		r.setLastError(nil)
	}
	r.statusMu.Lock()
	r.err = reason
	r.status = RunStateStopped
	r.statusMu.Unlock()
	r.persistMetadata()
	r.persistMetrics(true)
	r.releaseLock()
}

func (r *Runner) handlePause() {
	r.forceCheckpoint()
	r.setLastError(nil)
	r.statusMu.Lock()
	r.status = RunStatePaused
	r.statusMu.Unlock()
	r.persistMetadata()
	r.persistMetrics(true)
}

func (r *Runner) resumeFromPause() {
	r.setLastError(nil)
	r.statusMu.Lock()
	r.status = RunStateRunning
	r.statusMu.Unlock()
	r.persistMetadata()
}

func (r *Runner) handleCompletion() {
	r.setLastError(nil)
	r.statusMu.Lock()
	r.status = RunStateCompleted
	r.statusMu.Unlock()
	r.persistMetadata()
	r.persistMetrics(true)
	r.releaseLock()
}

func (r *Runner) handleFailure(err error) {
	r.forceCheckpoint()
	if err != nil {
		r.setLastError(err)
	}
	r.statusMu.Lock()
	r.err = err
	r.status = RunStateFailed
	r.statusMu.Unlock()
	r.persistMetadata()
	r.persistMetrics(true)
	r.releaseLock()
}

func (r *Runner) handleLiquidation() {
	r.forceCheckpoint()
	r.setLastError(errLiquidated)
	r.statusMu.Lock()
	r.err = errLiquidated
	r.status = RunStateLiquidated
	r.statusMu.Unlock()
	r.persistMetadata()
	r.persistMetrics(true)
	r.releaseLock()
}

func (r *Runner) Pause() {
	select {
	case r.pauseCh <- struct{}{}:
	default:
	}
}

func (r *Runner) Resume() {
	select {
	case r.resumeCh <- struct{}{}:
	default:
	}
}

func (r *Runner) Stop() {
	select {
	case r.stopCh <- struct{}{}:
	default:
	}
}

func (r *Runner) Wait() error {
	<-r.doneCh
	r.statusMu.RLock()
	defer r.statusMu.RUnlock()
	return r.err
}

// Status returns the current run state.
func (r *Runner) Status() RunState {
	r.statusMu.RLock()
	defer r.statusMu.RUnlock()
	return r.status
}

// StatusPayload builds the status response for the API.
func (r *Runner) StatusPayload() StatusPayload {
	snapshot := r.snapshotState()
	progress := progressPercent(snapshot, r.cfg)

	payload := StatusPayload{
		RunID:          r.cfg.RunID,
		State:          r.Status(),
		ProgressPct:    progress,
		ProcessedBars:  snapshot.BarIndex,
		CurrentTime:    snapshot.BarTimestamp,
		DecisionCycle:  snapshot.DecisionCycle,
		Equity:         snapshot.Equity,
		UnrealizedPnL:  snapshot.UnrealizedPnL,
		RealizedPnL:    snapshot.RealizedPnL,
		Note:           snapshot.LiquidationNote,
		LastError:      r.lastErrorString(),
		LastUpdatedIso: snapshot.LastUpdate.UTC().Format(time.RFC3339),
	}
	return payload
}

func (r *Runner) snapshotState() BacktestState {
	r.stateMu.RLock()
	defer r.stateMu.RUnlock()

	copyState := *r.state
	copyState.Positions = make(map[string]PositionSnapshot, len(r.state.Positions))
	for k, v := range r.state.Positions {
		copyState.Positions[k] = v
	}
	copyState.PendingOrders = append([]PendingOrder(nil), r.state.PendingOrders...)
	return copyState
}

func (r *Runner) persistMetadata() {
	state := r.snapshotState()
	meta := r.buildMetadata(state, r.Status())
	meta.CreatedAt = r.createdAt
	if err := SaveRunMetadata(meta); err != nil {
		logger.Infof("failed to save run metadata for %s: %v", r.cfg.RunID, err)
	} else {
		if err := updateRunIndex(meta, &r.cfg); err != nil {
			logger.Infof("failed to update index for %s: %v", r.cfg.RunID, err)
		}
	}
}

func (r *Runner) logDecision(record *store.DecisionRecord) error {
	if record == nil {
		return nil
	}
	persistDecisionRecord(r.cfg.RunID, record)
	return nil
}

func (r *Runner) persistMetrics(force bool) {
	if r.cfg.RunID == "" {
		return
	}

	if !force && !r.lastMetricsWrite.IsZero() {
		if time.Since(r.lastMetricsWrite) < metricsWriteInterval {
			return
		}
	}

	state := r.snapshotState()
	metrics, err := CalculateMetrics(r.cfg.RunID, &r.cfg, &state)
	if err != nil {
		logger.Infof("failed to compute metrics for %s: %v", r.cfg.RunID, err)
		return
	}
	if metrics == nil {
		return
	}
	if err := PersistMetrics(r.cfg.RunID, metrics); err != nil {
		logger.Infof("failed to persist metrics for %s: %v", r.cfg.RunID, err)
		return
	}
	r.lastMetricsWrite = time.Now()
}

func (r *Runner) buildMetadata(state BacktestState, runState RunState) *RunMetadata {
	if state.Liquidated && runState != RunStateLiquidated {
		runState = RunStateLiquidated
	}

	progress := progressPercent(state, r.cfg)

	summary := RunSummary{
		SymbolCount:     len(r.cfg.Symbols),
		DecisionTF:      r.cfg.DecisionTimeframe,
		ProcessedBars:   state.BarIndex,
		ProgressPct:     progress,
		EquityLast:      state.Equity,
		MaxDrawdownPct:  state.MaxDrawdownPct,
		Liquidated:      state.Liquidated,
		LiquidationNote: state.LiquidationNote,
	}

	meta := &RunMetadata{
		RunID:     r.cfg.RunID,
		UserID:    r.cfg.UserID,
		State:     runState,
		LastError: r.lastErrorString(),
		Summary:   summary,
	}

	return meta
}

func progressPercent(state BacktestState, cfg BacktestConfig) float64 {
	duration := cfg.Duration()
	if duration <= 0 {
		return 0
	}
	if state.BarTimestamp == 0 {
		return 0
	}

	start := time.Unix(cfg.StartTS, 0)
	end := time.Unix(cfg.EndTS, 0)
	current := time.UnixMilli(state.BarTimestamp)

	if !current.After(start) {
		return 0
	}
	if current.After(end) {
		return 100
	}

	elapsed := current.Sub(start)
	pct := float64(elapsed) / float64(duration) * 100
	if pct > 100 {
		pct = 100
	}
	if pct < 0 {
		pct = 0
	}
	return pct
}

func (r *Runner) buildCheckpointFromState(state BacktestState) *Checkpoint {
	return &Checkpoint{
		BarIndex:        state.BarIndex,
		BarTimestamp:    state.BarTimestamp,
		Cash:            state.Cash,
		Equity:          state.Equity,
		UnrealizedPnL:   state.UnrealizedPnL,
		RealizedPnL:     state.RealizedPnL,
		Positions:       r.snapshotForCheckpoint(state),
		DecisionCycle:   state.DecisionCycle,
		Liquidated:      state.Liquidated,
		LiquidationNote: state.LiquidationNote,
		MaxEquity:       state.MaxEquity,
		MinEquity:       state.MinEquity,
		MaxDrawdownPct:  state.MaxDrawdownPct,
		AICacheRef:      r.cachePath,
		PendingOrders:   append([]PendingOrder(nil), state.PendingOrders...),
		NextFundingTS:   state.NextFundingTS,
	}
}

func (r *Runner) saveCheckpoint(state BacktestState) error {
	ckpt := r.buildCheckpointFromState(state)
	if ckpt == nil {
		return nil
	}
	if err := SaveCheckpoint(r.cfg.RunID, ckpt); err != nil {
		return err
	}
	r.lastCheckpoint = time.Now()
	return nil
}

func (r *Runner) forceCheckpoint() {
	state := r.snapshotState()
	if err := r.saveCheckpoint(state); err != nil {
		logger.Infof("failed to save checkpoint for %s: %v", r.cfg.RunID, err)
	}
}

func (r *Runner) RestoreFromCheckpoint() error {
	ckpt, err := LoadCheckpoint(r.cfg.RunID)
	if err != nil {
		return err
	}
	return r.applyCheckpoint(ckpt)
}

func (r *Runner) applyCheckpoint(ckpt *Checkpoint) error {
	if ckpt == nil {
		return fmt.Errorf("checkpoint is nil")
	}
	r.account.RestoreFromSnapshots(ckpt.Cash, ckpt.RealizedPnL, ckpt.Positions)
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	r.state.BarIndex = ckpt.BarIndex
	r.state.BarTimestamp = ckpt.BarTimestamp
	r.state.Cash = ckpt.Cash
	r.state.Equity = ckpt.Equity
	r.state.UnrealizedPnL = ckpt.UnrealizedPnL
	r.state.RealizedPnL = ckpt.RealizedPnL
	r.state.DecisionCycle = ckpt.DecisionCycle
	r.state.Liquidated = ckpt.Liquidated
	r.state.LiquidationNote = ckpt.LiquidationNote
	r.state.MaxEquity = ckpt.MaxEquity
	r.state.MinEquity = ckpt.MinEquity
	r.state.MaxDrawdownPct = ckpt.MaxDrawdownPct
	r.state.PendingOrders = append([]PendingOrder(nil), ckpt.PendingOrders...)
	r.state.NextFundingTS = ckpt.NextFundingTS
	r.state.Positions = snapshotsToMap(ckpt.Positions)
	r.state.LastUpdate = time.Now().UTC()
	r.lastCheckpoint = time.Now()
	return nil
}

func snapshotsToMap(snaps []PositionSnapshot) map[string]PositionSnapshot {
	positions := make(map[string]PositionSnapshot, len(snaps))
	for _, snap := range snaps {
		key := fmt.Sprintf("%s:%s", snap.Symbol, snap.Side)
		positions[key] = snap
	}
	return positions
}

func sortDecisionsByPriority(decisions []decision.Decision) []decision.Decision {
	if len(decisions) <= 1 {
		return decisions
	}

	priority := func(action string) int {
		switch action {
		case "close_long", "close_short":
			return 1
		case "open_long", "open_short":
			return 2
		case "hold", "wait":
			return 3
		default:
			return 99
		}
	}

	result := make([]decision.Decision, len(decisions))
	copy(result, decisions)

	sort.Slice(result, func(i, j int) bool {
		pi := priority(result[i].Action)
		pj := priority(result[j].Action)
		if pi != pj {
			return pi < pj
		}
		return i < j
	})

	return result
}

func barVWAP(k market.Kline) float64 {
	values := []float64{k.Open, k.High, k.Low, k.Close}
	sum := 0.0
	count := 0.0
	for _, v := range values {
		if v > 0 {
			sum += v
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / count
}
