<template>
  <div class="public-pricing" :style="accentStyle">
    <!-- ≥sm：实付 / 官方 双区表格。宽度收在 720px 内，公开页容器(max-w-6xl)桌面端不横滑。 -->
    <div class="hidden overflow-x-auto sm:block">
      <table class="w-full min-w-[720px] table-fixed border-collapse text-sm tabular-nums">
        <colgroup>
          <col class="w-[24%]" />
          <col class="w-[11%]" />
          <col class="w-[11%]" />
          <col class="w-[16%]" />
          <col class="w-[11%]" />
          <col class="w-[11%]" />
          <col class="w-[16%]" />
        </colgroup>
        <thead>
          <tr class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400">
            <th
              rowspan="2"
              class="border-r border-gray-100 py-2.5 pr-4 text-left align-middle dark:border-dark-700/60"
            >
              {{ t('modelPricing.columns.model') }}
            </th>
            <th colspan="3" class="pz-bg pt-2 text-center">
              <div class="pz-title border-b pb-2 font-semibold">
                {{ t('modelPricing.sitePrice') }}
                <span class="pz-unit ml-1 font-normal normal-case">{{ t('publicModels.table.unitPerMillion') }}</span>
              </div>
            </th>
            <th colspan="3" class="border-l border-gray-100 pt-2 text-center dark:border-dark-700/60">
              <div class="border-b border-gray-200 pb-2 text-gray-400 dark:border-dark-600 dark:text-dark-500">
                {{ t('modelPricing.officialPrice') }}
                <span class="ml-1 font-normal normal-case text-gray-400 dark:text-dark-500">{{ t('publicModels.table.unitPerMillion') }}</span>
              </div>
            </th>
          </tr>
          <tr
            class="border-b border-gray-200 text-left text-[11px] font-medium uppercase leading-4 tracking-wide text-gray-400 dark:border-dark-700 dark:text-dark-500"
          >
            <th class="pz-bg px-3 py-2 font-medium">{{ t('publicModels.table.input') }}</th>
            <th class="pz-bg px-3 py-2 font-medium">{{ t('publicModels.table.output') }}</th>
            <th class="pz-bg px-3 py-2 font-medium">{{ t('publicModels.table.cache') }}</th>
            <th class="border-l border-gray-100 px-3 py-2 font-medium dark:border-dark-700/60">
              {{ t('publicModels.table.input') }}
            </th>
            <th class="px-3 py-2 font-medium">{{ t('publicModels.table.output') }}</th>
            <th class="px-3 py-2 font-medium">{{ t('publicModels.table.cache') }}</th>
          </tr>
        </thead>
        <tbody>
          <template v-for="m in sortedModels" :key="m.name">
            <tr
              class="border-b border-gray-100 transition-colors last:border-b-0 hover:bg-gray-50/70 dark:border-dark-800 dark:hover:bg-dark-800/50"
            >
              <!-- 模型名（点击复制）+ 非 token 计费徽章 -->
              <td class="border-r border-gray-100 py-2.5 pr-4 align-middle dark:border-dark-700/60">
                <div class="flex flex-wrap items-center gap-1.5">
                  <button
                    type="button"
                    class="model-name"
                    :title="t('publicModels.copyModelHint')"
                    @click="copyToClipboard(m.name)"
                  >
                    {{ m.name }}
                  </button>
                  <span
                    v-if="modeBadge(m)"
                    class="rounded-md bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-500 dark:bg-dark-700/70 dark:text-dark-300"
                  >
                    {{ modeBadge(m) }}
                  </span>
                </div>
              </td>

              <!-- 实付区：token 计费 → 输入 / 输出（阶梯内联）/ 缓存写读 -->
              <template v-if="!isPerRequest(m)">
                <td class="pz-cell px-3 py-2.5 align-middle font-mono font-semibold text-gray-900 dark:text-gray-50">
                  <template v-if="tokenIntervals(m).length">
                    <div
                      v-for="(iv, idx) in tokenIntervals(m)"
                      :key="idx"
                      class="whitespace-nowrap text-xs leading-5"
                    >
                      <span class="mr-1 font-sans font-normal text-gray-400 dark:text-dark-500">{{ tierLabel(iv) }}</span>
                      {{ sitePerM(iv.input_price) }}
                    </div>
                  </template>
                  <template v-else>{{ sitePerM(basePrice(m, 'input', 'site')) }}</template>
                </td>
                <td class="pz-cell px-3 py-2.5 align-middle font-mono font-semibold text-gray-900 dark:text-gray-50">
                  <template v-if="tokenIntervals(m).length">
                    <div
                      v-for="(iv, idx) in tokenIntervals(m)"
                      :key="idx"
                      class="whitespace-nowrap text-xs leading-5"
                    >
                      <span class="mr-1 font-sans font-normal text-gray-400 dark:text-dark-500">{{ tierLabel(iv) }}</span>
                      {{ sitePerM(iv.output_price) }}
                    </div>
                  </template>
                  <template v-else>{{ sitePerM(basePrice(m, 'output', 'site')) }}</template>
                </td>
                <td class="pz-cell px-3 py-2.5 align-middle">
                  <!-- 有区间：缓存价逐档取（扁平/官方缓存价此时不参与计费，展示即报错价） -->
                  <div
                    v-if="cacheIntervals(m).length"
                    class="space-y-1.5 font-mono text-xs text-gray-800 dark:text-gray-200"
                  >
                    <div v-for="(iv, idx) in cacheIntervals(m)" :key="idx" class="leading-5">
                      <div class="font-sans font-normal text-gray-400 dark:text-dark-500">{{ tierLabel(iv) }}</div>
                      <div>
                        <span class="mr-1 font-sans font-normal text-gray-400 dark:text-dark-500">{{ t('publicModels.table.cacheWrite') }}</span>
                        {{ sitePerM(iv.cache_write_price) }}
                      </div>
                      <div>
                        <span class="mr-1 font-sans font-normal text-gray-400 dark:text-dark-500">{{ t('publicModels.table.cacheRead') }}</span>
                        {{ sitePerM(iv.cache_read_price) }}
                      </div>
                    </div>
                  </div>
                  <div v-else-if="hasFlatCache(m)" class="space-y-0.5 font-mono text-xs text-gray-800 dark:text-gray-200">
                    <div>
                      <span class="mr-1 font-sans font-normal text-gray-400 dark:text-dark-500">{{ t('publicModels.table.cacheWrite') }}</span>
                      {{ sitePerM(basePrice(m, 'cache_write', 'site')) }}
                    </div>
                    <div>
                      <span class="mr-1 font-sans font-normal text-gray-400 dark:text-dark-500">{{ t('publicModels.table.cacheRead') }}</span>
                      {{ sitePerM(basePrice(m, 'cache_read', 'site')) }}
                    </div>
                  </div>
                  <span v-else class="text-gray-400 dark:text-dark-500">-</span>
                </td>
              </template>

              <!-- 实付区：按次 / 按图 → 优先 尺寸×质量 二维矩阵，解析不出再降级成阶梯芯片 -->
              <template v-else>
                <td colspan="3" class="pz-cell px-3 py-2.5 align-middle">
                  <table
                    v-if="matrix(m)"
                    class="border-separate border-spacing-0 overflow-hidden rounded-md border border-gray-200 text-xs dark:border-dark-700"
                  >
                    <thead>
                      <tr class="bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-gray-300">
                        <th class="px-3 py-1.5 text-left font-medium">{{ t('publicModels.table.tier') }}</th>
                        <th
                          v-for="col in matrix(m)!.cols"
                          :key="col"
                          class="px-3 py-1.5 text-right font-medium"
                        >
                          {{ col }}
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr
                        v-for="row in matrix(m)!.rows"
                        :key="row"
                        class="odd:bg-gray-50 even:bg-white dark:odd:bg-dark-800/40 dark:even:bg-dark-900/40"
                      >
                        <td class="px-3 py-1.5 text-left font-medium text-gray-700 dark:text-gray-200">{{ row }}</td>
                        <td
                          v-for="col in matrix(m)!.cols"
                          :key="col"
                          class="px-3 py-1.5 text-right font-mono text-gray-900 dark:text-gray-50"
                        >
                          {{ sitePerItem(matrix(m)!.cells[row]?.[col]) }}
                        </td>
                      </tr>
                    </tbody>
                  </table>
                  <div v-else-if="requestIntervals(m).length" class="flex flex-wrap items-center gap-1.5">
                    <span
                      v-for="(iv, idx) in requestIntervals(m)"
                      :key="idx"
                      class="inline-flex items-center gap-1 rounded-md bg-gray-100 px-2 py-0.5 font-mono text-xs text-gray-800 dark:bg-dark-700/60 dark:text-gray-200"
                    >
                      <span class="font-sans text-gray-400 dark:text-dark-500">{{ tierLabel(iv) }}</span>
                      {{ sitePerItem(iv.per_request_price)
                      }}<span class="font-sans text-gray-400 dark:text-dark-500">{{ perUnitSuffix(m) }}</span>
                    </span>
                  </div>
                  <template v-else-if="m.per_request_price != null">
                    <span class="font-mono font-semibold text-gray-900 dark:text-gray-50">
                      {{ sitePerItem(m.per_request_price) }}
                    </span>
                    <span class="ml-1 text-xs text-gray-400 dark:text-dark-500">{{ perUnitSuffix(m) }}</span>
                  </template>
                  <span v-else class="text-gray-400 dark:text-dark-500">-</span>
                </td>
              </template>

              <!-- 官方参考价（LiteLLM 原价，不乘倍率、不换汇） -->
              <td
                class="border-l border-gray-100 px-3 py-2.5 align-middle font-mono text-xs text-gray-500 dark:border-dark-700/60 dark:text-dark-400"
              >
                {{ officialPerM(m.official_input_price) }}
              </td>
              <td class="px-3 py-2.5 align-middle font-mono text-xs text-gray-500 dark:text-dark-400">
                {{ officialPerM(m.official_output_price) }}
              </td>
              <td class="px-3 py-2.5 align-middle">
                <div v-if="hasOfficialCache(m)" class="space-y-0.5 font-mono text-xs text-gray-500 dark:text-dark-400">
                  <div>
                    <span class="mr-1 font-sans font-normal text-gray-400 dark:text-dark-500">{{ t('publicModels.table.cacheWrite') }}</span>
                    {{ officialPerM(m.official_cache_write_price) }}
                  </div>
                  <div>
                    <span class="mr-1 font-sans font-normal text-gray-400 dark:text-dark-500">{{ t('publicModels.table.cacheRead') }}</span>
                    {{ officialPerM(m.official_cache_read_price) }}
                  </div>
                </div>
                <span v-else class="text-gray-400 dark:text-dark-500">-</span>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <!-- <sm：紧凑列表。上游的 min-w-[860px] 表格在 375px 屏上纯横滑，公开页不能这么干。 -->
    <ul class="divide-y divide-gray-100 sm:hidden dark:divide-dark-700/60">
      <li v-for="m in sortedModels" :key="m.name" class="py-3">
        <div class="flex flex-wrap items-center gap-1.5">
          <button
            type="button"
            class="model-name"
            :title="t('publicModels.copyModelHint')"
            @click="copyToClipboard(m.name)"
          >
            {{ m.name }}
          </button>
          <span
            v-if="modeBadge(m)"
            class="rounded-md bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-500 dark:bg-dark-700/70 dark:text-dark-300"
          >
            {{ modeBadge(m) }}
          </span>
        </div>

        <template v-if="!isPerRequest(m)">
          <dl class="mt-2 grid grid-cols-2 gap-x-3 gap-y-1">
            <div>
              <dt class="text-[10px] uppercase tracking-wide text-gray-400 dark:text-dark-500">
                {{ t('publicModels.table.input') }}
              </dt>
              <dd class="font-mono text-sm font-semibold text-primary-600 dark:text-primary-400">
                <template v-if="tokenIntervals(m).length">
                  <div v-for="(iv, idx) in tokenIntervals(m)" :key="idx" class="text-xs leading-5">
                    <span class="mr-1 font-sans font-normal text-gray-400 dark:text-dark-500">{{ tierLabel(iv) }}</span>
                    {{ sitePerM(iv.input_price) }}
                  </div>
                </template>
                <template v-else>{{ sitePerM(basePrice(m, 'input', 'site')) }}</template>
              </dd>
            </div>
            <div>
              <dt class="text-[10px] uppercase tracking-wide text-gray-400 dark:text-dark-500">
                {{ t('publicModels.table.output') }}
              </dt>
              <dd class="font-mono text-sm font-semibold text-primary-600 dark:text-primary-400">
                <template v-if="tokenIntervals(m).length">
                  <div v-for="(iv, idx) in tokenIntervals(m)" :key="idx" class="text-xs leading-5">
                    <span class="mr-1 font-sans font-normal text-gray-400 dark:text-dark-500">{{ tierLabel(iv) }}</span>
                    {{ sitePerM(iv.output_price) }}
                  </div>
                </template>
                <template v-else>{{ sitePerM(basePrice(m, 'output', 'site')) }}</template>
              </dd>
            </div>
          </dl>
          <!-- 有区间：缓存价逐档取，口径与桌面表一致 -->
          <p
            v-for="(iv, idx) in cacheIntervals(m)"
            :key="idx"
            class="mt-1 font-mono text-[11px] text-gray-500 dark:text-dark-400"
          >
            <span class="mr-1 font-sans text-gray-400 dark:text-dark-500">{{ tierLabel(iv) }}</span>
            <span class="font-sans text-gray-400 dark:text-dark-500">{{ t('publicModels.table.cacheWrite') }}</span>
            {{ sitePerM(iv.cache_write_price) }}
            <span class="ml-2 font-sans text-gray-400 dark:text-dark-500">{{ t('publicModels.table.cacheRead') }}</span>
            {{ sitePerM(iv.cache_read_price) }}
          </p>
          <p v-if="hasFlatCache(m)" class="mt-1 font-mono text-[11px] text-gray-500 dark:text-dark-400">
            <span class="font-sans text-gray-400 dark:text-dark-500">{{ t('publicModels.table.cacheWrite') }}</span>
            {{ sitePerM(basePrice(m, 'cache_write', 'site')) }}
            <span class="ml-2 font-sans text-gray-400 dark:text-dark-500">{{ t('publicModels.table.cacheRead') }}</span>
            {{ sitePerM(basePrice(m, 'cache_read', 'site')) }}
          </p>
          <p v-if="hasOfficialTokenPrice(m)" class="mt-1 text-[11px] text-gray-400 dark:text-dark-500">
            {{ t('modelPricing.officialPrice') }}
            <span class="font-mono">{{ officialPerM(m.official_input_price) }}</span>
            /
            <span class="font-mono">{{ officialPerM(m.official_output_price) }}</span>
          </p>
        </template>

        <template v-else>
          <div v-if="requestIntervals(m).length" class="mt-2 flex flex-wrap items-center gap-1.5">
            <span
              v-for="(iv, idx) in requestIntervals(m)"
              :key="idx"
              class="inline-flex items-center gap-1 rounded-md bg-gray-100 px-2 py-0.5 font-mono text-xs text-gray-800 dark:bg-dark-700/60 dark:text-gray-200"
            >
              <span class="font-sans text-gray-400 dark:text-dark-500">{{ tierLabel(iv) }}</span>
              {{ sitePerItem(iv.per_request_price)
              }}<span class="font-sans text-gray-400 dark:text-dark-500">{{ perUnitSuffix(m) }}</span>
            </span>
          </div>
          <p v-else-if="m.per_request_price != null" class="mt-2 font-mono text-sm font-semibold text-primary-600 dark:text-primary-400">
            {{ sitePerItem(m.per_request_price) }}
            <span class="font-sans text-xs font-normal text-gray-400 dark:text-dark-500">{{ perUnitSuffix(m) }}</span>
          </p>
          <p v-else class="mt-2 text-sm text-gray-400 dark:text-dark-500">-</p>
        </template>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useClipboard } from '@/composables/useClipboard'
