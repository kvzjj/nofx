import type {
  SystemStatus,
  AccountInfo,
  Position,
  DecisionRecord,
  Statistics,
  TraderInfo,
  TraderConfigData,
  AIModel,
  Exchange,
  CreateTraderRequest,
  CreateExchangeRequest,
  UpdateModelConfigRequest,
  UpdateExchangeConfigRequest,
  CompetitionData,
  BacktestRunsResponse,
  BacktestStartConfig,
  BacktestStatusPayload,
  BacktestEquityPoint,
  BacktestTradeEvent,
  BacktestMetrics,
  BacktestRunMetadata,
  Strategy,
  StrategyConfig,
  NotificationRecord,
  NotificationSettings,
  AuditEvent,
  BackupRecord,
  OrderHistoryResponse,
  FillHistoryResponse,
  PerformanceMetrics,
  NotificationChannel,
  NotificationLog,
  UserProfile,
  AdminUser,
  AdminSystemStatus,
  AdminDecisionFeedItem,
} from '../types'
import { CryptoService } from './crypto'
import { httpClient } from './httpClient'

const API_BASE = '/api'

// Helper function to get auth headers
function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem('auth_token')
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }

  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  return headers
}

async function handleJSONResponse<T>(res: Response): Promise<T> {
  const text = await res.text()
  if (!res.ok) {
    let message = text || res.statusText
    try {
      const data = text ? JSON.parse(text) : null
      if (data && typeof data === 'object') {
        message = data.error || data.message || message
      }
    } catch {
      /* ignore JSON parse errors */
    }
    throw new Error(message || '请求失败')
  }
  if (!text) {
    return {} as T
  }
  return JSON.parse(text) as T
}

