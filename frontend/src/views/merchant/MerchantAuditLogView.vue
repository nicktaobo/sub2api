<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex items-center justify-between gap-3">
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('merchant.owner.audit.title') }}
          </h1>
          <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="load">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="items" :loading="loading">
          <template #cell-actor="{ row }">
            <span class="text-sm">#{{ row.actor_user_id }}</span>
            <span v-if="row.actor_email" class="ml-1 text-xs text-gray-500">{{ row.actor_email }}</span>
          </template>
          <template #cell-action="{ value }">
            <span class="font-mono text-xs">{{ value }}</span>
          </template>
          <template #cell-reason="{ value }">
            <span class="text-sm">{{ value || '-' }}</span>
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
import { merchantAPI, type MerchantAuditLogEntry } from '@/api'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const items = ref<MerchantAuditLogEntry[]>([])
const total = ref(0)
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)

const columns = computed<Column[]>(() => [
  { key: 'id', label: t('merchant.fields.id') },
  { key: 'created_at', label: t('merchant.fields.time') },
  { key: 'actor', label: t('merchant.fields.actor') },
  { key: 'action', label: t('merchant.fields.action') },
  { key: 'reason', label: t('merchant.fields.reason') },
])

async function load(): Promise<void> {
  loading.value = true
  try {
    const offset = (page.value - 1) * pageSize.value
    const res = await merchantAPI.listAuditLog(offset, pageSize.value)
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