import { platformAccentColor } from '@/utils/platformColors'
import { BILLING_MODE_IMAGE } from '@/constants/channel'
import type { UserPricingInterval, UserPricingModel } from '@/api/channels'
import {
  basePrice,
  billableIntervals,
  formatPerItem,
  formatPerMillion,
  hasIntervalCachePrice,
  isPerRequestMode,
  sortByOfficialOutputDesc,
  tierLabel,
  tierMatrix,
  type PriceCtx,
} from '@/utils/sitePricing'

const props = defineProps<{
  models: UserPricingModel[]
  /** 分组平台；实付分区底色随平台着色，未知平台回退品牌青。 */
  platform?: string
  /**
   * 分组倍率。直接来自 `group.rate_multiplier`——商户白标域名的售价覆盖后端已经做过
   * （available_channel_handler.go 的 LookupSellRateByMerchant），前端不做任何二次处理。
   */
  rate: number
  /** CNY per USD。 */
  fxRate: number
}>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()

/** 实付分区只从平台拿一个主色，浅底/标题/下划线全部由 scoped CSS 用 color-mix 派生。 */
const accentStyle = computed(() => ({ '--pz-accent': platformAccentColor(props.platform ?? '') }))

const siteCtx = computed<PriceCtx>(() => ({ mode: 'site', rate: props.rate, fxRate: props.fxRate }))
const OFFICIAL_CTX: PriceCtx = { mode: 'official', rate: 1, fxRate: 1 }

