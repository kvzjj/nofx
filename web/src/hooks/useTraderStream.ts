import { useEffect, useRef, useState } from 'react'
import { mutate } from 'swr'
import type { AccountInfo, Position } from '../types'

export type StreamStatus = 'connecting' | 'connected' | 'disconnected'

interface StreamSnapshot {
  type: 'snapshot'
  ts: number
  account?: AccountInfo
  positions?: Position[]
}

/**
 * 订阅后端 SSE 实时快照（账户 + 持仓），直接写入 SWR 缓存。
 *
 * - 快照通过 `mutate(key, data, { revalidate: false })` 无感更新缓存，
 *   原有的 15s 轮询保留作为降级兜底（SSE 断开时数据依然可用）。
 * - EventSource 不能携带 Authorization 头，token 通过查询参数传递，
 *   后端 authMiddleware 做了相同的 JWT 校验。
 */
export function useTraderStream(traderId?: string): { status: StreamStatus } {
  const [status, setStatus] = useState<StreamStatus>('disconnected')
  const retryTimerRef = useRef<number | undefined>(undefined)

  useEffect(() => {
    if (!traderId) {
      setStatus('disconnected')
      return
    }

    const token = localStorage.getItem('auth_token')
    if (!token) {
      setStatus('disconnected')
      return
    }

    setStatus('connecting')
    let closed = false
    let attempt = 0

    // 指数退避重连（2s 起步，最长 30s）
    const retryDelay = () => {
      const delay = Math.min(2000 * Math.pow(1.5, attempt), 30000)
      attempt++
      return delay
    }

    const connect = () => {
      if (closed) return
      const source = new EventSource(
        `/api/traders/${encodeURIComponent(traderId)}/stream?token=${encodeURIComponent(token)}`
      )

      const handleSnapshot = (ev: MessageEvent) => {
        if (closed) return
        setStatus('connected')
        try {
          const payload = JSON.parse(ev.data) as StreamSnapshot
          if (payload.account) {
            mutate(`account-${traderId}`, payload.account, {
              revalidate: false,
            })
          }
          if (payload.positions) {
            mutate(`positions-${traderId}`, payload.positions, {
              revalidate: false,
            })
          }
        } catch {
          // 忽略损坏的帧，等待下一帧
        }
      }

      source.addEventListener('snapshot', handleSnapshot as EventListener)
      source.onopen = () => {
        if (!closed) setStatus('connected')
      }
      source.onerror = () => {
        source.close()
        if (closed) return
        setStatus('disconnected')
        retryTimerRef.current = window.setTimeout(connect, retryDelay())
      }
    }

    connect()

    return () => {
      closed = true
      if (retryTimerRef.current !== undefined) {
        window.clearTimeout(retryTimerRef.current)
        retryTimerRef.current = undefined
      }
      setStatus('disconnected')
    }
  }, [traderId])

  return { status }
}
