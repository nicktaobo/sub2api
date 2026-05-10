<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Header -->
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex items-center gap-3">
          <button class="btn btn-secondary btn-sm" @click="goBack">
            <Icon name="chevronLeft" size="sm" class="mr-1" />
            {{ t('common.back') }}
          </button>
          <div>
            <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
              {{ merchant?.name || t('merchant.detail.loading') }}
            </h1>
            <div class="text-sm text-gray-500 dark:text-dark-400">
              <span v-if="merchant">#{{ merchant.id }}</span>
              <span v-if="merchant" class="ml-3">
                {{ t('merchant.fields.ownerUserId') }}: {{ merchant.owner_user_id }}
              </span>
            </div>
          </div>
        </div>
        <div v-if="merchant" class="flex items-center gap-3">
          <span
            :class="[
              'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium',
              merchant.status === 'active'
                ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
                : 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400',
            ]"
          >
            {{ t('merchant.status.' + merchant.status) }}
          </span>
        </div>
      </div>

      <!-- Tabs -->
      <div class="card">
        <div class="border-b border-gray-200 px-4 dark:border-dark-700">
          <nav class="-mb-px flex flex-wrap gap-x-6">
            <button
              v-for="tab in tabs"
              :key="tab.value"
              :class="[
                'whitespace-nowrap border-b-2 px-1 py-3 text-sm font-medium transition-colors',
                activeTab === tab.value
                  ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                  : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700 dark:text-dark-400 dark:hover:text-dark-200',
              ]"
              @click="activeTab = tab.value"
            >
              {{ tab.label }}
            </button>
          </nav>
        </div>

        <!-- Info tab -->
        <div v-if="activeTab === 'info'" class="space-y-6 p-6">
          <div v-if="!merchant" class="py-8 text-center text-gray-500">{{ t('common.loading') }}</div>
          <template v-else>
            <!-- Discount -->
            <div class="grid gap-4 md:grid-cols-2">
              <div>
                <label class="input-label">{{ t('merchant.fields.discount') }}</label>
                <div class="flex gap-2">
                  <input
                    v-model.number="form.discount"
                    type="number"
                    min="0"
                    step="0.0001"
                    class="input"
                  />
                  <button class="btn btn-primary" :disabled="saving.discount" @click="saveDiscount">
                    {{ saving.discount ? t('common.saving') : t('common.save') }}
                  </button>
                </div>
                <div
                  v-if="form.discount < 0.5"
                  class="mt-2 rounded-md border border-rose-300 bg-rose-50 px-3 py-2 text-xs text-rose-700 dark:border-rose-700/40 dark:bg-rose-900/20 dark:text-rose-300"
                >
                  {{ t('merchant.detail.warnings.discountLow') }}
                </div>
              </div>
              <div>
                <label class="input-label">{{ t('merchant.fields.markupDefault') }}</label>
                <div class="flex gap-2">
                  <input
                    v-model.number="form.user_markup_default"
                    type="number"
                    min="1"
                    step="0.0001"
                    class="input"
                  />
                  <button class="btn btn-primary" :disabled="saving.markup" @click="saveMarkup">
                    {{ saving.markup ? t('common.saving') : t('common.save') }}
                  </button>
                </div>
                <div
                  v-if="form.user_markup_default > 2"
                  class="mt-2 rounded-md border border-rose-300 bg-rose-50 px-3 py-2 text-xs text-rose-700 dark:border-rose-700/40 dark:bg-rose-900/20 dark:text-rose-300"
                >
                  {{ t('merchant.detail.warnings.markupHigh') }}
                </div>
              </div>
            </div>

            <div class="grid gap-4 md:grid-cols-2">
              <div>
                <label class="input-label">{{ t('merchant.fields.lowBalanceThreshold') }}</label>
                <input
                  v-model.number="form.low_balance_threshold"
                  type="number"
                  min="0"
                  step="0.01"
                  class="input"
                  disabled
                />
                <p class="mt-1 text-xs text-gray-400">{{ t('merchant.detail.thresholdReadonlyHint') }}</p>
              </div>
              <div>
                <label class="input-label">{{ t('merchant.fields.notifyEmails') }}</label>
                <textarea
                  v-model="form.notify_emails_str"
                  class="input"
                  rows="2"
                  disabled
                ></textarea>
                <p class="mt-1 text-xs text-gray-400">{{ t('merchant.detail.notifyEmailsReadonlyHint') }}</p>
              </div>
            </div>

            <div class="rounded-lg border border-gray-100 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-800/50">
              <h3 class="mb-3 text-sm font-semibold text-gray-700 dark:text-gray-200">
                {{ t('merchant.detail.metrics') }}
              </h3>
              <div class="grid gap-3 sm:grid-cols-2 md:grid-cols-3">
                <div>
                  <div class="text-xs text-gray-500">{{ t('merchant.fields.balanceBaseline') }}</div>
                  <div class="font-mono text-lg">${{ Number(merchant.balance_baseline ?? 0).toFixed(2) }}</div>
                </div>
                <div>
                  <div class="text-xs text-gray-500">{{ t('merchant.fields.ownerBalance') }}</div>
                  <div class="font-mono text-lg">${{ Number(merchant.owner_balance ?? 0).toFixed(2) }}</div>
                </div>
                <div>
                  <div class="text-xs text-gray-500">{{ t('merchant.fields.createdAt') }}</div>
                  <div class="text-sm">{{ formatDateTime(merchant.created_at) }}</div>
                </div>
              </div>
            </div>
          </template>
        </div>

        <!-- Group pricing tab -->
        <div v-if="activeTab === 'pricing'" class="space-y-4 p-6">
          <div class="flex items-center justify-between">
            <div>
              <h3 class="text-sm font-semibold text-gray-700 dark:text-gray-200">
                {{ t('merchant.detail.groupPricing.title') }}
              </h3>
              <p class="text-xs text-gray-500">{{ t('merchant.detail.groupPricing.description') }}</p>
            </div>
            <button class="btn btn-primary btn-sm" @click="openMarkupForm()">
              <Icon name="plus" size="sm" class="mr-1" />
              {{ t('merchant.detail.groupPricing.addOverride') }}
            </button>
          </div>
          <DataTable :columns="markupColumns" :data="markups" :loading="loadingMarkups">
            <template #cell-group_id="{ value }">
              <span class="font-mono text-sm">#{{ value }}</span>
              <span class="ml-2 text-sm">{{ groupName(value) }}</span>
            </template>
            <template #cell-markup="{ value }">
              <span class="font-mono text-sm">{{ Number(value || 1).toFixed(4) }}</span>
            </template>
            <template #cell-actions="{ row }">
              <div class="flex items-center gap-2">
                <button class="btn btn-secondary btn-xs" @click="openMarkupForm(row)">
                  {{ t('common.edit') }}
                </button>
                <button class="btn btn-secondary btn-xs text-rose-600" @click="deleteMarkup(row)">
                  {{ t('common.delete') }}
                </button>
              </div>
            </template>
          </DataTable>
        </div>

        <!-- Ledger tab -->
        <div v-if="activeTab === 'ledger'" class="p-6">
          <DataTable :columns="ledgerColumns" :data="ledger" :loading="loadingLedger">
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
          <div class="mt-4 flex items-center justify-end">
            <Pagination
              v-if="ledgerTotal > ledgerPageSize"
              :total="ledgerTotal"
              :page="ledgerPage"
              :page-size="ledgerPageSize"
              @update:page="onLedgerPageChange"
              @update:pageSize="onLedgerPageSizeChange"
            />
          </div>
        </div>

        <!-- Audit log tab -->
        <div v-if="activeTab === 'audit'" class="p-6">
          <DataTable :columns="auditColumns" :data="audit" :loading="loadingAudit">
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
          <div class="mt-4 flex items-center justify-end">
            <Pagination
              v-if="auditTotal > auditPageSize"
              :total="auditTotal"
              :page="auditPage"
              :page-size="auditPageSize"
              @update:page="onAuditPageChange"
              @update:pageSize="onAuditPageSizeChange"
            />
          </div>
        </div>
      </div>
    </div>

    <!-- Group markup form dialog -->
    <BaseDialog
      :show="markupDialog.show"
      :title="markupDialog.editing ? t('merchant.detail.groupPricing.editTitle') : t('merchant.detail.groupPricing.addTitle')"
      width="normal"
      @close="markupDialog.show = false"
    >
      <form id="markup-form" class="space-y-4" @submit.prevent="submitMarkup">
        <div>
          <label class="input-label">{{ t('merchant.detail.groupPricing.group') }}</label>
          <Select
            v-model="markupDialog.group_id"
            :options="groupOptions"
            :placeholder="t('merchant.detail.groupPricing.selectGroup')"
            :disabled="markupDialog.editing"
          />
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
          <div
            v-if="markupDialog.markup > 2"
            class="mt-1 text-xs text-rose-600"
          >
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
            form="markup-form"
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
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import {
  merchantAPI,
  type Merchant,
  type MerchantGroupMarkup,
  type MerchantLedgerEntry,
  type MerchantAuditLogEntry,
} from '@/api'
import { groupsAPI } from '@/api/admin'
import type { AdminGroup } from '@/types'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const appStore = useAppStore()