const sortedModels = computed(() => sortByOfficialOutputDesc(props.models))

function isPerRequest(m: UserPricingModel): boolean {
  return isPerRequestMode(m.billing_mode)
}

function modeBadge(m: UserPricingModel): string {
  if (m.billing_mode === BILLING_MODE_IMAGE) return t('modelPricing.badge.image')
  if (isPerRequest(m)) return t('modelPricing.badge.perRequest')
  return ''
}

function perUnitSuffix(m: UserPricingModel): string {
  return m.billing_mode === BILLING_MODE_IMAGE
    ? t('publicModels.table.perUnitImage')
    : t('publicModels.table.perUnitRequest')
}

/** 本站实付价 $/1M：(rate / fx) × 基础单价 × 1e6，与 /model-pricing 的 site 模式逐字同源。 */
function sitePerM(v: number | null | undefined): string {
  return formatPerMillion(v, siteCtx.value)
}

/** 本站实付价（按次 / 按图，不换算 1M）。 */
function sitePerItem(v: number | null | undefined): string {
  return formatPerItem(v, siteCtx.value)
}

/** 官方参考价：原值 × 1e6，不乘倍率也不换汇。 */
function officialPerM(v: number | null | undefined): string {
  return formatPerMillion(v, OFFICIAL_CTX)
}

