<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex items-center justify-between gap-3">
        <div>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('merchant.owner.domains.title') }}
          </h1>
          <p class="text-sm text-gray-500 dark:text-dark-400">
            {{ t('merchant.owner.domains.description') }}
          </p>
        </div>
        <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="load">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>

      <div class="card">
        <DataTable :columns="columns" :data="items" :loading="loading">
          <template #cell-is_primary="{ value }">
            <span v-if="value" class="text-emerald-600">{{ t('common.yes') }}</span>
            <span v-else class="text-gray-400">{{ t('common.no') }}</span>
          </template>
          <template #cell-created_at="{ value }">
            <span class="text-sm">{{ value ? formatDateTime(value) : '-' }}</span>
          </template>
        </DataTable>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { merchantAPI, type MerchantDomain } from '@/api'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const items = ref<MerchantDomain[]>([])
const loading = ref(false)

const columns = computed<Column[]>(() => [
  { key: 'id', label: t('merchant.fields.id') },
  { key: 'domain', label: t('merchant.owner.domains.domain') },
  { key: 'is_primary', label: t('merchant.owner.domains.isPrimary') },
  { key: 'created_at', label: t('merchant.fields.createdAt') },
])

async function load(): Promise<void> {
  loading.value = true
  try {
    items.value = await merchantAPI.listDomains()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void load()
})
</script>
