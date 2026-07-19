package trader

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"nofx/hook"
	"nofx/logger"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)

// getBrOrderID generates unique order ID (for futures contracts)
// Format: x-{BR_ID}{TIMESTAMP}{RANDOM}
// Futures limit is 32 characters, use this limit consistently
// Uses nanosecond timestamp + random number to ensure global uniqueness (collision probability < 10^-20)
func getBrOrderID() string {
	brID := "KzrpZaP9" // Futures br ID

	// Calculate available space: 32 - len("x-KzrpZaP9") = 32 - 11 = 21 characters
	// Allocation: 13-digit timestamp + 8-digit random = 21 characters (perfect utilization)
	timestamp := time.Now().UnixNano() % 10000000000000 // 13-digit nanosecond timestamp

	// Generate 4-byte random number (8 hex digits)
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	randomHex := hex.EncodeToString(randomBytes)

	// Format: x-KzrpZaP9{13-digit timestamp}{8-digit random}
	// Example: x-KzrpZaP91234567890123abcdef12 (exactly 31 characters)
	orderID := fmt.Sprintf("x-%s%d%s", brID, timestamp, randomHex)

	// Ensure not exceeding 32-character limit (theoretically exactly 31 characters)
	if len(orderID) > 32 {
		orderID = orderID[:32]
	}

	return orderID
}

// FuturesTrader Binance futures trader
type FuturesTrader struct {
	client  *futures.Client
	testnet bool

	// Balance cache
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// Position cache
	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex
	positionsFetchMutex sync.Mutex

	// Cache validity period (15 seconds)
	cacheDuration time.Duration
}

func (t *FuturesTrader) ProtectionCoversFutureFills() bool { return true }

// NewFuturesTrader creates futures trader
func NewFuturesTrader(apiKey, secretKey string, userId string) *FuturesTrader {
	return NewFuturesTraderWithTestnet(apiKey, secretKey, userId, false)
}

// NewFuturesTraderWithTestnet creates futures trader with optional Binance Futures testnet endpoint.
func NewFuturesTraderWithTestnet(apiKey, secretKey string, userId string, testnet bool) *FuturesTrader {
	client := futures.NewClient(apiKey, secretKey)
	if testnet {
		client.BaseURL = futures.BaseApiTestnetUrl
	}

	hookRes := hook.HookExec[hook.NewBinanceTraderResult](hook.NEW_BINANCE_TRADER, userId, client)
	if hookRes != nil && hookRes.GetResult() != nil {
		client = hookRes.GetResult()
		if testnet {
			client.BaseURL = futures.BaseApiTestnetUrl
		}
	}

	// Sync time to avoid "Timestamp ahead" error
	syncBinanceServerTime(client)
	trader := &FuturesTrader{
		client:        client,
		testnet:       testnet,
		cacheDuration: 15 * time.Second, // 15-second cache
	}

	// Set dual-side position mode (Hedge Mode)
	// This is required because the code uses PositionSide (LONG/SHORT)
	if err := trader.setDualSidePosition(); err != nil {
		logger.Infof("⚠️ Failed to set dual-side position mode: %v (ignore this warning if already in dual-side mode)", err)
	}

	return trader
}

// setDualSidePosition sets dual-side position mode (called during initialization)
func (t *FuturesTrader) setDualSidePosition() error {
	// Try to set dual-side position mode
	err := t.client.NewChangePositionModeService().
		DualSide(true). // true = dual-side position (Hedge Mode)
		Do(context.Background())

	if err != nil {
		// If error message contains "No need to change", it means already in dual-side position mode
		if strings.Contains(err.Error(), "No need to change position side") {
			logger.Infof("  ✓ Account is already in dual-side position mode (Hedge Mode)")
			return nil
		}
		// Other errors are returned (but won't interrupt initialization in the caller)
		return err
	}

	logger.Infof("  ✓ Account has been switched to dual-side position mode (Hedge Mode)")
	logger.Infof("  ℹ️  Dual-side position mode allows holding both long and short positions simultaneously")
	return nil
}

// syncBinanceServerTime syncs Binance server time to ensure request timestamps are valid
func syncBinanceServerTime(client *futures.Client) {
	serverTime, err := client.NewServerTimeService().Do(context.Background())
	if err != nil {
		logger.Infof("⚠️ Failed to sync Binance server time: %v", err)
		return
	}

	now := time.Now().UnixMilli()
	offset := now - serverTime
	client.TimeOffset = offset
	logger.Infof("⏱ Binance server time synced, offset %dms", offset)
}

// GetBalance gets account balance (with cache)
func (t *FuturesTrader) GetBalance() (map[string]interface{}, error) {
	// First check if cache is valid
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		cacheAge := time.Since(t.balanceCacheTime)
		t.balanceCacheMutex.RUnlock()
		logger.Infof("✓ Using cached account balance (cache age: %.1f seconds ago)", cacheAge.Seconds())
		return t.cachedBalance, nil
	}
	t.balanceCacheMutex.RUnlock()

	// Cache expired or doesn't exist, call API
	logger.Infof("🔄 Cache expired, calling Binance API to get account balance...")
	account, err := t.client.NewGetAccountService().Do(context.Background())
	if err != nil {
		logger.Infof("❌ Binance API call failed: %v", err)
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}

	result := make(map[string]interface{})
	result["totalWalletBalance"], _ = strconv.ParseFloat(account.TotalWalletBalance, 64)
	result["availableBalance"], _ = strconv.ParseFloat(account.AvailableBalance, 64)
	result["totalUnrealizedProfit"], _ = strconv.ParseFloat(account.TotalUnrealizedProfit, 64)

	logger.Infof("✓ Binance API returned: total balance=%s, available=%s, unrealized PnL=%s",
		account.TotalWalletBalance,
		account.AvailableBalance,
		account.TotalUnrealizedProfit)

	// Update cache
	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return result, nil
}