const merchantId = computed(() => Number(route.params.id))
const merchant = ref<Merchant | null>(null)
const activeTab = ref<'info' | 'pricing' | 'ledger' | 'audit'>('info')

const tabs = computed(() => [
  { value: 'info' as const, label: t('merchant.detail.tabs.info') },
  { value: 'pricing' as const, label: t('merchant.detail.tabs.pricing') },
  { value: 'ledger' as const, label: t('merchant.detail.tabs.ledger') },
  { value: 'audit' as const, label: t('merchant.detail.tabs.audit') },
])

// Form state
const form = reactive({
  discount: 1,
  user_markup_default: 1,
  low_balance_threshold: 0,
  notify_emails_str: '',
})

const saving = reactive({ discount: false, markup: false })

function syncFormFromMerchant(): void {
  if (!merchant.value) return
  form.discount = Number(merchant.value.discount ?? 1)
  form.user_markup_default = Number(merchant.value.user_markup_default ?? 1)
  form.low_balance_threshold = Number(merchant.value.low_balance_threshold ?? 0)
  const emails = merchant.value.notify_emails
  if (Array.isArray(emails)) {
    form.notify_emails_str = emails
      .map((e) => (typeof e === 'string' ? e : e?.email ?? ''))
      .filter(Boolean)
      .join(', ')
  } else {
    form.notify_emails_str = ''
  }
}

