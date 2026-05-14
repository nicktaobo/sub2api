<template>
  <AppLayout>
    <div class="flex h-full flex-col gap-6">
      <!-- 顶部过滤条 -->
      <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div class="flex flex-wrap items-center gap-3">
          <div class="relative w-full sm:w-72">
            <Icon
              name="search"
              size="md"
              class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500"
            />
            <input
              v-model="searchQuery"
              type="text"
              :placeholder="t('modelPricing.searchPlaceholder')"
              class="input pl-10"
            />
          </div>
          <!-- platform 过滤 chip -->
          <div class="flex flex-wrap gap-1.5">
            <button
              type="button"
              class="platform-filter"
              :class="platformFilter === '' ? 'platform-filter-active' : ''"
              @click="platformFilter = ''"
            >
              <Icon name="grid" size="xs" class="mr-1" />
              {{ t('modelPricing.filterAll') }}
              <span class="ml-1 text-[10px] opacity-70">{{ groups.length }}</span>
            </button>
            <button
              v-for="p in platformOptions"
              :key="p.name"
              type="button"
              class="platform-filter"
              :class="platformFilter === p.name ? 'platform-filter-active' : ''"
              @click="platformFilter = p.name"
            >
              <PlatformIcon :platform="(p.name as GroupPlatform)" size="xs" class="mr-1" />
              {{ p.name }}
              <span class="ml-1 text-[10px] opacity-70">{{ p.count }}</span>
            </button>
          </div>
        </div>

        <div class="flex flex-wrap items-center gap-2">
          <span class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('modelPricing.fxNote', { rate: fxRate.toFixed(2) }) }}
          </span>
          <button
            type="button"
            class="btn btn-secondary"
            :title="t('common.refresh', 'Refresh')"
            :disabled="loading"
            @click="reload"
          >
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </div>

      <!-- 双栏正文 -->
      <div class="grid min-h-0 flex-1 grid-cols-1 gap-6 lg:grid-cols-[360px_minmax(0,1fr)]">
        <!-- 左：端点（group）列表 -->
        <div class="card flex flex-col overflow-hidden">
          <div class="flex items-center gap-2 border-b border-gray-100 px-5 py-4 dark:border-dark-700/50">
            <Icon name="server" size="md" class="text-primary-500" />
            <h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">
              {{ t('modelPricing.endpoints') }}
            </h2>
            <span class="ml-auto text-xs text-gray-400">{{ filteredGroups.length }}</span>
          </div>

          <div class="flex-1 space-y-3 overflow-y-auto px-3 py-3">
            <template v-if="loading && !groups.length">
              <div v-for="i in 4" :key="i" class="h-20 animate-pulse rounded-xl bg-gray-100 dark:bg-dark-700/50"></div>
            </template>

            <div
              v-else-if="!filteredGroups.length"
              class="flex flex-col items-center justify-center gap-2 px-4 py-12 text-center text-gray-400 dark:text-gray-500"
            >
              <Icon name="inbox" size="xl" />
              <p class="text-sm">{{ t('modelPricing.empty') }}</p>
            </div>

            <button
              v-for="g in filteredGroups"
              :key="g.id"
              type="button"
              class="endpoint-card group"
              :class="{ 'endpoint-card-active': g.id === selectedGroupId }"
              @click="selectedGroupId = g.id"
            >
              <div class="flex items-start gap-3">
                <div class="endpoint-icon-wrap" :class="iconWrapClass(g.platform)">
                  <PlatformIcon :platform="(g.platform as GroupPlatform)" size="md" />
                </div>
                <div class="min-w-0 flex-1">
                  <div class="flex items-start justify-between gap-2">
                    <div class="truncate text-sm font-semibold text-gray-900 dark:text-gray-100">
                      {{ g.name }}
                    </div>
                    <span class="rate-badge rate-badge-good">
                      <Icon name="bolt" size="xs" class="mr-0.5" />
                      {{ formatRate(g.rate_multiplier) }}
                    </span>
                  </div>
                  <div class="mt-1.5 flex flex-wrap items-center gap-1.5">
                    <span class="platform-chip">{{ g.platform }}</span>
                    <span
                      v-if="g.is_exclusive"
                      class="platform-chip platform-chip-exclusive"
                    >
                      <Icon name="key" size="xs" class="mr-0.5" />
                      {{ t('modelPricing.exclusive') }}
                    </span>
                    <span class="platform-chip platform-chip-count">
                      <Icon name="cube" size="xs" class="mr-0.5" />
                      {{ g.models.length }} {{ t('modelPricing.modelsUnit') }}
                    </span>
                  </div>
                </div>
              </div>
            </button>
          </div>
        </div>

        <!-- 右：详情 -->
        <div class="card flex min-h-0 flex-col overflow-hidden">
          <template v-if="selectedGroup">
            <div class="flex flex-wrap items-center gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700/50">
              <div class="endpoint-icon-wrap endpoint-icon-wrap-lg" :class="iconWrapClass(selectedGroup.platform)">
                <PlatformIcon :platform="(selectedGroup.platform as GroupPlatform)" size="md" />
              </div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">
                {{ selectedGroup.name }}
              </h2>
              <span class="platform-chip">{{ selectedGroup.platform }}</span>
              <span class="rate-badge rate-badge-good">
                <Icon name="bolt" size="xs" class="mr-0.5" />
                {{ formatRate(selectedGroup.rate_multiplier) }}
              </span>
              <div class="ml-auto inline-flex rounded-lg border border-gray-200 bg-white p-0.5 text-xs dark:border-dark-700 dark:bg-dark-800">
                <button
                  type="button"
                  class="rounded-md px-3 py-1.5 font-medium transition"
                  :class="priceMode === 'official'
                    ? 'bg-primary-500 text-white shadow-sm'
                    : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'"
                  @click="priceMode = 'official'"
                >{{ t('modelPricing.officialPrice') }}</button>
                <button
                  type="button"
                  class="rounded-md px-3 py-1.5 font-medium transition"
                  :class="priceMode === 'site'
                    ? 'bg-primary-500 text-white shadow-sm'
                    : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'"
                  @click="priceMode = 'site'"
                >{{ t('modelPricing.sitePrice') }}</button>
              </div>
            </div>

            <div class="flex-1 overflow-auto">
              <div v-if="!selectedGroup.models.length" class="px-5 py-12 text-center text-sm text-gray-400">
                {{ t('modelPricing.noModels') }}
              </div>
              <table v-else class="w-full text-sm">
                <thead>
                  <tr class="text-left text-xs font-medium uppercase tracking-wide text-gray-400 dark:text-gray-500">
                    <th class="px-5 py-3">
                      <span class="inline-flex items-center gap-1">
                        <Icon name="cube" size="xs" />
                        {{ t('modelPricing.columns.model') }}
                      </span>
                    </th>
                    <th class="px-3 py-3 text-right">
                      <span class="inline-flex items-center justify-end gap-1">
                        <Icon name="arrowDown" size="xs" />
                        {{ t('modelPricing.columns.input') }}
                      </span>
                    </th>
                    <th class="px-3 py-3 text-right">
                      <span class="inline-flex items-center justify-end gap-1">
                        <Icon name="arrowUp" size="xs" />
                        {{ t('modelPricing.columns.output') }}
                      </span>
                    </th>
                    <th class="px-3 py-3 text-right">
                      <span class="inline-flex items-center justify-end gap-1">
                        <Icon name="database" size="xs" />
                        {{ t('modelPricing.columns.cacheWrite') }}
                      </span>
                    </th>
                    <th class="px-3 py-3 text-right">
                      <span class="inline-flex items-center justify-end gap-1">
                        <Icon name="eye" size="xs" />
                        {{ t('modelPricing.columns.cacheRead') }}
                      </span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="model in selectedGroup.models"
                    :key="model.name"
                    class="border-t border-gray-50 transition hover:bg-primary-50/30 dark:border-dark-800 dark:hover:bg-primary-900/10"
                  >
                    <td class="px-5 py-3">
                      <div class="font-mono text-sm font-medium text-gray-900 dark:text-gray-100">
                        {{ model.name }}
                      </div>
                    </td>
                    <td class="px-3 py-3 text-right font-mono text-sm" :class="priceCellTone">
                      {{ formatPrice(model.official_input_price) }}
                    </td>
                    <td class="px-3 py-3 text-right font-mono text-sm" :class="priceCellTone">
                      {{ formatPrice(model.official_output_price) }}
                    </td>
                    <td class="px-3 py-3 text-right font-mono text-sm" :class="priceCellTone">
                      {{ formatPrice(model.official_cache_write_price) }}
                    </td>
                    <td class="px-3 py-3 text-right font-mono text-sm" :class="priceCellTone">
                      {{ formatPrice(model.official_cache_read_price) }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </template>

          <div
            v-else
            class="flex flex-1 flex-col items-center justify-center gap-2 px-4 py-16 text-center text-gray-400 dark:text-gray-500"
          >
            <Icon name="inbox" size="xl" />
            <p class="text-sm">{{ t('modelPricing.selectPrompt') }}</p>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import type { GroupPlatform } from '@/types'
import userChannelsAPI, { type UserPricingGroup } from '@/api/channels'
import systemAPI from '@/api/system'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { DEFAULT_CNY_PER_USD } from '@/utils/pricing'

const { t } = useI18n()
const appStore = useAppStore()

const groups = ref<UserPricingGroup[]>([])
const loading = ref(false)
const searchQuery = ref('')
const platformFilter = ref<string>('')
const priceMode = ref<'official' | 'site'>('site')
const selectedGroupId = ref<number | null>(null)
const fxRate = ref<number>(DEFAULT_CNY_PER_USD)

// platform 维度的过滤选项：按出现频率聚合，含每个 platform 下的 group 数量。
const platformOptions = computed(() => {
  const counts = new Map<string, number>()
  for (const g of groups.value) {
    counts.set(g.platform, (counts.get(g.platform) ?? 0) + 1)
  }
  return Array.from(counts.entries())
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count || a.name.localeCompare(b.name))
})

