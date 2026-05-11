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
          <button class="btn btn-primary" @click="openCreate">+ {{ t('merchant.owner.domains.addDomain') }}</button>
        </div>
      </div>

      <!-- 空态 -->
      <div v-if="!loading && !items.length" class="card flex flex-col items-center gap-3 py-12">
        <Icon name="globe" size="xl" class="text-gray-300" />
        <p class="text-sm text-gray-500">{{ t('merchant.owner.domains.empty') }}</p>
        <button class="btn btn-primary" @click="openCreate">+ {{ t('merchant.owner.domains.addDomain') }}</button>
      </div>

      <!-- 域名卡片列表 -->
      <div v-for="d in items" :key="d.id" class="card overflow-hidden">
        <!-- 卡片头部 -->
        <div class="flex items-center justify-between border-b border-gray-200 px-5 py-4 dark:border-dark-700">
          <div class="flex items-center gap-3">
            <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-amber-100 dark:bg-amber-900/20">
              <Icon :name="d.verified ? 'checkCircle' : 'clock'" size="md" :class="d.verified ? 'text-emerald-600' : 'text-amber-600'" />
            </div>
            <div>
              <div class="text-base font-semibold">{{ d.domain }}</div>
              <span
                class="mt-0.5 inline-block rounded px-2 py-0.5 text-xs"
                :class="d.verified ? 'bg-emerald-100 text-emerald-700' : 'bg-amber-100 text-amber-700'"
              >
                {{ d.verified ? t('merchant.owner.domains.verified') : t('merchant.owner.domains.unverified') }}
              </span>
            </div>
          </div>
          <div class="flex items-center gap-2">
            <button
              v-if="!d.verified"
              class="btn btn-sm btn-primary"
              :disabled="verifying === d.id"
              @click="onVerify(d)"
            >
              <Icon name="check" size="sm" class="mr-1" />
              {{ verifying === d.id ? t('common.loading') : t('merchant.owner.domains.verifyNow') }}
            </button>
            <button class="btn btn-sm btn-secondary" @click="openEdit(d)" :title="t('common.edit')">
              <Icon name="edit" size="sm" />
            </button>
            <button class="btn btn-sm btn-danger" @click="onDelete(d)" :title="t('common.delete')">
              <Icon name="trash" size="sm" />
            </button>
          </div>
        </div>

        <!-- 未验证：DNS 步骤指引 -->
        <div v-if="!d.verified" class="space-y-4 px-5 py-5">
          <p class="text-sm font-medium text-gray-700 dark:text-gray-200">
            {{ t('merchant.owner.domains.dnsInstructions') }}
          </p>

          <!-- 步骤 1：A 记录 -->
          <div>
            <p class="mb-2 text-sm font-medium text-amber-700 dark:text-amber-400">
              {{ t('merchant.owner.domains.step1Title') }}
            </p>
            <div class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700">
              <table class="w-full text-xs">
                <thead class="bg-gray-50 dark:bg-dark-800">
                  <tr>
                    <th class="w-20 px-3 py-2 text-left text-gray-500">{{ t('merchant.owner.domains.dnsType') }}</th>
                    <th class="px-3 py-2 text-left text-gray-500">{{ t('merchant.owner.domains.dnsHost') }}</th>
                    <th class="px-3 py-2 text-left text-gray-500">{{ t('merchant.owner.domains.dnsValue') }}</th>
                    <th class="w-12"></th>
                  </tr>
                </thead>
                <tbody>
                  <tr class="border-t border-gray-200 dark:border-dark-700">
                    <td class="px-3 py-2 font-mono">A</td>
                    <td class="px-3 py-2 font-mono">@</td>
                    <td class="px-3 py-2 font-mono">
                      <span v-if="dnsInfo?.has_server_ip">{{ dnsInfo.server_ip }}</span>
                      <span v-else class="text-amber-600">{{ t('merchant.owner.domains.serverIpMissing') }}</span>
                    </td>
                    <td class="px-3 py-2 text-right">
                      <button
                        v-if="dnsInfo?.has_server_ip"
                        class="text-gray-400 hover:text-gray-600"
                        :title="t('common.copy')"
                        @click="copy(dnsInfo!.server_ip)"
                      >
                        <Icon name="copy" size="sm" />
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <p class="mt-1 text-xs text-gray-500">{{ t('merchant.owner.domains.step1Hint') }}</p>
          </div>

          <!-- 步骤 2：TXT 记录 -->
          <div>
            <p class="mb-2 text-sm font-medium text-amber-700 dark:text-amber-400">
              {{ t('merchant.owner.domains.step2Title') }}
            </p>
            <div class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700">
              <table class="w-full text-xs">
                <thead class="bg-gray-50 dark:bg-dark-800">
                  <tr>
                    <th class="w-20 px-3 py-2 text-left text-gray-500">{{ t('merchant.owner.domains.dnsType') }}</th>
                    <th class="px-3 py-2 text-left text-gray-500">{{ t('merchant.owner.domains.dnsHost') }}</th>
                    <th class="px-3 py-2 text-left text-gray-500">{{ t('merchant.owner.domains.dnsValue') }}</th>
                    <th class="w-12"></th>
                  </tr>
                </thead>
                <tbody>
                  <tr class="border-t border-gray-200 dark:border-dark-700">
                    <td class="px-3 py-2 font-mono">TXT</td>
                    <td class="px-3 py-2 font-mono break-all">{{ txtHost(d) }}</td>
                    <td class="px-3 py-2 font-mono break-all">{{ txtValue(d) }}</td>
                    <td class="px-3 py-2 text-right">
                      <button class="text-gray-400 hover:text-gray-600" :title="t('common.copy')" @click="copy(txtValue(d))">
                        <Icon name="copy" size="sm" />
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <p class="mt-1 text-xs text-gray-500">{{ t('merchant.owner.domains.step2Hint') }}</p>
          </div>

          <!-- 步骤 3：验证 -->
          <div>
            <p class="mb-2 text-sm font-medium text-amber-700 dark:text-amber-400">
              {{ t('merchant.owner.domains.step3Title') }}
            </p>
            <button class="btn btn-primary" :disabled="verifying === d.id" @click="onVerify(d)">
              <Icon name="check" size="sm" class="mr-1" />
              {{ verifying === d.id ? t('common.loading') : t('merchant.owner.domains.verifyNow') }}
            </button>
            <p v-if="dnsInfo?.skip_dns_verify" class="mt-2 text-xs text-amber-600">
              {{ t('merchant.owner.domains.skipDnsHint') }}
            </p>
          </div>
        </div>

        <!-- 已验证：品牌信息摘要 -->
        <div v-else class="grid grid-cols-1 gap-4 px-5 py-4 text-sm md:grid-cols-3">
          <div>
            <div class="text-xs text-gray-500">{{ t('merchant.owner.domains.siteName') }}</div>
            <div class="mt-0.5">{{ d.site_name || '-' }}</div>
          </div>
          <div>
            <div class="text-xs text-gray-500">{{ t('merchant.owner.domains.brandColor') }}</div>
            <div class="mt-0.5 flex items-center gap-2">
              <span v-if="d.brand_color" class="inline-block h-4 w-4 rounded border border-gray-300" :style="{ backgroundColor: d.brand_color }"></span>
              <code class="text-xs">{{ d.brand_color || '-' }}</code>
            </div>
          </div>
          <div>
            <div class="text-xs text-gray-500">{{ t('merchant.owner.domains.verifiedAt') }}</div>
            <div class="mt-0.5">{{ d.verified_at ? formatDateTime(d.verified_at) : '-' }}</div>
          </div>
        </div>
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
import { merchantAPI, type MerchantDomain, type DomainBrandPayload, type DNSSetupInfo } from '@/api/merchant'
import { formatDateTime } from '@/utils/format'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const items = ref<MerchantDomain[]>([])
const dnsInfo = ref<DNSSetupInfo | null>(null)
const loading = ref(false)
const submitting = ref(false)
const verifying = ref<number | null>(null)

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

function txtHost(d: MerchantDomain) {
  const prefix = dnsInfo.value?.txt_host_prefix || '_domain-verify'
  return `${prefix}.${d.domain}`
}

function txtValue(d: MerchantDomain) {
  return `domain-verify=${d.verify_token || ''}`
}

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    appStore.showSuccess(t('common.copied'))
  } catch {
    appStore.showError(t('common.copyFailed'))
  }
}

async function load() {
  loading.value = true
  try {
    const [list, info] = await Promise.all([
      merchantAPI.listDomains(),
      merchantAPI.dnsSetup().catch(() => null),
    ])
    items.value = list
    dnsInfo.value = info
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
  verifying.value = d.id
  try {
    await merchantAPI.verifyDomain(d.id)
    appStore.showSuccess(t('merchant.owner.domains.verifiedToast'))
    await load()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'merchant.errors', t('common.error')))
  } finally {
    verifying.value = null
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
