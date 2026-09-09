import { useEffect, useState, type ReactNode } from 'react'
import useSWR from 'swr'
import { api } from '../lib/api'
import { t, type Language } from '../i18n/translations'
import { useLanguage } from '../contexts/LanguageContext'
import { toast } from 'sonner'
import {
  Bell,
  Plus,
  Send,
  Trash2,
  Pencil,
  Loader2,
  Globe,
  Mail,
  CheckCircle2,
  XCircle,
} from 'lucide-react'
import type { NotificationChannel, NotificationLog } from '../types'

// 各渠道类型的配置字段定义
const CHANNEL_FIELDS: Record<
  string,
  { key: string; secret?: boolean; placeholder: string }[]
> = {
  telegram: [
    { key: 'bot_token', secret: true, placeholder: '123456:ABC-DEF...' },
    { key: 'chat_id', placeholder: '-1001234567890' },
  ],
  webhook: [
    { key: 'url', placeholder: 'https://example.com/webhook' },
    {
      key: 'secret',
      secret: true,
      placeholder: 'HMAC-SHA256 secret (optional)',
    },
  ],
  email: [
    { key: 'smtp_host', placeholder: 'smtp.gmail.com' },
    { key: 'smtp_port', placeholder: '587' },
    { key: 'smtp_user', secret: true, placeholder: 'user@gmail.com' },
    { key: 'smtp_pass', secret: true, placeholder: 'app password' },
    { key: 'from', placeholder: 'auaiex@example.com' },
    { key: 'to', placeholder: 'me@example.com' },
  ],
}

const CHANNEL_TYPES: {
  value: 'telegram' | 'webhook' | 'email'
  labelKey: 'telegramChannel' | 'webhookChannel' | 'emailChannel'
  icon: ReactNode
}[] = [
  { value: 'telegram', labelKey: 'telegramChannel', icon: <Send size={16} /> },
  { value: 'webhook', labelKey: 'webhookChannel', icon: <Globe size={16} /> },
  { value: 'email', labelKey: 'emailChannel', icon: <Mail size={16} /> },
]

