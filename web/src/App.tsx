import { useEffect, useState, useRef, type ReactNode } from 'react'
import useSWR, { mutate } from 'swr'
import { api } from './lib/api'
import { ChartTabs } from './components/ChartTabs'
import { AITradersPage } from './components/AITradersPage'
import { LoginPage } from './components/LoginPage'
import { RegisterPage } from './components/RegisterPage'
import { ResetPasswordPage } from './components/ResetPasswordPage'
import { CompetitionPage } from './components/CompetitionPage'
import { LandingPage } from './pages/LandingPage'
import { FAQPage } from './pages/FAQPage'
import { StrategyStudioPage } from './pages/StrategyStudioPage'
import HeaderBar from './components/HeaderBar'
import { LanguageProvider, useLanguage } from './contexts/LanguageContext'
import { AuthProvider, useAuth } from './contexts/AuthContext'
import { ConfirmDialogProvider } from './components/ConfirmDialog'
import { t, type Language } from './i18n/translations'
import { confirmToast, notify } from './lib/notify'
import { useSystemConfig } from './hooks/useSystemConfig'
import { useTraderStream, type StreamStatus } from './hooks/useTraderStream'
import {
  useTradingAlerts,
  type TradingAlertsApi,
} from './hooks/useTradingAlerts'
import {
  formatPrice,
  formatQuantity,
  formatUsd,
  liquidationDistancePct,
  LIQ_RISK_THRESHOLD_PCT,
} from './lib/format'
import { exportCsv, exportJson, timestampSlug } from './lib/export'
import { DecisionCard } from './components/DecisionCard'
import { OrderHistoryPanel } from './components/OrderHistoryPanel'
import { PerformanceMetricsPanel } from './components/PerformanceMetricsPanel'
import { NotificationsPage } from './pages/NotificationsPage'
import { SettingsPage } from './pages/SettingsPage'
import { AdminPage } from './pages/AdminPage'
import { ManualOrderModal } from './components/ManualOrderModal'
import { PunkAvatar, getTraderAvatar } from './components/PunkAvatar'
import { BacktestPage } from './components/BacktestPage'
import {
  Activity,
  AlertTriangle,
  Bell,
  BellOff,
  Bot,
  BrainCircuit,
  BriefcaseBusiness,
  ChartNoAxesCombined,
  CircleDollarSign,
  Clock3,
  Database,
  LogOut,
  Loader2,
  OctagonX,
  RefreshCw,
  ShieldCheck,
  WalletCards,
  WifiOff,
  Zap,
  Download,
  Crosshair,
} from 'lucide-react'
import type {
  SystemStatus,
  AccountInfo,
  Position,
  DecisionRecord,
  Statistics,
  TraderInfo,
  Exchange,
} from './types'

type Page =
  | 'competition'
  | 'traders'
  | 'trader'
  | 'backtest'
  | 'strategy'
  | 'faq'
  | 'notifications'
  | 'settings'
  | 'admin'
  | 'login'
  | 'register'

// 获取友好的AI模型名称
function getModelDisplayName(modelId: string): string {
  switch (modelId.toLowerCase()) {
    case 'deepseek':
      return 'DeepSeek'
    case 'qwen':
      return 'Qwen'
    case 'claude':
      return 'Claude'
    default:
      return modelId.toUpperCase()
  }
}

// Helper function to get exchange display name from exchange ID (UUID)
function getExchangeDisplayNameFromList(
  exchangeId: string | undefined,
  exchanges: Exchange[] | undefined
): string {
  if (!exchangeId) return 'Unknown'
  const exchange = exchanges?.find((e) => e.id === exchangeId)
  if (!exchange) return exchangeId.substring(0, 8).toUpperCase() + '...'
  const typeName = exchange.exchange_type?.toUpperCase() || exchange.name
  return exchange.account_name
    ? `${typeName} - ${exchange.account_name}`
    : typeName
}

// Helper function to get exchange type from exchange ID (UUID) - for TradingView charts
function getExchangeTypeFromList(
  exchangeId: string | undefined,
  exchanges: Exchange[] | undefined
): string {
  if (!exchangeId) return 'BINANCE'
  const exchange = exchanges?.find((e) => e.id === exchangeId)
  if (!exchange) return 'BINANCE' // Default to BINANCE for charts
  return exchange.exchange_type?.toUpperCase() || 'BINANCE'
}

