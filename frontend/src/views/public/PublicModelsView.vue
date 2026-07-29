<template>
  <div class="min-h-screen bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white">
    <header class="border-b border-gray-200 bg-white/95 dark:border-dark-800 dark:bg-dark-900/95">
      <div class="mx-auto flex max-w-6xl items-center justify-between gap-4 px-4 py-4 sm:px-6">
        <RouterLink to="/home" class="flex min-w-0 items-center gap-3">
          <span class="flex h-10 w-10 flex-shrink-0 items-center justify-center overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-gray-200 dark:bg-dark-800 dark:ring-dark-700">
            <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
          </span>
          <span class="truncate text-base font-semibold text-gray-950 dark:text-white">
            {{ siteName }}
          </span>
        </RouterLink>
        <div class="flex flex-shrink-0 items-center gap-2">
          <LocaleSwitcher />
          <RouterLink
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="inline-flex items-center justify-center rounded-lg bg-primary-600 px-4 py-2 text-sm font-semibold text-white shadow-sm shadow-primary-600/20 transition hover:bg-primary-700"
          >
            {{ t('home.dashboard') }}
          </RouterLink>
          <RouterLink
            v-else
            to="/login"
            class="inline-flex items-center justify-center rounded-lg bg-primary-600 px-4 py-2 text-sm font-semibold text-white shadow-sm shadow-primary-600/20 transition hover:bg-primary-700"
          >
            {{ t('home.login') }}
          </RouterLink>
        </div>
      </div>
    </header>

    <main class="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:py-10">
      <!-- Hero banner with live stats -->
      <section class="relative mb-6 overflow-hidden rounded-2xl bg-gradient-to-br from-primary-600 via-primary-600 to-primary-800 px-6 py-7 text-white shadow-sm sm:mb-8 sm:px-9 sm:py-9">
        <div aria-hidden="true" class="pointer-events-none absolute -right-16 -top-24 h-64 w-64 rounded-full bg-white/10 blur-2xl"></div>
        <div aria-hidden="true" class="pointer-events-none absolute -bottom-24 left-1/3 h-56 w-56 rounded-full bg-primary-300/20 blur-3xl"></div>
        <div class="relative flex flex-col gap-7 sm:flex-row sm:items-center sm:justify-between">
          <div class="min-w-0">
            <span class="inline-flex items-center gap-1.5 rounded-full bg-white/15 px-3 py-1 text-xs font-medium ring-1 ring-white/20 backdrop-blur">
              <Icon name="grid" size="xs" />
              {{ t('publicModels.badge') }}
            </span>
            <h1 class="mt-3 break-words text-2xl font-bold tracking-tight sm:text-4xl">
              {{ t('publicModels.title') }}
            </h1>
            <p class="mt-2.5 max-w-xl text-sm text-white/80">
              {{ t('publicModels.subtitle') }}
            </p>
          </div>
          <div class="flex shrink-0 items-center gap-5 sm:gap-9">
            <div class="text-center">
              <div class="text-3xl font-bold tabular-nums sm:text-4xl">{{ groups.length }}</div>
              <p class="mt-1 text-xs text-white/70">{{ t('publicModels.statGroups') }}</p>
            </div>
            <div class="h-10 w-px bg-white/20"></div>
            <div class="text-center">
              <div class="text-3xl font-bold tabular-nums sm:text-4xl">{{ totalModelCount }}</div>
              <p class="mt-1 text-xs text-white/70">{{ t('publicModels.statModels') }}</p>
            </div>
            <div class="h-10 w-px bg-white/20"></div>
            <div class="text-center">
              <div class="text-3xl font-bold tabular-nums sm:text-4xl">{{ platformOptions.length }}</div>
              <p class="mt-1 text-xs text-white/70">{{ t('publicModels.statPlatforms') }}</p>
            </div>
          </div>
        </div>
      </section>

      <!-- API base URL -->
      <div
        v-if="apiBaseUrl"
        class="mb-6 flex flex-col gap-3 rounded-xl border border-gray-200 bg-white p-4 sm:flex-row sm:items-center sm:justify-between sm:gap-4 dark:border-dark-700 dark:bg-dark-800/40"
      >
        <div class="flex items-start gap-3">
          <span class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-primary-50 text-primary-700 dark:bg-primary-500/10 dark:text-primary-300">
            <Icon name="link" size="md" />
          </span>
          <div class="min-w-0">
            <p class="text-sm font-semibold text-gray-950 dark:text-white">{{ t('publicModels.apiBaseTitle') }}</p>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ t('publicModels.apiBaseHint') }}</p>
          </div>
        </div>
        <button
          type="button"
          class="inline-flex min-w-0 items-center gap-3 self-start rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 transition hover:border-primary-300 sm:self-auto dark:border-dark-700 dark:bg-dark-900/60 dark:hover:border-primary-500/40"
          :title="t('publicModels.copyApiBase')"
          @click="copyApiBase"
        >
          <code class="truncate font-mono text-sm text-gray-800 dark:text-dark-100">{{ apiBaseUrl }}/v1</code>
          <span class="flex-shrink-0 text-xs font-medium text-primary-600 dark:text-primary-300">{{ t('publicModels.copyApiBase') }}</span>
        </button>
      </div>

      <!-- Search + refresh -->
      <div class="mb-4 flex items-center gap-2">
        <div class="relative min-w-0 flex-1">
          <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-dark-400" />
          <input
            v-model="searchQuery"
            type="search"
            :placeholder="t('publicModels.searchPlaceholder')"
            class="w-full rounded-lg border border-gray-200 bg-white py-2.5 pl-9 pr-3 text-sm text-gray-900 placeholder:text-gray-400 transition focus:border-primary-400 focus:outline-none focus:ring-2 focus:ring-primary-500/20 dark:border-dark-700 dark:bg-dark-800/60 dark:text-white dark:placeholder:text-dark-400"
          />
        </div>
        <button
          type="button"
          class="inline-flex flex-shrink-0 items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 py-2.5 text-sm font-medium text-gray-700 transition hover:border-primary-300 hover:text-primary-700 disabled:cursor-not-allowed disabled:opacity-60 dark:border-dark-700 dark:bg-dark-800/40 dark:text-dark-200 dark:hover:border-primary-500/40"
          :disabled="loading"
          @click="reload"
        >
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          <span class="hidden sm:inline">{{ t('publicModels.refresh') }}</span>
        </button>
      </div>

      <div class="mb-3 flex flex-wrap items-center gap-2">
        <button
          type="button"
          class="platform-chip"
          :class="platformFilter === '' ? 'platform-chip-active' : ''"
          @click="platformFilter = ''"
        >
          <Icon name="grid" size="xs" class="mr-1" />
          {{ t('publicModels.filterAll') }}
          <span class="ml-1 text-[10px] opacity-70">{{ groups.length }}</span>
        </button>
        <button
          v-for="p in platformOptions"
          :key="p.name"
          type="button"
          class="platform-chip"
          :class="platformFilter === p.name ? 'platform-chip-active' : ''"
          @click="platformFilter = p.name"
        >
          <PlatformIcon :platform="(p.name as GroupPlatform)" size="xs" class="mr-1" />
          {{ p.name }}
          <span class="ml-1 text-[10px] opacity-70">{{ p.count }}</span>
        </button>
      </div>

      <!-- 倍率筛选：当前 platform 下无分组命中的倍率置灰但不消失（faceted filtering） -->
      <div v-if="rateOptions.length > 1" class="mb-6 flex flex-wrap items-center gap-2">
        <span class="text-xs font-medium text-gray-500 dark:text-dark-400">{{ t('publicModels.rateFilterLabel') }}</span>
        <button
          type="button"
          class="platform-chip"
          :class="rateFilter === 'all' ? 'platform-chip-active' : ''"
          @click="rateFilter = 'all'"
        >
          {{ t('publicModels.filterAll') }}
        </button>
        <button
          v-for="r in rateOptions"
          :key="r"
          type="button"
          class="platform-chip"
          :class="[
            rateFilter === r ? 'platform-chip-active' : '',
            enabledRates.has(r) ? '' : 'platform-chip-disabled',
          ]"
          :disabled="!enabledRates.has(r)"
          @click="rateFilter = r"
        >
          {{ formatRateMultiplier(r) }}
        </button>
      </div>

      <div v-if="loading && !groups.length" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <div v-for="i in 6" :key="i" class="h-44 animate-pulse rounded-xl bg-white dark:bg-dark-800/40"></div>
      </div>

      <div
        v-else-if="loadError"
        class="rounded-lg border border-red-200 bg-red-50 p-6 text-red-700 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-200"
      >
        <h2 class="text-base font-semibold">{{ t('publicModels.loadErrorTitle') }}</h2>
        <p class="mt-2 text-sm">{{ t('publicModels.loadErrorDescription') }}</p>
      </div>

      <div
        v-else-if="!filteredGroups.length"
        class="rounded-lg border border-dashed border-gray-300 bg-white px-6 py-14 text-center text-sm text-gray-500 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-400"
      >
        {{ searchQuery.trim() ? t('publicModels.searchEmpty') : t('publicModels.empty') }}
      </div>

      <div v-else class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <article
          v-for="group in filteredGroups"
          :key="group.id"
          class="group-card"
          :class="expandedGroupId === group.id ? 'sm:col-span-2 lg:col-span-3' : ''"
        >
          <header class="flex items-start justify-between gap-3">
            <div class="flex items-start gap-3">
              <span class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-700 dark:bg-dark-800 dark:text-dark-200">
                <PlatformIcon :platform="(group.platform as GroupPlatform)" size="md" />
              </span>
              <div class="min-w-0">
                <h3 class="truncate text-base font-semibold text-gray-950 dark:text-white">{{ group.name }}</h3>
                <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-dark-400">{{ group.platform }}</p>
              </div>
            </div>
            <span class="rounded-full bg-primary-50 px-2.5 py-0.5 text-xs font-semibold text-primary-700 dark:bg-primary-500/15 dark:text-primary-200">
              {{ formatRateMultiplier(group.rate_multiplier) }}
            </span>
          </header>

          <div class="mt-4 flex flex-wrap items-center justify-between gap-x-3 gap-y-1 text-xs text-gray-500 dark:text-dark-400">
            <span>{{ t('publicModels.modelCount', { count: group.models.length }) }}</span>
            <span
              v-if="startingPriceLabels[group.id]"
              class="font-semibold text-primary-600 dark:text-primary-300"
            >
              {{ startingPriceLabels[group.id] }}
            </span>
            <!-- 只有「一个价都没公布」才敢说暂未公布；算不出起价但有价的分组宁可留白 -->
            <span v-else-if="noPricingGroupIds.has(group.id)">{{ t('publicModels.noPricing') }}</span>
          </div>

          <!-- 折叠态：模型名 chip（点击复制）。展开后由价格表接管，避免重复列同一批模型。 -->
          <ul v-if="expandedGroupId !== group.id" class="mt-3 flex flex-wrap gap-1.5">
            <li v-for="m in displayedModels(group)" :key="m.name">
              <button
                type="button"
                class="model-chip"
                :title="t('publicModels.copyModelHint')"
                @click="copyModel(m.name)"
              >
                {{ m.name }}
              </button>
            </li>
            <li
              v-if="group.models.length > MAX_MODELS_PER_CARD"
              class="model-chip model-chip-more"
            >
              +{{ group.models.length - MAX_MODELS_PER_CARD }}
            </li>
          </ul>

          <!-- 展开态：v-if 而非 v-show，未展开的卡片零 DOM 成本（单组可能上百个模型） -->
          <div v-else class="mt-3 border-t border-gray-100 pt-3 dark:border-dark-700/60">
            <p
              v-if="expandedModels(group).length && expandedModels(group).length < group.models.length"
              class="mb-2 text-xs text-gray-500 dark:text-dark-400"
            >
              {{ t('publicModels.searchFilteredHint', { count: expandedModels(group).length, total: group.models.length }) }}
            </p>
            <!-- 表格吃的是搜索过滤后的模型（不截断）——搜到再展开必须还能看见命中项 -->
            <PublicPricingTable
              v-if="expandedModels(group).length"
              :models="expandedModels(group)"
              :platform="group.platform"
              :rate="group.rate_multiplier"
              :fx-rate="fxRate"
            />
            <!--
              groupSearchEmpty 是护栏而非常态：expandedModels 认下 groupsBeforeRate 放行分组的
              全部三个理由（组名 / 平台名 / 模型名命中），所以列表里的卡片展开必有内容。
              哪天 groupsBeforeRate 多加一个放行条件而这里没跟上，用户看到的是这句提示，
              不是一张空表。
            -->
            <p v-else class="py-6 text-center text-sm text-gray-400 dark:text-dark-500">
              {{ group.models.length ? t('publicModels.groupSearchEmpty') : t('modelPricing.noModels') }}
            </p>
          </div>

          <div class="mt-auto pt-4">
            <button
              type="button"
              class="expand-toggle"
              :aria-expanded="expandedGroupId === group.id"
              @click="toggleGroup(group.id)"
            >
              <Icon :name="expandedGroupId === group.id ? 'chevronUp' : 'chevronDown'" size="xs" class="mr-1" />
              {{ expandedGroupId === group.id ? t('publicModels.hidePricing') : t('publicModels.viewPricing') }}
            </button>
          </div>
        </article>
      </div>

      <p class="mt-8 text-center text-xs text-gray-500 dark:text-dark-400">
        {{ t('publicModels.priceNote') }}
        <span class="ml-1">{{ t('modelPricing.fxNote', { rate: fxRate.toFixed(2) }) }}</span>
      </p>
      <p class="mt-2 text-center text-xs text-gray-500 dark:text-dark-400">
        {{ t('publicModels.footnote') }}
      </p>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useHead } from '@unhead/vue'
