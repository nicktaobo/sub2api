<template>
  <div class="min-h-screen bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white">
    <header class="border-b border-gray-200 bg-white/95 dark:border-dark-800 dark:bg-dark-900/95">
      <div class="mx-auto flex max-w-6xl items-center justify-between gap-4 px-4 py-4 sm:px-6">
        <RouterLink to="/home" class="flex min-w-0 items-center gap-3">
          <span class="flex h-10 w-10 flex-shrink-0 items-center justify-center overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-gray-200 dark:bg-dark-800 dark:ring-dark-700">
            <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
          </span>
          <span class="truncate text-base font-semibold text-gray-950 dark:text-white">
            {{ siteName }}
          </span>
        </RouterLink>
        <div class="flex flex-shrink-0 items-center gap-2">
          <LocaleSwitcher />
          <RouterLink
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="inline-flex items-center justify-center rounded-lg bg-primary-600 px-4 py-2 text-sm font-semibold text-white shadow-sm shadow-primary-600/20 transition hover:bg-primary-700"
          >
            {{ t('home.dashboard') }}
          </RouterLink>
          <RouterLink
            v-else
            to="/login"
            class="inline-flex items-center justify-center rounded-lg bg-primary-600 px-4 py-2 text-sm font-semibold text-white shadow-sm shadow-primary-600/20 transition hover:bg-primary-700"
          >
            {{ t('home.login') }}
          </RouterLink>
        </div>
      </div>
    </header>

    <main class="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:py-10">
      <div class="mb-8 border-b border-gray-200 pb-6 dark:border-dark-700">
        <div class="flex items-start gap-4">
          <span class="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-md bg-primary-50 text-primary-700 dark:bg-primary-500/10 dark:text-primary-300">
            <Icon name="grid" size="md" />
          </span>
          <div class="min-w-0">
            <p class="text-sm font-medium text-primary-700 dark:text-primary-300">{{ t('publicModels.badge') }}</p>
            <h1 class="mt-2 break-words text-2xl font-bold tracking-normal text-gray-950 dark:text-white sm:text-3xl">
              {{ t('publicModels.title') }}
            </h1>
            <p class="mt-3 text-sm text-gray-600 dark:text-dark-300">
              {{ t('publicModels.subtitle') }}
            </p>
          </div>
        </div>
      </div>

      <div class="mb-6 flex flex-wrap items-center gap-2">
        <button
          type="button"
          class="platform-chip"
          :class="platformFilter === '' ? 'platform-chip-active' : ''"
          @click="platformFilter = ''"
        >
          <Icon name="grid" size="xs" class="mr-1" />
          {{ t('publicModels.filterAll') }}
          <span class="ml-1 text-[10px] opacity-70">{{ groups.length }}</span>
        </button>
        <button
          v-for="p in platformOptions"
          :key="p.name"
          type="button"
          class="platform-chip"
          :class="platformFilter === p.name ? 'platform-chip-active' : ''"
          @click="platformFilter = p.name"
        >
          <PlatformIcon :platform="(p.name as GroupPlatform)" size="xs" class="mr-1" />
          {{ p.name }}
          <span class="ml-1 text-[10px] opacity-70">{{ p.count }}</span>
        </button>
      </div>

      <div v-if="loading && !groups.length" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <div v-for="i in 6" :key="i" class="h-44 animate-pulse rounded-xl bg-white dark:bg-dark-800/40"></div>
      </div>

      <div
        v-else-if="loadError"
        class="rounded-lg border border-red-200 bg-red-50 p-6 text-red-700 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-200"
      >
        <h2 class="text-base font-semibold">{{ t('publicModels.loadErrorTitle') }}</h2>
        <p class="mt-2 text-sm">{{ t('publicModels.loadErrorDescription') }}</p>
      </div>

      <div
        v-else-if="!filteredGroups.length"
        class="rounded-lg border border-dashed border-gray-300 bg-white px-6 py-14 text-center text-sm text-gray-500 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-400"
      >
        {{ t('publicModels.empty') }}
      </div>

      <div v-else class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <article
          v-for="group in filteredGroups"
          :key="group.id"
          class="group-card"
        >
          <header class="flex items-start justify-between gap-3">
            <div class="flex items-start gap-3">
              <span class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-700 dark:bg-dark-800 dark:text-dark-200">
                <PlatformIcon :platform="(group.platform as GroupPlatform)" size="md" />
              </span>
              <div class="min-w-0">
                <h3 class="truncate text-base font-semibold text-gray-950 dark:text-white">{{ group.name }}</h3>
                <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-dark-400">{{ group.platform }}</p>
              </div>
            </div>
            <span class="rounded-full bg-primary-50 px-2.5 py-0.5 text-xs font-semibold text-primary-700 dark:bg-primary-500/15 dark:text-primary-200">
              {{ formatRate(group.rate_multiplier) }}
            </span>
          </header>

          <div class="mt-4 flex items-center justify-between text-xs text-gray-500 dark:text-dark-400">
            <span>{{ t('publicModels.modelCount', { count: group.models.length }) }}</span>
          </div>

          <ul class="mt-3 flex flex-wrap gap-1.5">
            <li
              v-for="m in displayedModels(group)"
              :key="m.name"
              class="model-chip"
            >
              {{ m.name }}
            </li>
            <li
              v-if="group.models.length > MAX_MODELS_PER_CARD"
              class="model-chip model-chip-more"
            >
              +{{ group.models.length - MAX_MODELS_PER_CARD }}
            </li>
          </ul>
        </article>
      </div>

      <p class="mt-8 text-center text-xs text-gray-500 dark:text-dark-400">
        {{ t('publicModels.footnote') }}
      </p>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import userChannelsAPI, { type UserPricingGroup } from '@/api/channels'
