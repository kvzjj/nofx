package trader

import (
	"fmt"
	"nofx/logger"
	"nofx/store"
	"sort"
	"strings"
	"sync"
	"time"
)

// PositionSyncManager Position status synchronization manager
// Responsible for periodically synchronizing exchange positions, detecting manual closures and other changes
type PositionSyncManager struct {
	store                *store.Store
	interval             time.Duration
	historySyncInterval  time.Duration // Interval for full history sync
	stopCh               chan struct{}
	wg                   sync.WaitGroup
	traderCache          map[string]Trader                  // trader_id -> Trader instance cache
	configCache          map[string]*store.TraderFullConfig // trader_id -> config cache
	cacheMutex           sync.RWMutex
	lastHistorySync      map[string]time.Time // trader_id -> last history sync time
	lastHistorySyncMutex sync.RWMutex
	accountMutexes       sync.Map // trader_id -> *sync.Mutex
}

// NewPositionSyncManager Create position synchronization manager
func NewPositionSyncManager(st *store.Store, interval time.Duration) *PositionSyncManager {
	if interval == 0 {
		interval = 10 * time.Second
	}
	return &PositionSyncManager{
		store:               st,
		interval:            interval,
		historySyncInterval: 5 * time.Minute, // Sync closed positions every 5 minutes
		stopCh:              make(chan struct{}),
		traderCache:         make(map[string]Trader),
		configCache:         make(map[string]*store.TraderFullConfig),
		lastHistorySync:     make(map[string]time.Time),
	}
}

// Start Start position synchronization service
func (m *PositionSyncManager) Start() {
	m.wg.Add(1)
	go m.run()
	logger.Info("📊 Position sync manager started")
}

// Stop Stop position synchronization service
func (m *PositionSyncManager) Stop() {
	close(m.stopCh)
	m.wg.Wait()

	// Clear cache
	m.cacheMutex.Lock()
	m.traderCache = make(map[string]Trader)
	m.configCache = make(map[string]*store.TraderFullConfig)
	m.cacheMutex.Unlock()

	logger.Info("📊 Position sync manager stopped")
}

// run Main loop
func (m *PositionSyncManager) run() {
	defer m.wg.Done()

	// Execute immediately on startup
	m.syncPositions()

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.syncPositions()
		}
	}
}

// syncPositions Synchronize all position statuses
func (m *PositionSyncManager) syncPositions() {
	traders, err := m.store.Trader().ListAll()
	if err != nil {
		logger.Infof("⚠️  Failed to list traders for reconciliation: %v", err)
		return
	}
	for _, traderInfo := range traders {
		m.syncTraderAccount(traderInfo.ID)
	}
}

// syncTraderPositions Synchronize positions for a single trader
func (m *PositionSyncManager) syncTraderPositions(traderID string, localPositions []*store.TraderPosition) {
	m.syncTraderAccountWithPositions(traderID, localPositions)
}

func (m *PositionSyncManager) syncTraderAccount(traderID string) {
	m.syncTraderAccountWithPositions(traderID, nil)
}

func (m *PositionSyncManager) syncTraderAccountWithPositions(traderID string, localPositions []*store.TraderPosition) {
	config, err := m.getTraderConfig(traderID)
	if err != nil {
		logger.Infof("⚠️  Failed to get trader config (ID: %s): %v", traderID, err)
		return
	}
	lockKey := config.Exchange.ID
	if lockKey == "" {
		lockKey = traderID
	}
	lockValue, _ := m.accountMutexes.LoadOrStore(lockKey, &sync.Mutex{})
	accountLock := lockValue.(*sync.Mutex)
	accountLock.Lock()
	defer accountLock.Unlock()
	if localPositions == nil {
		var err error
		localPositions, err = m.store.Position().GetOpenPositions(traderID)
		if err != nil {
			logger.Infof("⚠️  Failed to get local positions (ID: %s): %v", traderID, err)
			return
		}
	}

	// Get or create trader instance
	trader, err := m.getOrCreateTrader(traderID)
	if err != nil {
		logger.Infof("⚠️  Failed to get trader instance (ID: %s): %v", traderID, err)
		return
	}

	// Get exchange info for history sync
	exchangeID := config.Exchange.ID
	exchangeType := config.Exchange.ExchangeType

	// Maybe run periodic history sync
	if exchangeID != "" && exchangeType != "" {
		m.maybeRunHistorySync(traderID, exchangeID, exchangeType, trader)
	}

	// Get current exchange positions
	var exchangePositions []map[string]interface{}
	if freshReader, ok := trader.(FreshPositionReader); ok {
		exchangePositions, err = freshReader.GetPositionsFresh()
	} else {
		exchangePositions, err = trader.GetPositions()
	}
	if err != nil {
		logger.Infof("⚠️  Failed to get exchange positions (ID: %s): %v", traderID, err)
	} else {
		m.reconcilePositionSnapshot(traderID, exchangeID, exchangeType, trader, localPositions, exchangePositions)
	}

	var openOrders []ExchangeOpenOrder
	openOrdersAvailable := false
	if lister, ok := trader.(OpenOrderLister); ok {
		openOrders, err = lister.ListOpenOrders()
		if err != nil {
			logger.Infof("⚠️  Failed to list exchange open orders (ID: %s): %v", traderID, err)
		} else {
			openOrdersAvailable = true
		}
	}
	m.reconcileOrders(traderID, exchangeID, exchangeType, trader, openOrders, openOrdersAvailable)
	symbols := make([]string, 0, len(localPositions)+len(exchangePositions)+len(openOrders))
	for _, pos := range localPositions {
		symbols = append(symbols, pos.Symbol)
	}
	for _, pos := range exchangePositions {
		if symbol, _, _ := normalizeExchangePosition(pos); symbol != "" {
			symbols = append(symbols, symbol)
		}
	}
	for _, order := range openOrders {
		symbols = append(symbols, order.Symbol)
	}
	for _, symbol := range strings.FieldsFunc(config.Trader.TradingSymbols, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t'
	}) {
		symbols = append(symbols, strings.ToUpper(strings.TrimSpace(symbol)))
	}
	m.reconcileFills(traderID, exchangeID, trader, symbols)
	if openOrdersAvailable {
		currentPositions, err := m.store.Position().GetOpenPositions(traderID)
		if err != nil {
			logger.Infof("⚠️  Failed to reload positions for protection reconciliation (ID: %s): %v", traderID, err)
			return
		}
		m.reconcileProtections(traderID, exchangeID, trader, openOrders, currentPositions)
	}
}