async function loadMerchant(): Promise<void> {
  try {
    merchant.value = await merchantAPI.adminGet(merchantId.value)
    syncFormFromMerchant()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  }
}

async function saveDiscount(): Promise<void> {
  saving.discount = true
  try {
    merchant.value = await merchantAPI.adminSetDiscount(merchantId.value, form.discount)
    syncFormFromMerchant()
    appStore.showSuccess(t('common.saved'))
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    saving.discount = false
  }
}

async function saveMarkup(): Promise<void> {
  saving.markup = true
  try {
    merchant.value = await merchantAPI.adminSetMarkupDefault(merchantId.value, form.user_markup_default)
    syncFormFromMerchant()
    appStore.showSuccess(t('common.saved'))
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    saving.markup = false
  }
}

// Group pricing
const markups = ref<MerchantGroupMarkup[]>([])
const loadingMarkups = ref(false)
const groups = ref<AdminGroup[]>([])

const markupColumns = computed<Column[]>(() => [
  { key: 'group_id', label: t('merchant.detail.groupPricing.group') },
  { key: 'markup', label: t('merchant.fields.markup') },
  { key: 'actions', label: t('common.actions') },
])

const groupOptions = computed(() =>
  groups.value.map((g) => ({ value: g.id, label: `${g.name} (#${g.id})` })),
)

function groupName(id: number): string {
  return groups.value.find((g) => g.id === id)?.name ?? ''
}

async function loadMarkups(): Promise<void> {
  loadingMarkups.value = true
  try {
    markups.value = await merchantAPI.adminListGroupMarkups(merchantId.value)
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    loadingMarkups.value = false
  }
}

