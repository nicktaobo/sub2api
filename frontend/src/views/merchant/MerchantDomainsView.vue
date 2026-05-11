<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex items-center justify-between gap-3">
        <div>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('merchant.owner.domains.title') }}
          </h1>
          <p class="text-sm text-gray-500 dark:text-dark-400">
            {{ t('merchant.owner.domains.description') }}
          </p>
        </div>
        <div class="flex items-center gap-2">
          <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="load">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
          <button class="btn btn-primary" @click="openCreate">
            {{ t('merchant.owner.domains.addDomain') }}
          </button>
        </div>
      </div>

      <!-- 列表 -->
      <div class="card overflow-hidden">
        <table class="w-full text-sm">
          <thead class="bg-gray-50 dark:bg-dark-800">
            <tr>
              <th class="px-4 py-3 text-left">{{ t('merchant.owner.domains.domain') }}</th>
              <th class="px-4 py-3 text-left">{{ t('merchant.owner.domains.siteName') }}</th>
              <th class="px-4 py-3 text-left">{{ t('merchant.owner.domains.verifyStatus') }}</th>
              <th class="px-4 py-3 text-left">{{ t('merchant.owner.domains.brandColor') }}</th>
              <th class="px-4 py-3 text-left">{{ t('merchant.owner.domains.createdAt') }}</th>
              <th class="px-4 py-3 text-right">{{ t('merchant.owner.domains.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading && !items.length">
              <td colspan="6" class="px-4 py-12 text-center text-gray-400">{{ t('common.loading') }}</td>
            </tr>
            <tr v-else-if="!items.length">
              <td colspan="6" class="px-4 py-12 text-center text-gray-400">{{ t('common.empty') }}</td>
            </tr>
            <tr v-for="d in items" :key="d.id" class="border-t border-gray-200 dark:border-dark-700">
              <td class="px-4 py-3 font-mono">{{ d.domain }}</td>
              <td class="px-4 py-3">{{ d.site_name || '-' }}</td>
              <td class="px-4 py-3">
                <span v-if="d.verified" class="text-emerald-600">{{ t('merchant.owner.domains.verified') }}</span>
                <span v-else class="text-amber-600">{{ t('merchant.owner.domains.unverified') }}</span>
              </td>
              <td class="px-4 py-3">
                <div v-if="d.brand_color" class="flex items-center gap-2">
                  <span class="inline-block h-4 w-4 rounded border border-gray-300" :style="{ backgroundColor: d.brand_color }"></span>
                  <code class="text-xs">{{ d.brand_color }}</code>
                </div>
                <span v-else class="text-gray-400">-</span>
              </td>
              <td class="px-4 py-3 text-gray-500">{{ d.created_at ? formatDateTime(d.created_at) : '-' }}</td>
              <td class="px-4 py-3 text-right space-x-2">
                <button class="btn btn-sm btn-secondary" @click="openEdit(d)">{{ t('common.edit') }}</button>
                <button v-if="!d.verified" class="btn btn-sm btn-primary" @click="onVerify(d)">{{ t('merchant.owner.domains.markVerified') }}</button>
                <button class="btn btn-sm btn-danger" @click="onDelete(d)">{{ t('common.delete') }}</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 添加/编辑对话框 -->
      <BaseDialog
        :show="dialog.open"
        :title="dialog.id ? t('merchant.owner.domains.editTitle') : t('merchant.owner.domains.addTitle')"
        width="wide"
        @close="dialog.open = false"
      >
        <div class="space-y-4">
          <div>
            <label class="label">{{ t('merchant.owner.domains.domain') }} <span class="text-red-500">*</span></label>
            <input
              v-model="dialog.form.domain"
              type="text"
              class="input"
              :placeholder="t('merchant.owner.domains.domainPlaceholder')"
              :disabled="!!dialog.id"
            />
            <p v-if="!dialog.id" class="mt-1 text-xs text-gray-500">{{ t('merchant.owner.domains.domainHint') }}</p>
          </div>

          <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div>
              <label class="label">{{ t('merchant.owner.domains.siteName') }}</label>
              <input v-model="dialog.form.site_name" type="text" class="input" :placeholder="t('merchant.owner.domains.siteNamePlaceholder')" />
            </div>
            <div>
              <label class="label">{{ t('merchant.owner.domains.brandColor') }}</label>
              <div class="flex items-center gap-2">
                <input v-model="dialog.form.brand_color" type="color" class="h-10 w-16 cursor-pointer rounded border border-gray-300" />
                <input v-model="dialog.form.brand_color" type="text" class="input flex-1" placeholder="#3B82F6" />
              </div>
            </div>
          </div>

          <div>
            <label class="label">{{ t('merchant.owner.domains.siteLogo') }}</label>
            <input v-model="dialog.form.site_logo" type="text" class="input" :placeholder="t('merchant.owner.domains.siteLogoPlaceholder')" />
            <p class="mt-1 text-xs text-gray-500">{{ t('merchant.owner.domains.siteLogoHint') }}</p>
          </div>

          <div>
            <label class="label">{{ t('merchant.owner.domains.seoTitle') }}</label>
            <input v-model="dialog.form.seo_title" type="text" class="input" />
          </div>

          <div>
            <label class="label">{{ t('merchant.owner.domains.seoDescription') }}</label>
            <textarea v-model="dialog.form.seo_description" rows="2" class="input"></textarea>
          </div>

          <div>
            <label class="label">{{ t('merchant.owner.domains.seoKeywords') }}</label>
            <input v-model="dialog.form.seo_keywords" type="text" class="input" :placeholder="t('merchant.owner.domains.seoKeywordsPlaceholder')" />
          </div>

          <div>
            <label class="label">{{ t('merchant.owner.domains.customCss') }}</label>
            <textarea v-model="dialog.form.custom_css" rows="4" class="input font-mono text-xs" :placeholder="'/* CSS */'"></textarea>
          </div>

          <div>
            <label class="label">{{ t('merchant.owner.domains.homeContent') }}</label>
            <textarea v-model="dialog.form.home_content" rows="8" class="input font-mono text-xs" :placeholder="'<h1>欢迎</h1><p>自定义首页 HTML</p>'"></textarea>
            <p class="mt-1 text-xs text-amber-600">{{ t('merchant.owner.domains.homeContentHint') }}</p>
          </div>
        </div>

        <template #footer>
          <button class="btn btn-secondary" @click="dialog.open = false">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="submitting" @click="onSubmit">
            {{ submitting ? t('common.saving') : t('common.save') }}
          </button>
        </template>
      </BaseDialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { merchantAPI, type MerchantDomain, type DomainBrandPayload } from '@/api/merchant'
import { formatDateTime } from '@/utils/format'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const items = ref<MerchantDomain[]>([])
const loading = ref(false)
const submitting = ref(false)

const dialog = reactive({
  open: false,
  id: 0 as number,
  form: {
    domain: '',
    site_name: '',
    site_logo: '',
    brand_color: '#3B82F6',
    custom_css: '',
    home_content: '',
    seo_title: '',
    seo_description: '',
    seo_keywords: '',
  } as DomainBrandPayload,
})

function resetForm() {
  dialog.id = 0
  dialog.form = {
    domain: '',
    site_name: '',
    site_logo: '',
    brand_color: '#3B82F6',
    custom_css: '',
    home_content: '',
    seo_title: '',
    seo_description: '',
    seo_keywords: '',
  }
}

function openCreate() {
  resetForm()
  dialog.open = true
}

function openEdit(d: MerchantDomain) {
  dialog.id = d.id
  dialog.form = {
    domain: d.domain,
    site_name: d.site_name || '',
    site_logo: d.site_logo || '',
    brand_color: d.brand_color || '#3B82F6',
    custom_css: d.custom_css || '',
    home_content: d.home_content || '',
    seo_title: d.seo_title || '',
    seo_description: d.seo_description || '',
    seo_keywords: d.seo_keywords || '',
  }
  dialog.open = true
}

async function load() {
  loading.value = true
  try {
    items.value = await merchantAPI.listDomains()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

async function onSubmit() {
  submitting.value = true
  try {
    if (dialog.id) {
      const { domain: _unused, ...payload } = dialog.form
      void _unused
      await merchantAPI.updateDomain(dialog.id, payload)
      appStore.showSuccess(t('merchant.owner.domains.updated'))
    } else {
      await merchantAPI.createDomain(dialog.form)
      appStore.showSuccess(t('merchant.owner.domains.created'))
    }
    dialog.open = false
    await load()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    submitting.value = false
  }
}

async function onVerify(d: MerchantDomain) {
  try {
    await merchantAPI.verifyDomain(d.id)
    appStore.showSuccess(t('merchant.owner.domains.verifiedToast'))
    await load()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  }
}

async function onDelete(d: MerchantDomain) {
  if (!confirm(t('merchant.owner.domains.confirmDelete', { domain: d.domain }))) return
  try {
    await merchantAPI.deleteDomain(d.id)
    appStore.showSuccess(t('common.deleted'))
    await load()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  }
}

onMounted(load)
</script>
