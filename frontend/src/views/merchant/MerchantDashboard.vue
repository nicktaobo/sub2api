<template>
  <AppLayout>
    <div class="space-y-6">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
          {{ t('merchant.owner.dashboard.title') }}
        </h1>
        <p class="text-sm text-gray-500 dark:text-dark-400">
          {{ t('merchant.owner.dashboard.description') }}
        </p>
      </div>

      <div v-if="loading" class="flex justify-center py-12">
        <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <div v-else-if="!info" class="card p-8 text-center text-gray-500">
        {{ t('merchant.owner.notMerchant') }}
      </div>

      <template v-else>
        <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <div class="card p-5">
            <div class="text-xs uppercase tracking-wide text-gray-500">
              {{ t('merchant.owner.dashboard.balance') }}
            </div>
            <div class="mt-2 font-mono text-2xl font-semibold text-gray-900 dark:text-white">
              ${{ Number(authStore.user?.balance ?? 0).toFixed(2) }}
            </div>
          </div>
          <div class="card p-5">
            <div class="text-xs uppercase tracking-wide text-gray-500">
              {{ t('merchant.owner.dashboard.todayEarnings') }}
            </div>
            <div class="mt-2 font-mono text-2xl font-semibold text-emerald-600">
              ${{ todayEarnings.toFixed(4) }}
            </div>
            <div class="mt-1 text-xs text-gray-400">
              {{ t('merchant.owner.dashboard.last24h') }}
            </div>
          </div>
          <div class="card p-5">
            <div class="text-xs uppercase tracking-wide text-gray-500">
              {{ t('merchant.fields.discount') }}
            </div>
            <div class="mt-2 font-mono text-2xl font-semibold">
              {{ Number(info.discount ?? 1).toFixed(4) }}
            </div>
          </div>
        </div>

        <div class="card space-y-3 p-6">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ info.name }}
          </h2>
          <div class="grid gap-3 text-sm sm:grid-cols-2">
            <div>
              <div class="text-xs text-gray-500">{{ t('merchant.fields.id') }}</div>
              <div class="font-mono">#{{ info.id }}</div>
            </div>
            <div>
              <div class="text-xs text-gray-500">{{ t('merchant.fields.status') }}</div>
              <span
                :class="[
                  'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
                  info.status === 'active'
                    ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
                    : 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400',
                ]"
              >
                {{ t('merchant.status.' + info.status) }}
              </span>
            </div>
            <div>
              <div class="text-xs text-gray-500">{{ t('merchant.fields.lowBalanceThreshold') }}</div>
              <div class="font-mono">${{ Number(info.low_balance_threshold ?? 0).toFixed(2) }}</div>
            </div>
            <div>
              <div class="text-xs text-gray-500">{{ t('merchant.fields.createdAt') }}</div>
              <div>{{ formatDateTime(info.created_at) }}</div>
            </div>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { merchantAPI, type MerchantInfo, type MerchantLedgerEntry } from '@/api'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const info = ref<MerchantInfo | null>(null)
const recentLedger = ref<MerchantLedgerEntry[]>([])
const loading = ref(false)

const todayEarnings = computed(() => {
  const cutoff = Date.now() - 24 * 60 * 60 * 1000
  return recentLedger.value
    .filter((e) => e.direction === 'credit' && new Date(e.created_at).getTime() >= cutoff)
    .reduce((sum, e) => sum + Number(e.amount || 0), 0)
})

async function load(): Promise<void> {
  loading.value = true
  try {
    info.value = await merchantAPI.info()
    const res = await merchantAPI.listLedger(0, 100)
    recentLedger.value = res.items || []
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
    info.value = null
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void load()
})
</script>
