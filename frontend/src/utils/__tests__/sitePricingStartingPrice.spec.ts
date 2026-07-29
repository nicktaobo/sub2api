/**
 * 公开页分组卡片「起价」的口径回归。
 *
 * 三条历史缺陷都钉在这里：
 *   A2 起价跳过所有按次 / 图片模型 → 全是图片模型的分组被判成「暂未公布价格」
 *   A2 起价要求模型级 input_price 非空 → 只配阶梯价的分组同样被判成「暂未公布」
 *   A3 起价忽略 intervals 直接读扁平价 → 卡片数字与正下方表格自相矛盾
 */
import { describe, expect, it } from 'vitest'
import {
  groupStartingPrice,
  hasPublishedPrice,
  minPerItemPrice,
  minTokenInputPrice,
} from '@/utils/sitePricing'
import type { UserPricingInterval, UserPricingModel } from '@/api/channels'

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

/** 有阶梯价、但模型级扁平价为 nil 的 token 模型（真实线上形态）。 */
const tieredTokenModel: UserPricingModel = {
  name: 'claude-sonnet-4',
  billing_mode: 'token',
  input_price: null,
  output_price: null,
  intervals: [
    interval({ min_tokens: 0, max_tokens: 200_000, input_price: 3e-6, output_price: 1.5e-5 }),
    interval({ min_tokens: 200_000, max_tokens: null, input_price: 6e-6, output_price: 3e-5 }),
  ],
}

const imageModel: UserPricingModel = {
  name: 'gpt-image-2',
  billing_mode: 'image',
  intervals: [
    interval({ tier_label: '1024x1024-standard', per_request_price: 0.04 }),
    interval({ tier_label: '1024x1024-hd', per_request_price: 0.08 }),
  ],
}

const perRequestModel: UserPricingModel = {
  name: 'web-search',
  billing_mode: 'per_request',
  per_request_price: 0.01,
}

describe('minTokenInputPrice', () => {
  // A3：起价必须与展开表格逐档展示的数字同源
  it('有 intervals 时取各档最低输入价,不回退永不生效的扁平价', () => {
    expect(minTokenInputPrice(tieredTokenModel)).toBe(3e-6)
  })

  it('扁平价存在也不能盖过 intervals —— 后端命中区间后直接 return,扁平字段不参与计费', () => {
    const trap: UserPricingModel = { ...tieredTokenModel, input_price: 1e-9 }
    // 若实现回退扁平价会得到 1e-9(比任何档位都低),那是个用户永远付不到的价
    expect(minTokenInputPrice(trap)).toBe(3e-6)
  })

  it('只配了缓存价的区间同样把扁平价踢出计费,此时没有可宣传的输入起价', () => {
    const cacheOnly: UserPricingModel = {
      name: 'cache-only',
      billing_mode: 'token',
      input_price: 5e-6,
      intervals: [interval({ cache_write_price: 1e-6, cache_read_price: 1e-7 })],
    }
    expect(minTokenInputPrice(cacheOnly)).toBeNull()
  })

  it('无 intervals 时用扁平价,缺失回退官方价', () => {
    expect(minTokenInputPrice({ name: 'a', input_price: 2e-6, official_input_price: 3e-6 })).toBe(2e-6)
    expect(minTokenInputPrice({ name: 'b', official_input_price: 3e-6 })).toBe(3e-6)
    expect(minTokenInputPrice({ name: 'c' })).toBeNull()
  })

  it('按次 / 按图模型不产出 token 起价', () => {
    expect(minTokenInputPrice(imageModel)).toBeNull()
    expect(minTokenInputPrice(perRequestModel)).toBeNull()
  })
})

describe('minPerItemPrice', () => {
  it('取各档最低按次价', () => {
    expect(minPerItemPrice(imageModel)).toBe(0.04)
  })

  it('无档位时用模型级按次价', () => {
    expect(minPerItemPrice(perRequestModel)).toBe(0.01)
  })

  it('token 模型不产出按次起价', () => {
    expect(minPerItemPrice(tieredTokenModel)).toBeNull()
  })
})

describe('groupStartingPrice', () => {
  // A3
  it('阶梯 token 模型的起价 = 各档最低输入价(与展开表格同源)', () => {
    expect(groupStartingPrice([tieredTokenModel])).toEqual({ kind: 'token', value: 3e-6 })
  })

  // A2：全是图片模型的分组，历史实现返回 null → 模板落到「暂未公布价格」
  it('全是图片模型的分组照样有起价,形态标记为 image', () => {
    expect(groupStartingPrice([imageModel])).toEqual({ kind: 'image', value: 0.04 })
  })

  // A2
  it('全是按次模型的分组照样有起价,形态标记为 per_request', () => {
    expect(groupStartingPrice([perRequestModel])).toEqual({ kind: 'per_request', value: 0.01 })
  })

  it('混合分组优先给 token 起价($/1M 与 $/次不可比,token 更有参考价值)', () => {
    expect(groupStartingPrice([imageModel, tieredTokenModel])).toEqual({
      kind: 'token',
      value: 3e-6,
    })
  })

  it('一个价都没配才返回 null', () => {
    expect(groupStartingPrice([{ name: 'bare' }])).toBeNull()
    expect(groupStartingPrice([])).toBeNull()
  })
})

describe('hasPublishedPrice', () => {
  it('只配了缓存价 / 只配了阶梯价的分组都算已公布价格', () => {
    expect(
      hasPublishedPrice([
        {
          name: 'cache-only',
          billing_mode: 'token',
          intervals: [interval({ cache_write_price: 1e-6 })],
        },
      ]),
    ).toBe(true)
    expect(hasPublishedPrice([tieredTokenModel])).toBe(true)
    expect(hasPublishedPrice([imageModel])).toBe(true)
  })

  it('确实一个价都没有才为 false', () => {
    expect(hasPublishedPrice([{ name: 'bare' }, { name: 'bare2', billing_mode: 'image' }])).toBe(false)
  })
})