async function loadGroups(): Promise<void> {
  try {
    groups.value = await groupsAPI.getAll()
  } catch {
    // non-fatal
  }
}

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
    await merchantAPI.adminSetGroupMarkup(
      merchantId.value,
      markupDialog.group_id,
      markupDialog.markup,
      markupDialog.reason || undefined,
    )
    appStore.showSuccess(t('common.saved'))
    markupDialog.show = false
    await loadMarkups()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    markupDialog.submitting = false
  }
}

async function deleteMarkup(row: MerchantGroupMarkup): Promise<void> {
  if (!window.confirm(t('merchant.detail.groupPricing.confirmDelete'))) return
  try {
    await merchantAPI.adminDeleteGroupMarkup(merchantId.value, row.group_id)
    appStore.showSuccess(t('common.deleted'))
    await loadMarkups()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  }
}

// Ledger
const ledger = ref<MerchantLedgerEntry[]>([])
const loadingLedger = ref(false)
const ledgerPage = ref(1)
const ledgerPageSize = ref(20)
const ledgerTotal = ref(0)
const ledgerColumns = computed<Column[]>(() => [
  { key: 'id', label: t('merchant.fields.id') },
  { key: 'created_at', label: t('merchant.fields.time') },
  { key: 'direction', label: t('merchant.ledger.directionLabel') },
  { key: 'amount', label: t('merchant.fields.amount') },
  { key: 'balance_after', label: t('merchant.fields.balanceAfter') },
  { key: 'source', label: t('merchant.fields.source') },
  { key: 'ref_type', label: t('merchant.fields.refType') },
  { key: 'ref_id', label: t('merchant.fields.refId') },
  { key: 'idempotency_key', label: t('merchant.fields.idempotencyKey') },
])

async function loadLedger(): Promise<void> {
  loadingLedger.value = true
  try {
    const offset = (ledgerPage.value - 1) * ledgerPageSize.value
    const res = await merchantAPI.adminListLedger(merchantId.value, offset, ledgerPageSize.value)
    ledger.value = res.items || []
    ledgerTotal.value = res.total || 0
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    loadingLedger.value = false
  }
}

function onLedgerPageChange(p: number): void {
  ledgerPage.value = p
  void loadLedger()
}

function onLedgerPageSizeChange(s: number): void {
  ledgerPageSize.value = s
  ledgerPage.value = 1
  void loadLedger()
}

// Audit
const audit = ref<MerchantAuditLogEntry[]>([])
const loadingAudit = ref(false)
const auditPage = ref(1)
const auditPageSize = ref(20)
const auditTotal = ref(0)

const auditColumns = computed<Column[]>(() => [
  { key: 'id', label: t('merchant.fields.id') },
  { key: 'created_at', label: t('merchant.fields.time') },
  { key: 'actor', label: t('merchant.fields.actor') },
  { key: 'action', label: t('merchant.fields.action') },
  { key: 'reason', label: t('merchant.fields.reason') },
])

async function loadAudit(): Promise<void> {
  loadingAudit.value = true
  try {
    const offset = (auditPage.value - 1) * auditPageSize.value
    const res = await merchantAPI.adminListAuditLog(merchantId.value, offset, auditPageSize.value)
    audit.value = res.items || []
    auditTotal.value = res.total || 0
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    loadingAudit.value = false
  }
}

function onAuditPageChange(p: number): void {
  auditPage.value = p
  void loadAudit()
}

function onAuditPageSizeChange(s: number): void {
  auditPageSize.value = s
  auditPage.value = 1
  void loadAudit()
}

watch(activeTab, (tab) => {
  if (tab === 'pricing' && groups.value.length === 0) {
    void loadGroups()
  }
  if (tab === 'pricing' && markups.value.length === 0) {
    void loadMarkups()
  }
  if (tab === 'ledger' && ledger.value.length === 0) {
    void loadLedger()
  }
  if (tab === 'audit' && audit.value.length === 0) {
    void loadAudit()
  }
})

function goBack(): void {
  router.push('/admin/merchants')
}

onMounted(() => {
  void loadMerchant()
})
</script>
