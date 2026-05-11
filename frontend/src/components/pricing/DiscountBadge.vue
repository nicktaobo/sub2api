<template>
  <span
    v-if="discountPct !== null"
    class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-semibold"
    :class="toneClass"
  >
    {{ label }}
  </span>
  <span v-else class="text-xs text-gray-300 dark:text-gray-600">-</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  /** 本站单价（USD / per token），基准价（未乘倍率）。 */
  site?: number | null
  /** 官方原厂单价（USD / per token）。 */
  official?: number | null
  /** 渠道倍率：实际本站价 = site × rate */
  rate: number
}>()

const { t } = useI18n()

// 返回 +N 表示用户便宜（折扣），-N 表示用户更贵（溢价）。null 表示无法计算。
const discountPct = computed<number | null>(() => {
  if (typeof props.site !== 'number' || typeof props.official !== 'number') return null
  if ((props.official ?? 0) <= 0) return null
  const effectiveSite = (props.site ?? 0) * (props.rate || 1)
  if (effectiveSite < 0) return null
  return Math.round((1 - effectiveSite / props.official) * 100)
})

const label = computed(() => {
  const p = discountPct.value
  if (p === null) return '-'
  if (p === 0) return t('modelPricing.discount.equal')
  if (p > 0) return t('modelPricing.discount.save', { pct: p })
  return t('modelPricing.discount.markup', { pct: -p })
})

const toneClass = computed(() => {
  const p = discountPct.value
  if (p === null) return 'bg-gray-100 text-gray-500 dark:bg-dark-700/50 dark:text-gray-400'
  if (p >= 50) return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (p > 0) return 'bg-emerald-50 text-emerald-600 dark:bg-emerald-900/20 dark:text-emerald-300'
  if (p === 0) return 'bg-gray-100 text-gray-500 dark:bg-dark-700/50 dark:text-gray-400'
  return 'bg-rose-100 text-rose-600 dark:bg-rose-900/30 dark:text-rose-300'
})
</script>
