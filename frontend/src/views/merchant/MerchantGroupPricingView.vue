<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- 标题 + 刷新 -->
      <div class="flex items-center justify-between gap-3">
        <div>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('merchant.owner.pricing.title') }}
          </h1>
          <p class="text-sm text-gray-500 dark:text-dark-400">
            {{ t('merchant.owner.pricing.description') }}
          </p>
        </div>
        <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="load">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>

      <!-- 分组定价列表（平铺所有可定价分组） -->
      <div class="card">
        <div class="border-b border-gray-200 px-4 py-3 dark:border-dark-700">
          <h2 class="text-sm font-semibold text-gray-700 dark:text-gray-200">
            {{ t('merchant.owner.pricing.groupsTitle') }}
          </h2>
          <p class="mt-0.5 text-xs text-gray-500">
            {{ t('merchant.owner.pricing.groupsHint') }}
          </p>
        </div>
        <DataTable :columns="columns" :data="groups" :loading="loading">
          <template #cell-name="{ row }">
            <div class="flex items-center gap-2">
              <span class="font-medium text-gray-900 dark:text-gray-100">{{ row.name }}</span>
              <span class="text-xs text-gray-400">#{{ row.id }}</span>
              <span
                :class="row.is_exclusive
                  ? 'rounded-full bg-amber-100 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
                  : 'rounded-full bg-emerald-100 px-1.5 py-0.5 text-[10px] font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'"
              >
                {{ row.is_exclusive ? t('merchant.owner.pricing.typeExclusive') : t('merchant.owner.pricing.typePublic') }}
              </span>
            </div>
          </template>
          <template #cell-rate_multiplier="{ value }">
            <span class="font-mono text-sm text-gray-600 dark:text-gray-300">{{ Number(value || 1).toFixed(4) }}x</span>
          </template>
          <template #cell-cost_rate="{ row }">
            <div class="flex items-center gap-2">
              <span class="font-mono text-sm text-gray-700 dark:text-gray-300">
                {{ effectiveCost(row).toFixed(4) }}x
              </span>
              <span v-if="row.cost_rate == null" class="text-[10px] text-gray-400">
                {{ t('merchant.owner.pricing.followsSiteRate') }}
              </span>
            </div>
          </template>
          <template #cell-sell_rate="{ row }">
            <div class="flex items-center gap-2">
              <span
                v-if="row.sell_rate != null"
                class="font-mono text-sm font-semibold"
                :class="effectiveSell(row) >= effectiveCost(row)
                  ? 'text-primary-600 dark:text-primary-400'
                  : 'text-rose-600 dark:text-rose-400'"
              >
                {{ effectiveSell(row).toFixed(4) }}x
              </span>
              <span v-else class="text-xs italic text-gray-400">
                {{ t('merchant.owner.pricing.notConfigured') }}
              </span>
            </div>
          </template>
          <template #cell-profit="{ row }">
            <span
              v-if="row.sell_rate != null"
              class="font-mono text-sm"
              :class="profitOf(row) > 0
                ? 'text-emerald-600 dark:text-emerald-400'
                : profitOf(row) < 0
                  ? 'text-rose-600 dark:text-rose-400'
                  : 'text-gray-500'"
            >
              {{ profitOf(row) >= 0 ? '+' : '' }}{{ profitOf(row).toFixed(4) }}x
            </span>
            <span v-else class="text-xs text-gray-300 dark:text-gray-600">—</span>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex gap-2">
              <button class="text-sm text-primary-600 hover:underline dark:text-primary-400" @click="openSellForm(row)">
                {{ row.sell_rate != null ? t('common.edit') : t('merchant.owner.pricing.startSelling') }}
              </button>
              <button
                v-if="row.sell_rate != null"
                class="text-sm text-rose-600 hover:underline dark:text-rose-400"
                @click="confirmStop(row)"
              >
                {{ t('merchant.owner.pricing.stopSelling') }}
              </button>
            </div>
          </template>
        </DataTable>
      </div>
    </div>

    <!-- 售价编辑弹框 -->
    <BaseDialog
      :show="sellDialog.show"
      :title="t('merchant.owner.pricing.editGroupTitle', { name: sellDialog.groupName })"
      width="normal"
      @close="sellDialog.show = false"
    >
      <form id="merchant-group-sell-form" class="space-y-4" @submit.prevent="submitSell">
        <div class="rounded-lg bg-gray-50 p-3 text-sm dark:bg-dark-800/40">
          <div class="flex justify-between text-gray-500 dark:text-gray-400">
            <span>{{ t('merchant.owner.pricing.siteRate') }}</span>
            <span class="font-mono">{{ sellDialog.siteRate.toFixed(4) }}x</span>
          </div>
          <div class="mt-1 flex justify-between text-gray-500 dark:text-gray-400">
            <span>{{ t('merchant.owner.pricing.myCost') }}</span>
            <span class="font-mono">{{ sellDialog.costRate.toFixed(4) }}x</span>
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('merchant.owner.pricing.sellRate') }}</label>
          <input
            v-model.number="sellDialog.sellRate"
            type="number"
            :min="sellDialog.costRate"
            step="0.0001"
            required
            class="input"
          />
          <p class="mt-1 text-xs text-gray-500">{{ t('merchant.owner.pricing.sellRateHint') }}</p>
          <div v-if="sellDialog.sellRate < sellDialog.costRate" class="mt-1 text-xs text-rose-600">
            {{ t('merchant.owner.pricing.sellBelowCostWarn', { cost: sellDialog.costRate.toFixed(4) }) }}
          </div>
          <div v-if="sellDialog.sellRate >= sellDialog.costRate" class="mt-1 text-xs text-emerald-600">
            {{ t('merchant.owner.pricing.profitPreview', { profit: (sellDialog.sellRate - sellDialog.costRate).toFixed(4) }) }}
          </div>
        </div>
        <div>
          <label class="input-label">
            {{ t('merchant.fields.reason') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('common.optional') }})</span>
          </label>
          <textarea v-model="sellDialog.reason" rows="2" class="input"></textarea>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="sellDialog.show = false">
            {{ t('common.cancel') }}
          </button>
          <button
            type="submit"
            form="merchant-group-sell-form"
            :disabled="sellDialog.submitting || sellDialog.sellRate < sellDialog.costRate"
            class="btn btn-primary"
          >
            {{ sellDialog.submitting ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { merchantAPI } from '@/api'
import type { MerchantPricingGroup } from '@/api/merchant'
import { extractI18nErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const groups = ref<MerchantPricingGroup[]>([])
const loading = ref(false)

function effectiveCost(row: MerchantPricingGroup): number {
  return row.cost_rate != null ? Number(row.cost_rate) : Number(row.rate_multiplier || 1)
}

function effectiveSell(row: MerchantPricingGroup): number {
  return row.sell_rate != null ? Number(row.sell_rate) : 0
}

function profitOf(row: MerchantPricingGroup): number {
  if (row.sell_rate == null) return 0
  return effectiveSell(row) - effectiveCost(row)
}

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('merchant.detail.groupPricing.group') },
  { key: 'rate_multiplier', label: t('merchant.owner.pricing.siteRate') },
  { key: 'cost_rate', label: t('merchant.owner.pricing.myCost') },
  { key: 'sell_rate', label: t('merchant.owner.pricing.mySell') },
  { key: 'profit', label: t('merchant.owner.pricing.profit') },
  { key: 'actions', label: t('common.actions') },
])