func (m *PositionSyncManager) reconcileFills(traderID, exchangeID string, trader Trader, symbols []string) {
	lister, ok := trader.(RecentFillLister)
	if !ok {
		return
	}
	since, err := m.store.Execution().LastExchangeFillTime(traderID, exchangeID)
	if err != nil {
		logger.Infof("⚠️  Failed to get fill reconciliation cursor (ID: %s): %v", traderID, err)
		return
	}
	fills, err := lister.ListRecentFills(symbols, since.Add(-time.Minute))
	if err != nil {
		logger.Infof("⚠️  Failed to list recent exchange fills (ID: %s): %v", traderID, err)
		return
	}
	for _, fill := range fills {
		if fill.TradeID == "" || fill.Time.IsZero() {
			continue
		}
		if err := m.store.Execution().UpsertExchangeFill(store.ExchangeFill{
			TraderID: traderID, ExchangeID: exchangeID, TradeID: fill.TradeID,
			OrderID: fill.OrderID, Symbol: fill.Symbol, PositionSide: strings.ToUpper(fill.PositionSide),
			Side: strings.ToUpper(fill.Side), Quantity: fill.Quantity, Price: fill.Price,
			Fee: fill.Fee, RealizedPnL: fill.RealizedPnL, ExecutedAt: fill.Time,
		}); err != nil {
			logger.Infof("⚠️  Failed to persist exchange fill %s: %v", fill.TradeID, err)
		}
	}
}

func (m *PositionSyncManager) reconcilePositionSnapshot(traderID, exchangeID, exchangeType string, trader Trader, localPositions []*store.TraderPosition, exchangePositions []map[string]interface{}) {
	exchangeMap := make(map[string]map[string]interface{})
	for _, pos := range exchangePositions {
		symbol, side, qty := normalizeExchangePosition(pos)
		if symbol == "" || side == "" || qty < 0.0000001 {
			continue
		}
		exchangeMap[fmt.Sprintf("%s_%s", symbol, side)] = pos
	}
	for _, localPos := range localPositions {
		key := fmt.Sprintf("%s_%s", localPos.Symbol, localPos.Side)
		exchangePos, exists := exchangeMap[key]
		if !exists {
			// The exchange position is gone (SL/TP triggered or external
			// close): cancel any surviving sibling protective orders for this
			// side immediately instead of waiting for the orphan sweep in a
			// later cycle. This provides the OCO linkage that independent
			// conditional orders lack — a stale closePosition order must never
			// act on a future position on the same side.
			if canceler, ok := trader.(PositionOrderCanceler); ok {
				if err := canceler.CancelPositionOrders(localPos.Symbol, localPos.Side); err != nil {
					logger.Infof("⚠️  Failed to cancel sibling protection orders for %s %s: %v", localPos.Symbol, localPos.Side, err)
				}
			}
			m.closeLocalPosition(localPos, trader, "manual")
			continue
		}
		_, _, qty := normalizeExchangePosition(exchangePos)
		entryPrice := getFloatFromMap(exchangePos, "entryPrice")
		leverage := int(getFloatFromMap(exchangePos, "leverage"))
		if leverage <= 0 {
			leverage = localPos.Leverage
		}
		if entryPrice <= 0 {
			entryPrice = localPos.EntryPrice
		}
		if err := m.store.Position().UpdateOpenPositionProjection(localPos.ID, qty, entryPrice, leverage); err != nil {
			logger.Infof("⚠️  Failed to update exchange position projection %s %s: %v", localPos.Symbol, localPos.Side, err)
		}
		delete(exchangeMap, key)
	}
	for _, exchangePos := range exchangeMap {
		m.createExternalPosition(traderID, exchangeID, exchangeType, exchangePos)
	}
}

// closeLocalPosition Mark local position as closed
func (m *PositionSyncManager) closeLocalPosition(pos *store.TraderPosition, trader Trader, reason string) {
	// Try to get accurate closure data from exchange first
	closedPnLRecord := m.findClosedPnLRecord(trader, pos)

	var exitPrice, realizedPnL, fee float64
	var closeReason, exitOrderID string

	if closedPnLRecord != nil {
		// Use accurate data from exchange
		exitPrice = closedPnLRecord.ExitPrice
		realizedPnL = closedPnLRecord.RealizedPnL
		fee = closedPnLRecord.Fee
		closeReason = closedPnLRecord.CloseType
		exitOrderID = closedPnLRecord.OrderID
		logger.Infof("📊 Found accurate closure data from exchange for %s %s", pos.Symbol, pos.Side)
	} else {
		logger.Infof("⚠️  Deferring closure of %s %s until exchange fill/PnL data is available", pos.Symbol, pos.Side)
		return
	}

	err := m.store.Position().ClosePositionWithAccurateData(
		pos.ID,
		exitPrice,
		exitOrderID,
		closedPnLRecord.ExitTime,
		realizedPnL,
		fee,
		closeReason,
	)

	if err != nil {
		logger.Infof("⚠️  Failed to update position status: %v", err)
	} else {
		logger.Infof("📊 Position closed [%s] %s %s @ %.4f → %.4f, PnL: %.2f, Fee: %.4f (%s)",
			pos.TraderID[:8], pos.Symbol, pos.Side, pos.EntryPrice, exitPrice, realizedPnL, fee, closeReason)
	}
}

