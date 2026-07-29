/**
 * 本站价（site price）口径的唯一真源。
 *
 * fork 的计费链路是 `actualCost = totalCost × RateMultiplier`，展示侧再按当前
 * CNY/USD 汇率折回美元，所以本站价恒等于：
 *
 *     sitePerM = (group.rate_multiplier / fxRate) × officialPerToken × 1e6
 *
 * 登录页 `views/user/ModelPricingView.vue` 与公开页 `views/public/PublicModelsView.vue`
 * 必须共用本文件，否则同一模型会在两页显示相差一个汇率倍数（约 7 倍）的数字。
 *
 * 注意：上游 `components/modelPlaza/*` 用的是 `base × effectiveRate`（不做汇率换算），
 * 那是另一套口径，不要往本文件里搬。
 */

import type { UserPricingInterval, UserPricingModel } from '@/api/channels'
import { BILLING_MODE_IMAGE, BILLING_MODE_PER_REQUEST } from '@/constants/channel'
import { DEFAULT_CNY_PER_USD } from '@/utils/pricing'

/** 'site' = 本站实付价（乘倍率 / 汇率）；'official' = LiteLLM 官方参考价（原值）。 */
export type PriceMode = 'site' | 'official'

export interface PriceCtx {
  mode: PriceMode
  /** 分组倍率。official 模式下不参与计算，传 1 即可。 */
  rate: number
  /** CNY per USD。official 模式下不参与计算，传 1 即可。 */
  fxRate: number
}

/** tier_label 里的行/列分隔符（如 `1024x1024-hd` → 行 `1024x1024`、列 `hd`）。 */
export const TIER_SEP = '-'

export const PER_MILLION = 1_000_000

export type TierMatrixData = {
  rows: string[]
  cols: string[]
  cells: Record<string, Record<string, number | null | undefined>>
}

/**
 * trimPrice 按量级选小数位后去掉无意义的尾零：
 *   ≥100 → 0 位、≥10 → 2 位、其余 → 4 位。
 *
 * 注意：只剥离小数点之后的零。历史实现用 `/\.?0+$/` 一刀切，会把整数位的零也吃掉
 * （100 → "1"、3000 → "3"），此处已修正。
 */
export function trimPrice(n: number): string {
  if (!Number.isFinite(n) || n === 0) return '0'
  const digits = n >= 100 ? 0 : n >= 10 ? 2 : 4
  const fixed = n.toFixed(digits)
  if (!fixed.includes('.')) return fixed
  const trimmed = fixed.replace(/0+$/, '').replace(/\.$/, '')
  return trimmed || '0'
}

/** 有效汇率：后端未下发或异常值时回落到默认汇率，避免除零得到 Infinity。 */
function safeFx(fxRate: number): number {
  return Number.isFinite(fxRate) && fxRate > 0 ? fxRate : DEFAULT_CNY_PER_USD
}

/**
 * formatPerMillion 把 per-token 美元价格式化成 `$x/M`。
 *   - official：`perTokenUSD × 1e6`
 *   - site    ：`(rate / fx) × perTokenUSD × 1e6`
 */
export function formatPerMillion(perTokenUSD: number | null | undefined, ctx: PriceCtx): string {
  if (perTokenUSD == null) return '-'
  const officialPerM = perTokenUSD * PER_MILLION
  if (ctx.mode === 'official') return `$${trimPrice(officialPerM)}/M`
  return `$${trimPrice((ctx.rate / safeFx(ctx.fxRate)) * officialPerM)}/M`
}

/** formatPerItem：按次 / 按图单价（不换算 1M），口径同 formatPerMillion。 */
export function formatPerItem(perItemUSD: number | null | undefined, ctx: PriceCtx): string {
  if (perItemUSD == null) return '-'
  if (ctx.mode === 'official') return `$${trimPrice(perItemUSD)}`
  return `$${trimPrice((ctx.rate / safeFx(ctx.fxRate)) * perItemUSD)}`
}

/**
 * basePrice 选取展示用的基础单价（per-token USD）。
 *   - official 模式：始终用 LiteLLM 官方价（official_*）
 *   - site     模式：优先渠道管理员配的 channel 单价，未配置回退到 official
 * 两类基础单价语义一致，后续由 formatPerMillion 统一乘 rate / fx。
 */
export function basePrice(
  model: UserPricingModel,
  field: 'input' | 'output' | 'cache_write' | 'cache_read',
  mode: PriceMode,
): number | null | undefined {
  const officialKey = `official_${field}_price` as const
  if (mode === 'official') return model[officialKey]
  const siteKey = `${field}_price` as const
  return model[siteKey] ?? model[officialKey]
}