function App() {
  const { language, setLanguage } = useLanguage()
  const { user, token, logout, isLoading } = useAuth()
  const { loading: configLoading } = useSystemConfig()
  const [route, setRoute] = useState(window.location.pathname)

  // 从URL路径读取初始页面状态（支持刷新保持页面）
  const getInitialPage = (): Page => {
    const path = window.location.pathname
    const hash = window.location.hash.slice(1) // 去掉 #

    if (path === '/traders' || hash === 'traders') return 'traders'
    if (path === '/backtest' || hash === 'backtest') return 'backtest'
    if (path === '/strategy' || hash === 'strategy') return 'strategy'
    if (path === '/notifications' || hash === 'notifications')
      return 'notifications'
    if (path === '/settings' || hash === 'settings') return 'settings'
    if (path === '/admin' || hash === 'admin') return 'admin'
    if (path === '/dashboard' || hash === 'trader' || hash === 'details')
      return 'trader'
    return 'competition' // 默认为竞赛页面
  }

  const [currentPage, setCurrentPage] = useState<Page>(getInitialPage())
  const [selectedTraderId, setSelectedTraderId] = useState<string | undefined>()
  const [lastUpdate, setLastUpdate] = useState<string>('--:--:--')
  const [lastUpdateMs, setLastUpdateMs] = useState<number | undefined>(
    undefined
  )
  const [decisionsLimit, setDecisionsLimit] = useState<number>(5)

  // 监听URL变化，同步页面状态
  useEffect(() => {
    const handleRouteChange = () => {
      const path = window.location.pathname
      const hash = window.location.hash.slice(1)

      if (path === '/traders' || hash === 'traders') {
        setCurrentPage('traders')
      } else if (path === '/backtest' || hash === 'backtest') {
        setCurrentPage('backtest')
      } else if (path === '/strategy' || hash === 'strategy') {
        setCurrentPage('strategy')
      } else if (path === '/notifications' || hash === 'notifications') {
        setCurrentPage('notifications')
      } else if (path === '/settings' || hash === 'settings') {
        setCurrentPage('settings')
      } else if (path === '/admin' || hash === 'admin') {
        setCurrentPage('admin')
      } else if (
        path === '/dashboard' ||
        hash === 'trader' ||
        hash === 'details'
      ) {
        setCurrentPage('trader')
      } else if (
        path === '/competition' ||
        hash === 'competition' ||
        hash === ''
      ) {
        setCurrentPage('competition')
      }
      setRoute(path)
    }

    window.addEventListener('hashchange', handleRouteChange)
    window.addEventListener('popstate', handleRouteChange)
    return () => {
      window.removeEventListener('hashchange', handleRouteChange)
      window.removeEventListener('popstate', handleRouteChange)
    }
  }, [])

  // 切换页面时更新URL hash (当前通过按钮直接调用setCurrentPage，这个函数暂时保留用于未来扩展)
  // const navigateToPage = (page: Page) => {
  //   setCurrentPage(page);
  //   window.location.hash = page === 'competition' ? '' : 'trader';
  // };

  // 获取trader列表（仅在用户登录时）
  const { data: traders, error: tradersError } = useSWR<TraderInfo[]>(
    user && token ? 'traders' : null,
    api.getTraders,
    {
      refreshInterval: 10000,
      shouldRetryOnError: false, // 避免在后端未运行时无限重试
    }
  )

  // 获取exchanges列表（用于显示交易所名称）
  const { data: exchanges } = useSWR<Exchange[]>(
    user && token ? 'exchanges' : null,
    api.getExchangeConfigs,
    {
      refreshInterval: 60000, // 1分钟刷新一次
      shouldRetryOnError: false,
    }
  )

  // 当获取到traders后，设置默认选中第一个
  useEffect(() => {
    if (traders && traders.length > 0 && !selectedTraderId) {
      setSelectedTraderId(traders[0].trader_id)
    }
  }, [traders, selectedTraderId])

  // 如果在trader页面，获取该trader的数据
  const { data: status } = useSWR<SystemStatus>(
    currentPage === 'trader' && selectedTraderId
      ? `status-${selectedTraderId}`
      : null,
    () => api.getStatus(selectedTraderId),
    {
      refreshInterval: 15000, // 15秒刷新（配合后端15秒缓存）
      revalidateOnFocus: false, // 禁用聚焦时重新验证，减少请求
      dedupingInterval: 10000, // 10秒去重，防止短时间内重复请求
    }
  )

  const { data: account, error: accountError } = useSWR<AccountInfo>(
    currentPage === 'trader' && selectedTraderId
      ? `account-${selectedTraderId}`
      : null,
    () => api.getAccount(selectedTraderId),
    {
      refreshInterval: 15000, // 15秒轮询作为 SSE 的降级兑底
      revalidateOnFocus: false, // 禁用聚焦时重新验证，减少请求
      dedupingInterval: 10000, // 10秒去重，防止短时间内重复请求
    }
  )

  const { data: positions, error: positionsError } = useSWR<Position[]>(
    currentPage === 'trader' && selectedTraderId
      ? `positions-${selectedTraderId}`
      : null,
    () => api.getPositions(selectedTraderId),
    {
      refreshInterval: 15000,
      revalidateOnFocus: false,
      dedupingInterval: 10000,
    }
  )

  const { data: decisions } = useSWR<DecisionRecord[]>(
    currentPage === 'trader' && selectedTraderId
      ? `decisions/latest-${selectedTraderId}-${decisionsLimit}`
      : null,
    () => api.getLatestDecisions(selectedTraderId, decisionsLimit),
    {
      refreshInterval: 30000, // 30秒刷新（决策更新频率较低）
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  )

  const { data: stats } = useSWR<Statistics>(
    currentPage === 'trader' && selectedTraderId
      ? `statistics-${selectedTraderId}`
      : null,
    () => api.getStatistics(selectedTraderId),
    {
      refreshInterval: 30000, // 30秒刷新（统计数据更新频率较低）
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  )

  useEffect(() => {
    // 账户或持仓任一更新都刷新"最后更新时间"，
    // 用于连接断开/数据过期横幅的判定
    if (account || positions) {
      const now = new Date()
      setLastUpdate(now.toLocaleTimeString())
      setLastUpdateMs(now.getTime())
    }
  }, [account, positions])

  const selectedTrader = traders?.find((t) => t.trader_id === selectedTraderId)

  // SSE 实时快照：直接写入 SWR 缓存，轮询作为降级兑底
  const { status: streamStatus } = useTraderStream(
    currentPage === 'trader' && selectedTraderId ? selectedTraderId : undefined
  )

  // 交易事件浏览器通知（AI 开/平仓、强平风险、决策失败）
  const tradingAlerts = useTradingAlerts({
    traderName: selectedTrader?.trader_name,
    positions,
    decisions,
    language,
  })

  // Handle routing
  useEffect(() => {
    const handlePopState = () => {
      setRoute(window.location.pathname)
    }
    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [])

  // Set current page based on route for consistent navigation state
  useEffect(() => {
    if (route === '/competition') {
      setCurrentPage('competition')
    } else if (route === '/traders') {
      setCurrentPage('traders')
    } else if (route === '/dashboard') {
      setCurrentPage('trader')
    } else if (route === '/notifications') {
      setCurrentPage('notifications')
    } else if (route === '/settings') {
      setCurrentPage('settings')
    }
  }, [route])

  // Show loading spinner while checking auth or config
  if (isLoading || configLoading) {
    return (
      <div
        className="min-h-screen flex items-center justify-center"
        style={{ background: '#0B0E11' }}
      >
        <div className="text-center">
          <img
            src="/icons/auaiex.svg"
            alt="Auaiex Logo"
            className="w-16 h-16 mx-auto mb-4 animate-pulse"
          />
          <p style={{ color: '#EAECEF' }}>{t('loading', language)}</p>
        </div>
      </div>
    )
  }

  // Handle specific routes regardless of authentication
  if (route === '/login') {
    return <LoginPage />
  }
  if (route === '/register') {
    return <RegisterPage />
  }
  if (route === '/faq') {
    return <FAQPage />
  }
  if (route === '/reset-password') {
    return <ResetPasswordPage />
  }
  if (route === '/competition') {
    return (
      <div
        className="min-h-screen"
        style={{ background: '#000000', color: '#EAECEF' }}
      >
        <HeaderBar
          isLoggedIn={!!user}
          currentPage="competition"
          language={language}
          onLanguageChange={setLanguage}
          user={user}
          onLogout={logout}
          onPageChange={(page: Page) => {
            if (page === 'competition') {
              window.history.pushState({}, '', '/competition')
              setRoute('/competition')
              setCurrentPage('competition')
            } else if (page === 'traders') {
              window.history.pushState({}, '', '/traders')
              setRoute('/traders')
              setCurrentPage('traders')
            } else if (page === 'trader') {
              window.history.pushState({}, '', '/dashboard')
              setRoute('/dashboard')
              setCurrentPage('trader')
            } else if (page === 'faq') {
              window.history.pushState({}, '', '/faq')
              setRoute('/faq')
            } else if (page === 'backtest') {
              window.history.pushState({}, '', '/backtest')
              setRoute('/backtest')
              setCurrentPage('backtest')
            } else if (page === 'strategy') {
              window.history.pushState({}, '', '/strategy')
              setRoute('/strategy')
              setCurrentPage('strategy')
            } else if (page === 'notifications') {
              window.history.pushState({}, '', '/notifications')
              setRoute('/notifications')
              setCurrentPage('notifications')
            } else if (page === 'settings') {
              window.history.pushState({}, '', '/settings')
              setRoute('/settings')
              setCurrentPage('settings')
            }
          }}
        />
        <main className="max-w-[1920px] mx-auto px-6 py-6 pt-24">
          <CompetitionPage />
        </main>
      </div>
    )
  }

  // Show landing page for root route
  if (route === '/' || route === '') {
    return <LandingPage />
  }

  // Allow unauthenticated users to open backtest page directly (others仍展示 Landing)
  if (!user || !token) {
    if (route === '/backtest' || currentPage === 'backtest') {
      return (
        <div
          className="min-h-screen"
          style={{ background: '#0B0E11', color: '#EAECEF' }}
        >
          <HeaderBar
            isLoggedIn={false}
            currentPage="backtest"
            language={language}
            onLanguageChange={setLanguage}
            onPageChange={(page: Page) => {
              if (page === 'competition') {
                window.history.pushState({}, '', '/competition')
                setRoute('/competition')
                setCurrentPage('competition')
              } else if (page === 'traders') {
                window.history.pushState({}, '', '/traders')
                setRoute('/traders')
                setCurrentPage('traders')
              }
            }}
          />
          <main className="max-w-[1920px] mx-auto px-6 py-6 pt-24">
            <BacktestPage />
          </main>
        </div>
      )
    }
    return <LandingPage />
  }

  // Show main app for authenticated users on other routes
  if (!user || !token) {
    // Default to landing page when not authenticated and no specific route
    return <LandingPage />
  }

  return (
    <div
      className="min-h-screen"
      style={{ background: '#0B0E11', color: '#EAECEF' }}
    >
      <HeaderBar
        isLoggedIn={!!user}
        currentPage={currentPage}
        language={language}
        onLanguageChange={setLanguage}
        user={user}
        onLogout={logout}
        onPageChange={(page: Page) => {
          if (page === 'competition') {
            window.history.pushState({}, '', '/competition')
            setRoute('/competition')
            setCurrentPage('competition')
          } else if (page === 'traders') {
            window.history.pushState({}, '', '/traders')
            setRoute('/traders')
            setCurrentPage('traders')
          } else if (page === 'trader') {
            window.history.pushState({}, '', '/dashboard')
            setRoute('/dashboard')
            setCurrentPage('trader')
          } else if (page === 'backtest') {
            window.history.pushState({}, '', '/backtest')
            setRoute('/backtest')
            setCurrentPage('backtest')
          } else if (page === 'strategy') {
            window.history.pushState({}, '', '/strategy')
            setRoute('/strategy')
            setCurrentPage('strategy')
          } else if (page === 'faq') {
            window.history.pushState({}, '', '/faq')
            setRoute('/faq')
          } else if (page === 'notifications') {
            window.history.pushState({}, '', '/notifications')
            setRoute('/notifications')
            setCurrentPage('notifications')
          } else if (page === 'settings') {
            window.history.pushState({}, '', '/settings')
            setRoute('/settings')
            setCurrentPage('settings')
          } else if (page === 'admin') {
            window.history.pushState({}, '', '/admin')
            setRoute('/admin')
            setCurrentPage('admin')
          }
        }}
      />

      {/* Main Content */}
      <main className="max-w-[1920px] mx-auto px-4 sm:px-6 py-6 pt-20 sm:pt-24">
        {currentPage === 'competition' ? (
          <CompetitionPage />
        ) : currentPage === 'traders' ? (
          <AITradersPage
            onTraderSelect={(traderId) => {
              setSelectedTraderId(traderId)
              window.history.pushState({}, '', '/dashboard')
              setRoute('/dashboard')
              setCurrentPage('trader')
            }}
          />
        ) : currentPage === 'backtest' ? (
          <BacktestPage />
        ) : currentPage === 'strategy' ? (
          <StrategyStudioPage />
        ) : currentPage === 'notifications' ? (
          <NotificationsPage />
        ) : currentPage === 'settings' ? (
          <SettingsPage />
        ) : currentPage === 'admin' ? (
          <AdminPage />
        ) : (
          <TraderDetailsPage
            selectedTrader={selectedTrader}
            status={status}
            account={account}
            accountError={accountError}
            positions={positions}
            positionsError={positionsError}
            decisions={decisions}
            decisionsLimit={decisionsLimit}
            onDecisionsLimitChange={setDecisionsLimit}
            stats={stats}
            lastUpdate={lastUpdate}
            lastUpdateMs={lastUpdateMs}
            streamStatus={streamStatus}
            alerts={tradingAlerts}
            language={language}
            traders={traders}
            tradersError={tradersError}
            selectedTraderId={selectedTraderId}
            onTraderSelect={setSelectedTraderId}
            onNavigateToTraders={() => {
              window.history.pushState({}, '', '/traders')
              setRoute('/traders')
              setCurrentPage('traders')
            }}
            exchanges={exchanges}
          />
        )}
      </main>

      <footer
        className="mt-16"
        style={{ borderTop: '1px solid #2B3139', background: '#181A20' }}
      >
        <div
          className="max-w-[1920px] mx-auto px-6 py-6 text-center text-sm"
          style={{ color: '#5E6673' }}
        >
          <p>{t('footerTitle', language)}</p>
          <p className="mt-1">{t('footerWarning', language)}</p>
        </div>
      </footer>
    </div>
  )
}

// Trader Details Page Component
function TraderDetailsPage({
  selectedTrader,
  status,
  account,
  accountError,
  positions,
  positionsError,
  decisions,
  decisionsLimit,
  onDecisionsLimitChange,
  stats,
  lastUpdate,
  lastUpdateMs,
  streamStatus,
  alerts,
  language,
  traders,
  tradersError,
  selectedTraderId,
  onTraderSelect,
  onNavigateToTraders,
  exchanges,
}: {
  selectedTrader?: TraderInfo
  traders?: TraderInfo[]
  tradersError?: Error
  selectedTraderId?: string
  onTraderSelect: (traderId: string) => void
  onNavigateToTraders: () => void
  status?: SystemStatus
  account?: AccountInfo
  accountError?: Error
  positions?: Position[]
  positionsError?: Error
  decisions?: DecisionRecord[]
  decisionsLimit: number
  onDecisionsLimitChange: (limit: number) => void
  stats?: Statistics
  lastUpdate: string
  lastUpdateMs?: number
  streamStatus?: StreamStatus
  alerts?: TradingAlertsApi
  language: Language
  exchanges?: Exchange[]
}) {
  const [closingPosition, setClosingPosition] = useState<string | null>(null)
  const [closingAll, setClosingAll] = useState(false)
  const [stoppingTrader, setStoppingTrader] = useState(false)
  const [manualOrderOpen, setManualOrderOpen] = useState(false)
  const [selectedChartSymbol, setSelectedChartSymbol] = useState<
    string | undefined
  >(undefined)
  const [chartUpdateKey, setChartUpdateKey] = useState<number>(0)
  const chartSectionRef = useRef<HTMLDivElement>(null)

  // 单仓平仓操作
  const handleClosePosition = async (symbol: string, side: string) => {
    if (!selectedTraderId) return

    const confirmed = await confirmToast(
      t('confirmClosePosition', language, {
        symbol,
        side: t(side === 'LONG' ? 'longPosition' : 'shortPosition', language),
      }),
      {
        title: t('confirmCloseTitle', language),
        okText: t('confirmBtn', language),
        cancelText: t('cancelBtn', language),
      }
    )

    if (!confirmed) return

    setClosingPosition(symbol)
    try {
      await api.closePosition(selectedTraderId, symbol, side)
      notify.success(t('closeSuccess', language))
      // 使用 SWR mutate 刷新数据而非重新加载页面
      await Promise.all([
        mutate(`positions-${selectedTraderId}`),
        mutate(`account-${selectedTraderId}`),
      ])
    } catch (err: unknown) {
      const errorMsg =
        err instanceof Error ? err.message : t('closeFailed', language)
      notify.error(errorMsg)
    } finally {
      setClosingPosition(null)
    }
  }

  // 一键全平：逐个半仓调用现有平仓接口（同币种同方向去重）
  const handleCloseAllPositions = async () => {
    if (!selectedTraderId || !positions || positions.length === 0) return

    const uniqueKeys = Array.from(
      new Set(positions.map((p) => `${p.symbol}:${p.side.toUpperCase()}`))
    )

    const confirmed = await confirmToast(
      t('confirmCloseAll', language, { count: uniqueKeys.length }),
      {
        title: t('closeAll', language),
        okText: t('confirmBtn', language),
        cancelText: t('cancelBtn', language),
      }
    )
    if (!confirmed) return

    setClosingAll(true)
    let success = 0
    let failed = 0
    for (const key of uniqueKeys) {
      const [symbol, side] = key.split(':')
      try {
        await api.closePosition(selectedTraderId, symbol, side)
        success++
      } catch {
        failed++
      }
    }
    setClosingAll(false)

    if (failed === 0) {
      notify.success(t('closeAllSuccess', language))
    } else {
      notify.warning(
        t('closeAllPartial', language, {
          success,
          total: uniqueKeys.length,
          failed,
        })
      )
    }
    await Promise.all([
      mutate(`positions-${selectedTraderId}`),
      mutate(`account-${selectedTraderId}`),
    ])
  }

  // 停止自动交易（紧急风控）
  const handleStopTrading = async () => {
    if (!selectedTraderId) return
    const confirmed = await confirmToast(t('confirmStopTrading', language), {
      title: t('stopTrading', language),
      okText: t('confirmBtn', language),
      cancelText: t('cancelBtn', language),
    })
    if (!confirmed) return

    setStoppingTrader(true)
    try {
      await api.stopTrader(selectedTraderId)
      notify.success(t('traderStopped', language))
      await Promise.all([
        mutate('traders'),
        mutate(`status-${selectedTraderId}`),
      ])
    } catch (err: unknown) {
      const errorMsg =
        err instanceof Error ? err.message : t('stopTradingFailed', language)
      notify.error(errorMsg)
    } finally {
      setStoppingTrader(false)
    }
  }

  // 导出当前持仓 CSV
  const handleExportPositions = () => {
    if (!positions || positions.length === 0) return
    exportCsv(
      `positions-${selectedTrader?.trader_name ?? 'trader'}-${timestampSlug()}.csv`,
      [
        'Symbol',
        'Side',
        'Entry Price',
        'Mark Price',
        'Quantity',
        'Leverage',
        'Unrealized PnL',
        'Unrealized PnL %',
        'Liquidation Price',
      ],
      positions.map((p) => [
        p.symbol,
        p.side,
        p.entry_price,
        p.mark_price,
        p.quantity,
        p.leverage,
        p.unrealized_pnl,
        p.unrealized_pnl_pct,
        p.liquidation_price,
      ])
    )
  }

  // 导出决策日志 JSON（含思维链，便于离线分析）
  const handleExportDecisions = () => {
    if (!decisions || decisions.length === 0) return
    exportJson(
      `decisions-${selectedTrader?.trader_name ?? 'trader'}-${timestampSlug()}.json`,
      {
        trader: selectedTrader?.trader_name,
        trader_id: selectedTraderId,
        exported_at: new Date().toISOString(),
        count: decisions.length,
        decisions,
      }
    )
  }

  // 连接状态：接口报错 → 连接断开；数据超过 60s 未更新 → 数据过期
  const [nowTick, setNowTick] = useState(() => Date.now())
  useEffect(() => {
    const timer = window.setInterval(() => setNowTick(Date.now()), 10000)
    return () => window.clearInterval(timer)
  }, [])
  const hasConnectionError = Boolean(accountError || positionsError)
  const dataAgeMs = lastUpdateMs
    ? nowTick - lastUpdateMs
    : Number.POSITIVE_INFINITY
  const isDataStale =
    !hasConnectionError &&
    Number.isFinite(dataAgeMs) &&
    dataAgeMs > 60000 &&
    lastUpdate !== '--:--:--'
  // If API failed with error, show empty state (likely backend not running)
  if (tradersError) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <div className="text-center max-w-md mx-auto px-6">
          {/* Icon */}
          <div
            className="w-24 h-24 mx-auto mb-6 rounded-full flex items-center justify-center"
            style={{
              background: 'rgba(240, 185, 11, 0.1)',
              border: '2px solid rgba(240, 185, 11, 0.3)',
            }}
          >
            <svg
              className="w-12 h-12"
              style={{ color: '#F0B90B' }}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
              />
            </svg>
          </div>

          {/* Title */}
          <h2 className="text-2xl font-bold mb-3" style={{ color: '#EAECEF' }}>
            {t('dashboardEmptyTitle', language)}
          </h2>

          {/* Description */}
          <p className="text-base mb-6" style={{ color: '#848E9C' }}>
            {t('dashboardEmptyDescription', language)}
          </p>

          {/* CTA Button */}
          <button
            onClick={onNavigateToTraders}
            className="px-6 py-3 rounded-lg font-semibold transition-all hover:scale-105 active:scale-95"
            style={{
              background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
              color: '#0B0E11',
              boxShadow: '0 4px 12px rgba(240, 185, 11, 0.3)',
            }}
          >
            {t('goToTradersPage', language)}
          </button>
        </div>
      </div>
    )
  }

  // If traders is loaded and empty, show empty state
  if (traders && traders.length === 0) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <div className="text-center max-w-md mx-auto px-6">
          {/* Icon */}
          <div
            className="w-24 h-24 mx-auto mb-6 rounded-full flex items-center justify-center"
            style={{
              background: 'rgba(240, 185, 11, 0.1)',
              border: '2px solid rgba(240, 185, 11, 0.3)',
            }}
          >
            <svg
              className="w-12 h-12"
              style={{ color: '#F0B90B' }}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
              />
            </svg>
          </div>

          {/* Title */}
          <h2 className="text-2xl font-bold mb-3" style={{ color: '#EAECEF' }}>
            {t('dashboardEmptyTitle', language)}
          </h2>

          {/* Description */}
          <p className="text-base mb-6" style={{ color: '#848E9C' }}>
            {t('dashboardEmptyDescription', language)}
          </p>

          {/* CTA Button */}
          <button
            onClick={onNavigateToTraders}
            className="px-6 py-3 rounded-lg font-semibold transition-all hover:scale-105 active:scale-95"
            style={{
              background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
              color: '#0B0E11',
              boxShadow: '0 4px 12px rgba(240, 185, 11, 0.3)',
            }}
          >
            {t('goToTradersPage', language)}
          </button>
        </div>
      </div>
    )
  }

  // If traders is still loading or selectedTrader is not ready, show skeleton
  if (!selectedTrader) {
    return (
      <div className="space-y-6">
        {/* Loading Skeleton - Binance Style */}
        <div className="binance-card p-6 animate-pulse">
          <div className="skeleton h-8 w-48 mb-3"></div>
          <div className="flex gap-4">
            <div className="skeleton h-4 w-32"></div>
            <div className="skeleton h-4 w-24"></div>
            <div className="skeleton h-4 w-28"></div>
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="binance-card p-5 animate-pulse">
              <div className="skeleton h-4 w-24 mb-3"></div>
              <div className="skeleton h-8 w-32"></div>
            </div>
          ))}
        </div>
        <div className="binance-card p-6 animate-pulse">
          <div className="skeleton h-6 w-40 mb-4"></div>
          <div className="skeleton h-64 w-full"></div>
        </div>
      </div>
    )
  }

  const modelName = getModelDisplayName(
    selectedTrader.ai_model.split('_').pop() || selectedTrader.ai_model
  )
  const exchangeName = getExchangeDisplayNameFromList(
    selectedTrader.exchange_id,
    exchanges
  )
  const strategyName = selectedTrader.strategy_name || t('noStrategy', language)

  // 交易统计（后端已返回但此前从未展示）
  const totalCycles = stats?.total_cycles ?? 0
  const successRate =
    totalCycles > 0
      ? Math.round(((stats?.successful_cycles ?? 0) / totalCycles) * 100)
      : null

  return (
    <div className="dashboard-shell">
      {/* Trader Header */}
      <section className="dashboard-hero animate-scale-in">
        <div className="dashboard-hero__main">
          <div className="dashboard-hero__identity">
            <PunkAvatar
              seed={getTraderAvatar(
                selectedTrader.trader_id,
                selectedTrader.trader_name
              )}
              size={52}
              className="dashboard-hero__avatar"
            />
            <div className="min-w-0">
              <div className="dashboard-eyebrow">
                <span
                  className={`dashboard-status-dot ${status?.is_running ? 'is-online' : ''}`}
                />
                {status?.is_running
                  ? t('autoTradingActive', language)
                  : t('traderOverview', language)}
                {streamStatus === 'connected' && (
                  <span className="dashboard-live-badge">
                    {t('liveLabel', language)}
                  </span>
                )}
              </div>
              <h1 className="dashboard-hero__title">
                {selectedTrader.trader_name}
              </h1>
              <div className="dashboard-meta-list">
                <span>
                  <Bot size={14} />
                  {modelName}
                </span>
                <span>
                  <Database size={14} />
                  {exchangeName}
                </span>
                <span>
                  <ShieldCheck size={14} />
                  {strategyName}
                </span>
              </div>
            </div>
          </div>

          {/* Risk & alert actions */}
          <div className="dashboard-hero__actions">
            {/* 手动下单 */}
            <button
              type="button"
              onClick={() => setManualOrderOpen(true)}
              className="inline-flex items-center gap-1.5 px-3 py-2 rounded-lg text-xs font-bold transition-all hover:scale-105"
              style={{
                background: '#1E2329',
                border: '1px solid #2B3139',
                color: '#FCD535',
              }}
              title={t('manualOrder', language)}
            >
              <Crosshair className="w-4 h-4" />
              {t('manualOrder', language)}
            </button>
            {status?.is_running && (
              <button
                type="button"
                onClick={handleStopTrading}
                disabled={stoppingTrader}
                className="btn-danger inline-flex items-center gap-1.5 px-3 py-2 rounded-lg text-xs font-bold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed"
                title={t('stopTrading', language)}
              >
                {stoppingTrader ? (
                  <Loader2 className="w-4 h-4 animate-spin" />
                ) : (
                  <OctagonX className="w-4 h-4" />
                )}
                {t('stopTrading', language)}
              </button>
            )}
            {alerts?.supported && (
              <button
                type="button"
                onClick={() => alerts.toggle()}
                className={`inline-flex items-center gap-1.5 px-3 py-2 rounded-lg text-xs font-bold transition-all hover:scale-105 ${
                  alerts.enabled ? 'alerts-bell-btn is-on' : 'alerts-bell-btn'
                }`}
                title={
                  alerts.enabled
                    ? t('notificationsEnabled', language)
                    : t('enableNotifications', language)
                }
              >
                {alerts.enabled ? (
                  <Bell className="w-4 h-4" />
                ) : (
                  <BellOff className="w-4 h-4" />
                )}
                {alerts.enabled
                  ? t('notificationsEnabled', language)
                  : t('enableNotifications', language)}
              </button>
            )}
          </div>

          {/* Trader Selector */}
          {traders && traders.length > 0 && (
            <label className="dashboard-trader-select">
              <span>{t('switchTrader', language)}</span>
              <select
                value={selectedTraderId}
                onChange={(e) => onTraderSelect(e.target.value)}
              >
                {traders.map((trader) => (
                  <option key={trader.trader_id} value={trader.trader_id}>
                    {trader.trader_name}
                  </option>
                ))}
              </select>
            </label>
          )}
        </div>
        <div className="dashboard-hero__footer">
          <div className="dashboard-runtime">
            <span>
              <Activity size={15} />
              {t('cyclesCount', language, { count: status?.call_count ?? 0 })}
            </span>
            <span>
              <Clock3 size={15} />
              {status?.runtime_minutes ?? 0} min
            </span>
          </div>
          <div className="dashboard-updated">
            <RefreshCw size={14} />
            {t('updatedAt', language, { time: lastUpdate })}
          </div>
        </div>
      </section>

      {/* 连接状态横幅：接口报错或数据过期时提醒，避免静默展示陈旧数据 */}
      {manualOrderOpen && selectedTraderId && (
        <ManualOrderModal
          traderId={selectedTraderId}
          language={language}
          onClose={() => setManualOrderOpen(false)}
        />
      )}
      {(hasConnectionError || isDataStale) && (
        <div
          className={`dashboard-connection-banner ${hasConnectionError ? 'is-error' : 'is-stale'}`}
          role="alert"
        >
          {hasConnectionError ? (
            <WifiOff size={16} className="flex-shrink-0" />
          ) : (
            <AlertTriangle size={16} className="flex-shrink-0" />
          )}
          <span>
            {hasConnectionError
              ? t('connectionLost', language, { time: lastUpdate })
              : t('dataStale', language, { time: lastUpdate })}
          </span>
        </div>
      )}

      {/* Account Overview */}
      <div className="dashboard-stat-grid">
        <StatCard
          title={t('totalEquity', language)}
          value={`${account?.total_equity?.toFixed(2) || '0.00'} USDT`}
          change={account?.total_pnl_pct || 0}
          positive={(account?.total_pnl ?? 0) > 0}
          icon={<WalletCards size={18} />}
          accent="yellow"
        />
        <StatCard
          title={t('availableBalance', language)}
          value={`${account?.available_balance?.toFixed(2) || '0.00'} USDT`}
          subtitle={`${account?.available_balance && account?.total_equity ? ((account.available_balance / account.total_equity) * 100).toFixed(1) : '0.0'}% ${t('free', language)}`}
          icon={<CircleDollarSign size={18} />}
          accent="blue"
        />
        <StatCard
          title={t('totalPnL', language)}
          value={`${account?.total_pnl !== undefined && account.total_pnl >= 0 ? '+' : ''}${account?.total_pnl?.toFixed(2) || '0.00'} USDT`}
          change={account?.total_pnl_pct || 0}
          positive={(account?.total_pnl ?? 0) >= 0}
          icon={<ChartNoAxesCombined size={18} />}
          accent={(account?.total_pnl ?? 0) >= 0 ? 'green' : 'red'}
        />
        <StatCard
          title={t('positions', language)}
          value={`${account?.position_count || 0}`}
          subtitle={`${t('margin', language)}: ${account?.margin_used_pct?.toFixed(1) || '0.0'}%`}
          icon={<BriefcaseBusiness size={18} />}
          accent="violet"
        />
      </div>

      {/* Trading Statistics —— 后端统计数据（胜率/周期/开平仓次数） */}
      <div className="dashboard-stats-strip">
        <div className="stats-strip__item">
          <div className="stats-strip__label">
            {t('cycleSuccessRate', language)}
          </div>
          <div className="stats-strip__value">
            {successRate !== null ? (
              <>
                <span
                  className={successRate >= 50 ? 'is-positive' : 'is-negative'}
                >
                  {successRate}%
                </span>
                <span className="stats-strip__bar">
                  <span
                    className={`stats-strip__bar-fill ${successRate >= 50 ? 'is-positive' : 'is-negative'}`}
                    style={{ width: `${successRate}%` }}
                  />
                </span>
              </>
            ) : (
              <span className="stats-strip__muted">
                {t('noStatsData', language)}
              </span>
            )}
          </div>
        </div>
        <div className="stats-strip__item">
          <div className="stats-strip__label">
            {t('totalCyclesLabel', language)}
          </div>
          <div className="stats-strip__value">{totalCycles}</div>
        </div>
        <div className="stats-strip__item">
          <div className="stats-strip__label">
            {t('openTradesTotal', language)}
          </div>
          <div className="stats-strip__value">
            {stats?.total_open_positions ?? 0}
          </div>
        </div>
        <div className="stats-strip__item">
          <div className="stats-strip__label">
            {t('closeTradesTotal', language)}
          </div>
          <div className="stats-strip__value">
            {stats?.total_close_positions ?? 0}
          </div>
        </div>
      </div>

      {/* 主要内容区：左右分屏 */}
      <div className="dashboard-workspace">
        {/* 左侧：图表 + 持仓 */}
        <div className="space-y-6">
          {/* Chart Tabs (Equity / K-line) */}
          <div
            ref={chartSectionRef}
            className="chart-container animate-slide-in scroll-mt-32"
            style={{ animationDelay: '0.1s' }}
          >
            <ChartTabs
              traderId={selectedTrader.trader_id}
              selectedSymbol={selectedChartSymbol}
              updateKey={chartUpdateKey}
              exchangeId={getExchangeTypeFromList(
                selectedTrader.exchange_id,
                exchanges
              )}
            />
          </div>

          {/* Current Positions */}
          <div
            className="binance-card dashboard-panel animate-slide-in"
            style={{ animationDelay: '0.15s' }}
          >
            <div className="dashboard-panel__header">
              <div>
                <div className="dashboard-panel__eyebrow">
                  {t('liveExposure', language)}
                </div>
                <h2 className="dashboard-panel__title">
                  <BriefcaseBusiness size={19} />
                  {t('currentPositions', language)}
                </h2>
              </div>
              {positions && positions.length > 0 && (
                <div className="dashboard-panel__actions">
                  {/* 导出持仓 CSV */}
                  <button
                    type="button"
                    onClick={handleExportPositions}
                    className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-[11px] font-bold transition-all hover:scale-105"
                    style={{
                      background: '#1E2329',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                    }}
                    title={t('exportPositions', language)}
                  >
                    <Download className="w-3.5 h-3.5" />
                    {t('exportCsv', language)}
                  </button>
                  {/* 一键全平（紧急风控） */}
                  <button
                    type="button"
                    onClick={handleCloseAllPositions}
                    disabled={closingAll || closingPosition !== null}
                    className="btn-danger inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-[11px] font-bold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed"
                    title={t('closeAll', language)}
                  >
                    {closingAll ? (
                      <Loader2 className="w-3.5 h-3.5 animate-spin" />
                    ) : (
                      <Zap className="w-3.5 h-3.5" />
                    )}
                    {t('closeAll', language)}
                  </button>
                  <div className="dashboard-count-badge">
                    {positions.length} {t('active', language)}
                  </div>
                </div>
              )}
            </div>
            {positions && positions.length > 0 ? (
              <div className="hidden md:block overflow-x-auto">
                <table className="w-full text-xs">
                  <thead className="text-left border-b border-gray-800">
                    <tr>
                      <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-left">
                        {t('symbol', language)}
                      </th>
                      <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-center">
                        {t('side', language)}
                      </th>
                      <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-center">
                        {t('actionCol', language)}
                      </th>
                      <th
                        className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right"
                        title={t('entryPrice', language)}
                      >
                        {t('entryShort', language)}
                      </th>
                      <th
                        className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right"
                        title={t('markPrice', language)}
                      >
                        {t('markShort', language)}
                      </th>
                      <th
                        className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right"
                        title={t('quantity', language)}
                      >
                        {t('qtyShort', language)}
                      </th>
                      <th
                        className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right"
                        title={t('positionValue', language)}
                      >
                        {t('valueShort', language)}
                      </th>
                      <th
                        className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-center"
                        title={t('leverage', language)}
                      >
                        {t('levShort', language)}
                      </th>
                      <th
                        className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right"
                        title={t('unrealizedPnL', language)}
                      >
                        {t('upnlShort', language)}
                      </th>
                      <th
                        className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right"
                        title={t('distanceToLiq', language)}
                      >
                        {t('distanceToLiq', language)}
                      </th>
                      <th
                        className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right"
                        title={t('liqPrice', language)}
                      >
                        {t('liqShort', language)}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {positions.map((pos, i) => {
                      const liqDist = liquidationDistancePct(
                        pos.mark_price,
                        pos.liquidation_price
                      )
                      const isLiqRisk =
                        liqDist !== undefined &&
                        liqDist < LIQ_RISK_THRESHOLD_PCT
                      return (
                        <tr
                          key={i}
                          className={`border-b border-gray-800 last:border-0 transition-colors hover:bg-opacity-10 hover:bg-yellow-500 cursor-pointer ${
                            isLiqRisk ? 'pos-liq-risk' : ''
                          }`}
                          onClick={() => {
                            setSelectedChartSymbol(pos.symbol)
                            setChartUpdateKey(Date.now())
                            // Smooth scroll to chart with ref
                            if (chartSectionRef.current) {
                              chartSectionRef.current.scrollIntoView({
                                behavior: 'smooth',
                                block: 'start',
                              })
                            }
                          }}
                        >
                          <td className="px-1 py-3 font-mono font-semibold whitespace-nowrap text-left">
                            {pos.symbol}
                          </td>
                          <td className="px-1 py-3 whitespace-nowrap text-center">
                            <span
                              className="px-1.5 py-0.5 rounded text-[10px] font-bold"
                              style={
                                pos.side === 'long'
                                  ? {
                                      background: 'rgba(14, 203, 129, 0.1)',
                                      color: '#0ECB81',
                                    }
                                  : {
                                      background: 'rgba(246, 70, 93, 0.1)',
                                      color: '#F6465D',
                                    }
                              }
                            >
                              {t(
                                pos.side === 'long' ? 'long' : 'short',
                                language
                              )}
                            </span>
                          </td>
                          <td className="px-1 py-3 whitespace-nowrap text-center">
                            <button
                              type="button"
                              onClick={(e) => {
                                e.stopPropagation() // Prevent row click
                                handleClosePosition(
                                  pos.symbol,
                                  pos.side.toUpperCase()
                                )
                              }}
                              disabled={closingPosition === pos.symbol}
                              className="btn-danger inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-semibold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed mx-auto"
                              title={t('closePositionTitle', language)}
                            >
                              {closingPosition === pos.symbol ? (
                                <Loader2 className="w-3 h-3 animate-spin" />
                              ) : (
                                <LogOut className="w-3 h-3" />
                              )}
                              {t('closeBtn', language)}
                            </button>
                          </td>
                          <td
                            className="px-1 py-3 font-mono whitespace-nowrap text-right"
                            style={{ color: '#EAECEF' }}
                          >
                            {formatPrice(pos.entry_price)}
                          </td>
                          <td
                            className="px-1 py-3 font-mono whitespace-nowrap text-right"
                            style={{ color: '#EAECEF' }}
                          >
                            {formatPrice(pos.mark_price)}
                          </td>
                          <td
                            className="px-1 py-3 font-mono whitespace-nowrap text-right"
                            style={{ color: '#EAECEF' }}
                          >
                            {formatQuantity(pos.quantity)}
                          </td>
                          <td
                            className="px-1 py-3 font-mono font-bold whitespace-nowrap text-right"
                            style={{ color: '#EAECEF' }}
                          >
                            {formatUsd(pos.quantity * pos.mark_price)}
                          </td>
                          <td
                            className="px-1 py-3 font-mono whitespace-nowrap text-center"
                            style={{ color: '#F0B90B' }}
                          >
                            {pos.leverage}x
                          </td>
                          <td className="px-1 py-3 font-mono whitespace-nowrap text-right">
                            <span
                              style={{
                                color:
                                  pos.unrealized_pnl >= 0
                                    ? '#0ECB81'
                                    : '#F6465D',
                                fontWeight: 'bold',
                              }}
                            >
                              {pos.unrealized_pnl >= 0 ? '+' : ''}
                              {pos.unrealized_pnl.toFixed(2)}
                              <span className="text-[10px] ml-0.5">
                                ({pos.unrealized_pnl >= 0 ? '+' : ''}
                                {pos.unrealized_pnl_pct.toFixed(2)}%)
                              </span>
                            </span>
                          </td>
                          <td
                            className="px-1 py-3 font-mono whitespace-nowrap text-right"
                            style={{
                              color: isLiqRisk ? '#F6465D' : '#848E9C',
                              fontWeight: isLiqRisk ? 'bold' : 'normal',
                            }}
                          >
                            {isLiqRisk && (
                              <AlertTriangle className="w-3 h-3 inline mr-1 -mt-0.5" />
                            )}
                            {liqDist !== undefined
                              ? `${liqDist.toFixed(1)}%`
                              : '--'}
                          </td>
                          <td
                            className="px-1 py-3 font-mono whitespace-nowrap text-right"
                            style={{ color: '#848E9C' }}
                          >
                            {formatPrice(pos.liquidation_price)}
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            ) : null}

            {/* 移动端持仓卡片：小屏下表格不可用，改用卡片列表 */}
            {positions && positions.length > 0 && (
              <div className="md:hidden positions-mobile">
                {positions.map((pos, i) => {
                  const liqDist = liquidationDistancePct(
                    pos.mark_price,
                    pos.liquidation_price
                  )
                  const isLiqRisk =
                    liqDist !== undefined && liqDist < LIQ_RISK_THRESHOLD_PCT
                  return (
                    <div
                      key={i}
                      className={`positions-mobile__card ${
                        isLiqRisk ? 'pos-liq-risk' : ''
                      }`}
                    >
                      <div className="positions-mobile__top">
                        <button
                          type="button"
                          className="positions-mobile__symbol"
                          onClick={() => {
                            setSelectedChartSymbol(pos.symbol)
                            setChartUpdateKey(Date.now())
                            if (chartSectionRef.current) {
                              chartSectionRef.current.scrollIntoView({
                                behavior: 'smooth',
                                block: 'start',
                              })
                            }
                          }}
                        >
                          {pos.symbol}
                        </button>
                        <span
                          className="positions-mobile__side"
                          style={
                            pos.side === 'long'
                              ? {
                                  background: 'rgba(14,203,129,0.12)',
                                  color: '#0ECB81',
                                }
                              : {
                                  background: 'rgba(246,70,93,0.12)',
                                  color: '#F6465D',
                                }
                          }
                        >
                          {t(pos.side === 'long' ? 'long' : 'short', language)}
                          {pos.leverage}x
                        </span>
                        <span
                          className="positions-mobile__pnl"
                          style={{
                            color:
                              pos.unrealized_pnl >= 0 ? '#0ECB81' : '#F6465D',
                          }}
                        >
                          {pos.unrealized_pnl >= 0 ? '+' : ''}
                          {pos.unrealized_pnl.toFixed(2)}
                          <span className="text-[10px]">
                            ({pos.unrealized_pnl >= 0 ? '+' : ''}
                            {pos.unrealized_pnl_pct.toFixed(2)}%)
                          </span>
                        </span>
                      </div>
                      <div className="positions-mobile__grid">
                        <span>
                          {t('entryShort', language)}:{' '}
                          {formatPrice(pos.entry_price)}
                        </span>
                        <span>
                          {t('markShort', language)}:{' '}
                          {formatPrice(pos.mark_price)}
                        </span>
                        <span>
                          {t('qtyShort', language)}:{' '}
                          {formatQuantity(pos.quantity)}
                        </span>
                        <span>
                          {t('valueShort', language)}:{' '}
                          {formatUsd(pos.quantity * pos.mark_price)}
                        </span>
                        <span>
                          {t('liqShort', language)}:{' '}
                          {formatPrice(pos.liquidation_price)}
                        </span>
                        <span
                          style={{
                            color: isLiqRisk ? '#F6465D' : undefined,
                            fontWeight: isLiqRisk ? 'bold' : undefined,
                          }}
                        >
                          {t('distanceToLiq', language)}:{' '}
                          {liqDist !== undefined
                            ? `${liqDist.toFixed(1)}%`
                            : '--'}
                          {isLiqRisk && (
                            <AlertTriangle className="w-3 h-3 inline ml-1 -mt-0.5" />
                          )}
                        </span>
                      </div>
                      <div className="positions-mobile__footer">
                        <button
                          type="button"
                          onClick={() =>
                            handleClosePosition(
                              pos.symbol,
                              pos.side.toUpperCase()
                            )
                          }
                          disabled={closingPosition === pos.symbol}
                          className="btn-danger inline-flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs font-bold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed"
                        >
                          {closingPosition === pos.symbol ? (
                            <Loader2 className="w-3.5 h-3.5 animate-spin" />
                          ) : (
                            <LogOut className="w-3.5 h-3.5" />
                          )}
                          {t('closeBtn', language)}
                        </button>
                      </div>
                    </div>
                  )
                })}
              </div>
            )}

            {(!positions || positions.length === 0) && (
              <div className="dashboard-empty-state">
                <BriefcaseBusiness size={34} />
                <div className="text-lg font-semibold mb-2">
                  {t('noPositions', language)}
                </div>
                <div className="text-sm">
                  {t('noActivePositions', language)}
                </div>
              </div>
            )}
          </div>

          {/* Performance Metrics & Order History */}
          <PerformanceMetricsPanel
            traderId={selectedTrader.trader_id}
            language={language}
          />

          {/* Order & Fill History */}
          <OrderHistoryPanel
            traderId={selectedTrader.trader_id}
            language={language}
          />
        </div>
        {/* 左侧结束 */}

        {/* 右侧：Recent Decisions - 卡片容器 */}
        <div
          className="binance-card dashboard-panel animate-slide-in h-fit lg:sticky lg:top-20 lg:max-h-[calc(100vh-96px)]"
          style={{ animationDelay: '0.2s' }}
        >
          {/* 标题 */}
          <div className="dashboard-panel__header dashboard-panel__header--divided">
            <div className="dashboard-panel__icon dashboard-panel__icon--violet">
              <BrainCircuit size={20} />
            </div>
            <div className="flex-1">
              <h2 className="dashboard-panel__title">
                {t('recentDecisions', language)}
              </h2>
              {decisions && decisions.length > 0 && (
                <div className="text-xs" style={{ color: '#848E9C' }}>
                  {t('lastCycles', language, { count: decisions.length })}
                </div>
              )}
            </div>
            {/* 导出决策日志 */}
            <button
              type="button"
              onClick={handleExportDecisions}
              disabled={!decisions || decisions.length === 0}
              className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-[11px] font-bold transition-all hover:scale-105 disabled:opacity-40 disabled:cursor-not-allowed flex-shrink-0"
              style={{
                background: '#1E2329',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
              title={t('exportDecisions', language)}
            >
              <Download className="w-3.5 h-3.5" />
              JSON
            </button>
            {/* 数量选择器 */}
            <select
              value={decisionsLimit}
              onChange={(e) => onDecisionsLimitChange(Number(e.target.value))}
              className="dashboard-limit-select"
            >
              <option value={5}>5</option>
              <option value={10}>10</option>
              <option value={20}>20</option>
              <option value={50}>50</option>
              <option value={100}>100</option>
            </select>
          </div>

          {/* 决策列表 - 可滚动 */}
          <div
            className="space-y-4 overflow-y-auto pr-2"
            style={{ maxHeight: 'calc(100vh - 280px)' }}
          >
            {decisions && decisions.length > 0 ? (
              decisions.map((decision, i) => (
                <DecisionCard key={i} decision={decision} language={language} />
              ))
            ) : (
              <div className="dashboard-empty-state">
                <BrainCircuit size={34} />
                <div
                  className="text-lg font-semibold mb-2"
                  style={{ color: '#EAECEF' }}
                >
                  {t('noDecisionsYet', language)}
                </div>
                <div className="text-sm" style={{ color: '#848E9C' }}>
                  {t('aiDecisionsWillAppear', language)}
                </div>
              </div>
            )}
          </div>
        </div>
        {/* 右侧结束 */}
      </div>
    </div>
  )
}

// Stat Card Component - Binance Style Enhanced
function StatCard({
  title,
  value,
  change,
  positive,
  subtitle,
  icon,
  accent = 'yellow',
}: {
  title: string
  value: string
  change?: number
  positive?: boolean
  subtitle?: string
  icon: ReactNode
  accent?: 'yellow' | 'blue' | 'green' | 'red' | 'violet'
}) {
  return (
    <div className={`stat-card stat-card--${accent} animate-fade-in`}>
      <div className="stat-card__header">
        <div className="stat-card__title">{title}</div>
        <div className="stat-card__icon">{icon}</div>
      </div>
      <div className="stat-card__value">{value}</div>
      {change !== undefined && (
        <div
          className={`stat-card__change ${positive ? 'is-positive' : 'is-negative'}`}
        >
          {positive ? '▲' : '▼'} {positive ? '+' : ''}
          {change.toFixed(2)}%
        </div>
      )}
      {subtitle && <div className="stat-card__subtitle">{subtitle}</div>}
    </div>
  )
}

// Wrap App with providers
export default function AppWithProviders() {
  return (
    <LanguageProvider>
      <AuthProvider>
        <ConfirmDialogProvider>
          <App />
        </ConfirmDialogProvider>
      </AuthProvider>
    </LanguageProvider>
  )
}
