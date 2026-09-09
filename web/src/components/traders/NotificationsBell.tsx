import { useState, useEffect, useRef } from 'react'
import { useSWRConfig } from 'swr'
import { Bell, Check, CheckCheck } from 'lucide-react'
import { api } from '../../lib/api'
import type { NotificationRecord } from '../../types'
import { formatDistanceToNow } from 'date-fns'
import { zhCN } from 'date-fns/locale'

const SEVERITY_COLORS: Record<string, string> = {
  info: '#848E9C',
  warning: '#F0B90B',
  critical: '#F6465D',
}

export function NotificationsBell() {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const { mutate } = useSWRConfig()

  // Poll notifications every 30s while mounted
  const [data, setData] = useState<{
    notifications: NotificationRecord[]
    unread: number
  } | null>(null)
  useEffect(() => {
    let cancelled = false
    const load = () => {
      api
        .getNotifications(15, 0)
        .then((d) => {
          if (!cancelled) setData(d)
        })
        .catch(() => {
          /* unauthenticated or network error: stay silent */
        })
    }
    load()
    const timer = setInterval(load, 30_000)
    return () => {
      cancelled = true
      clearInterval(timer)
    }
  }, [])

  // Close on outside click
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  const markRead = async (id: string) => {
    await api.markNotificationRead(id).catch(() => {})
    setData((prev) =>
      prev
        ? {
            notifications: prev.notifications.map((n) =>
              n.id === id ? { ...n, is_read: true } : n
            ),
            unread: Math.max(0, prev.unread - 1),
          }
        : prev
    )
  }

  const markAll = async () => {
    await api.markAllNotificationsRead().catch(() => {})
    setData((prev) =>
      prev
        ? {
            notifications: prev.notifications.map((n) => ({
              ...n,
              is_read: true,
            })),
            unread: 0,
          }
        : prev
    )
    mutate(
      (key: string) =>
        typeof key === 'string' && key.startsWith('notifications')
    )
  }

  const unread = data?.unread || 0

  return (
    <div className="relative" ref={ref}>
      <button
        onClick={() => setOpen((v) => !v)}
        className="relative p-2 rounded transition-all hover:scale-110"
        style={{ background: '#2B3139', border: '1px solid #474D57' }}
        aria-label="notifications"
      >
        <Bell className="w-4 h-4" style={{ color: '#EAECEF' }} />
        {unread > 0 && (
          <span
            className="absolute -top-1 -right-1 min-w-[16px] h-4 px-1 rounded-full text-[10px] font-bold flex items-center justify-center"
            style={{ background: '#F6465D', color: '#fff' }}
          >
            {unread > 99 ? '99+' : unread}
          </span>
        )}
      </button>

      {open && (
        <div
          className="absolute right-0 mt-2 w-80 rounded-lg shadow-2xl overflow-hidden z-50"
          style={{ background: '#1E2329', border: '1px solid #2B3139' }}
        >
          <div
            className="flex items-center justify-between px-4 py-2"
            style={{ borderBottom: '1px solid #2B3139' }}
          >
            <span
              className="text-sm font-semibold"
              style={{ color: '#EAECEF' }}
            >
              通知 / Notifications
            </span>
            {unread > 0 && (
              <button
                onClick={markAll}
                className="flex items-center gap-1 text-xs hover:underline"
                style={{ color: '#F0B90B' }}
              >
                <CheckCheck className="w-3.5 h-3.5" />
                全部已读
              </button>
            )}
          </div>

          <div className="max-h-96 overflow-y-auto">
            {!data || data.notifications.length === 0 ? (
              <p
                className="px-4 py-6 text-center text-xs"
                style={{ color: '#848E9C' }}
              >
                暂无通知 / No notifications
              </p>
            ) : (
              data.notifications.map((n) => (
                <div
                  key={n.id}
                  className="px-4 py-3 flex gap-2 items-start"
                  style={{
                    borderBottom: '1px solid #181A20',
                    background: n.is_read
                      ? 'transparent'
                      : 'rgba(240,185,11,0.06)',
                    cursor: n.is_read ? 'default' : 'pointer',
                  }}
                  onClick={() => !n.is_read && markRead(n.id)}
                >
                  <span
                    className="mt-1.5 w-2 h-2 rounded-full shrink-0"
                    style={{
                      background: SEVERITY_COLORS[n.severity] || '#848E9C',
                    }}
                  />
                  <div className="min-w-0 flex-1">
                    <p
                      className="text-xs font-semibold truncate"
                      style={{ color: '#EAECEF' }}
                      title={n.title}
                    >
                      {n.title}
                    </p>
                    {n.body && (
                      <p
                        className="text-[11px] mt-0.5 line-clamp-2 whitespace-pre-line"
                        style={{ color: '#848E9C' }}
                      >
                        {n.body}
                      </p>
                    )}
                    <p
                      className="text-[10px] mt-1"
                      style={{ color: '#5E6673' }}
                    >
                      {formatDistanceToNow(new Date(n.created_at), {
                        addSuffix: true,
                        locale: zhCN,
                      })}
                    </p>
                  </div>
                  {!n.is_read && (
                    <Check
                      className="w-3.5 h-3.5 mt-1 shrink-0"
                      style={{ color: '#F0B90B' }}
                    />
                  )}
                </div>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  )
}
