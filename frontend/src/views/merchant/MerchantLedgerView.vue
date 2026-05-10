<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex items-center justify-between gap-3">
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('merchant.owner.ledger.title') }}
          </h1>
          <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="load">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
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
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const items = ref<MerchantLedgerEntry[]>([])
const total = ref(0)
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)

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
    const res = await merchantAPI.listLedger(offset, pageSize.value)
    items.value = res.items || []
    total.value = res.total || 0
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