func normalizeExchangePosition(pos map[string]interface{}) (string, string, float64) {
	symbol, _ := pos["symbol"].(string)
	side, _ := pos["side"].(string)
	switch strings.ToUpper(side) {
	case "BUY", "LONG":
		side = "LONG"
	case "SELL", "SHORT":
		side = "SHORT"
	default:
		return symbol, "", 0
	}
	qty := getFloatFromMap(pos, "positionAmt")
	if qty < 0 {
		qty = -qty
	}
	return symbol, side, qty
}

func (m *PositionSyncManager) createExternalPosition(traderID, exchangeID, exchangeType string, pos map[string]interface{}) {
	symbol, side, qty := normalizeExchangePosition(pos)
	if symbol == "" || side == "" || qty < 0.0000001 {
		return
	}
	if exists, err := m.store.Position().HasOpenPositionForExchange(exchangeID, symbol, side); err != nil {
		logger.Infof("⚠️  Failed to check existing exchange position %s %s: %v", symbol, side, err)
		return
	} else if exists {
		return
	}
	entryPrice := getFloatFromMap(pos, "entryPrice")
	leverage := int(getFloatFromMap(pos, "leverage"))
	if leverage <= 0 {
		leverage = 1
	}
	createdTime := getFloatFromMap(pos, "createdTime")
	entryTime := time.Now()
	if createdTime > 0 {
		entryTime = time.UnixMilli(int64(createdTime))
	}
	newPos := &store.TraderPosition{
		TraderID: traderID, ExchangeID: exchangeID, ExchangeType: exchangeType,
		ExchangePositionID: fmt.Sprintf("%s_%s_%d", symbol, side, entryTime.UnixMilli()), Symbol: symbol,
		Side: side, Quantity: qty, EntryPrice: entryPrice, EntryTime: entryTime,
		Leverage: leverage, Source: "sync",
	}
	if err := m.store.Position().CreateOpenPosition(newPos); err != nil {
		logger.Infof("⚠️  Failed to create external position record: %v", err)
	} else {
		logger.Infof("📊 Reconciled external position: [%s] %s %s @ %.4f (qty: %.4f)", traderID[:8], symbol, side, entryPrice, qty)
	}
}

func (m *PositionSyncManager) reconcileOrders(traderID, exchangeID, exchangeType string, trader Trader, openOrders []ExchangeOpenOrder, openOrdersAvailable bool) {
	orders, err := m.store.Execution().ListActiveOrders(traderID, exchangeID)
	if err != nil {
		logger.Infof("⚠️  Failed to list active local orders (ID: %s): %v", traderID, err)
		return
	}
	localOrders := make(map[string]store.TradeOrder, len(orders))
	for _, order := range orders {
		localOrders[order.OrderID] = order
	}
	if openOrdersAvailable {
		for _, exchangeOrder := range openOrders {
			if exchangeOrder.Kind != "" {
				continue
			}
			order, exists := localOrders[exchangeOrder.OrderID]
			if !exists {
				order = store.TradeOrder{
					TraderID: traderID, ExchangeID: exchangeID, ExchangeType: exchangeType,
					OrderID: exchangeOrder.OrderID, Symbol: exchangeOrder.Symbol,
					PositionSide: strings.ToUpper(exchangeOrder.PositionSide), Action: "external",
					RequestedQty: exchangeOrder.Quantity,
				}
			}
			order.ExecutedQty = exchangeOrder.ExecutedQty
			order.AvgPrice = exchangeOrder.AvgPrice
			order.Fee = exchangeOrder.Fee
			order.Status = string(exchangeOrder.Status)
			order.LastError = ""
			if err := m.store.Execution().UpsertOrder(order); err != nil {
				logger.Infof("⚠️  Failed to import exchange order %s: %v", order.OrderID, err)
				continue
			}
			if err := m.store.Execution().RecordFill(order); err != nil {
				logger.Infof("⚠️  Failed to persist exchange order fill %s: %v", order.OrderID, err)
			}
			delete(localOrders, exchangeOrder.OrderID)
		}
	}
	for _, order := range localOrders {
		status, err := trader.GetOrderStatus(order.Symbol, order.OrderID)
		if err != nil {
			logger.Infof("⚠️  Failed to reconcile order %s: %v", order.OrderID, err)
			continue
		}
		order.ExchangeType = exchangeType
		order.Status = string(normalizeOrderState(fmt.Sprint(status["status"])))
		order.ExecutedQty = getFloatFromMap(status, "executedQty")
		order.AvgPrice = getFloatFromMap(status, "avgPrice")
		order.Fee = getFloatFromMap(status, "commission")
		order.LastError = ""
		if err := m.store.Execution().UpsertOrder(order); err != nil {
			logger.Infof("⚠️  Failed to persist reconciled order %s: %v", order.OrderID, err)
			continue
		}
		if err := m.store.Execution().RecordFill(order); err != nil {
			logger.Infof("⚠️  Failed to persist reconciled fill %s: %v", order.OrderID, err)
		}
	}
}