export function NotificationsPage() {
  const { language } = useLanguage()
  const { data: channels, mutate: mutateChannels } = useSWR<
    NotificationChannel[]
  >('notification-channels', api.getNotificationChannels)
  const { data: logs, mutate: mutateLogs } = useSWR<NotificationLog[]>(
    'notification-logs',
    api.getNotificationLogs
  )

  const [editing, setEditing] = useState<NotificationChannel | null>(null)
  const [creating, setCreating] = useState(false)
  const [testing, setTesting] = useState<string | null>(null)

  const handleCloseModal = () => {
    setEditing(null)
    setCreating(false)
  }

  const handleSaved = async () => {
    handleCloseModal()
    await mutateChannels()
  }

  const handleDelete = async (id: string) => {
    try {
      await api.deleteNotificationChannel(id)
      toast.success(t('channelDeleted', language))
      await mutateChannels()
    } catch {
      toast.error(t('operationFailed', language))
    }
  }

  const handleToggle = async (channel: NotificationChannel) => {
    try {
      await api.updateNotificationChannel(channel.id, {
        name: channel.name,
        type: channel.type,
        config: channel.config,
        enabled: !channel.enabled,
      })
      await mutateChannels()
    } catch {
      toast.error(t('operationFailed', language))
    }
  }

  const handleTest = async (id: string) => {
    setTesting(id)
    try {
      await api.testNotificationChannel(id)
      toast.success(t('testSent', language))
      await mutateLogs()
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t('testFailed', language)
      )
    } finally {
      setTesting(null)
    }
  }

  return (
    <div className="space-y-6">
      {/* 页头 */}
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div>
          <h1
            className="text-2xl font-bold flex items-center gap-2"
            style={{ color: '#EAECEF' }}
          >
            <Bell size={24} />
            {t('notificationsTitle', language)}
          </h1>
          <p className="text-sm mt-1" style={{ color: '#848E9C' }}>
            {t('notificationsDesc', language)}
          </p>
        </div>
        <button
          onClick={() => setCreating(true)}
          className="px-4 py-2 rounded-lg font-bold text-sm transition-all hover:scale-105 inline-flex items-center gap-2"
          style={{
            background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
            color: '#0B0E11',
          }}
        >
          <Plus size={16} />
          {t('addChannel', language)}
        </button>
      </div>

      {/* 渠道列表 */}
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
        {channels?.map((channel) => (
          <div key={channel.id} className="binance-card p-4 space-y-3">
            <div className="flex items-start justify-between gap-2">
              <div className="flex items-center gap-2 min-w-0">
                <div
                  className="w-9 h-9 rounded-lg flex items-center justify-center flex-shrink-0"
                  style={{
                    background: 'rgba(240, 185, 11, 0.12)',
                    color: '#F0B90B',
                  }}
                >
                  {CHANNEL_TYPES.find((ct) => ct.value === channel.type)?.icon}
                </div>
                <div className="min-w-0">
                  <div
                    className="font-bold truncate"
                    style={{ color: '#EAECEF' }}
                  >
                    {channel.name}
                  </div>
                  <div
                    className="text-[11px] uppercase"
                    style={{ color: '#848E9C' }}
                  >
                    {t(
                      CHANNEL_TYPES.find((ct) => ct.value === channel.type)
                        ?.labelKey ?? 'telegramChannel',
                      language
                    )}
                  </div>
                </div>
              </div>
              {/* 启用开关 */}
              <button
                type="button"
                onClick={() => handleToggle(channel)}
                className="flex-shrink-0 relative w-10 h-5 rounded-full transition-colors"
                style={{
                  background: channel.enabled ? '#0ECB81' : '#2B3139',
                }}
                title={
                  channel.enabled
                    ? t('enabled', language)
                    : t('disabled', language)
                }
              >
                <span
                  className="absolute top-0.5 w-4 h-4 rounded-full bg-white transition-all"
                  style={{ left: channel.enabled ? '22px' : '2px' }}
                />
              </button>
            </div>

            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => handleTest(channel.id)}
                disabled={testing === channel.id}
                className="flex-1 px-3 py-1.5 rounded text-xs font-bold transition-all hover:scale-105 inline-flex items-center justify-center gap-1.5 disabled:opacity-50"
                style={{
                  background: 'rgba(14, 203, 129, 0.1)',
                  color: '#0ECB81',
                }}
              >
                {testing === channel.id ? (
                  <Loader2 size={13} className="animate-spin" />
                ) : (
                  <Send size={13} />
                )}
                {t('sendTest', language)}
              </button>
              <button
                type="button"
                onClick={() => setEditing(channel)}
                className="px-3 py-1.5 rounded text-xs font-bold transition-all hover:scale-105"
                style={{
                  background: 'rgba(255, 193, 7, 0.1)',
                  color: '#FFC107',
                }}
              >
                <Pencil size={13} />
              </button>
              <button
                type="button"
                onClick={() => handleDelete(channel.id)}
                className="px-3 py-1.5 rounded text-xs font-bold transition-all hover:scale-105"
                style={{
                  background: 'rgba(246, 70, 93, 0.1)',
                  color: '#F6465D',
                }}
              >
                <Trash2 size={13} />
              </button>
            </div>
          </div>
        ))}

        {channels && channels.length === 0 && (
          <div
            className="col-span-full text-center py-14 rounded-lg"
            style={{ border: '1px dashed #2B3139', color: '#848E9C' }}
          >
            <Bell size={36} className="mx-auto mb-3 opacity-50" />
            <div className="text-sm">{t('noChannels', language)}</div>
            <div className="text-xs mt-1">{t('noChannelsDesc', language)}</div>
          </div>
        )}
      </div>

      {/* 发送日志 */}
      <div className="binance-card p-4">
        <h2
          className="font-bold mb-3 flex items-center gap-2"
          style={{ color: '#EAECEF' }}
        >
          <Bell size={16} />
          {t('deliveryLogs', language)}
        </h2>
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead className="text-left border-b border-gray-800">
              <tr>
                <th className="pb-2 font-semibold text-gray-400">
                  {t('timeCol', language)}
                </th>
                <th className="pb-2 font-semibold text-gray-400">
                  {t('channelName', language)}
                </th>
                <th className="pb-2 font-semibold text-gray-400">
                  {t('eventCol', language)}
                </th>
                <th className="pb-2 font-semibold text-gray-400">
                  {t('messageCol', language)}
                </th>
                <th className="pb-2 font-semibold text-gray-400 text-right">
                  {t('statusCol', language)}
                </th>
              </tr>
            </thead>
            <tbody>
              {(logs ?? []).map((log) => (
                <tr
                  key={log.id}
                  className="border-b border-gray-800 last:border-0"
                >
                  <td
                    className="py-2 font-mono whitespace-nowrap"
                    style={{ color: '#848E9C' }}
                  >
                    {new Date(log.created_at).toLocaleString()}
                  </td>
                  <td
                    className="py-2 whitespace-nowrap"
                    style={{ color: '#EAECEF' }}
                  >
                    {log.channel_name}
                  </td>
                  <td
                    className="py-2 whitespace-nowrap"
                    style={{ color: '#848E9C' }}
                  >
                    {log.event}
                  </td>
                  <td
                    className="py-2 max-w-xs truncate"
                    style={{ color: '#EAECEF' }}
                    title={log.message}
                  >
                    {log.message}
                  </td>
                  <td className="py-2 text-right">
                    {log.success ? (
                      <span
                        className="inline-flex items-center gap-1"
                        style={{ color: '#0ECB81' }}
                      >
                        <CheckCircle2 size={13} /> OK
                      </span>
                    ) : (
                      <span
                        className="inline-flex items-center gap-1"
                        style={{ color: '#F6465D' }}
                        title={log.error}
                      >
                        <XCircle size={13} /> {t('failedCol', language)}
                      </span>
                    )}
                  </td>
                </tr>
              ))}
              {(!logs || logs.length === 0) && (
                <tr>
                  <td
                    colSpan={5}
                    className="py-6 text-center"
                    style={{ color: '#848E9C' }}
                  >
                    {t('noDeliveryLogs', language)}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* 新建/编辑弹窗 */}
      {(creating || editing) && (
        <ChannelModal
          channel={editing}
          language={language}
          onClose={handleCloseModal}
          onSaved={handleSaved}
        />
      )}
    </div>
  )
}

