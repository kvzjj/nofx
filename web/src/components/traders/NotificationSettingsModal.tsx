import { useState, useEffect } from 'react'
import { Bell, Send, X } from 'lucide-react'
import { t, type Language } from '../../i18n/translations'
import { api } from '../../lib/api'
import type { NotificationSettings } from '../../types'
import { toast } from 'sonner'

// Masked placeholder returned by the API for stored secrets
const MASKED = '••••••••'

// Event types offered for per-event subscription toggles
const EVENT_TYPES: Array<{ key: string; labelZh: string; labelEn: string }> = [
  { key: 'position.opened', labelZh: '开仓', labelEn: 'Position opened' },
  { key: 'position.closed', labelZh: '平仓', labelEn: 'Position closed' },
  {
    key: 'position.partial_closed',
    labelZh: '部分平仓',
    labelEn: 'Partial close',
  },
  {
    key: 'position.take_profit',
    labelZh: '止盈保护触发',
    labelEn: 'TP protection',
  },
  {
    key: 'position.drawdown_alert',
    labelZh: '回撤告警',
    labelEn: 'Drawdown alert',
  },
  {
    key: 'risk.control_triggered',
    labelZh: '风控触发',
    labelEn: 'Risk control',
  },
  { key: 'trader.stopped', labelZh: '交易员停止', labelEn: 'Trader stopped' },
  { key: 'trader.error', labelZh: '交易员异常', labelEn: 'Trader error' },
  {
    key: 'report.daily_summary',
    labelZh: '每日摘要',
    labelEn: 'Daily summary',
  },
  { key: 'system.notice', labelZh: '系统通知', labelEn: 'System notice' },
]

interface NotificationSettingsModalProps {
  onClose: () => void
  language: Language
}

