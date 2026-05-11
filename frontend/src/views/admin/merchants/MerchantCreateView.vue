<template>
  <AppLayout>
    <div class="mx-auto max-w-3xl space-y-6">
      <div class="flex items-center gap-3">
        <button class="btn btn-secondary btn-sm" @click="goBack">
          <Icon name="chevronLeft" size="sm" class="mr-1" />
          {{ t('common.back') }}
        </button>
        <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
          {{ t('merchant.admin.createTitle') }}
        </h1>
      </div>

      <form class="card space-y-6 p-6" @submit.prevent="submit">
        <div class="grid gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">
              {{ t('merchant.fields.owner') }}
              <span class="text-rose-500">*</span>
            </label>
            <UserSelectRemote
              v-model="form.owner_user_id"
              :placeholder="t('merchant.admin.ownerUserIdPlaceholder')"
            />
            <p class="mt-1 text-xs text-gray-500">{{ t('merchant.admin.ownerUserIdHint') }}</p>
          </div>
          <div>
            <label class="input-label">
              {{ t('merchant.fields.name') }}
              <span class="text-rose-500">*</span>
            </label>
            <input
              v-model="form.name"
              type="text"
              required
              class="input"
              :placeholder="t('merchant.admin.namePlaceholder')"
            />
          </div>
        </div>

        <div class="grid gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('merchant.fields.discount') }}</label>
            <input
              v-model.number="form.discount"
              type="number"
              min="0"
              step="0.0001"
              class="input"
            />
            <p class="mt-1 text-xs text-gray-500">{{ t('merchant.admin.discountHint') }}</p>
            <div
              v-if="form.discount < 0.5"
              class="mt-1 text-xs text-rose-600"
            >
              {{ t('merchant.detail.warnings.discountLow') }}
            </div>
          </div>
          <div>
            <label class="input-label">{{ t('merchant.fields.markupDefault') }}</label>
            <input
              v-model.number="form.user_markup_default"
              type="number"
              min="1"
              step="0.0001"
              class="input"
            />
            <p class="mt-1 text-xs text-gray-500">{{ t('merchant.admin.markupHint') }}</p>
            <div
              v-if="form.user_markup_default > 2"
              class="mt-1 text-xs text-rose-600"
            >
              {{ t('merchant.detail.warnings.markupHigh') }}
            </div>
          </div>
        </div>

        <div class="grid gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('merchant.fields.lowBalanceThreshold') }}</label>
            <input
              v-model.number="form.low_balance_threshold"
              type="number"
              min="0"
              step="0.01"
              class="input"
            />
          </div>
          <div>
            <label class="input-label">
              {{ t('merchant.fields.notifyEmails') }}
              <span class="ml-1 text-xs font-normal text-gray-400">({{ t('merchant.admin.notifyEmailsHint') }})</span>
            </label>
            <input
              v-model="form.notify_emails_str"
              type="text"
              class="input"
              :placeholder="t('merchant.admin.notifyEmailsPlaceholder')"
            />
          </div>
        </div>

        <div>
          <label class="input-label">
            {{ t('merchant.fields.reason') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('common.optional') }})</span>
          </label>
          <textarea v-model="form.reason" rows="3" class="input"></textarea>
        </div>

        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="goBack">
            {{ t('common.cancel') }}
          </button>
          <button type="submit" :disabled="submitting" class="btn btn-primary">
            {{ submitting ? t('merchant.admin.creating') : t('common.create') }}
          </button>
        </div>
      </form>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import UserSelectRemote from '@/components/common/UserSelectRemote.vue'
import { useAppStore } from '@/stores/app'
import { merchantAPI, type CreateMerchantPayload } from '@/api'
import { extractI18nErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

const form = reactive<{
  owner_user_id: number | null
  name: string
  discount: number
  user_markup_default: number
  low_balance_threshold: number
  notify_emails_str: string
  reason: string
}>({
  owner_user_id: null,
  name: '',
  discount: 1,
  user_markup_default: 1,
  low_balance_threshold: 0,
  notify_emails_str: '',
  reason: '',
})

const submitting = ref(false)

async function submit(): Promise<void> {
  if (!form.owner_user_id || form.owner_user_id <= 0) {
    appStore.showError(t('merchant.errors.ownerUserIdRequired'))
    return
  }
  if (!form.name.trim()) {
    appStore.showError(t('merchant.errors.nameRequired'))
    return
  }
  submitting.value = true
  try {
    const emails = form.notify_emails_str
      .split(/[,;\s]+/)
      .map((s) => s.trim())
      .filter(Boolean)
    const payload: CreateMerchantPayload = {
      owner_user_id: form.owner_user_id,
      name: form.name.trim(),
      discount: form.discount,
      user_markup_default: form.user_markup_default,
      low_balance_threshold: form.low_balance_threshold,
      notify_emails: emails,
      reason: form.reason.trim() || undefined,
    }
    const created = await merchantAPI.adminCreate(payload)
    appStore.showSuccess(t('merchant.admin.toast.created'))
    router.replace(`/admin/merchants/${created.id}`)
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    submitting.value = false
  }
}

function goBack(): void {
  router.push('/admin/merchants')
}
</script>
