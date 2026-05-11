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

      <!-- 分组覆盖列表 -->
      <div class="card">
        <div class="flex items-center justify-between border-b border-gray-200 px-4 py-3 dark:border-dark-700">
          <div>
            <h2 class="text-sm font-semibold text-gray-700 dark:text-gray-200">
              {{ t('merchant.owner.pricing.overridesTitle') }}
            </h2>
            <p class="mt-0.5 text-xs text-gray-500">
              {{ t('merchant.owner.pricing.overridesHint') }}
            </p>
          </div>
          <button class="btn btn-primary" @click="openMarkupForm()">
            <Icon name="plus" size="sm" />
            <span class="ml-1">{{ t('merchant.owner.pricing.addOverride') }}</span>
          </button>
        </div>
        <DataTable :columns="columns" :data="markups" :loading="loading">
          <template #cell-group_id="{ row }">
            <div class="flex items-center gap-2">
              <span class="font-mono text-sm">#{{ row.group_id }}</span>
              <span v-if="groupName(row.group_id)" class="text-sm text-gray-700 dark:text-gray-200">
                {{ groupName(row.group_id) }}
              </span>
            </div>
          </template>
          <template #cell-markup="{ value }">
            <span class="font-mono text-sm">{{ Number(value || 1).toFixed(4) }}</span>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex gap-2">
              <button class="text-sm text-primary-600 hover:underline dark:text-primary-400" @click="openMarkupForm(row)">
                {{ t('common.edit') }}
              </button>
              <button class="text-sm text-rose-600 hover:underline dark:text-rose-400" @click="confirmDelete(row)">
                {{ t('common.delete') }}
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

    <!-- 分组 markup 增/改弹框 -->
    <BaseDialog
      :show="markupDialog.show"
      :title="markupDialog.editing ? t('merchant.owner.pricing.editOverrideTitle') : t('merchant.owner.pricing.addOverrideTitle')"
      width="normal"
      @close="markupDialog.show = false"
    >
      <form id="merchant-group-markup-form" class="space-y-4" @submit.prevent="submitMarkup">
        <div>
          <label class="input-label">{{ t('merchant.detail.groupPricing.group') }}</label>
          <Select
            v-model="markupDialog.group_id"
            :options="groupOptions"
            :placeholder="t('merchant.detail.groupPricing.selectGroup')"
            :disabled="markupDialog.editing"
          />
          <p class="mt-1 text-xs text-gray-500">
            {{ t('merchant.owner.pricing.groupSourceHint') }}
          </p>
        </div>
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
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { merchantAPI, type MerchantGroupMarkup, type MerchantInfo } from '@/api'
import userGroupsAPI from '@/api/groups'
import type { Group } from '@/types'
import { extractI18nErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const markups = ref<MerchantGroupMarkup[]>([])
const info = ref<MerchantInfo | null>(null)
const groups = ref<Group[]>([])
const loading = ref(false)

const defaultMarkup = computed(() => Number(info.value?.user_markup_default ?? 1))

// 已设置覆盖的分组从 add 下拉里剔除（编辑模式不剔除自己）。
const groupOptions = computed(() => {
  const usedExcept = (excludeId: number | null): number[] =>
    markups.value.map((m) => m.group_id).filter((id) => id !== excludeId)
  const excluded = markupDialog.editing ? usedExcept(markupDialog.group_id) : usedExcept(null)
  return groups.value
    .filter((g) => !excluded.includes(g.id))
    .map((g) => ({ value: g.id, label: `${g.name} (#${g.id})` }))
})

function groupName(id: number): string {
  return groups.value.find((g) => g.id === id)?.name ?? ''
}

const columns = computed<Column[]>(() => [
  { key: 'group_id', label: t('merchant.detail.groupPricing.group') },
  { key: 'markup', label: t('merchant.fields.markup') },
  { key: 'actions', label: t('common.actions') },
])

async function load(): Promise<void> {
  loading.value = true
  try {
    const [m, list, gs] = await Promise.all([
      merchantAPI.info(),
      merchantAPI.listGroupMarkups(),
      userGroupsAPI.getAvailable().catch(() => [] as Group[]),
    ])
    info.value = m
    markups.value = list
    groups.value = gs
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

// ============ 分组 markup ============

const markupDialog = reactive({
  show: false,
  editing: false,
  group_id: null as number | null,
  markup: 1,
  reason: '',
  submitting: false,
})

function openMarkupForm(row?: MerchantGroupMarkup): void {
  if (row) {
    markupDialog.editing = true
    markupDialog.group_id = row.group_id
    markupDialog.markup = Number(row.markup ?? 1)
  } else {
    markupDialog.editing = false
    markupDialog.group_id = null
    markupDialog.markup = 1
  }
  markupDialog.reason = ''
  markupDialog.submitting = false
  markupDialog.show = true
}

async function submitMarkup(): Promise<void> {
  if (markupDialog.group_id == null) {
    appStore.showError(t('merchant.errors.groupRequired'))
    return
  }
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

async function confirmDelete(row: MerchantGroupMarkup): Promise<void> {
  if (!window.confirm(t('merchant.detail.groupPricing.confirmDelete'))) return
  try {
    await merchantAPI.deleteGroupMarkup(row.group_id)
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