import { useAuthStore, useAppStore, useMerchantStore } from '@/stores'
import type { GroupPlatform } from '@/types'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const merchantStore = useMerchantStore()

const MAX_MODELS_PER_CARD = 12

const groups = ref<UserPricingGroup[]>([])
const loading = ref(false)
const loadError = ref(false)
const platformFilter = ref<string>('')

const platformOptions = computed(() => {
  const counts = new Map<string, number>()
  for (const g of groups.value) {
    counts.set(g.platform, (counts.get(g.platform) ?? 0) + 1)
  }
  return Array.from(counts.entries())
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count || a.name.localeCompare(b.name))
})

const filteredGroups = computed(() =>
  platformFilter.value
    ? groups.value.filter((g) => g.platform === platformFilter.value)
    : groups.value,
)

function displayedModels(group: UserPricingGroup) {
  return group.models.slice(0, MAX_MODELS_PER_CARD)
}

function formatRate(rate: number): string {
  const r = Number(rate || 1)
  if (Math.abs(r - 1) < 1e-6) return '1x'
  if (r >= 10) return `${r.toFixed(0)}x`
  return `${parseFloat(r.toFixed(3))}x`
}

const siteName = computed(() =>
  (merchantStore.isMerchantSite && merchantStore.siteName) ||
  appStore.cachedPublicSettings?.site_name ||
  appStore.siteName ||
  'Sub2API'
)
const siteLogo = computed(() =>
  (merchantStore.isMerchantSite && merchantStore.siteLogo) ||
  appStore.cachedPublicSettings?.site_logo ||
  appStore.siteLogo ||
  ''
)
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')

async function reload() {
  loading.value = true
  loadError.value = false
  try {
    groups.value = await userChannelsAPI.getPublicPricingGroups()
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
  reload()
})
</script>

<style scoped>
.platform-chip {
  @apply inline-flex items-center rounded-full border border-gray-200 bg-white px-3 py-1.5 text-xs font-medium text-gray-700
         transition hover:border-primary-300 hover:text-primary-700
         dark:border-dark-700 dark:bg-dark-800/40 dark:text-dark-200 dark:hover:border-primary-500/40;
}
.platform-chip-active {
  @apply border-primary-500 bg-primary-50 text-primary-700
         dark:border-primary-500 dark:bg-primary-500/15 dark:text-primary-200;
}
.group-card {
  @apply rounded-xl border border-gray-200 bg-white p-5 transition-all duration-200
         hover:border-primary-200 hover:shadow-md
         dark:border-dark-700 dark:bg-dark-800/40 dark:hover:border-primary-500/40;
}
.model-chip {
  @apply inline-flex items-center rounded-md border border-gray-200 bg-gray-50 px-2 py-0.5 font-mono text-[12px] text-gray-700
         dark:border-dark-700 dark:bg-dark-800 dark:text-dark-200;
}
.model-chip-more {
  @apply border-primary-200 bg-primary-50 text-primary-700 dark:border-primary-500/30 dark:bg-primary-500/10 dark:text-primary-200;
}
</style>