func (m *PositionSyncManager) reconcileProtections(traderID, exchangeID string, trader Trader, orders []ExchangeOpenOrder, localPositions []*store.TraderPosition) {
	type protectionState struct {
		stopQty, takeQty     float64
		stopClose, takeClose bool
	}
	states := make(map[string]protectionState)
	positionKeys := make(map[string]struct{}, len(localPositions))
	for _, pos := range localPositions {
		positionKeys[pos.Symbol+"_"+pos.Side] = struct{}{}
	}
	orphans := make(map[string]ExchangeOpenOrder)
	for _, order := range orders {
		if order.Kind == "" {
			continue
		}
		positionSide := strings.ToUpper(order.PositionSide)
		if positionSide != "LONG" && positionSide != "SHORT" {
			continue
		}
		key := order.Symbol + "_" + positionSide
		if _, exists := positionKeys[key]; !exists {
			// A shared exchange account may legitimately hold this position for
			// another trader; never cancel their protective orders from this
			// trader's reconciliation cycle.
			held, heldErr := m.store.Position().HasOpenPositionForExchange(exchangeID, order.Symbol, positionSide)
			if heldErr != nil {
				logger.Infof("⚠️  Failed to check shared-account positions for %s: %v", key, heldErr)
				continue
			}
			if held {
				continue
			}
			orphans[key] = order
			continue
		}
		state := states[key]
		if order.Kind == "STOP_LOSS" {
			state.stopQty += order.Quantity
			state.stopClose = state.stopClose || order.ClosePosition
		}
		if order.Kind == "TAKE_PROFIT" {
			state.takeQty += order.Quantity
			state.takeClose = state.takeClose || order.ClosePosition
		}
		states[key] = state
	}
	if canceler, ok := trader.(PositionOrderCanceler); ok {
		for key, order := range orphans {
			if err := canceler.CancelPositionOrders(order.Symbol, order.PositionSide); err != nil {
				logger.Infof("⚠️  Failed to cancel orphan protection orders %s: %v", key, err)
			} else {
				logger.Infof("📊 Canceled orphan protection orders %s", key)
			}
		}
	}
	for _, pos := range localPositions {
		if err := m.store.Execution().MarkProtectionsMissing(traderID, exchangeID, pos.Symbol, pos.Side); err != nil {
			logger.Infof("⚠️  Failed to clear stale protections for %s %s: %v", pos.Symbol, pos.Side, err)
		}
		state := states[pos.Symbol+"_"+pos.Side]
		if state.stopClose {
			state.stopQty = pos.Quantity
		}
		if state.takeClose {
			state.takeQty = pos.Quantity
		}
		status := "UNPROTECTED"
		tolerance := pos.Quantity*1e-6 + 1e-9
		// Quantity-scoped protections that over-cover a shrunken position
		// would be rejected at trigger time (order quantity exceeds the
		// remaining position). Rebalance them: cancel the over-covering leg and
		// let the intent repair below re-place an exact-size order.
		if !state.stopClose && state.stopQty > pos.Quantity+tolerance {
			m.rebalanceOverCoverage(trader, pos, "STOP_LOSS", &state.stopQty)
		}
		if !state.takeClose && state.takeQty > pos.Quantity+tolerance {
			m.rebalanceOverCoverage(trader, pos, "TAKE_PROFIT", &state.takeQty)
		}
		if state.stopQty+tolerance >= pos.Quantity && state.takeQty+tolerance >= pos.Quantity {
			status = "PROTECTED"
		}
		if err := m.store.Position().SetOpenPositionProtectionStatus(traderID, pos.Symbol, pos.Side, status); err != nil {
			logger.Infof("⚠️  Failed to derive protection status for %s %s: %v", pos.Symbol, pos.Side, err)
		}
		if pos.EntryOrderID == "" {
			entryOrder, findErr := m.store.Execution().FindEntryOrderForPosition(traderID, exchangeID, pos.Symbol, pos.Side)
			if findErr != nil {
				logger.Infof("⚠️  Failed to recover entry order for %s %s: %v", pos.Symbol, pos.Side, findErr)
			} else if entryOrder != nil {
				pos.EntryOrderID = entryOrder.OrderID
				if err := m.store.Position().AttachEntryOrderID(pos.ID, entryOrder.OrderID); err != nil {
					logger.Infof("⚠️  Failed to attach recovered entry order %s: %v", entryOrder.OrderID, err)
				}
			}
		}
		if pos.EntryOrderID == "" {
			if state.stopQty+tolerance < pos.Quantity {
				m.emergencyCloseUnprotected(traderID, exchangeID, trader, pos, "missing entry protection intent")
			}
			continue
		}
		for _, order := range orders {
			if order.Kind == "" || order.Symbol != pos.Symbol || !strings.EqualFold(order.PositionSide, pos.Side) {
				continue
			}
			protectionQty := order.Quantity
			if order.ClosePosition {
				protectionQty = pos.Quantity
			}
			if err := m.store.Execution().UpsertReconciledProtection(traderID, exchangeID, pos.EntryOrderID, order.OrderID, pos.Symbol, pos.Side, order.Kind, protectionQty, order.TriggerPrice); err != nil {
				logger.Infof("⚠️  Failed to reconcile protection order %s: %v", order.OrderID, err)
			}
		}
		intents, err := m.store.Execution().ListProtectionOrders(traderID, exchangeID, pos.EntryOrderID)
		if err != nil {
			logger.Infof("⚠️  Failed to load protection intents for %s %s: %v", pos.Symbol, pos.Side, err)
			continue
		}
		closedForSafety := false
		for _, intent := range intents {
			covered := state.stopQty
			if intent.Kind == "TAKE_PROFIT" {
				covered = state.takeQty
			}
			if covered+tolerance >= pos.Quantity || intent.TriggerPrice <= 0 {
				continue
			}
			missingQuantity := pos.Quantity - covered
			var repairErr error
			if intent.Kind == "STOP_LOSS" {
				repairErr = trader.SetStopLoss(pos.Symbol, pos.Side, missingQuantity, intent.TriggerPrice)
			} else if intent.Kind == "TAKE_PROFIT" {
				repairErr = trader.SetTakeProfit(pos.Symbol, pos.Side, missingQuantity, intent.TriggerPrice)
			} else {
				continue
			}
			repairStatus := "ACTIVE"
			repairErrorText := ""
			if repairErr != nil {
				repairStatus = "FAILED"
				repairErrorText = repairErr.Error()
			}
			if err := m.store.Execution().UpsertProtection(traderID, exchangeID, pos.EntryOrderID, pos.Symbol, pos.Side, intent.Kind, missingQuantity, intent.TriggerPrice, repairStatus, intent.AttemptCount+1, repairErrorText); err != nil {
				logger.Infof("⚠️  Failed to persist protection repair for %s %s: %v", pos.Symbol, pos.Side, err)
			}
			if intent.Kind == "STOP_LOSS" && repairErr != nil {
				m.emergencyCloseUnprotected(traderID, exchangeID, trader, pos, "stop-loss repair failed: "+repairErr.Error())
				closedForSafety = true
				break
			}
		}
		if !closedForSafety && state.stopQty+tolerance < pos.Quantity {
			hasStopIntent := false
			for _, intent := range intents {
				if intent.Kind == "STOP_LOSS" && intent.TriggerPrice > 0 {
					hasStopIntent = true
					break
				}
			}
			if !hasStopIntent {
				m.emergencyCloseUnprotected(traderID, exchangeID, trader, pos, "stop-loss intent is unavailable")
			}
		}
	}
}

