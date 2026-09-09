import { useState } from 'react'
import useSWR from 'swr'
import { api } from '../lib/api'
import { formatPrice, formatQuantity } from '../lib/format'
import { exportCsv, timestampSlug } from '../lib/export'
import { t, type Language } from '../i18n/translations'
import { ClipboardList, Loader2, History, Download } from 'lucide-react'
import type { TradeOrderRecord, ExchangeFillRecord } from '../types'

const PAGE_SIZE = 20

// 订单状态 → 展示文案/颜色
function orderStatusMeta(status: string): { color: string; bold?: boolean } {
  switch (status) {
    case 'FILLED':
      return { color: '#0ECB81', bold: true }
    case 'PARTIAL':
      return { color: '#F0B90B', bold: true }
    case 'INTENT':
    case 'SUBMITTED':
      return { color: '#848E9C' }
    case 'FAILED':
    case 'REJECTED':
    case 'CANCELED':
      return { color: '#F6465D' }
    default:
      return { color: '#848E9C' }
  }
}

// 订单动作 → 展示文案
function actionLabel(action: string): string {
  switch (action) {
    case 'open_long':
      return 'OPEN LONG'
    case 'open_short':
      return 'OPEN SHORT'
    case 'close_long':
      return 'CLOSE LONG'
    case 'close_short':
      return 'CLOSE SHORT'
    default:
      return action.toUpperCase()
  }
}