import Icon from '@/components/icons/Icon.vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import PublicPricingTable from '@/components/publicModels/PublicPricingTable.vue'
import userChannelsAPI, { type UserPricingGroup, type UserPricingModel } from '@/api/channels'
import { useAuthStore, useAppStore, useMerchantStore } from '@/stores'
import { useClipboard } from '@/composables/useClipboard'
import { useFxRate } from '@/composables/useFxRate'
import {
  formatPerItem,
  formatPerMillion,
  formatRateMultiplier,
  groupStartingPrice,
  hasPublishedPrice,
  type PriceCtx,
} from '@/utils/sitePricing'
import type { GroupPlatform } from '@/types'

const { t, locale } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const merchantStore = useMerchantStore()
const { copyToClipboard } = useClipboard()
const { fxRate, ensureFxRate } = useFxRate()

const MAX_MODELS_PER_CARD = 12

const groups = ref<UserPricingGroup[]>([])
const loading = ref(false)
const loadError = ref(false)
const platformFilter = ref<string>('')
const searchQuery = ref('')
/** 同时只允许展开一个分组：展开卡在 grid 里横跨整行，两张同时展开会把网格撕烂。 */
const expandedGroupId = ref<number | null>(null)
const rateFilter = ref<number | 'all'>('all')

const platformOptions = computed(() => {
  const counts = new Map<string, number>()
  for (const g of groups.value) {
    counts.set(g.platform, (counts.get(g.platform) ?? 0) + 1)
  }
  return Array.from(counts.entries())
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count || a.name.localeCompare(b.name))
})

