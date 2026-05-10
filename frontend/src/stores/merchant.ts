/**
 * Merchant brand store (RFC v1.13)
 * Holds the result of GET /merchant_brand. Used to detect whether the current
 * host is a merchant-branded site and to expose brand metadata to the layout.
 */

import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { merchantAPI, type MerchantBrand } from '@/api'

export const useMerchantStore = defineStore('merchant', () => {
  // ==================== State ====================
  const brand = ref<MerchantBrand | null>(null)
  const loading = ref(false)
  const loaded = ref(false)
  const error = ref<string | null>(null)

  // ==================== Computed ====================
  const isMerchantSite = computed(() => brand.value?.is_merchant_site === true)
  const merchantId = computed(() => brand.value?.merchant_id ?? null)
  const merchantName = computed(() => brand.value?.merchant_name ?? '')
  const status = computed(() => brand.value?.status ?? null)
  const domain = computed(() => brand.value?.domain ?? '')
  const siteName = computed(() => brand.value?.site_name ?? '')
  const siteLogo = computed(() => brand.value?.site_logo ?? '')
  const brandColor = computed(() => brand.value?.brand_color ?? '')
  const customCss = computed(() => brand.value?.custom_css ?? '')
  const homeContent = computed(() => brand.value?.home_content ?? '')
  const seoTitle = computed(() => brand.value?.seo_title ?? '')
  const seoDescription = computed(() => brand.value?.seo_description ?? '')
  const seoKeywords = computed(() => brand.value?.seo_keywords ?? '')

  // ==================== Actions ====================
  async function loadBrand(force = false): Promise<void> {
    if (loaded.value && !force) return
    loading.value = true
    error.value = null
    try {
      brand.value = await merchantAPI.brand()
      loaded.value = true
    } catch (err) {
      brand.value = { is_merchant_site: false }
      // Errors here are non-fatal; record for diagnostics but don't disrupt boot.
      const e = err as { message?: string } | undefined
      error.value = e?.message ?? 'Failed to load merchant brand'
      // eslint-disable-next-line no-console
      console.warn('[merchantStore] loadBrand failed:', error.value)
    } finally {
      loading.value = false
    }
  }

  function reset(): void {
    brand.value = null
    loaded.value = false
    error.value = null
  }

  return {
    // state
    brand,
    loading,
    loaded,
    error,
    // computed
    isMerchantSite,
    merchantId,
    merchantName,
    status,
    domain,
    siteName,
    siteLogo,
    brandColor,
    customCss,
    homeContent,
    seoTitle,
    seoDescription,
    seoKeywords,
    // actions
    loadBrand,
    reset,
  }
})
