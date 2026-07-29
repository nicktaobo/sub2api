/**
 * 公开页（模型广场）三条本地能力的回归护栏：
 *
 *   A1 搜索在展开态被抹掉 —— 展开的价格表必须吃搜索过滤后的模型，且不截断
 *   A2 「起价」对按次 / 图片分组撒谎 —— 已公布价格的分组不能显示「暂未公布价格」
 *   A3 阶梯 token 模型的「起价」与正下方表格自相矛盾 —— 起价取各档最低输入价
 *
 * 断言刻意打在渲染结果上（表格行数 / 卡片文案），而不是内部函数：
 * 缺陷本身出在模板 wiring（`:models="group.models"`），只测纯函数抓不到。
 */
import { describe, expect, it, beforeEach, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { UserPricingGroup, UserPricingModel } from '@/api/channels'

const { getPublicPricingGroups } = vi.hoisted(() => ({ getPublicPricingGroups: vi.fn() }))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      // 保留插值，起价文案要能断言出具体价格
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}(${Object.values(params).join('|')})` : key,
      locale: { value: 'zh' },
    }),
  }
})

vi.mock('@unhead/vue', () => ({ useHead: vi.fn() }))

vi.mock('@/api/channels', () => ({
  default: { getPublicPricingGroups },
  getPublicPricingGroups,
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => ({ isAuthenticated: false, isAdmin: false, checkAuth: vi.fn() }),
  useAppStore: () => ({
    cachedPublicSettings: null,
    siteName: 'Sub2API',
    siteLogo: '',
    publicSettingsLoaded: true,
    fetchPublicSettings: vi.fn(),
  }),
  useMerchantStore: () => ({ isMerchantSite: false, siteName: '', siteLogo: '' }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copied: { value: false }, copyToClipboard: vi.fn() }),
}))

vi.mock('@/composables/useFxRate', async () => {
  const { ref } = await vi.importActual<typeof import('vue')>('vue')
  const fxRate = ref(6.8)
  return {
    useFxRate: () => ({ fxRate, fxLoaded: ref(true), ensureFxRate: vi.fn().mockResolvedValue(6.8) }),
  }
})

import PublicModelsView from '../PublicModelsView.vue'

function tokenModel(name: string, inputPrice: number | null = 3e-6): UserPricingModel {
  return {
    name,
    billing_mode: 'token',
    input_price: inputPrice,
    output_price: inputPrice == null ? null : inputPrice * 5,
    official_output_price: inputPrice == null ? null : inputPrice * 5,
  }
}

function group(overrides: Partial<UserPricingGroup> = {}): UserPricingGroup {
  return {
    id: 1,
    name: 'shared-pool',
    platform: 'anthropic',
    rate_multiplier: 1.8,
    models: [],
    ...overrides,
  } as UserPricingGroup
}

async function mountView(groups: UserPricingGroup[]) {
  getPublicPricingGroups.mockResolvedValue(groups)
  const wrapper = mount(PublicModelsView, {
    global: {
      stubs: {
        RouterLink: { template: '<a><slot /></a>' },
        LocaleSwitcher: true,
        Icon: true,
        PlatformIcon: true,
      },
    },
  })
  await flushPromises()
  return wrapper
}

/** 展开指定分组卡片（点卡片底部的展开按钮）。 */
async function expandCard(wrapper: ReturnType<typeof mount>, index = 0) {
  const toggles = wrapper.findAll('button.expand-toggle')
  await toggles[index].trigger('click')
  await flushPromises()
}

async function search(wrapper: ReturnType<typeof mount>, q: string) {
  await wrapper.find('input[type="search"]').setValue(q)
  await flushPromises()
}

/** 展开态价格表里的模型名（桌面表格首列按钮）。 */
function tableModelNames(wrapper: ReturnType<typeof mount>): string[] {
  return wrapper.findAll('.public-pricing tbody tr').map((tr) => tr.find('td button').text())
}

beforeEach(() => {
  getPublicPricingGroups.mockReset()
})

describe('PublicModelsView —— A1 展开态尊重搜索', () => {
  const manyModels = [
    ...Array.from({ length: 20 }, (_, i) => tokenModel(`filler-model-${i}`)),
    tokenModel('claude-opus-4'),
  ]

  it('搜到某个模型再展开,表格只列命中项(不是全量)', async () => {
    const wrapper = await mountView([group({ models: manyModels })])
    await search(wrapper, 'opus')
    await expandCard(wrapper)

    expect(tableModelNames(wrapper)).toEqual(['claude-opus-4'])
    // 顺带说明「其余的去哪了」，否则用户以为分组只剩一个模型
    expect(wrapper.text()).toContain(`publicModels.searchFilteredHint(1|${manyModels.length})`)
  })

  it('无搜索时展开表格不截断,MAX_MODELS_PER_CARD 只管折叠态 chip', async () => {
    const wrapper = await mountView([group({ models: manyModels })])
    // 折叠态 chip 截断到 12 个 + 一个 "+N" 提示
    expect(wrapper.findAll('button.model-chip')).toHaveLength(12)
    expect(wrapper.text()).toContain('+9')

    await expandCard(wrapper)
    expect(tableModelNames(wrapper)).toHaveLength(manyModels.length)
  })

  it('搜索词命中分组名 / 平台名时整组放行,表格不会变成空表', async () => {
    const wrapper = await mountView([group({ models: manyModels })])
    await search(wrapper, 'shared-pool')
    await expandCard(wrapper)

    expect(tableModelNames(wrapper)).toHaveLength(manyModels.length)
  })

  it('搜索词命中平台名时同样整组放行(卡片在列表里就不该展开出空表)', async () => {
    const wrapper = await mountView([group({ platform: 'anthropic', models: manyModels })])
    await search(wrapper, 'anthropic')
    await expandCard(wrapper)

    expect(tableModelNames(wrapper)).toHaveLength(manyModels.length)
    // 没有过滤发生，就别打「已按搜索过滤」的旗号
    expect(wrapper.text()).not.toContain('publicModels.searchFilteredHint')
  })

  /**
   * 不变量：能留在列表里的分组，展开后必定有内容。
   * groupsBeforeRate 放行分组的三个理由（组名 / 平台名 / 任一模型名命中）
   * 必须都被 expandedModels 认账，少认一个就会渲染出空表。
   */
  it('列表里的每个分组展开后都不是空表(组名 / 平台名 / 模型名三种命中路径)', async () => {
    const groups = [
      group({ id: 1, name: 'alpha-pool', platform: 'anthropic', models: [tokenModel('claude-opus-4')] }),
      group({ id: 2, name: 'beta-pool', platform: 'openai', models: [tokenModel('alpha-tuned-model')] }),
    ]
    for (const q of ['alpha', 'anthropic', 'claude', 'pool']) {
      const wrapper = await mountView(groups)
      await search(wrapper, q)
      const cards = wrapper.findAll('article.group-card')
      expect(cards.length, `搜索「${q}」应至少留下一个分组`).toBeGreaterThan(0)
      for (let i = 0; i < cards.length; i += 1) {
        await expandCard(wrapper, i)
        expect(tableModelNames(wrapper).length, `搜索「${q}」展开第 ${i} 组不应是空表`).toBeGreaterThan(0)
        expect(wrapper.text()).not.toContain('publicModels.groupSearchEmpty')
        await expandCard(wrapper, i) // 收起，避免影响下一轮
      }
    }
  })
})

describe('PublicModelsView —— A2 起价不再对按次 / 图片分组撒谎', () => {
  const imageModel: UserPricingModel = {
    name: 'gpt-image-2',
    billing_mode: 'image',
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
    ],
  }

  it('全是图片模型的分组不显示「暂未公布价格」,而是按图起价', async () => {
    const wrapper = await mountView([group({ models: [imageModel] })])
    const text = wrapper.text()

    expect(text).not.toContain('publicModels.noPricing')
    // 1.8 / 6.8 × 0.04 = 0.010588…
    expect(text).toContain('publicModels.fromPriceImage($0.0106)')
  })

  it('全是按次模型的分组走按次文案', async () => {
    const wrapper = await mountView([
      group({
        models: [{ name: 'web-search', billing_mode: 'per_request', per_request_price: 0.01 }],
      }),
    ])
    const text = wrapper.text()

    expect(text).not.toContain('publicModels.noPricing')
    // 1.8 / 6.8 × 0.01 = 0.0026470…
    expect(text).toContain('publicModels.fromPriceRequest($0.0026)')
  })

  it('只配阶梯价、模型级 input_price 为 nil 的分组同样有起价', async () => {
    const wrapper = await mountView([
      group({
        models: [
          {
            name: 'claude-sonnet-4',
            billing_mode: 'token',
            input_price: null,
            output_price: null,
            intervals: [
              {
                min_tokens: 0,
                max_tokens: 200_000,
                input_price: 3e-6,
                output_price: 1.5e-5,
                cache_write_price: null,
                cache_read_price: null,
                per_request_price: null,
              },
            ],
          },
        ],
      }),
    ])

    expect(wrapper.text()).not.toContain('publicModels.noPricing')
  })

  it('确实一个价都没配的分组仍然显示「暂未公布价格」', async () => {
    const wrapper = await mountView([group({ models: [{ name: 'mystery-model' }] })])
    expect(wrapper.text()).toContain('publicModels.noPricing')
  })
})

describe('PublicModelsView —— A3 起价与正下方表格口径一致', () => {
  const tiered: UserPricingModel = {
    name: 'claude-sonnet-4',
    billing_mode: 'token',
    // 陷阱：扁平价存在但后端命中 intervals 后根本不参与计费
    input_price: 1e-5,
    output_price: 5e-5,
    intervals: [
      {
        min_tokens: 0,
        max_tokens: 200_000,
        input_price: 3e-6,
        output_price: 1.5e-5,
        cache_write_price: null,
        cache_read_price: null,
        per_request_price: null,
      },
      {
        min_tokens: 200_000,
        max_tokens: null,
        input_price: 6e-6,
        output_price: 3e-5,
        cache_write_price: null,
        cache_read_price: null,
        per_request_price: null,
      },
    ],
  }

  it('起价取各档最低输入价,且这个数字在展开表格里真实出现', async () => {
    const wrapper = await mountView([group({ models: [tiered] })])

    // 1.8 / 6.8 × $3/M = $0.7941/M —— 最低档
    expect(wrapper.text()).toContain('publicModels.fromPrice($0.7941/M)')
    // 扁平价 1e-5 会算出 $2.6471/M，那是个用户永远付不到的价
    expect(wrapper.text()).not.toContain('publicModels.fromPrice($2.6471/M)')

    await expandCard(wrapper)
    const tableText = wrapper.find('.public-pricing').text()
    expect(tableText).toContain('$0.7941/M')
    expect(tableText).toContain('$1.5882/M')
  })
})
