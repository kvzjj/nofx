import { describe, it, expect } from 'vitest'
import {
  formatPrice,
  formatQuantity,
  formatUsd,
  priceDecimals,
  downsampleLTTB,
  liquidationDistancePct,
  LIQ_RISK_THRESHOLD_PCT,
} from './format'

describe('priceDecimals', () => {
  it('大型币种（BTC 级别）用 2 位小数', () => {
    expect(priceDecimals(95000.12)).toBe(2)
    expect(priceDecimals(1500.5)).toBe(2)
  })
  it('中价币用 2~3 位小数', () => {
    expect(priceDecimals(350)).toBe(2)
    expect(priceDecimals(2.35)).toBe(3)
  })
  it('小价币（DOGE 级别）用 5 位小数', () => {
    expect(priceDecimals(0.31)).toBe(5)
  })
  it('微型币（SHIB/WIF 级别）保留 6~8 位小数', () => {
    expect(priceDecimals(0.00042)).toBe(6)
    expect(priceDecimals(0.00001234)).toBe(8)
    expect(priceDecimals(0.00000042)).toBe(8)
  })
})

describe('formatPrice', () => {
  it('BTC 显示千分位 + 2 位小数', () => {
    expect(formatPrice(95123.4567)).toBe('95,123.46')
  })
  it('SHIB 不再被截断成 0.0000', () => {
    expect(formatPrice(0.00042)).toBe('0.000420')
    expect(formatPrice(0.00001234)).toBe('0.00001234')
  })
  it('空值/非法值显示占位符', () => {
    expect(formatPrice(undefined)).toBe('--')
    expect(formatPrice(null)).toBe('--')
    expect(formatPrice(NaN)).toBe('--')
  })
})

describe('formatQuantity', () => {
  it('大数量收敛小数位', () => {
    expect(formatQuantity(123456.789)).toBe('123,457')
  })
  it('小数量保留精度', () => {
    expect(formatQuantity(0.0012345)).toBe('0.001235')
  })
})

describe('formatUsd', () => {
  it('金额千分位 + 2 位小数', () => {
    expect(formatUsd(12345.678)).toBe('12,345.68')
  })
})

describe('liquidationDistancePct', () => {
  it('正确计算距强平价百分比', () => {
    // 标记价 100，强平价 90：距离 10%
    expect(liquidationDistancePct(100, 90)).toBeCloseTo(10)
  })
  it('强平价为 0（如全仓无强平价）时返回 undefined', () => {
    expect(liquidationDistancePct(100, 0)).toBeUndefined()
  })
  it('阈值常量为 10%', () => {
    expect(LIQ_RISK_THRESHOLD_PCT).toBe(10)
  })
})

describe('downsampleLTTB', () => {
  it('点数不足时原样返回', () => {
    const data = [1, 2, 3]
    expect(downsampleLTTB(data, 10, (_, i) => i, (v) => v)).toBe(data)
  })
  it('保留首尾点', () => {
    const data = Array.from({ length: 1000 }, (_, i) => i)
    const sampled = downsampleLTTB(data, 100, (_, i) => i, (v) => v)
    expect(sampled.length).toBe(100)
    expect(sampled[0]).toBe(0)
    expect(sampled[sampled.length - 1]).toBe(999)
  })
  it('保留极值形状：单点尖峰不被丢弃', () => {
    // 构造 0..499 的数据，在中间埋一个尖峰
    const data = Array.from({ length: 500 }, (_, i) => (i === 250 ? 1000 : i % 10))
    const sampled = downsampleLTTB(data, 50, (_, i) => i, (v) => v)
    // LTTB 的三角形面积策略会优先保留尖峰
    expect(sampled.includes(1000)).toBe(true)
  })
})