export const api = {
  // AI交易员管理接口
  async getTraders(): Promise<TraderInfo[]> {
    const result = await httpClient.get<TraderInfo[]>(`${API_BASE}/my-traders`)
    if (!result.success) throw new Error('获取trader列表失败')
    return Array.isArray(result.data) ? result.data : []
  },

  // 获取公开的交易员列表（无需认证）
  async getPublicTraders(): Promise<any[]> {
    const result = await httpClient.get<any[]>(`${API_BASE}/traders`)
    if (!result.success) throw new Error('获取公开trader列表失败')
    return result.data!
  },

  async getPerformance(traderId?: string): Promise<any> {
    const query = traderId ? `?trader_id=${encodeURIComponent(traderId)}` : ''
    const result = await httpClient.get<any>(`${API_BASE}/performance${query}`)
    if (!result.success) throw new Error('获取交易表现失败')
    return result.data!
  },

  async createTrader(request: CreateTraderRequest): Promise<TraderInfo> {
    const result = await httpClient.post<TraderInfo>(
      `${API_BASE}/traders`,
      request
    )
    if (!result.success) throw new Error('创建交易员失败')
    return result.data!
  },

  async deleteTrader(traderId: string): Promise<void> {
    const result = await httpClient.delete(`${API_BASE}/traders/${traderId}`)
    if (!result.success) throw new Error('删除交易员失败')
  },

  async startTrader(traderId: string): Promise<void> {
    const result = await httpClient.post(
      `${API_BASE}/traders/${traderId}/start`
    )
    if (!result.success) throw new Error('启动交易员失败')
  },

  async stopTrader(traderId: string): Promise<void> {
    const result = await httpClient.post(`${API_BASE}/traders/${traderId}/stop`)
    if (!result.success) throw new Error('停止交易员失败')
  },

  async resetPaperAccount(
    traderId: string
  ): Promise<{ message: string; initial_balance: number }> {
    const result = await httpClient.post<{
      message: string
      initial_balance: number
    }>(`${API_BASE}/traders/${traderId}/reset-paper`)
    if (!result.success) throw new Error('重置模拟账户失败')
    return result.data!
  },

  // 同步交易所余额到初始资金（真实交易所账户）
  async syncBalance(traderId: string): Promise<{
    message: string
    old_balance: number
    new_balance: number
    change_percent: number
    change_type: string
  }> {
    const result = await httpClient.post<{
      message: string
      old_balance: number
      new_balance: number
      change_percent: number
      change_type: string
    }>(`${API_BASE}/traders/${traderId}/sync-balance`)
    if (!result.success) throw new Error('同步余额失败')
    return result.data!
  },

  async toggleCompetition(
    traderId: string,
    showInCompetition: boolean
  ): Promise<void> {
    const result = await httpClient.put(
      `${API_BASE}/traders/${traderId}/competition`,
      { show_in_competition: showInCompetition }
    )
    if (!result.success) throw new Error('更新竞技场显示设置失败')
  },

  async closePosition(
    traderId: string,
    symbol: string,
    side: string
  ): Promise<{ message: string }> {
    const result = await httpClient.post<{ message: string }>(
      `${API_BASE}/traders/${traderId}/close-position`,
      { symbol, side }
    )
    if (!result.success) throw new Error('平仓失败')
    return result.data!
  },

  // 手动下单（与 AI 决策共用同一执行常风控/仓位计算/保护单链路）
  async manualOrder(
    traderId: string,
    request: {
      symbol: string
      action: 'open_long' | 'open_short' | 'close_long' | 'close_short'
      stop_loss_pct?: number
      take_profit_pct?: number
    }
  ): Promise<{ message: string }> {
    const result = await httpClient.post<{ message: string }>(
      `${API_BASE}/traders/${traderId}/manual-order`,
      request
    )
    if (!result.success) throw new Error('手动下单失败')
    return result.data!
  },

  async updateTraderPrompt(
    traderId: string,
    customPrompt: string
  ): Promise<void> {
    const result = await httpClient.put(
      `${API_BASE}/traders/${traderId}/prompt`,
      { custom_prompt: customPrompt }
    )
    if (!result.success) throw new Error('更新自定义策略失败')
  },

  async getTraderConfig(traderId: string): Promise<TraderConfigData> {
    const result = await httpClient.get<TraderConfigData>(
      `${API_BASE}/traders/${traderId}/config`
    )
    if (!result.success) throw new Error('获取交易员配置失败')
    return result.data!
  },

  async updateTrader(
    traderId: string,
    request: CreateTraderRequest
  ): Promise<TraderInfo> {
    const result = await httpClient.put<TraderInfo>(
      `${API_BASE}/traders/${traderId}`,
      request
    )
    if (!result.success) throw new Error('更新交易员失败')
    return result.data!
  },

  // AI模型配置接口
  async getModelConfigs(): Promise<AIModel[]> {
    const result = await httpClient.get<AIModel[]>(`${API_BASE}/models`)
    if (!result.success) throw new Error('获取模型配置失败')
    return Array.isArray(result.data) ? result.data : []
  },

  // 获取系统支持的AI模型列表（无需认证）
  async getSupportedModels(): Promise<AIModel[]> {
    const result = await httpClient.get<AIModel[]>(
      `${API_BASE}/supported-models`
    )
    if (!result.success) throw new Error('获取支持的模型失败')
    return result.data!
  },

  async getPromptTemplates(): Promise<string[]> {
    const res = await fetch(`${API_BASE}/prompt-templates`)
    if (!res.ok) throw new Error('获取提示词模板失败')
    const data = await res.json()
    if (Array.isArray(data.templates)) {
      return data.templates.map((item: { name: string }) => item.name)
    }
    return []
  },

  async updateModelConfigs(request: UpdateModelConfigRequest): Promise<void> {
    // 检查是否启用了传输加密
    const config = await CryptoService.fetchCryptoConfig()

    if (!config.transport_encryption) {
      // 传输加密禁用时，直接发送明文
      const result = await httpClient.put(`${API_BASE}/models`, request)
      if (!result.success) throw new Error('更新模型配置失败')
      return
    }

    // 获取RSA公钥
    const publicKey = await CryptoService.fetchPublicKey()

    // 初始化加密服务
    await CryptoService.initialize(publicKey)

    // 获取用户信息（从localStorage或其他地方）
    const userId = localStorage.getItem('user_id') || ''
    const sessionId = sessionStorage.getItem('session_id') || ''

    // 加密敏感数据
    const encryptedPayload = await CryptoService.encryptSensitiveData(
      JSON.stringify(request),
      userId,
      sessionId
    )

    // 发送加密数据
    const result = await httpClient.put(`${API_BASE}/models`, encryptedPayload)
    if (!result.success) throw new Error('更新模型配置失败')
  },

  // 交易所配置接口
  async getExchangeConfigs(): Promise<Exchange[]> {
    const result = await httpClient.get<Exchange[]>(`${API_BASE}/exchanges`)
    if (!result.success) throw new Error('获取交易所配置失败')
    return result.data!
  },

  // 获取系统支持的交易所列表（无需认证）
  async getSupportedExchanges(): Promise<Exchange[]> {
    const result = await httpClient.get<Exchange[]>(
      `${API_BASE}/supported-exchanges`
    )
    if (!result.success) throw new Error('获取支持的交易所失败')
    return result.data!
  },

  async updateExchangeConfigs(
    request: UpdateExchangeConfigRequest
  ): Promise<void> {
    const result = await httpClient.put(`${API_BASE}/exchanges`, request)
    if (!result.success) throw new Error('更新交易所配置失败')
  },

  // 创建新的交易所账户
  async createExchange(
    request: CreateExchangeRequest
  ): Promise<{ id: string }> {
    const result = await httpClient.post<{ id: string }>(
      `${API_BASE}/exchanges`,
      request
    )
    if (!result.success) throw new Error('创建交易所账户失败')
    return result.data!
  },

  // 创建新的交易所账户（加密传输）
  async createExchangeEncrypted(
    request: CreateExchangeRequest
  ): Promise<{ id: string }> {
    // 检查是否启用了传输加密
    const config = await CryptoService.fetchCryptoConfig()

    if (!config.transport_encryption) {
      // 传输加密禁用时，直接发送明文
      const result = await httpClient.post<{ id: string }>(
        `${API_BASE}/exchanges`,
        request
      )
      if (!result.success) throw new Error('创建交易所账户失败')
      return result.data!
    }

    // 获取RSA公钥
    const publicKey = await CryptoService.fetchPublicKey()

    // 初始化加密服务
    await CryptoService.initialize(publicKey)

    // 获取用户信息
    const userId = localStorage.getItem('user_id') || ''
    const sessionId = sessionStorage.getItem('session_id') || ''

    // 加密敏感数据
    const encryptedPayload = await CryptoService.encryptSensitiveData(
      JSON.stringify(request),
      userId,
      sessionId
    )

    // 发送加密数据
    const result = await httpClient.post<{ id: string }>(
      `${API_BASE}/exchanges`,
      encryptedPayload
    )
    if (!result.success) throw new Error('创建交易所账户失败')
    return result.data!
  },

  // 删除交易所账户
  async deleteExchange(exchangeId: string): Promise<void> {
    const result = await httpClient.delete(
      `${API_BASE}/exchanges/${exchangeId}`
    )
    if (!result.success) throw new Error('删除交易所账户失败')
  },

  // 使用加密传输更新交易所配置（自动检测是否启用加密）
  async updateExchangeConfigsEncrypted(
    request: UpdateExchangeConfigRequest
  ): Promise<void> {
    // 检查是否启用了传输加密
    const config = await CryptoService.fetchCryptoConfig()

    if (!config.transport_encryption) {
      // 传输加密禁用时，直接发送明文
      const result = await httpClient.put(`${API_BASE}/exchanges`, request)
      if (!result.success) throw new Error('更新交易所配置失败')
      return
    }

    // 获取RSA公钥
    const publicKey = await CryptoService.fetchPublicKey()

    // 初始化加密服务
    await CryptoService.initialize(publicKey)

    // 获取用户信息（从localStorage或其他地方）
    const userId = localStorage.getItem('user_id') || ''
    const sessionId = sessionStorage.getItem('session_id') || ''

    // 加密敏感数据
    const encryptedPayload = await CryptoService.encryptSensitiveData(
      JSON.stringify(request),
      userId,
      sessionId
    )

    // 发送加密数据
    const result = await httpClient.put(
      `${API_BASE}/exchanges`,
      encryptedPayload
    )
    if (!result.success) throw new Error('更新交易所配置失败')
  },

  // 获取系统状态（支持trader_id）
  async getStatus(traderId?: string): Promise<SystemStatus> {
    const url = traderId
      ? `${API_BASE}/status?trader_id=${traderId}`
      : `${API_BASE}/status`
    const result = await httpClient.get<SystemStatus>(url)
    if (!result.success) throw new Error('获取系统状态失败')
    return result.data!
  },

  // 获取账户信息（支持trader_id）
  async getAccount(traderId?: string): Promise<AccountInfo> {
    const url = traderId
      ? `${API_BASE}/account?trader_id=${traderId}`
      : `${API_BASE}/account`
    const result = await httpClient.get<AccountInfo>(url)
    if (!result.success) throw new Error('获取账户信息失败')
    return result.data!
  },

  // 获取持仓列表（支持trader_id）
  async getPositions(traderId?: string): Promise<Position[]> {
    const url = traderId
      ? `${API_BASE}/positions?trader_id=${traderId}`
      : `${API_BASE}/positions`
    const result = await httpClient.get<Position[]>(url)
    if (!result.success) throw new Error('获取持仓列表失败')
    return result.data!
  },

  // 获取决策日志（支持trader_id）
  async getDecisions(traderId?: string): Promise<DecisionRecord[]> {
    const url = traderId
      ? `${API_BASE}/decisions?trader_id=${traderId}`
      : `${API_BASE}/decisions`
    const result = await httpClient.get<DecisionRecord[]>(url)
    if (!result.success) throw new Error('获取决策日志失败')
    return result.data!
  },

  // 获取最新决策（支持trader_id和limit参数）
  async getLatestDecisions(
    traderId?: string,
    limit: number = 5
  ): Promise<DecisionRecord[]> {
    const params = new URLSearchParams()
    if (traderId) {
      params.append('trader_id', traderId)
    }
    params.append('limit', limit.toString())

    const result = await httpClient.get<DecisionRecord[]>(
      `${API_BASE}/decisions/latest?${params}`
    )
    if (!result.success) throw new Error('获取最新决策失败')
    return result.data!
  },

  // 获取统计信息（支持trader_id）
  async getStatistics(traderId?: string): Promise<Statistics> {
    const url = traderId
      ? `${API_BASE}/statistics?trader_id=${traderId}`
      : `${API_BASE}/statistics`
    const result = await httpClient.get<Statistics>(url)
    if (!result.success) throw new Error('获取统计信息失败')
    return result.data!
  },

  // 获取历史订单（支持分页）
  async getOrderHistory(
    traderId?: string,
    limit: number = 50,
    offset: number = 0
  ): Promise<OrderHistoryResponse> {
    const params = new URLSearchParams()
    if (traderId) params.append('trader_id', traderId)
    params.append('limit', limit.toString())
    params.append('offset', offset.toString())
    const result = await httpClient.get<OrderHistoryResponse>(
      `${API_BASE}/orders?${params}`
    )
    if (!result.success) throw new Error('获取历史订单失败')
    return result.data!
  },

  // 获取成交记录（支持分页）
  async getFillHistory(
    traderId?: string,
    limit: number = 50,
    offset: number = 0
  ): Promise<FillHistoryResponse> {
    const params = new URLSearchParams()
    if (traderId) params.append('trader_id', traderId)
    params.append('limit', limit.toString())
    params.append('offset', offset.toString())
    const result = await httpClient.get<FillHistoryResponse>(
      `${API_BASE}/fills?${params}`
    )
    if (!result.success) throw new Error('获取成交记录失败')
    return result.data!
  },

  // 获取绩效指标（最大回撤/胜率/盈亏比/按币种PnL）
  async getPerformanceMetrics(traderId?: string): Promise<PerformanceMetrics> {
    const url = traderId
      ? `${API_BASE}/performance-metrics?trader_id=${traderId}`
      : `${API_BASE}/performance-metrics`
    const result = await httpClient.get<PerformanceMetrics>(url)
    if (!result.success) throw new Error('获取绩效指标失败')
    return result.data!
  },

  // ============ 通知渠道 ============
  async getNotificationChannels(): Promise<NotificationChannel[]> {
    const result = await httpClient.get<NotificationChannel[]>(
      `${API_BASE}/notifications/channels`
    )
    if (!result.success) throw new Error('获取通知渠道失败')
    return result.data ?? []
  },

  async createNotificationChannel(request: {
    name: string
    type: string
    config: Record<string, string>
    enabled: boolean
  }): Promise<NotificationChannel> {
    const result = await httpClient.post<NotificationChannel>(
      `${API_BASE}/notifications/channels`,
      request
    )
    if (!result.success) throw new Error('创建通知渠道失败')
    return result.data!
  },

  async updateNotificationChannel(
    id: string,
    request: {
      name: string
      type: string
      config: Record<string, string>
      enabled: boolean
    }
  ): Promise<NotificationChannel> {
    const result = await httpClient.put<NotificationChannel>(
      `${API_BASE}/notifications/channels/${id}`,
      request
    )
    if (!result.success) throw new Error('更新通知渠道失败')
    return result.data!
  },

  async deleteNotificationChannel(id: string): Promise<void> {
    const result = await httpClient.delete(
      `${API_BASE}/notifications/channels/${id}`
    )
    if (!result.success) throw new Error('删除通知渠道失败')
  },

  async testNotificationChannel(id: string): Promise<void> {
    const result = await httpClient.post(
      `${API_BASE}/notifications/channels/${id}/test`
    )
    if (!result.success) throw new Error('发送测试通知失败')
  },

  async getNotificationLogs(): Promise<NotificationLog[]> {
    const result = await httpClient.get<NotificationLog[]>(
      `${API_BASE}/notifications/logs`
    )
    if (!result.success) throw new Error('获取发送日志失败')
    return result.data ?? []
  },

  // ============ 用户设置 ============
  async getUserProfile(): Promise<UserProfile> {
    const result = await httpClient.get<UserProfile>(`${API_BASE}/user/profile`)
    if (!result.success) throw new Error('获取用户信息失败')
    return result.data!
  },

  async changePassword(
    oldPassword: string,
    newPassword: string
  ): Promise<void> {
    const result = await httpClient.put(`${API_BASE}/user/password`, {
      old_password: oldPassword,
      new_password: newPassword,
    })
    if (!result.success) throw new Error('修改密码失败')
  },

  async reset2FA(): Promise<{ secret: string; qr_code_url: string }> {
    const result = await httpClient.post<{
      secret: string
      qr_code_url: string
    }>(`${API_BASE}/user/2fa/reset`)
    if (!result.success) throw new Error('重置 2FA 失败')
    return result.data!
  },

  async confirm2FA(code: string): Promise<void> {
    const result = await httpClient.post(`${API_BASE}/user/2fa/confirm`, {
      code,
    })
    if (!result.success) throw new Error('验证失败')
  },

  // ============ 管理后台 ============
  async getAdminUsers(): Promise<AdminUser[]> {
    const result = await httpClient.get<AdminUser[]>(`${API_BASE}/admin/users`)
    if (!result.success) throw new Error('获取用户列表失败')
    return result.data ?? []
  },

  async getAdminSystemStatus(): Promise<AdminSystemStatus> {
    const result = await httpClient.get<AdminSystemStatus>(
      `${API_BASE}/admin/system-status`
    )
    if (!result.success) throw new Error('获取系统状态失败')
    return result.data!
  },

  async getAdminRecentDecisions(): Promise<AdminDecisionFeedItem[]> {
    const result = await httpClient.get<AdminDecisionFeedItem[]>(
      `${API_BASE}/admin/recent-decisions`
    )
    if (!result.success) throw new Error('获取决策动态失败')
    return result.data ?? []
  },

  // 获取收益率历史数据（支持trader_id）
  async getEquityHistory(traderId?: string): Promise<any[]> {
    const url = traderId
      ? `${API_BASE}/equity-history?trader_id=${traderId}`
      : `${API_BASE}/equity-history`
    const result = await httpClient.get<any[]>(url)
    if (!result.success) throw new Error('获取历史数据失败')
    return result.data!
  },

  // 批量获取多个交易员的历史数据（无需认证）
  async getEquityHistoryBatch(traderIds: string[]): Promise<any> {
    const result = await httpClient.post<any>(
      `${API_BASE}/equity-history-batch`,
      { trader_ids: traderIds }
    )
    if (!result.success) throw new Error('获取批量历史数据失败')
    return result.data!
  },

  // 获取前5名交易员数据（无需认证）
  async getTopTraders(): Promise<any[]> {
    const result = await httpClient.get<any[]>(`${API_BASE}/top-traders`)
    if (!result.success) throw new Error('获取前5名交易员失败')
    return result.data!
  },

  // 获取公开交易员配置（无需认证）
  async getPublicTraderConfig(traderId: string): Promise<any> {
    const result = await httpClient.get<any>(
      `${API_BASE}/trader/${traderId}/config`
    )
    if (!result.success) throw new Error('获取公开交易员配置失败')
    return result.data!
  },

  // 获取竞赛数据（无需认证）
  async getCompetition(): Promise<CompetitionData> {
    const result = await httpClient.get<CompetitionData>(
      `${API_BASE}/competition`
    )
    if (!result.success) throw new Error('获取竞赛数据失败')
    return result.data!
  },

  // 获取服务器IP（需要认证，用于白名单配置）
  async getServerIP(): Promise<{
    public_ip: string
    message: string
  }> {
    const result = await httpClient.get<{
      public_ip: string
      message: string
    }>(`${API_BASE}/server-ip`)
    if (!result.success) throw new Error('获取服务器IP失败')
    return result.data!
  },

  // Backtest APIs
  async getBacktestRuns(params?: {
    state?: string
    search?: string
    limit?: number
    offset?: number
  }): Promise<BacktestRunsResponse> {
    const query = new URLSearchParams()
    if (params?.state) query.set('state', params.state)
    if (params?.search) query.set('search', params.search)
    if (params?.limit) query.set('limit', String(params.limit))
    if (params?.offset) query.set('offset', String(params.offset))
    const res = await fetch(
      `${API_BASE}/backtest/runs${query.toString() ? `?${query}` : ''}`,
      {
        headers: getAuthHeaders(),
      }
    )
    return handleJSONResponse<BacktestRunsResponse>(res)
  },

  async startBacktest(
    config: BacktestStartConfig
  ): Promise<BacktestRunMetadata> {
    const res = await fetch(`${API_BASE}/backtest/start`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ config }),
    })
    return handleJSONResponse<BacktestRunMetadata>(res)
  },

  async pauseBacktest(runId: string): Promise<BacktestRunMetadata> {
    const res = await fetch(`${API_BASE}/backtest/pause`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ run_id: runId }),
    })
    return handleJSONResponse<BacktestRunMetadata>(res)
  },

  async resumeBacktest(runId: string): Promise<BacktestRunMetadata> {
    const res = await fetch(`${API_BASE}/backtest/resume`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ run_id: runId }),
    })
    return handleJSONResponse<BacktestRunMetadata>(res)
  },

  async stopBacktest(runId: string): Promise<BacktestRunMetadata> {
    const res = await fetch(`${API_BASE}/backtest/stop`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ run_id: runId }),
    })
    return handleJSONResponse<BacktestRunMetadata>(res)
  },

  async updateBacktestLabel(
    runId: string,
    label: string
  ): Promise<BacktestRunMetadata> {
    const res = await fetch(`${API_BASE}/backtest/label`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ run_id: runId, label }),
    })
    return handleJSONResponse<BacktestRunMetadata>(res)
  },

  async deleteBacktestRun(runId: string): Promise<void> {
    const res = await fetch(`${API_BASE}/backtest/delete`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ run_id: runId }),
    })
    if (!res.ok) {
      throw new Error(await res.text())
    }
  },

  async getBacktestStatus(runId: string): Promise<BacktestStatusPayload> {
    const res = await fetch(`${API_BASE}/backtest/status?run_id=${runId}`, {
      headers: getAuthHeaders(),
    })
    return handleJSONResponse<BacktestStatusPayload>(res)
  },

  async getBacktestEquity(
    runId: string,
    timeframe?: string,
    limit?: number
  ): Promise<BacktestEquityPoint[]> {
    const query = new URLSearchParams({ run_id: runId })
    if (timeframe) query.set('tf', timeframe)
    if (limit) query.set('limit', String(limit))
    const res = await fetch(`${API_BASE}/backtest/equity?${query}`, {
      headers: getAuthHeaders(),
    })
    return handleJSONResponse<BacktestEquityPoint[]>(res)
  },

  async getBacktestTrades(
    runId: string,
    limit = 200
  ): Promise<BacktestTradeEvent[]> {
    const query = new URLSearchParams({
      run_id: runId,
      limit: String(limit),
    })
    const res = await fetch(`${API_BASE}/backtest/trades?${query}`, {
      headers: getAuthHeaders(),
    })
    return handleJSONResponse<BacktestTradeEvent[]>(res)
  },

  async getBacktestMetrics(runId: string): Promise<BacktestMetrics> {
    const res = await fetch(`${API_BASE}/backtest/metrics?run_id=${runId}`, {
      headers: getAuthHeaders(),
    })
    return handleJSONResponse<BacktestMetrics>(res)
  },

  async getBacktestTrace(
    runId: string,
    cycle?: number
  ): Promise<DecisionRecord> {
    const query = new URLSearchParams({ run_id: runId })
    if (cycle) query.set('cycle', String(cycle))
    const res = await fetch(`${API_BASE}/backtest/trace?${query}`, {
      headers: getAuthHeaders(),
    })
    return handleJSONResponse<DecisionRecord>(res)
  },

  async getBacktestDecisions(
    runId: string,
    limit = 20,
    offset = 0
  ): Promise<DecisionRecord[]> {
    const query = new URLSearchParams({
      run_id: runId,
      limit: String(limit),
      offset: String(offset),
    })
    const res = await fetch(`${API_BASE}/backtest/decisions?${query}`, {
      headers: getAuthHeaders(),
    })
    return handleJSONResponse<DecisionRecord[]>(res)
  },

  async exportBacktest(runId: string): Promise<Blob> {
    const res = await fetch(`${API_BASE}/backtest/export?run_id=${runId}`, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) {
      const text = await res.text()
      try {
        const data = text ? JSON.parse(text) : null
        throw new Error(
          data?.error || data?.message || text || '导出失败，请稍后再试'
        )
      } catch (err) {
        if (err instanceof Error && err.message) {
          throw err
        }
        throw new Error(text || '导出失败，请稍后再试')
      }
    }
    return res.blob()
  },

  // Strategy APIs
  async getStrategies(): Promise<Strategy[]> {
    const result = await httpClient.get<{ strategies: Strategy[] }>(
      `${API_BASE}/strategies`
    )
    if (!result.success) throw new Error('获取策略列表失败')
    const strategies = result.data?.strategies
    return Array.isArray(strategies) ? strategies : []
  },

  async getStrategy(strategyId: string): Promise<Strategy> {
    const result = await httpClient.get<Strategy>(
      `${API_BASE}/strategies/${strategyId}`
    )
    if (!result.success) throw new Error('获取策略失败')
    return result.data!
  },

  async getActiveStrategy(): Promise<Strategy> {
    const result = await httpClient.get<Strategy>(
      `${API_BASE}/strategies/active`
    )
    if (!result.success) throw new Error('获取激活策略失败')
    return result.data!
  },

  async getDefaultStrategyConfig(): Promise<StrategyConfig> {
    const result = await httpClient.get<StrategyConfig>(
      `${API_BASE}/strategies/default-config`
    )
    if (!result.success) throw new Error('获取默认策略配置失败')
    return result.data!
  },

  async createStrategy(data: {
    name: string
    description: string
    config: StrategyConfig
  }): Promise<Strategy> {
    const result = await httpClient.post<Strategy>(
      `${API_BASE}/strategies`,
      data
    )
    if (!result.success) throw new Error('创建策略失败')
    return result.data!
  },

  async updateStrategy(
    strategyId: string,
    data: {
      name?: string
      description?: string
      config?: StrategyConfig
    }
  ): Promise<Strategy> {
    const result = await httpClient.put<Strategy>(
      `${API_BASE}/strategies/${strategyId}`,
      data
    )
    if (!result.success) throw new Error('更新策略失败')
    return result.data!
  },

  async deleteStrategy(strategyId: string): Promise<void> {
    const result = await httpClient.delete(
      `${API_BASE}/strategies/${strategyId}`
    )
    if (!result.success) throw new Error('删除策略失败')
  },

  async activateStrategy(strategyId: string): Promise<Strategy> {
    const result = await httpClient.post<Strategy>(
      `${API_BASE}/strategies/${strategyId}/activate`
    )
    if (!result.success) throw new Error('激活策略失败')
    return result.data!
  },

  async duplicateStrategy(strategyId: string): Promise<Strategy> {
    const result = await httpClient.post<Strategy>(
      `${API_BASE}/strategies/${strategyId}/duplicate`
    )
    if (!result.success) throw new Error('复制策略失败')
    return result.data!
  },

  // ==================== 通知系统 (P0) ====================

  async getNotifications(
    limit = 50,
    offset = 0
  ): Promise<{ notifications: NotificationRecord[]; unread: number }> {
    const res = await fetch(
      `${API_BASE}/notifications?limit=${limit}&offset=${offset}`,
      {
        headers: getAuthHeaders(),
      }
    )
    return handleJSONResponse(res)
  },

  async markNotificationRead(id: string): Promise<void> {
    const res = await fetch(`${API_BASE}/notifications/${id}/read`, {
      method: 'POST',
      headers: getAuthHeaders(),
    })
    await handleJSONResponse(res)
  },

  async markAllNotificationsRead(): Promise<void> {
    const res = await fetch(`${API_BASE}/notifications/read-all`, {
      method: 'POST',
      headers: getAuthHeaders(),
    })
    await handleJSONResponse(res)
  },

  async getNotificationSettings(): Promise<NotificationSettings> {
    const res = await fetch(`${API_BASE}/notification-settings`, {
      headers: getAuthHeaders(),
    })
    const data = await handleJSONResponse<{ settings: NotificationSettings }>(
      res
    )
    return data.settings
  },

  async updateNotificationSettings(
    settings: NotificationSettings
  ): Promise<NotificationSettings> {
    const res = await fetch(`${API_BASE}/notification-settings`, {
      method: 'PUT',
      headers: getAuthHeaders(),
      body: JSON.stringify(settings),
    })
    const data = await handleJSONResponse<{ settings: NotificationSettings }>(
      res
    )
    return data.settings
  },

  async testNotification(): Promise<{ message: string }> {
    const res = await fetch(`${API_BASE}/notification-settings/test`, {
      method: 'POST',
      headers: getAuthHeaders(),
    })
    return handleJSONResponse(res)
  },

  // ==================== 审计日志 (P0) ====================

  async getAuditLogs(
    limit = 100,
    offset = 0,
    action?: string
  ): Promise<AuditEvent[]> {
    const params = new URLSearchParams({
      limit: String(limit),
      offset: String(offset),
    })
    if (action) params.set('action', action)
    const res = await fetch(`${API_BASE}/audit-logs?${params}`, {
      headers: getAuthHeaders(),
    })
    const data = await handleJSONResponse<{ events: AuditEvent[] }>(res)
    return data.events || []
  },

  // ==================== 备份 (P1) ====================

  async triggerBackup(): Promise<{ message: string; path: string }> {
    const res = await fetch(`${API_BASE}/backups`, {
      method: 'POST',
      headers: getAuthHeaders(),
    })
    return handleJSONResponse(res)
  },

  async getBackups(): Promise<BackupRecord[]> {
    const res = await fetch(`${API_BASE}/backups`, {
      headers: getAuthHeaders(),
    })
    const data = await handleJSONResponse<{ backups: BackupRecord[] }>(res)
    return data.backups || []
  },
}