// GetPositions gets all positions (with cache)
func (t *FuturesTrader) GetPositions() ([]map[string]interface{}, error) {
	return t.getPositions(false)
}

func (t *FuturesTrader) getPositions(forceFresh bool) ([]map[string]interface{}, error) {
	t.positionsFetchMutex.Lock()
	defer t.positionsFetchMutex.Unlock()
	// First check if cache is valid
	t.positionsCacheMutex.RLock()
	if !forceFresh && t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		cacheAge := time.Since(t.positionsCacheTime)
		t.positionsCacheMutex.RUnlock()
		logger.Infof("✓ Using cached position information (cache age: %.1f seconds ago)", cacheAge.Seconds())
		return t.cachedPositions, nil
	}
	t.positionsCacheMutex.RUnlock()

	// Cache expired or doesn't exist, call API
	logger.Infof("🔄 Cache expired, calling Binance API to get position information...")
	positions, err := t.client.NewGetPositionRiskService().Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		posAmt, _ := strconv.ParseFloat(pos.PositionAmt, 64)
		if posAmt == 0 {
			continue // Skip positions with zero amount
		}

		posMap := make(map[string]interface{})
		posMap["symbol"] = pos.Symbol
		posMap["positionAmt"], _ = strconv.ParseFloat(pos.PositionAmt, 64)
		posMap["entryPrice"], _ = strconv.ParseFloat(pos.EntryPrice, 64)
		posMap["markPrice"], _ = strconv.ParseFloat(pos.MarkPrice, 64)
		posMap["unRealizedProfit"], _ = strconv.ParseFloat(pos.UnRealizedProfit, 64)
		posMap["leverage"], _ = strconv.ParseFloat(pos.Leverage, 64)
		posMap["liquidationPrice"], _ = strconv.ParseFloat(pos.LiquidationPrice, 64)
		// Note: Binance SDK doesn't expose updateTime field, will fallback to local tracking

		// Determine direction
		if posAmt > 0 {
			posMap["side"] = "long"
		} else {
			posMap["side"] = "short"
		}

		result = append(result, posMap)
	}

	// Update cache
	t.positionsCacheMutex.Lock()
	t.cachedPositions = result
	t.positionsCacheTime = time.Now()
	t.positionsCacheMutex.Unlock()

	return result, nil
}

// GetPositionsFresh bypasses the read cache for account reconciliation.
func (t *FuturesTrader) GetPositionsFresh() ([]map[string]interface{}, error) {
	return t.getPositions(true)
}

// SetMarginMode sets margin mode
func (t *FuturesTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	var marginType futures.MarginType
	if isCrossMargin {
		marginType = futures.MarginTypeCrossed
	} else {
		marginType = futures.MarginTypeIsolated
	}

	// Try to set margin mode
	err := t.client.NewChangeMarginTypeService().
		Symbol(symbol).
		MarginType(marginType).
		Do(context.Background())

	marginModeStr := "Cross Margin"
	if !isCrossMargin {
		marginModeStr = "Isolated Margin"
	}

	if err != nil {
		// If error message contains "No need to change", margin mode is already set to target value
		if contains(err.Error(), "No need to change margin type") {
			logger.Infof("  ✓ %s margin mode is already %s", symbol, marginModeStr)
			return nil
		}
		// If there is an open position, margin mode cannot be changed, but this doesn't affect trading
		if contains(err.Error(), "Margin type cannot be changed if there exists position") {
			logger.Infof("  ⚠️ %s has open positions, cannot change margin mode, continuing with current mode", symbol)
			return nil
		}
		// Detect Multi-Assets mode (error code -4168)
		if contains(err.Error(), "Multi-Assets mode") || contains(err.Error(), "-4168") || contains(err.Error(), "4168") {
			logger.Infof("  ⚠️ %s detected Multi-Assets mode, forcing Cross Margin mode", symbol)
			logger.Infof("  💡 Tip: To use Isolated Margin mode, please disable Multi-Assets mode in Binance")
			return nil
		}
		// Detect Unified Account API (Portfolio Margin)
		if contains(err.Error(), "unified") || contains(err.Error(), "portfolio") || contains(err.Error(), "Portfolio") {
			logger.Infof("  ❌ %s detected Unified Account API, unable to trade futures", symbol)
			return fmt.Errorf("please use 'Spot & Futures Trading' API permission, do not use 'Unified Account API'")
		}
		logger.Infof("  ⚠️ Failed to set margin mode: %v", err)
		// Don't return error, let trading continue
		return nil
	}

	logger.Infof("  ✓ %s margin mode set to %s", symbol, marginModeStr)
	return nil
}