const filteredGroups = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  const pf = platformFilter.value
  return groups.value.filter((g) => {
    if (pf && g.platform !== pf) return false
    if (!q) return true
    if (g.name.toLowerCase().includes(q)) return true
    if (g.platform.toLowerCase().includes(q)) return true
    return g.models.some((m) => m.name.toLowerCase().includes(q))
  })
})

const selectedGroup = computed(() =>
  filteredGroups.value.find((g) => g.id === selectedGroupId.value) ?? null,
)

/**
 * iconWrapClass 按 platform 返回端点 icon 圆角 wrap 的配色类。
 * 跟项目里其它 platform badge 的色系保持一致：
 *   anthropic → orange、openai → green、gemini → blue、antigravity → purple。
 */
function iconWrapClass(platform: string): string {
  switch (platform) {
    case 'anthropic':
      return 'endpoint-icon-wrap-anthropic'
    case 'openai':
      return 'endpoint-icon-wrap-openai'
    case 'gemini':
      return 'endpoint-icon-wrap-gemini'
    case 'antigravity':
      return 'endpoint-icon-wrap-antigravity'
    default:
      return ''
  }
}

/**
 * formatRate 展示分组的"计费倍率"，例如 1.8x、0.4x、25x。
 * 保留 2 位小数后去掉无意义零（1.50 → 1.5）。
 */
