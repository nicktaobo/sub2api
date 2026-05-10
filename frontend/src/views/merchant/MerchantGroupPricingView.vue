<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex items-center justify-between gap-3">
        <div>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('merchant.owner.pricing.title') }}
          </h1>
          <p class="text-sm text-gray-500 dark:text-dark-400">
            {{ t('merchant.owner.pricing.description') }}
          </p>
        </div>
        <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="load">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>

      <div class="card p-6">
        <div class="text-sm text-gray-500">{{ t('merchant.owner.pricing.defaultMarkup') }}</div>
        <div class="mt-1 font-mono text-2xl font-semibold">
          {{ defaultMarkup.toFixed(4) }}
        </div>
      </div>

      <div class="card">
        <div class="border-b border-gray-200 px-4 py-3 dark:border-dark-700">
          <h2 class="text-sm font-semibold text-gray-700 dark:text-gray-200">
            {{ t('merchant.owner.pricing.overridesTitle') }}
          </h2>
        </div>
        <DataTable :columns="columns" :data="markups" :loading="loading">
          <template #cell-group_id="{ value }">
            <span class="font-mono text-sm">#{{ value }}</span>
          </template>
          <template #cell-markup="{ value }">
            <span class="font-mono text-sm">{{ Number(value || 1).toFixed(4) }}</span>
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
import { merchantAPI, type MerchantGroupMarkup, type MerchantInfo } from '@/api'
import { extractI18nErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const markups = ref<MerchantGroupMarkup[]>([])
const info = ref<MerchantInfo | null>(null)
const loading = ref(false)

const defaultMarkup = computed(() => Number(info.value?.user_markup_default ?? 1))

const columns = computed<Column[]>(() => [
  { key: 'group_id', label: t('merchant.detail.groupPricing.group') },
  { key: 'markup', label: t('merchant.fields.markup') },
])

async function load(): Promise<void> {
  loading.value = true
  try {
    const [m, list] = await Promise.all([
      merchantAPI.info(),
      merchantAPI.listGroupMarkups(),
    ])
    info.value = m
    markups.value = list
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