async function load(): Promise<void> {
  loading.value = true
  try {
    groups.value = await merchantAPI.listPricingGroups()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

// ============ 售价编辑 ============

const sellDialog = reactive({
  show: false,
  group_id: null as number | null,
  groupName: '',
  siteRate: 1,
  costRate: 1,
  sellRate: 1,
  reason: '',
  submitting: false,
})

function openSellForm(row: MerchantPricingGroup): void {
  sellDialog.group_id = row.id
  sellDialog.groupName = row.name
  sellDialog.siteRate = Number(row.rate_multiplier || 1)
  sellDialog.costRate = effectiveCost(row)
  // 已有 sell_rate 用它，没有的话以拿货价作为起点（商户不亏不赚）
  sellDialog.sellRate = row.sell_rate != null ? Number(row.sell_rate) : sellDialog.costRate
  sellDialog.reason = ''
  sellDialog.submitting = false
  sellDialog.show = true
}

async function submitSell(): Promise<void> {
  if (sellDialog.group_id == null) return
  sellDialog.submitting = true
  try {
    await merchantAPI.setGroupMarkup(
      sellDialog.group_id,
      sellDialog.sellRate,
      sellDialog.reason || undefined,
    )
    appStore.showSuccess(t('common.saved'))
    sellDialog.show = false
    await load()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    sellDialog.submitting = false
  }
}

async function confirmStop(row: MerchantPricingGroup): Promise<void> {
  if (!window.confirm(t('merchant.owner.pricing.confirmStop', { name: row.name }))) return
  try {
    await merchantAPI.deleteGroupMarkup(row.id)
    appStore.showSuccess(t('common.deleted'))
    await load()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  }
}

onMounted(() => {
  void load()
})
</script>
