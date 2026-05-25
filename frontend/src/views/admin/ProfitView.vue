<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- 顶部筛选条 -->
      <div class="card p-4">
        <div class="flex flex-wrap items-center gap-4">
          <div class="flex items-center gap-2">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.dashboard.timeRange') }}:
            </span>
            <DateRangePicker
              v-model:start-date="startDate"
              v-model:end-date="endDate"
              @change="loadSummary"
            />
          </div>
          <div class="ml-auto flex items-center gap-2">
            <button class="btn btn-secondary" :disabled="loading" @click="loadSummary">
              {{ t('common.refresh') }}
            </button>
          </div>
        </div>
      </div>

      <!-- 维度 tabs -->
      <div class="card p-4">
        <div class="flex flex-wrap items-center gap-2">
          <button
            v-for="tab in dimensionTabs"
            :key="tab.id"
            class="rounded-md px-4 py-2 text-sm font-medium transition-colors"
            :class="
              activeDimension === tab.id
                ? 'bg-primary-500 text-white'
                : 'bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600'
            "
            @click="switchDimension(tab.id)"
          >
            {{ tab.label }}
          </button>

          <!-- 团队维度：嵌套属性选择（分公司/销售/...） -->
          <template v-if="activeDimension === 'team' && attributeDefs.length > 0">
            <span class="ml-2 text-sm text-gray-500 dark:text-gray-400">/</span>
            <button
              v-for="def in attributeDefs"
              :key="def.id"
              class="rounded-md px-3 py-1.5 text-sm transition-colors"
              :class="
                selectedAttrId === def.id
                  ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300'
                  : 'bg-gray-50 text-gray-600 hover:bg-gray-100 dark:bg-dark-800 dark:text-gray-400 dark:hover:bg-dark-700'
              "
              @click="selectAttribute(def.id)"
            >
              {{ def.name }}
            </button>
          </template>
          <span
            v-else-if="activeDimension === 'team' && !attributeDefs.length"
            class="text-sm text-gray-500 dark:text-gray-400"
          >
            {{ t('admin.profit.noAttributes') }}
          </span>
        </div>
      </div>

      <!-- 统计卡片 -->
      <div class="grid grid-cols-1 gap-4 md:grid-cols-4">
        <StatCard
          :title="t('admin.profit.totalRevenue')"
          :value="formatCurrency(summary?.total_revenue ?? 0)"
        />
        <StatCard
          :title="t('admin.profit.totalCost')"
          :value="formatCurrency(summary?.total_cost ?? 0)"
        />
        <StatCard
          :title="t('admin.profit.totalProfit')"
          :value="formatCurrency(summary?.total_profit ?? 0)"
        />
        <StatCard :title="t('admin.profit.profitRate')" :value="overallProfitRateText" />
      </div>

      <!-- 表格 -->
      <div class="card overflow-hidden">
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-600">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  {{ dimensionColumnLabel }}
                </th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  {{ t('admin.profit.revenue') }}
                </th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  {{ t('admin.profit.cost') }}
                </th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  {{ t('admin.profit.profit') }}
                </th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  {{ t('admin.profit.profitRate') }}
                </th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  {{ t('admin.profit.requests') }}
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-600 dark:bg-dark-900">
              <tr v-if="loading">
                <td colspan="6" class="px-4 py-12 text-center text-sm text-gray-500 dark:text-gray-400">
                  {{ t('common.loading') }}
                </td>
              </tr>
              <tr v-else-if="!summary?.rows.length">
                <td colspan="6" class="px-4 py-12 text-center text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.noDataAvailable') }}
                </td>
              </tr>
              <tr
                v-for="row in summary?.rows ?? []"
                v-else
                :key="row.key"
                class="hover:bg-gray-50 dark:hover:bg-dark-800"
              >
                <td class="whitespace-nowrap px-4 py-3 text-sm text-gray-900 dark:text-gray-100">
                  {{ displayName(row) }}
                </td>
                <td class="whitespace-nowrap px-4 py-3 text-right text-sm tabular-nums text-gray-900 dark:text-gray-100">
                  {{ formatCurrency(row.revenue) }}
                </td>
                <td class="whitespace-nowrap px-4 py-3 text-right text-sm tabular-nums text-gray-900 dark:text-gray-100">
                  {{ formatCurrency(row.cost) }}
                </td>
                <td
                  class="whitespace-nowrap px-4 py-3 text-right text-sm tabular-nums"
                  :class="row.profit >= 0 ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'"
                >
                  {{ formatCurrency(row.profit) }}
                </td>
                <td class="whitespace-nowrap px-4 py-3 text-right text-sm tabular-nums text-gray-700 dark:text-gray-300">
                  {{ formatPercent(row.profit_rate) }}
                </td>
                <td class="whitespace-nowrap px-4 py-3 text-right text-sm tabular-nums text-gray-700 dark:text-gray-300">
                  {{ row.request_count.toLocaleString() }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import StatCard from '@/components/common/StatCard.vue'
import { adminAPI } from '@/api/admin'
import type { ProfitGroupBy, ProfitRow, ProfitSummaryResponse } from '@/api/admin/profit'
import type { UserAttributeDefinition } from '@/types'
import { formatCurrency } from '@/utils/format'

const { t } = useI18n()

type DimensionId = 'merchant' | 'user' | 'team'

const dimensionTabs = computed(() => [
  { id: 'merchant' as DimensionId, label: t('admin.profit.tabs.merchant') },
  { id: 'user' as DimensionId, label: t('admin.profit.tabs.user') },
  { id: 'team' as DimensionId, label: t('admin.profit.tabs.team') },
])

const dimensionColumnLabel = computed(() => {
  if (activeDimension.value === 'merchant') return t('admin.profit.colMerchant')
  if (activeDimension.value === 'user') return t('admin.profit.colUser')
  const def = attributeDefs.value.find((d) => d.id === selectedAttrId.value)
  return def?.name ?? t('admin.profit.colAttribute')
})

const today = new Date()
const thirtyDaysAgo = new Date(today.getTime() - 30 * 24 * 60 * 60 * 1000)
const formatDate = (d: Date) => {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

const startDate = ref(formatDate(thirtyDaysAgo))
const endDate = ref(formatDate(today))
const activeDimension = ref<DimensionId>('merchant')
const selectedAttrId = ref<number | null>(null)
const attributeDefs = ref<UserAttributeDefinition[]>([])
const summary = ref<ProfitSummaryResponse | null>(null)
const loading = ref(false)

const overallProfitRateText = computed(() => {
  if (!summary.value || summary.value.total_revenue <= 0) return '—'
  return formatPercent(summary.value.total_profit / summary.value.total_revenue)
})

function formatPercent(rate: number): string {
  if (!Number.isFinite(rate)) return '—'
  return `${(rate * 100).toFixed(2)}%`
}

function displayName(row: ProfitRow): string {
  if (row.key === '__unassigned__') return t('admin.profit.unassigned')
  return row.name || row.key
}

// DateRangePicker emits YYYY-MM-DD local dates; the backend expects RFC3339.
// 把"开始"取本地零点、"结束"取本地次日零点（开区间），覆盖完整自然日。
function toStartRFC3339(date: string): string {
  return new Date(`${date}T00:00:00`).toISOString()
}
function toEndRFC3339(date: string): string {
  const d = new Date(`${date}T00:00:00`)
  d.setDate(d.getDate() + 1)
  return d.toISOString()
}

function switchDimension(id: DimensionId) {
  if (activeDimension.value === id) return
  activeDimension.value = id
  loadSummary()
}

function selectAttribute(id: number) {
  if (selectedAttrId.value === id) return
  selectedAttrId.value = id
  loadSummary()
}

function currentGroupBy(): ProfitGroupBy {
  if (activeDimension.value === 'team') return 'attribute'
  return activeDimension.value
}

async function loadSummary() {
  if (activeDimension.value === 'team' && !selectedAttrId.value) {
    summary.value = null
    return
  }
  loading.value = true
  try {
    const res = await adminAPI.profit.getSummary({
      start: toStartRFC3339(startDate.value),
      end: toEndRFC3339(endDate.value),
      group_by: currentGroupBy(),
      attribute_id:
        activeDimension.value === 'team' ? selectedAttrId.value ?? undefined : undefined,
    })
    summary.value = res
  } catch (e) {
    console.error('Failed to load profit summary:', e)
    summary.value = null
  } finally {
    loading.value = false
  }
}

async function loadAttributeDefs() {
  try {
    attributeDefs.value = await adminAPI.userAttributes.listEnabledDefinitions()
    if (attributeDefs.value.length > 0 && selectedAttrId.value === null) {
      selectedAttrId.value = attributeDefs.value[0].id
    }
  } catch (e) {
    console.error('Failed to load attribute definitions:', e)
  }
}

onMounted(async () => {
  await loadAttributeDefs()
  await loadSummary()
})
</script>