func (m *PositionSyncManager) rebalanceOverCoverage(trader Trader, pos *store.TraderPosition, kind string, covered *float64) {
	kindCanceler, ok := trader.(ProtectionOrderCanceler)
	if !ok {
		return
	}
	if err := kindCanceler.CancelProtectionOrders(pos.Symbol, pos.Side, kind); err != nil {
		logger.Infof("⚠️  Failed to rebalance over-covering %s protections for %s %s: %v", kind, pos.Symbol, pos.Side, err)
		return
	}
	logger.Infof("📊 Rebalanced over-covering %s protections for %s %s (%.8f > %.8f)", kind, pos.Symbol, pos.Side, *covered, pos.Quantity)
	*covered = 0
}

func (m *PositionSyncManager) emergencyCloseUnprotected(traderID, exchangeID string, trader Trader, pos *store.TraderPosition, reason string) {
	logger.Errorf("CRITICAL: closing unprotected %s %s for trader %s: %s", pos.Symbol, pos.Side, traderID, reason)
	// Cancel resting entry orders first: an emergency close must not be
	// reverted by a pending limit entry filling right afterwards.
	m.cancelPendingEntries(traderID, exchangeID, trader, pos.Symbol, pos.Side)
	var result map[string]interface{}
	var err error
	if pos.Side == "LONG" {
		result, err = trader.CloseLong(pos.Symbol, 0)
	} else {
		result, err = trader.CloseShort(pos.Symbol, 0)
	}
	if err != nil {
		logger.Errorf("CRITICAL: failed to submit emergency close for %s %s: %v", pos.Symbol, pos.Side, err)
		return
	}
	orderID := getOrderIDString(result)
	status := normalizeOrderState(fmt.Sprint(result["status"]))
	if orderID != "" && orderID != "0" {
		if fresh, statusErr := trader.GetOrderStatus(pos.Symbol, orderID); statusErr == nil {
			result = fresh
			status = normalizeOrderState(fmt.Sprint(fresh["status"]))
		}
	}
	order := store.TradeOrder{
		TraderID: traderID, ExchangeID: exchangeID, OrderID: orderID, Symbol: pos.Symbol,
		PositionSide: pos.Side, Action: "close_" + strings.ToLower(pos.Side), RequestedQty: pos.Quantity,
		ExecutedQty: getFloatFromMap(result, "executedQty"), AvgPrice: getFloatFromMap(result, "avgPrice"),
		Fee: getFloatFromMap(result, "commission"), Status: string(status), LastError: reason,
	}
	if orderID != "" && orderID != "0" {
		if err := m.store.Execution().UpsertOrder(order); err != nil {
			logger.Infof("⚠️  Failed to persist emergency close order %s: %v", orderID, err)
		}
	}
	if status == OrderStateFilled && pos.EntryOrderID != "" {
		if err := m.store.Execution().UpdateOrderProtection(traderID, exchangeID, pos.EntryOrderID, pos.Quantity, "CLOSED"); err != nil {
			logger.Infof("⚠️  Failed to finalize unprotected entry %s: %v", pos.EntryOrderID, err)
		}
	}
	if canceler, ok := trader.(PositionOrderCanceler); ok {
		if err := canceler.CancelPositionOrders(pos.Symbol, pos.Side); err != nil {
			logger.Infof("⚠️  Failed to cancel residual protection orders for %s %s: %v", pos.Symbol, pos.Side, err)
		}
	}
}

// cancelPendingEntries cancels resting local entry orders for one symbol and
// position side before an emergency close, so the close cannot be silently
// reverted by a pending limit order filling afterwards.
func (m *PositionSyncManager) cancelPendingEntries(traderID, exchangeID string, trader Trader, symbol, positionSide string) {
	canceler, ok := trader.(SingleOrderCanceler)
	if !ok {
		return
	}
	orders, err := m.store.Execution().ListActiveOrders(traderID, exchangeID)
	if err != nil {
		logger.Infof("⚠️  Failed to list pending entry orders for %s %s: %v", symbol, positionSide, err)
		return
	}
	action := "open_" + strings.ToLower(positionSide)
	for _, order := range orders {
		if order.Symbol != symbol || order.Action != action || order.OrderID == "" {
			continue
		}
		if err := canceler.CancelOrder(order.Symbol, order.OrderID); err != nil {
			logger.Infof("⚠️  Failed to cancel pending entry order %s: %v", order.OrderID, err)
		} else {
			logger.Infof("📊 Canceled pending entry order %s before emergency close", order.OrderID)
		}
	}
}