const totalModelCount = computed(() => {
  const names = new Set<string>()
  for (const g of groups.value) {
    for (const m of g.models) names.add(m.name)
  }
  return names.size
})

/** platform + 搜索过滤后的结果（倍率维度之前），倍率 chip 的 faceted 置灰以此为准。 */
const groupsBeforeRate = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  let result = platformFilter.value
    ? groups.value.filter((g) => g.platform === platformFilter.value)
    : groups.value
  if (q) {
    result = result.filter(
      (g) =>
        g.name.toLowerCase().includes(q) ||
        g.platform.toLowerCase().includes(q) ||
        g.models.some((m) => m.name.toLowerCase().includes(q)),
    )
  }
  return result
})

/** 全量倍率选项（不随其它筛选变化，只置灰不消失，避免筛选条抖动）。 */
const rateOptions = computed(() =>
  Array.from(new Set(groups.value.map((g) => g.rate_multiplier))).sort((a, b) => a - b),
)

const enabledRates = computed(() => new Set(groupsBeforeRate.value.map((g) => g.rate_multiplier)))

const filteredGroups = computed(() => {
  const rf = rateFilter.value
  if (rf === 'all') return groupsBeforeRate.value
  return groupsBeforeRate.value.filter((g) => g.rate_multiplier === rf)
})

