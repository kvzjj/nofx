import useSWR from 'swr'
import { api } from '../lib/api'
import { t, type Language } from '../i18n/translations'
import { PieChart, TrendingDown } from 'lucide-react'

/**
 * 风控/绩效指标面板：
 * - 最大回撤（基于权益快照）
 * - 平仓胜率 / 盈亏比 / 平均盈亏
 * - 按币种的已实现盈亏分布
 */
export function PerformanceMetricsPanel({
  traderId,
  language,
}: {
  traderId: string
  language: Language
}) {
  const { data } = useSWR(
    traderId ? `performance-metrics-${traderId}` : null,
    () => api.getPerformanceMetrics(traderId),
    { refreshInterval: 60000, dedupingInterval: 30000 }
  )

  const exec = data?.execution
  const dd = data?.drawdown
  const hasClosed = (exec?.closed_trades ?? 0) > 0
  const symbolPnL = (exec?.pnl_by_symbol ?? []).slice(0, 10)
  const maxAbs = Math.max(...symbolPnL.map((s) => Math.abs(s.realized_pnl)), 1)

  return (
    <div className="binance-card dashboard-panel animate-slide-in">
      <div className="dashboard-panel__header">
        <div>
          <div className="dashboard-panel__eyebrow">
            {t('riskAnalytics', language)}
          </div>
          <h2 className="dashboard-panel__title">
            <TrendingDown size={19} />
            {t('performanceMetrics', language)}
          </h2>
        </div>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-6 gap-3 px-1 pb-2">
        <MetricTile
          label={t('maxDrawdown', language)}
          value={
            dd && dd.max_drawdown_pct > 0
              ? `-${dd.max_drawdown_pct.toFixed(2)}%`
              : '--'
          }
          sub={
            dd && dd.max_drawdown_abs > 0
              ? `-${dd.max_drawdown_abs.toFixed(2)} USDT`
              : undefined
          }
          negative
        />
        <MetricTile
          label={t('closedWinRate', language)}
          value={hasClosed ? `${exec!.win_rate.toFixed(1)}%` : '--'}
          sub={
            hasClosed
              ? `${exec!.winning_trades}W / ${exec!.losing_trades}L`
              : undefined
          }
          positive={hasClosed && exec!.win_rate >= 50}
          negative={hasClosed && exec!.win_rate < 50}
        />
        <MetricTile
          label={t('profitFactor', language)}
          value={
            hasClosed && exec!.profit_factor > 0
              ? exec!.profit_factor.toFixed(2)
              : '--'
          }
          sub={
            hasClosed && exec!.gross_loss > 0
              ? `${t('grossProfit', language)}: ${exec!.gross_profit.toFixed(0)} / ${t('grossLoss', language)}: ${exec!.gross_loss.toFixed(0)}`
              : undefined
          }
          positive={hasClosed && exec!.profit_factor >= 1}
          negative={hasClosed && exec!.profit_factor < 1}
        />
        <MetricTile
          label={t('realizedPnL', language)}
          value={
            exec && exec.closed_trades > 0
              ? `${exec.total_realized_pnl >= 0 ? '+' : ''}${exec.total_realized_pnl.toFixed(2)}`
              : '--'
          }
          sub={
            exec && exec.closed_trades > 0
              ? `${t('feeCol', language)}: ${exec.total_fee.toFixed(2)}`
              : undefined
          }
          positive={hasClosed && exec!.total_realized_pnl >= 0}
          negative={hasClosed && exec!.total_realized_pnl < 0}
        />
        <MetricTile
          label={t('avgWinLabel', language)}
          value={
            exec && exec.winning_trades > 0
              ? `+${exec.avg_win.toFixed(2)}`
              : '--'
          }
          positive
        />
        <MetricTile
          label={t('avgLossLabel', language)}
          value={
            exec && exec.losing_trades > 0
              ? `-${exec.avg_loss.toFixed(2)}`
              : '--'
          }
          negative
        />
      </div>

      {/* 按币种已实现盈亏 */}
      {symbolPnL.length > 0 && (
        <div className="px-1 pt-2 pb-1">
          <div
            className="text-[11px] font-bold uppercase tracking-wide mb-2 flex items-center gap-1.5"
            style={{ color: '#848E9C' }}
          >
            <PieChart size={13} />
            {t('pnlBySymbol', language)}
          </div>
          <div className="space-y-1.5">
            {symbolPnL.map((item) => (
              <div
                key={item.symbol}
                className="flex items-center gap-2 text-xs"
              >
                <span
                  className="font-mono font-semibold w-24 truncate flex-shrink-0"
                  style={{ color: '#EAECEF' }}
                >
                  {item.symbol}
                </span>
                <div
                  className="flex-1 h-2 rounded overflow-hidden relative"
                  style={{ background: '#1E2329' }}
                >
                  <div
                    className="absolute top-0 h-full rounded"
                    style={{
                      width: `${(Math.abs(item.realized_pnl) / maxAbs) * 50}%`,
                      left: item.realized_pnl >= 0 ? '50%' : undefined,
                      right: item.realized_pnl < 0 ? '50%' : undefined,
                      background:
                        item.realized_pnl >= 0 ? '#0ECB81' : '#F6465D',
                    }}
                  />
                  {/* 中线 */}
                  <div
                    className="absolute top-0 h-full"
                    style={{ left: '50%', width: '1px', background: '#2B3139' }}
                  />
                </div>
                <span
                  className="font-mono w-20 text-right flex-shrink-0"
                  style={{
                    color: item.realized_pnl >= 0 ? '#0ECB81' : '#F6465D',
                    fontWeight: 'bold',
                  }}
                >
                  {item.realized_pnl >= 0 ? '+' : ''}
                  {item.realized_pnl.toFixed(2)}
                </span>
                <span
                  className="text-[10px] w-12 text-right flex-shrink-0"
                  style={{ color: '#848E9C' }}
                >
                  {item.closed_trades}x
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

function MetricTile({
  label,
  value,
  sub,
  positive,
  negative,
}: {
  label: string
  value: string
  sub?: string
  positive?: boolean
  negative?: boolean
}) {
  const color = positive ? '#0ECB81' : negative ? '#F6465D' : '#EAECEF'
  return (
    <div
      className="rounded-lg p-3"
      style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
    >
      <div
        className="text-[10px] uppercase tracking-wide mb-1"
        style={{ color: '#848E9C' }}
      >
        {label}
      </div>
      <div
        className="text-lg font-bold font-mono leading-tight"
        style={{ color }}
      >
        {value}
      </div>
      {sub && (
        <div
          className="text-[10px] mt-0.5 truncate"
          style={{ color: '#5E6673' }}
          title={sub}
        >
          {sub}
        </div>
      )}
    </div>
  )
}