// findClosedPnLRecord Try to find matching ClosedPnL record from exchange
// For Binance, directly query trades for the specific symbol (more reliable than Income API)
func (m *PositionSyncManager) findClosedPnLRecord(trader Trader, pos *store.TraderPosition) *ClosedPnLRecord {
	// Try to get trades directly for this symbol (Binance-specific, more reliable)
	if binanceTrader, ok := trader.(*FuturesTrader); ok {
		return m.findClosedPnLFromBinanceTrades(binanceTrader, pos)
	}

	// Fallback: use GetClosedPnL for other exchanges
	startTime := pos.EntryTime.Add(-time.Minute)
	records, err := trader.GetClosedPnL(startTime, 100)
	if err != nil {
		logger.Infof("⚠️  Failed to get closed PnL records: %v", err)
		return nil
	}

	return m.aggregateClosedRecords(records, pos)
}

// findClosedPnLFromBinanceTrades queries Binance directly for trades of a specific symbol
func (m *PositionSyncManager) findClosedPnLFromBinanceTrades(trader *FuturesTrader, pos *store.TraderPosition) *ClosedPnLRecord {
	// Query from this position lifecycle instead of mixing in an earlier one.
	startTime := pos.EntryTime.Add(-time.Minute)
	trades, err := trader.GetTradesForSymbol(pos.Symbol, startTime, 100)
	if err != nil {
		logger.Infof("⚠️  Failed to get trades for %s: %v", pos.Symbol, err)
		return nil
	}

	if len(trades) == 0 {
		logger.Infof("⚠️  No trades found for %s in the last hour", pos.Symbol)
		return nil
	}

	// Find all closing trades (realizedPnl != 0) that match this position
	var totalQty, totalPnL, totalFee float64
	var weightedExitPrice float64
	var latestExitTime time.Time
	var latestTradeID string
	matchCount := 0

	posSide := strings.ToLower(pos.Side)
	sort.Slice(trades, func(i, j int) bool { return trades[i].Time.Before(trades[j].Time) })

	for _, trade := range trades {
		if trade.Time.Before(pos.EntryTime.Add(-time.Second)) {
			continue
		}
		// Determine if this trade closes our position
		// For LONG position: SELL closes it
		// For SHORT position: BUY closes it
		isClosingTrade := false
		tradeSide := strings.ToUpper(trade.Side)
		positionSide := strings.ToUpper(trade.PositionSide)

		if (positionSide == "LONG" || positionSide == "BOTH" || positionSide == "") && posSide == "long" && tradeSide == "SELL" {
			isClosingTrade = true
		} else if (positionSide == "SHORT" || positionSide == "BOTH" || positionSide == "") && posSide == "short" && tradeSide == "BUY" {
			isClosingTrade = true
		}

		if !isClosingTrade {
			continue
		}

		// Aggregate this trade
		totalQty += trade.Quantity
		totalPnL += trade.RealizedPnL
		totalFee += trade.Fee
		weightedExitPrice += trade.Price * trade.Quantity
		matchCount++

		if trade.Time.After(latestExitTime) {
			latestExitTime = trade.Time
			latestTradeID = trade.TradeID
		}
		if totalQty+1e-9 >= pos.Quantity {
			break
		}
	}

	if matchCount == 0 || totalQty <= 0 {
		logger.Infof("⚠️  No closing trades found for %s %s", pos.Symbol, pos.Side)
		return nil
	}

	avgExitPrice := weightedExitPrice / totalQty

	logger.Infof("📊 Found %d closing trades for %s %s: qty=%.4f, exitPrice=%.6f, pnl=%.4f, fee=%.4f",
		matchCount, pos.Symbol, pos.Side, totalQty, avgExitPrice, totalPnL, totalFee)

	return &ClosedPnLRecord{
		Symbol:      pos.Symbol,
		Side:        posSide,
		EntryPrice:  pos.EntryPrice,
		ExitPrice:   avgExitPrice,
		Quantity:    totalQty,
		RealizedPnL: totalPnL,
		Fee:         totalFee,
		ExitTime:    latestExitTime,
		EntryTime:   pos.EntryTime,
		OrderID:     latestTradeID,
		ExchangeID:  latestTradeID,
		CloseType:   "unknown",
	}
}

// aggregateClosedRecords aggregates closed PnL records for a position
func (m *PositionSyncManager) aggregateClosedRecords(records []ClosedPnLRecord, pos *store.TraderPosition) *ClosedPnLRecord {
	if len(records) == 0 {
		return nil
	}

	posSide := strings.ToLower(pos.Side)
	var matchingRecords []ClosedPnLRecord

	for i := range records {
		record := &records[i]
		if record.Symbol != pos.Symbol {
			continue
		}
		if record.ExitTime.Before(pos.EntryTime) {
			continue
		}

		recordSide := strings.ToLower(record.Side)
		if recordSide != posSide {
			continue
		}

		matchingRecords = append(matchingRecords, *record)
	}

	if len(matchingRecords) == 0 {
		return nil
	}

	var totalQty, totalPnL, totalFee float64
	var weightedExitPrice float64
	var latestExitTime time.Time
	var latestOrderID, latestExchangeID string

	sort.Slice(matchingRecords, func(i, j int) bool { return matchingRecords[i].ExitTime.Before(matchingRecords[j].ExitTime) })
	for _, rec := range matchingRecords {
		totalQty += rec.Quantity
		totalPnL += rec.RealizedPnL
		totalFee += rec.Fee
		weightedExitPrice += rec.ExitPrice * rec.Quantity

		if rec.ExitTime.After(latestExitTime) {
			latestExitTime = rec.ExitTime
			latestOrderID = rec.OrderID
			latestExchangeID = rec.ExchangeID
		}
		if totalQty+1e-9 >= pos.Quantity {
			break
		}
	}
	if totalQty <= 0 {
		return nil
	}

	avgExitPrice := weightedExitPrice / totalQty

	logger.Infof("📊 Aggregated %d closing trades for %s %s: qty=%.4f, pnl=%.4f, fee=%.4f",
		len(matchingRecords), pos.Symbol, pos.Side, totalQty, totalPnL, totalFee)

	return &ClosedPnLRecord{
		Symbol:      pos.Symbol,
		Side:        posSide,
		EntryPrice:  pos.EntryPrice,
		ExitPrice:   avgExitPrice,
		Quantity:    totalQty,
		RealizedPnL: totalPnL,
		Fee:         totalFee,
		ExitTime:    latestExitTime,
		EntryTime:   pos.EntryTime,
		OrderID:     latestOrderID,
		ExchangeID:  latestExchangeID,
		CloseType:   "unknown",
	}
}