/**
 * token 模式的阶梯定价（内联进输入/输出/缓存列）。
 *
 * 有效性判定走 billableIntervals，与后端 filterValidIntervals 同口径：只配了缓存价的
 * 区间同样生效，此时扁平 / 官方 input-output 已经不参与计费，绝不能回落展示。
 */
function tokenIntervals(m: UserPricingModel): UserPricingInterval[] {
  if (isPerRequest(m)) return []
  return billableIntervals(m.intervals)
}

/** 按次/按图模式的阶梯定价（仅保留配了按次价的档位）。 */
function requestIntervals(m: UserPricingModel): UserPricingInterval[] {
  return (m.intervals ?? []).filter((iv) => iv.per_request_price != null)
}

/** 二维矩阵（尺寸 × 质量）；tier_label 不含分隔符时返回 null，模板降级成芯片。 */
function matrix(m: UserPricingModel) {
  if (!isPerRequest(m)) return null
  return tierMatrix(requestIntervals(m))
}

/**
 * 阶梯模型的缓存价档位。
 *
 * 后端命中区间后走 intervalToModelPricing（model_pricing_resolver.go:255+），缓存价只认
 * `iv.CacheWritePrice / iv.CacheReadPrice`，扁平 / 官方缓存价一分钱都不参与计费。所以
 * 有区间时缓存列必须逐档取值，和输入/输出列同一套逻辑。
 *
 * 一档缓存价都没配时返回空数组 —— 调用方渲染 `-`，不逐档铺一堆空行。
 */
