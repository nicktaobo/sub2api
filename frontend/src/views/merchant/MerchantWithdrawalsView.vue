<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex items-center justify-between gap-3">
        <div>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('merchant.owner.withdraw.title') }}
          </h1>
          <p class="text-sm text-gray-500 dark:text-dark-400">
            {{ t('merchant.owner.withdraw.description') }}
          </p>
        </div>
        <div class="flex items-center gap-3">
          <div class="rounded-lg border border-blue-500 px-4 py-2">
            <div class="text-xs text-gray-500">{{ t('merchant.owner.withdraw.available') }}</div>
            <div class="text-xl font-bold text-blue-600">¥{{ fmt(stats?.available_balance) }}</div>
          </div>
          <button class="btn btn-primary" :disabled="!stats || stats.available_balance <= 0" @click="openCreate">
            {{ t('merchant.owner.withdraw.apply') }}
          </button>
        </div>
      </div>

      <!-- 状态筛选 -->
      <div class="flex items-center gap-3">
        <span class="text-sm text-gray-500">{{ t('merchant.owner.withdraw.filterStatus') }}</span>
        <select v-model="filterStatus" class="input w-40" @change="load">
          <option value="">{{ t('common.all') }}</option>
          <option value="pending">{{ t('merchant.owner.withdraw.statusPending') }}</option>
          <option value="approved">{{ t('merchant.owner.withdraw.statusApproved') }}</option>
          <option value="paid">{{ t('merchant.owner.withdraw.statusPaid') }}</option>
          <option value="rejected">{{ t('merchant.owner.withdraw.statusRejected') }}</option>
        </select>
      </div>

      <!-- 列表 -->
      <div class="card overflow-hidden">
        <table class="w-full text-sm">
          <thead class="bg-gray-50 dark:bg-dark-800">
            <tr>
              <th class="px-4 py-3 text-left">ID</th>
              <th class="px-4 py-3 text-right">{{ t('merchant.owner.withdraw.amount') }}</th>
              <th class="px-4 py-3 text-left">{{ t('merchant.owner.withdraw.status') }}</th>
              <th class="px-4 py-3 text-left">{{ t('merchant.owner.withdraw.paymentMethod') }}</th>
              <th class="px-4 py-3 text-left">{{ t('merchant.owner.withdraw.paymentAccount') }}</th>
              <th class="px-4 py-3 text-left">{{ t('merchant.owner.withdraw.paymentName') }}</th>
              <th class="px-4 py-3 text-left">{{ t('merchant.owner.withdraw.createdAt') }}</th>
              <th class="px-4 py-3 text-left">{{ t('merchant.owner.withdraw.note') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading && !items.length">
              <td colspan="8" class="px-4 py-12 text-center text-gray-400">{{ t('common.loading') }}</td>
            </tr>
            <tr v-else-if="!items.length">
              <td colspan="8" class="px-4 py-12 text-center text-gray-400">{{ t('common.noData') }}</td>
            </tr>
            <tr v-for="w in items" :key="w.id" class="border-t border-gray-200 dark:border-dark-700">
              <td class="px-4 py-3">{{ w.id }}</td>
              <td class="px-4 py-3 text-right font-mono">¥{{ fmt(w.amount) }}</td>
              <td class="px-4 py-3">
                <span class="rounded-full px-2 py-0.5 text-xs" :class="statusClass(w.status)">
                  {{ t(`merchant.owner.withdraw.status${w.status[0].toUpperCase() + w.status.slice(1)}`) }}
                </span>
              </td>
              <td class="px-4 py-3">{{ paymentMethodLabel(w.payment_method) }}</td>
              <td class="px-4 py-3 break-all">{{ w.payment_account }}</td>
              <td class="px-4 py-3">{{ w.payment_name }}</td>
              <td class="px-4 py-3 text-gray-500">{{ formatDateTime(w.created_at) }}</td>
              <td class="px-4 py-3 text-gray-500">
                <span v-if="w.status === 'rejected' && w.reject_reason" class="text-red-500">{{ w.reject_reason }}</span>
                <span v-else>{{ w.note || '-' }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 申请提现对话框 -->
      <BaseDialog
        :show="dialog.open"
        :title="t('merchant.owner.withdraw.applyTitle')"
        width="normal"
        @close="dialog.open = false"
      >
        <div class="space-y-4">
          <div class="rounded-md bg-blue-50 px-4 py-3 dark:bg-blue-900/20">
            <p class="text-xs text-gray-500">{{ t('merchant.owner.withdraw.available') }}</p>
            <p class="text-xl font-bold text-blue-600">¥{{ fmt(stats?.available_balance) }}</p>
          </div>
          <div>
            <label class="label">{{ t('merchant.owner.withdraw.amount') }} <span class="text-red-500">*</span></label>
            <input v-model.number="dialog.form.amount" type="number" min="0" step="0.01" :max="stats?.available_balance" class="input" />
          </div>
          <div>
            <label class="label">{{ t('merchant.owner.withdraw.paymentMethod') }} <span class="text-red-500">*</span></label>
            <select v-model="dialog.form.payment_method" class="input">
              <option value="alipay">{{ t('merchant.owner.withdraw.methodAlipay') }}</option>
              <option value="wechat">{{ t('merchant.owner.withdraw.methodWechat') }}</option>
              <option value="bank">{{ t('merchant.owner.withdraw.methodBank') }}</option>
              <option value="usdt">{{ t('merchant.owner.withdraw.methodUsdt') }}</option>
              <option value="other">{{ t('merchant.owner.withdraw.methodOther') }}</option>
            </select>
          </div>
          <div>
            <label class="label">{{ t('merchant.owner.withdraw.paymentAccount') }} <span class="text-red-500">*</span></label>
            <input v-model="dialog.form.payment_account" type="text" class="input" />
          </div>
          <div>
            <label class="label">{{ t('merchant.owner.withdraw.paymentName') }} <span class="text-red-500">*</span></label>
            <input v-model="dialog.form.payment_name" type="text" class="input" />
          </div>
          <div>
            <label class="label">{{ t('merchant.owner.withdraw.note') }}</label>
            <textarea v-model="dialog.form.note" rows="2" class="input"></textarea>
          </div>
        </div>
        <template #footer>
          <button class="btn btn-secondary" @click="dialog.open = false">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="submitting" @click="onSubmit">
            {{ submitting ? t('common.saving') : t('merchant.owner.withdraw.submit') }}
          </button>
        </template>
      </BaseDialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { merchantAPI, type WithdrawRequest, type MerchantStats } from '@/api/merchant'
import { formatDateTime } from '@/utils/format'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
void Icon

const items = ref<WithdrawRequest[]>([])
const stats = ref<MerchantStats | null>(null)
const loading = ref(false)
const submitting = ref(false)
const filterStatus = ref('')

const dialog = reactive({
  open: false,
  form: {
    amount: 0,
    payment_method: 'alipay',
    payment_account: '',
    payment_name: '',
    note: '',
  },
})

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
    const [list, s] = await Promise.all([
      merchantAPI.listWithdrawals(filterStatus.value || undefined),
      merchantAPI.stats().catch(() => null),
    ])
    items.value = list.items || []
    stats.value = s
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

function openCreate() {
  dialog.form = { amount: stats.value?.available_balance || 0, payment_method: 'alipay', payment_account: '', payment_name: '', note: '' }
  dialog.open = true
}

async function onSubmit() {
  if (!dialog.form.amount || dialog.form.amount <= 0) {
    appStore.showError(t('merchant.errors.invalidAmount'))
    return
  }
  submitting.value = true
  try {
    await merchantAPI.createWithdrawal(dialog.form)
    appStore.showSuccess(t('merchant.owner.withdraw.submitted'))
    dialog.open = false
    await load()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    submitting.value = false
  }
}

onMounted(load)
</script>