// abs returns absolute value of float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// getOrCreateTrader Get or create trader instance
func (m *PositionSyncManager) getOrCreateTrader(traderID string) (Trader, error) {
	m.cacheMutex.RLock()
	trader, exists := m.traderCache[traderID]
	m.cacheMutex.RUnlock()

	if exists && trader != nil {
		return trader, nil
	}

	// Need to create new trader instance
	config, err := m.getTraderConfig(traderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get trader config: %w", err)
	}

	trader, err = m.createTrader(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create trader instance: %w", err)
	}

	m.cacheMutex.Lock()
	m.traderCache[traderID] = trader
	m.cacheMutex.Unlock()

	return trader, nil
}

// getTraderConfig Get trader configuration
func (m *PositionSyncManager) getTraderConfig(traderID string) (*store.TraderFullConfig, error) {
	m.cacheMutex.RLock()
	config, exists := m.configCache[traderID]
	m.cacheMutex.RUnlock()

	if exists {
		return config, nil
	}

	// Get from database
	traders, err := m.store.Trader().ListAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get trader list: %w", err)
	}

	var userID string
	for _, t := range traders {
		if t.ID == traderID {
			userID = t.UserID
			break
		}
	}

	if userID == "" {
		return nil, fmt.Errorf("trader not found: %s", traderID)
	}

	config, err = m.store.Trader().GetFullConfig(userID, traderID)
	if err != nil {
		return nil, err
	}

	m.cacheMutex.Lock()
	m.configCache[traderID] = config
	m.cacheMutex.Unlock()

	return config, nil
}

// createTrader Create trader instance based on configuration
func (m *PositionSyncManager) createTrader(config *store.TraderFullConfig) (Trader, error) {
	exchange := config.Exchange

	// Use exchange.ExchangeType to determine specific exchange, not exchange.ID (UUID) or exchange.Type (cex/dex)
	switch exchange.ExchangeType {
	case "binance":
		return NewFuturesTrader(exchange.APIKey, exchange.SecretKey, config.Trader.UserID), nil

	default:
		return nil, fmt.Errorf("unsupported exchange type: %s", exchange.ExchangeType)
	}
}

// InvalidateCache Invalidate cache
func (m *PositionSyncManager) InvalidateCache(traderID string) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	delete(m.traderCache, traderID)
	delete(m.configCache, traderID)
}

// getFloatFromMap Get float64 value from map
func getFloatFromMap(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case int64:
			return float64(val)
		case int:
			return float64(val)
		case string:
			var f float64
			fmt.Sscanf(val, "%f", &f)
			return f
		}
	}
	return 0
}

// =============================================================================
// Startup and History Sync Methods
// =============================================================================

// startupSync performs initial sync on startup
// 1. Sync existing positions from exchange (to detect external positions)
// 2. Sync closed positions history from exchange
func (m *PositionSyncManager) startupSync() {
	logger.Info("📊 Starting startup sync...")

	// Get all traders
	traders, err := m.store.Trader().ListAll()
	if err != nil {
		logger.Infof("⚠️  Failed to get traders for startup sync: %v", err)
		return
	}

	for _, traderInfo := range traders {
		traderID := traderInfo.ID

		// Get trader instance
		trader, err := m.getOrCreateTrader(traderID)
		if err != nil {
			logger.Infof("⚠️  Failed to get trader instance for startup sync (ID: %s): %v", traderID, err)
			continue
		}

		// Get exchange info
		config, err := m.getTraderConfig(traderID)
		if err != nil {
			logger.Infof("⚠️  Failed to get trader config for startup sync (ID: %s): %v", traderID, err)
			continue
		}
		exchangeID := config.Exchange.ID             // UUID
		exchangeType := config.Exchange.ExchangeType // "binance", "bybit" etc

		// 1. Sync current open positions from exchange
		m.syncExternalPositions(traderID, exchangeID, exchangeType, trader)

		// 2. Sync closed positions history from exchange
		m.syncClosedPositionsHistory(traderID, exchangeID, exchangeType, trader)
	}

	logger.Info("📊 Startup sync completed")
}

