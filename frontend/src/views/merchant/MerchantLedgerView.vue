<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="space-y-4">
          <!-- 标题 + 刷新 -->
          <div class="flex items-center justify-between gap-3">
            <div>
              <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
                {{ t('merchant.owner.stats.title') }}
              </h1>
              <p class="text-sm text-gray-500 dark:text-dark-400">
                {{ t('merchant.owner.stats.description') }}
              </p>
            </div>
            <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="load">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>

          <!-- 5 个指标卡片 -->
          <div class="grid grid-cols-1 gap-4 md:grid-cols-3 xl:grid-cols-5">
            <div class="card p-4">
              <div class="text-xs text-gray-500">{{ t('merchant.owner.stats.totalRecharge') }}</div>
              <div class="mt-1 text-2xl font-bold text-purple-600">¥{{ fmt(stats?.total_recharge) }}</div>
            </div>
            <div class="card p-4">
              <div class="text-xs text-gray-500">{{ t('merchant.owner.stats.totalProfit') }}</div>
              <div class="mt-1 text-2xl font-bold text-emerald-600">¥{{ fmt(stats?.total_profit) }}</div>
              <div v-if="selfRechargeAmount > 0" class="mt-1 text-[11px] text-gray-400">
                {{ t('merchant.owner.stats.totalProfitHint', { amount: fmt(selfRechargeAmount) }) }}
              </div>
            </div>
            <div class="card p-4">
              <div class="text-xs text-gray-500">{{ t('merchant.owner.stats.withdrawn') }}</div>
              <div class="mt-1 text-2xl font-bold">¥{{ fmt(stats?.withdrawn_amount) }}</div>
            </div>
            <div class="card p-4">
              <div class="text-xs text-gray-500">{{ t('merchant.owner.stats.pending') }}</div>
              <div class="mt-1 text-2xl font-bold text-amber-600">¥{{ fmt(stats?.pending_withdraw) }}</div>
            </div>
            <div class="card p-4">
              <div class="text-xs text-gray-500">{{ t('merchant.owner.stats.available') }}</div>
              <div class="mt-1 text-2xl font-bold text-blue-600">¥{{ fmt(stats?.available_balance) }}</div>
            </div>
          </div>

          <!-- 资金明细子标题 -->
          <div class="pt-2">
            <h2 class="text-sm font-semibold text-gray-700 dark:text-gray-200">
              {{ t('merchant.owner.ledger.title') }}
            </h2>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="items" :loading="loading">
          <template #cell-direction="{ value }">
            <span
              :class="[
                'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
                value === 'credit'
                  ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
                  : 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400',
              ]"
            >
              {{ t('merchant.ledger.direction.' + value) }}
            </span>
          </template>
          <template #cell-amount="{ value }">
            <span class="font-mono text-sm">${{ Number(value || 0).toFixed(4) }}</span>
          </template>
          <template #cell-balance_after="{ value }">
            <span class="font-mono text-sm">${{ Number(value || 0).toFixed(4) }}</span>
          </template>
          <template #cell-created_at="{ value }">
            <span class="text-sm">{{ formatDateTime(value) }}</span>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="total > pageSize"
          :total="total"
          :page="page"
          :page-size="pageSize"
          @update:page="onPageChange"
          @update:pageSize="onPageSizeChange"
        />
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { merchantAPI, type MerchantLedgerEntry } from '@/api'
import type { MerchantStats } from '@/api/merchant'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const items = ref<MerchantLedgerEntry[]>([])
const total = ref(0)
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)

// 顶部统计卡片：总充值 / 累计利润 / 已提现 / 待提现 / 可用余额。
const stats = ref<MerchantStats | null>(null)

// 累计自充值 = total_share − total_profit；用作"累计利润"卡片底部提示。
const selfRechargeAmount = computed(() => {
  if (!stats.value) return 0
  const v = Number(stats.value.total_share ?? 0) - Number(stats.value.total_profit ?? 0)
  return v > 0 ? v : 0
})

function fmt(n?: number): string {
  return Number(n ?? 0).toFixed(2)
}

const columns = computed<Column[]>(() => [
  { key: 'id', label: t('merchant.fields.id') },
  { key: 'created_at', label: t('merchant.fields.time') },
  { key: 'direction', label: t('merchant.ledger.directionLabel') },
  { key: 'amount', label: t('merchant.fields.amount') },
  { key: 'balance_after', label: t('merchant.fields.balanceAfter') },
  { key: 'source', label: t('merchant.fields.source') },
  { key: 'ref_type', label: t('merchant.fields.refType') },
  { key: 'ref_id', label: t('merchant.fields.refId') },
])

async function load(): Promise<void> {
  loading.value = true
  try {
    const offset = (page.value - 1) * pageSize.value
    // 统计和流水分页并发拉取；统计失败不阻塞表格，降级为卡片显示 0。
    const [ledger, s] = await Promise.all([
      merchantAPI.listLedger(offset, pageSize.value),
      merchantAPI.stats().catch(() => null),
    ])
    items.value = ledger.items || []
    total.value = ledger.total || 0
    stats.value = s
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

function onPageChange(p: number): void {
  page.value = p
  void load()
}

function onPageSizeChange(s: number): void {
  pageSize.value = s
  page.value = 1
  void load()
}

onMounted(() => {
  void load()
})
</script>