function formatRate(rate: number): string {
  const r = Number(rate || 1)
  if (Math.abs(r - 1) < 1e-6) return '1x'
  if (r >= 10) return `${r.toFixed(0)}x`
  return `${parseFloat(r.toFixed(3))}x`
}


const priceCellTone = computed(() =>
  priceMode.value === 'site'
    ? 'text-primary-600 dark:text-primary-400'
    : 'text-gray-700 dark:text-gray-300',
)

/**
 * 价格格式化：
 *   - 入参 v 是 per-token 美元价（如 0.000003）
 *   - "official" 模式：直接 × 1M = 官方对外 $/M token
 *   - "site"     模式：(group.rate / fx) × v × 1M = 等效美元 $/M token
 */
function formatPrice(perTokenUSD: number | null | undefined): string {
  if (perTokenUSD == null) return '-'
  const officialPerM = perTokenUSD * 1_000_000
  if (priceMode.value === 'official') {
    return `$${trimNum(officialPerM)}/M`
  }
  const rate = selectedGroup.value?.rate_multiplier ?? 1
  const sitePerM = (rate / fxRate.value) * officialPerM
  return `$${trimNum(sitePerM)}/M`
}

function trimNum(n: number): string {
  if (n === 0) return '0'
  const digits = n >= 100 ? 0 : n >= 10 ? 2 : 4
  const fixed = n.toFixed(digits)
  return fixed.replace(/\.?0+$/, '') || '0'
}

