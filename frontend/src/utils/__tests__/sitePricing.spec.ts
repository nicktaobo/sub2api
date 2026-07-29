import { describe, expect, it } from 'vitest'
import {
  basePrice,
  billableIntervals,
  formatPerItem,
  formatPerMillion,
  formatRateMultiplier,
  formatTokenCount,
  hasIntervalCachePrice,
  hasTierBlock,
  isBillableInterval,
  isPerRequestMode,
  sortByOfficialOutputDesc,
  tierLabel,
  tierMatrix,
  trimPrice,
  type PriceCtx,
} from '@/utils/sitePricing'
import type { UserPricingInterval, UserPricingModel } from '@/api/channels'

const SITE: PriceCtx = { mode: 'site', rate: 1.8, fxRate: 6.8 }
const OFFICIAL: PriceCtx = { mode: 'official', rate: 1, fxRate: 1 }

function interval(overrides: Partial<UserPricingInterval> = {}): UserPricingInterval {
  return {
    min_tokens: 0,
    max_tokens: null,
    input_price: null,
    output_price: null,
    cache_write_price: null,
    cache_read_price: null,
    per_request_price: null,
    ...overrides,
  }
}

describe('trimPrice', () => {
  it('按量级选小数位:≥100 → 0 位、≥10 → 2 位、其余 4 位', () => {
    expect(trimPrice(123.456)).toBe('123')
    expect(trimPrice(12.3456)).toBe('12.35')
    expect(trimPrice(1.23456)).toBe('1.2346')
  })

  it('只剥离小数点后的零,不吃掉整数位的零(历史 trimNum bug)', () => {
    expect(trimPrice(100)).toBe('100')
    expect(trimPrice(3000)).toBe('3000')
    expect(trimPrice(250)).toBe('250')
    // 小数尾零仍然剥离
    expect(trimPrice(10)).toBe('10')
    expect(trimPrice(1.5)).toBe('1.5')
    expect(trimPrice(0)).toBe('0')
  })
})

describe('formatPerMillion', () => {
  it('site 模式用 fork 口径 (rate / fx) × 官方单价 × 1e6', () => {
    // 3e-6 × 1e6 = $3/M 官方价;本站 = 1.8 / 6.8 × 3 = 0.79411...
    expect(formatPerMillion(3e-6, SITE)).toBe('$0.7941/M')
    // 上游 modelPlaza 的 base × rate 口径会得到 $5.40 —— 明确不是这个数
    expect(formatPerMillion(3e-6, SITE)).not.toBe('$5.4/M')
  })

  it('official 模式只做 × 1e6,不乘倍率也不换汇', () => {
    expect(formatPerMillion(3e-6, OFFICIAL)).toBe('$3/M')
    expect(formatPerMillion(1.5e-5, OFFICIAL)).toBe('$15/M')
  })

  it('null/undefined 返回 -', () => {
    expect(formatPerMillion(null, SITE)).toBe('-')
    expect(formatPerMillion(undefined, OFFICIAL)).toBe('-')
  })

  it('汇率为 0 / NaN 时回落默认汇率,不产生 Infinity', () => {
    expect(formatPerMillion(3e-6, { mode: 'site', rate: 1.8, fxRate: 0 })).toBe('$0.7941/M')
    expect(formatPerMillion(3e-6, { mode: 'site', rate: 1.8, fxRate: Number.NaN })).toBe('$0.7941/M')
  })
})

describe('formatPerItem', () => {
  it('按次/按图单价不换算 1M', () => {
    // 0.04 × 1.8 / 6.8 = 0.010588...
    expect(formatPerItem(0.04, SITE)).toBe('$0.0106')
    expect(formatPerItem(0.04, OFFICIAL)).toBe('$0.04')
    expect(formatPerItem(null, SITE)).toBe('-')
  })
})

describe('basePrice', () => {
  const model: UserPricingModel = {
    name: 'm',
    input_price: 2e-6,
    official_input_price: 3e-6,
    official_output_price: 1.5e-5,
  }

  it('site 模式优先渠道价,缺失回退官方价', () => {
    expect(basePrice(model, 'input', 'site')).toBe(2e-6)
    expect(basePrice(model, 'output', 'site')).toBe(1.5e-5)
  })

  it('official 模式始终用官方价', () => {
    expect(basePrice(model, 'input', 'official')).toBe(3e-6)
    expect(basePrice(model, 'cache_write', 'official')).toBeUndefined()
  })
})

describe('isPerRequestMode / hasTierBlock', () => {
  it('per_request 与 image 视为非 token 计费', () => {
    expect(isPerRequestMode('per_request')).toBe(true)
    expect(isPerRequestMode('image')).toBe(true)
    expect(isPerRequestMode('token')).toBe(false)
    expect(isPerRequestMode(undefined)).toBe(false)
  })

  it('hasTierBlock 要求非 token 计费且有档位', () => {
    expect(hasTierBlock({ name: 'a', billing_mode: 'image', intervals: [interval()] })).toBe(true)
    expect(hasTierBlock({ name: 'a', billing_mode: 'image', intervals: [] })).toBe(false)
    expect(hasTierBlock({ name: 'a', billing_mode: 'token', intervals: [interval()] })).toBe(false)
  })
})