function formatTime(value: string | undefined): string {
  if (!value) return '--'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return '--'
  return d.toLocaleString(undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

/**
 * 历史订单/成交记录面板：
 * - Orders 标签：本地持久化的订单 saga（含状态、SL/TP、保护状态）
 * - Fills 标签：交易所成交回报（含已实现盈亏、手续费）
 */
export function OrderHistoryPanel({
  traderId,
  language,
}: {
  traderId: string
  language: Language
}) {
  const [tab, setTab] = useState<'orders' | 'fills'>('orders')
  const [offset, setOffset] = useState(0)
  const [exporting, setExporting] = useState(false)

  // 导出当前标签页（最多 1000 条）为 CSV
  const handleExport = async () => {
    setExporting(true)
    try {
      if (tab === 'orders') {
        const res = await api.getOrderHistory(traderId, 1000, 0)
        exportCsv(
          `orders-${traderId.slice(0, 8)}-${timestampSlug()}.csv`,
          [
            'Time',
            'Symbol',
            'Action',
            'Price',
            'Executed Qty',
            'Requested Qty',
            'Stop Loss',
            'Take Profit',
            'Fee',
            'Status',
            'Protection',
            'Error',
          ],
          res.orders.map((o) => [
            o.created_at,
            o.symbol,
            o.action,
            o.avg_price,
            o.executed_qty,
            o.requested_qty,
            o.stop_loss,
            o.take_profit,
            o.fee,
            o.status,
            o.protection_status,
            o.last_error,
          ])
        )
      } else {
        const res = await api.getFillHistory(traderId, 1000, 0)
        exportCsv(
          `fills-${traderId.slice(0, 8)}-${timestampSlug()}.csv`,
          [
            'Time',
            'Symbol',
            'Side',
            'Price',
            'Quantity',
            'Fee',
            'Realized PnL',
          ],
          res.fills.map((f) => [
            f.executed_at,
            f.symbol,
            f.side,
            f.price,
            f.quantity,
            f.fee,
            f.realized_pnl,
          ])
        )
      }
    } finally {
      setExporting(false)
    }
  }

  const { data: ordersData, isLoading: ordersLoading } = useSWR(
    traderId ? `orders-${traderId}-${offset}` : null,
    () => api.getOrderHistory(traderId, PAGE_SIZE, offset),
    { refreshInterval: 30000, dedupingInterval: 15000 }
  )

  const { data: fillsData, isLoading: fillsLoading } = useSWR(
    traderId ? `fills-${traderId}-${offset}` : null,
    () => api.getFillHistory(traderId, PAGE_SIZE, offset),
    { refreshInterval: 30000, dedupingInterval: 15000 }
  )

  const orders: TradeOrderRecord[] = ordersData?.orders ?? []
  const fills: ExchangeFillRecord[] = fillsData?.fills ?? []
  const total =
    tab === 'orders' ? (ordersData?.total ?? 0) : (fillsData?.total ?? 0)
  const loading = tab === 'orders' ? ordersLoading : fillsLoading
  const hasMore = offset + PAGE_SIZE < total

  return (
    <div className="binance-card dashboard-panel animate-slide-in">
      <div className="dashboard-panel__header">
        <div>
          <div className="dashboard-panel__eyebrow">
            {t('executionHistory', language)}
          </div>
          <h2 className="dashboard-panel__title">
            <History size={19} />
            {t('orderHistory', language)}
          </h2>
        </div>
        <div className="dashboard-panel__actions flex items-center gap-2">
          {/* 标签切换 */}
          <div
            className="flex rounded-lg overflow-hidden text-[11px] font-bold"
            style={{ border: '1px solid #2B3139' }}
          >
            <button
              type="button"
              onClick={() => {
                setTab('orders')
                setOffset(0)
              }}
              className="px-3 py-1.5 transition-colors"
              style={{
                background: tab === 'orders' ? '#2B3139' : 'transparent',
                color: tab === 'orders' ? '#FCD535' : '#848E9C',
              }}
            >
              {t('ordersTab', language)}
            </button>
            <button
              type="button"
              onClick={() => {
                setTab('fills')
                setOffset(0)
              }}
              className="px-3 py-1.5 transition-colors"
              style={{
                background: tab === 'fills' ? '#2B3139' : 'transparent',
                color: tab === 'fills' ? '#FCD535' : '#848E9C',
              }}
            >
              {t('fillsTab', language)}
            </button>
          </div>
          <button
            type="button"
            onClick={handleExport}
            disabled={exporting}
            title={t('exportCsv', language)}
            className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-[11px] font-bold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed"
            style={{
              background: '#1E2329',
              border: '1px solid #2B3139',
              color: '#EAECEF',
            }}
          >
            {exporting ? (
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
            ) : (
              <Download className="w-3.5 h-3.5" />
            )}
            {t('exportCsv', language)}
          </button>
          <div className="dashboard-count-badge">
            {total} {t('recordsUnit', language)}
          </div>
        </div>
      </div>

      {tab === 'orders' ? (
        orders.length > 0 ? (
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead className="text-left border-b border-gray-800">
                <tr>
                  <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-left">
                    {t('timeCol', language)}
                  </th>
                  <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-left">
                    {t('symbol', language)}
                  </th>
                  <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-center">
                    {t('actionCol', language)}
                  </th>
                  <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right">
                    {t('priceCol', language)}
                  </th>
                  <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right">
                    {t('qtyShort', language)}
                  </th>
                  <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right">
                    SL / TP
                  </th>
                  <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right">
                    {t('feeCol', language)}
                  </th>
                  <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-center">
                    {t('statusCol', language)}
                  </th>
                </tr>
              </thead>
              <tbody>
                {orders.map((order, i) => {
                  const meta = orderStatusMeta(order.status)
                  return (
                    <tr
                      key={`${order.order_id}-${i}`}
                      className="border-b border-gray-800 last:border-0"
                      title={order.last_error || undefined}
                    >
                      <td
                        className="px-1 py-2.5 font-mono whitespace-nowrap text-left"
                        style={{ color: '#848E9C' }}
                      >
                        {formatTime(order.created_at)}
                      </td>
                      <td className="px-1 py-2.5 font-mono font-semibold whitespace-nowrap text-left">
                        {order.symbol}
                      </td>
                      <td className="px-1 py-2.5 whitespace-nowrap text-center">
                        <span
                          className="text-[10px] font-bold"
                          style={{
                            color: order.action.includes('open')
                              ? '#0ECB81'
                              : '#F6465D',
                          }}
                        >
                          {actionLabel(order.action)}
                        </span>
                      </td>
                      <td
                        className="px-1 py-2.5 font-mono whitespace-nowrap text-right"
                        style={{ color: '#EAECEF' }}
                      >
                        {order.avg_price > 0
                          ? formatPrice(order.avg_price)
                          : '--'}
                      </td>
                      <td
                        className="px-1 py-2.5 font-mono whitespace-nowrap text-right"
                        style={{ color: '#EAECEF' }}
                      >
                        {formatQuantity(order.executed_qty)}
                        {order.executed_qty !== order.requested_qty && (
                          <span
                            className="text-[10px] ml-0.5"
                            style={{ color: '#848E9C' }}
                          >
                            /{formatQuantity(order.requested_qty)}
                          </span>
                        )}
                      </td>
                      <td
                        className="px-1 py-2.5 font-mono whitespace-nowrap text-right text-[10px]"
                        style={{ color: '#848E9C' }}
                      >
                        {order.stop_loss > 0 || order.take_profit > 0 ? (
                          <>
                            <span style={{ color: '#F6465D' }}>
                              {order.stop_loss > 0
                                ? formatPrice(order.stop_loss)
                                : '--'}
                            </span>
                            {' / '}
                            <span style={{ color: '#0ECB81' }}>
                              {order.take_profit > 0
                                ? formatPrice(order.take_profit)
                                : '--'}
                            </span>
                          </>
                        ) : (
                          '--'
                        )}
                      </td>
                      <td
                        className="px-1 py-2.5 font-mono whitespace-nowrap text-right"
                        style={{ color: '#848E9C' }}
                      >
                        {order.fee > 0 ? order.fee.toFixed(4) : '--'}
                      </td>
                      <td className="px-1 py-2.5 whitespace-nowrap text-center">
                        <span
                          className="text-[10px] font-bold"
                          style={{ color: meta.color }}
                        >
                          {order.status}
                        </span>
                        {order.protection_status === 'PROTECTED' && (
                          <span
                            className="ml-1 text-[9px]"
                            style={{ color: '#0ECB81' }}
                            title={t('protectionActive', language)}
                          >
                            🛡
                          </span>
                        )}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        ) : (
          <EmptyState
            icon={<ClipboardList size={30} />}
            language={language}
            title={t('noOrdersYet', language)}
            description={t('noOrdersDesc', language)}
            loading={loading}
          />
        )
      ) : fills.length > 0 ? (
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead className="text-left border-b border-gray-800">
              <tr>
                <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-left">
                  {t('timeCol', language)}
                </th>
                <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-left">
                  {t('symbol', language)}
                </th>
                <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-center">
                  {t('side', language)}
                </th>
                <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right">
                  {t('priceCol', language)}
                </th>
                <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right">
                  {t('qtyShort', language)}
                </th>
                <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right">
                  {t('feeCol', language)}
                </th>
                <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right">
                  {t('realizedPnL', language)}
                </th>
              </tr>
            </thead>
            <tbody>
              {fills.map((fill, i) => (
                <tr
                  key={`${fill.trade_id}-${i}`}
                  className="border-b border-gray-800 last:border-0"
                >
                  <td
                    className="px-1 py-2.5 font-mono whitespace-nowrap text-left"
                    style={{ color: '#848E9C' }}
                  >
                    {formatTime(fill.executed_at)}
                  </td>
                  <td className="px-1 py-2.5 font-mono font-semibold whitespace-nowrap text-left">
                    {fill.symbol}
                  </td>
                  <td className="px-1 py-2.5 whitespace-nowrap text-center">
                    <span
                      className="text-[10px] font-bold"
                      style={{
                        color:
                          fill.side === 'BUY' || fill.side === 'buy'
                            ? '#0ECB81'
                            : '#F6465D',
                      }}
                    >
                      {fill.side.toUpperCase()}
                    </span>
                  </td>
                  <td
                    className="px-1 py-2.5 font-mono whitespace-nowrap text-right"
                    style={{ color: '#EAECEF' }}
                  >
                    {formatPrice(fill.price)}
                  </td>
                  <td
                    className="px-1 py-2.5 font-mono whitespace-nowrap text-right"
                    style={{ color: '#EAECEF' }}
                  >
                    {formatQuantity(fill.quantity)}
                  </td>
                  <td
                    className="px-1 py-2.5 font-mono whitespace-nowrap text-right"
                    style={{ color: '#848E9C' }}
                  >
                    {fill.fee > 0 ? fill.fee.toFixed(4) : '--'}
                  </td>
                  <td className="px-1 py-2.5 font-mono whitespace-nowrap text-right font-bold">
                    {fill.realized_pnl !== 0 ? (
                      <span
                        style={{
                          color: fill.realized_pnl >= 0 ? '#0ECB81' : '#F6465D',
                        }}
                      >
                        {fill.realized_pnl >= 0 ? '+' : ''}
                        {fill.realized_pnl.toFixed(2)}
                      </span>
                    ) : (
                      <span style={{ color: '#848E9C' }}>--</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <EmptyState
          icon={<ClipboardList size={30} />}
          language={language}
          title={t('noFillsYet', language)}
          description={t('noFillsDesc', language)}
          loading={loading}
        />
      )}

      {/* 分页 */}
      {(total > PAGE_SIZE || offset > 0) && (
        <div
          className="flex items-center justify-between px-1 pt-3 text-xs"
          style={{ color: '#848E9C' }}
        >
          <span>
            {offset + 1}–{Math.min(offset + PAGE_SIZE, total)} / {total}
          </span>
          <div className="flex items-center gap-2">
            <button
              type="button"
              disabled={offset === 0}
              onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
              className="px-2.5 py-1 rounded font-semibold transition-all hover:scale-105 disabled:opacity-40 disabled:cursor-not-allowed"
              style={{
                background: '#1E2329',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
            >
              ←
            </button>
            <button
              type="button"
              disabled={!hasMore}
              onClick={() => setOffset(offset + PAGE_SIZE)}
              className="px-2.5 py-1 rounded font-semibold transition-all hover:scale-105 disabled:opacity-40 disabled:cursor-not-allowed"
              style={{
                background: '#1E2329',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
            >
              →
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

function EmptyState({
  icon,
  title,
  description,
  loading,
}: {
  icon: React.ReactNode
  title: string
  description: string
  language: Language
  loading?: boolean
}) {
  return (
    <div className="dashboard-empty-state">
      {loading ? <Loader2 className="animate-spin" size={30} /> : icon}
      <div className="text-lg font-semibold mb-2" style={{ color: '#EAECEF' }}>
        {title}
      </div>
      <div className="text-sm" style={{ color: '#848E9C' }}>
        {description}
      </div>
    </div>
  )
}
