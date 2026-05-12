<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-col justify-between gap-4 lg:flex-row lg:items-start">
          <div class="flex flex-1 flex-wrap items-center gap-3">
            <div class="relative w-full sm:w-64">
              <Icon
                name="search"
                size="md"
                class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500"
              />
              <input
                v-model="searchQuery"
                type="text"
                :placeholder="t('merchant.admin.searchPlaceholder')"
                class="input pl-10"
              />
            </div>
            <Select
              v-model="statusFilter"
              :options="statusOptions"
              :placeholder="t('merchant.admin.allStatus')"
              class="w-44"
              @change="reload"
            />
          </div>
          <div class="flex flex-shrink-0 items-center gap-3">
            <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="reload">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button class="btn btn-primary" @click="goCreate">
              <Icon name="plus" size="md" class="mr-2" />
              {{ t('merchant.admin.create') }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="filteredItems" :loading="loading">
          <template #cell-status="{ value }">
            <span
              :class="[
                'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium',
                value === 'active'
                  ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
                  : 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400',
              ]"
            >
              {{ t('merchant.status.' + value) }}
            </span>
          </template>
          <template #cell-domains="{ row }">
            <span v-if="!row.domains?.length" class="text-gray-400">-</span>
            <div v-else class="flex flex-col gap-0.5">
              <a
                v-for="d in row.domains"
                :key="d"
                :href="'https://' + d"
                target="_blank"
                class="text-xs text-orange-500 hover:underline"
              >{{ d }}</a>
            </div>
          </template>
          <template #cell-sub_user_count="{ value }">
            <span class="text-sm">{{ value ?? 0 }}</span>
          </template>
          <template #cell-owner_balance="{ value }">
            <span class="font-mono text-sm">${{ Number(value || 0).toFixed(2) }}</span>
          </template>
          <template #cell-balance_baseline="{ value }">
            <span class="font-mono text-sm">${{ Number(value || 0).toFixed(2) }}</span>
          </template>
          <template #cell-created_at="{ value }">
            <span class="text-sm text-gray-700 dark:text-gray-300">{{ formatDateTime(value) }}</span>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex flex-wrap items-center gap-2">
              <button class="btn btn-secondary btn-xs" @click="goDetail(row.id)">
                {{ t('merchant.admin.actions.view') }}
              </button>
              <button class="btn btn-secondary btn-xs" @click="openRecharge(row)">
                {{ t('merchant.admin.actions.recharge') }}
              </button>
              <button class="btn btn-secondary btn-xs" @click="openRefund(row)">
                {{ t('merchant.admin.actions.refund') }}
              </button>
              <button
                v-if="row.status === 'active'"
                class="btn btn-secondary btn-xs"
                @click="openStatusChange(row, 'suspended')"
              >
                {{ t('merchant.admin.actions.suspend') }}
              </button>
              <button
                v-else
                class="btn btn-secondary btn-xs"
                @click="openStatusChange(row, 'active')"
              >
                {{ t('merchant.admin.actions.activate') }}
              </button>
            </div>
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

    <!-- Recharge / Refund / Status dialog -->
    <BaseDialog
      :show="actionDialog.show"
      :title="actionDialogTitle"
      width="normal"
      @close="closeActionDialog"
    >
      <form id="merchant-action-form" class="space-y-4" @submit.prevent="submitAction">
        <div v-if="actionDialog.type !== 'status'">
          <label class="input-label">{{ t('merchant.fields.amount') }}</label>
          <input
            v-model.number="actionDialog.amount"
            type="number"
            step="0.01"
            min="0.01"
            required
            class="input"
          />
        </div>
        <div>
          <label class="input-label">
            {{ t('merchant.fields.reason') }}
            <span
              v-if="actionDialog.type !== 'refund'"
              class="ml-1 text-xs font-normal text-gray-400"
            >({{ t('common.optional') }})</span>
          </label>
          <textarea
            v-model="actionDialog.reason"
            rows="3"
            class="input"
            :required="actionDialog.type === 'refund'"
          ></textarea>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeActionDialog">
            {{ t('common.cancel') }}
          </button>
          <button
            type="submit"
            form="merchant-action-form"
            :disabled="actionDialog.submitting"
            class="btn btn-primary"
          >
            {{ actionDialog.submitting ? t('common.processing') : t('common.confirm') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { merchantAPI, type Merchant, type MerchantStatus } from '@/api'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()
const router = useRouter()

const items = ref<Merchant[]>([])
const total = ref(0)
const loading = ref(false)
const searchQuery = ref('')
const statusFilter = ref<'' | MerchantStatus>('')
const page = ref(1)
const pageSize = ref(20)

const statusOptions = computed(() => [
  { value: '', label: t('merchant.admin.allStatus') },
  { value: 'active', label: t('merchant.status.active') },
  { value: 'suspended', label: t('merchant.status.suspended') },
])

const columns = computed<Column[]>(() => [
  { key: 'id', label: t('merchant.fields.id') },
  { key: 'name', label: t('merchant.fields.name') },
  { key: 'owner_user_id', label: t('merchant.fields.ownerUserId') },
  { key: 'domains', label: t('merchant.fields.domains') },
  { key: 'sub_user_count', label: t('merchant.fields.subUserCount') },
  { key: 'owner_balance', label: t('merchant.fields.ownerBalance') },
  { key: 'status', label: t('merchant.fields.status') },
  { key: 'created_at', label: t('merchant.fields.createdAt') },
  { key: 'actions', label: t('common.actions') },
])

const filteredItems = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return items.value
  return items.value.filter((m) => m.name.toLowerCase().includes(q))
})