// SetLeverage sets leverage (with smart detection and cooldown period)
func (t *FuturesTrader) SetLeverage(symbol string, leverage int) error {
	// First try to get current leverage (from position information)
	currentLeverage := 0
	positions, err := t.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == symbol {
				if lev, ok := pos["leverage"].(float64); ok {
					currentLeverage = int(lev)
					break
				}
			}
		}
	}

	// If current leverage is already the target leverage, skip
	if currentLeverage == leverage && currentLeverage > 0 {
		logger.Infof("  ✓ %s leverage is already %dx, no need to change", symbol, leverage)
		return nil
	}

	// Change leverage
	_, err = t.client.NewChangeLeverageService().
		Symbol(symbol).
		Leverage(leverage).
		Do(context.Background())

	if err != nil {
		// If error message contains "No need to change", leverage is already the target value
		if contains(err.Error(), "No need to change") {
			logger.Infof("  ✓ %s leverage is already %dx", symbol, leverage)
			return nil
		}
		if isInvalidLeverageError(err) {
			logger.Infof("  ⚠️ %s leverage %dx is not valid, trying lower leverage", symbol, leverage)
			for _, fallback := range leverageFallbackCandidates(leverage) {
				_, retryErr := t.client.NewChangeLeverageService().
					Symbol(symbol).
					Leverage(fallback).
					Do(context.Background())
				if retryErr == nil {
					logger.Infof("  ✓ %s leverage changed to fallback %dx", symbol, fallback)
					time.Sleep(5 * time.Second)
					return nil
				}
				if contains(retryErr.Error(), "No need to change") {
					logger.Infof("  ✓ %s leverage is already %dx", symbol, fallback)
					return nil
				}
				if !isInvalidLeverageError(retryErr) {
					return fmt.Errorf("failed to set leverage: %w", retryErr)
				}
			}
		}
		return fmt.Errorf("failed to set leverage: %w", err)
	}

	logger.Infof("  ✓ %s leverage changed to %dx", symbol, leverage)

	// Wait 5 seconds after changing leverage (to avoid cooldown period errors)
	logger.Infof("  ⏱ Waiting 5 seconds for cooldown period...")
	time.Sleep(5 * time.Second)

	return nil
}

// OpenLong opens a long position
func (t *FuturesTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return t.OpenLongWithOptions(symbol, quantity, leverage, OrderOptions{Type: OrderTypeMarket})
}

// OpenLongWithOptions opens a long position using market or limit order.
func (t *FuturesTrader) OpenLongWithOptions(symbol string, quantity float64, leverage int, options OrderOptions) (map[string]interface{}, error) {
	return t.openPosition(symbol, quantity, leverage, futures.SideTypeBuy, futures.PositionSideTypeLong, "long", options)
}

// OpenShort opens a short position
func (t *FuturesTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return t.OpenShortWithOptions(symbol, quantity, leverage, OrderOptions{Type: OrderTypeMarket})
}

// OpenShortWithOptions opens a short position using market or limit order.
func (t *FuturesTrader) OpenShortWithOptions(symbol string, quantity float64, leverage int, options OrderOptions) (map[string]interface{}, error) {
	return t.openPosition(symbol, quantity, leverage, futures.SideTypeSell, futures.PositionSideTypeShort, "short", options)
}

func (t *FuturesTrader) openPosition(symbol string, quantity float64, leverage int, side futures.SideType, positionSide futures.PositionSideType, sideLabel string, options OrderOptions) (map[string]interface{}, error) {
	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// Note: Margin mode should be set by the caller (AutoTrader) before opening position via SetMarginMode

	// Format quantity to correct precision
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Check if formatted quantity is 0 (prevent rounding errors)
	quantityFloat, parseErr := strconv.ParseFloat(quantityStr, 64)
	if parseErr != nil || quantityFloat <= 0 {
		return nil, fmt.Errorf("position size too small, rounded to 0 (original: %.8f → formatted: %s). Suggest increasing position amount or selecting a lower-priced coin", quantity, quantityStr)
	}

	// Check minimum notional value (Binance requires at least 10 USDT)
	if err := t.CheckMinNotional(symbol, quantityFloat); err != nil {
		return nil, err
	}

	orderType := options.Type
	if orderType == "" {
		orderType = OrderTypeMarket
	}

	orderService := t.client.NewCreateOrderService().
		Symbol(symbol).
		Side(side).
		PositionSide(positionSide).
		Quantity(quantityStr).
		NewClientOrderID(getBrOrderID()).
		Type(futures.OrderTypeMarket)

	var submittedPrice string
	switch orderType {
	case OrderTypeMarket:
		orderService.Type(futures.OrderTypeMarket)
	case OrderTypeLimit:
		if options.LimitPrice <= 0 {
			return nil, fmt.Errorf("limit price must be greater than 0")
		}
		priceStr, err := t.FormatLimitPrice(symbol, options.LimitPrice, side)
		if err != nil {
			return nil, err
		}
		submittedPrice = priceStr
		orderService.Type(futures.OrderTypeLimit).
			TimeInForce(futures.TimeInForceTypeGTC).
			Price(priceStr)
	default:
		return nil, fmt.Errorf("unsupported order type: %s", orderType)
	}

	order, err := orderService.Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("failed to open %s position: %w", sideLabel, err)
	}

	if orderType == OrderTypeLimit {
		logger.Infof("✓ Submitted limit %s order successfully: %s quantity: %s price: %s", sideLabel, symbol, quantityStr, submittedPrice)
	} else {
		logger.Infof("✓ Submitted market %s order: %s quantity: %s", sideLabel, symbol, quantityStr)
	}
	logger.Infof("  Order ID: %d", order.OrderID)

	result := make(map[string]interface{})
	result["orderId"] = order.OrderID
	result["symbol"] = order.Symbol
	result["status"] = order.Status
	result["orderType"] = string(orderType)
	if submittedPrice != "" {
		result["price"] = submittedPrice
	}
	return result, nil
}

