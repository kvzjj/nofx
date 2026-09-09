import useSWR from 'swr'
import { api } from '../lib/api'
import { t } from '../i18n/translations'
import { useLanguage } from '../contexts/LanguageContext'
import { ShieldCheck, Users, Bot, Activity, Database, Cpu } from 'lucide-react'
import type {
  AdminUser,
  AdminSystemStatus,
  AdminDecisionFeedItem,
} from '../types'

export function AdminPage() {
  const { language } = useLanguage()
  const { data: users } = useSWR<AdminUser[]>('admin-users', async () => {
    try {
      return await api.getAdminUsers()
    } catch {
      return []
    }
  })
  const { data: status } = useSWR<AdminSystemStatus>(
    'admin-status',
    api.getAdminSystemStatus,
    { refreshInterval: 15000 }
  )
  const { data: feed } = useSWR<AdminDecisionFeedItem[]>(
    'admin-feed',
    api.getAdminRecentDecisions,
    { refreshInterval: 30000 }
  )

  return (
    <div className="space-y-6 max-w-6xl">
      {/* 页头 */}
      <div>
        <h1
          className="text-2xl font-bold flex items-center gap-2"
          style={{ color: '#EAECEF' }}
        >
          <ShieldCheck size={24} />
          {t('adminTitle', language)}
        </h1>
        <p className="text-sm mt-1" style={{ color: '#848E9C' }}>
          {t('adminDesc', language)}
        </p>
      </div>

      {/* 系统状态卡片 */}
      <div className="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-6 gap-3">
        <StatTile
          icon={<Users size={15} />}
          label={t('adminUsersTotal', language)}
          value={status ? `${status.users_total}` : '--'}
          sub={
            status
              ? `${t('adminVerified', language)}: ${status.users_verified}`
              : undefined
          }
        />
        <StatTile
          icon={<Bot size={15} />}
          label={t('adminTradersTotal', language)}
          value={status ? `${status.traders_total}` : '--'}
        />
        <StatTile
          icon={<Activity size={15} />}
          label={t('adminTradersRunning', language)}
          value={status ? `${status.traders_running}` : '--'}
          accent={status && status.traders_running > 0 ? '#0ECB81' : undefined}
        />
        <StatTile
          icon={<Database size={15} />}
          label="DB"
          value={status ? `${status.db_size_mb.toFixed(1)} MB` : '--'}
        />
        <StatTile
          icon={<Cpu size={15} />}
          label={t('adminMemory', language)}
          value={status ? `${status.heap_alloc_mb.toFixed(0)} MB` : '--'}
          sub={
            status
              ? `${t('adminGoroutines', language)}: ${status.goroutines}`
              : undefined
          }
        />
        <StatTile
          icon={<Cpu size={15} />}
          label="CPU / Go"
          value={status ? `${status.cpu_count}c / ${status.go_version}` : '--'}
          sub={
            status
              ? new Date(status.server_time).toLocaleTimeString()
              : undefined
          }
        />
      </div>

      {/* 用户列表 */}
      <div className="binance-card p-4">
        <h2
          className="font-bold mb-3 flex items-center gap-2"
          style={{ color: '#EAECEF' }}
        >
          <Users size={16} />
          {t('adminUserList', language)}
        </h2>
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead className="text-left border-b border-gray-800">
              <tr>
                <th className="pb-2 font-semibold text-gray-400">
                  {t('emailCol', language)}
                </th>
                <th className="pb-2 font-semibold text-gray-400">
                  {t('registeredAt', language)}
                </th>
                <th className="pb-2 font-semibold text-gray-400 text-center">
                  2FA
                </th>
                <th className="pb-2 font-semibold text-gray-400 text-right">
                  {t('adminTraderCount', language)}
                </th>
                <th className="pb-2 font-semibold text-gray-400 text-right">
                  {t('adminRunningCount', language)}
                </th>
              </tr>
            </thead>
            <tbody>
              {(users ?? []).map((u) => (
                <tr
                  key={u.user_id}
                  className="border-b border-gray-800 last:border-0"
                >
                  <td className="py-2 font-mono" style={{ color: '#EAECEF' }}>
                    {u.email}
                  </td>
                  <td className="py-2 font-mono" style={{ color: '#848E9C' }}>
                    {new Date(u.created_at).toLocaleDateString()}
                  </td>
                  <td className="py-2 text-center">
                    <span
                      style={{ color: u.otp_verified ? '#0ECB81' : '#848E9C' }}
                    >
                      {u.otp_verified ? '✓' : '—'}
                    </span>
                  </td>
                  <td
                    className="py-2 text-right font-mono"
                    style={{ color: '#EAECEF' }}
                  >
                    {u.trader_count}
                  </td>
                  <td className="py-2 text-right font-mono">
                    <span
                      style={{
                        color: u.running_count > 0 ? '#0ECB81' : '#848E9C',
                      }}
                    >
                      {u.running_count}
                    </span>
                  </td>
                </tr>
              ))}
              {(!users || users.length === 0) && (
                <tr>
                  <td
                    colSpan={5}
                    className="py-6 text-center"
                    style={{ color: '#848E9C' }}
                  >
                    {t('adminNoUsers', language)}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* 全局决策动态 */}
      <div className="binance-card p-4">
        <h2
          className="font-bold mb-3 flex items-center gap-2"
          style={{ color: '#EAECEF' }}
        >
          <Activity size={16} />
          {t('adminRecentActivity', language)}
        </h2>
        <div className="space-y-1.5 max-h-80 overflow-y-auto">
          {(feed ?? []).map((item, i) => (
            <div
              key={i}
              className="flex items-center justify-between gap-3 text-xs py-1.5 px-2 rounded"
              style={{ background: '#0B0E11' }}
            >
              <span className="font-mono truncate" style={{ color: '#EAECEF' }}>
                {item.trader_id}
              </span>
              <span className="flex-shrink-0" style={{ color: '#848E9C' }}>
                #{item.cycle_number}
              </span>
              <span
                className="flex-shrink-0 font-bold"
                style={{ color: item.success ? '#0ECB81' : '#F6465D' }}
                title={item.error || undefined}
              >
                {item.success ? '✓' : '✗'}
              </span>
              <span
                className="flex-shrink-0 font-mono"
                style={{ color: '#5E6673' }}
              >
                {new Date(item.timestamp).toLocaleTimeString()}
              </span>
            </div>
          ))}
          {(!feed || feed.length === 0) && (
            <div
              className="py-6 text-center text-xs"
              style={{ color: '#848E9C' }}
            >
              {t('noDecisionsYet', language)}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function StatTile({
  icon,
  label,
  value,
  sub,
  accent,
}: {
  icon: React.ReactNode
  label: string
  value: string
  sub?: string
  accent?: string
}) {
  return (
    <div
      className="rounded-lg p-3"
      style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
    >
      <div
        className="text-[10px] uppercase tracking-wide mb-1 flex items-center gap-1"
        style={{ color: '#848E9C' }}
      >
        {icon}
        {label}
      </div>
      <div
        className="text-lg font-bold font-mono leading-tight"
        style={{ color: accent ?? '#EAECEF' }}
      >
        {value}
      </div>
      {sub && (
        <div className="text-[10px] mt-0.5" style={{ color: '#5E6673' }}>
          {sub}
        </div>
      )}
    </div>
  )
}