describe('tierMatrix', () => {
  it('tier_label 含分隔符时 pivot 成 行 × 列 矩阵', () => {
    const m = tierMatrix([
      interval({ tier_label: '1024x1024-standard', per_request_price: 0.04 }),
      interval({ tier_label: '1024x1024-hd', per_request_price: 0.08 }),
      interval({ tier_label: '1792x1024-standard', per_request_price: 0.08 }),
    ])
    expect(m).not.toBeNull()
    expect(m!.rows).toEqual(['1024x1024', '1792x1024'])
    expect(m!.cols).toEqual(['standard', 'hd'])
    expect(m!.cells['1024x1024']['hd']).toBe(0.08)
  })

  it('任一档位不含分隔符则返回 null(调用方降级成扁平表)', () => {
    expect(tierMatrix([interval({ tier_label: '1K', per_request_price: 0.01 })])).toBeNull()
    expect(tierMatrix([])).toBeNull()
    expect(tierMatrix(undefined)).toBeNull()
  })
})

describe('formatRateMultiplier', () => {
  it('1 显示 1x,≥10 取整,其余去尾零', () => {
    expect(formatRateMultiplier(1)).toBe('1x')
    expect(formatRateMultiplier(1.5)).toBe('1.5x')
    expect(formatRateMultiplier(0.4)).toBe('0.4x')
    expect(formatRateMultiplier(25)).toBe('25x')
  })
})

describe('tierLabel / formatTokenCount', () => {
  it('优先 tier_label,缺失时按 token 区间生成', () => {
    expect(tierLabel(interval({ tier_label: 'hd' }))).toBe('hd')
    expect(tierLabel(interval({ min_tokens: 0, max_tokens: 200000 }))).toBe('≤200K')
    expect(tierLabel(interval({ min_tokens: 200000, max_tokens: null }))).toBe('>200K')
    expect(tierLabel(interval({ min_tokens: 200000, max_tokens: 1000000 }))).toBe('200K–1M')
  })

  it('formatTokenCount 压缩量级', () => {
    expect(formatTokenCount(500)).toBe('500')
    expect(formatTokenCount(1500)).toBe('1.5K')
    expect(formatTokenCount(2_000_000)).toBe('2M')
  })
})

describe('sortByOfficialOutputDesc', () => {
  it('官方输出价降序,无价排最后,同价按名升序,且不改原数组', () => {
    const input: UserPricingModel[] = [
      { name: 'cheap', official_output_price: 5e-6 },
      { name: 'none' },
      { name: 'pricey', official_output_price: 7.5e-5 },
    ]
    const sorted = sortByOfficialOutputDesc(input)
    expect(sorted.map((m) => m.name)).toEqual(['pricey', 'cheap', 'none'])
    expect(input.map((m) => m.name)).toEqual(['cheap', 'none', 'pricey'])
  })
})

describe('isBillableInterval / billableIntervals', () => {
  it('任一价格字段非空即有效,与后端 filterValidIntervals 同口径', () => {
    expect(isBillableInterval(interval({ input_price: 1e-6 }))).toBe(true)
    expect(isBillableInterval(interval({ output_price: 1e-6 }))).toBe(true)
    expect(isBillableInterval(interval({ per_request_price: 0.01 }))).toBe(true)
    // 只配缓存价的区间后端照样认定有效(此时 input/output 按 0 计费)
    expect(isBillableInterval(interval({ cache_write_price: 1e-6 }))).toBe(true)
    expect(isBillableInterval(interval({ cache_read_price: 1e-7 }))).toBe(true)
  })

  it('价格全空的区间无效(前端可能建出只有 min/max 的空档)', () => {
    expect(isBillableInterval(interval({ min_tokens: 0, max_tokens: 200000 }))).toBe(false)
  })

  it('价格为 0 也算配置过(0 是显式免费,不是未配置)', () => {
    expect(isBillableInterval(interval({ input_price: 0 }))).toBe(true)
  })

  it('billableIntervals 过滤空档,null/undefined 返回空数组', () => {
    const valid = interval({ cache_read_price: 1e-7 })
    expect(billableIntervals([interval(), valid])).toEqual([valid])
    expect(billableIntervals(null)).toEqual([])
    expect(billableIntervals(undefined)).toEqual([])
  })
})

describe('hasIntervalCachePrice', () => {
  it('任一档配了写或读缓存价即为 true', () => {
    expect(hasIntervalCachePrice([interval({ input_price: 1e-6 }), interval({ cache_read_price: 1e-7 })])).toBe(true)
    expect(hasIntervalCachePrice([interval({ cache_write_price: 1e-6 })])).toBe(true)
  })

  it('一档缓存价都没配则为 false', () => {
    expect(hasIntervalCachePrice([interval({ input_price: 1e-6, output_price: 2e-6 })])).toBe(false)
    expect(hasIntervalCachePrice([])).toBe(false)
  })
})
