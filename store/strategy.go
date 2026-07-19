package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// StrategyStore strategy storage
type StrategyStore struct {
	db *sql.DB
}

var defaultStaticCoins = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT", "HYPEUSDT"}

// Strategy strategy configuration
type Strategy struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`  // whether it is active (a user can only have one active strategy)
	IsDefault   bool      `json:"is_default"` // whether it is a system default strategy
	Config      string    `json:"config"`     // strategy configuration in JSON format
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// StrategyConfig strategy configuration details (JSON structure)
type StrategyConfig struct {
	// coin source configuration
	CoinSource CoinSourceConfig `json:"coin_source"`
	// quantitative data configuration
	Indicators IndicatorConfig `json:"indicators"`
	// custom prompt (appended at the end)
	CustomPrompt string `json:"custom_prompt,omitempty"`
	// risk control configuration
	RiskControl RiskControlConfig `json:"risk_control"`
	// editable sections of System Prompt
	PromptSections PromptSectionsConfig `json:"prompt_sections,omitempty"`
}

// PromptSectionsConfig editable sections of System Prompt
type PromptSectionsConfig struct {
	// role definition (title + description)
	RoleDefinition string `json:"role_definition,omitempty"`
	// trading frequency awareness
	TradingFrequency string `json:"trading_frequency,omitempty"`
	// entry standards
	EntryStandards string `json:"entry_standards,omitempty"`
	// decision process
	DecisionProcess string `json:"decision_process,omitempty"`
}

// CoinSourceConfig coin source configuration
type CoinSourceConfig struct {
	// source type: only "static" is supported
	SourceType string `json:"source_type"`
	// static coin list
	StaticCoins []string `json:"static_coins,omitempty"`
}

// IndicatorConfig indicator configuration
type IndicatorConfig struct {
	// K-line configuration
	Klines KlineConfig `json:"klines"`
	// raw kline data (OHLCV) - always enabled, required for AI analysis
	EnableRawKlines bool `json:"enable_raw_klines"`
	// technical indicator switches
	EnableEMA         bool `json:"enable_ema"`
	EnableMACD        bool `json:"enable_macd"`
	EnableRSI         bool `json:"enable_rsi"`
	EnableATR         bool `json:"enable_atr"`
	EnableVolume      bool `json:"enable_volume"`
	EnableOI          bool `json:"enable_oi"`           // open interest
	EnableFundingRate bool `json:"enable_funding_rate"` // funding rate
	// EMA period configuration
	EMAPeriods []int `json:"ema_periods,omitempty"` // default [20, 50]
	// RSI period configuration
	RSIPeriods []int `json:"rsi_periods,omitempty"` // default [7, 14]
	// ATR period configuration
	ATRPeriods []int `json:"atr_periods,omitempty"` // default [14]
	// external data sources
	ExternalDataSources []ExternalDataSource `json:"external_data_sources,omitempty"`
	// quantitative data sources (capital flow, position changes, price changes)
	EnableQuantData    bool   `json:"enable_quant_data"`            // whether to enable quantitative data
	QuantDataAPIURL    string `json:"quant_data_api_url,omitempty"` // quantitative data API address
	EnableQuantOI      bool   `json:"enable_quant_oi"`              // whether to show OI data
	EnableQuantNetflow bool   `json:"enable_quant_netflow"`         // whether to show Netflow data
	// OI ranking data (market-wide open interest increase/decrease rankings)
	EnableOIRanking   bool   `json:"enable_oi_ranking"`             // whether to enable OI ranking data
	OIRankingAPIURL   string `json:"oi_ranking_api_url,omitempty"`  // OI ranking API base URL
	OIRankingDuration string `json:"oi_ranking_duration,omitempty"` // duration: 1h, 4h, 24h
	OIRankingLimit    int    `json:"oi_ranking_limit,omitempty"`    // number of entries (default 10)
}

// KlineConfig K-line configuration
type KlineConfig struct {
	// primary timeframe: "1m", "3m", "5m", "15m", "1h", "4h"
	PrimaryTimeframe string `json:"primary_timeframe"`
	// primary timeframe K-line count
	PrimaryCount int `json:"primary_count"`
	// longer timeframe
	LongerTimeframe string `json:"longer_timeframe,omitempty"`
	// longer timeframe K-line count
	LongerCount int `json:"longer_count,omitempty"`
	// whether to enable multi-timeframe analysis
	EnableMultiTimeframe bool `json:"enable_multi_timeframe"`
	// selected timeframe list (new: supports multi-timeframe selection)
	SelectedTimeframes []string `json:"selected_timeframes,omitempty"`
}

// ExternalDataSource external data source configuration
type ExternalDataSource struct {
	Name        string            `json:"name"`   // data source name
	Type        string            `json:"type"`   // type: "api" | "webhook"
	URL         string            `json:"url"`    // API URL
	Method      string            `json:"method"` // HTTP method
	Headers     map[string]string `json:"headers,omitempty"`
	DataPath    string            `json:"data_path,omitempty"`    // JSON data path
	RefreshSecs int               `json:"refresh_secs,omitempty"` // refresh interval (seconds)
}

// RiskControlConfig risk control configuration
// All parameters are clearly defined without ambiguity:
//
// Position Limits:
//   - MaxPositions: max number of coins held simultaneously (CODE ENFORCED)
//
// Trading Leverage (exchange leverage for opening positions):
//   - BTCETHMaxLeverage: BTC/ETH max exchange leverage (AI guided)
//   - AltcoinMaxLeverage: Altcoin max exchange leverage (AI guided)
//
// Position Value Limits:
//   - MaxPositionSize: maximum notional value of one opening order (CODE ENFORCED)
//   - MaxTotalPositionSize: maximum total notional value after opening (CODE ENFORCED)
//
// Risk Controls:
//   - MinPositionSize: minimum position size in USDT (CODE ENFORCED)
//   - MinRiskRewardRatio: minimum reward/risk ratio at the expected entry price (CODE ENFORCED)
//   - MinConfidence: min AI confidence to open position (AI guided)
//
// Order Execution:
//   - OrderType: "market" or "limit" for opening positions
//   - LimitPriceOffsetPct: limit order offset from current price in percent
type RiskControlConfig struct {
	// Max number of coins held simultaneously (CODE ENFORCED)
	MaxPositions int `json:"max_positions"`

	// BTC/ETH exchange leverage for opening positions (AI guided)
	BTCETHMaxLeverage int `json:"btc_eth_max_leverage"`
	// Altcoin exchange leverage for opening positions (AI guided)
	AltcoinMaxLeverage int `json:"altcoin_max_leverage"`

	// Maximum notional value of a single opening order in USDT (CODE ENFORCED)
	MaxPositionSize float64 `json:"max_position_size"`
	// Maximum total open-position notional value in USDT (CODE ENFORCED)
	MaxTotalPositionSize float64 `json:"max_total_position_size"`
	// Min position size in USDT (CODE ENFORCED)
	MinPositionSize float64 `json:"min_position_size"`

	// Minimum reward/risk ratio at the expected entry price (CODE ENFORCED)
	MinRiskRewardRatio float64 `json:"min_risk_reward_ratio"`
	// Min AI confidence to open position (AI guided)
	MinConfidence int `json:"min_confidence"`

	// Account-level circuit breakers. All percentages are expressed as 0-100.
	MaxDailyLossPct           float64 `json:"max_daily_loss_pct,omitempty"`
	MaxDrawdownPct            float64 `json:"max_drawdown_pct,omitempty"`
	MaxMarginUsagePct         float64 `json:"max_margin_usage_pct,omitempty"`
	MinLiquidationDistancePct float64 `json:"min_liquidation_distance_pct,omitempty"`
	MaxConsecutiveFailures    int     `json:"max_consecutive_failures,omitempty"`
	MaxMarketMovePct          float64 `json:"max_market_move_pct,omitempty"`
	StopTradingMinutes        int     `json:"stop_trading_minutes,omitempty"`

	// Opening order type: "market" or "limit" (close orders remain market for safety)
	OrderType string `json:"order_type,omitempty"`
	// Limit order offset from current price in percent. Long uses below market, short uses above market.
	LimitPriceOffsetPct float64 `json:"limit_price_offset_pct,omitempty"`
	// Number of retries for installing stop-loss/take-profit orders after a fill.
	ProtectionRetries int `json:"protection_retries,omitempty"`
	// Delay between protection retries in milliseconds.
	ProtectionRetryDelayMs int `json:"protection_retry_delay_ms,omitempty"`
	// Action after protection retries are exhausted: "close", "reduce", or "keep_unprotected".
	ProtectionFailureAction string `json:"protection_failure_action,omitempty"`
	// Percentage to close when ProtectionFailureAction is "reduce".
	ProtectionFailureReducePct float64 `json:"protection_failure_reduce_pct,omitempty"`
}

func (s *StrategyStore) initTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS strategies (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			is_active BOOLEAN DEFAULT 0,
			is_default BOOLEAN DEFAULT 0,
			config TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// create indexes
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_strategies_user_id ON strategies(user_id)`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_strategies_is_active ON strategies(is_active)`)

	// trigger: automatically update updated_at on update
	_, err = s.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_strategies_updated_at
		AFTER UPDATE ON strategies
		BEGIN
			UPDATE strategies SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END
	`)

	return err
}