// CloseLong closes a long position
func (t *FuturesTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	// If quantity is 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "long" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("no long position found for %s", symbol)
		}
	}

	// Format quantity
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Create market sell order (close long, using br ID)
	order, err := t.client.NewCreateOrderService().
		Symbol(symbol).
		Side(futures.SideTypeSell).
		PositionSide(futures.PositionSideTypeLong).
		Type(futures.OrderTypeMarket).
		Quantity(quantityStr).
		NewClientOrderID(getBrOrderID()).
		Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("failed to close long position: %w", err)
	}

	logger.Infof("✓ Submitted close-long order: %s quantity: %s", symbol, quantityStr)

	result := make(map[string]interface{})
	result["orderId"] = order.OrderID
	result["symbol"] = order.Symbol
	result["status"] = order.Status
	return result, nil
}

// CloseShort closes a short position
func (t *FuturesTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// If quantity is 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "short" {
				quantity = -pos["positionAmt"].(float64) // Short position quantity is negative, take absolute value
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("no short position found for %s", symbol)
		}
	}

	// Format quantity
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Create market buy order (close short, using br ID)
	order, err := t.client.NewCreateOrderService().
		Symbol(symbol).
		Side(futures.SideTypeBuy).
		PositionSide(futures.PositionSideTypeShort).
		Type(futures.OrderTypeMarket).
		Quantity(quantityStr).
		NewClientOrderID(getBrOrderID()).
		Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("failed to close short position: %w", err)
	}

	logger.Infof("✓ Submitted close-short order: %s quantity: %s", symbol, quantityStr)

	result := make(map[string]interface{})
	result["orderId"] = order.OrderID
	result["symbol"] = order.Symbol
	result["status"] = order.Status
	return result, nil
}

// CancelStopLossOrders cancels only stop-loss orders (doesn't affect take-profit orders)
func (t *FuturesTrader) CancelStopLossOrders(symbol string) error {
	// Get all open orders for this symbol
	orders, err := t.client.NewListOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err != nil {
		return fmt.Errorf("failed to get open orders: %w", err)
	}

	// Filter out stop-loss orders and cancel them (cancel all directions including LONG and SHORT)
	canceledCount := 0
	var cancelErrors []error
	for _, order := range orders {
		orderType := order.Type

		// Only cancel stop-loss orders (don't cancel take-profit orders)
		if orderType == futures.OrderTypeStopMarket || orderType == futures.OrderTypeStop {
			_, err := t.client.NewCancelOrderService().
				Symbol(symbol).
				OrderID(order.OrderID).
				Do(context.Background())

			if err != nil {
				errMsg := fmt.Sprintf("Order ID %d: %v", order.OrderID, err)
				cancelErrors = append(cancelErrors, fmt.Errorf("%s", errMsg))
				logger.Infof("  ⚠ Failed to cancel stop-loss order: %s", errMsg)
				continue
			}

			canceledCount++
			logger.Infof("  ✓ Canceled stop-loss order (Order ID: %d, Type: %s, Side: %s)", order.OrderID, orderType, order.PositionSide)
		}
	}

	if canceledCount == 0 && len(cancelErrors) == 0 {
		logger.Infof("  ℹ %s has no stop-loss orders to cancel", symbol)
	} else if canceledCount > 0 {
		logger.Infof("  ✓ Canceled %d stop-loss order(s) for %s", canceledCount, symbol)
	}

	// If all cancellations failed, return error
	if len(cancelErrors) > 0 && canceledCount == 0 {
		return fmt.Errorf("failed to cancel stop-loss orders: %v", cancelErrors)
	}

	return nil
}

