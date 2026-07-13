import { useState, useEffect } from 'react'
import { EquityChart } from './EquityChart'
import { TradingViewChart } from './TradingViewChart'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { BarChart3, CandlestickChart } from 'lucide-react'
import { motion, AnimatePresence } from 'framer-motion'

interface ChartTabsProps {
  traderId: string
  selectedSymbol?: string // 从外部选择的币种
  updateKey?: number // 强制更新的 key
  exchangeId?: string // 交易所ID
}

type ChartTab = 'equity' | 'kline'

export function ChartTabs({
  traderId,
  selectedSymbol,
  updateKey,
  exchangeId,
}: ChartTabsProps) {
  const { language } = useLanguage()
  const [activeTab, setActiveTab] = useState<ChartTab>('equity')
  const [chartSymbol, setChartSymbol] = useState<string>('BTCUSDT')

  // 当从外部选择币种时，自动切换到K线图
  useEffect(() => {
    if (selectedSymbol) {
      console.log(
        '[ChartTabs] 收到币种选择:',
        selectedSymbol,
        'updateKey:',
        updateKey
      )
      setChartSymbol(selectedSymbol)
      setActiveTab('kline')
    }
  }, [selectedSymbol, updateKey])

  return (
    <div className="binance-card dashboard-chart">
      {/* Tab Headers - 这是Tab切换按钮区域 */}
      <div className="dashboard-chart__tabs">
        <button
          onClick={() => {
            setActiveTab('equity')
          }}
          className={`dashboard-chart__tab ${activeTab === 'equity' ? 'is-active' : ''}`}
        >
          <BarChart3 className="w-4 h-4" />
          {t('accountEquityCurve', language)}
        </button>

        <button
          onClick={() => {
            setActiveTab('kline')
          }}
          className={`dashboard-chart__tab ${activeTab === 'kline' ? 'is-active' : ''}`}
        >
          <CandlestickChart className="w-4 h-4" />
          {t('marketChart', language)}
        </button>
      </div>

      {/* Tab Content */}
      <div className="relative overflow-hidden min-h-[400px]">
        <AnimatePresence mode="wait">
          {activeTab === 'equity' ? (
            <motion.div
              key="equity"
              initial={{ opacity: 0, x: -20 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: 20 }}
              transition={{ duration: 0.2 }}
              className="h-full"
            >
              <EquityChart traderId={traderId} embedded />
            </motion.div>
          ) : (
            <motion.div
              key={`kline-${chartSymbol}-${exchangeId}`}
              initial={{ opacity: 0, x: 20 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: -20 }}
              transition={{ duration: 0.2 }}
              className="h-full"
            >
              <TradingViewChart
                height={400}
                embedded
                defaultSymbol={chartSymbol}
                defaultExchange={exchangeId}
                key={`${chartSymbol}-${exchangeId}-${updateKey || ''}`}
              />
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </div>
  )
}
