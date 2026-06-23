import { describe, it, expect } from 'vitest'
import { costSymbol, formatTokenPricePerMillion } from '@/utils/usagePricing'

describe('costSymbol', () => {
  it('国产人民币计价(CNY)返回 ¥，其余返回 $', () => {
    expect(costSymbol('CNY')).toBe('¥')
    expect(costSymbol('USD')).toBe('$')
    expect(costSymbol(undefined)).toBe('$')
    expect(costSymbol(null)).toBe('$')
    expect(costSymbol('')).toBe('$')
  })
})

describe('formatTokenPricePerMillion currencySymbol', () => {
  it('默认 $，可传 ¥', () => {
    // 6.5 / 1M tokens * 1M = 6.5 单价
    expect(formatTokenPricePerMillion(6.5, 1_000_000, { currencySymbol: '¥' })).toBe('¥6.5000')
    expect(formatTokenPricePerMillion(3, 1_000_000)).toBe('$3.0000')
  })

  it('withCurrencySymbol=false 时不带符号', () => {
    expect(formatTokenPricePerMillion(3, 1_000_000, { withCurrencySymbol: false })).toBe('3.0000')
  })

  it('无效输入返回 emptyValue', () => {
    expect(formatTokenPricePerMillion(1, 0, { currencySymbol: '¥' })).toBe('-')
  })
})
