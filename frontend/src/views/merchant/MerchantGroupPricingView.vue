<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- 标题 + 刷新 -->
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

      <!-- 默认 markup 卡片 -->
      <div class="card p-6">
        <div class="flex items-start justify-between gap-4">
          <div>
            <div class="text-sm text-gray-500">{{ t('merchant.owner.pricing.defaultMarkup') }}</div>
            <div class="mt-1 font-mono text-2xl font-semibold">
              {{ defaultMarkup.toFixed(4) }}
            </div>
            <p class="mt-2 text-xs text-gray-400">
              {{ t('merchant.owner.pricing.defaultMarkupHint') }}
            </p>
          </div>
          <button class="btn btn-secondary" @click="openDefaultForm">
            <Icon name="edit" size="sm" />
            <span class="ml-1">{{ t('common.edit') }}</span>
          </button>
        </div>
      </div>

      <!-- 分组定价列表（平铺所有可定价分组） -->
      <div class="card">
        <div class="border-b border-gray-200 px-4 py-3 dark:border-dark-700">
          <h2 class="text-sm font-semibold text-gray-700 dark:text-gray-200">
            {{ t('merchant.owner.pricing.groupsTitle') }}
          </h2>
          <p class="mt-0.5 text-xs text-gray-500">
            {{ t('merchant.owner.pricing.groupsHint') }}
          </p>
        </div>
        <DataTable :columns="columns" :data="groups" :loading="loading">
          <template #cell-name="{ row }">
            <div class="flex items-center gap-2">
              <span class="font-medium text-gray-900 dark:text-gray-100">{{ row.name }}</span>
              <span class="text-xs text-gray-400">#{{ row.id }}</span>
              <span
                :class="row.is_exclusive
                  ? 'rounded-full bg-amber-100 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
                  : 'rounded-full bg-emerald-100 px-1.5 py-0.5 text-[10px] font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'"
              >
                {{ row.is_exclusive ? t('merchant.owner.pricing.typeExclusive') : t('merchant.owner.pricing.typePublic') }}
              </span>
            </div>
          </template>
          <template #cell-rate_multiplier="{ value }">
            <span class="font-mono text-sm text-gray-600 dark:text-gray-300">{{ Number(value || 1).toFixed(4) }}</span>
          </template>
          <template #cell-markup="{ row }">
            <div class="flex items-center gap-2">
              <span class="font-mono text-sm font-semibold">
                {{ effectiveMarkup(row).toFixed(4) }}
              </span>
              <span
                v-if="row.markup != null"
                class="rounded-full bg-primary-100 px-1.5 py-0.5 text-[10px] font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300"
              >
                {{ t('merchant.owner.pricing.customLabel') }}
              </span>
              <span v-else class="text-[10px] text-gray-400">
                {{ t('merchant.owner.pricing.inheritedLabel') }}
              </span>
            </div>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex gap-2">
              <button class="text-sm text-primary-600 hover:underline dark:text-primary-400" @click="openMarkupForm(row)">
                {{ t('common.edit') }}
              </button>
              <button
                v-if="row.markup != null"
                class="text-sm text-rose-600 hover:underline dark:text-rose-400"
                @click="confirmReset(row)"
              >
                {{ t('merchant.owner.pricing.resetToDefault') }}
              </button>
            </div>
          </template>
        </DataTable>
      </div>
    </div>

    <!-- 默认 markup 编辑弹框 -->
    <BaseDialog
      :show="defaultDialog.show"
      :title="t('merchant.owner.pricing.editDefaultTitle')"
      width="normal"
      @close="defaultDialog.show = false"
    >
      <form id="merchant-default-markup-form" class="space-y-4" @submit.prevent="submitDefault">
        <div>
          <label class="input-label">{{ t('merchant.fields.markup') }}</label>
          <input
            v-model.number="defaultDialog.markup"
            type="number"
            min="1"
            step="0.0001"
            required
            class="input"
          />
          <p class="mt-1 text-xs text-gray-500">{{ t('merchant.owner.pricing.markupHint') }}</p>
          <div v-if="defaultDialog.markup > 2" class="mt-1 text-xs text-rose-600">
            {{ t('merchant.detail.warnings.markupHigh') }}
          </div>
        </div>
        <div>
          <label class="input-label">
            {{ t('merchant.fields.reason') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('common.optional') }})</span>
          </label>
          <textarea v-model="defaultDialog.reason" rows="2" class="input"></textarea>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="defaultDialog.show = false">
            {{ t('common.cancel') }}
          </button>
          <button
            type="submit"
            form="merchant-default-markup-form"
            :disabled="defaultDialog.submitting"
            class="btn btn-primary"
          >
            {{ defaultDialog.submitting ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- 分组 markup 编辑弹框（行内编辑） -->
    <BaseDialog
      :show="markupDialog.show"
      :title="t('merchant.owner.pricing.editGroupTitle', { name: markupDialog.groupName })"
      width="normal"
      @close="markupDialog.show = false"
    >
      <form id="merchant-group-markup-form" class="space-y-4" @submit.prevent="submitMarkup">
        <div>
          <label class="input-label">{{ t('merchant.fields.markup') }}</label>
          <input
            v-model.number="markupDialog.markup"
            type="number"
            min="1"
            step="0.0001"
            required
            class="input"
          />
          <p class="mt-1 text-xs text-gray-500">{{ t('merchant.owner.pricing.markupHint') }}</p>
          <div v-if="markupDialog.markup > 2" class="mt-1 text-xs text-rose-600">
            {{ t('merchant.detail.warnings.markupHigh') }}
          </div>
        </div>
        <div>
          <label class="input-label">
            {{ t('merchant.fields.reason') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('common.optional') }})</span>
          </label>
          <textarea v-model="markupDialog.reason" rows="2" class="input"></textarea>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="markupDialog.show = false">
            {{ t('common.cancel') }}
          </button>
          <button
            type="submit"
            form="merchant-group-markup-form"
            :disabled="markupDialog.submitting"
            class="btn btn-primary"
          >
            {{ markupDialog.submitting ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { merchantAPI, type MerchantInfo } from '@/api'
import type { MerchantPricingGroup } from '@/api/merchant'
import { extractI18nErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const groups = ref<MerchantPricingGroup[]>([])
const info = ref<MerchantInfo | null>(null)
const loading = ref(false)

const defaultMarkup = computed(() => Number(info.value?.user_markup_default ?? 1))

function effectiveMarkup(row: MerchantPricingGroup): number {
  return row.markup != null ? Number(row.markup) : defaultMarkup.value
}

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('merchant.detail.groupPricing.group') },
  { key: 'rate_multiplier', label: t('merchant.owner.pricing.siteRate') },
  { key: 'markup', label: t('merchant.owner.pricing.myMarkup') },
  { key: 'actions', label: t('common.actions') },
])

async function load(): Promise<void> {
  loading.value = true
  try {
    const [m, list] = await Promise.all([
      merchantAPI.info(),
      merchantAPI.listPricingGroups(),
    ])
    info.value = m
    groups.value = list
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

// ============ 默认 markup ============

const defaultDialog = reactive({
  show: false,
  markup: 1,
  reason: '',
  submitting: false,
})

function openDefaultForm(): void {
  defaultDialog.markup = defaultMarkup.value
  defaultDialog.reason = ''
  defaultDialog.submitting = false
  defaultDialog.show = true
}

async function submitDefault(): Promise<void> {
  defaultDialog.submitting = true
  try {
    await merchantAPI.setMarkupDefault(defaultDialog.markup, defaultDialog.reason || undefined)
    appStore.showSuccess(t('common.saved'))
    defaultDialog.show = false
    await load()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    defaultDialog.submitting = false
  }
}

// ============ 分组 markup（行内编辑） ============

const markupDialog = reactive({
  show: false,
  group_id: null as number | null,
  groupName: '',
  markup: 1,
  reason: '',
  submitting: false,
})

function openMarkupForm(row: MerchantPricingGroup): void {
  markupDialog.group_id = row.id
  markupDialog.groupName = row.name
  // 已有 override 用它，没有的话以默认 markup 作为起点
  markupDialog.markup = row.markup != null ? Number(row.markup) : defaultMarkup.value
  markupDialog.reason = ''
  markupDialog.submitting = false
  markupDialog.show = true
}

async function submitMarkup(): Promise<void> {
  if (markupDialog.group_id == null) return
  markupDialog.submitting = true
  try {
    await merchantAPI.setGroupMarkup(
      markupDialog.group_id,
      markupDialog.markup,
      markupDialog.reason || undefined,
    )
    appStore.showSuccess(t('common.saved'))
    markupDialog.show = false
    await load()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    markupDialog.submitting = false
  }
}

async function confirmReset(row: MerchantPricingGroup): Promise<void> {
  if (!window.confirm(t('merchant.owner.pricing.confirmReset', { name: row.name }))) return
  try {
    await merchantAPI.deleteGroupMarkup(row.id)
    appStore.showSuccess(t('common.deleted'))
    await load()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  }
}

onMounted(() => {
  void load()
})
</script>