/**
 * isBillableInterval 与后端 `filterValidIntervals`
 * （backend/internal/service/model_pricing_resolver.go:229）逐字对齐：
 * **任意一个**价格字段非空，该区间即生效。
 *
 * 判定必须与后端完全一致，否则会出现「后端已按区间计费、前端还在展示扁平价」：
 * applyTokenOverrides 在存在有效区间时直接 return（:148-166），扁平字段
 * （input_price/output_price/cache_*）根本不参与计费；只配了缓存价的区间同样有效，
 * 此时 input/output 按 0 计费——若前端在这种模型上回落展示扁平/官方 input/output，
 * 方向是**多报价**。
 */
export function isBillableInterval(iv: UserPricingInterval): boolean {
  return (
    iv.input_price != null ||
    iv.output_price != null ||
    iv.cache_write_price != null ||
    iv.cache_read_price != null ||
    iv.per_request_price != null
  )
}

/** billableIntervals：与后端 filterValidIntervals 同口径的有效区间列表（永不返回 null）。 */
export function billableIntervals(
  intervals: UserPricingInterval[] | null | undefined,
): UserPricingInterval[] {
  return (intervals ?? []).filter(isBillableInterval)
}

/** hasIntervalCachePrice：区间里是否有任何一档配了缓存价（都没配则整列无缓存可展示）。 */
export function hasIntervalCachePrice(intervals: UserPricingInterval[]): boolean {
  return intervals.some((iv) => iv.cache_write_price != null || iv.cache_read_price != null)
}

/** isPerRequestMode 判定「按次 / 按图」计费（非 token 计费）。 */
export function isPerRequestMode(billingMode?: string | null): boolean {
  return billingMode === BILLING_MODE_PER_REQUEST || billingMode === BILLING_MODE_IMAGE
}

/** hasTierBlock：按次 / 按图且配了档位，需要额外渲染阶梯区块。 */
export function hasTierBlock(model: UserPricingModel): boolean {
  return isPerRequestMode(model.billing_mode) && (model.intervals?.length ?? 0) > 0
}

/** minPositive：取一组价里最小的正数；全空 / 全非正返回 null。 */
function minPositive(values: Array<number | null | undefined>): number | null {
  let min: number | null = null
  for (const v of values) {
    if (v == null || !Number.isFinite(v) || !(v > 0)) continue
    if (min == null || v < min) min = v
  }
  return min
}

/**
 * minTokenInputPrice：token 模型的最低输入基础价（起价用）。
 *
 * 有有效区间时**只看区间价**，绝不回退模型级扁平价：后端
 * `model_pricing_resolver.go:143-166` 命中 intervals 后直接 return，扁平 input_price
 * 根本不参与计费。展示一个永不生效的价，等于对用户报错价。
 * 「有没有区间」用 billableIntervals 判定，与后端 filterValidIntervals 同口径——
 * 只配了缓存价的区间同样把扁平价踢出计费，此时输入按 0 计费、没有可宣传的起价。
 */
export function minTokenInputPrice(model: UserPricingModel): number | null {
  if (isPerRequestMode(model.billing_mode)) return null
  const ivs = billableIntervals(model.intervals)
  if (ivs.length) return minPositive(ivs.map((iv) => iv.input_price))
  return minPositive([basePrice(model, 'input', 'site')])
}

/** minPerItemPrice：按次 / 按图模型的最低单价，同样是「有区间只看区间」。 */
export function minPerItemPrice(model: UserPricingModel): number | null {
  if (!isPerRequestMode(model.billing_mode)) return null
  const ivs = billableIntervals(model.intervals)
  if (ivs.length) return minPositive(ivs.map((iv) => iv.per_request_price))
  return minPositive([model.per_request_price])
}

/**
 * 起价的计费形态。token 是 $/1M，per_request / image 是 $/次、$/张——
 * 三者单位不同，不可比较，所以卡片文案必须按形态分开写，不能都塞进「输入低至」。
 */
export type StartingPriceKind = 'token' | 'per_request' | 'image'

export interface StartingPrice {
  kind: StartingPriceKind
  /** 基础单价（USD，未乘倍率 / 未换汇），由 formatPerMillion / formatPerItem 渲染。 */
  value: number
}

/**
 * groupStartingPrice：分组卡片的「起价」。
 *
 * token 价优先（可比性最好）；整组一个 token 价都没有时才回落到按次 / 按图单价，
 * 这样「全是图片模型」的分组不会因为没有 token 价就被判成「暂未公布价格」。
 */
