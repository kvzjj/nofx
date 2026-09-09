import React, { useState, useEffect } from 'react'
import type { Exchange } from '../../types'
import { t, type Language } from '../../i18n/translations'
import { api } from '../../lib/api'
import { getExchangeIcon } from '../ExchangeIcons'
import {
  WebCryptoEnvironmentCheck,
  type WebCryptoCheckStatus,
} from '../WebCryptoEnvironmentCheck'
import { BookOpen, Trash2, ExternalLink, UserPlus } from 'lucide-react'
import { toast } from 'sonner'
import { getShortName } from './utils'

// Supported exchange templates for creating new accounts
const SUPPORTED_EXCHANGE_TEMPLATES = [
  { exchange_type: 'binance', name: 'Binance Futures', type: 'cex' as const },
  {
    exchange_type: 'paper',
    name: 'Paper Trading (Simulated)',
    type: 'sim' as const,
  },
]

interface ExchangeConfigModalProps {
  allExchanges: Exchange[]
  editingExchangeId: string | null
  onSave: (
    exchangeId: string | null, // null for creating new account
    exchangeType: string,
    accountName: string,
    apiKey: string,
    secretKey?: string,
    passphrase?: string, // OKX专用
    testnet?: boolean,
    hyperliquidWalletAddr?: string,
    asterUser?: string,
    asterSigner?: string,
    asterPrivateKey?: string,
    lighterWalletAddr?: string,
    lighterPrivateKey?: string,
    lighterApiKeyPrivateKey?: string
  ) => Promise<void>
  onDelete: (exchangeId: string) => void
  onClose: () => void
  language: Language
}

