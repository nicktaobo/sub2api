<template>
  <Teleport to="body">
    <Transition name="agreement-fade">
      <div
        v-if="visible"
        class="fixed inset-0 z-[150] flex items-stretch justify-center overflow-y-auto bg-gray-950/60 p-4 backdrop-blur-sm sm:items-center"
      >
        <div
          class="flex w-full max-w-4xl flex-col overflow-hidden rounded-2xl bg-white shadow-2xl ring-1 ring-black/10 dark:bg-dark-900 dark:ring-white/10"
          :style="{ maxHeight: 'min(900px, 92vh)' }"
        >
          <div class="border-b border-gray-100 bg-white px-6 py-5 dark:border-dark-800 dark:bg-dark-900">
            <div class="flex items-start gap-4">
              <span class="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-xl bg-primary-50 text-primary-700 ring-1 ring-primary-100 dark:bg-primary-500/10 dark:text-primary-300 dark:ring-primary-500/20">
                <Icon name="shield" size="md" />
              </span>
              <div class="min-w-0 flex-1">
                <h2 class="text-xl font-bold tracking-normal text-gray-950 dark:text-white">
                  {{ t('auth.registerAgreement.title') }}
                </h2>
                <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-dark-300">
                  {{ t('auth.registerAgreement.description') }}
                </p>
              </div>
            </div>
          </div>

          <div
            v-if="visibleDocuments.length > 1"
            class="flex flex-wrap gap-2 border-b border-gray-100 bg-gray-50/60 px-6 py-3 dark:border-dark-800 dark:bg-dark-950/40"
          >
            <button
              v-for="doc in visibleDocuments"
              :key="doc.id || doc.title"
              type="button"
              class="inline-flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-sm font-medium transition"
              :class="
                doc === activeDoc
                  ? 'border-primary-500 bg-primary-50 text-primary-700 dark:border-primary-500/40 dark:bg-primary-500/10 dark:text-primary-200'
                  : 'border-gray-200 bg-white text-gray-600 hover:border-primary-200 hover:text-primary-700 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-200 dark:hover:border-primary-500/30 dark:hover:text-primary-200'
              "
              @click="activeDocId = doc.id || doc.title"
            >
              <Icon :name="documentIcon(doc)" size="sm" />
              <span>{{ resolveTitle(doc) }}</span>
            </button>
          </div>

          <div class="flex-1 overflow-y-auto px-6 py-5">
            <template v-if="activeDoc">
              <div class="mb-4 flex items-center gap-2">
                <span class="flex h-7 w-7 items-center justify-center rounded-md bg-primary-50 text-primary-700 ring-1 ring-primary-100 dark:bg-primary-500/10 dark:text-primary-300 dark:ring-primary-500/20">
                  <Icon :name="documentIcon(activeDoc)" size="sm" />
                </span>
                <h3 class="text-base font-semibold text-gray-950 dark:text-white">
                  {{ resolveTitle(activeDoc) }}
                </h3>
              </div>
              <div
                v-if="activeHtml"
                class="legal-document-content"
                v-html="activeHtml"
              ></div>
              <p
                v-else
                class="rounded-xl border border-dashed border-gray-200 bg-gray-50/60 px-4 py-8 text-center text-sm text-gray-500 dark:border-dark-700 dark:bg-dark-800/40 dark:text-dark-400"
              >
                {{ t('auth.registerAgreement.emptyContent') }}
              </p>
            </template>
            <p
              v-else
              class="rounded-xl border border-dashed border-gray-200 bg-gray-50/60 px-4 py-6 text-center text-sm text-gray-500 dark:border-dark-700 dark:bg-dark-800/40 dark:text-dark-400"
            >
              {{ t('auth.registerAgreement.noDocuments') }}
            </p>
          </div>

          <div class="border-t border-gray-100 bg-gray-50/80 px-6 py-4 dark:border-dark-800 dark:bg-dark-950/60">
            <div class="grid grid-cols-2 gap-3">
              <button
                type="button"
                class="rounded-xl border border-gray-200 bg-white px-4 py-3 text-sm font-semibold text-gray-700 transition hover:bg-gray-100 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-200 dark:hover:bg-dark-700"
                @click="emit('decline')"
              >
                {{ t('auth.registerAgreement.decline') }}
              </button>
              <button
                type="button"
                :disabled="countdown > 0"
                class="rounded-xl bg-primary-600 px-4 py-3 text-sm font-semibold text-white shadow-sm shadow-primary-600/20 transition hover:bg-primary-700 disabled:cursor-not-allowed disabled:bg-primary-400 disabled:hover:bg-primary-400"
                @click="handleAccept"
              >
                {{ countdown > 0
                  ? t('auth.registerAgreement.acceptCountdown', { seconds: countdown })
                  : t('auth.registerAgreement.accept') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, ref, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import Icon from '@/components/icons/Icon.vue'
import {
  resolveLoginAgreementDocumentIcon,
  resolveLoginAgreementDocumentLocale,
  hasLoginAgreementTitle,
} from '@/utils/loginAgreement'
import type { LoginAgreementDocument } from '@/types'

const props = withDefaults(defineProps<{
  visible: boolean
  documents: LoginAgreementDocument[]
  countdownSeconds?: number
}>(), {
  countdownSeconds: 5,
})

const emit = defineEmits<{
  accept: []
  decline: []
}>()

const { t, locale } = useI18n()

marked.setOptions({
  breaks: true,
  gfm: true,
})

const countdown = ref<number>(props.countdownSeconds)
let timer: ReturnType<typeof setInterval> | null = null

function startCountdown(): void {
  stopCountdown()
  countdown.value = props.countdownSeconds
  timer = setInterval(() => {
    if (countdown.value > 0) {
      countdown.value -= 1
    }
    if (countdown.value <= 0) {
      stopCountdown()
    }
  }, 1000)
}

function stopCountdown(): void {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

watch(
  () => props.visible,
  (value) => {
    if (value) {
      startCountdown()
    } else {
      stopCountdown()
    }
  },
  { immediate: true },
)

onUnmounted(() => {
  stopCountdown()
})

const visibleDocuments = computed(() =>
  props.documents.filter((doc) => hasLoginAgreementTitle(doc, locale.value)),
)

const activeDocId = ref<string>('')

watch(
  visibleDocuments,
  (docs) => {
    if (docs.length === 0) {
      activeDocId.value = ''
      return
    }
    const exists = docs.some((d) => (d.id || d.title) === activeDocId.value)
    if (!exists) {
      activeDocId.value = docs[0].id || docs[0].title
    }
  },
  { immediate: true },
)

const activeDoc = computed<LoginAgreementDocument | null>(() => {
  return (
    visibleDocuments.value.find((d) => (d.id || d.title) === activeDocId.value) ||
    visibleDocuments.value[0] ||
    null
  )
})

const activeHtml = computed(() => {
  if (!activeDoc.value) return ''
  const resolved = resolveLoginAgreementDocumentLocale(activeDoc.value, locale.value)
  const md = resolved.content_md.trim()
  if (!md) return ''
  const html = marked.parse(md) as string
  return DOMPurify.sanitize(html)
})

function resolveTitle(doc: LoginAgreementDocument): string {
  return resolveLoginAgreementDocumentLocale(doc, locale.value).title
}

function documentIcon(doc: LoginAgreementDocument): 'document' | 'shield' | 'globe' | 'cog' {
  return resolveLoginAgreementDocumentIcon(doc.id, resolveTitle(doc))
}

function handleAccept(): void {
  if (countdown.value > 0) {
    return
  }
  emit('accept')
}
</script>

<style scoped>
.agreement-fade-enter-active,
.agreement-fade-leave-active {
  transition: opacity 0.18s ease;
}

.agreement-fade-enter-from,
.agreement-fade-leave-to {
  opacity: 0;
}

.agreement-fade-enter-active > div,
.agreement-fade-leave-active > div {
  transition: transform 0.18s ease, opacity 0.18s ease;
}

.agreement-fade-enter-from > div,
.agreement-fade-leave-to > div {
  opacity: 0;
  transform: translateY(8px) scale(0.98);
}

.legal-document-content {
  line-height: 1.75;
  overflow-wrap: anywhere;
  color: inherit;
}

.legal-document-content :deep(h1) {
  @apply mb-3 mt-6 border-b border-gray-200 pb-2 text-2xl font-bold dark:border-dark-700;
}

.legal-document-content :deep(h2) {
  @apply mb-3 mt-5 text-xl font-bold;
}

.legal-document-content :deep(h3) {
  @apply mb-2 mt-4 text-lg font-semibold;
}

.legal-document-content :deep(h4) {
  @apply mb-2 mt-3 text-base font-semibold;
}

.legal-document-content :deep(p) {
  @apply mb-3 text-sm text-gray-700 dark:text-dark-200;
}

.legal-document-content :deep(a) {
  @apply text-primary-600 underline underline-offset-4 hover:text-primary-700 dark:text-primary-300 dark:hover:text-primary-200;
}

.legal-document-content :deep(ul) {
  @apply mb-3 list-disc pl-6 text-sm;
}

.legal-document-content :deep(ol) {
  @apply mb-3 list-decimal pl-6 text-sm;
}

.legal-document-content :deep(li) {
  @apply mb-1 text-gray-700 dark:text-dark-200;
}

.legal-document-content :deep(blockquote) {
  @apply my-4 border-l-4 border-gray-300 pl-4 text-sm text-gray-600 dark:border-dark-600 dark:text-dark-300;
}

.legal-document-content :deep(code) {
  @apply rounded bg-gray-100 px-1.5 py-0.5 font-mono text-xs dark:bg-dark-800;
}

.legal-document-content :deep(pre) {
  @apply my-4 overflow-x-auto rounded-lg bg-gray-950 p-3 text-xs text-gray-100;
}

.legal-document-content :deep(pre code) {
  @apply bg-transparent p-0 text-inherit;
}

.legal-document-content :deep(table) {
  @apply my-4 block w-full overflow-x-auto border-collapse text-sm;
}

.legal-document-content :deep(th) {
  @apply border border-gray-300 bg-gray-50 px-3 py-2 text-left font-semibold dark:border-dark-600 dark:bg-dark-800;
}

.legal-document-content :deep(td) {
  @apply border border-gray-300 px-3 py-2 dark:border-dark-600;
}

.legal-document-content :deep(img) {
  @apply my-4 h-auto max-w-full rounded-lg;
}

.legal-document-content :deep(hr) {
  @apply my-5 border-gray-200 dark:border-dark-700;
}
</style>
