<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex items-center justify-between gap-3">
        <div>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('merchant.owner.stats.title') }}
          </h1>
          <p class="text-sm text-gray-500 dark:text-dark-400">
            {{ t('merchant.owner.stats.description') }}
          </p>
        </div>
        <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="load">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>

      <!-- 5 个指标卡片 -->
      <div class="grid grid-cols-1 gap-4 md:grid-cols-3 xl:grid-cols-5">
        <div class="card p-4">
          <div class="text-xs text-gray-500">{{ t('merchant.owner.stats.totalRecharge') }}</div>
          <div class="mt-1 text-2xl font-bold text-purple-600">¥{{ fmt(stats?.total_recharge) }}</div>
        </div>
        <div class="card p-4">
          <div class="text-xs text-gray-500">{{ t('merchant.owner.stats.totalShare') }}</div>
          <div class="mt-1 text-2xl font-bold text-emerald-600">¥{{ fmt(stats?.total_share) }}</div>
          <div class="mt-1 text-xs text-gray-400">
            {{ t('merchant.owner.stats.shareRate') }}: {{ shareRate }}
          </div>
        </div>
        <div class="card p-4">
          <div class="text-xs text-gray-500">{{ t('merchant.owner.stats.withdrawn') }}</div>
          <div class="mt-1 text-2xl font-bold">¥{{ fmt(stats?.withdrawn_amount) }}</div>
        </div>
        <div class="card p-4">
          <div class="text-xs text-gray-500">{{ t('merchant.owner.stats.pending') }}</div>
          <div class="mt-1 text-2xl font-bold text-amber-600">¥{{ fmt(stats?.pending_withdraw) }}</div>
        </div>
        <div class="card p-4">
          <div class="text-xs text-gray-500">{{ t('merchant.owner.stats.available') }}</div>
          <div class="mt-1 text-2xl font-bold text-blue-600">¥{{ fmt(stats?.available_balance) }}</div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { merchantAPI, type MerchantStats } from '@/api/merchant'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const stats = ref<MerchantStats | null>(null)
const info = ref<{ discount: number } | null>(null)
const loading = ref(false)

const shareRate = computed(() => {
  if (!info.value) return '-'
  const r = (1 - info.value.discount) * 100
  return r.toFixed(0) + '%'
})

function fmt(n?: number) {
  return Number(n ?? 0).toFixed(2)
}

async function load() {
  loading.value = true
  try {
    const [s, m] = await Promise.all([
      merchantAPI.stats(),
      merchantAPI.info().catch(() => null),
    ])
    stats.value = s
    info.value = m
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>
