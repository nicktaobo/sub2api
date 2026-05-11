<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex items-center justify-between gap-3">
        <div>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('merchant.owner.subUsers.title') }}
          </h1>
          <p class="text-sm text-gray-500 dark:text-dark-400">
            {{ t('merchant.owner.subUsers.description') }}
          </p>
        </div>
        <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="load">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>

      <!-- 搜索栏 -->
      <div class="flex items-center gap-3">
        <div class="relative flex-1 max-w-md">
          <input
            v-model="searchInput"
            type="text"
            class="input pl-10"
            :placeholder="t('merchant.owner.subUsers.searchPlaceholder')"
            @keyup.enter="onSearch"
          />
          <Icon name="search" size="sm" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
        </div>
        <button class="btn btn-primary" @click="onSearch">{{ t('common.search') }}</button>
      </div>

      <!-- 列表 -->
      <div class="card overflow-hidden">
        <table class="w-full text-sm">
          <thead class="bg-gray-50 dark:bg-dark-800">
            <tr>
              <th class="px-4 py-3 text-left">ID</th>
              <th class="px-4 py-3 text-left">{{ t('merchant.owner.subUsers.email') }}</th>
              <th class="px-4 py-3 text-left">{{ t('merchant.owner.subUsers.username') }}</th>
              <th class="px-4 py-3 text-right">{{ t('merchant.owner.subUsers.balance') }}</th>
              <th class="px-4 py-3 text-left">{{ t('merchant.owner.subUsers.status') }}</th>
              <th class="px-4 py-3 text-left">{{ t('merchant.owner.subUsers.createdAt') }}</th>
              <th class="px-4 py-3 text-left">{{ t('merchant.owner.subUsers.lastActiveAt') }}</th>
              <th class="px-4 py-3 text-right">{{ t('merchant.owner.subUsers.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading && !items.length">
              <td colspan="8" class="px-4 py-12 text-center text-gray-400">{{ t('common.loading') }}</td>
            </tr>
            <tr v-else-if="!items.length">
              <td colspan="8" class="px-4 py-12 text-center text-gray-400">{{ t('common.noData') }}</td>
            </tr>
            <tr v-for="u in items" :key="u.id" class="border-t border-gray-200 dark:border-dark-700">
              <td class="px-4 py-3 font-mono">{{ u.id }}</td>
              <td class="px-4 py-3">{{ u.email }}</td>
              <td class="px-4 py-3 text-gray-600 dark:text-dark-300">{{ u.username || '-' }}</td>
              <td class="px-4 py-3 text-right font-mono">${{ Number(u.balance || 0).toFixed(2) }}</td>
              <td class="px-4 py-3">
                <span
                  class="rounded-full px-2 py-0.5 text-xs"
                  :class="u.status === 'active' ? 'bg-emerald-100 text-emerald-700' : 'bg-gray-200 text-gray-600'"
                >{{ u.status }}</span>
              </td>
              <td class="px-4 py-3 text-gray-500">{{ u.created_at ? formatDateTime(u.created_at) : '-' }}</td>
              <td class="px-4 py-3 text-gray-500">{{ u.last_active_at ? formatDateTime(u.last_active_at) : '-' }}</td>
              <td class="px-4 py-3 text-right">
                <button class="btn btn-sm btn-primary" @click="openPay(u)">
                  {{ t('merchant.owner.subUsers.payAction') }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>

        <!-- 分页 -->
        <div v-if="total > 0" class="flex items-center justify-between border-t border-gray-200 px-4 py-3 dark:border-dark-700">
          <div class="text-sm text-gray-500">
            {{ t('merchant.owner.subUsers.totalCount', { total }) }}
          </div>
          <div class="flex items-center gap-2">
            <button class="btn btn-sm btn-secondary" :disabled="page <= 1" @click="onPrev">{{ t('common.prev') }}</button>
            <span class="text-sm">{{ page }} / {{ Math.max(1, Math.ceil(total / pageSize)) }}</span>
            <button class="btn btn-sm btn-secondary" :disabled="page >= Math.ceil(total / pageSize)" @click="onNext">{{ t('common.next') }}</button>
          </div>
        </div>
      </div>

      <!-- 充值对话框 -->
      <BaseDialog
        :show="payDialog.open"
        :title="t('merchant.owner.subUsers.payTitle')"
        width="normal"
        @close="payDialog.open = false"
      >
        <div class="space-y-4">
          <div class="rounded-md bg-gray-50 px-4 py-3 dark:bg-dark-800">
            <p class="text-xs text-gray-500">{{ t('merchant.owner.subUsers.targetUser') }}</p>
            <p class="font-medium">#{{ payDialog.user?.id }} {{ payDialog.user?.email }}</p>
            <p class="mt-1 text-xs text-gray-500">
              {{ t('merchant.owner.subUsers.currentBalance') }}: ${{ Number(payDialog.user?.balance || 0).toFixed(2) }}
            </p>
          </div>
          <div>
            <label class="label">{{ t('merchant.owner.subUsers.amount') }} <span class="text-red-500">*</span></label>
            <input v-model.number="payDialog.amount" type="number" min="0" step="0.01" class="input" />
          </div>
          <div>
            <label class="label">{{ t('merchant.owner.subUsers.reason') }}</label>
            <textarea v-model="payDialog.reason" rows="2" class="input"></textarea>
          </div>
        </div>
        <template #footer>
          <button class="btn btn-secondary" @click="payDialog.open = false">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="paying" @click="onPay">
            {{ paying ? t('common.saving') : t('merchant.owner.subUsers.confirmPay') }}
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
import { merchantAPI, type SubUserSummary } from '@/api/merchant'
import { formatDateTime } from '@/utils/format'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const items = ref<SubUserSummary[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const searchInput = ref('')
const searchQuery = ref('')
const loading = ref(false)
const paying = ref(false)

const payDialog = reactive({
  open: false,
  user: null as SubUserSummary | null,
  amount: 0,
  reason: '',
})

async function load() {
  loading.value = true
  try {
    const offset = (page.value - 1) * pageSize
    const res = await merchantAPI.listSubUsers(searchQuery.value || undefined, offset, pageSize)
    items.value = res.items || []
    total.value = res.total || 0
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

function onSearch() {
  searchQuery.value = searchInput.value.trim()
  page.value = 1
  void load()
}

function onPrev() {
  if (page.value > 1) {
    page.value--
    void load()
  }
}

function onNext() {
  if (page.value < Math.ceil(total.value / pageSize)) {
    page.value++
    void load()
  }
}

function openPay(u: SubUserSummary) {
  payDialog.user = u
  payDialog.amount = 0
  payDialog.reason = ''
  payDialog.open = true
}

async function onPay() {
  if (!payDialog.user || !payDialog.amount || payDialog.amount <= 0) {
    appStore.showError(t('merchant.errors.invalidAmount'))
    return
  }
  paying.value = true
  try {
    await merchantAPI.payToUser(payDialog.user.id, payDialog.amount, payDialog.reason || undefined)
    appStore.showSuccess(t('merchant.owner.subUsers.paySuccess'))
    payDialog.open = false
    await load()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    paying.value = false
  }
}

onMounted(load)
</script>