export function groupStartingPrice(models: UserPricingModel[]): StartingPrice | null {
  const tokenMin = minPositive(models.map((m) => minTokenInputPrice(m)))
  if (tokenMin != null) return { kind: 'token', value: tokenMin }

  let itemMin: number | null = null
  let kind: StartingPriceKind = 'per_request'
  for (const m of models) {
    const p = minPerItemPrice(m)
    if (p == null) continue
    if (itemMin == null || p < itemMin) {
      itemMin = p
      kind = m.billing_mode === BILLING_MODE_IMAGE ? 'image' : 'per_request'
    }
  }
  return itemMin == null ? null : { kind, value: itemMin }
}

/**
 * hasPublishedPrice：分组里是否存在任何可展示的价格（含缓存价 / 档位价 / 按次价）。
 *
 * 起价算不出来（例如只配了缓存价，或价格恰为 0）时用它兜底：只有它也为 false，
 * 才允许对用户说「暂未公布价格」。宁可留白，也不能对已公布价格的分组撒谎。
 */
export function hasPublishedPrice(models: UserPricingModel[]): boolean {
  return models.some((m) => {
    if (billableIntervals(m.intervals).length > 0) return true
    if (isPerRequestMode(m.billing_mode)) return m.per_request_price != null
    return (
      basePrice(m, 'input', 'site') != null ||
      basePrice(m, 'output', 'site') != null ||
      basePrice(m, 'cache_write', 'site') != null ||
      basePrice(m, 'cache_read', 'site') != null
    )
  })
}

/**
 * tierMatrix 把 `tier_label` 形如 `<行>-<列>` 的档位 pivot 成二维矩阵
 * （典型场景：图片模型的 尺寸 × 质量）。任一档位不含分隔符即返回 null，
 * 调用方降级成扁平档位表。
 */
export function tierMatrix(intervals: UserPricingInterval[] | null | undefined): TierMatrixData | null {
  const ivs = intervals ?? []
  if (ivs.length === 0) return null
  const rows: string[] = []
  const cols: string[] = []
  const cells: Record<string, Record<string, number | null | undefined>> = {}
  for (const iv of ivs) {
    const label = (iv.tier_label ?? '').trim()
    const sepIdx = label.indexOf(TIER_SEP)
    if (sepIdx <= 0 || sepIdx >= label.length - 1) return null
    const row = label.slice(0, sepIdx).trim()
    const col = label.slice(sepIdx + 1).trim()
    if (!row || !col) return null
    if (!rows.includes(row)) rows.push(row)
    if (!cols.includes(col)) cols.push(col)
    if (!cells[row]) cells[row] = {}
    cells[row][col] = iv.per_request_price
  }
  return { rows, cols, cells }
}

/**
 * formatRateMultiplier 展示分组的计费倍率，例如 1.8x、0.4x、25x。
 * 保留 3 位小数后去掉无意义零（1.500 → 1.5）。
 */
export function formatRateMultiplier(rate: number): string {
  const r = Number(rate || 1)
  if (Math.abs(r - 1) < 1e-6) return '1x'
  if (r >= 10) return `${r.toFixed(0)}x`
  return `${parseFloat(r.toFixed(3))}x`
}

/** formatTokenCount 把 token 数压成 1.2K / 200K / 1M 形式。 */
export function formatTokenCount(n: number): string {
  if (n >= 1_000_000) return `${Math.round((n / 1_000_000) * 100) / 100}M`
  if (n >= 1_000) return `${Math.round((n / 1_000) * 100) / 100}K`
  return String(n)
}

/**
 * tierLabel 档位标签：优先管理员配置的 tier_label，缺失时按 token 区间生成
 * （≤200K / >200K / 200K–1M）。
 */
export function tierLabel(iv: UserPricingInterval): string {
  if (iv.tier_label) return iv.tier_label
  const { min_tokens: min, max_tokens: max } = iv
  if (max == null) return `>${formatTokenCount(min)}`
  if (min === 0) return `≤${formatTokenCount(max)}`
  return `${formatTokenCount(min)}–${formatTokenCount(max)}`
}

/** 展示顺序：官方输出价从高到低；无官方价的排最后；同价按名称升序。不改原数组。 */
export function sortByOfficialOutputDesc(models: UserPricingModel[]): UserPricingModel[] {
  return [...models].sort((a, b) => {
    const pa = a.official_output_price ?? null
    const pb = b.official_output_price ?? null
    if (pa != null && pb != null && pa !== pb) return pb - pa
    if (pa != null && pb == null) return -1
    if (pa == null && pb != null) return 1
    return a.name.localeCompare(b.name)
  })
}
