export interface SystemStatus {
  trader_id: string
  trader_name: string
  ai_model: string
  is_running: boolean
  start_time: string
  runtime_minutes: number
  call_count: number
  initial_balance: number
  scan_interval: string
  stop_until: string
  last_reset_time: string
  ai_provider: string
}

export interface AccountInfo {
  total_equity: number
  wallet_balance: number
  unrealized_profit: number // 未实现盈亏（交易所API官方值）
  available_balance: number
  total_pnl: number
  total_pnl_pct: number
  initial_balance: number
  daily_pnl: number
  position_count: number
  margin_used: number
  margin_used_pct: number
}

export interface Position {
  symbol: string
  side: string
  entry_price: number
  mark_price: number
  quantity: number
  leverage: number
  unrealized_pnl: number
  unrealized_pnl_pct: number
  liquidation_price: number
  margin_used: number
}

export interface DecisionAction {
  action: string
  symbol: string
  quantity: number
  leverage: number
  price: number
  order_id: number
  timestamp: string
  success: boolean
  error?: string
  reasoning?: string
}

export interface AccountSnapshot {
  total_balance: number
  available_balance: number
  total_unrealized_profit: number
  position_count: number
  margin_used_pct: number
}

export interface DecisionRecord {
  timestamp: string
  cycle_number: number
  input_prompt: string
  cot_trace: string
  decision_json: string
  account_state: AccountSnapshot
  positions: any[]
  candidate_coins: string[]
  decisions: DecisionAction[]
  execution_log: string[]
  success: boolean
  error_message?: string
}

export interface Statistics {
  total_cycles: number
  successful_cycles: number
  failed_cycles: number
  total_open_positions: number
  total_close_positions: number
}

// 历史订单（后端 store.TradeOrder + 时间戳）
export interface TradeOrderRecord {
  trader_id: string
  exchange_id: string
  exchange_type: string
  order_id: string
  symbol: string
  position_side: string
  action: string
  requested_qty: number
  executed_qty: number
  avg_price: number
  fee: number
  status: string
  last_error: string
  leverage: number
  stop_loss: number
  take_profit: number
  protected_qty: number
  protection_status: string
  created_at: string
  updated_at: string
}

export interface OrderHistoryResponse {
  orders: TradeOrderRecord[]
  total: number
  limit: number
  offset: number
}

// 成交记录（后端 store.ExchangeFill）
export interface ExchangeFillRecord {
  trade_id: string
  order_id: string
  symbol: string
  position_side: string
  side: string
  quantity: number
  price: number
  fee: number
  realized_pnl: number
  executed_at: string
}

export interface FillHistoryResponse {
  fills: ExchangeFillRecord[]
  total: number
  limit: number
  offset: number
}

// 绩效指标（后端 /performance-metrics）
export interface SymbolPnL {
  symbol: string
  realized_pnl: number
  fee: number
  closed_trades: number
}

export interface ExecutionStats {
  closed_trades: number
  winning_trades: number
  losing_trades: number
  win_rate: number
  total_realized_pnl: number
  total_fee: number
  gross_profit: number
  gross_loss: number
  profit_factor: number
  avg_win: number
  avg_loss: number
  pnl_by_symbol: SymbolPnL[]
}

export interface DrawdownStats {
  max_drawdown_abs: number
  max_drawdown_pct: number
  peak_equity: number
  trough_equity: number
}

export interface PerformanceMetrics {
  execution: ExecutionStats
  drawdown: DrawdownStats
}

// 通知渠道与发送日志（后端 /notifications/*）
export interface NotificationChannel {
  id: string
  name: string
  type: 'telegram' | 'webhook' | 'email'
  enabled: boolean
  config: Record<string, string>
  created_at: string
  updated_at: string
}

export interface NotificationLog {
  id: number
  channel_id: string
  channel_name: string
  channel_type: string
  event: string
  title: string
  message: string
  success: boolean
  error?: string
  created_at: string
}