async function reload() {
  loading.value = true
  try {
    const [list, fx] = await Promise.all([
      userChannelsAPI.getPricingGroups(),
      systemAPI.getFXRate().catch(() => null),
    ])
    groups.value = list
    if (fx && fx.cny_per_usd > 0) fxRate.value = fx.cny_per_usd
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    loading.value = false
  }
}

watch(filteredGroups, (list) => {
  if (!list.length) {
    selectedGroupId.value = null
    return
  }
  if (!list.some((g) => g.id === selectedGroupId.value)) {
    selectedGroupId.value = list[0].id
  }
}, { immediate: true })

onMounted(reload)
</script>

<style scoped>
.endpoint-card {
  @apply relative w-full rounded-xl border border-gray-100 bg-white px-4 py-3.5 text-left
         transition-all duration-200
         hover:border-primary-200 hover:bg-primary-50/30 hover:shadow-md hover:-translate-y-0.5
         dark:border-dark-700/50 dark:bg-dark-800/40 dark:hover:border-primary-500/40
         dark:hover:bg-primary-900/10;
}

.endpoint-card-active {
  @apply border-primary-300 bg-gradient-to-br from-primary-50 to-white shadow-md
         dark:border-primary-500/60 dark:from-primary-900/20 dark:to-dark-800/40;
}

.endpoint-card-active::before {
  content: '';
  @apply absolute left-0 top-3 bottom-3 w-1 rounded-r bg-primary-500;
}

.endpoint-icon-wrap {
  @apply flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg
         bg-gradient-to-br from-gray-100 to-gray-50 text-gray-600
         dark:from-dark-700/40 dark:to-dark-800/40 dark:text-gray-300;
}

.endpoint-icon-wrap-lg {
  @apply h-10 w-10;
}

.endpoint-icon-wrap-anthropic {
  @apply bg-gradient-to-br from-orange-100 to-orange-50 text-orange-600
         dark:from-orange-900/40 dark:to-orange-800/20 dark:text-orange-300;
}

.endpoint-icon-wrap-openai {
  @apply bg-gradient-to-br from-emerald-100 to-emerald-50 text-emerald-600
         dark:from-emerald-900/40 dark:to-emerald-800/20 dark:text-emerald-300;
}

.endpoint-icon-wrap-gemini {
  @apply bg-gradient-to-br from-blue-100 to-blue-50 text-blue-600
         dark:from-blue-900/40 dark:to-blue-800/20 dark:text-blue-300;
}

.endpoint-icon-wrap-antigravity {
  @apply bg-gradient-to-br from-purple-100 to-purple-50 text-purple-600
         dark:from-purple-900/40 dark:to-purple-800/20 dark:text-purple-300;
}

.rate-badge {
  @apply inline-flex flex-shrink-0 items-center rounded-full px-2 py-0.5 text-xs font-semibold;
}

.rate-badge-good {
  @apply bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300;
}

.platform-chip {
  @apply inline-flex items-center rounded-md bg-gray-100 px-1.5 py-0.5 text-[10px]
         font-medium text-gray-600 dark:bg-dark-700/50 dark:text-gray-300;
}

.platform-filter {
  @apply inline-flex items-center rounded-full border border-gray-200 bg-white px-3
         py-1 text-xs font-medium text-gray-600 transition
         hover:border-primary-300 hover:bg-primary-50 hover:text-primary-700
         dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300
         dark:hover:border-primary-500/50 dark:hover:bg-primary-900/20
         dark:hover:text-primary-300;
}

.platform-filter-active {
  @apply border-primary-500 bg-primary-500 text-white shadow-sm
         hover:border-primary-500 hover:bg-primary-500 hover:text-white
         dark:border-primary-500 dark:bg-primary-500 dark:text-white;
}

.platform-chip-exclusive {
  @apply bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300;
}

.platform-chip-count {
  @apply bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300;
}
</style>