export function NotificationSettingsModal({
  onClose,
  language,
}: NotificationSettingsModalProps) {
  const [settings, setSettings] = useState<NotificationSettings | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)

  useEffect(() => {
    let cancelled = false
    api
      .getNotificationSettings()
      .then((s) => {
        if (!cancelled) setSettings(s)
      })
      .catch(() => toast.error(t('notifications.loadFailed', language)))
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [language])

  const update = (patch: Partial<NotificationSettings>) => {
    setSettings((prev) => (prev ? { ...prev, ...patch } : prev))
  }

  const updateEventSub = (key: string, enabled: boolean) => {
    setSettings((prev) =>
      prev
        ? {
            ...prev,
            event_subscriptions: {
              ...(prev.event_subscriptions || {}),
              [key]: enabled,
            },
          }
        : prev
    )
  }

  const handleSave = async () => {
    if (!settings) return
    setSaving(true)
    try {
      const saved = await api.updateNotificationSettings(settings)
      setSettings(saved)
      toast.success(t('notifications.saved', language))
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : t('notifications.saveFailed', language)
      )
    } finally {
      setSaving(false)
    }
  }

  const handleTest = async () => {
    // Save first so the test uses the latest configuration.
    if (!settings) return
    setTesting(true)
    try {
      await api.updateNotificationSettings(settings)
      await api.testNotification()
      toast.success(t('notifications.testSent', language))
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : t('notifications.testFailed', language)
      )
    } finally {
      setTesting(false)
    }
  }

  const labelStyle = { color: '#EAECEF' }
  const inputStyle = {
    background: '#0B0E11',
    border: '1px solid #2B3139',
    color: '#EAECEF',
  }

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4 overflow-y-auto">
      <div
        className="rounded-lg w-full max-w-2xl relative my-8"
        style={{ background: '#1E2329', maxHeight: 'calc(100vh - 4rem)' }}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-6 pt-6 pb-2">
          <h3
            className="text-xl font-bold flex items-center gap-2"
            style={labelStyle}
          >
            <Bell className="w-5 h-5" style={{ color: '#F0B90B' }} />
            {t('notifications.title', language)}
          </h3>
          <button onClick={onClose} className="p-1 rounded hover:bg-white/10">
            <X className="w-5 h-5" style={{ color: '#848E9C' }} />
          </button>
        </div>

        {loading || !settings ? (
          <div className="px-6 pb-6" style={{ color: '#848E9C' }}>
            {t('notifications.loading', language)}
          </div>
        ) : (
          <div
            className="px-6 pb-6 space-y-5 overflow-y-auto"
            style={{ maxHeight: 'calc(100vh - 12rem)' }}
          >
            {/* Master switch */}
            <label className="flex items-center justify-between cursor-pointer">
              <span className="text-sm font-semibold" style={labelStyle}>
                {t('notifications.master', language)}
              </span>
              <input
                type="checkbox"
                checked={settings.enabled}
                onChange={(e) => update({ enabled: e.target.checked })}
                className="w-5 h-5 accent-yellow-500"
              />
            </label>

            {/* Telegram */}
            <section
              className="rounded p-4 space-y-3"
              style={{ background: '#0B0E11' }}
            >
              <label className="flex items-center justify-between cursor-pointer">
                <span className="text-sm font-semibold" style={labelStyle}>
                  Telegram
                </span>
                <input
                  type="checkbox"
                  checked={settings.telegram_enabled}
                  onChange={(e) =>
                    update({ telegram_enabled: e.target.checked })
                  }
                  className="w-5 h-5 accent-yellow-500"
                />
              </label>
              {settings.telegram_enabled && (
                <>
                  <input
                    type="text"
                    value={settings.telegram_bot_token || ''}
                    onChange={(e) =>
                      update({ telegram_bot_token: e.target.value })
                    }
                    placeholder={
                      language === 'zh'
                        ? 'Bot Token (123:ABC...)'
                        : 'Bot Token (123:ABC...)'
                    }
                    className="w-full px-3 py-2 rounded text-sm"
                    style={inputStyle}
                  />
                  <input
                    type="text"
                    value={settings.telegram_chat_id || ''}
                    onChange={(e) =>
                      update({ telegram_chat_id: e.target.value })
                    }
                    placeholder={
                      language === 'zh'
                        ? 'Chat ID (-100...)'
                        : 'Chat ID (-100...)'
                    }
                    className="w-full px-3 py-2 rounded text-sm"
                    style={inputStyle}
                  />
                </>
              )}
            </section>

            {/* Webhook */}
            <section
              className="rounded p-4 space-y-3"
              style={{ background: '#0B0E11' }}
            >
              <label className="flex items-center justify-between cursor-pointer">
                <span className="text-sm font-semibold" style={labelStyle}>
                  Webhook
                </span>
                <input
                  type="checkbox"
                  checked={settings.webhook_enabled}
                  onChange={(e) =>
                    update({ webhook_enabled: e.target.checked })
                  }
                  className="w-5 h-5 accent-yellow-500"
                />
              </label>
              {settings.webhook_enabled && (
                <>
                  <input
                    type="url"
                    value={settings.webhook_url || ''}
                    onChange={(e) => update({ webhook_url: e.target.value })}
                    placeholder="https://example.com/hook"
                    className="w-full px-3 py-2 rounded text-sm"
                    style={inputStyle}
                  />
                  <input
                    type="text"
                    value={settings.webhook_secret || ''}
                    onChange={(e) => update({ webhook_secret: e.target.value })}
                    placeholder={
                      language === 'zh'
                        ? '签名密钥（可选）'
                        : 'Signing secret (optional)'
                    }
                    className="w-full px-3 py-2 rounded text-sm"
                    style={inputStyle}
                  />
                  <p className="text-xs" style={{ color: '#848E9C' }}>
                    {language === 'zh'
                      ? 'POST JSON，附带 X-AUAIEX-Signature: sha256=HMAC 头'
                      : 'POST JSON with X-AUAIEX-Signature: sha256=HMAC header'}
                  </p>
                </>
              )}
            </section>

            {/* Email */}
            <section
              className="rounded p-4 space-y-3"
              style={{ background: '#0B0E11' }}
            >
              <label className="flex items-center justify-between cursor-pointer">
                <span className="text-sm font-semibold" style={labelStyle}>
                  Email (SMTP)
                </span>
                <input
                  type="checkbox"
                  checked={settings.email_enabled}
                  onChange={(e) => update({ email_enabled: e.target.checked })}
                  className="w-5 h-5 accent-yellow-500"
                />
              </label>
              {settings.email_enabled && (
                <div className="grid grid-cols-2 gap-3">
                  <input
                    type="email"
                    value={settings.email_to || ''}
                    onChange={(e) => update({ email_to: e.target.value })}
                    placeholder={language === 'zh' ? '收件邮箱' : 'Recipient'}
                    className="col-span-2 px-3 py-2 rounded text-sm"
                    style={inputStyle}
                  />
                  <input
                    type="text"
                    value={settings.smtp_host || ''}
                    onChange={(e) => update({ smtp_host: e.target.value })}
                    placeholder="SMTP Host"
                    className="px-3 py-2 rounded text-sm"
                    style={inputStyle}
                  />
                  <input
                    type="number"
                    value={settings.smtp_port || 587}
                    onChange={(e) =>
                      update({ smtp_port: Number(e.target.value) })
                    }
                    placeholder="Port"
                    className="px-3 py-2 rounded text-sm"
                    style={inputStyle}
                  />
                  <input
                    type="text"
                    value={settings.smtp_username || ''}
                    onChange={(e) => update({ smtp_username: e.target.value })}
                    placeholder="Username"
                    className="px-3 py-2 rounded text-sm"
                    style={inputStyle}
                  />
                  <input
                    type="password"
                    value={settings.smtp_password || ''}
                    onChange={(e) => update({ smtp_password: e.target.value })}
                    placeholder={
                      settings.smtp_password === MASKED
                        ? 'Password'
                        : 'Password'
                    }
                    className="px-3 py-2 rounded text-sm"
                    style={inputStyle}
                  />
                  <input
                    type="text"
                    value={settings.smtp_from || ''}
                    onChange={(e) => update({ smtp_from: e.target.value })}
                    placeholder="From (noreply@...)"
                    className="col-span-2 px-3 py-2 rounded text-sm"
                    style={inputStyle}
                  />
                </div>
              )}
            </section>

            {/* Quiet hours */}
            <section
              className="rounded p-4 space-y-2"
              style={{ background: '#0B0E11' }}
            >
              <span className="text-sm font-semibold" style={labelStyle}>
                {t('notifications.quietHours', language)}
              </span>
              <div className="flex items-center gap-3">
                <input
                  type="number"
                  min={-1}
                  max={23}
                  value={settings.quiet_hours_start_utc ?? -1}
                  onChange={(e) =>
                    update({ quiet_hours_start_utc: Number(e.target.value) })
                  }
                  className="w-20 px-3 py-2 rounded text-sm"
                  style={inputStyle}
                />
                <span style={{ color: '#848E9C' }}>→</span>
                <input
                  type="number"
                  min={-1}
                  max={23}
                  value={settings.quiet_hours_end_utc ?? -1}
                  onChange={(e) =>
                    update({ quiet_hours_end_utc: Number(e.target.value) })
                  }
                  className="w-20 px-3 py-2 rounded text-sm"
                  style={inputStyle}
                />
                <span className="text-xs" style={{ color: '#848E9C' }}>
                  {t('notifications.quietHoursHint', language)}
                </span>
              </div>
            </section>

            {/* Event subscriptions */}
            <section
              className="rounded p-4 space-y-2"
              style={{ background: '#0B0E11' }}
            >
              <span className="text-sm font-semibold" style={labelStyle}>
                {t('notifications.events', language)}
              </span>
              <div className="grid grid-cols-2 gap-2">
                {EVENT_TYPES.map((ev) => {
                  // absent = enabled (default)
                  const disabled =
                    settings.event_subscriptions?.[ev.key] === false
                  return (
                    <label
                      key={ev.key}
                      className="flex items-center gap-2 text-xs cursor-pointer"
                      style={{ color: '#B7BDC6' }}
                    >
                      <input
                        type="checkbox"
                        checked={!disabled}
                        onChange={(e) =>
                          updateEventSub(ev.key, e.target.checked)
                        }
                        className="w-4 h-4 accent-yellow-500"
                      />
                      {language === 'zh' ? ev.labelZh : ev.labelEn}
                    </label>
                  )
                })}
              </div>
            </section>

            {/* Actions */}
            <div className="flex gap-3 pt-1">
              <button
                onClick={handleSave}
                disabled={saving}
                className="flex-1 px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-[1.02] disabled:opacity-50"
                style={{ background: '#F0B90B', color: '#000' }}
              >
                {saving ? '...' : t('notifications.save', language)}
              </button>
              <button
                onClick={handleTest}
                disabled={testing}
                className="flex-1 px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-[1.02] disabled:opacity-50 flex items-center justify-center gap-2"
                style={{
                  background: '#2B3139',
                  color: '#EAECEF',
                  border: '1px solid #474D57',
                }}
              >
                <Send className="w-4 h-4" />
                {testing ? '...' : t('notifications.test', language)}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
