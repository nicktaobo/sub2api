<template>
  <div ref="rootRef" class="relative">
    <input
      ref="inputRef"
      v-model="display"
      type="text"
      autocomplete="off"
      class="input w-full"
      :placeholder="placeholder || t('common.searchUserPlaceholder')"
      :disabled="disabled"
      @input="onInput"
      @focus="onFocus"
    />
    <button
      v-if="selectedUser && !disabled"
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
          v-for="u in results"
          :key="u.id"
          type="button"
          class="flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm hover:bg-gray-50 dark:hover:bg-dark-600"
          @click="select(u)"
        >
          <span class="text-gray-400">#{{ u.id }}</span>
          <span class="text-gray-900 dark:text-white">{{ u.username || u.email }}</span>
          <span v-if="u.username" class="text-xs text-gray-400">{{ u.email }}</span>
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
import { adminAPI } from '@/api/admin'
import type { AdminUser } from '@/types'

const props = defineProps<{
  modelValue: number | null
  placeholder?: string
  disabled?: boolean
  // 可选过滤：只显示符合条件的用户（如 role='user'）
  role?: 'admin' | 'user'
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: number | null): void
  (e: 'select', user: AdminUser | null): void
}>()

const { t } = useI18n()
const rootRef = ref<HTMLElement | null>(null)
const inputRef = ref<HTMLInputElement | null>(null)
const display = ref('')
const query = ref('')
const results = ref<AdminUser[]>([])
const selectedUser = ref<AdminUser | null>(null)
const showDropdown = ref(false)
const loading = ref(false)
let timer: ReturnType<typeof setTimeout> | null = null

// 外部 modelValue 变化时反向同步显示（编辑场景预填）
watch(
  () => props.modelValue,
  async (val) => {
    if (val === null || val === undefined) {
      display.value = ''
      query.value = ''
      selectedUser.value = null
      return
    }
    if (selectedUser.value && selectedUser.value.id === val) return
    try {
      const res = await adminAPI.users.list(1, 1, { search: String(val), role: props.role })
      const u = res.items.find(x => x.id === val) || res.items[0] || null
      if (u && u.id === val) {
        selectedUser.value = u
        display.value = u.username ? `${u.username} (${u.email})` : u.email
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
}

function onInput() {
  query.value = display.value
  if (timer) clearTimeout(timer)
  // 用户改了输入意味着取消之前的选择
  if (selectedUser.value) {
    selectedUser.value = null
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
      const res = await adminAPI.users.list(1, 10, { search: q, role: props.role })
      results.value = res.items
    } catch {
      results.value = []
    } finally {
      loading.value = false
    }
  }, 300)
}

function select(u: AdminUser) {
  selectedUser.value = u
  display.value = u.username ? `${u.username} (${u.email})` : u.email
  showDropdown.value = false
  results.value = []
  emit('update:modelValue', u.id)
  emit('select', u)
}

function clear() {
  display.value = ''
  query.value = ''
  selectedUser.value = null
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
