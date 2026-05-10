<template>
  <AppLayout>
    <div class="mx-auto max-w-3xl space-y-6">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
          {{ t('merchant.owner.subUsers.title') }}
        </h1>
        <p class="text-sm text-gray-500 dark:text-dark-400">
          {{ t('merchant.owner.subUsers.description') }}
        </p>
      </div>

      <form class="card space-y-4 p-6" @submit.prevent="submit">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('merchant.owner.subUsers.payTitle') }}
        </h2>
        <div class="grid gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">
              {{ t('merchant.owner.subUsers.subUserId') }}
              <span class="text-rose-500">*</span>
            </label>
            <input
              v-model.number="form.sub_user_id"
              type="number"
              min="1"
              required
              class="input"
              :placeholder="t('merchant.owner.subUsers.subUserIdPlaceholder')"
            />
          </div>
          <div>
            <label class="input-label">
              {{ t('merchant.fields.amount') }}
              <span class="text-rose-500">*</span>
            </label>
            <input
              v-model.number="form.amount"
              type="number"
              step="0.01"
              min="0.01"
              required
              class="input"
            />
          </div>
        </div>
        <div>
          <label class="input-label">
            {{ t('merchant.fields.reason') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('common.optional') }})</span>
          </label>
          <textarea v-model="form.reason" rows="2" class="input"></textarea>
        </div>
        <div class="flex justify-end">
          <button type="submit" :disabled="submitting" class="btn btn-primary">
            {{ submitting ? t('common.processing') : t('merchant.owner.subUsers.payButton') }}
          </button>
        </div>
      </form>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'
import { merchantAPI } from '@/api'
import { extractI18nErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const form = reactive({
  sub_user_id: 0,
  amount: 0,
  reason: '',
})

const submitting = ref(false)

async function submit(): Promise<void> {
  if (!form.sub_user_id || form.sub_user_id <= 0) {
    appStore.showError(t('merchant.errors.subUserIdRequired'))
    return
  }
  if (!form.amount || form.amount <= 0) {
    appStore.showError(t('merchant.errors.invalidAmount'))
    return
  }
  submitting.value = true
  try {
    await merchantAPI.payToUser(form.sub_user_id, form.amount, form.reason || undefined)
    appStore.showSuccess(t('merchant.owner.subUsers.paySuccess'))
    form.amount = 0
    form.reason = ''
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    submitting.value = false
  }
}
</script>
