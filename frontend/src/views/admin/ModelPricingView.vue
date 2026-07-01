<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex items-center justify-center py-16">
        <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
      </div>

      <template v-else>
        <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('admin.modelPricing.title') }}</h1>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.modelPricing.description') }}</p>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button type="button" class="btn btn-secondary inline-flex items-center gap-2" :disabled="refreshing" @click="openRefresh">
              <Icon name="refresh" size="sm" :class="refreshing ? 'animate-spin' : ''" />
              {{ refreshing ? t('admin.modelPricing.refreshing') : t('admin.modelPricing.refresh') }}
            </button>
            <button type="button" class="btn btn-secondary inline-flex items-center gap-2" @click="addRow">
              <Icon name="plus" size="sm" />
              {{ t('admin.modelPricing.addRow') }}
            </button>
            <button type="button" class="btn btn-primary inline-flex items-center gap-2" :disabled="saving" @click="handleSave">
              <Icon name="check" size="sm" />
              {{ t('admin.modelPricing.save') }}
            </button>
          </div>
        </div>

        <!-- 价格覆盖(可编辑) -->
        <div>
          <h2 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">{{ t('admin.modelPricing.overridesTitle') }}</h2>
          <div class="overflow-x-auto rounded-lg border border-gray-100 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800">
            <table class="min-w-full text-sm">
              <thead>
                <tr class="border-b border-gray-100 text-left text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400">
                  <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.columns.model') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.columns.currency') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.columns.input') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.columns.output') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.columns.cache') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.columns.hasCache') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.columns.enabled') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.columns.actions') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(row, i) in rows" :key="i" class="border-b border-gray-50 dark:border-dark-700/60">
                  <td class="px-3 py-2">
                    <input v-model="row.model" type="text" class="input w-48" placeholder="qwen-plus" />
                  </td>
                  <td class="px-3 py-2">
                    <Select v-model="row.currency" :options="currencyOptions" class="w-28" />
                  </td>
                  <td class="px-3 py-2">
                    <input v-model.number="row.input" type="number" min="0" step="0.01" class="input w-28" />
                  </td>
                  <td class="px-3 py-2">
                    <input v-model.number="row.output" type="number" min="0" step="0.01" class="input w-28" />
                  </td>
                  <td class="px-3 py-2">
                    <input v-model.number="row.cache" type="number" min="0" step="0.01" class="input w-28" :disabled="!row.has_cache" />
                  </td>
                  <td class="px-3 py-2">
                    <Toggle v-model="row.has_cache" />
                  </td>
                  <td class="px-3 py-2">
                    <Toggle v-model="row.enabled" />
                  </td>
                  <td class="px-3 py-2">
                    <button type="button" class="text-red-500 hover:text-red-600" :title="t('admin.modelPricing.columns.actions')" @click="removeRow(i)">
                      <Icon name="trash" size="sm" />
                    </button>
                  </td>
                </tr>
                <tr v-if="rows.length === 0">
                  <td colspan="8" class="px-3 py-10 text-center text-sm text-gray-400">{{ t('admin.modelPricing.empty') }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <p class="mt-2 text-xs text-gray-400">{{ t('admin.modelPricing.hint') }}</p>
        </div>

        <!-- 内置默认价(只读) -->
        <div>
          <div class="mb-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <h2 class="text-sm font-semibold text-gray-700 dark:text-gray-200">
              {{ t('admin.modelPricing.builtinTitle') }}
              <span class="ml-1 text-xs font-normal text-gray-400">({{ builtin.length }})</span>
            </h2>
            <div class="relative w-full sm:w-72">
              <Icon name="search" size="sm" class="pointer-events-none absolute left-2 top-1/2 -translate-y-1/2 text-gray-400" />
              <input v-model="builtinSearch" type="text" class="input w-full pl-8" :placeholder="t('admin.modelPricing.searchPlaceholder')" />
            </div>
          </div>
          <p class="mb-2 text-xs text-gray-400">{{ t('admin.modelPricing.builtinHint') }}</p>

          <!-- 平台分类筛选 -->
          <div class="mb-2 flex flex-wrap gap-1.5">
            <button type="button" :class="chipClass('all')" @click="activeCategory = 'all'">
              {{ t('admin.modelPricing.categoryAll') }} ({{ searchedBuiltin.length }})
            </button>
            <button v-for="g in groupedBuiltin" :key="g.key" type="button" :class="chipClass(g.key)" @click="activeCategory = g.key">
              {{ g.label }} ({{ g.items.length }})
            </button>
          </div>

          <div class="overflow-x-auto rounded-lg border border-gray-100 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800">
            <table class="min-w-full text-sm">
              <thead>
                <tr class="border-b border-gray-100 text-left text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400">
                  <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.columns.model') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.source') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.columns.currency') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.columns.input') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.columns.output') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.columns.cache') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.columns.actions') }}</th>
                </tr>
              </thead>
              <tbody v-for="g in visibleGroups" :key="g.key">
                <tr class="select-none border-b border-gray-100 bg-gray-50 dark:border-dark-700 dark:bg-dark-800/60">
                  <td colspan="7" class="p-0">
                    <button
                      type="button"
                      class="flex w-full items-center gap-1 px-3 py-1.5 text-xs font-semibold text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700/60"
                      :aria-expanded="isExpanded(g.key)"
                      @click="toggleGroup(g.key)"
                    >
                      <Icon :name="isExpanded(g.key) ? 'chevronDown' : 'chevronRight'" size="xs" />
                      {{ g.label }}
                      <span class="font-normal text-gray-400">({{ g.items.length }})</span>
                    </button>
                  </td>
                </tr>
                <template v-if="isExpanded(g.key)">
                  <tr v-for="entry in g.items" :key="entry.model" class="border-b border-gray-50 dark:border-dark-700/60">
                    <td class="px-3 py-2 font-mono text-xs">{{ entry.model }}</td>
                    <td class="px-3 py-2">
                      <span class="rounded bg-gray-100 px-1.5 py-0.5 text-[11px] text-gray-500 dark:bg-dark-700 dark:text-gray-400">{{ entry.source }}</span>
                    </td>
                    <td class="px-3 py-2">{{ entry.currency }}</td>
                    <td class="px-3 py-2">{{ fmtPrice(entry.input) }}</td>
                    <td class="px-3 py-2">{{ fmtPrice(entry.output) }}</td>
                    <td class="px-3 py-2">{{ entry.has_cache ? fmtPrice(entry.cache) : '—' }}</td>
                    <td class="px-3 py-2">
                      <span v-if="isOverridden(entry.model)" class="text-xs text-emerald-500">{{ t('admin.modelPricing.overridden') }}</span>
                      <button v-else type="button" class="text-xs text-primary-600 hover:text-primary-700" @click="overrideBuiltin(entry)">
                        {{ t('admin.modelPricing.override') }}
                      </button>
                    </td>
                  </tr>
                </template>
              </tbody>
              <tbody v-if="visibleGroups.length === 0">
                <tr>
                  <td colspan="7" class="px-3 py-10 text-center text-sm text-gray-400">{{ t('admin.modelPricing.builtinEmpty') }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </template>
    </div>

    <!-- 一键拉价 diff 预览 -->
    <BaseDialog :show="showPreview" :title="t('admin.modelPricing.preview.title')" width="wide" @close="showPreview = false">
      <div v-if="preview" class="space-y-4">
        <p class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.modelPricing.preview.summary', { remote: preview.remote_count, current: preview.current_count }) }}
        </p>
        <div class="flex flex-wrap gap-3 text-sm">
          <span class="rounded bg-emerald-50 px-2 py-1 text-emerald-600 dark:bg-emerald-900/30">{{ t('admin.modelPricing.preview.added') }}: {{ preview.added }}</span>
          <span class="rounded bg-amber-50 px-2 py-1 text-amber-600 dark:bg-amber-900/30">{{ t('admin.modelPricing.preview.changed') }}: {{ preview.changed }}</span>
          <span class="rounded bg-red-50 px-2 py-1 text-red-600 dark:bg-red-900/30">{{ t('admin.modelPricing.preview.removed') }}: {{ preview.removed }}</span>
        </div>
        <p v-if="preview.added + preview.changed + preview.removed === 0" class="py-6 text-center text-sm text-gray-400">
          {{ t('admin.modelPricing.preview.noChanges') }}
        </p>
        <div v-else class="max-h-80 overflow-y-auto rounded-lg border border-gray-100 dark:border-dark-700">
          <table class="min-w-full text-xs">
            <thead class="sticky top-0 bg-gray-50 dark:bg-dark-800">
              <tr class="text-left text-gray-500 dark:text-gray-400">
                <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.preview.colModel') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.preview.colKind') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.preview.colInput') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.preview.colOutput') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('admin.modelPricing.preview.colCache') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(ch, i) in preview.changes" :key="i" class="border-b border-gray-50 dark:border-dark-700/60">
                <td class="px-3 py-1.5 font-mono">{{ ch.model }}</td>
                <td class="px-3 py-1.5">
                  <span :class="kindClass(ch.kind)">{{ kindLabel(ch.kind) }}</span>
                </td>
                <td class="px-3 py-1.5">{{ diffCell(ch.kind, ch.old_input, ch.new_input) }}</td>
                <td class="px-3 py-1.5">{{ diffCell(ch.kind, ch.old_output, ch.new_output) }}</td>
                <td class="px-3 py-1.5">{{ diffCell(ch.kind, ch.old_cache, ch.new_cache) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-if="preview.truncated" class="text-xs text-amber-500">{{ t('admin.modelPricing.preview.truncated', { n: preview.changes.length }) }}</p>
      </div>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="showPreview = false">{{ t('admin.modelPricing.preview.cancel') }}</button>
        <button type="button" class="btn btn-primary" :disabled="applying || !preview" @click="applyRefresh">
          {{ applying ? t('admin.modelPricing.preview.applying') : t('admin.modelPricing.preview.apply') }}
        </button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminAPI } from '@/api/admin'
import type { ModelPricingEntry, BuiltinPricingEntry, PricingRefreshPreview } from '@/api/admin'
import type { SelectOption } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const saving = ref(false)
const rows = ref<ModelPricingEntry[]>([])
const builtin = ref<BuiltinPricingEntry[]>([])
const builtinSearch = ref('')
const activeCategory = ref('all')
// 默认折叠体量大的西方平台与「其他」,初始视图聚焦国产 + Claude(国产是本项目重点)。
const collapsed = ref<Set<string>>(new Set(['gpt', 'gemini', 'other']))

const refreshing = ref(false)
const applying = ref(false)
const showPreview = ref(false)
const preview = ref<PricingRefreshPreview | null>(null)

const currencyOptions: SelectOption[] = [
  { value: 'CNY', label: '¥ CNY' },
  { value: 'USD', label: '$ USD' }
]

// 平台分类(按品牌归类内置模型,顺序即展示顺序)。
const CATEGORIES: { key: string; label: string }[] = [
  { key: 'claude', label: 'Claude' },
  { key: 'gpt', label: 'GPT · OpenAI' },
  { key: 'gemini', label: 'Gemini' },
  { key: 'deepseek', label: 'DeepSeek' },
  { key: 'qwen', label: 'Qwen 通义' },
  { key: 'glm', label: 'GLM 智谱' },
  { key: 'kimi', label: 'Kimi · Moonshot' }
]

function categoryOf(e: BuiltinPricingEntry): string {
  const m = e.model.toLowerCase()
  const s = (e.source || '').toLowerCase()
  if (m.includes('claude') || s.includes('anthropic')) return 'claude'
  if (m.includes('gemini') || m.includes('gemma') || s.includes('vertex') || s.includes('google')) return 'gemini'
  if (m.includes('deepseek') || s.includes('deepseek')) return 'deepseek'
  if (m.includes('qwen') || m.startsWith('qwq') || m.startsWith('qvq') || s.includes('qwen') || s.includes('dashscope')) return 'qwen'
  if (m.includes('glm') || s.includes('zhipu') || s.includes('z-ai') || s.includes('bigmodel')) return 'glm'
  if (m.includes('kimi') || m.includes('moonshot') || s.includes('moonshot')) return 'kimi'
  if (/^(gpt|o1|o3|o4|chatgpt|codex)/.test(m) || m.includes('gpt') || s.includes('openai')) return 'gpt'
  return 'other'
}

function categoryLabel(key: string): string {
  return CATEGORIES.find((c) => c.key === key)?.label ?? t('admin.modelPricing.categoryOther')
}

// 文本搜索过滤(模型名 / 来源)。
const searchedBuiltin = computed(() => {
  const q = builtinSearch.value.trim().toLowerCase()
  if (!q) return builtin.value
  return builtin.value.filter((e) => e.model.toLowerCase().includes(q) || e.source.toLowerCase().includes(q))
})

// 按平台分组(仅保留非空组,品牌组在前、「其他」垫底)。
const groupedBuiltin = computed(() => {
  const map: Record<string, BuiltinPricingEntry[]> = {}
  for (const e of searchedBuiltin.value) {
    const k = categoryOf(e)
    ;(map[k] ||= []).push(e)
  }
  const ordered = [...CATEGORIES.map((c) => c.key), 'other']
  return ordered
    .filter((k) => (map[k]?.length ?? 0) > 0)
    .map((k) => ({ key: k, label: categoryLabel(k), items: map[k] }))
})

const visibleGroups = computed(() =>
  activeCategory.value === 'all'
    ? groupedBuiltin.value
    : groupedBuiltin.value.filter((g) => g.key === activeCategory.value)
)

function toggleGroup(key: string) {
  const next = new Set(collapsed.value)
  next.has(key) ? next.delete(key) : next.add(key)
  collapsed.value = next
}

// 选中具体分类、或正在搜索时一律展开,便于直接看到结果。
function isExpanded(key: string): boolean {
  if (activeCategory.value !== 'all') return true
  if (builtinSearch.value.trim()) return true
  return !collapsed.value.has(key)
}

function chipClass(key: string): string {
  const base = 'rounded-full px-3 py-1 text-xs transition-colors'
  return activeCategory.value === key
    ? `${base} bg-primary-600 text-white`
    : `${base} bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600`
}

// 搜索变化时回到「全部」,避免停留在搜索后已为空的分类上。
watch(builtinSearch, () => {
  activeCategory.value = 'all'
})

// 与后端 matchOverride 同口径(精确或最长前缀):标记某内置项是否已被启用的覆盖盖住。
function isOverridden(model: string): boolean {
  const m = model.toLowerCase()
  return rows.value.some((r) => {
    if (!r.enabled) return false
    const om = (r.model || '').trim().toLowerCase()
    return om !== '' && (om === m || m.startsWith(om))
  })
}

function fmtPrice(n: number): string {
  if (n === null || n === undefined || Number.isNaN(n)) return '—'
  if (n === 0) return '0'
  return Number(n.toFixed(4)).toString()
}

function kindLabel(kind: string): string {
  if (kind === 'added') return t('admin.modelPricing.preview.kindAdded')
  if (kind === 'removed') return t('admin.modelPricing.preview.kindRemoved')
  return t('admin.modelPricing.preview.kindChanged')
}
function kindClass(kind: string): string {
  if (kind === 'added') return 'text-emerald-500'
  if (kind === 'removed') return 'text-red-500'
  return 'text-amber-500'
}
function diffCell(kind: string, oldV: number, newV: number): string {
  if (kind === 'added') return fmtPrice(newV)
  if (kind === 'removed') return fmtPrice(oldV)
  return `${fmtPrice(oldV)} → ${fmtPrice(newV)}`
}

async function loadConfig() {
  loading.value = true
  try {
    const cfg = await adminAPI.modelPricing.getModelPricing()
    rows.value = cfg.entries ?? []
    builtin.value = cfg.builtin ?? []
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.modelPricing.loadFailed')))
  } finally {
    loading.value = false
  }
}

function addRow() {
  rows.value.push({ model: '', currency: 'CNY', input: 0, output: 0, cache: 0, has_cache: false, enabled: true })
}

function removeRow(i: number) {
  rows.value.splice(i, 1)
}

// 基于一条内置默认价新建可编辑覆盖行(预填值)。
function overrideBuiltin(entry: BuiltinPricingEntry) {
  rows.value.unshift({
    model: entry.model,
    currency: entry.currency,
    input: entry.input,
    output: entry.output,
    cache: entry.cache,
    has_cache: entry.has_cache,
    enabled: true
  })
  appStore.showInfo(t('admin.modelPricing.overrideAdded', { model: entry.model }))
}

async function handleSave() {
  saving.value = true
  try {
    // 价格框清空时 v-model.number 会产出空字符串，需转数兜底，否则后端 float 解析返回 400。
    const entries = rows.value.map((r) => ({
      ...r,
      input: Number(r.input) || 0,
      output: Number(r.output) || 0,
      cache: Number(r.cache) || 0
    }))
    const cfg = await adminAPI.modelPricing.updateModelPricing({ entries })
    rows.value = cfg.entries ?? []
    appStore.showSuccess(t('admin.modelPricing.saveSuccess'))
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.modelPricing.saveFailed')))
  } finally {
    saving.value = false
  }
}

async function openRefresh() {
  refreshing.value = true
  try {
    preview.value = await adminAPI.modelPricing.refreshPreview()
    showPreview.value = true
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.modelPricing.refreshFailed')))
  } finally {
    refreshing.value = false
  }
}

async function applyRefresh() {
  applying.value = true
  try {
    await adminAPI.modelPricing.refreshApply()
    showPreview.value = false
    appStore.showSuccess(t('admin.modelPricing.applySuccess'))
    await loadConfig()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.modelPricing.applyFailed')))
  } finally {
    applying.value = false
  }
}

onMounted(loadConfig)
</script>