// 用户资料（后端 /user/profile）
export interface UserProfile {
  user_id: string
  email: string
  otp_verified: boolean
  created_at: string
}

// 管理后台（后端 /admin/*）
export interface AdminUser {
  user_id: string
  email: string
  otp_verified: boolean
  created_at: string
  trader_count: number
  running_count: number
}

export interface AdminSystemStatus {
  users_total: number
  users_verified: number
  traders_total: number
  traders_running: number
  goroutines: number
  heap_alloc_mb: number
  sys_mem_mb: number
  db_size_mb: number
  cpu_count: number
  go_version: string
  server_time: string
}

export interface AdminDecisionFeedItem {
  trader_id: string
  success: boolean
  error?: string
  timestamp: string
  cycle_number: number
}

// AI Trading相关类型
export interface TraderInfo {
  trader_id: string
  trader_name: string
  ai_model: string
  exchange_id?: string
  is_running?: boolean
  show_in_competition?: boolean
  strategy_id?: string
  strategy_name?: string
  custom_prompt?: string
  use_coin_pool?: boolean
  use_oi_top?: boolean
  system_prompt_template?: string
}

export interface AIModel {
  id: string
  name: string
  provider: string
  enabled: boolean
  apiKey?: string
  customApiUrl?: string
  customModelName?: string
}

export interface Exchange {
  id: string // UUID (empty for supported exchange templates)
  exchange_type: string // "binance"
  account_name: string // User-defined account name
  name: string // Display name
  type: 'cex' | 'dex' | 'sim'
  enabled: boolean
  apiKey?: string
  secretKey?: string
  testnet?: boolean
}

export interface CreateExchangeRequest {
  exchange_type: string // "binance"
  account_name: string // User-defined account name
  enabled: boolean
  api_key?: string
  secret_key?: string
  testnet?: boolean
}

export interface CreateTraderRequest {
  name: string
  ai_model_id: string
  exchange_id: string
  strategy_id?: string // 策略ID（新版，使用保存的策略配置）
  initial_balance?: number // 可选：创建时由后端自动获取，编辑时可手动更新
  scan_interval_minutes?: number
  is_cross_margin?: boolean
  show_in_competition?: boolean // 是否在竞技场显示
  // 以下字段为向后兼容保留，新版使用策略配置
  btc_eth_leverage?: number
  altcoin_leverage?: number
  trading_symbols?: string
  custom_prompt?: string
  override_base_prompt?: boolean
  system_prompt_template?: string
  use_coin_pool?: boolean
  use_oi_top?: boolean
}

export interface UpdateModelConfigRequest {
  models: {
    [key: string]: {
      enabled: boolean
      api_key: string
      custom_api_url?: string
      custom_model_name?: string
    }
  }
}

export interface UpdateExchangeConfigRequest {
  exchanges: {
    [key: string]: {
      enabled: boolean
      api_key: string
      secret_key: string
      testnet?: boolean
    }
  }
}

// Competition related types
export interface CompetitionTraderData {
  trader_id: string
  trader_name: string
  ai_model: string
  exchange: string
  total_equity: number
  total_pnl: number
  total_pnl_pct: number
  position_count: number
  margin_used_pct: number
  is_running: boolean
}

export interface CompetitionData {
  traders: CompetitionTraderData[]
  count: number
}

// Trader Configuration Data for View Modal
export interface TraderConfigData {
  trader_id?: string
  trader_name: string
  ai_model: string
  exchange_id: string
  strategy_id?: string // 策略ID
  strategy_name?: string // 策略名称
  is_cross_margin: boolean
  show_in_competition: boolean // 是否在竞技场显示
  scan_interval_minutes: number
  initial_balance: number
  is_running: boolean
  // 以下为旧版字段（向后兼容）
  btc_eth_leverage?: number
  altcoin_leverage?: number
  trading_symbols?: string
  custom_prompt?: string
  override_base_prompt?: boolean
  system_prompt_template?: string
  use_coin_pool?: boolean
  use_oi_top?: boolean
}