// syncExternalPositions syncs positions that exist on exchange but not locally
// These could be positions opened manually or from other systems
func (m *PositionSyncManager) syncExternalPositions(traderID, exchangeID, exchangeType string, trader Trader) {
	// Get current positions from exchange
	exchangePositions, err := trader.GetPositions()
	if err != nil {
		logger.Infof("⚠️  Failed to get exchange positions for external sync (ID: %s): %v", traderID, err)
		return
	}

	// Get local open positions
	localPositions, err := m.store.Position().GetOpenPositions(traderID)
	if err != nil {
		logger.Infof("⚠️  Failed to get local positions for external sync (ID: %s): %v", traderID, err)
		return
	}

	// Build local position map: symbol_side -> position
	localMap := make(map[string]*store.TraderPosition)
	for _, pos := range localPositions {
		key := fmt.Sprintf("%s_%s", pos.Symbol, pos.Side)
		localMap[key] = pos
	}

	// Find positions that exist on exchange but not locally
	for _, pos := range exchangePositions {
		symbol, _ := pos["symbol"].(string)
		side, _ := pos["side"].(string)
		if symbol == "" || side == "" {
			continue
		}

		// Normalize side
		normalizedSide := side
		if side == "Buy" || side == "LONG" || side == "long" {
			normalizedSide = "LONG"
		} else if side == "Sell" || side == "SHORT" || side == "short" {
			normalizedSide = "SHORT"
		}

		key := fmt.Sprintf("%s_%s", symbol, normalizedSide)

		// Check if we already have this position locally
		if _, exists := localMap[key]; exists {
			continue // Already tracking this position
		}

		// This is an external position - create local record
		qty := getFloatFromMap(pos, "positionAmt")
		if qty < 0 {
			qty = -qty
		}
		if qty < 0.0000001 {
			continue // No actual position
		}

		entryPrice := getFloatFromMap(pos, "entryPrice")
		leverage := int(getFloatFromMap(pos, "leverage"))
		if leverage == 0 {
			leverage = 1
		}

		// Get entry time if available
		createdTime := getFloatFromMap(pos, "createdTime")
		var entryTime time.Time
		if createdTime > 0 {
			entryTime = time.UnixMilli(int64(createdTime))
		} else {
			entryTime = time.Now() // Use current time as fallback
		}

		// Generate unique exchange position ID
		exchangePositionID := fmt.Sprintf("%s_%s_%d", symbol, normalizedSide, entryTime.UnixMilli())

		newPos := &store.TraderPosition{
			TraderID:           traderID,
			ExchangeID:         exchangeID,
			ExchangeType:       exchangeType,
			ExchangePositionID: exchangePositionID,
			Symbol:             symbol,
			Side:               normalizedSide,
			Quantity:           qty,
			EntryPrice:         entryPrice,
			EntryTime:          entryTime,
			Leverage:           leverage,
			Source:             "sync", // Mark as synced from exchange
		}

		if err := m.store.Position().CreateOpenPosition(newPos); err != nil {
			logger.Infof("⚠️  Failed to create external position record: %v", err)
		} else {
			logger.Infof("📊 Synced external position: [%s] %s %s @ %.4f (qty: %.4f)",
				traderID[:8], symbol, normalizedSide, entryPrice, qty)
		}
	}
}

// syncClosedPositionsHistory syncs closed positions from exchange history
// IMPORTANT: Only exchanges with position-level history API should sync history.
// Binance only has trade-level data, which cannot accurately reconstruct positions.
func (m *PositionSyncManager) syncClosedPositionsHistory(traderID, exchangeID, exchangeType string, trader Trader) {
	// Only sync history for exchanges with position-level API
	switch exchangeType {
	case "okx":
	default:
		// Other exchanges don't have accurate position history API
		// Their GetClosedPnL only returns recent trades for closure detection, not for history sync
		return
	}

	// Get last sync time from database
	lastSyncTime, err := m.store.Position().GetLastClosedPositionTime(traderID)
	if err != nil {
		logger.Infof("⚠️  Failed to get last closed position time (ID: %s): %v", traderID, err)
		// First sync: go back 90 days to get more history
		lastSyncTime = time.Now().Add(-90 * 24 * time.Hour)
	}

	// Subtract a small buffer to avoid missing positions at the boundary
	startTime := lastSyncTime.Add(-1 * time.Minute)

	// Pagination loop to get all records
	const batchSize = 500
	totalCreated := 0
	totalSkipped := 0

	for {
		// Get closed positions from exchange
		closedRecords, err := trader.GetClosedPnL(startTime, batchSize)
		if err != nil {
			logger.Infof("⚠️  Failed to get closed PnL records (ID: %s): %v", traderID, err)
			break
		}

		if len(closedRecords) == 0 {
			break
		}

		// Convert to store.ClosedPnLRecord and sync
		storeRecords := make([]store.ClosedPnLRecord, len(closedRecords))
		var latestExitTime time.Time
		for i, rec := range closedRecords {
			storeRecords[i] = store.ClosedPnLRecord{
				Symbol:      rec.Symbol,
				Side:        rec.Side,
				EntryPrice:  rec.EntryPrice,
				ExitPrice:   rec.ExitPrice,
				Quantity:    rec.Quantity,
				RealizedPnL: rec.RealizedPnL,
				Fee:         rec.Fee,
				Leverage:    rec.Leverage,
				EntryTime:   rec.EntryTime,
				ExitTime:    rec.ExitTime,
				OrderID:     rec.OrderID,
				CloseType:   rec.CloseType,
				ExchangeID:  rec.ExchangeID,
			}
			// Track latest exit time for pagination
			if rec.ExitTime.After(latestExitTime) {
				latestExitTime = rec.ExitTime
			}
		}

		created, skipped, err := m.store.Position().SyncClosedPositions(traderID, exchangeID, exchangeType, storeRecords)
		if err != nil {
			logger.Infof("⚠️  Failed to sync closed positions (ID: %s): %v", traderID, err)
			break
		}

		totalCreated += created
		totalSkipped += skipped

		// If we got fewer records than batch size, we've reached the end
		if len(closedRecords) < batchSize {
			break
		}

		// Move start time forward for next batch (add 1ms to avoid duplicate)
		startTime = latestExitTime.Add(time.Millisecond)
	}

	if totalCreated > 0 {
		logger.Infof("📊 Synced %d new closed positions for trader %s (skipped %d duplicates)",
			totalCreated, traderID[:8], totalSkipped)
	}

	// Update last history sync time
	m.lastHistorySyncMutex.Lock()
	m.lastHistorySync[traderID] = time.Now()
	m.lastHistorySyncMutex.Unlock()
}

// maybeRunHistorySync checks if it's time to run history sync for a trader
func (m *PositionSyncManager) maybeRunHistorySync(traderID, exchangeID, exchangeType string, trader Trader) {
	m.lastHistorySyncMutex.RLock()
	lastSync, exists := m.lastHistorySync[traderID]
	m.lastHistorySyncMutex.RUnlock()

	if !exists || time.Since(lastSync) >= m.historySyncInterval {
		m.syncClosedPositionsHistory(traderID, exchangeID, exchangeType, trader)
	}
}