func (s *StrategyStore) initDefaultData() error {
	// No longer pre-populate strategies - create on demand when user configures
	return nil
}

// GetDefaultStrategyConfig returns the default strategy configuration for the given language
func GetDefaultStrategyConfig(lang string) StrategyConfig {
	config := StrategyConfig{
		CoinSource: CoinSourceConfig{
			SourceType:  "static",
			StaticCoins: append([]string(nil), defaultStaticCoins...),
		},
		Indicators: IndicatorConfig{
			Klines: KlineConfig{
				PrimaryTimeframe:     "5m",
				PrimaryCount:         30,
				LongerTimeframe:      "4h",
				LongerCount:          10,
				EnableMultiTimeframe: true,
				SelectedTimeframes:   []string{"5m", "15m", "1h", "4h"},
			},
			EnableRawKlines:    true, // Required - raw OHLCV data for AI analysis
			EnableEMA:          false,
			EnableMACD:         false,
			EnableRSI:          false,
			EnableATR:          false,
			EnableVolume:       true,
			EnableOI:           true,
			EnableFundingRate:  true,
			EMAPeriods:         []int{20, 50},
			RSIPeriods:         []int{7, 14},
			ATRPeriods:         []int{14},
			EnableQuantData:    true,
			QuantDataAPIURL:    "http://nofxaios.com:30006/api/coin/{symbol}?include=netflow,oi,price&auth=cm_568c67eae410d912c54c",
			EnableQuantOI:      true,
			EnableQuantNetflow: true,
			// OI ranking data - market-wide OI increase/decrease rankings
			EnableOIRanking:   true,
			OIRankingAPIURL:   "http://nofxaios.com:30006",
			OIRankingDuration: "1h",
			OIRankingLimit:    10,
		},
		RiskControl: RiskControlConfig{
			MaxPositions:               3,    // Max 3 coins simultaneously (CODE ENFORCED)
			BTCETHMaxLeverage:          5,    // BTC/ETH exchange leverage (AI guided)
			AltcoinMaxLeverage:         5,    // Altcoin exchange leverage (AI guided)
			MaxPositionSize:            1000, // Max 1,000 USDT per opening order
			MaxTotalPositionSize:       3000, // Max 3,000 USDT total open notional
			MinPositionSize:            12,   // Min 12 USDT per position (CODE ENFORCED)
			MinRiskRewardRatio:         3.0,  // Min 3:1 profit/loss ratio (AI guided)
			MinConfidence:              75,   // Min 75% confidence (AI guided)
			OrderType:                  "market",
			LimitPriceOffsetPct:        0.05,
			ProtectionRetries:          3,
			ProtectionRetryDelayMs:     1000,
			ProtectionFailureAction:    "close",
			ProtectionFailureReducePct: 50,
		},
	}

	if lang == "zh" {
		config.PromptSections = PromptSectionsConfig{
			RoleDefinition: `# 你是一个专业的加密市场多资产交易AI

你的任务是根据提供的市场数据，交易加密原生资产以及加密交易场所上的代币化美股/美股挂钩永续合约。你擅长多时间框架分析、衍生品定价和风险管理。

对每个标的先识别资产类型。对美股挂钩标的，要区分“加密场所合约”与“美股现货”：合约可能24/7交易，但主要价格发现、流动性和跳空风险仍受美股盘前、常规时段、盘后、周末及休市影响。不要默认它们跟随BTC。

只使用输入中明确提供的数据。不得臆测实时美股现货价、基差、财报、新闻、分红、停牌或开收盘状态；当这些信息对决策很关键但输入缺失时，降低信心度并优先 wait/hold。`,
			TradingFrequency: `# ⏱️ 交易频率意识

- 优秀交易员：每天2-4笔 ≈ 每小时0.1-0.2笔
- 每小时超过2笔 = 过度交易
- 单笔持仓时间 ≥ 30-60分钟
如果你发现自己每个周期都在交易 → 标准太低；如果持仓不到30分钟就平仓 → 太冲动。`,
			EntryStandards: `# 🎯 入场标准（严格）

只在多个信号共振时入场。自由使用任何有效的分析方法，避免单一指标、信号矛盾、横盘震荡、或平仓后立即重新开仓等低质量行为。

对美股挂钩标的还必须：
- 根据当前日期、UTC时间和美国夏令时评估处于盘前/常规时段/盘后/休市；无法确认时不要伪造时段结论
- 对开盘、收盘和休市后重开附近的波动扩张、滑点和跳空保持谨慎，避免追逐首个脉冲
- 将资金费率、OI和成交量视为“该加密场所合约”的信号，不等同于美股现货的机构资金流
- 同一行业或高相关美股挂钩仓位视为集中风险，不因符号不同而误判为分散化`,
			DecisionProcess: `# 📋 决策流程

1. 检查持仓 → 是否止盈/止损
2. 识别每个候选标的的资产类型、交易时段和特有风险
3. 扫描候选标的 + 多时间框架 → 是否存在强信号
4. 输出简洁、可审计的决策依据，再输出结构化JSON`,
		}
	} else {
		config.PromptSections = PromptSectionsConfig{
			RoleDefinition: `# You are a professional multi-asset AI for crypto venues

Trade both crypto-native assets and tokenized US equities or US-equity-linked perpetuals listed on crypto venues. You are skilled in multi-timeframe analysis, derivatives pricing, and risk management.

Classify each instrument before analyzing it. For equity-linked instruments, distinguish the crypto-venue contract from the underlying US cash equity: the contract may trade 24/7, while price discovery, liquidity, and gap risk still depend on pre-market, regular US hours, after-hours, weekends, and market holidays. Do not assume these instruments follow BTC.

Use only data explicitly present in the input. Never invent a live cash-equity price, basis, earnings result, news event, dividend, halt, or market-session status. If missing information is material, lower confidence and prefer wait/hold.`,
			TradingFrequency: `# ⏱️ Trading Frequency Awareness

- Excellent trader: 2-4 trades per day ≈ 0.1-0.2 trades per hour
- >2 trades per hour = overtrading
- Single position holding time ≥ 30-60 minutes
If you find yourself trading every cycle → standards are too low; if closing positions in <30 minutes → too impulsive.`,
			EntryStandards: `# 🎯 Entry Standards (Strict)

Only enter positions when multiple signals resonate. Freely use any effective analysis methods, avoid low-quality behaviors such as single indicators, contradictory signals, sideways oscillation, or immediately restarting after closing positions.

For equity-linked instruments also:
- Evaluate pre-market, regular hours, after-hours, or closure from the date, UTC time, and US daylight-saving rules; do not fabricate a session conclusion when uncertain
- Treat the open, close, and post-closure reopen as elevated volatility, slippage, and gap-risk windows; avoid chasing the first impulse
- Treat funding, OI, and volume as signals for the crypto-venue contract, not as equivalent to institutional cash-equity flow
- Treat positions in the same industry or highly correlated equities as concentrated exposure, not diversification`,
			DecisionProcess: `# 📋 Decision Process

1. Check positions → whether to take profit/stop loss
2. Classify each candidate instrument and identify its session regime and asset-specific risks
3. Scan candidate instruments + multi-timeframe → whether strong signals exist
4. Output concise, auditable decision factors, then structured JSON`,
		}
	}

	return config
}

