import { useState } from 'react'
import { api } from '../lib/api'
import { t, type Language } from '../i18n/translations'
import { confirmToast, notify } from '../lib/notify'
import { mutate } from 'swr'
import { Loader2, Crosshair } from 'lucide-react'

/**
 * 手动下单弹窗：开/平仓走与 AI 决策完全一致的执行链路
 * （风控校验、后端仓位计算、止损止盈保护单、决策日志留痕）。
 */
export function ManualOrderModal({
  traderId,
  language,
  onClose,
}: {
  traderId: string
  language: Language
  onClose: () => void
}) {
  const [symbol, setSymbol] = useState('BTCUSDT')
  const [action, setAction] = useState<
    'open_long' | 'open_short' | 'close_long' | 'close_short'
  >('open_long')
  const [stopLossPct, setStopLossPct] = useState('2')
  const [takeProfitPct, setTakeProfitPct] = useState('4')
  const [submitting, setSubmitting] = useState(false)

  const isClose = action === 'close_long' || action === 'close_short'

  const handleSubmit = async () => {
    const confirmed = await confirmToast(
      t('confirmManualOrder', language, {
        action: action.replace('_', ' ').toUpperCase(),
        symbol: symbol.toUpperCase(),
      }),
      {
        title: t('manualOrder', language),
        okText: t('confirmBtn', language),
        cancelText: t('cancelBtn', language),
      }
    )
    if (!confirmed) return

    setSubmitting(true)
    try {
      await api.manualOrder(traderId, {
        symbol: symbol.toUpperCase().trim(),
        action,
        ...(isClose
          ? {}
          : {
              stop_loss_pct: Number(stopLossPct),
              take_profit_pct: Number(takeProfitPct),
            }),
      })
      notify.success(t('manualOrderSuccess', language))
      await Promise.all([
        mutate(`positions-${traderId}`),
        mutate(`account-${traderId}`),
      ])
      onClose()
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : t('manualOrderFailed', language)
      )
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center p-4"
      style={{ background: 'rgba(0,0,0,0.7)' }}
      onClick={onClose}
    >
      <div
        className="w-full max-w-md rounded-xl p-5 space-y-4 max-h-[90vh] overflow-y-auto"
        style={{ background: '#181A20', border: '1px solid #2B3139' }}
        onClick={(e) => e.stopPropagation()}
      >
        <h2
          className="text-lg font-bold flex items-center gap-2"
          style={{ color: '#EAECEF' }}
        >
          <Crosshair size={19} />
          {t('manualOrder', language)}
        </h2>

        {/* 方向 */}
        <div className="grid grid-cols-2 gap-2">
          {(
            ['open_long', 'open_short', 'close_long', 'close_short'] as const
          ).map((a) => (
            <button
              key={a}
              type="button"
              onClick={() => setAction(a)}
              className="px-2 py-2 rounded-lg text-xs font-bold transition-all"
              style={{
                background:
                  action === a ? 'rgba(240, 185, 11, 0.15)' : '#0B0E11',
                border: `1px solid ${action === a ? '#F0B90B' : '#2B3139'}`,
                color:
                  action === a
                    ? '#F0B90B'
                    : a.includes('open')
                      ? a === 'open_long'
                        ? '#0ECB81'
                        : '#F6465D'
                      : a === 'close_long'
                        ? '#0ECB81'
                        : '#F6465D',
              }}
            >
              {a.replace('_', ' ').toUpperCase()}
            </button>
          ))}
        </div>

        {/* 交易对 */}
        <div>
          <label
            className="text-xs font-bold block mb-1.5"
            style={{ color: '#848E9C' }}
          >
            {t('symbol', language)}
          </label>
          <input
            value={symbol}
            onChange={(e) => setSymbol(e.target.value.toUpperCase())}
            className="w-full px-3 py-2 rounded-lg text-sm outline-none font-mono"
            style={{
              background: '#0B0E11',
              border: '1px solid #2B3139',
              color: '#EAECEF',
            }}
            placeholder="BTCUSDT"
          />
        </div>

        {/* 止损/止盈（仅开仓） */}
        {!isClose && (
          <>
            <div>
              <label
                className="text-xs font-bold block mb-1.5"
                style={{ color: '#848E9C' }}
              >
                {t('stopLossPctLabel', language)}
              </label>
              <input
                type="number"
                value={stopLossPct}
                onChange={(e) => setStopLossPct(e.target.value)}
                min="0.1"
                max="49"
                step="0.1"
                className="w-full px-3 py-2 rounded-lg text-sm outline-none font-mono"
                style={{
                  background: '#0B0E11',
                  border: '1px solid #2B3139',
                  color: '#F6465D',
                }}
              />
            </div>
            <div>
              <label
                className="text-xs font-bold block mb-1.5"
                style={{ color: '#848E9C' }}
              >
                {t('takeProfitPctLabel', language)}
              </label>
              <input
                type="number"
                value={takeProfitPct}
                onChange={(e) => setTakeProfitPct(e.target.value)}
                min="0.1"
                max="99"
                step="0.1"
                className="w-full px-3 py-2 rounded-lg text-sm outline-none font-mono"
                style={{
                  background: '#0B0E11',
                  border: '1px solid #2B3139',
                  color: '#0ECB81',
                }}
              />
            </div>
            <p
              className="text-[11px] leading-relaxed"
              style={{ color: '#5E6673' }}
            >
              {t('manualOrderHelp', language)}
            </p>
          </>
        )}

        {isClose && (
          <p
            className="text-[11px] leading-relaxed"
            style={{ color: '#5E6673' }}
          >
            {t('manualCloseHelp', language)}
          </p>
        )}

        {/* 按钮 */}
        <div className="flex gap-2 pt-1">
          <button
            type="button"
            onClick={onClose}
            className="flex-1 px-4 py-2 rounded-lg text-sm font-bold"
            style={{
              background: '#1E2329',
              color: '#848E9C',
              border: '1px solid #2B3139',
            }}
          >
            {t('cancelBtn', language)}
          </button>
          <button
            type="button"
            onClick={handleSubmit}
            disabled={submitting || !symbol.trim()}
            className="flex-1 px-4 py-2 rounded-lg text-sm font-bold transition-all hover:scale-105 disabled:opacity-50 inline-flex items-center justify-center gap-1.5"
            style={{
              background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
              color: '#0B0E11',
            }}
          >
            {submitting && <Loader2 size={14} className="animate-spin" />}
            {t('submitOrder', language)}
          </button>
        </div>
      </div>
    </div>
  )
}
