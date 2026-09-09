/**
 * 数字与价格格式化工具
 *
 * 交易界面的价格精度不应写死：BTC (>1000) 用 2 位小数即可读，
 * SHIB/WIF (<0.01) 需要 6~8 位小数才有意义。
 * formatPrice 按数量级动态选择小数位数。
 */

/** 根据数值大小决定合适的小数位数 */
export function priceDecimals(value: number): number {
  const abs = Math.abs(value)
  if (!isFinite(abs) || abs === 0) return 2
  if (abs >= 1000) return 2
  if (abs >= 100) return 2
  if (abs >= 1) return 3
  if (abs >= 0.01) return 5
  if (abs >= 0.0001) return 6
  return 8
}

/** 动态精度价格格式化，用于入场价/标记价/强平价等 */
export function formatPrice(value: number | undefined | null): string {
  if (value === undefined || value === null || !isFinite(value)) return '--'
  const decimals = priceDecimals(value)
  return value.toLocaleString('en-US', {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  })
}

/** 数量格式化：大数量收敛小数位，小数量保留精度 */
export function formatQuantity(value: number | undefined | null): string {
  if (value === undefined || value === null || !isFinite(value)) return '--'
  const abs = Math.abs(value)
  let decimals: number
  if (abs >= 10000) decimals = 0
  else if (abs >= 100) decimals = 2
  else if (abs >= 1) decimals = 3
  else if (abs >= 0.01) decimals = 4
  else decimals = 6
  return value.toLocaleString('en-US', {
    minimumFractionDigits: 0,
    maximumFractionDigits: decimals,
  })
}

/** 大额 USDT 金额（权益、保证金等）：千分位 + 2 位小数 */
export function formatUsd(value: number | undefined | null): string {
  if (value === undefined || value === null || !isFinite(value)) return '--'
  return value.toLocaleString('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })
}

/**
 * LTTB (Largest Triangle Three Buckets) 降采样算法
 * 在尽量保留视觉形状的前提下把曲线点数降到 targetCount。
 * 用于长周期净值曲线的渲染性能优化。
 */
export function downsampleLTTB<T>(
  data: T[],
  targetCount: number,
  getX: (point: T) => number,
  getY: (point: T) => number
): T[] {
  if (data.length <= targetCount || targetCount < 3) return data

  const sampled: T[] = []
  const every = (data.length - 2) / (targetCount - 2)
  let index = 0

  sampled.push(data[0])

  for (let i = 0; i < targetCount - 2; i++) {
    // 当前区间内的平均点（作为三角形的固定顶点）
    const avgStart = Math.floor((i + 0) * every) + 1
    const avgEnd = Math.min(Math.floor((i + 1) * every) + 1, data.length)
    let avgX = 0
    let avgY = 0
    let count = 0
    for (let j = avgStart; j < avgEnd; j++) {
      avgX += getX(data[j])
      avgY += getY(data[j])
      count++
    }
    if (count > 0) {
      avgX /= count
      avgY /= count
    }

    // 在下一个区间内选择与三角形面积最大的点
    const rangeStart = Math.floor((i + 1) * every) + 1
    const rangeEnd = Math.min(Math.floor((i + 2) * every) + 1, data.length)
    const pointAX = getX(data[index])
    const pointAY = getY(data[index])

    let maxArea = -1
    let maxIdx = rangeStart
    for (let j = rangeStart; j < rangeEnd; j++) {
      const area = Math.abs(
        (pointAX - avgX) * (getY(data[j]) - pointAY) -
          (pointAX - getX(data[j])) * (avgY - pointAY)
      )
      if (area > maxArea) {
        maxArea = area
        maxIdx = j
      }
    }

    sampled.push(data[maxIdx])
    index = maxIdx
  }

  sampled.push(data[data.length - 1])
  return sampled
}

/**
 * 计算强平距离百分比：当前标记价距强平价还有多远（%）
 * 返回 undefined 表示无法计算（如强平价为 0）
 */
export function liquidationDistancePct(
  markPrice: number,
  liquidationPrice: number
): number | undefined {
  if (!isFinite(markPrice) || !isFinite(liquidationPrice)) return undefined
  if (markPrice <= 0 || liquidationPrice <= 0) return undefined
  return (Math.abs(markPrice - liquidationPrice) / markPrice) * 100
}

/** 强平风险阈值（%），低于该值视为高风险 */
export const LIQ_RISK_THRESHOLD_PCT = 10