/**
 * 折叠态卡片的「起价」文案。
 *
 * 起价口径由 sitePricing.groupStartingPrice 统一给：token 模型有阶梯时取各档最低输入价
 * （与展开表格逐档展示的数字同源，不会出现卡片说 $A、正下方表格全是 $B 的自相矛盾）；
 * 整组没有 token 价时回落到按次 / 按图单价，并换一句文案——$/1M 与 $/次单位不同，
 * 塞进同一句「输入低至」就是误导。
 */
const startingPriceLabels = computed<Record<number, string>>(() => {
  const out: Record<number, string> = {}
  for (const g of groups.value) {
    const sp = groupStartingPrice(g.models)
    if (!sp) continue
    const ctx: PriceCtx = { mode: 'site', rate: g.rate_multiplier, fxRate: fxRate.value }
    if (sp.kind === 'token') {
      out[g.id] = t('publicModels.fromPrice', { price: formatPerMillion(sp.value, ctx) })
      continue
    }
    const price = formatPerItem(sp.value, ctx)
    out[g.id] =
      sp.kind === 'image'
        ? t('publicModels.fromPriceImage', { price })
        : t('publicModels.fromPriceRequest', { price })
  }
  return out
})

/**
 * 允许显示「暂未公布价格」的分组：一个可展示价格都没有的才算。
 * 起价算不出来但确实配了价（例如只配了缓存价 / 价格恰为 0）时宁可留白——
 * 对已公布价格的分组说「暂未公布」是直接把转化打掉。
 */