// CancelTakeProfitOrders cancels only take-profit orders (doesn't affect stop-loss orders)
func (t *FuturesTrader) CancelTakeProfitOrders(symbol string) error {
	// Get all open orders for this symbol
	orders, err := t.client.NewListOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err != nil {
		return fmt.Errorf("failed to get open orders: %w", err)
	}

	// Filter out take-profit orders and cancel them (cancel all directions including LONG and SHORT)
	canceledCount := 0
	var cancelErrors []error
	for _, order := range orders {
		orderType := order.Type

		// Only cancel take-profit orders (don't cancel stop-loss orders)
		if orderType == futures.OrderTypeTakeProfitMarket || orderType == futures.OrderTypeTakeProfit {
			_, err := t.client.NewCancelOrderService().
				Symbol(symbol).
				OrderID(order.OrderID).
				Do(context.Background())

			if err != nil {
				errMsg := fmt.Sprintf("Order ID %d: %v", order.OrderID, err)
				cancelErrors = append(cancelErrors, fmt.Errorf("%s", errMsg))
				logger.Infof("  ⚠ Failed to cancel take-profit order: %s", errMsg)
				continue
			}

			canceledCount++
			logger.Infof("  ✓ Canceled take-profit order (Order ID: %d, Type: %s, Side: %s)", order.OrderID, orderType, order.PositionSide)
		}
	}

	if canceledCount == 0 && len(cancelErrors) == 0 {
		logger.Infof("  ℹ %s has no take-profit orders to cancel", symbol)
	} else if canceledCount > 0 {
		logger.Infof("  ✓ Canceled %d take-profit order(s) for %s", canceledCount, symbol)
	}

	// If all cancellations failed, return error
	if len(cancelErrors) > 0 && canceledCount == 0 {
		return fmt.Errorf("failed to cancel take-profit orders: %v", cancelErrors)
	}

	return nil
}

// CancelAllOrders cancels all pending orders for this symbol
func (t *FuturesTrader) CancelAllOrders(symbol string) error {
	err := t.client.NewCancelAllOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err != nil {
		return fmt.Errorf("failed to cancel pending orders: %w", err)
	}

	logger.Infof("  ✓ Canceled all pending orders for %s", symbol)
	return nil
}

// CancelOrder cancels one order by exchange order ID.
func (t *FuturesTrader) CancelOrder(symbol, orderID string) error {
	id, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid order ID %q: %w", orderID, err)
	}
	if _, err := t.client.NewCancelOrderService().Symbol(symbol).OrderID(id).Do(context.Background()); err != nil {
		return fmt.Errorf("failed to cancel order %s: %w", orderID, err)
	}
	return nil
}

// ListOpenOrders returns a fresh account-level snapshot of regular and
// protective futures orders.
func (t *FuturesTrader) ListOpenOrders() ([]ExchangeOpenOrder, error) {
	orders, err := t.client.NewListOpenOrdersService().Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to list account open orders: %w", err)
	}
	result := make([]ExchangeOpenOrder, 0, len(orders))
	for _, order := range orders {
		qty, _ := strconv.ParseFloat(order.OrigQuantity, 64)
		executed, _ := strconv.ParseFloat(order.ExecutedQuantity, 64)
		avgPrice, _ := strconv.ParseFloat(order.AvgPrice, 64)
		triggerPrice, _ := strconv.ParseFloat(order.StopPrice, 64)
		kind := ""
		switch order.Type {
		case futures.OrderTypeStopMarket, futures.OrderTypeStop:
			kind = "STOP_LOSS"
		case futures.OrderTypeTakeProfitMarket, futures.OrderTypeTakeProfit:
			kind = "TAKE_PROFIT"
		}
		positionSide := string(order.PositionSide)
		if kind != "" && strings.EqualFold(positionSide, "BOTH") {
			if order.Side == futures.SideTypeSell {
				positionSide = "LONG"
			} else if order.Side == futures.SideTypeBuy {
				positionSide = "SHORT"
			}
		}
		result = append(result, ExchangeOpenOrder{
			OrderID: strconv.FormatInt(order.OrderID, 10), Symbol: order.Symbol,
			PositionSide: positionSide, Kind: kind,
			Status: normalizeOrderState(string(order.Status)), Quantity: qty,
			ExecutedQty: executed, AvgPrice: avgPrice, TriggerPrice: triggerPrice,
			ClosePosition: order.ClosePosition,
		})
	}
	return result, nil
}

// ListRecentFills returns immutable account trades for the requested symbols.
func (t *FuturesTrader) ListRecentFills(symbols []string, since time.Time) ([]ExchangeFill, error) {
	seenSymbols := make(map[string]struct{})
	var result []ExchangeFill
	for _, symbol := range symbols {
		if symbol == "" {
			continue
		}
		if _, exists := seenSymbols[symbol]; exists {
			continue
		}
		seenSymbols[symbol] = struct{}{}
		var fromID int64
		for {
			service := t.client.NewListAccountTradeService().Symbol(symbol).Limit(1000)
			if fromID > 0 {
				service.FromID(fromID)
			} else {
				service.StartTime(since.UnixMilli())
			}
			trades, err := service.Do(context.Background())
			if err != nil {
				return nil, fmt.Errorf("failed to list recent fills for %s: %w", symbol, err)
			}
			for _, trade := range trades {
				price, _ := strconv.ParseFloat(trade.Price, 64)
				qty, _ := strconv.ParseFloat(trade.Quantity, 64)
				fee, _ := strconv.ParseFloat(trade.Commission, 64)
				pnl, _ := strconv.ParseFloat(trade.RealizedPnl, 64)
				result = append(result, ExchangeFill{
					TradeID: trade.Symbol + ":" + strconv.FormatInt(trade.ID, 10), OrderID: strconv.FormatInt(trade.OrderID, 10),
					Symbol: trade.Symbol, Side: string(trade.Side), PositionSide: string(trade.PositionSide),
					Price: price, Quantity: qty, Fee: fee, RealizedPnL: pnl, Time: time.UnixMilli(trade.Time),
				})
			}
			if len(trades) < 1000 {
				break
			}
			fromID = trades[len(trades)-1].ID + 1
		}
	}
	return result, nil
}

