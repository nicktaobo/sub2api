<template>
  <AppLayout>
    <div class="mx-auto flex w-full max-w-3xl flex-col gap-6">
      <div>
        <h1 class="text-xl font-semibold text-gray-900 dark:text-gray-100">
          {{ t('merchantAffiliate.title') }}
        </h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('merchantAffiliate.subtitle') }}
        </p>
      </div>

      <div v-if="loading" class="h-40 animate-pulse rounded-xl bg-gray-100 dark:bg-dark-700/50"></div>

      <template v-else-if="overview">
        <!-- 未开启提示 -->
        <div
          v-if="!overview.enabled"
          class="rounded-xl border border-amber-200 bg-amber-50 px-5 py-4 text-sm text-amber-700 dark:border-amber-900/40 dark:bg-amber-900/20 dark:text-amber-300"
        >
          {{ t('merchantAffiliate.disabled') }}
        </div>

        <template v-else>
          <!-- 邀请链接 -->
          <div class="card p-5">
            <p class="text-sm font-medium text-gray-700 dark:text-gray-200">
              {{ t('merchantAffiliate.yourLink') }}
            </p>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('merchantAffiliate.rateNote', { rate: trimRate(overview.rate_percent) }) }}
            </p>
            <div class="mt-3 flex items-center gap-2">
              <input :value="inviteLink" readonly class="input flex-1 font-mono text-xs" />
              <button type="button" class="btn btn-primary whitespace-nowrap" @click="copyLink">
                {{ copied ? t('common.copied', 'Copied') : t('common.copy', 'Copy') }}
              </button>
            </div>
          </div>

          <!-- 统计 -->
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div class="card p-5">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('merchantAffiliate.inviteeCount') }}</p>
              <p class="mt-1 text-2xl font-semibold text-gray-900 dark:text-gray-100">{{ overview.invitee_count }}</p>
            </div>
            <div class="card p-5">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('merchantAffiliate.totalRebate') }}</p>
              <p class="mt-1 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">
                {{ formatMoney(overview.total_rebate) }}
              </p>
            </div>
          </div>

          <p class="text-xs text-gray-400 dark:text-gray-500">
            {{ t('merchantAffiliate.howItWorks') }}
          </p>
        </template>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import userAPI, { type MerchantAffiliateOverview } from '@/api/user'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const overview = ref<MerchantAffiliateOverview | null>(null)
const copied = ref(false)

const inviteLink = computed(() => {
  if (!overview.value?.aff_code) return ''
  const code = encodeURIComponent(overview.value.aff_code)
  if (typeof window === 'undefined') return `/register?aff=${code}`
  return `${window.location.origin}/register?aff=${code}`
})

function trimRate(n: number): string {
  return `${parseFloat(Number(n || 0).toFixed(2))}`
}

function formatMoney(n: number): string {
  return `$${Number(n || 0).toFixed(4)}`
}

async function copyLink() {
  try {
    await navigator.clipboard.writeText(inviteLink.value)
    copied.value = true
    setTimeout(() => (copied.value = false), 1500)
  } catch {
    // clipboard 不可用时静默
  }
}

onMounted(async () => {
  try {
    overview.value = await userAPI.getMerchantAffiliate()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    loading.value = false
  }
})
</script>