const noPricingGroupIds = computed<Set<number>>(() => {
  const out = new Set<number>()
  for (const g of groups.value) {
    if (g.models.length && !hasPublishedPrice(g.models)) out.add(g.id)
  }
  return out
})

function toggleGroup(id: number) {
  expandedGroupId.value = expandedGroupId.value === id ? null : id
}

/** 搜索词命中该模型名。 */
function matchesQuery(model: UserPricingModel, q: string): boolean {
  return model.name.toLowerCase().includes(q)
}

/**
 * 折叠态 chip 用：截断到 MAX_MODELS_PER_CARD，命中项顶到前面。
 * 卡片只有一格宽，放不下上百个 chip，所以这里必须截断。
 */
function displayedModels(group: UserPricingGroup): UserPricingModel[] {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return group.models.slice(0, MAX_MODELS_PER_CARD)
  // Surface models that match the search first so the hit is visible on the card.
  const matched = group.models.filter((m) => matchesQuery(m, q))
  const rest = group.models.filter((m) => !matchesQuery(m, q))
  return [...matched, ...rest].slice(0, MAX_MODELS_PER_CARD)
}

/**
 * 展开态价格表用：按搜索过滤但**不截断**（展开卡横跨整行，容得下全部命中）。
 * 与 displayedModels 是两种消费场景，别合并：
 *   - chip   ：截断 + 命中优先，只是给个预览
 *   - 表格   ：搜什么就只看什么，截断会把命中项藏掉
 * 搜索词命中分组名 / 平台名时整组算命中（groupsBeforeRate 就是这么放行的），
 * 否则搜 "openai" 展开会得到一张空表。
 */
function expandedModels(group: UserPricingGroup): UserPricingModel[] {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return group.models
  if (group.name.toLowerCase().includes(q) || group.platform.toLowerCase().includes(q)) {
    return group.models
  }
  return group.models.filter((m) => matchesQuery(m, q))
}

function copyModel(name: string) {
  copyToClipboard(name)
}

// API 接入域名：优先后台配置的 api_base_url，否则回退当前站点 origin（与首页示例一致）
const apiBaseUrl = computed(() => {
  const configured = (appStore.cachedPublicSettings?.api_base_url || '').trim()
  if (configured) return configured.replace(/\/+$/, '')
  if (typeof window !== 'undefined' && window.location?.origin) return window.location.origin
  return ''
})