function ChannelModal({
  channel,
  language,
  onClose,
  onSaved,
}: {
  channel: NotificationChannel | null
  language: Language
  onClose: () => void
  onSaved: () => void
}) {
  const [name, setName] = useState(channel?.name ?? '')
  const [type, setType] = useState<'telegram' | 'webhook' | 'email'>(
    (channel?.type as 'telegram' | 'webhook' | 'email') ?? 'telegram'
  )
  const [config, setConfig] = useState<Record<string, string>>(
    Object.fromEntries(
      Object.entries(channel?.config ?? {}).map(([k, v]) => [
        k,
        String(v ?? ''),
      ])
    )
  )
  const [saving, setSaving] = useState(false)

  // 切换渠道类型时清空配置
  useEffect(() => {
    setConfig({})
  }, [type])

  const fields = CHANNEL_FIELDS[type] ?? []

  const handleSave = async () => {
    if (!name.trim()) {
      toast.error(t('channelNameRequired', language))
      return
    }
    setSaving(true)
    try {
      if (channel) {
        await api.updateNotificationChannel(channel.id, {
          name: name.trim(),
          type,
          config,
          enabled: channel.enabled,
        })
      } else {
        await api.createNotificationChannel({
          name: name.trim(),
          type,
          config,
          enabled: true,
        })
      }
      toast.success(t('channelSaved', language))
      onSaved()
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t('operationFailed', language)
      )
    } finally {
      setSaving(false)
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
        <h2 className="text-lg font-bold" style={{ color: '#EAECEF' }}>
          {channel ? t('editChannel', language) : t('addChannel', language)}
        </h2>

        {/* 名称 */}
        <div>
          <label
            className="text-xs font-bold block mb-1.5"
            style={{ color: '#848E9C' }}
          >
            {t('channelName', language)}
          </label>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="w-full px-3 py-2 rounded-lg text-sm outline-none"
            style={{
              background: '#0B0E11',
              border: '1px solid #2B3139',
              color: '#EAECEF',
            }}
            placeholder="My Telegram"
          />
        </div>

        {/* 渠道类型 */}
        <div>
          <label
            className="text-xs font-bold block mb-1.5"
            style={{ color: '#848E9C' }}
          >
            {t('channelType', language)}
          </label>
          <div className="grid grid-cols-3 gap-2">
            {CHANNEL_TYPES.map((ct) => (
              <button
                key={ct.value}
                type="button"
                onClick={() => setType(ct.value)}
                disabled={!!channel}
                className="px-2 py-2 rounded-lg text-xs font-bold transition-all inline-flex flex-col items-center gap-1 disabled:opacity-60"
                style={{
                  background:
                    type === ct.value ? 'rgba(240, 185, 11, 0.15)' : '#0B0E11',
                  border: `1px solid ${type === ct.value ? '#F0B90B' : '#2B3139'}`,
                  color: type === ct.value ? '#F0B90B' : '#848E9C',
                }}
              >
                {ct.icon}
                {t(ct.labelKey, language)}
              </button>
            ))}
          </div>
        </div>

        {/* 类型相关配置字段 */}
        {fields.map((field) => (
          <div key={field.key}>
            <label
              className="text-xs font-bold block mb-1.5"
              style={{ color: '#848E9C' }}
            >
              {field.key}
              {field.secret && (
                <span className="ml-1 font-normal" style={{ color: '#5E6673' }}>
                  ({t('secretField', language)})
                </span>
              )}
            </label>
            <input
              value={config[field.key] ?? ''}
              onChange={(e) =>
                setConfig({ ...config, [field.key]: e.target.value })
              }
              type={field.secret ? 'password' : 'text'}
              className="w-full px-3 py-2 rounded-lg text-sm outline-none font-mono"
              style={{
                background: '#0B0E11',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
              placeholder={field.placeholder}
              autoComplete="off"
            />
          </div>
        ))}

        <p className="text-[11px] leading-relaxed" style={{ color: '#5E6673' }}>
          {t('channelHelp', language)}
        </p>

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
            onClick={handleSave}
            disabled={saving}
            className="flex-1 px-4 py-2 rounded-lg text-sm font-bold transition-all hover:scale-105 disabled:opacity-50 inline-flex items-center justify-center gap-1.5"
            style={{
              background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
              color: '#0B0E11',
            }}
          >
            {saving && <Loader2 size={14} className="animate-spin" />}
            {t('save', language)}
          </button>
        </div>
      </div>
    </div>
  )
}
