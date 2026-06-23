<template>
  <div ref="rootRef" class="relative">
    <input
      ref="inputRef"
      v-model="display"
      type="text"
      autocomplete="off"
      class="input w-full"
      :placeholder="placeholder || t('common.searchMerchantPlaceholder')"
      :disabled="disabled"
      @input="onInput"
      @focus="onFocus"
    />
    <button
      v-if="selectedMerchant && !disabled"
      type="button"
      class="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
      :title="t('common.clear')"
      @click="clear"
    >
      <Icon name="x" size="sm" />
    </button>
    <div
      v-if="showDropdown && (loading || results.length > 0 || query.trim().length > 0)"
      class="absolute left-0 right-0 top-full z-20 mt-1 max-h-64 overflow-y-auto rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-500 dark:bg-dark-700"
    >
      <div v-if="loading" class="px-3 py-2 text-sm text-gray-400">{{ t('common.loading') }}</div>
      <template v-else>
        <button
          v-for="m in results"
          :key="m.id"
          type="button"
          class="flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm hover:bg-gray-50 dark:hover:bg-dark-600"
          @click="select(m)"
        >
          <span class="text-gray-400">#{{ m.id }}</span>
          <span class="text-gray-900 dark:text-white">{{ m.name }}</span>
        </button>
        <div v-if="results.length === 0 && query.trim().length > 0" class="px-3 py-2 text-sm text-gray-400">
          {{ t('common.noData') }}
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { merchantAPI, type Merchant } from '@/api/merchant'

const props = defineProps<{
  modelValue: number | null
  placeholder?: string
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: number | null): void
  (e: 'select', merchant: Merchant | null): void
}>()

const { t } = useI18n()
const rootRef = ref<HTMLElement | null>(null)
const inputRef = ref<HTMLInputElement | null>(null)
const display = ref('')
const query = ref('')
const results = ref<Merchant[]>([])
const selectedMerchant = ref<Merchant | null>(null)
const showDropdown = ref(false)
const loading = ref(false)
let timer: ReturnType<typeof setTimeout> | null = null

watch(
  () => props.modelValue,
  async (val) => {
    if (val === null || val === undefined) {
      display.value = ''
      query.value = ''
      selectedMerchant.value = null
      return
    }
    if (selectedMerchant.value && selectedMerchant.value.id === val) return
    try {
      const res = await merchantAPI.adminList(undefined, 0, 50)
      const m = res.items.find(x => x.id === val) || null
      if (m) {
        selectedMerchant.value = m
        display.value = `#${m.id} ${m.name}`
      } else {
        display.value = `#${val}`
      }
    } catch {
      display.value = `#${val}`
    }
  },
  { immediate: true },
)

function onFocus() {
  if (results.value.length > 0 || query.value.trim()) showDropdown.value = true
  // 首次 focus 时如果没结果，预拉前 20 条
  if (results.value.length === 0 && !query.value.trim() && !loading.value) {
    loading.value = true
    showDropdown.value = true
    merchantAPI
      .adminList(undefined, 0, 20)
      .then(res => { results.value = res.items })
      .catch(() => { results.value = [] })
      .finally(() => { loading.value = false })
  }
}

function onInput() {
  query.value = display.value
  if (timer) clearTimeout(timer)
  if (selectedMerchant.value) {
    selectedMerchant.value = null
    emit('update:modelValue', null)
    emit('select', null)
  }
  const q = query.value.trim()
  if (!q) {
    results.value = []
    showDropdown.value = false
    return
  }
  loading.value = true
  showDropdown.value = true
  timer = setTimeout(async () => {
    try {
      const res = await merchantAPI.adminList(undefined, 0, 10, q)
      results.value = res.items
    } catch {
      results.value = []
    } finally {
      loading.value = false
    }
  }, 300)
}

function select(m: Merchant) {
  selectedMerchant.value = m
  display.value = `#${m.id} ${m.name}`
  showDropdown.value = false
  results.value = []
  emit('update:modelValue', m.id)
  emit('select', m)
}

function clear() {
  display.value = ''
  query.value = ''
  selectedMerchant.value = null
  results.value = []
  showDropdown.value = false
  emit('update:modelValue', null)
  emit('select', null)
  inputRef.value?.focus()
}

function handleDocumentClick(e: MouseEvent) {
  if (rootRef.value && !rootRef.value.contains(e.target as Node)) {
    showDropdown.value = false
  }
}

onMounted(() => {
  document.addEventListener('mousedown', handleDocumentClick)
})
onBeforeUnmount(() => {
  document.removeEventListener('mousedown', handleDocumentClick)
  if (timer) clearTimeout(timer)
})
</script>
