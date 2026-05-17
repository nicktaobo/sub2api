<template>
  <Teleport to="body">
    <Transition name="agreement-fade">
      <div
        v-if="visible"
        class="fixed inset-0 z-[150] flex items-center justify-center overflow-y-auto bg-gray-950/60 p-4 backdrop-blur-sm"
      >
        <div class="w-full max-w-[600px] overflow-hidden rounded-2xl bg-white shadow-2xl ring-1 ring-black/10 dark:bg-dark-900 dark:ring-white/10">
          <div class="border-b border-gray-100 bg-white px-6 py-6 dark:border-dark-800 dark:bg-dark-900">
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

          <div class="max-h-[58vh] overflow-y-auto px-6 py-5">
            <p class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('auth.registerAgreement.relatedDocuments') }}
            </p>

            <div
              v-if="visibleDocuments.length > 0"
              class="grid grid-cols-1 gap-3 sm:grid-cols-2"
            >
              <RouterLink
                v-for="doc in visibleDocuments"
                :key="doc.id || doc.title"
                :to="documentRoute(doc)"
                target="_blank"
                rel="noopener noreferrer"
                class="group flex min-h-[72px] w-full items-center gap-3 rounded-xl border border-gray-200 bg-gray-50/70 px-4 py-3 text-left transition hover:-translate-y-0.5 hover:border-primary-200 hover:bg-white hover:shadow-sm dark:border-dark-700 dark:bg-dark-800/70 dark:hover:border-primary-500/30 dark:hover:bg-dark-800"
              >
                <span class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-white text-gray-700 ring-1 ring-gray-200 transition group-hover:bg-primary-50 group-hover:text-primary-700 group-hover:ring-primary-100 dark:bg-dark-900 dark:text-dark-200 dark:ring-dark-700 dark:group-hover:bg-primary-500/10 dark:group-hover:text-primary-200 dark:group-hover:ring-primary-500/20">
                  <Icon :name="documentIcon(doc)" size="sm" />
                </span>
                <span class="min-w-0 flex-1">
                  <span class="block truncate text-sm font-semibold text-gray-950 dark:text-white">{{ resolveTitle(doc) }}</span>
                </span>
                <span class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full text-gray-400 transition group-hover:bg-primary-50 group-hover:text-primary-600 dark:group-hover:bg-primary-500/10 dark:group-hover:text-primary-300">
                  <Icon name="externalLink" size="sm" />
                </span>
              </RouterLink>
            </div>

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

function documentRoute(doc: LoginAgreementDocument) {
  return {
    name: 'LegalDocument',
    params: {
      documentId: doc.id || doc.title,
    },
  }
}

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
</style>
