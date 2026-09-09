import { Shield, AlertTriangle } from 'lucide-react'
import type { RiskControlConfig } from '../../types'

interface RiskControlEditorProps {
  config: RiskControlConfig
  onChange: (config: RiskControlConfig) => void
  disabled?: boolean
  language: string
}

export function RiskControlEditor({
  config,
  onChange,
  disabled,
  language,
}: RiskControlEditorProps) {
  const t = (key: string) => {
    const translations: Record<string, Record<string, string>> = {
      positionLimits: { zh: '仓位限制', en: 'Position Limits' },
      maxPositions: { zh: '最大持仓数量', en: 'Max Positions' },
      maxPositionsDesc: { zh: '同时持有的最大币种数量', en: 'Maximum coins held simultaneously' },
      // Trading leverage (exchange leverage)
      tradingLeverage: { zh: '交易杠杆（交易所杠杆）', en: 'Trading Leverage (Exchange)' },
      btcEthLeverage: { zh: 'BTC/ETH 交易杠杆', en: 'BTC/ETH Trading Leverage' },
      btcEthLeverageDesc: { zh: '交易所开仓使用的杠杆倍数', en: 'Exchange leverage for opening positions' },
      altcoinLeverage: { zh: '山寨币交易杠杆', en: 'Altcoin Trading Leverage' },
      altcoinLeverageDesc: { zh: '交易所开仓使用的杠杆倍数', en: 'Exchange leverage for opening positions' },
      positionValueLimits: { zh: '金额上限（代码强制）', en: 'Notional Limits (CODE ENFORCED)' },
      maxPositionSize: { zh: '单次最大开单金额', en: 'Max Opening Order' },
      maxPositionSizeDesc: { zh: '每次开仓允许的最大名义金额', en: 'Maximum notional value per opening order' },
      maxTotalPositionSize: { zh: '总仓位最大金额', en: 'Max Total Positions' },
      maxTotalPositionSizeDesc: { zh: '开仓后全部持仓的最大名义金额', en: 'Maximum total open-position notional value' },
      riskParameters: { zh: '风险参数', en: 'Risk Parameters' },
      minRiskReward: { zh: '最小风险回报比', en: 'Min Risk/Reward Ratio' },
      minRiskRewardDesc: { zh: '开仓要求的最低盈亏比', en: 'Minimum profit ratio for opening' },
      entryRequirements: { zh: '开仓要求', en: 'Entry Requirements' },
      minPositionSize: { zh: '最小开仓金额', en: 'Min Position Size' },
      minPositionSizeDesc: { zh: 'USDT 最小名义价值', en: 'Minimum notional value in USDT' },
      minConfidence: { zh: '最小信心度', en: 'Min Confidence' },
      minConfidenceDesc: { zh: 'AI 开仓信心度阈值', en: 'AI confidence threshold for entry' },
      orderExecution: { zh: '订单执行', en: 'Order Execution' },
      orderType: { zh: '开仓订单类型', en: 'Entry Order Type' },
      orderTypeDesc: { zh: '平仓仍使用市价单，避免限价平仓无法成交', en: 'Close orders still use market orders to avoid unfilled exits' },
      marketOrder: { zh: '市价单', en: 'Market' },
      limitOrder: { zh: '限价单', en: 'Limit' },
      limitOffset: { zh: '限价偏移', en: 'Limit Offset' },
      limitOffsetDesc: { zh: '做多挂当前价下方，做空挂当前价上方', en: 'Long below current price, short above current price' },
      pendingTimeout: { zh: '挂单监控时长', en: 'Pending Timeout' },
      pendingTimeoutDesc: { zh: '限价挂单超过该时长后自动撤销（分钟）', en: 'Resting limit entries are canceled after this window (minutes)' },
      pendingPoll: { zh: '挂单轮询间隔', en: 'Pending Poll' },
      pendingPollDesc: { zh: '挂单成交检查间隔（秒）', en: 'How often pending fills are checked (seconds)' },
    }
    return translations[key]?.[language] || key
  }

  const updateField = <K extends keyof RiskControlConfig>(
    key: K,
    value: RiskControlConfig[K]
  ) => {
    if (!disabled) {
      onChange({ ...config, [key]: value })
    }
  }

  return (
    <div className="space-y-6">
      {/* Position Limits */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('positionLimits')}
          </h3>
        </div>

        <div className="grid grid-cols-1 gap-4 mb-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('maxPositions')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('maxPositionsDesc')}
            </p>
            <input
              type="number"
              value={config.max_positions ?? 3}
              onChange={(e) =>
                updateField('max_positions', parseInt(e.target.value) || 3)
              }
              disabled={disabled}
              min={1}
              max={10}
              className="w-32 px-3 py-2 rounded"
              style={{
                background: '#1E2329',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
            />
          </div>
        </div>

        {/* Trading Leverage (Exchange) */}
        <div className="mb-2">
          <p className="text-xs font-medium mb-2" style={{ color: '#F0B90B' }}>
            {t('tradingLeverage')}
          </p>
        </div>
        <div className="grid grid-cols-2 gap-4 mb-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('btcEthLeverage')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('btcEthLeverageDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.btc_eth_max_leverage ?? 5}
                onChange={(e) =>
                  updateField('btc_eth_max_leverage', parseInt(e.target.value))
                }
                disabled={disabled}
                min={1}
                max={20}
                className="flex-1 accent-yellow-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#F0B90B' }}
              >
                {config.btc_eth_max_leverage ?? 5}x
              </span>
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('altcoinLeverage')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('altcoinLeverageDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.altcoin_max_leverage ?? 5}
                onChange={(e) =>
                  updateField('altcoin_max_leverage', parseInt(e.target.value))
                }
                disabled={disabled}
                min={1}
                max={20}
                className="flex-1 accent-yellow-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#F0B90B' }}
              >
                {config.altcoin_max_leverage ?? 5}x
              </span>
            </div>
          </div>
        </div>

        {/* Absolute notional limits */}
        <div className="mb-2">
          <p className="text-xs font-medium" style={{ color: '#0ECB81' }}>
            {t('positionValueLimits')}
          </p>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #0ECB81' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('maxPositionSize')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('maxPositionSizeDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input type="number" value={config.max_position_size ?? 1000}
                onChange={(e) => updateField('max_position_size', Number(e.target.value))}
                disabled={disabled} min={1} step={10} className="w-32 px-3 py-2 rounded"
                style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }} />
              <span style={{ color: '#848E9C' }}>USDT</span>
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #0ECB81' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('maxTotalPositionSize')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('maxTotalPositionSizeDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input type="number" value={config.max_total_position_size ?? 3000}
                onChange={(e) => updateField('max_total_position_size', Number(e.target.value))}
                disabled={disabled} min={1} step={10} className="w-32 px-3 py-2 rounded"
                style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }} />
              <span style={{ color: '#848E9C' }}>USDT</span>
            </div>
          </div>
        </div>
      </div>

      {/* Risk Parameters */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <AlertTriangle className="w-5 h-5" style={{ color: '#F6465D' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('riskParameters')}
          </h3>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('minRiskReward')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('minRiskRewardDesc')}
            </p>
            <div className="flex items-center">
              <span style={{ color: '#848E9C' }}>1:</span>
              <input
                type="number"
                value={config.min_risk_reward_ratio ?? 3}
                onChange={(e) =>
                  updateField('min_risk_reward_ratio', parseFloat(e.target.value) || 3)
                }
                disabled={disabled}
                min={1}
                max={10}
                step={0.5}
                className="w-20 px-3 py-2 rounded ml-2"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
            </div>
          </div>

        </div>
      </div>

      <div>
        <div className="flex items-center gap-2 mb-4">
          <AlertTriangle className="w-5 h-5" style={{ color: '#F6465D' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {language === 'zh' ? '账户熔断（代码强制）' : 'Account Circuit Breakers (CODE ENFORCED)'}
          </h3>
        </div>
        <div className="grid grid-cols-2 gap-4">
          {([
            ['max_daily_loss_pct', language === 'zh' ? '日亏损上限 (%)' : 'Daily Loss Limit (%)', 5, 0.5],
            ['max_drawdown_pct', language === 'zh' ? '权益回撤上限 (%)' : 'Equity Drawdown Limit (%)', 10, 0.5],
            ['max_margin_usage_pct', language === 'zh' ? '保证金占用上限 (%)' : 'Margin Usage Limit (%)', 80, 1],
            ['min_liquidation_distance_pct', language === 'zh' ? '最小清算距离 (%)' : 'Min Liquidation Distance (%)', 5, 0.5],
            ['max_consecutive_failures', language === 'zh' ? '最大连续失败次数' : 'Max Consecutive Failures', 3, 1],
            ['max_market_move_pct', language === 'zh' ? '异常行情阈值 (%)' : 'Abnormal Market Move (%)', 8, 0.5],
            ['stop_trading_minutes', language === 'zh' ? '熔断暂停 (分钟)' : 'Circuit Breaker Pause (min)', 60, 1],
          ] as const).map(([key, label, fallback, step]) => (
            <div key={key} className="p-4 rounded-lg" style={{ background: '#0B0E11', border: '1px solid #F6465D' }}>
              <label className="block text-sm mb-2" style={{ color: '#EAECEF' }}>{label}</label>
              <input
                type="number"
                value={config[key] ?? fallback}
                onChange={(e) => updateField(key, Number(e.target.value))}
                disabled={disabled}
                min={step}
                step={step}
                className="w-28 px-3 py-2 rounded"
                style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
              />
            </div>
          ))}
        </div>
      </div>

      {/* Entry Requirements */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-5 h-5" style={{ color: '#0ECB81' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('entryRequirements')}
          </h3>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('minPositionSize')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('minPositionSizeDesc')}
            </p>
            <div className="flex items-center">
              <input
                type="number"
                value={config.min_position_size ?? 12}
                onChange={(e) =>
                  updateField('min_position_size', parseFloat(e.target.value) || 12)
                }
                disabled={disabled}
                min={10}
                max={1000}
                className="w-24 px-3 py-2 rounded"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
              <span className="ml-2" style={{ color: '#848E9C' }}>
                USDT
              </span>
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('minConfidence')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('minConfidenceDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.min_confidence ?? 75}
                onChange={(e) =>
                  updateField('min_confidence', parseInt(e.target.value))
                }
                disabled={disabled}
                min={50}
                max={100}
                className="flex-1 accent-green-500"
              />
              <span className="w-12 text-center font-mono" style={{ color: '#0ECB81' }}>
                {config.min_confidence ?? 75}
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Order Execution */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('orderExecution')}
          </h3>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('orderType')}
            </label>
            <p className="text-xs mb-3" style={{ color: '#848E9C' }}>
              {t('orderTypeDesc')}
            </p>
            <div className="grid grid-cols-2 gap-2">
              {(['market', 'limit'] as const).map((type) => {
                const active = (config.order_type ?? 'market') === type
                return (
                  <button
                    key={type}
                    type="button"
                    disabled={disabled}
                    onClick={() => updateField('order_type', type)}
                    className="px-3 py-2 rounded text-sm font-semibold disabled:opacity-50"
                    style={{
                      background: active ? '#F0B90B' : '#1E2329',
                      border: `1px solid ${active ? '#F0B90B' : '#2B3139'}`,
                      color: active ? '#000' : '#EAECEF',
                    }}
                  >
                    {type === 'market' ? t('marketOrder') : t('limitOrder')}
                  </button>
                )
              })}
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{
              background: '#0B0E11',
              border: `1px solid ${(config.order_type ?? 'market') === 'limit' ? '#F0B90B' : '#2B3139'}`,
            }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('limitOffset')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('limitOffsetDesc')}
            </p>
            <div className="flex items-center">
              <input
                type="number"
                value={config.limit_price_offset_pct ?? 0.05}
                onChange={(e) =>
                  updateField('limit_price_offset_pct', parseFloat(e.target.value) || 0.05)
                }
                disabled={disabled || (config.order_type ?? 'market') !== 'limit'}
                min={0.01}
                max={5}
                step={0.01}
                className="w-24 px-3 py-2 rounded disabled:opacity-50"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
              <span className="ml-2" style={{ color: '#848E9C' }}>
                %
              </span>
            </div>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-4 mt-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('pendingTimeout')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('pendingTimeoutDesc')}
            </p>
            <div className="flex items-center">
              <input
                type="number"
                value={Math.round((config.pending_order_timeout_sec ?? 1800) / 60)}
                onChange={(e) =>
                  updateField(
                    'pending_order_timeout_sec',
                    Math.max(1, Math.round((parseFloat(e.target.value) || 30) * 60))
                  )
                }
                disabled={disabled || (config.order_type ?? 'market') !== 'limit'}
                min={1}
                max={1440}
                step={1}
                className="w-24 px-3 py-2 rounded disabled:opacity-50"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
              <span className="ml-2" style={{ color: '#848E9C' }}>
                min
              </span>
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('pendingPoll')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('pendingPollDesc')}
            </p>
            <div className="flex items-center">
              <input
                type="number"
                value={config.pending_order_poll_sec ?? 2}
                onChange={(e) =>
                  updateField('pending_order_poll_sec', Math.max(1, Math.round(parseFloat(e.target.value) || 2)))
                }
                disabled={disabled || (config.order_type ?? 'market') !== 'limit'}
                min={1}
                max={60}
                step={1}
                className="w-24 px-3 py-2 rounded disabled:opacity-50"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
              <span className="ml-2" style={{ color: '#848E9C' }}>
                s
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
