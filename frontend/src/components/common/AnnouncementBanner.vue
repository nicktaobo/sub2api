<template>
  <div
    v-if="visibleItems.length > 0"
    class="flex h-9 w-full max-w-xl items-center gap-2.5 rounded-xl border border-blue-200/60 bg-gradient-to-r from-blue-50 via-indigo-50 to-purple-50 px-3 shadow-sm dark:border-blue-900/40 dark:from-blue-950/40 dark:via-indigo-950/30 dark:to-purple-950/20"
    @mouseenter="onPillEnter"
    @mouseleave="onPillLeave"
  >
    <!-- Megaphone icon -->
    <span class="flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-md bg-gradient-to-br from-blue-500 to-indigo-600 text-white shadow-sm">
      <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.2">
        <path stroke-linecap="round" stroke-linejoin="round" d="M11 5.882V19.24a1.76 1.76 0 01-3.417.592l-2.147-6.15M18 13a3 3 0 100-6M5.436 13.683A4.001 4.001 0 017 6h1.832c4.1 0 7.625-1.234 9.168-3v14c-1.543-1.766-5.067-3-9.168-3H7a3.988 3.988 0 01-1.564-.317z" />
      </svg>
    </span>

    <!-- Scrolling title (manual cross-fade between items). Vue <Transition>
         with :key swapping leaves enter-from/leave-active classes stuck on
         the element across rotations, freezing opacity at 0 — verified with
         live DOM inspection. We drive the fade ourselves: hide → swap text
         on the next animation frame → show.
         When the title overflows, hovering the pill scrolls the text leftward
         to reveal the rest; auto-rotation pauses while hovered. The title
         attribute provides a native tooltip as a fallback. -->
    <button
      type="button"
      @click="openDetail(current)"
      :title="displayedTitle"
      class="ann-banner-title group relative flex min-w-0 flex-1 items-center overflow-hidden text-left"
      :aria-label="t('announcements.title')"
    >
      <span
        ref="titleEl"
        class="block whitespace-nowrap text-sm font-medium text-gray-800 ease-linear group-hover:text-blue-700 dark:text-gray-200 dark:group-hover:text-blue-300"
        :class="titleFading ? 'opacity-0' : 'opacity-100'"
        :style="titleStyle"
      >
        {{ displayedTitle }}
      </span>
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

// Manual cross-fade: hold the displayed title for 300ms at opacity 0 while
// swapping text, then fade back in. Replaces a buggy Vue <Transition> that
// stranded enter-from/leave-active classes during keyed rotation.
const displayedTitle = ref<string>('')
const titleFading = ref<boolean>(false)
const TITLE_FADE_MS = 300

watch(
  current,
  (next, prev) => {
    if (!next) {
      displayedTitle.value = ''
      titleFading.value = false
      return
    }
    if (!prev || prev.id === next.id) {
      // First render or same item (e.g. dismissals shrink the list but the
      // active id is unchanged) — show immediately without a fade flicker.
      displayedTitle.value = next.title
      titleFading.value = false
      return
    }
    titleFading.value = true
    setTimeout(() => {
      displayedTitle.value = next.title
      titleFading.value = false
    }, TITLE_FADE_MS)
  },
  { immediate: true }
)

// Rotation timer — only runs when there's more than one item, and pauses
// while the user is hovering the pill so they can finish reading a marquee.
let rotateTimer: ReturnType<typeof setInterval> | null = null
const hovering = ref(false)

function clearTimer() {
  if (rotateTimer !== null) {
    clearInterval(rotateTimer)
    rotateTimer = null
  }
}

function startTimer() {
  clearTimer()
  if (visibleItems.value.length <= 1) return
  if (hovering.value) return
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

// Marquee on hover: when the title overflows its container, slide it leftward
// so the rest is readable, then snap back when the cursor leaves. Pure JS
// measurement so it always picks up the live displayed text width.
const titleEl = ref<HTMLSpanElement | null>(null)
const marqueeOffset = ref(0)
const marqueeDurationMs = ref(0)

const titleStyle = computed(() => ({
  transform: `translateX(${marqueeOffset.value}px)`,
  transition: `transform ${marqueeDurationMs.value}ms linear, opacity 300ms ease`,
}))

// 50ms per px of overflow → ~10s for a 200px overflow, comfortable reading.
const MARQUEE_PX_PER_MS = 0.05
const MARQUEE_TAIL_PADDING = 12
const MARQUEE_RESET_MS = 250

function onPillEnter() {
  hovering.value = true
  clearTimer()
  const el = titleEl.value
  const wrap = el?.parentElement
  if (!el || !wrap) return
  const overflow = el.scrollWidth - wrap.clientWidth
  if (overflow <= 0) return
  const distance = overflow + MARQUEE_TAIL_PADDING
  marqueeDurationMs.value = Math.max(2000, Math.round(distance / MARQUEE_PX_PER_MS))
  marqueeOffset.value = -distance
}

function onPillLeave() {
  hovering.value = false
  marqueeDurationMs.value = MARQUEE_RESET_MS
  marqueeOffset.value = 0
  startTimer()
}

// Reset marquee whenever the active title swaps — guards against the old
// translate sticking around when the user wasn't hovering at swap time.
watch(displayedTitle, () => {
  if (!hovering.value) {
    marqueeDurationMs.value = 0
    marqueeOffset.value = 0
  }
})

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
/* Right-edge fade so users see that the title is clipped (and thus
   scrollable on hover). Uses mask-image so it composes with the existing
   gradient background of the pill. */
.ann-banner-title {
  -webkit-mask-image: linear-gradient(
    to right,
    black 0,
    black calc(100% - 18px),
    transparent 100%
  );
  mask-image: linear-gradient(
    to right,
    black 0,
    black calc(100% - 18px),
    transparent 100%
  );
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
