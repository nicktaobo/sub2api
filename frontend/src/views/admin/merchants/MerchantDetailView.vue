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
          <div>
            <h3 class="text-sm font-semibold text-gray-700 dark:text-gray-200">
              {{ t('merchant.detail.groupPricing.title') }}
            </h3>
            <p class="text-xs text-gray-500">{{ t('merchant.detail.groupPricing.description') }}</p>
          </div>
          <DataTable :columns="markupColumns" :data="pricingRows" :loading="loadingMarkups">
            <template #cell-group_name="{ row }">
              <span class="font-medium text-gray-900 dark:text-gray-100">{{ row.group_name }}</span>
              <span class="ml-1 text-xs text-gray-400">#{{ row.group_id }}</span>
            </template>
            <template #cell-rate_multiplier="{ value }">
              <span class="font-mono text-sm">{{ Number(value || 1).toFixed(4) }}x</span>
            </template>
            <template #cell-cost_rate="{ row }">
              <span class="font-mono text-sm">{{ effectiveCost(row).toFixed(4) }}x</span>
              <span v-if="row.cost_rate == null" class="ml-1 text-[10px] text-gray-400">
                {{ t('merchant.detail.groupPricing.followsSite') }}
              </span>
            </template>
            <template #cell-sell_rate="{ row }">
              <span v-if="row.sell_rate != null" class="font-mono text-sm font-semibold text-primary-600 dark:text-primary-400">
                {{ Number(row.sell_rate).toFixed(4) }}x
              </span>
              <span v-else class="text-xs italic text-gray-400">
                {{ t('merchant.detail.groupPricing.notConfigured') }}
              </span>
            </template>
            <template #cell-actions="{ row }">
              <div class="flex items-center gap-2">
                <button class="btn btn-secondary btn-xs" @click="openCostForm(row)">
                  {{ t('merchant.detail.groupPricing.editCost') }}
                </button>
                <button v-if="row.cost_rate != null" class="btn btn-secondary btn-xs text-rose-600" @click="deleteCost(row)">
                  {{ t('merchant.detail.groupPricing.clearCost') }}
                </button>
                <button class="btn btn-secondary btn-xs" @click="openSellForm(row)">
                  {{ t('merchant.detail.groupPricing.editSell') }}
                </button>
                <button v-if="row.sell_rate != null" class="btn btn-secondary btn-xs text-rose-600" @click="deleteSell(row)">
                  {{ t('merchant.detail.groupPricing.clearSell') }}
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

    <!-- Cost rate (admin only) dialog -->
    <BaseDialog
      :show="costDialog.show"
      :title="t('merchant.detail.groupPricing.editCostTitle', { name: costDialog.groupName })"
      width="normal"
      @close="costDialog.show = false"
    >
      <form id="cost-form" class="space-y-4" @submit.prevent="submitCost">
        <div class="rounded-lg bg-gray-50 p-3 text-sm dark:bg-dark-800/40">
          <div class="flex justify-between text-gray-500">
            <span>{{ t('merchant.detail.groupPricing.siteRate') }}</span>
            <span class="font-mono">{{ costDialog.rateMultiplier.toFixed(4) }}x</span>
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('merchant.detail.groupPricing.costRate') }}</label>
          <input v-model.number="costDialog.costRate" type="number" min="0.0001" step="0.0001" required class="input" />
          <p class="mt-1 text-xs text-gray-500">{{ t('merchant.detail.groupPricing.costRateHint') }}</p>
        </div>
        <div>
          <label class="input-label">
            {{ t('merchant.fields.reason') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('common.optional') }})</span>
          </label>
          <textarea v-model="costDialog.reason" rows="2" class="input"></textarea>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="costDialog.show = false">{{ t('common.cancel') }}</button>
          <button type="submit" form="cost-form" :disabled="costDialog.submitting" class="btn btn-primary">
            {{ costDialog.submitting ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Sell rate (admin overrides on behalf of merchant) dialog -->
    <BaseDialog
      :show="sellDialog.show"
      :title="t('merchant.detail.groupPricing.editSellTitle', { name: sellDialog.groupName })"
      width="normal"
      @close="sellDialog.show = false"
    >
      <form id="sell-form" class="space-y-4" @submit.prevent="submitSell">
        <div class="rounded-lg bg-gray-50 p-3 text-sm dark:bg-dark-800/40">
          <div class="flex justify-between text-gray-500">
            <span>{{ t('merchant.detail.groupPricing.costRate') }}</span>
            <span class="font-mono">{{ sellDialog.costRate.toFixed(4) }}x</span>
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('merchant.detail.groupPricing.sellRate') }}</label>
          <input
            v-model.number="sellDialog.sellRate"
            type="number"
            :min="sellDialog.costRate"
            step="0.0001"
            required
            class="input"
          />
          <p class="mt-1 text-xs text-gray-500">{{ t('merchant.detail.groupPricing.sellRateHint') }}</p>
          <div v-if="sellDialog.sellRate < sellDialog.costRate" class="mt-1 text-xs text-rose-600">
            {{ t('merchant.detail.groupPricing.sellBelowCost', { cost: sellDialog.costRate.toFixed(4) }) }}
          </div>
        </div>
        <div>
          <label class="input-label">
            {{ t('merchant.fields.reason') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('common.optional') }})</span>
          </label>
          <textarea v-model="sellDialog.reason" rows="2" class="input"></textarea>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="sellDialog.show = false">{{ t('common.cancel') }}</button>
          <button
            type="submit"
            form="sell-form"
            :disabled="sellDialog.submitting || sellDialog.sellRate < sellDialog.costRate"
            class="btn btn-primary"
          >
            {{ sellDialog.submitting ? t('common.saving') : t('common.save') }}
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
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import {
  merchantAPI,
  type Merchant,
  type MerchantGroupMarkup,
  type MerchantGroupCost,
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
  low_balance_threshold: 0,
  notify_emails_str: '',
})

