import { useCallback, useEffect, useRef, useState } from 'react'
import type { Position, DecisionRecord } from '../types'
import { t, type Language } from '../i18n/translations'
import { notify as appNotify } from '../lib/notify'
import { liquidationDistancePct, LIQ_RISK_THRESHOLD_PCT } from '../lib/format'

const STORAGE_KEY = 'tradingAlertsEnabled'

/** 仓位唯一键：同一币种可能同时有多空两个方向 */
const positionKey = (p: Position) => `${p.symbol}:${p.side.toUpperCase()}`

export interface TradingAlertsApi {
  /** 用户是否开启了通知 */
  enabled: boolean
  /** 浏览器是否支持 Notification API */
  supported: boolean
  /** 开/关通知（开启时会向浏览器申请权限） */
  toggle: () => Promise<void>
}

interface UseTradingAlertsOptions {
  traderName?: string
  positions?: Position[]
  decisions?: DecisionRecord[]
  language: Language
}

/**
 * 交易事件浏览器通知：
 * - AI 开仓 / 平仓（持仓列表 diff）
 * - 强平风险（距强平价 < 10%，同一仓位只提醒一次，恢复后重置）
 * - 决策周期失败（新的失败周期）
 *
 * 页面在前台时用应用内 toast，后台或不可见时用系统通知。
 */
export function useTradingAlerts({
  traderName,
  positions,
  decisions,
  language,
}: UseTradingAlertsOptions): TradingAlertsApi {
  const [enabled, setEnabled] = useState<boolean>(() => {
    try {
      return localStorage.getItem(STORAGE_KEY) === '1'
    } catch {
      return false
    }
  })
  const enabledRef = useRef(enabled)
  useEffect(() => {
    enabledRef.current = enabled
  }, [enabled])

  const supported = typeof window !== 'undefined' && 'Notification' in window

  const send = useCallback((title: string, body: string) => {
    if (
      typeof document !== 'undefined' &&
      document.hidden &&
      'Notification' in window &&
      Notification.permission === 'granted'
    ) {
      try {
        const n = new Notification(title, { body, icon: '/icons/nofx.svg' })
        // 点击通知时回到交易页面
        n.onclick = () => {
          window.focus()
          n.close()
        }
      } catch {
        appNotify.info(`${title} — ${body}`)
      }
    } else {
      appNotify.info(title, { description: body })
    }
  }, [])

  const toggle = useCallback(async () => {
    if (!('Notification' in window)) {
      appNotify.error(t('notificationDenied', language))
      return
    }
    if (enabled) {
      setEnabled(false)
      try {
        localStorage.setItem(STORAGE_KEY, '0')
      } catch {
        /* ignore */
      }
      return
    }
    let permission = Notification.permission
    if (permission !== 'granted') {
      permission = await Notification.requestPermission()
    }
    if (permission === 'granted') {
      setEnabled(true)
      try {
        localStorage.setItem(STORAGE_KEY, '1')
      } catch {
        /* ignore */
      }
      appNotify.success(t('notificationsEnabled', language))
    } else {
      appNotify.error(t('notificationDenied', language))
    }
  }, [enabled, language])

  // —— 持仓开/平仓 & 强平风险检测 ——
  const prevPositionsRef = useRef<Map<string, Position>>(new Map())
  const liqAlertedRef = useRef<Set<string>>(new Set())
  const seededRef = useRef(false)

  useEffect(() => {
    if (!positions) return

    const current = new Map(positions.map((p) => [positionKey(p), p]))
    const prev = prevPositionsRef.current

    // 首次见到数据时只做基线填充，不把存量持仓当作"新开仓"轰炸用户
    if (!seededRef.current) {
      seededRef.current = true
      prevPositionsRef.current = current
      return
    }

    if (enabledRef.current) {
      const name = traderName || 'Trader'

      // 新开仓：当前有、之前没有
      for (const [key, pos] of current) {
        if (!prev.has(key)) {
          send(
            t('notifPositionOpened', language, {
              trader: name,
              side: pos.side.toUpperCase(),
              symbol: pos.symbol,
            }),
            `${pos.leverage}x`
          )
        }
      }

      // 已平仓：之前有、当前没有
      for (const [key, pos] of prev) {
        if (!current.has(key)) {
          send(
            t('notifPositionClosed', language, {
              trader: name,
              side: pos.side.toUpperCase(),
              symbol: pos.symbol,
            }),
            ''
          )
        }
      }

      // 强平风险（去重：恢复到安全距离后重置，可再次告警）
      for (const [key, pos] of current) {
        const dist = liquidationDistancePct(
          pos.mark_price,
          pos.liquidation_price
        )
        if (dist !== undefined && dist < LIQ_RISK_THRESHOLD_PCT) {
          if (!liqAlertedRef.current.has(key)) {
            liqAlertedRef.current.add(key)
            send(
              t('liqWarningTitle', language),
              t('notifLiqRisk', language, {
                symbol: pos.symbol,
                pct: dist.toFixed(1),
              })
            )
          }
        } else {
          liqAlertedRef.current.delete(key)
        }
      }
    }

    prevPositionsRef.current = current
  }, [positions, traderName, language, send])

  // —— 决策周期失败检测 ——
  const lastCycleRef = useRef<number>(0)

  useEffect(() => {
    if (!decisions || decisions.length === 0) return

    const maxCycle = Math.max(...decisions.map((d) => d.cycle_number))

    // 首次加载只记录水位线
    if (lastCycleRef.current === 0) {
      lastCycleRef.current = maxCycle
      return
    }

    if (!enabledRef.current) {
      lastCycleRef.current = maxCycle
      return
    }

    const failed = decisions.find(
      (d) => d.cycle_number > lastCycleRef.current && !d.success
    )
    if (failed) {
      send(
        t('notifDecisionError', language, { trader: traderName || 'Trader' }),
        failed.error_message || `Cycle #${failed.cycle_number}`
      )
    }
    lastCycleRef.current = Math.max(lastCycleRef.current, maxCycle)
  }, [decisions, traderName, language, send])

  return { enabled, supported, toggle }
}
