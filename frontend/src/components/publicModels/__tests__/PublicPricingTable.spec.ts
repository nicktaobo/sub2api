import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PublicPricingTable from '../PublicPricingTable.vue'
import type { UserPricingModel } from '@/api/channels'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const copySpy = vi.fn()
vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copied: { value: false }, copyToClipboard: copySpy }),
}))

function tokenModel(overrides: Partial<UserPricingModel> = {}): UserPricingModel {
  return {
    name: 'claude-sonnet',
    billing_mode: 'token',
    official_input_price: 3e-6,
    official_output_price: 1.5e-5,
    official_cache_write_price: 3.75e-6,
    official_cache_read_price: 3e-7,
    ...overrides,
  }
}

function mountTable(models: UserPricingModel[], rate = 1.8, fxRate = 6.8) {
  return mount(PublicPricingTable, {
    props: { models, platform: 'anthropic', rate, fxRate },
  })
}

describe('PublicPricingTable', () => {
  it('本站价走 fork 口径 (rate / fx) × 官方价,不是上游的 base × rate', () => {
    const wrapper = mountTable([tokenModel()])
    const text = wrapper.text()
    // 1.8 / 6.8 × $3/M = $0.7941/M
    expect(text).toContain('$0.7941/M')
    // 1.8 / 6.8 × $15/M = $3.9706/M
    expect(text).toContain('$3.9706/M')
    // 上游 modelPlaza 的 base × rate 会是 $5.40 / $27.00 —— 明确不能出现
    expect(text).not.toContain('$5.4')
    expect(text).not.toContain('$27')
  })

  it('官方区展示原价(不乘倍率、不换汇)', () => {
    const text = mountTable([tokenModel()]).text()
    expect(text).toContain('$3/M')
    expect(text).toContain('$15/M')
    expect(text).toContain('$3.75/M')
    expect(text).toContain('$0.3/M')
  })

  it('倍率为 1 且汇率为 1 时本站价 == 官方价', () => {
    const text = mountTable([tokenModel()], 1, 1).text()
    // 输入列本站价与官方价都是 $3/M
    expect(text.match(/\$3\/M/g)?.length).toBeGreaterThanOrEqual(2)
  })

  it('site 模式优先渠道价,渠道未配则回退官方价', () => {
    const wrapper = mountTable([tokenModel({ input_price: 6e-6 })])
    const text = wrapper.text()
    // 渠道价 6e-6 → 1.8 / 6.8 × 6 = 1.5882
    expect(text).toContain('$1.5882/M')
    // 输出未配渠道价,回退官方 1.5e-5
    expect(text).toContain('$3.9706/M')
  })

  it('模型按官方输出价从高到低排序,无官方价的排最后', () => {
    const wrapper = mountTable([
      tokenModel({ name: 'model-cheap', official_output_price: 5e-6 }),
      tokenModel({ name: 'model-no-official', official_output_price: null }),
      tokenModel({ name: 'model-expensive', official_output_price: 7.5e-5 }),
    ])
    const names = wrapper.findAll('tbody tr').map((tr) => tr.find('td button').text())
    expect(names).toEqual(['model-expensive', 'model-cheap', 'model-no-official'])
  })

  it('点击模型名复制', async () => {
    copySpy.mockClear()
    const wrapper = mountTable([tokenModel()])
    await wrapper.find('tbody td button').trigger('click')
    expect(copySpy).toHaveBeenCalledWith('claude-sonnet')
  })

  it('token 阶梯内联进输入/输出列,标签按 token 区间生成', () => {
    const wrapper = mountTable([
      tokenModel({
        intervals: [
          {
            min_tokens: 0,
            max_tokens: 200000,
            input_price: 3e-6,
            output_price: 1.5e-5,
            cache_write_price: null,
            cache_read_price: null,
            per_request_price: null,
          },
          {
            min_tokens: 200000,
            max_tokens: null,
            input_price: 6e-6,
            output_price: 3e-5,
            cache_write_price: null,
            cache_read_price: null,
            per_request_price: null,
          },
        ],
      }),
    ])
    const text = wrapper.text()
    expect(text).toContain('≤200K')
    expect(text).toContain('>200K')
    expect(text).toContain('$0.7941/M')
    expect(text).toContain('$1.5882/M')
  })

  it('按图模型 tier_label 含分隔符时渲染二维矩阵(fork 优势,上游只有扁平芯片)', () => {
    const wrapper = mountTable(
      [
        tokenModel({
          name: 'gpt-image-2',
          billing_mode: 'image',
          official_input_price: null,
          official_output_price: null,
          official_cache_write_price: null,
          official_cache_read_price: null,
          intervals: [
            {
              min_tokens: 0,
              max_tokens: null,
              tier_label: '1024x1024-standard',
              input_price: null,
              output_price: null,
              cache_write_price: null,
              cache_read_price: null,
              per_request_price: 0.04,
            },
            {
              min_tokens: 0,
              max_tokens: null,
              tier_label: '1024x1024-hd',
              input_price: null,
              output_price: null,
              cache_write_price: null,
              cache_read_price: null,
              per_request_price: 0.08,
            },
          ],
        }),
      ],
      1,
      1,
    )
    const text = wrapper.text()
    expect(text).toContain('1024x1024')
    expect(text).toContain('standard')
    expect(text).toContain('hd')
    // 倍率/汇率均为 1 → 原值
    expect(text).toContain('$0.04')
    expect(text).toContain('$0.08')
    expect(text).toContain('modelPricing.badge.image')
  })

  it('按次模型无法 pivot 时降级成阶梯芯片并带单位后缀', () => {
    const wrapper = mountTable(
      [
        tokenModel({
          name: 'web-search',
          billing_mode: 'per_request',
          official_input_price: null,
          official_output_price: null,
          official_cache_write_price: null,
          official_cache_read_price: null,
          intervals: [
            {
              min_tokens: 0,
              max_tokens: null,
              tier_label: '1K',
              input_price: null,
              output_price: null,
              cache_write_price: null,
              cache_read_price: null,
              per_request_price: 0.01,
            },
          ],
        }),
      ],
      1,
      1,
    )
    const text = wrapper.text()
    expect(text).toContain('1K')
    expect(text).toContain('$0.01')
    expect(text).toContain('publicModels.table.perUnitRequest')
    expect(text).toContain('modelPricing.badge.perRequest')
  })

  it('窄屏走 sm:hidden 紧凑列表,不套 min-w 横滑表格', () => {
    const wrapper = mountTable([tokenModel()])
    // 表格容器在 <sm 隐藏
    expect(wrapper.find('div.hidden.sm\\:block').exists()).toBe(true)
    // 紧凑列表在 ≥sm 隐藏
    expect(wrapper.find('ul.sm\\:hidden').exists()).toBe(true)
    // 上游的 min-w-[860px] 没被搬进来(去掉倍率/1h 列后 720px 够用,桌面端容器 1152px 不横滑)
    const tableClasses = wrapper.find('table').classes()
    expect(tableClasses).toContain('min-w-[720px]')
    expect(tableClasses).not.toContain('min-w-[860px]')
  })

  it('缺价字段显示 -,不显示 $NaN', () => {
    const wrapper = mountTable([
      tokenModel({
        official_cache_write_price: null,
        official_cache_read_price: null,
      }),
    ])
    expect(wrapper.text()).not.toContain('NaN')
    expect(wrapper.text()).toContain('-')
  })
})

