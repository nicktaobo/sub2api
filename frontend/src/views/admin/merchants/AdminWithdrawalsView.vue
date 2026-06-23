<template>
  <AppLayout>
    <div class="space-y-6">
      <div>
        <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
          {{ t('merchant.admin.withdraw.title') }}
        </h1>
        <p class="text-sm text-gray-500 dark:text-dark-400">
          {{ t('merchant.admin.withdraw.description') }}
        </p>
      </div>

      <!-- 筛选 -->
      <div class="card flex items-center gap-3 p-4">
        <span class="text-sm text-gray-500">{{ t('merchant.admin.withdraw.filterStatus') }}</span>
        <select v-model="filterStatus" class="input w-40" @change="load">
          <option value="">{{ t('common.all') }}</option>
          <option value="pending">{{ t('merchant.owner.withdraw.statusPending') }}</option>
          <option value="approved">{{ t('merchant.owner.withdraw.statusApproved') }}</option>
          <option value="paid">{{ t('merchant.owner.withdraw.statusPaid') }}</option>
          <option value="rejected">{{ t('merchant.owner.withdraw.statusRejected') }}</option>
        </select>
        <span class="text-sm text-gray-500">{{ t('merchant.admin.withdraw.filterMerchant') }}</span>
        <div class="w-64">
          <MerchantSelectRemote v-model="filterMerchantId" :placeholder="t('merchant.admin.withdraw.merchantPlaceholder')" @update:modelValue="load" />
        </div>
        <button class="btn btn-secondary" @click="reset">{{ t('common.reset') }}</button>
        <button class="btn btn-secondary ml-auto" :disabled="loading" :title="t('common.refresh')" @click="load">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>

      <!-- 列表 -->
      <div class="card overflow-hidden">
        <table class="w-full text-sm">
          <thead class="bg-gray-50 dark:bg-dark-800">
            <tr>
              <th class="px-4 py-3 text-left">ID</th>
              <th class="px-4 py-3 text-left">{{ t('merchant.admin.withdraw.merchantId') }}</th>
              <th class="px-4 py-3 text-right">{{ t('merchant.admin.withdraw.amount') }}</th>
              <th class="px-4 py-3 text-left">{{ t('merchant.admin.withdraw.status') }}</th>
              <th class="px-4 py-3 text-left">{{ t('merchant.admin.withdraw.paymentMethod') }}</th>
              <th class="px-4 py-3 text-left">{{ t('merchant.admin.withdraw.paymentAccount') }}</th>
              <th class="px-4 py-3 text-left">{{ t('merchant.admin.withdraw.paymentName') }}</th>
              <th class="px-4 py-3 text-left">{{ t('merchant.admin.withdraw.note') }}</th>
              <th class="px-4 py-3 text-left">{{ t('merchant.admin.withdraw.createdAt') }}</th>
              <th class="px-4 py-3 text-right">{{ t('merchant.admin.withdraw.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading && !items.length">
              <td colspan="10" class="px-4 py-12 text-center text-gray-400">{{ t('common.loading') }}</td>
            </tr>
            <tr v-else-if="!items.length">
              <td colspan="10" class="px-4 py-12 text-center text-gray-400">{{ t('common.noData') }}</td>
            </tr>
            <tr v-for="w in items" :key="w.id" class="border-t border-gray-200 dark:border-dark-700">
              <td class="px-4 py-3">{{ w.id }}</td>
              <td class="px-4 py-3">{{ w.merchant_id }}</td>
              <td class="px-4 py-3 text-right font-mono">¥{{ fmt(w.amount) }}</td>
              <td class="px-4 py-3">
                <span class="rounded-full px-2 py-0.5 text-xs" :class="statusClass(w.status)">
                  {{ t(`merchant.owner.withdraw.status${w.status[0].toUpperCase() + w.status.slice(1)}`) }}
                </span>
              </td>
              <td class="px-4 py-3">{{ paymentMethodLabel(w.payment_method) }}</td>
              <td class="px-4 py-3 break-all">{{ w.payment_account }}</td>
              <td class="px-4 py-3">{{ w.payment_name }}</td>
              <td class="px-4 py-3 text-gray-500">
                <span v-if="w.status === 'rejected' && w.reject_reason" class="text-red-500">{{ w.reject_reason }}</span>
                <span v-else>{{ w.note || '-' }}</span>
              </td>
              <td class="px-4 py-3 text-gray-500">{{ formatDateTime(w.created_at) }}</td>
              <td class="px-4 py-3 text-right space-x-2">
                <template v-if="w.status === 'pending' || w.status === 'approved'">
                  <button class="btn btn-sm btn-primary" @click="onApprove(w)">{{ t('merchant.admin.withdraw.approve') }}</button>
                  <button class="btn btn-sm btn-danger" @click="onReject(w)">{{ t('merchant.admin.withdraw.reject') }}</button>
                </template>
                <span v-else>—</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- 拒绝原因对话框 -->
    <BaseDialog
      :show="rejectDialog.open"
      :title="t('merchant.admin.withdraw.rejectTitle')"
      width="normal"
      @close="rejectDialog.open = false"
    >
      <div class="space-y-3">
        <p class="text-sm">{{ t('merchant.admin.withdraw.rejectHint', { id: rejectDialog.id, amount: fmt(rejectDialog.amount) }) }}</p>
        <div>
          <label class="label">{{ t('merchant.admin.withdraw.rejectReason') }}</label>
          <textarea v-model="rejectDialog.reason" rows="3" class="input"></textarea>
        </div>
      </div>
      <template #footer>
        <button class="btn btn-secondary" @click="rejectDialog.open = false">{{ t('common.cancel') }}</button>
        <button class="btn btn-danger" @click="confirmReject">{{ t('merchant.admin.withdraw.confirmReject') }}</button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import MerchantSelectRemote from '@/components/common/MerchantSelectRemote.vue'
import { merchantAPI, type WithdrawRequest } from '@/api/merchant'
import { formatDateTime } from '@/utils/format'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const items = ref<WithdrawRequest[]>([])
const loading = ref(false)
const filterStatus = ref('')
const filterMerchantId = ref<number | null>(null)

const rejectDialog = reactive({ open: false, id: 0, amount: 0, reason: '' })

function fmt(n?: number) {
  return Number(n ?? 0).toFixed(2)
}

function statusClass(s: string) {
  switch (s) {
    case 'paid':
      return 'bg-emerald-100 text-emerald-700'
    case 'rejected':
      return 'bg-red-100 text-red-700'
    case 'approved':
      return 'bg-blue-100 text-blue-700'
    default:
      return 'bg-amber-100 text-amber-700'
  }
}

function paymentMethodLabel(m: string) {
  return t('merchant.owner.withdraw.method' + (m[0]?.toUpperCase() + m.slice(1)))
}

async function load() {
  loading.value = true
  try {
    const res = await merchantAPI.adminListWithdrawals(
      filterStatus.value || undefined,
      filterMerchantId.value || undefined,
    )
    items.value = res.items || []
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

function reset() {
  filterStatus.value = ''
  filterMerchantId.value = null
  void load()
}

async function onApprove(w: WithdrawRequest) {
  if (!confirm(t('merchant.admin.withdraw.confirmApprove', { amount: fmt(w.amount) }))) return
  try {
    await merchantAPI.adminApproveWithdrawal(w.id)
    appStore.showSuccess(t('merchant.admin.withdraw.approved'))
    await load()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  }
}

function onReject(w: WithdrawRequest) {
  rejectDialog.id = w.id
  rejectDialog.amount = w.amount
  rejectDialog.reason = ''
  rejectDialog.open = true
}

async function confirmReject() {
  try {
    await merchantAPI.adminRejectWithdrawal(rejectDialog.id, rejectDialog.reason)
    appStore.showSuccess(t('merchant.admin.withdraw.rejected'))
    rejectDialog.open = false
    await load()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  }
}

onMounted(load)
</script>