// CancelPositionOrders cancels protective orders for exactly one hedge-mode
// position side. It intentionally leaves the opposite side and normal entry
// orders untouched.
func (t *FuturesTrader) CancelPositionOrders(symbol, positionSide string) error {
	orders, err := t.client.NewListOpenOrdersService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get open orders: %w", err)
	}

	targetSide := futures.PositionSideType(strings.ToUpper(positionSide))
	var cancelErrors []error
	for _, order := range orders {
		if order.PositionSide != targetSide {
			continue
		}
		if !strings.HasPrefix(order.ClientOrderID, "x-KzrpZaP9") {
			continue
		}
		if order.Type != futures.OrderTypeStopMarket && order.Type != futures.OrderTypeTakeProfitMarket &&
			order.Type != futures.OrderTypeStop && order.Type != futures.OrderTypeTakeProfit {
			continue
		}
		if _, err := t.client.NewCancelOrderService().Symbol(symbol).OrderID(order.OrderID).Do(context.Background()); err != nil {
			cancelErrors = append(cancelErrors, fmt.Errorf("order %d: %w", order.OrderID, err))
		}
	}
	if len(cancelErrors) > 0 {
		return fmt.Errorf("failed to cancel %s protective orders: %v", positionSide, cancelErrors)
	}
	return nil
}

// CancelStopOrders cancels take-profit/stop-loss orders for this symbol (used to adjust TP/SL positions)
func (t *FuturesTrader) CancelStopOrders(symbol string) error {
	// Get all open orders for this symbol
	orders, err := t.client.NewListOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err != nil {
		return fmt.Errorf("failed to get open orders: %w", err)
	}

	// Filter out take-profit and stop-loss orders and cancel them
	canceledCount := 0
	for _, order := range orders {
		orderType := order.Type

		// Only cancel stop-loss and take-profit orders
		if orderType == futures.OrderTypeStopMarket ||
			orderType == futures.OrderTypeTakeProfitMarket ||
			orderType == futures.OrderTypeStop ||
			orderType == futures.OrderTypeTakeProfit {

			_, err := t.client.NewCancelOrderService().
				Symbol(symbol).
				OrderID(order.OrderID).
				Do(context.Background())

			if err != nil {
				logger.Infof("  ⚠ Failed to cancel order %d: %v", order.OrderID, err)
				continue
			}

			canceledCount++
			logger.Infof("  ✓ Canceled take-profit/stop-loss order for %s (Order ID: %d, Type: %s)",
				symbol, order.OrderID, orderType)
		}
	}

	if canceledCount == 0 {
		logger.Infof("  ℹ %s has no take-profit/stop-loss orders to cancel", symbol)
	} else {
		logger.Infof("  ✓ Canceled %d take-profit/stop-loss order(s) for %s", canceledCount, symbol)
	}

	return nil
}

// GetMarketPrice gets market price
func (t *FuturesTrader) GetMarketPrice(symbol string) (float64, error) {
	prices, err := t.client.NewListPricesService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return 0, fmt.Errorf("failed to get price: %w", err)
	}

	if len(prices) == 0 {
		return 0, fmt.Errorf("price not found")
	}

	price, err := strconv.ParseFloat(prices[0].Price, 64)
	if err != nil {
		return 0, err
	}

	return price, nil
}

// CalculatePositionSize calculates position size
func (t *FuturesTrader) CalculatePositionSize(balance, riskPercent, price float64, leverage int) float64 {
	riskAmount := balance * (riskPercent / 100.0)
	positionValue := riskAmount * float64(leverage)
	quantity := positionValue / price
	return quantity
}

// SetStopLoss sets stop-loss order
func (t *FuturesTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	var side futures.SideType
	var posSide futures.PositionSideType

	if positionSide == "LONG" {
		side = futures.SideTypeSell
		posSide = futures.PositionSideTypeLong
	} else {
		side = futures.SideTypeBuy
		posSide = futures.PositionSideTypeShort
	}

	_, err := t.client.NewCreateOrderService().
		Symbol(symbol).
		Side(side).
		PositionSide(posSide).
		Type(futures.OrderTypeStopMarket).
		StopPrice(fmt.Sprintf("%.8f", stopPrice)).
		WorkingType(futures.WorkingTypeContractPrice).
		ClosePosition(true).
		NewClientOrderID(getBrOrderID()).
		Do(context.Background())

	if err != nil {
		return fmt.Errorf("failed to set stop-loss: %w", err)
	}

	logger.Infof("  Stop-loss price set: %.4f", stopPrice)
	return nil
}

// SetTakeProfit sets take-profit order
func (t *FuturesTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	var side futures.SideType
	var posSide futures.PositionSideType

	if positionSide == "LONG" {
		side = futures.SideTypeSell
		posSide = futures.PositionSideTypeLong
	} else {
		side = futures.SideTypeBuy
		posSide = futures.PositionSideTypeShort
	}

	_, err := t.client.NewCreateOrderService().
		Symbol(symbol).
		Side(side).
		PositionSide(posSide).
		Type(futures.OrderTypeTakeProfitMarket).
		StopPrice(fmt.Sprintf("%.8f", takeProfitPrice)).
		WorkingType(futures.WorkingTypeContractPrice).
		ClosePosition(true).
		NewClientOrderID(getBrOrderID()).
		Do(context.Background())

	if err != nil {
		return fmt.Errorf("failed to set take-profit: %w", err)
	}

	logger.Infof("  Take-profit price set: %.4f", takeProfitPrice)
	return nil
}