function syncFormFromMerchant(): void {
  if (!merchant.value) return
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

// Group pricing（v2.0 cost/sell 双值模型）
// admin 配 cost_rate（商户拿货价），商户配 sell_rate（对外售价）；admin 可代商户改 sell。
interface PricingRow {
  group_id: number
  group_name: string
  rate_multiplier: number
  cost_rate: number | null
  sell_rate: number | null
}

const sells = ref<MerchantGroupMarkup[]>([])
const costs = ref<MerchantGroupCost[]>([])
const loadingMarkups = ref(false)
const groups = ref<AdminGroup[]>([])

const markupColumns = computed<Column[]>(() => [
  { key: 'group_name', label: t('merchant.detail.groupPricing.group') },
  { key: 'rate_multiplier', label: t('merchant.detail.groupPricing.siteRate') },
  { key: 'cost_rate', label: t('merchant.detail.groupPricing.costRate') },
  { key: 'sell_rate', label: t('merchant.detail.groupPricing.sellRate') },
  { key: 'actions', label: t('common.actions') },
])

const pricingRows = computed<PricingRow[]>(() => {
  const costMap = new Map<number, number>()
  for (const c of costs.value) costMap.set(c.group_id, c.cost_rate)
  const sellMap = new Map<number, number>()
  for (const s of sells.value) sellMap.set(s.group_id, s.sell_rate)
  return groups.value.map((g) => ({
    group_id: g.id,
    group_name: g.name,
    rate_multiplier: g.rate_multiplier,
    cost_rate: costMap.get(g.id) ?? null,
    sell_rate: sellMap.get(g.id) ?? null,
  }))
})

function effectiveCost(row: PricingRow): number {
  return row.cost_rate != null ? row.cost_rate : row.rate_multiplier
}

async function loadMarkups(): Promise<void> {
  loadingMarkups.value = true
  try {
    const [s, c] = await Promise.all([
      merchantAPI.adminListGroupMarkups(merchantId.value),
      merchantAPI.adminListGroupCosts(merchantId.value),
    ])
    sells.value = s
    costs.value = c
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

const costDialog = reactive({
  show: false,
  group_id: null as number | null,
  groupName: '',
  rateMultiplier: 1,
  costRate: 1,
  reason: '',
  submitting: false,
})

function openCostForm(row: PricingRow): void {
  costDialog.group_id = row.group_id
  costDialog.groupName = row.group_name
  costDialog.rateMultiplier = row.rate_multiplier
  costDialog.costRate = effectiveCost(row)
  costDialog.reason = ''
  costDialog.submitting = false
  costDialog.show = true
}

async function submitCost(): Promise<void> {
  if (costDialog.group_id == null) return
  costDialog.submitting = true
  try {
    await merchantAPI.adminSetGroupCost(
      merchantId.value,
      costDialog.group_id,
      costDialog.costRate,
      costDialog.reason || undefined,
    )
    appStore.showSuccess(t('common.saved'))
    costDialog.show = false
    await loadMarkups()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    costDialog.submitting = false
  }
}

async function deleteCost(row: PricingRow): Promise<void> {
  if (!window.confirm(t('merchant.detail.groupPricing.confirmDeleteCost', { name: row.group_name }))) return
  try {
    await merchantAPI.adminDeleteGroupCost(merchantId.value, row.group_id)
    appStore.showSuccess(t('common.deleted'))
    await loadMarkups()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  }
}

const sellDialog = reactive({
  show: false,
  group_id: null as number | null,
  groupName: '',
  costRate: 1,
  sellRate: 1,
  reason: '',
  submitting: false,
})

function openSellForm(row: PricingRow): void {
  sellDialog.group_id = row.group_id
  sellDialog.groupName = row.group_name
  sellDialog.costRate = effectiveCost(row)
  sellDialog.sellRate = row.sell_rate != null ? row.sell_rate : sellDialog.costRate
  sellDialog.reason = ''
  sellDialog.submitting = false
  sellDialog.show = true
}

async function submitSell(): Promise<void> {
  if (sellDialog.group_id == null) return
  sellDialog.submitting = true
  try {
    await merchantAPI.adminSetGroupMarkup(
      merchantId.value,
      sellDialog.group_id,
      sellDialog.sellRate,
      sellDialog.reason || undefined,
    )
    appStore.showSuccess(t('common.saved'))
    sellDialog.show = false
    await loadMarkups()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    sellDialog.submitting = false
  }
}

async function deleteSell(row: PricingRow): Promise<void> {
  if (!window.confirm(t('merchant.detail.groupPricing.confirmDeleteSell', { name: row.group_name }))) return
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
  if (tab === 'pricing' && sells.value.length === 0 && costs.value.length === 0) {
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
