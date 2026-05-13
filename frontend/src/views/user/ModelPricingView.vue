<template>
  <AppLayout>
    <div class="flex h-full flex-col gap-6">
      <!-- 顶部过滤条 -->
      <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div class="relative w-full sm:w-80">
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
        <!-- 左：端点列表 -->
        <div class="card flex flex-col overflow-hidden">
          <div class="flex items-center gap-2 border-b border-gray-100 px-5 py-4 dark:border-dark-700/50">
            <Icon name="server" size="md" class="text-primary-500" />
            <h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">
              {{ t('modelPricing.endpoints') }}
            </h2>
            <span class="ml-auto text-xs text-gray-400">{{ filteredChannels.length }}</span>
          </div>

          <div class="flex-1 space-y-3 overflow-y-auto px-3 py-3">
            <template v-if="loading && !channels.length">
              <div v-for="i in 4" :key="i" class="h-20 animate-pulse rounded-xl bg-gray-100 dark:bg-dark-700/50"></div>
            </template>

            <div
              v-else-if="!filteredChannels.length"
              class="flex flex-col items-center justify-center gap-2 px-4 py-12 text-center text-gray-400 dark:text-gray-500"
            >
              <Icon name="inbox" size="xl" />
              <p class="text-sm">{{ t('modelPricing.empty') }}</p>
            </div>

            <button
              v-for="ch in filteredChannels"
              :key="ch.name"
              type="button"
              class="endpoint-card group"
              :class="{ 'endpoint-card-active': ch.name === selectedChannelName }"
              @click="selectedChannelName = ch.name"
            >
              <div class="flex items-start justify-between gap-2">
                <div class="min-w-0 flex-1">
                  <div class="truncate text-sm font-semibold text-gray-900 dark:text-gray-100">
                    {{ ch.name }}
                  </div>
                  <div v-if="ch.description" class="mt-1 line-clamp-2 text-xs text-gray-500 dark:text-gray-400">
                    {{ ch.description }}
                  </div>
                </div>
                <span class="rate-badge" :class="rateBadgeTone(channelBestRate(ch))">
                  {{ t('modelPricing.rateLabel') }}: {{ formatRate(channelBestRate(ch)) }}
                </span>
              </div>
              <div class="mt-2 flex flex-wrap gap-1">
                <span
                  v-for="p in ch.platforms.slice(0, 4)"
                  :key="p.platform"
                  class="platform-chip"
                >
                  {{ p.platform }}
                </span>
                <span v-if="ch.platforms.length > 4" class="platform-chip">+{{ ch.platforms.length - 4 }}</span>
              </div>
            </button>
          </div>
        </div>

        <!-- 右：详情 -->
        <div class="card flex min-h-0 flex-col overflow-hidden">
          <template v-if="selectedChannel">
            <div class="flex flex-wrap items-center gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700/50">
              <Icon name="cube" size="md" class="text-primary-500" />
              <h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">
                {{ selectedChannel.name }}
              </h2>
              <span class="rate-badge" :class="rateBadgeTone(selectedRate)">
                {{ t('modelPricing.rateLabel') }}: {{ formatRate(selectedRate) }}
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
              <div
                v-for="section in selectedChannel.platforms"
                :key="section.platform"
                class="border-b border-gray-100 last:border-b-0 dark:border-dark-800"
              >
                <div class="flex items-center gap-2 bg-gray-50/60 px-5 py-2 text-xs font-medium text-gray-500 dark:bg-dark-800/40 dark:text-gray-400">
                  <Icon name="cube" size="sm" />
                  <span>{{ section.platform }}</span>
                  <span class="text-gray-300 dark:text-gray-600">·</span>
                  <span>{{ section.supported_models.length }} {{ t('modelPricing.modelsUnit') }}</span>
                </div>

                <div v-if="!section.supported_models.length" class="px-5 py-6 text-center text-sm text-gray-400">
                  {{ t('modelPricing.noModels') }}
                </div>

                <table v-else class="w-full text-sm">
                  <thead>
                    <tr class="text-left text-xs font-medium uppercase tracking-wide text-gray-400 dark:text-gray-500">
                      <th class="px-5 py-3">{{ t('modelPricing.columns.model') }}</th>
                      <th class="px-3 py-3 text-right">{{ t('modelPricing.columns.input') }}</th>
                      <th class="px-3 py-3 text-right">{{ t('modelPricing.columns.output') }}</th>
                      <th class="px-3 py-3 text-right">{{ t('modelPricing.columns.cacheWrite') }}</th>
                      <th class="px-3 py-3 text-right">{{ t('modelPricing.columns.cacheRead') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr
                      v-for="model in section.supported_models"
                      :key="model.platform + '/' + model.name"
                      class="border-t border-gray-50 transition hover:bg-primary-50/30 dark:border-dark-800 dark:hover:bg-primary-900/10"
                    >
                      <td class="px-5 py-3">
                        <div class="font-mono text-sm font-medium text-gray-900 dark:text-gray-100">
                          {{ model.name }}
                        </div>
                      </td>
                      <td class="px-3 py-3 text-right font-mono text-sm" :class="priceCellTone">
                        {{ formatPrice(model.pricing?.official_input_price) }}
                      </td>
                      <td class="px-3 py-3 text-right font-mono text-sm" :class="priceCellTone">
                        {{ formatPrice(model.pricing?.official_output_price) }}
                      </td>
                      <td class="px-3 py-3 text-right font-mono text-sm" :class="priceCellTone">
                        {{ formatPrice(model.pricing?.official_cache_write_price) }}
                      </td>
                      <td class="px-3 py-3 text-right font-mono text-sm" :class="priceCellTone">
                        {{ formatPrice(model.pricing?.official_cache_read_price) }}
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
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
import userChannelsAPI, { type UserAvailableChannel } from '@/api/channels'
import systemAPI from '@/api/system'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { DEFAULT_CNY_PER_USD } from '@/utils/pricing'

const { t } = useI18n()
const appStore = useAppStore()

const channels = ref<UserAvailableChannel[]>([])
const loading = ref(false)
const searchQuery = ref('')
const priceMode = ref<'official' | 'site'>('site')
const selectedChannelName = ref<string>('')
const fxRate = ref<number>(DEFAULT_CNY_PER_USD)

const filteredChannels = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return channels.value
  return channels.value
    .map((ch) => {
      const nameHit = ch.name.toLowerCase().includes(q)
      const descHit = (ch.description || '').toLowerCase().includes(q)
      if (nameHit || descHit) return ch
      const matched = ch.platforms.filter(
        (p) =>
          p.platform.toLowerCase().includes(q) ||
          p.supported_models.some((m) => m.name.toLowerCase().includes(q)),
      )
      if (!matched.length) return null
      return { ...ch, platforms: matched }
    })
    .filter((ch): ch is UserAvailableChannel => ch !== null)
})