function copyApiBase() {
  if (apiBaseUrl.value) copyToClipboard(`${apiBaseUrl.value}/v1`)
}

const siteName = computed(() =>
  (merchantStore.isMerchantSite && merchantStore.siteName) ||
  appStore.cachedPublicSettings?.site_name ||
  appStore.siteName ||
  'Sub2API'
)
const siteLogo = computed(() =>
  (merchantStore.isMerchantSite && merchantStore.siteLogo) ||
  appStore.cachedPublicSettings?.site_logo ||
  appStore.siteLogo ||
  ''
)
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')

async function reload() {
  loading.value = true
  loadError.value = false
  try {
    // fx-rate 是公开路由，匿名可用；ensureFxRate 内部吞掉失败并回落默认汇率，
    // 所以它永远不会把整页拖成"加载失败"。
    const [list] = await Promise.all([userChannelsAPI.getPublicPricingGroups(), ensureFxRate()])
    groups.value = list
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
}

// platform / 搜索变化后当前倍率选项可能落空，自动回到「全部」避免出现无法自愈的空结果。
watch(enabledRates, (rates) => {
  if (rateFilter.value !== 'all' && !rates.has(rateFilter.value)) {
    rateFilter.value = 'all'
  }
})

// 展开的分组被筛掉后收起，否则筛选回来时会出现"记忆中的展开态"错位。
watch(filteredGroups, (list) => {
  if (expandedGroupId.value != null && !list.some((g) => g.id === expandedGroupId.value)) {
    expandedGroupId.value = null
  }
})

useHead(() => ({
  title: `${t('publicModels.pageTitle')} | ${siteName.value}`,
  htmlAttrs: { lang: locale.value },
  meta: [
    { name: 'description', content: t('publicModels.subtitle') },
    { property: 'og:type', content: 'website' },
    { property: 'og:title', content: `${t('publicModels.title')} | ${siteName.value}` },
    { property: 'og:description', content: t('publicModels.subtitle') },
    { property: 'og:site_name', content: siteName.value },
  ],
}))

onMounted(() => {
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
  reload()
})
</script>

<style scoped>
.platform-chip {
  @apply inline-flex items-center rounded-full border border-gray-200 bg-white px-3 py-1.5 text-xs font-medium text-gray-700
         transition hover:border-primary-300 hover:text-primary-700
         dark:border-dark-700 dark:bg-dark-800/40 dark:text-dark-200 dark:hover:border-primary-500/40;
}
.platform-chip-active {
  @apply border-primary-500 bg-primary-50 text-primary-700
         dark:border-primary-500 dark:bg-primary-500/15 dark:text-primary-200;
}
.platform-chip-disabled {
  @apply cursor-not-allowed opacity-40 hover:border-gray-200 hover:text-gray-700
         dark:hover:border-dark-700 dark:hover:text-dark-200;
}
.group-card {
  @apply flex flex-col rounded-xl border border-gray-200 bg-white p-5 transition-all duration-200
         hover:border-primary-200 hover:shadow-md
         dark:border-dark-700 dark:bg-dark-800/40 dark:hover:border-primary-500/40;
}
.expand-toggle {
  @apply inline-flex w-full items-center justify-center rounded-lg border border-gray-200 bg-gray-50 px-3 py-2
         text-xs font-medium text-gray-600 transition
         hover:border-primary-300 hover:text-primary-700
         dark:border-dark-700 dark:bg-dark-900/50 dark:text-dark-300
         dark:hover:border-primary-500/40 dark:hover:text-primary-200;
}
.model-chip {
  @apply inline-flex items-center rounded-md border border-gray-200 bg-gray-50 px-2 py-0.5 font-mono text-[12px] text-gray-700
         dark:border-dark-700 dark:bg-dark-800 dark:text-dark-200;
}
button.model-chip {
  @apply cursor-pointer transition-colors hover:border-primary-300 hover:bg-primary-50 hover:text-primary-700
         dark:hover:border-primary-500/40 dark:hover:bg-primary-500/10 dark:hover:text-primary-200;
}
.model-chip-more {
  @apply border-primary-200 bg-primary-50 text-primary-700 dark:border-primary-500/30 dark:bg-primary-500/10 dark:text-primary-200;
}
</style>