// Backtest types
export interface BacktestRunSummary {
  symbol_count: number
  decision_tf: string
  processed_bars: number
  progress_pct: number
  equity_last: number
  max_drawdown_pct: number
  liquidated: boolean
  liquidation_note?: string
}

export interface BacktestRunMetadata {
  run_id: string
  label?: string
  user_id?: string
  last_error?: string
  version: number
  state: string
  created_at: string
  updated_at: string
  summary: BacktestRunSummary
}

export interface BacktestRunsResponse {
  total: number
  items: BacktestRunMetadata[]
}

export interface BacktestStatusPayload {
  run_id: string
  state: string
  progress_pct: number
  processed_bars: number
  current_time: number
  decision_cycle: number
  equity: number
  unrealized_pnl: number
  realized_pnl: number
  note?: string
  last_error?: string
  last_updated_iso: string
}

export interface BacktestEquityPoint {
  ts: number
  equity: number
  available: number
  pnl: number
  pnl_pct: number
  dd_pct: number
  cycle: number
}

export interface BacktestTradeEvent {
  ts: number
  symbol: string
  action: string
  side?: string
  qty: number
  price: number
  fee: number
  slippage: number
  order_value: number
  realized_pnl: number
  leverage?: number
  cycle: number
  position_after: number
  liquidation: boolean
  note?: string
}

export interface BacktestMetrics {
  total_return_pct: number
  max_drawdown_pct: number
  sharpe_ratio: number
  profit_factor: number
  win_rate: number
  trades: number
  avg_win: number
  avg_loss: number
  best_symbol: string
  worst_symbol: string
  liquidated: boolean
  symbol_stats?: Record<
    string,
    {
      total_trades: number
      winning_trades: number
      losing_trades: number
      total_pnl: number
      avg_pnl: number
      win_rate: number
    }
  >
}

export interface BacktestStartConfig {
  run_id?: string
  ai_model_id?: string
  symbols: string[]
  timeframes: string[]
  decision_timeframe: string
  decision_cadence_nbars: number
  start_ts: number
  end_ts: number
  initial_balance: number
  fee_bps: number
  slippage_bps: number
  fill_policy: string
  prompt_variant?: string
  prompt_template?: string
  custom_prompt?: string
  override_prompt?: boolean
  cache_ai?: boolean
  replay_only?: boolean
  checkpoint_interval_bars?: number
  checkpoint_interval_seconds?: number
  replay_decision_dir?: string
  shared_ai_cache_path?: string
  ai?: {
    provider?: string
    model?: string
    key?: string
    secret_key?: string
    base_url?: string
  }
  leverage?: {
    btc_eth_leverage?: number
    altcoin_leverage?: number
  }
}

// Strategy Studio Types
export interface Strategy {
  id: string
  name: string
  description: string
  is_active: boolean
  is_default: boolean
  config: StrategyConfig
  created_at: string
  updated_at: string
}

export interface PromptSectionsConfig {
  role_definition?: string
  trading_frequency?: string
  entry_standards?: string
  decision_process?: string
}

export interface StrategyConfig {
  coin_source: CoinSourceConfig
  indicators: IndicatorConfig
  custom_prompt?: string
  risk_control: RiskControlConfig
  prompt_sections?: PromptSectionsConfig
}

export interface CoinSourceConfig {
  source_type: 'static'
  static_coins?: string[]
}

export interface IndicatorConfig {
  klines: KlineConfig
  // Raw OHLCV kline data - required for AI analysis
  enable_raw_klines: boolean
  // Technical indicators (optional)
  enable_ema: boolean
  enable_macd: boolean
  enable_rsi: boolean
  enable_atr: boolean
  enable_volume: boolean
  enable_oi: boolean
  enable_funding_rate: boolean
  ema_periods?: number[]
  rsi_periods?: number[]
  atr_periods?: number[]
  external_data_sources?: ExternalDataSource[]
  // 量化数据源（资金流向、持仓变化、价格变化）
  enable_quant_data?: boolean
  quant_data_api_url?: string
  enable_quant_oi?: boolean
  enable_quant_netflow?: boolean
  // OI 排行数据（市场持仓量增减排行）
  enable_oi_ranking?: boolean
  oi_ranking_api_url?: string
  oi_ranking_duration?: string // "1h", "4h", "24h"
  oi_ranking_limit?: number
}

