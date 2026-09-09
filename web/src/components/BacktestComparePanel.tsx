import { useEffect, useMemo, useState } from 'react'
import useSWR from 'swr'
import { api } from '../lib/api'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts'
import { GitCompareArrows } from 'lucide-react'
import type {
  BacktestRunMetadata,
  BacktestEquityPoint,
  BacktestMetrics,
} from '../types'

const MAX_COMPARE = 4

// 对比曲线配色（与黄黑主题协调）
const SERIES_COLORS = ['#FCD535', '#6366F1', '#0ECB81', '#F6465D']

/**
 * 回测对比视图：选择多个已完成的回测运行，
 * 叠加归一化收益率曲线并排对比核心指标。
 */
export function BacktestComparePanel({
  runs,
  tr,
}: {
  runs: BacktestRunMetadata[]
  tr: (key: string, params?: Record<string, string | number>) => string
}) {
  const [selected, setSelected] = useState<string[]>([])

  const finished = useMemo(
    () => runs.filter((r) => r.state === 'finished' || r.state === 'completed'),
    [runs]
  )

  // 数据源被删除时清理选择
  useEffect(() => {
    setSelected((prev) =>
      prev.filter((id) => finished.some((r) => r.run_id === id))
    )
  }, [finished])

  const toggle = (runId: string) => {
    setSelected((prev) => {
      if (prev.includes(runId)) return prev.filter((id) => id !== runId)
      if (prev.length >= MAX_COMPARE) return prev
      return [...prev, runId]
    })
  }

  // 拉取每个选中运行的权益曲线与指标
  const { data: equityByRun } = useSWR(
    selected.length > 0 ? ['bt-compare-equity', ...selected] : null,
    async () => {
      const entries = await Promise.all(
        selected.map(async (runId) => {
          const equity = await api.getBacktestEquity(runId)
          return [runId, equity] as const
        })
      )
      return Object.fromEntries(entries) as Record<
        string,
        BacktestEquityPoint[]
      >
    },
    { revalidateOnFocus: false }
  )

  const { data: metricsByRun } = useSWR(
    selected.length > 0 ? ['bt-compare-metrics', ...selected] : null,
    async () => {
      const entries = await Promise.all(
        selected.map(async (runId) => {
          const metrics = await api.getBacktestMetrics(runId)
          return [runId, metrics] as const
        })
      )
      return Object.fromEntries(entries) as Record<string, BacktestMetrics>
    },
    { revalidateOnFocus: false }
  )

  // 合并为按时间对齐的序列（按各曲线自身时间轴合并，缺失点沿用前值）
  const chartData = useMemo(() => {
    if (!equityByRun) return []
    const allPoints = new Map<number, Record<string, number>>()
    for (const runId of selected) {
      const series = equityByRun[runId] ?? []
      let lastPct = 0
      for (const point of series) {
        lastPct = point.pnl_pct
        const row = allPoints.get(point.ts) ?? { ts: point.ts }
        row[runId] = lastPct
        allPoints.set(point.ts, row)
      }
    }
    const rows = Array.from(allPoints.values())
    // 前向填充：确保每行都有全部序列的值
    const carry: Record<string, number> = {}
    for (const row of rows) {
      for (const runId of selected) {
        if (row[runId] === undefined) {
          row[runId] = carry[runId] ?? 0
        }
        carry[runId] = row[runId]
      }
    }
    return rows
  }, [equityByRun, selected])

  const labelOf = (runId: string) => {
    const run = runs.find((r) => r.run_id === runId)
    return run?.label || runId.slice(0, 8)
  }

  return (
    <div className="p-5 space-y-4 binance-card xl:col-span-2">
      <div className="flex flex-wrap gap-3 justify-between items-center">
        <div>
          <h3
            className="text-lg font-semibold flex items-center gap-2"
            style={{ color: '#EAECEF' }}
          >
            <GitCompareArrows size={18} />
            {tr('compare.title')}
          </h3>
          <p className="text-xs" style={{ color: '#848E9C' }}>
            {tr('compare.desc', { max: MAX_COMPARE })}
          </p>
        </div>
        {selected.length > 0 && (
          <button
            type="button"
            onClick={() => setSelected([])}
            className="px-3 py-1.5 rounded text-xs font-bold"
            style={{
              background: '#1E2329',
              border: '1px solid #2B3139',
              color: '#848E9C',
            }}
          >
            {tr('compare.clear')}
          </button>
        )}
      </div>

      {/* 运行选择器 */}
      {finished.length === 0 ? (
        <p className="text-xs py-3" style={{ color: '#848E9C' }}>
          {tr('compare.noFinished')}
        </p>
      ) : (
        <div className="flex flex-wrap gap-2">
          {finished.map((run) => {
            const idx = selected.indexOf(run.run_id)
            const active = idx >= 0
            return (
              <button
                key={run.run_id}
                type="button"
                onClick={() => toggle(run.run_id)}
                className="px-3 py-1.5 rounded text-xs font-semibold transition-all"
                style={{
                  background: active ? `${SERIES_COLORS[idx]}22` : '#0B0E11',
                  border: `1px solid ${active ? SERIES_COLORS[idx] : '#2B3139'}`,
                  color: active ? SERIES_COLORS[idx] : '#848E9C',
                }}
                title={run.run_id}
              >
                {active && (
                  <span
                    className="inline-block w-2 h-2 rounded-full mr-1.5"
                    style={{ background: SERIES_COLORS[idx] }}
                  />
                )}
                {run.label || run.run_id.slice(0, 8)}
              </button>
            )
          })}
        </div>
      )}

      {/* 叠加权益曲线（归一化收益率 %） */}
      {selected.length > 0 && chartData.length > 0 && (
        <div style={{ width: '100%', height: 320 }}>
          <ResponsiveContainer>
            <LineChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#2B3139" />
              <XAxis
                dataKey="ts"
                type="number"
                scale="time"
                domain={['dataMin', 'dataMax']}
                tickFormatter={(ts: number) =>
                  new Date(ts * 1000).toLocaleDateString()
                }
                stroke="#848E9C"
                fontSize={11}
              />
              <YAxis
                stroke="#848E9C"
                fontSize={11}
                tickFormatter={(v: number) => `${v.toFixed(1)}%`}
              />
              <Tooltip
                contentStyle={{
                  background: '#181A20',
                  border: '1px solid #2B3139',
                  borderRadius: 8,
                  fontSize: 12,
                }}
                labelFormatter={(ts) =>
                  new Date(Number(ts) * 1000).toLocaleString()
                }
                formatter={(value: number, name: string) => [
                  `${Number(value).toFixed(2)}%`,
                  labelOf(name),
                ]}
              />
              <Legend formatter={(name: string) => labelOf(name)} />
              {selected.map((runId, i) => (
                <Line
                  key={runId}
                  type="monotone"
                  dataKey={runId}
                  stroke={SERIES_COLORS[i]}
                  dot={false}
                  strokeWidth={2}
                />
              ))}
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}

      {/* 指标对比表 */}
      {selected.length > 0 && metricsByRun && (
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead className="text-left border-b border-gray-800">
              <tr>
                <th className="pb-2 font-semibold text-gray-400">
                  {tr('compare.metric')}
                </th>
                {selected.map((runId, i) => (
                  <th key={runId} className="pb-2 font-semibold text-right">
                    <span style={{ color: SERIES_COLORS[i] }}>
                      {labelOf(runId)}
                    </span>
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {(
                [
                  ['total_return_pct', tr('compare.totalReturn'), 'pct', true],
                  ['max_drawdown_pct', tr('compare.maxDrawdown'), 'pct', false],
                  ['sharpe_ratio', tr('compare.sharpe'), 'num', true],
                  ['profit_factor', tr('compare.profitFactor'), 'num', true],
                  ['win_rate', tr('compare.winRate'), 'pct', true],
                  ['trades', tr('compare.trades'), 'int', null],
                ] as const
              ).map(([key, label, fmt, higherBetter]) => {
                const values = selected.map(
                  (runId) =>
                    (metricsByRun[runId] as any)?.[key] as number | undefined
                )
                // 最优值高亮
                const valid = values.filter(
                  (v) => typeof v === 'number'
                ) as number[]
                let best: number | undefined
                if (valid.length > 1 && higherBetter !== null) {
                  best = higherBetter ? Math.max(...valid) : Math.min(...valid)
                }
                return (
                  <tr
                    key={key}
                    className="border-b border-gray-800 last:border-0"
                  >
                    <td className="py-2" style={{ color: '#848E9C' }}>
                      {label}
                    </td>
                    {values.map((v, i) => {
                      const isBest = best !== undefined && v === best
                      let text = '--'
                      if (typeof v === 'number') {
                        if (fmt === 'pct') text = `${v.toFixed(2)}%`
                        else if (fmt === 'num') text = v.toFixed(2)
                        else text = String(v)
                      }
                      return (
                        <td
                          key={selected[i]}
                          className="py-2 text-right font-mono"
                          style={{
                            color: isBest ? '#FCD535' : '#EAECEF',
                            fontWeight: isBest ? 'bold' : 'normal',
                          }}
                        >
                          {text}
                        </td>
                      )
                    })}
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
