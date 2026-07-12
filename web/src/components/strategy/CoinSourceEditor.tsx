import { useState } from 'react'
import { Plus, X } from 'lucide-react'
import type { CoinSourceConfig } from '../../types'

interface CoinSourceEditorProps {
  config: CoinSourceConfig
  onChange: (config: CoinSourceConfig) => void
  disabled?: boolean
  language: string
}

export function CoinSourceEditor({
  config,
  onChange,
  disabled,
  language,
}: CoinSourceEditorProps) {
  const [newCoin, setNewCoin] = useState('')

  const t = (key: string) => {
    const translations: Record<string, Record<string, string>> = {
      staticCoins: { zh: '自定义币种', en: 'Custom Coins' },
      addCoin: { zh: '添加币种', en: 'Add Coin' },
    }
    return translations[key]?.[language] || key
  }

  const handleAddCoin = () => {
    if (!newCoin.trim()) return
    const symbol = newCoin.toUpperCase().trim()
    const formattedSymbol = symbol.endsWith('USDT') ? symbol : `${symbol}USDT`
    const currentCoins = config.static_coins || []
    if (!currentCoins.includes(formattedSymbol)) {
      onChange({
        source_type: 'static',
        static_coins: [...currentCoins, formattedSymbol],
      })
    }
    setNewCoin('')
  }

  const handleRemoveCoin = (coin: string) => {
    onChange({
      source_type: 'static',
      static_coins: (config.static_coins || []).filter((c) => c !== coin),
    })
  }

  return (
    <div className="space-y-6">
      <div>
        <label className="block text-sm font-medium mb-3" style={{ color: '#EAECEF' }}>
          {t('staticCoins')}
        </label>
        <div className="flex flex-wrap gap-2 mb-3">
          {(config.static_coins || []).map((coin) => (
            <span
              key={coin}
              className="flex items-center gap-1 px-3 py-1.5 rounded-full text-sm"
              style={{ background: '#2B3139', color: '#EAECEF' }}
            >
              {coin}
              {!disabled && (
                <button
                  onClick={() => handleRemoveCoin(coin)}
                  className="ml-1 hover:text-red-400 transition-colors"
                >
                  <X className="w-3 h-3" />
                </button>
              )}
            </span>
          ))}
        </div>
        {!disabled && (
          <div className="flex gap-2">
            <input
              type="text"
              value={newCoin}
              onChange={(e) => setNewCoin(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleAddCoin()}
              placeholder="BTC, ETH, SOL..."
              className="flex-1 px-4 py-2 rounded-lg"
              style={{
                background: '#0B0E11',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
            />
            <button
              onClick={handleAddCoin}
              className="px-4 py-2 rounded-lg flex items-center gap-2 transition-colors"
              style={{ background: '#F0B90B', color: '#0B0E11' }}
            >
              <Plus className="w-4 h-4" />
              {t('addCoin')}
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