/**
 * 展示价格必须与后端实际计费一致（model_pricing_resolver.go）：
 *   - :143-166 模型有 intervals 时提前 return，扁平字段根本不参与计费；
 *   - :255+ intervalToModelPricing 缓存价只认 iv.CacheWritePrice / iv.CacheReadPrice；
 *   - :229-241 filterValidIntervals 认定「只配缓存价」的区间同样有效。
 */
describe('PublicPricingTable —— 展示价与实际计费对齐', () => {
  function tieredCacheModel() {
    return tokenModel({
      // 扁平 + 官方缓存价都在，但有 intervals ⇒ 二者都不参与计费
      cache_write_price: 3.75e-6,
      cache_read_price: 3e-7,
      intervals: [
        {
          min_tokens: 0,
          max_tokens: 200000,
          input_price: 3e-6,
          output_price: 1.5e-5,
          cache_write_price: 1e-6,
          cache_read_price: 1e-7,
          per_request_price: null,
        },
        {
          min_tokens: 200000,
          max_tokens: null,
          input_price: 6e-6,
          output_price: 3e-5,
          cache_write_price: 2e-6,
          cache_read_price: null,
          per_request_price: null,
        },
      ],
    })
  }

  it('B1: 阶梯模型的缓存列按档位取值,不回落扁平/官方缓存价', () => {
    const text = mountTable([tieredCacheModel()], 1, 1).text()
    // 档位缓存价（倍率/汇率均为 1 → 原值）
    expect(text).toContain('$1/M') // 档1 写 1e-6
    expect(text).toContain('$0.1/M') // 档1 读 1e-7
    expect(text).toContain('$2/M') // 档2 写 2e-6
    // 扁平缓存价 3.75e-6 / 3e-7 永不生效 —— 本站(实付)列不得出现
    const siteCell = mountTable([tieredCacheModel()], 1, 1).findAll('tbody td')[3]
    expect(siteCell.text()).not.toContain('$3.75/M')
    expect(siteCell.text()).not.toContain('$0.3/M')
  })

  it('B1: 缓存列逐档带档位标签,与输入/输出列同一批档位', () => {
    const cacheCell = mountTable([tieredCacheModel()], 1, 1).findAll('tbody td')[3]
    expect(cacheCell.text()).toContain('≤200K')
    expect(cacheCell.text()).toContain('>200K')
  })

  it('B1: 档位未配缓存价显示 -,不冒充 0 也不借用扁平价', () => {
    const cacheCell = mountTable([tieredCacheModel()], 1, 1).findAll('tbody td')[3]
    // 档2 读价为 null → '-'
    expect(cacheCell.text()).toContain('-')
    expect(cacheCell.text()).not.toContain('$0.3/M')
  })

  it('B1: 所有档位都没配缓存价时整列显示 -,不铺空行', () => {
    const cacheCell = mountTable(
      [
        tokenModel({
          cache_write_price: 3.75e-6,
          cache_read_price: 3e-7,
          intervals: [
            {
              min_tokens: 0,
              max_tokens: null,
              input_price: 3e-6,
              output_price: 1.5e-5,
              cache_write_price: null,
              cache_read_price: null,
              per_request_price: null,
            },
          ],
        }),
      ],
      1,
      1,
    ).findAll('tbody td')[3]
    expect(cacheCell.text().trim()).toBe('-')
  })

  it('B1: 窄屏紧凑列表同样逐档取缓存价,不回落扁平价', () => {
    const mobile = mountTable([tieredCacheModel()], 1, 1).find('ul.sm\\:hidden')
    expect(mobile.text()).toContain('$1/M')
    expect(mobile.text()).toContain('$0.1/M')
    expect(mobile.text()).not.toContain('$3.75/M')
  })

  it('B1: 无区间的模型仍展示扁平缓存价(回归保护)', () => {
    const cacheCell = mountTable([tokenModel({ cache_write_price: 3.75e-6 })], 1, 1).findAll('tbody td')[3]
    expect(cacheCell.text()).toContain('$3.75/M')
  })

  it('B2: 只配缓存价的区间同样有效,输入/输出不得回落扁平/官方价', () => {
    const wrapper = mountTable(
      [
        tokenModel({
          input_price: 3e-6,
          output_price: 1.5e-5,
          intervals: [
            {
              min_tokens: 0,
              max_tokens: null,
              input_price: null,
              output_price: null,
              cache_write_price: 1e-6,
              cache_read_price: 1e-7,
              per_request_price: null,
            },
          ],
        }),
      ],
      1,
      1,
    )
    const cells = wrapper.findAll('tbody td')
    // 后端此时按区间计费,input/output 按 0 收 —— 扁平 3e-6 / 1.5e-5 是多报价
    expect(cells[1].text()).not.toContain('$3/M')
    expect(cells[2].text()).not.toContain('$15/M')
    // 缓存价按档位展示
    expect(cells[3].text()).toContain('$1/M')
    expect(cells[3].text()).toContain('$0.1/M')
    // 官方参考列不受影响,仍是 LiteLLM 原价
    expect(cells[4].text()).toContain('$3/M')
    expect(cells[5].text()).toContain('$15/M')
  })

  it('B2: 价格字段全空的区间视为无效,仍走扁平价(与后端 filterValidIntervals 一致)', () => {
    const cells = mountTable(
      [
        tokenModel({
          input_price: 3e-6,
          intervals: [
            {
              min_tokens: 0,
              max_tokens: 200000,
              input_price: null,
              output_price: null,
              cache_write_price: null,
              cache_read_price: null,
              per_request_price: null,
            },
          ],
        }),
      ],
      1,
      1,
    ).findAll('tbody td')
    expect(cells[1].text()).toContain('$3/M')
    expect(cells[1].text()).not.toContain('≤200K')
  })
})