interface ActionDialogState {
  show: boolean
  type: 'recharge' | 'refund' | 'status'
  merchant: Merchant | null
  amount: number
  reason: string
  newStatus: MerchantStatus
  submitting: boolean
}

const actionDialog = ref<ActionDialogState>({
  show: false,
  type: 'recharge',
  merchant: null,
  amount: 0,
  reason: '',
  newStatus: 'active',
  submitting: false,
})

const actionDialogTitle = computed(() => {
  if (!actionDialog.value.merchant) return ''
  const name = actionDialog.value.merchant.name
  if (actionDialog.value.type === 'recharge') {
    return t('merchant.admin.dialog.rechargeTitle', { name })
  }
  if (actionDialog.value.type === 'refund') {
    return t('merchant.admin.dialog.refundTitle', { name })
  }
  return actionDialog.value.newStatus === 'suspended'
    ? t('merchant.admin.dialog.suspendTitle', { name })
    : t('merchant.admin.dialog.activateTitle', { name })
})

async function reload(): Promise<void> {
  loading.value = true
  try {
    const offset = (page.value - 1) * pageSize.value
    const status = statusFilter.value || undefined
    const res = await merchantAPI.adminList(status, offset, pageSize.value)
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
  void reload()
}

function onPageSizeChange(size: number): void {
  pageSize.value = size
  page.value = 1
  void reload()
}

function goCreate(): void {
  router.push('/admin/merchants/new')
}

function goDetail(id: number): void {
  router.push(`/admin/merchants/${id}`)
}

function openRecharge(m: Merchant): void {
  actionDialog.value = {
    show: true,
    type: 'recharge',
    merchant: m,
    amount: 0,
    reason: '',
    newStatus: 'active',
    submitting: false,
  }
}

function openRefund(m: Merchant): void {
  actionDialog.value = {
    show: true,
    type: 'refund',
    merchant: m,
    amount: 0,
    reason: '',
    newStatus: 'active',
    submitting: false,
  }
}

function openStatusChange(m: Merchant, status: MerchantStatus): void {
  actionDialog.value = {
    show: true,
    type: 'status',
    merchant: m,
    amount: 0,
    reason: '',
    newStatus: status,
    submitting: false,
  }
}

function closeActionDialog(): void {
  actionDialog.value.show = false
}

async function submitAction(): Promise<void> {
  const m = actionDialog.value.merchant
  if (!m) return
  actionDialog.value.submitting = true
  try {
    if (actionDialog.value.type === 'recharge') {
      if (actionDialog.value.amount <= 0) {
        appStore.showError(t('merchant.errors.invalid_amount', t('merchant.errors.invalidAmount')))
        return
      }
      await merchantAPI.adminRecharge(m.id, actionDialog.value.amount, actionDialog.value.reason || undefined)
      appStore.showSuccess(t('merchant.admin.toast.rechargeSuccess'))
    } else if (actionDialog.value.type === 'refund') {
      if (actionDialog.value.amount <= 0) {
        appStore.showError(t('merchant.errors.invalid_amount', t('merchant.errors.invalidAmount')))
        return
      }
      if (!actionDialog.value.reason.trim()) {
        appStore.showError(t('merchant.errors.reasonRequired'))
        return
      }
      await merchantAPI.adminRefund(m.id, actionDialog.value.amount, actionDialog.value.reason)
      appStore.showSuccess(t('merchant.admin.toast.refundSuccess'))
    } else {
      await merchantAPI.adminSetStatus(m.id, actionDialog.value.newStatus, actionDialog.value.reason || undefined)
      appStore.showSuccess(t('merchant.admin.toast.statusUpdated'))
    }
    closeActionDialog()
    await reload()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    actionDialog.value.submitting = false
  }
}

onMounted(() => {
  void reload()
})
</script>
