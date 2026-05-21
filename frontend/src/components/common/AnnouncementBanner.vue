<template>
  <div
    v-if="visibleItems.length > 0"
    class="relative z-20 w-full border-b border-blue-200/60 bg-gradient-to-r from-blue-50 via-indigo-50 to-purple-50 dark:border-blue-900/40 dark:from-blue-950/40 dark:via-indigo-950/30 dark:to-purple-950/20"
  >
    <div class="mx-auto flex h-10 max-w-7xl items-center gap-3 px-4">
      <!-- Megaphone icon -->
      <span class="flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-md bg-gradient-to-br from-blue-500 to-indigo-600 text-white shadow-sm">
        <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M11 5.882V19.24a1.76 1.76 0 01-3.417.592l-2.147-6.15M18 13a3 3 0 100-6M5.436 13.683A4.001 4.001 0 017 6h1.832c4.1 0 7.625-1.234 9.168-3v14c-1.543-1.766-5.067-3-9.168-3H7a3.988 3.988 0 01-1.564-.317z" />
        </svg>
      </span>

      <!-- Scrolling title (cross-fade between items) -->
      <button
        type="button"
        @click="openDetail(current)"
        class="group relative flex min-w-0 flex-1 items-center text-left"
        :aria-label="t('announcements.title')"
      >
        <Transition name="ann-banner-fade" mode="out-in">
          <span
            :key="current.id"
            class="block min-w-0 truncate text-sm font-medium text-gray-800 transition-colors group-hover:text-blue-700 dark:text-gray-200 dark:group-hover:text-blue-300"
          >
            {{ current.title }}
          </span>
        </Transition>
      </button>

      <!-- Pagination dots (only when multiple) -->
      <div v-if="visibleItems.length > 1" class="flex flex-shrink-0 items-center gap-1">
        <span
          v-for="(item, idx) in visibleItems"
          :key="item.id"
          class="h-1.5 w-1.5 rounded-full transition-all"
          :class="idx === currentIndex
            ? 'w-3 bg-blue-600 dark:bg-blue-400'
            : 'bg-gray-400/60 dark:bg-gray-500/60'"
        ></span>
      </div>

      <!-- Dismiss current -->
      <button
        type="button"
        @click="dismissCurrent"
        class="flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-md text-gray-500 transition-colors hover:bg-white/60 hover:text-gray-800 dark:text-gray-400 dark:hover:bg-dark-700/60 dark:hover:text-gray-200"
        :aria-label="t('common.close')"
      >
        <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>

    <!-- Detail modal -->
    <Teleport to="body">
      <Transition name="ann-banner-detail">
        <div
          v-if="detail"
          class="fixed inset-0 z-[110] flex items-start justify-center overflow-y-auto bg-black/60 p-4 pt-[8vh] backdrop-blur-sm"
          @click.self="closeDetail"
        >
          <div class="w-full max-w-[680px] overflow-hidden rounded-2xl bg-white shadow-2xl ring-1 ring-black/5 dark:bg-dark-800 dark:ring-white/10">
            <div class="flex items-start justify-between gap-4 border-b border-gray-100 px-6 py-4 dark:border-dark-700">
              <div class="min-w-0">
                <h2 class="truncate text-lg font-semibold text-gray-900 dark:text-white">{{ detail.title }}</h2>
                <time class="mt-1 block text-xs text-gray-500 dark:text-gray-400">
                  {{ formatRelativeWithDateTime(detail.created_at) }}
                </time>
              </div>
              <button
                type="button"
                @click="closeDetail"
                class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-800 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-gray-200"
                :aria-label="t('common.close')"
              >
                <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
            <div class="max-h-[60vh] overflow-y-auto bg-white px-6 py-5 dark:bg-dark-800">
              <div class="markdown-body prose prose-sm max-w-none dark:prose-invert" v-html="renderedDetail"></div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { storeToRefs } from 'pinia'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { useAnnouncementStore } from '@/stores/announcements'
import { formatRelativeWithDateTime } from '@/utils/format'
import type { UserAnnouncement } from '@/types'

const { t } = useI18n()
const store = useAnnouncementStore()
const { bannerAnnouncements } = storeToRefs(store)

const ROTATE_INTERVAL_MS = 6000

marked.setOptions({ breaks: true, gfm: true })

// Locally dismissed ids (kept until next fetch / page refresh) — used to hide
// items the user closed without forcing a re-render race with markAsRead.
const localDismissed = ref<Set<number>>(new Set())

const visibleItems = computed<UserAnnouncement[]>(() =>
  bannerAnnouncements.value.filter((a) => !localDismissed.value.has(a.id))
)

const currentIndex = ref(0)

// Keep currentIndex within bounds as the list size changes.
watch(
  visibleItems,
  (items) => {
    if (items.length === 0) {
      currentIndex.value = 0
      return
    }
    if (currentIndex.value >= items.length) {
      currentIndex.value = 0
    }
  }
)

const current = computed<UserAnnouncement>(
  () => visibleItems.value[currentIndex.value] ?? visibleItems.value[0]
)

// Rotation timer — only runs when there's more than one item.
let rotateTimer: ReturnType<typeof setInterval> | null = null

function clearTimer() {
  if (rotateTimer !== null) {
    clearInterval(rotateTimer)
    rotateTimer = null
  }
}

function startTimer() {
  clearTimer()
  if (visibleItems.value.length <= 1) return
  rotateTimer = setInterval(() => {
    if (visibleItems.value.length <= 1) return
    currentIndex.value = (currentIndex.value + 1) % visibleItems.value.length
  }, ROTATE_INTERVAL_MS)
}

watch(
  () => visibleItems.value.length,
  () => startTimer(),
  { immediate: true }
)

onBeforeUnmount(clearTimer)

// Detail modal state
const detail = ref<UserAnnouncement | null>(null)

const renderedDetail = computed(() => {
  const content = detail.value?.content
  if (!content) return ''
  return DOMPurify.sanitize(marked.parse(content) as string)
})

function openDetail(item: UserAnnouncement) {
  if (!item) return
  detail.value = item
  // Opening counts as acknowledgement: mark as read.
  localDismissed.value.add(item.id)
  store.markAsRead(item.id)
}

function closeDetail() {
  detail.value = null
}

function dismissCurrent() {
  const item = current.value
  if (!item) return
  localDismissed.value.add(item.id)
  store.markAsRead(item.id)
}
</script>

<style scoped>
.ann-banner-fade-enter-active,
.ann-banner-fade-leave-active {
  transition: opacity 0.35s ease, transform 0.35s ease;
}

.ann-banner-fade-enter-from {
  opacity: 0;
  transform: translateY(6px);
}

.ann-banner-fade-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}

.ann-banner-detail-enter-active,
.ann-banner-detail-leave-active {
  transition: opacity 0.2s ease;
}

.ann-banner-detail-enter-from,
.ann-banner-detail-leave-to {
  opacity: 0;
}
</style>