export interface KlineConfig {
  primary_timeframe: string
  primary_count: number
  longer_timeframe?: string
  longer_count?: number
  enable_multi_timeframe: boolean
  // 新增：支持选择多个时间周期
  selected_timeframes?: string[]
}

export interface ExternalDataSource {
  name: string
  type: 'api' | 'webhook'
  url: string
  method: string
  headers?: Record<string, string>
  data_path?: string
  refresh_secs?: number
}

export interface RiskControlConfig {
  // Max number of coins held simultaneously (CODE ENFORCED)
  max_positions: number

  // Trading Leverage - exchange leverage for opening positions (AI guided)
  btc_eth_max_leverage: number // BTC/ETH max exchange leverage
  altcoin_max_leverage: number // Altcoin max exchange leverage

  max_position_size: number // Max notional value of one opening order
  max_total_position_size: number // Max total open-position notional value

  // Risk Parameters
  min_position_size: number // Min position size in USDT (CODE ENFORCED)
  min_risk_reward_ratio: number // Min take_profit / stop_loss ratio (AI guided)
  max_daily_loss_pct?: number // Account daily mark-to-market loss circuit breaker
  max_drawdown_pct?: number // Equity high-water drawdown circuit breaker
  max_margin_usage_pct?: number // Projected account margin usage ceiling
  min_liquidation_distance_pct?: number // Minimum mark-to-liquidation distance
  max_consecutive_failures?: number // Consecutive execution failure circuit breaker
  max_market_move_pct?: number // 1h/4h abnormal move circuit breaker
  stop_trading_minutes?: number // Circuit breaker pause duration
  order_type?: 'market' | 'limit' // Opening order type; close orders remain market
  limit_price_offset_pct?: number // Limit order offset from current price in percent
  pending_order_timeout_sec?: number // How long a resting limit entry is monitored before cancel
  pending_order_poll_sec?: number // Status polling interval for pending limit entries
  // Trailing profit protection (backend-enforced, once per minute)
  enable_trailing_profit_exit?: boolean // Default true; explicitly false disables
  trailing_profit_trigger_pct?: number // Arm once leveraged unrealized profit ≥ this %
  trailing_profit_giveback_pct?: number // Close when profit retraces this % from peak
}

// ==================== 通知系统 (P0) ====================

// 通知记录（站内信）
export interface NotificationRecord {
  id: string
  user_id: string
  trader_id?: string
  trader_name?: string
  event_type: string
  severity: 'info' | 'warning' | 'critical'
  title: string
  body: string
  is_read: boolean
  created_at: string
}

// 通知渠道设置
export interface NotificationSettings {
  user_id: string
  enabled: boolean

  telegram_enabled: boolean
  telegram_bot_token?: string
  telegram_chat_id?: string

  webhook_enabled: boolean
  webhook_url?: string
  webhook_secret?: string

  email_enabled: boolean
  email_to?: string
  smtp_host?: string
  smtp_port?: number
  smtp_username?: string
  smtp_password?: string
  smtp_from?: string
  smtp_use_tls?: boolean

  event_subscriptions?: Record<string, boolean>
  quiet_hours_start_utc?: number
  quiet_hours_end_utc?: number
}

// 审计日志（P0）
export interface AuditEvent {
  id: string
  user_id: string
  email: string
  action: string
  resource_type: string
  resource_id: string
  detail: string
  status: 'success' | 'failure'
  ip: string
  user_agent: string
  created_at: string
}

// 备份记录（P1）
export interface BackupRecord {
  path: string
  size_bytes: number
  status: string
  error: string
  created_at: string
}
