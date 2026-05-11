<template>
  <td class="px-3 py-3 align-top">
    <template v-if="hasSite">
      <div class="font-semibold text-primary-600 dark:text-primary-300">
        <template v-if="currency === 'CNY'">
          <span class="text-[10px] text-primary-400 dark:text-primary-300/70">¥</span>{{ formatNumber(siteCNYPerM) }}
        </template>
        <template v-else>
          <span class="text-[10px] text-primary-400 dark:text-primary-300/70">$</span>{{ formatNumber(siteUSDPerM) }}
        </template>
        <span class="ml-0.5 text-[10px] font-normal text-gray-400">/M</span>
      </div>
      <div v-if="hasOfficial" class="mt-0.5 text-[11px] text-gray-400 line-through dark:text-gray-500">
        <span>$</span>{{ formatNumber(officialUSDPerM) }}
        <span class="text-[10px]">/M</span>
      </div>
    </template>
    <span v-else class="text-xs text-gray-300 dark:text-gray-600">-</span>
  </td>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { USD_TO_CNY } from '@/utils/pricing'

const props = defineProps<{
  /** 本站价（USD / per token）。已含渠道倍率前的基准价。 */
  site?: number | null
  /** 官方原厂价（USD / per token）。 */
  official?: number | null
  /** 渠道倍率：本站实际单价 = site × rate */
  rate: number
  /** 当前显示货币 */
  currency: 'CNY' | 'USD'
}>()

const hasSite = computed(() => typeof props.site === 'number' && (props.site ?? 0) >= 0)
const hasOfficial = computed(() => typeof props.official === 'number' && (props.official ?? 0) > 0)

// 单位：USD / 1M tokens
const siteUSDPerM = computed(() => (props.site ?? 0) * (props.rate || 1) * 1_000_000)
const officialUSDPerM = computed(() => (props.official ?? 0) * 1_000_000)
const siteCNYPerM = computed(() => siteUSDPerM.value * USD_TO_CNY)

// 数字格式化：去掉无意义尾零，保留至多 4 位小数；> 1 时通常 2 位即可。
function formatNumber(n: number): string {
  if (!isFinite(n)) return '-'
  if (n === 0) return '0'
  const digits = n >= 100 ? 0 : n >= 10 ? 2 : 4
  const fixed = n.toFixed(digits)
  return fixed.replace(/\.?0+$/, '') || '0'
}
</script>