// GetMinNotional gets minimum notional value (Binance requirement)
func (t *FuturesTrader) GetMinNotional(symbol string) float64 {
	// Use conservative default value of 10 USDT to ensure order passes exchange validation
	return 10.0
}

// CheckMinNotional checks if order meets minimum notional value requirement
func (t *FuturesTrader) CheckMinNotional(symbol string, quantity float64) error {
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return fmt.Errorf("failed to get market price: %w", err)
	}

	notionalValue := quantity * price
	minNotional := t.GetMinNotional(symbol)

	if notionalValue < minNotional {
		return fmt.Errorf(
			"order amount %.2f USDT is below minimum requirement %.2f USDT (quantity: %.4f, price: %.4f)",
			notionalValue, minNotional, quantity, price,
		)
	}

	return nil
}

// GetSymbolPrecision gets the quantity precision for a trading pair
func (t *FuturesTrader) GetSymbolPrecision(symbol string) (int, error) {
	exchangeInfo, err := t.client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		return 0, fmt.Errorf("failed to get trading rules: %w", err)
	}

	for _, s := range exchangeInfo.Symbols {
		if s.Symbol == symbol {
			// Get precision from LOT_SIZE filter
			for _, filter := range s.Filters {
				if filter["filterType"] == "LOT_SIZE" {
					stepSize := filter["stepSize"].(string)
					precision := calculatePrecision(stepSize)
					logger.Infof("  %s quantity precision: %d (stepSize: %s)", symbol, precision, stepSize)
					return precision, nil
				}
			}
		}
	}

	logger.Infof("  ⚠ %s precision information not found, using default precision 3", symbol)
	return 3, nil // Default precision is 3
}

// GetSymbolPriceFilter gets price precision and tick size for a trading pair.
func (t *FuturesTrader) GetSymbolPriceFilter(symbol string) (int, float64, error) {
	exchangeInfo, err := t.client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get trading rules: %w", err)
	}

	for _, s := range exchangeInfo.Symbols {
		if s.Symbol == symbol {
			for _, filter := range s.Filters {
				if filter["filterType"] == "PRICE_FILTER" {
					tickSize := filter["tickSize"].(string)
					precision := calculatePrecision(tickSize)
					tick, _ := strconv.ParseFloat(tickSize, 64)
					logger.Infof("  %s price precision: %d (tickSize: %s)", symbol, precision, tickSize)
					return precision, tick, nil
				}
			}
		}
	}

	logger.Infof("  ⚠ %s price precision information not found, using default precision 4", symbol)
	return 4, 0, nil
}

// calculatePrecision calculates precision from stepSize
func calculatePrecision(stepSize string) int {
	// Remove trailing zeros
	stepSize = trimTrailingZeros(stepSize)

	// Find decimal point
	dotIndex := -1
	for i := 0; i < len(stepSize); i++ {
		if stepSize[i] == '.' {
			dotIndex = i
			break
		}
	}

	// If no decimal point or decimal point is at the end, precision is 0
	if dotIndex == -1 || dotIndex == len(stepSize)-1 {
		return 0
	}

	// Return number of digits after decimal point
	return len(stepSize) - dotIndex - 1
}

// trimTrailingZeros removes trailing zeros
func trimTrailingZeros(s string) string {
	// If no decimal point, return directly
	if !stringContains(s, ".") {
		return s
	}

	// Iterate backwards to remove trailing zeros
	for len(s) > 0 && s[len(s)-1] == '0' {
		s = s[:len(s)-1]
	}

	// If last character is decimal point, remove it too
	if len(s) > 0 && s[len(s)-1] == '.' {
		s = s[:len(s)-1]
	}

	return s
}

// FormatQuantity formats quantity to correct precision
func (t *FuturesTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	precision, err := t.GetSymbolPrecision(symbol)
	if err != nil {
		// If retrieval fails, use default format
		return fmt.Sprintf("%.3f", quantity), nil
	}

	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, quantity), nil
}

// FormatLimitPrice formats and aligns limit price to Binance tick size.
func (t *FuturesTrader) FormatLimitPrice(symbol string, price float64, side futures.SideType) (string, error) {
	precision, tickSize, err := t.GetSymbolPriceFilter(symbol)
	if err != nil {
		return fmt.Sprintf("%.4f", price), nil
	}
	if tickSize > 0 {
		steps := price / tickSize
		if side == futures.SideTypeBuy {
			price = math.Floor(steps) * tickSize
		} else {
			price = math.Ceil(steps) * tickSize
		}
	}

	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, price), nil
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// GetOrderStatus gets order status
func (t *FuturesTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	// Convert orderID to int64
	orderIDInt, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid order ID: %s", orderID)
	}

	order, err := t.client.NewGetOrderService().
		Symbol(symbol).
		OrderID(orderIDInt).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get order status: %w", err)
	}

	// Parse execution price
	avgPrice, _ := strconv.ParseFloat(order.AvgPrice, 64)
	executedQty, _ := strconv.ParseFloat(order.ExecutedQuantity, 64)

	result := map[string]interface{}{
		"orderId":     order.OrderID,
		"symbol":      order.Symbol,
		"status":      string(order.Status),
		"avgPrice":    avgPrice,
		"executedQty": executedQty,
		"side":        string(order.Side),
		"type":        string(order.Type),
		"time":        order.Time,
		"updateTime":  order.UpdateTime,
	}

	// Binance futures commission fee needs to be obtained through GetUserTrades, not retrieved here for now
	// Can be obtained later through WebSocket or separate query
	result["commission"] = 0.0

	return result, nil
}