// Create create a strategy
func (s *StrategyStore) Create(strategy *Strategy) error {
	_, err := s.db.Exec(`
		INSERT INTO strategies (id, user_id, name, description, is_active, is_default, config)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, strategy.ID, strategy.UserID, strategy.Name, strategy.Description, strategy.IsActive, strategy.IsDefault, strategy.Config)
	return err
}

// Update update a strategy
func (s *StrategyStore) Update(strategy *Strategy) error {
	_, err := s.db.Exec(`
		UPDATE strategies SET
			name = ?, description = ?, config = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND user_id = ?
	`, strategy.Name, strategy.Description, strategy.Config, strategy.ID, strategy.UserID)
	return err
}

// Delete delete a strategy
func (s *StrategyStore) Delete(userID, id string) error {
	// do not allow deleting system default strategy
	var isDefault bool
	s.db.QueryRow(`SELECT is_default FROM strategies WHERE id = ?`, id).Scan(&isDefault)
	if isDefault {
		return fmt.Errorf("cannot delete system default strategy")
	}

	_, err := s.db.Exec(`DELETE FROM strategies WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// List get user's strategy list
func (s *StrategyStore) List(userID string) ([]*Strategy, error) {
	// get user's own strategies + system default strategy
	rows, err := s.db.Query(`
		SELECT id, user_id, name, description, is_active, is_default, config, created_at, updated_at
		FROM strategies
		WHERE user_id = ? OR is_default = 1
		ORDER BY is_default DESC, created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var strategies []*Strategy
	for rows.Next() {
		var st Strategy
		var createdAt, updatedAt string
		err := rows.Scan(
			&st.ID, &st.UserID, &st.Name, &st.Description,
			&st.IsActive, &st.IsDefault, &st.Config,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		st.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		st.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		strategies = append(strategies, &st)
	}
	return strategies, nil
}

// Get get a single strategy
func (s *StrategyStore) Get(userID, id string) (*Strategy, error) {
	var st Strategy
	var createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT id, user_id, name, description, is_active, is_default, config, created_at, updated_at
		FROM strategies
		WHERE id = ? AND (user_id = ? OR is_default = 1)
	`, id, userID).Scan(
		&st.ID, &st.UserID, &st.Name, &st.Description,
		&st.IsActive, &st.IsDefault, &st.Config,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	st.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	st.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &st, nil
}

// GetActive get user's currently active strategy
func (s *StrategyStore) GetActive(userID string) (*Strategy, error) {
	var st Strategy
	var createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT id, user_id, name, description, is_active, is_default, config, created_at, updated_at
		FROM strategies
		WHERE user_id = ? AND is_active = 1
	`, userID).Scan(
		&st.ID, &st.UserID, &st.Name, &st.Description,
		&st.IsActive, &st.IsDefault, &st.Config,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		// no active strategy, return system default strategy
		return s.GetDefault()
	}
	if err != nil {
		return nil, err
	}
	st.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	st.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &st, nil
}

// GetDefault get system default strategy
func (s *StrategyStore) GetDefault() (*Strategy, error) {
	var st Strategy
	var createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT id, user_id, name, description, is_active, is_default, config, created_at, updated_at
		FROM strategies
		WHERE is_default = 1
		LIMIT 1
	`).Scan(
		&st.ID, &st.UserID, &st.Name, &st.Description,
		&st.IsActive, &st.IsDefault, &st.Config,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	st.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	st.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &st, nil
}

// SetActive set active strategy (will first deactivate other strategies)
func (s *StrategyStore) SetActive(userID, strategyID string) error {
	// begin transaction
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// first deactivate all strategies for the user
	_, err = tx.Exec(`UPDATE strategies SET is_active = 0 WHERE user_id = ?`, userID)
	if err != nil {
		return err
	}

	// activate specified strategy
	_, err = tx.Exec(`UPDATE strategies SET is_active = 1 WHERE id = ? AND (user_id = ? OR is_default = 1)`, strategyID, userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Duplicate duplicate a strategy (used to create custom strategy based on default strategy)
func (s *StrategyStore) Duplicate(userID, sourceID, newID, newName string) error {
	// get source strategy
	source, err := s.Get(userID, sourceID)
	if err != nil {
		return fmt.Errorf("failed to get source strategy: %w", err)
	}

	// create new strategy
	newStrategy := &Strategy{
		ID:          newID,
		UserID:      userID,
		Name:        newName,
		Description: "Created based on [" + source.Name + "]",
		IsActive:    false,
		IsDefault:   false,
		Config:      source.Config,
	}

	return s.Create(newStrategy)
}

// ParseConfig parse strategy configuration JSON
func (s *Strategy) ParseConfig() (*StrategyConfig, error) {
	var config StrategyConfig
	if err := json.Unmarshal([]byte(s.Config), &config); err != nil {
		return nil, fmt.Errorf("failed to parse strategy configuration: %w", err)
	}
	config.ApplyDefaults()
	return &config, nil
}

// ApplyDefaults normalizes omitted fields, including strategies saved by older versions.
func (config *StrategyConfig) ApplyDefaults() {
	config.CoinSource.SourceType = "static"
	if len(config.CoinSource.StaticCoins) == 0 {
		config.CoinSource.StaticCoins = append([]string(nil), defaultStaticCoins...)
	}
	if config.RiskControl.OrderType == "" {
		config.RiskControl.OrderType = "market"
	}
	if config.RiskControl.LimitPriceOffsetPct <= 0 {
		config.RiskControl.LimitPriceOffsetPct = 0.05
	}
	if config.RiskControl.ProtectionRetries <= 0 {
		config.RiskControl.ProtectionRetries = 3
	}
	if config.RiskControl.ProtectionRetryDelayMs <= 0 {
		config.RiskControl.ProtectionRetryDelayMs = 1000
	}
	if config.RiskControl.ProtectionFailureAction == "" {
		config.RiskControl.ProtectionFailureAction = "close"
	}
	switch config.RiskControl.ProtectionFailureAction {
	case "close", "reduce", "keep_unprotected":
	default:
		config.RiskControl.ProtectionFailureAction = "close"
	}
	if config.RiskControl.ProtectionFailureReducePct <= 0 || config.RiskControl.ProtectionFailureReducePct > 100 {
		config.RiskControl.ProtectionFailureReducePct = 50
	}
	if config.RiskControl.MaxPositionSize <= 0 {
		config.RiskControl.MaxPositionSize = 1000
	}
	if config.RiskControl.MaxTotalPositionSize <= 0 {
		config.RiskControl.MaxTotalPositionSize = 3000
	}
	if config.RiskControl.MaxPositions <= 0 {
		config.RiskControl.MaxPositions = 3
	}
	if config.RiskControl.BTCETHMaxLeverage <= 0 {
		config.RiskControl.BTCETHMaxLeverage = 5
	}
	if config.RiskControl.AltcoinMaxLeverage <= 0 {
		config.RiskControl.AltcoinMaxLeverage = 5
	}
	if config.RiskControl.MinPositionSize <= 0 {
		config.RiskControl.MinPositionSize = 12
	}
	if config.RiskControl.MinRiskRewardRatio <= 0 {
		config.RiskControl.MinRiskRewardRatio = 3
	}
	if config.RiskControl.MinConfidence <= 0 {
		config.RiskControl.MinConfidence = 75
	}
	if config.RiskControl.MaxDailyLossPct <= 0 {
		config.RiskControl.MaxDailyLossPct = 1.5
	}
	if config.RiskControl.MaxDrawdownPct <= 0 {
		config.RiskControl.MaxDrawdownPct = 8
	}
	if config.RiskControl.MaxMarginUsagePct <= 0 {
		config.RiskControl.MaxMarginUsagePct = 80
	}
	if config.RiskControl.MinLiquidationDistancePct <= 0 {
		config.RiskControl.MinLiquidationDistancePct = 5
	}
	if config.RiskControl.MaxConsecutiveFailures <= 0 {
		config.RiskControl.MaxConsecutiveFailures = 3
	}
	if config.RiskControl.MaxMarketMovePct <= 0 {
		config.RiskControl.MaxMarketMovePct = 8
	}
	if config.RiskControl.StopTradingMinutes <= 0 {
		config.RiskControl.StopTradingMinutes = 60
	}
}

// SetConfig set strategy configuration
func (s *Strategy) SetConfig(config *StrategyConfig) error {
	config.ApplyDefaults()
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to serialize strategy configuration: %w", err)
	}
	s.Config = string(data)
	return nil
}