export function ExchangeConfigModal({
  allExchanges,
  editingExchangeId,
  onSave,
  onDelete,
  onClose,
  language,
}: ExchangeConfigModalProps) {
  // Selected exchange type for creating new accounts
  const [selectedExchangeType, setSelectedExchangeType] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [secretKey, setSecretKey] = useState('')
  const [testnet, setTestnet] = useState(false)
  const [showGuide, setShowGuide] = useState(false)
  const [serverIP, setServerIP] = useState<{
    public_ip: string
    message: string
  } | null>(null)
  const [loadingIP, setLoadingIP] = useState(false)
  const [copiedIP, setCopiedIP] = useState(false)
  const [webCryptoStatus, setWebCryptoStatus] =
    useState<WebCryptoCheckStatus>('idle')

  // 币安配置指南展开状态
  const [showBinanceGuide, setShowBinanceGuide] = useState(false)

  // 保存中状态
  const [isSaving, setIsSaving] = useState(false)

  // 账户名称
  const [accountName, setAccountName] = useState('')

  // 获取当前编辑的交易所信息或模板
  // For editing: find the existing account by id (UUID)
  // For creating: use the selected exchange template
  const selectedExchange = editingExchangeId
    ? allExchanges?.find((e) => e.id === editingExchangeId)
    : null

  // Get the exchange template for displaying UI fields
  const selectedTemplate = editingExchangeId
    ? SUPPORTED_EXCHANGE_TEMPLATES.find((t) => t.exchange_type === selectedExchange?.exchange_type)
    : SUPPORTED_EXCHANGE_TEMPLATES.find((t) => t.exchange_type === selectedExchangeType)

  // Get the current exchange type (from existing account or selected template)
  const currentExchangeType = editingExchangeId
    ? selectedExchange?.exchange_type
    : selectedExchangeType
  const supportsTestnet = currentExchangeType === 'binance'
  const testnetDescription =
    language === 'zh'
      ? '启用后使用 Binance Futures Testnet，请填写测试网 API Key'
      : 'Uses Binance Futures Testnet. Enter testnet API keys.'

  // 交易所注册链接配置
  const exchangeRegistrationLinks: Record<string, { url: string; hasReferral?: boolean }> = {
    binance: { url: 'https://www.binance.com/join?ref=AUAIEXENG', hasReferral: true },
  }

  // 如果是编辑现有交易所，初始化表单数据
  useEffect(() => {
    if (editingExchangeId && selectedExchange) {
      setAccountName(selectedExchange.account_name || '')
      setApiKey(selectedExchange.apiKey || '')
      setSecretKey(selectedExchange.secretKey || '')
      setTestnet(selectedExchange.testnet || false)
    }
  }, [editingExchangeId, selectedExchange])

  // 加载服务器IP（当选择binance时）
  useEffect(() => {
    if (currentExchangeType === 'binance' && !serverIP) {
      setLoadingIP(true)
      api
        .getServerIP()
        .then((data) => {
          setServerIP(data)
        })
        .catch((err) => {
          console.error('Failed to load server IP:', err)
        })
        .finally(() => {
          setLoadingIP(false)
        })
    }
  }, [currentExchangeType])

  const handleCopyIP = async (ip: string) => {
    try {
      // 优先使用现代 Clipboard API
      if (navigator.clipboard && navigator.clipboard.writeText) {
        await navigator.clipboard.writeText(ip)
        setCopiedIP(true)
        setTimeout(() => setCopiedIP(false), 2000)
        toast.success(t('ipCopied', language))
      } else {
        // 降级方案: 使用传统的 execCommand 方法
        const textArea = document.createElement('textarea')
        textArea.value = ip
        textArea.style.position = 'fixed'
        textArea.style.left = '-999999px'
        textArea.style.top = '-999999px'
        document.body.appendChild(textArea)
        textArea.focus()
        textArea.select()

        try {
          const successful = document.execCommand('copy')
          if (successful) {
            setCopiedIP(true)
            setTimeout(() => setCopiedIP(false), 2000)
            toast.success(t('ipCopied', language))
          } else {
            throw new Error('复制命令执行失败')
          }
        } finally {
          document.body.removeChild(textArea)
        }
      }
    } catch (err) {
      console.error('复制失败:', err)
      // 显示错误提示
      toast.error(
        t('copyIPFailed', language) || `复制失败: ${ip}\n请手动复制此IP地址`
      )
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (isSaving) return

    // For creating, we need the exchange type
    if (!editingExchangeId && !selectedExchangeType) return

    // Validate account name
    const trimmedAccountName = accountName.trim()
    if (!trimmedAccountName) {
      toast.error(language === 'zh' ? '请输入账户名称' : 'Please enter account name')
      return
    }

    const exchangeId = editingExchangeId || null
    const exchangeType = currentExchangeType || ''
    const isPaper = exchangeType === 'paper'

    setIsSaving(true)
    try {
      // Paper trading needs no credentials; every other exchange does.
      if (!isPaper && (!apiKey.trim() || !secretKey.trim())) return
      await onSave(
        exchangeId,
        exchangeType,
        trimmedAccountName,
        isPaper ? '' : apiKey.trim(),
        isPaper ? '' : secretKey.trim(),
        '',
        isPaper ? false : testnet
      )
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4 overflow-y-auto">
      <div
        className="bg-gray-800 rounded-lg w-full max-w-lg relative my-8"
        style={{
          background: '#1E2329',
          maxHeight: 'calc(100vh - 4rem)',
        }}
      >
        <div
          className="flex items-center justify-between p-6 pb-4 sticky top-0 z-10"
          style={{ background: '#1E2329' }}
        >
          <h3 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
            {editingExchangeId
              ? t('editExchange', language)
              : t('addExchange', language)}
          </h3>
          <div className="flex items-center gap-2">
            {currentExchangeType === 'binance' && (
              <button
                type="button"
                onClick={() => setShowGuide(true)}
                className="px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105 flex items-center gap-2"
                style={{
                  background: 'rgba(240, 185, 11, 0.1)',
                  color: '#F0B90B',
                }}
              >
                <BookOpen className="w-4 h-4" />
                {t('viewGuide', language)}
              </button>
            )}
            {editingExchangeId && (
              <button
                type="button"
                onClick={() => onDelete(editingExchangeId)}
                className="p-2 rounded hover:bg-red-100 transition-colors"
                style={{
                  background: 'rgba(246, 70, 93, 0.1)',
                  color: '#F6465D',
                }}
                title={t('delete', language)}
              >
                <Trash2 className="w-4 h-4" />
              </button>
            )}
          </div>
        </div>

        <form onSubmit={handleSubmit} className="px-6 pb-6">
          <div
            className="space-y-4 overflow-y-auto"
            style={{ maxHeight: 'calc(100vh - 16rem)' }}
          >
            {!editingExchangeId && (
              <div className="space-y-3">
                <div className="space-y-2">
                  <div
                    className="text-xs font-semibold uppercase tracking-wide"
                    style={{ color: '#F0B90B' }}
                  >
                    {t('environmentSteps.checkTitle', language)}
                  </div>
                  <WebCryptoEnvironmentCheck
                    language={language}
                    variant="card"
                    onStatusChange={setWebCryptoStatus}
                  />
                </div>
                <div className="space-y-2">
                  <div
                    className="text-xs font-semibold uppercase tracking-wide"
                    style={{ color: '#F0B90B' }}
                  >
                    {t('environmentSteps.selectTitle', language)}
                  </div>
                  <select
                    value={selectedExchangeType}
                    onChange={(e) => setSelectedExchangeType(e.target.value)}
                    className="w-full px-3 py-2 rounded"
                    style={{
                      background: '#0B0E11',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                    }}
                    aria-label={t('selectExchange', language)}
                    disabled={
                      webCryptoStatus !== 'secure' &&
                      webCryptoStatus !== 'disabled'
                    }
                    required
                  >
                    <option value="">
                      {t('pleaseSelectExchange', language)}
                    </option>
                    {SUPPORTED_EXCHANGE_TEMPLATES.map((template) => (
                      <option key={template.exchange_type} value={template.exchange_type}>
                        {getShortName(template.name)} (
                        {template.type.toUpperCase()})
                      </option>
                    ))}
                  </select>
                </div>
              </div>
            )}

            {selectedTemplate && (
              <div
                className="p-4 rounded"
                style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
              >
                <div className="flex items-center gap-3 mb-3">
                  <div className="w-8 h-8 flex items-center justify-center">
                    {getExchangeIcon(selectedTemplate.exchange_type, {
                      width: 32,
                      height: 32,
                    })}
                  </div>
                  <div>
                    <div className="font-semibold" style={{ color: '#EAECEF' }}>
                      {getShortName(selectedTemplate.name)}
                      {editingExchangeId && selectedExchange?.account_name && (
                        <span className="text-sm font-normal ml-2" style={{ color: '#848E9C' }}>
                          - {selectedExchange.account_name}
                        </span>
                      )}
                    </div>
                    <div className="text-xs" style={{ color: '#848E9C' }}>
                      {selectedTemplate.type.toUpperCase()} •{' '}
                      {selectedTemplate.exchange_type}
                    </div>
                  </div>
                </div>

                {/* 账户名称输入 */}
                <div className="mt-3">
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: '#EAECEF' }}
                  >
                    {language === 'zh' ? '账户名称' : 'Account Name'} *
                  </label>
                  <input
                    type="text"
                    value={accountName}
                    onChange={(e) => setAccountName(e.target.value)}
                    placeholder={language === 'zh' ? '例如：主账户、套利账户' : 'e.g., Main Account, Arbitrage Account'}
                    className="w-full px-3 py-2 rounded"
                    style={{
                      background: '#1E2329',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                    }}
                    required
                  />
                  <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                    {language === 'zh'
                      ? '为此账户设置一个易于识别的名称，以便区分同一交易所的多个账户'
                      : 'Set an easily recognizable name for this account to distinguish multiple accounts on the same exchange'}
                  </div>
                </div>

                {/* 注册链接 */}
                <a
                  href={exchangeRegistrationLinks[currentExchangeType || '']?.url || '#'}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center justify-between p-3 rounded-lg mt-3 transition-all hover:scale-[1.02]"
                  style={{
                    background: 'rgba(240, 185, 11, 0.08)',
                    border: '1px solid rgba(240, 185, 11, 0.2)',
                  }}
                >
                  <div className="flex items-center gap-2">
                    <UserPlus className="w-4 h-4" style={{ color: '#F0B90B' }} />
                    <span className="text-sm" style={{ color: '#EAECEF' }}>
                      {language === 'zh' ? '还没有交易所账号？点击注册' : "No exchange account? Register here"}
                    </span>
                    {exchangeRegistrationLinks[currentExchangeType || '']?.hasReferral && (
                      <span
                        className="text-xs px-1.5 py-0.5 rounded"
                        style={{ background: 'rgba(14, 203, 129, 0.2)', color: '#0ECB81' }}
                      >
                        {language === 'zh' ? '折扣优惠' : 'Discount'}
                      </span>
                    )}
                  </div>
                  <ExternalLink className="w-4 h-4" style={{ color: '#848E9C' }} />
                </a>
              </div>
            )}

            {selectedTemplate && (
              <>
                {supportsTestnet && (
                  <div
                    className="p-4 rounded"
                    style={{
                      background: testnet ? 'rgba(240, 185, 11, 0.10)' : '#0B0E11',
                      border: `1px solid ${testnet ? 'rgba(240, 185, 11, 0.35)' : '#2B3139'}`,
                    }}
                  >
                    <label className="flex items-center justify-between gap-4 cursor-pointer">
                      <div>
                        <div className="text-sm font-semibold" style={{ color: '#EAECEF' }}>
                          {language === 'zh' ? '测试网环境' : 'Testnet Environment'}
                        </div>
                        <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                          {testnetDescription}
                        </div>
                      </div>
                      <input
                        type="checkbox"
                        checked={testnet}
                        onChange={(e) => setTestnet(e.target.checked)}
                        className="w-5 h-5 accent-yellow-500"
                      />
                    </label>
                  </div>
                )}

                {/* 币安的输入字段 */}
                {currentExchangeType !== 'paper' && (
                    <>
                      {/* 币安用户配置提示 (D1 方案) */}
                      {currentExchangeType === 'binance' && (
                        <div
                          className="mb-4 p-3 rounded cursor-pointer transition-colors"
                          style={{
                            background: '#1a3a52',
                            border: '1px solid #2b5278',
                          }}
                          onClick={() => setShowBinanceGuide(!showBinanceGuide)}
                        >
                          <div className="flex items-center justify-between">
                            <div className="flex items-center gap-2">
                              <span style={{ color: '#58a6ff' }}>ℹ️</span>
                              <span
                                className="text-sm font-medium"
                                style={{ color: '#EAECEF' }}
                              >
                                <strong>币安用户必读：</strong>
                                使用「现货与合约交易」API，不要用「统一账户
                                API」
                              </span>
                            </div>
                            <span style={{ color: '#8b949e' }}>
                              {showBinanceGuide ? '▲' : '▼'}
                            </span>
                          </div>

                          {/* 展开的详细说明 */}
                          {showBinanceGuide && (
                            <div
                              className="mt-3 pt-3"
                              style={{
                                borderTop: '1px solid #2b5278',
                                fontSize: '0.875rem',
                                color: '#c9d1d9',
                              }}
                              onClick={(e) => e.stopPropagation()}
                            >
                              <p className="mb-2" style={{ color: '#8b949e' }}>
                                <strong>原因：</strong>统一账户 API
                                权限结构不同，会导致订单提交失败
                              </p>

                              <p
                                className="font-semibold mb-1"
                                style={{ color: '#EAECEF' }}
                              >
                                正确配置步骤：
                              </p>
                              <ol
                                className="list-decimal list-inside space-y-1 mb-3"
                                style={{ paddingLeft: '0.5rem' }}
                              >
                                <li>
                                  登录币安 → 个人中心 →{' '}
                                  <strong>API 管理</strong>
                                </li>
                                <li>
                                  创建 API → 选择「
                                  <strong>系统生成的 API 密钥</strong>」
                                </li>
                                <li>
                                  勾选「<strong>现货与合约交易</strong>」（
                                  <span style={{ color: '#f85149' }}>
                                    不选统一账户
                                  </span>
                                  ）
                                </li>
                                <li>
                                  IP 限制选「<strong>无限制</strong>
                                  」或添加服务器 IP
                                </li>
                              </ol>

                              <p
                                className="mb-2 p-2 rounded"
                                style={{
                                  background: '#3d2a00',
                                  border: '1px solid #9e6a03',
                                }}
                              >
                                💡 <strong>多资产模式用户注意：</strong>
                                如果您开启了多资产模式，将强制使用全仓模式。建议关闭多资产模式以支持逐仓交易。
                              </p>

                              <a
                                href="https://www.binance.com/zh-CN/support/faq/how-to-create-api-keys-on-binance-360002502072"
                                target="_blank"
                                rel="noopener noreferrer"
                                className="inline-block text-sm hover:underline"
                                style={{ color: '#58a6ff' }}
                              >
                                📖 查看币安官方教程 ↗
                              </a>
                            </div>
                          )}
                        </div>
                      )}

                      <div>
                        <label
                          className="block text-sm font-semibold mb-2"
                          style={{ color: '#EAECEF' }}
                        >
                          {t('apiKey', language)}
                        </label>
                        <input
                          type="password"
                          value={apiKey}
                          onChange={(e) => setApiKey(e.target.value)}
                          placeholder={t('enterAPIKey', language)}
                          className="w-full px-3 py-2 rounded"
                          style={{
                            background: '#0B0E11',
                            border: '1px solid #2B3139',
                            color: '#EAECEF',
                          }}
                          required
                        />
                      </div>

                      <div>
                        <label
                          className="block text-sm font-semibold mb-2"
                          style={{ color: '#EAECEF' }}
                        >
                          {t('secretKey', language)}
                        </label>
                        <input
                          type="password"
                          value={secretKey}
                          onChange={(e) => setSecretKey(e.target.value)}
                          placeholder={t('enterSecretKey', language)}
                          className="w-full px-3 py-2 rounded"
                          style={{
                            background: '#0B0E11',
                            border: '1px solid #2B3139',
                            color: '#EAECEF',
                          }}
                          required
                        />
                      </div>


                      {currentExchangeType === 'paper' && (
                        <div
                          className="p-4 rounded"
                          style={{
                            background: 'rgba(14, 203, 129, 0.08)',
                            border: '1px solid rgba(14, 203, 129, 0.25)',
                          }}
                        >
                          <div
                            className="text-sm font-semibold mb-2"
                            style={{ color: '#0ECB81' }}
                          >
                            {language === 'zh'
                              ? '📒 模拟盘：零风险验证策略'
                              : '📒 Paper Trading: validate strategies risk-free'}
                          </div>
                          <ul
                            className="text-xs space-y-1"
                            style={{ color: '#848E9C' }}
                          >
                            <li>
                              {language === 'zh'
                                ? '• 使用真实行情价格本地模拟撮合，不需要 API Key，不发生真实下单'
                                : '• Real market prices with locally simulated fills; no API keys, no real orders'}
                            </li>
                            <li>
                              {language === 'zh'
                                ? '• 市价单按当前价 ± 滑点成交，限价单/止损/止盈由后台撮合器触发'
                                : '• Market orders fill at live price ± slippage; limit/SL/TP orders are triggered by the background matcher'}
                            </li>
                            <li>
                              {language === 'zh'
                                ? '• 初始资金由关联 Trader 的「初始余额」决定，模拟账户状态重启后保留'
                                : '• Initial capital comes from the linked trader’s initial balance; simulated state survives restarts'}
                            </li>
                            <li>
                              {language === 'zh'
                                ? '• 资金费率未模拟，长期持仓的模拟盈亏略偏乐观'
                                : '• Funding rates are not simulated; long-held positions read slightly optimistic'}
                            </li>
                          </ul>
                        </div>
                      )}

                      {/* Binance 白名单IP提示 */}
                      {currentExchangeType === 'binance' && (
                        <div
                          className="p-4 rounded"
                          style={{
                            background: 'rgba(240, 185, 11, 0.1)',
                            border: '1px solid rgba(240, 185, 11, 0.2)',
                          }}
                        >
                          <div
                            className="text-sm font-semibold mb-2"
                            style={{ color: '#F0B90B' }}
                          >
                            {t('whitelistIP', language)}
                          </div>
                          <div
                            className="text-xs mb-3"
                            style={{ color: '#848E9C' }}
                          >
                            {t('whitelistIPDesc', language)}
                          </div>

                          {loadingIP ? (
                            <div
                              className="text-xs"
                              style={{ color: '#848E9C' }}
                            >
                              {t('loadingServerIP', language)}
                            </div>
                          ) : serverIP && serverIP.public_ip ? (
                            <div
                              className="flex items-center gap-2 p-2 rounded"
                              style={{ background: '#0B0E11' }}
                            >
                              <code
                                className="flex-1 text-sm font-mono"
                                style={{ color: '#F0B90B' }}
                              >
                                {serverIP.public_ip}
                              </code>
                              <button
                                type="button"
                                onClick={() => handleCopyIP(serverIP.public_ip)}
                                className="px-3 py-1 rounded text-xs font-semibold transition-all hover:scale-105"
                                style={{
                                  background: 'rgba(240, 185, 11, 0.2)',
                                  color: '#F0B90B',
                                }}
                              >
                                {copiedIP
                                  ? t('ipCopied', language)
                                  : t('copyIP', language)}
                              </button>
                            </div>
                          ) : null}
                        </div>
                      )}
                    </>
                  )}



              </>
            )}
          </div>

          <div
            className="flex gap-3 mt-6 pt-4 sticky bottom-0"
            style={{ background: '#1E2329' }}
          >
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#2B3139', color: '#848E9C' }}
            >
              {t('cancel', language)}
            </button>
            <button
              type="submit"
              disabled={
                isSaving ||
                !selectedTemplate ||
                !accountName.trim() ||
                !apiKey.trim() ||
                !secretKey.trim()
              }
              className="flex-1 px-4 py-2 rounded text-sm font-semibold disabled:opacity-50"
              style={{ background: '#F0B90B', color: '#000' }}
            >
              {isSaving ? t('saving', language) || '保存中...' : t('saveConfig', language)}
            </button>
          </div>
        </form>
      </div>

      {/* Binance Setup Guide Modal */}
      {showGuide && (
        <div
          className="fixed inset-0 bg-black bg-opacity-75 flex items-center justify-center z-50 p-4"
          onClick={() => setShowGuide(false)}
        >
          <div
            className="bg-gray-800 rounded-lg p-6 w-full max-w-4xl relative"
            style={{ background: '#1E2329' }}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between mb-4">
              <h3
                className="text-xl font-bold flex items-center gap-2"
                style={{ color: '#EAECEF' }}
              >
                <BookOpen className="w-6 h-6" style={{ color: '#F0B90B' }} />
                {t('binanceSetupGuide', language)}
              </h3>
              <button
                onClick={() => setShowGuide(false)}
                className="px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
                style={{ background: '#2B3139', color: '#848E9C' }}
              >
                {t('closeGuide', language)}
              </button>
            </div>
            <div className="overflow-y-auto max-h-[80vh]">
              <img
                src="/images/guide.png"
                alt={t('binanceSetupGuide', language)}
                className="w-full h-auto rounded"
              />
            </div>
          </div>
        </div>
      )}

    </div>
  )
}