function cacheIntervals(m: UserPricingModel): UserPricingInterval[] {
  const ivs = tokenIntervals(m)
  return hasIntervalCachePrice(ivs) ? ivs : []
}

/**
 * 扁平缓存价只在**没有任何有效区间**时才展示。
 *
 * 区间存在时扁平字段永不生效，展示它等于对用户报一个永远收不到的价。
 * 注意档位里没配缓存价时本列显示 `-` 而不是 `$0/M`：多数模型确实按 0 计费，但
 * billing_service 的 gemini / GPT-5.6 兜底策略（fillGeminiCacheWritePrice、
 * 缓存写价 = 输入价 × 1.25）会在缓存写价为空时补价，写死 `$0` 反而是新的假承诺。
 */
function hasFlatCache(m: UserPricingModel): boolean {
  if (tokenIntervals(m).length) return false
  return basePrice(m, 'cache_write', 'site') != null || basePrice(m, 'cache_read', 'site') != null
}

function hasOfficialCache(m: UserPricingModel): boolean {
  return m.official_cache_write_price != null || m.official_cache_read_price != null
}

function hasOfficialTokenPrice(m: UserPricingModel): boolean {
  return m.official_input_price != null || m.official_output_price != null
}
</script>

<style scoped>
/* 实付分区配色统一从 --pz-accent(平台主色)派生，新增平台无需扩展样式 */
.public-pricing {
  --pz-title: color-mix(in srgb, var(--pz-accent) 88%, black);
  --pz-bg: color-mix(in srgb, var(--pz-accent) 7%, transparent);
  --pz-bg-hover: color-mix(in srgb, var(--pz-accent) 13%, transparent);
}

.dark .public-pricing {
  --pz-title: color-mix(in srgb, var(--pz-accent) 70%, white);
  --pz-bg: color-mix(in srgb, var(--pz-accent) 6%, transparent);
  --pz-bg-hover: color-mix(in srgb, var(--pz-accent) 10%, transparent);
}

.pz-bg,
.pz-cell {
  background-color: var(--pz-bg);
}

.pz-cell {
  transition: background-color 150ms cubic-bezier(0.4, 0, 0.2, 1);
}

tbody tr:hover .pz-cell {
  background-color: var(--pz-bg-hover);
}

.pz-title {
  /* color-mix 不可用的老浏览器回退为平台原色 */
  color: var(--pz-accent);
  color: var(--pz-title);
  border-color: color-mix(in srgb, var(--pz-title) 30%, transparent);
}

.pz-unit {
  color: color-mix(in srgb, var(--pz-title) 62%, transparent);
}

.model-name {
  @apply cursor-pointer break-all text-left font-mono text-sm font-medium text-gray-900 transition-colors
         hover:text-primary-600 dark:text-white dark:hover:text-primary-400;
}
</style>