const selectedChannel = computed(() =>
  filteredChannels.value.find((c) => c.name === selectedChannelName.value) ?? null,
)

// 同一渠道下可能有多个分组，每个分组有自己的倍率；展示用"用户最优价"——
// 取所有可见分组里最小的倍率，因为倍率越小用户付得越少。
function channelBestRate(ch: UserAvailableChannel): number {
  let best = Infinity
  for (const p of ch.platforms) {
    for (const g of p.groups) {
      if (typeof g.rate_multiplier === 'number' && g.rate_multiplier < best) {
        best = g.rate_multiplier
      }
    }
  }
  return Number.isFinite(best) ? best : 1
}

const selectedRate = computed(() =>
  selectedChannel.value ? channelBestRate(selectedChannel.value) : 1,
)

function formatRate(rate: number): string {
  const r = Number(rate || 1)
  if (Math.abs(r - 1) < 1e-6) return '1x'
  if (r >= 10) return `${r.toFixed(0)}x`
  return `${parseFloat(r.toFixed(3))}x`
}

function rateBadgeTone(rate: number): string {
  if (rate < 1) return 'rate-badge-good'
  if (rate > 1) return 'rate-badge-warn'
  return 'rate-badge-neutral'
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
 *
 * 站点用户充值 ¥1 = $1 名义额度，但市场汇率是 ¥{fx} = $1，所以名义 $X
 * 对应的真实美元等效是 $X / fx。因此本站价 = group.rate × 官方价 / fx。
 */
function formatPrice(perTokenUSD: number | null | undefined): string {
  if (perTokenUSD == null) return '-'
  const officialPerM = perTokenUSD * 1_000_000
  if (priceMode.value === 'official') {
    return `$${trimNum(officialPerM)}/M`
  }
  const sitePerM = (selectedRate.value / fxRate.value) * officialPerM
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
      userChannelsAPI.getAvailable(),
      systemAPI.getFXRate().catch(() => null),
    ])
    channels.value = list
    if (fx && fx.cny_per_usd > 0) fxRate.value = fx.cny_per_usd
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    loading.value = false
  }
}

watch(filteredChannels, (list) => {
  if (!list.length) {
    selectedChannelName.value = ''
    return
  }
  if (!list.some((c) => c.name === selectedChannelName.value)) {
    selectedChannelName.value = list[0].name
  }
}, { immediate: true })

onMounted(reload)
</script>

<style scoped>
.endpoint-card {
  @apply w-full rounded-xl border border-gray-100 bg-white px-4 py-3 text-left
         transition hover:border-primary-200 hover:bg-primary-50/40 hover:shadow-sm
         dark:border-dark-700/50 dark:bg-dark-800/40 dark:hover:border-primary-500/40
         dark:hover:bg-primary-900/10;
}

.endpoint-card-active {
  @apply border-primary-300 bg-gradient-to-br from-primary-50 to-white shadow-sm
         dark:border-primary-500/60 dark:from-primary-900/20 dark:to-dark-800/40;
}

.rate-badge {
  @apply inline-flex flex-shrink-0 items-center rounded-full px-2 py-0.5 text-xs font-semibold;
}

.rate-badge-good {
  @apply bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300;
}

.rate-badge-warn {
  @apply bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300;
}

.rate-badge-neutral {
  @apply bg-gray-100 text-gray-600 dark:bg-dark-700/60 dark:text-gray-300;
}

.platform-chip {
  @apply inline-flex items-center rounded-md bg-gray-100 px-1.5 py-0.5 text-[10px]
         font-medium text-gray-600 dark:bg-dark-700/50 dark:text-gray-300;
}
</style>