// GetClosedPnL retrieves recent closing trades from Binance Futures
// Note: Binance does NOT have a position history API, only trade history.
// This returns individual closing trades (realizedPnl != 0) for real-time position closure detection.
// NOT suitable for historical position reconstruction - use only for matching recent closures.
func (t *FuturesTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
	trades, err := t.GetTrades(startTime, limit)
	if err != nil {
		return nil, err
	}

	// Filter only closing trades (realizedPnl != 0) and convert to ClosedPnLRecord
	var records []ClosedPnLRecord
	for _, trade := range trades {
		if trade.RealizedPnL == 0 {
			continue // Skip opening trades
		}

		// Determine side from trade
		side := "long"
		if trade.PositionSide == "SHORT" || trade.PositionSide == "short" {
			side = "short"
		} else if trade.PositionSide == "BOTH" || trade.PositionSide == "" {
			// One-way mode: selling closes long, buying closes short
			if trade.Side == "SELL" || trade.Side == "Sell" {
				side = "long"
			} else {
				side = "short"
			}
		}

		// Calculate entry price from PnL (mathematically accurate for this trade)
		var entryPrice float64
		if trade.Quantity > 0 {
			if side == "long" {
				entryPrice = trade.Price - trade.RealizedPnL/trade.Quantity
			} else {
				entryPrice = trade.Price + trade.RealizedPnL/trade.Quantity
			}
		}

		records = append(records, ClosedPnLRecord{
			Symbol:      trade.Symbol,
			Side:        side,
			EntryPrice:  entryPrice,
			ExitPrice:   trade.Price,
			Quantity:    trade.Quantity,
			RealizedPnL: trade.RealizedPnL,
			Fee:         trade.Fee,
			ExitTime:    trade.Time,
			EntryTime:   trade.Time, // Approximate
			OrderID:     trade.TradeID,
			ExchangeID:  trade.TradeID,
			CloseType:   "unknown",
		})
	}

	return records, nil
}

// GetTrades retrieves trade history from Binance Futures using Income API
// Note: Income API has delays (~minutes), for real-time use GetTradesForSymbol instead
func (t *FuturesTrader) GetTrades(startTime time.Time, limit int) ([]TradeRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	// Use Income API to get REALIZED_PNL records (all symbols)
	incomes, err := t.client.NewGetIncomeHistoryService().
		IncomeType("REALIZED_PNL").
		StartTime(startTime.UnixMilli()).
		Limit(int64(limit)).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get income history: %w", err)
	}

	var trades []TradeRecord
	for _, income := range incomes {
		pnl, _ := strconv.ParseFloat(income.Income, 64)
		if pnl == 0 {
			continue // Skip zero PnL records
		}

		// Income API doesn't provide full trade details, create a minimal record
		// This is mainly used for detecting recent closures, not historical reconstruction
		trade := TradeRecord{
			TradeID:     strconv.FormatInt(income.TranID, 10),
			Symbol:      income.Symbol,
			RealizedPnL: pnl,
			Time:        time.UnixMilli(income.Time),
			// Note: Income API doesn't provide price, quantity, side, fee
			// For accurate data, use GetTradesForSymbol with specific symbol
		}
		trades = append(trades, trade)
	}

	return trades, nil
}

// GetTradesForSymbol retrieves trade history for a specific symbol
// This is more reliable than using Income API which may have delays
func (t *FuturesTrader) GetTradesForSymbol(symbol string, startTime time.Time, limit int) ([]TradeRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	accountTrades, err := t.client.NewListAccountTradeService().
		Symbol(symbol).
		StartTime(startTime.UnixMilli()).
		Limit(limit).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get trade history for %s: %w", symbol, err)
	}

	var trades []TradeRecord
	for _, at := range accountTrades {
		price, _ := strconv.ParseFloat(at.Price, 64)
		qty, _ := strconv.ParseFloat(at.Quantity, 64)
		fee, _ := strconv.ParseFloat(at.Commission, 64)
		pnl, _ := strconv.ParseFloat(at.RealizedPnl, 64)

		trade := TradeRecord{
			TradeID:      strconv.FormatInt(at.ID, 10),
			Symbol:       at.Symbol,
			Side:         string(at.Side),
			PositionSide: string(at.PositionSide),
			Price:        price,
			Quantity:     qty,
			RealizedPnL:  pnl,
			Fee:          fee,
			Time:         time.UnixMilli(at.Time),
		}
		trades = append(trades, trade)
	}

	return trades, nil
}
