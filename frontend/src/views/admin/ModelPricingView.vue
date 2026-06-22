<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex items-center justify-center py-16">
        <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
      </div>

      <template v-else>
        <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('admin.modelPricing.title') }}</h1>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.modelPricing.description') }}</p>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button type="button" class="btn btn-secondary inline-flex items-center gap-2" @click="addRow">
              <Icon name="plus" size="sm" />
              {{ t('admin.modelPricing.addRow') }}
            </button>
            <button type="button" class="btn btn-primary inline-flex items-center gap-2" :disabled="saving" @click="handleSave">
              <Icon name="check" size="sm" />
              {{ t('admin.modelPricing.save') }}
            </button>
          </div>
        </div>

        <div class="overflow-x-auto rounded-lg border border-gray-100 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <table class="min-w-full text-sm">
            <thead>
              <tr class="border-b border-gray-100 text-left text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400">
                <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.columns.model') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.columns.currency') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.columns.input') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.columns.output') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.columns.cache') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.columns.hasCache') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.columns.enabled') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.columns.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(row, i) in rows" :key="i" class="border-b border-gray-50 dark:border-dark-700/60">
                <td class="px-3 py-2">
                  <input v-model="row.model" type="text" class="input w-48" placeholder="qwen-plus" />
                </td>
                <td class="px-3 py-2">
                  <Select v-model="row.currency" :options="currencyOptions" class="w-28" />
                </td>
                <td class="px-3 py-2">
                  <input v-model.number="row.input" type="number" min="0" step="0.01" class="input w-28" />
                </td>
                <td class="px-3 py-2">
                  <input v-model.number="row.output" type="number" min="0" step="0.01" class="input w-28" />
                </td>
                <td class="px-3 py-2">
                  <input v-model.number="row.cache" type="number" min="0" step="0.01" class="input w-28" :disabled="!row.has_cache" />
                </td>
                <td class="px-3 py-2">
                  <Toggle v-model="row.has_cache" />
                </td>
                <td class="px-3 py-2">
                  <Toggle v-model="row.enabled" />
                </td>
                <td class="px-3 py-2">
                  <button type="button" class="text-red-500 hover:text-red-600" :title="t('admin.modelPricing.columns.actions')" @click="removeRow(i)">
                    <Icon name="trash" size="sm" />
                  </button>
                </td>
              </tr>
              <tr v-if="rows.length === 0">
                <td colspan="8" class="px-3 py-10 text-center text-sm text-gray-400">{{ t('admin.modelPricing.empty') }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <p class="text-xs text-gray-400">{{ t('admin.modelPricing.hint') }}</p>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import { adminAPI } from '@/api/admin'
import type { ModelPricingEntry } from '@/api/admin'
import type { SelectOption } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const saving = ref(false)
const rows = ref<ModelPricingEntry[]>([])

const currencyOptions: SelectOption[] = [
  { value: 'CNY', label: '¥ CNY' },
  { value: 'USD', label: '$ USD' }
]

async function loadConfig() {
  loading.value = true
  try {
    const cfg = await adminAPI.modelPricing.getModelPricing()
    rows.value = cfg.entries ?? []
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.modelPricing.loadFailed')))
  } finally {
    loading.value = false
  }
}

function addRow() {
  rows.value.push({ model: '', currency: 'CNY', input: 0, output: 0, cache: 0, has_cache: false, enabled: true })
}

function removeRow(i: number) {
  rows.value.splice(i, 1)
}

async function handleSave() {
  saving.value = true
  try {
    // 价格框清空时 v-model.number 会产出空字符串，需转数兜底，否则后端 float 解析返回 400。
    const entries = rows.value.map((r) => ({
      ...r,
      input: Number(r.input) || 0,
      output: Number(r.output) || 0,
      cache: Number(r.cache) || 0
    }))
    const cfg = await adminAPI.modelPricing.updateModelPricing({ entries })
    rows.value = cfg.entries ?? []
    appStore.showSuccess(t('admin.modelPricing.saveSuccess'))
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.modelPricing.saveFailed')))
  } finally {
    saving.value = false
  }
}

onMounted(loadConfig)
</script>
