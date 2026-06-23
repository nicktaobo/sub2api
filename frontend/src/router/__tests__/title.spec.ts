import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { resolveDocumentTitle, resolveRouteDocumentTitle } from '@/router/title'
import { useMerchantStore } from '@/stores/merchant'

describe('resolveDocumentTitle', () => {
  it('路由存在标题时，使用“路由标题 - 站点名”格式', () => {
    expect(resolveDocumentTitle('Usage Records', 'My Site')).toBe('Usage Records - My Site')
  })

  it('路由无标题时，回退到站点名', () => {
    expect(resolveDocumentTitle(undefined, 'My Site')).toBe('My Site')
  })

  it('站点名为空时，回退默认站点名', () => {
    expect(resolveDocumentTitle('Dashboard', '')).toBe('Dashboard - Sub2API')
    expect(resolveDocumentTitle(undefined, '   ')).toBe('Sub2API')
  })

  it('站点名变更时仅影响后续路由标题计算', () => {
    const before = resolveDocumentTitle('Admin Dashboard', 'Alpha')
    const after = resolveDocumentTitle('Admin Dashboard', 'Beta')

    expect(before).toBe('Admin Dashboard - Alpha')
    expect(after).toBe('Admin Dashboard - Beta')
  })
})

describe('resolveRouteDocumentTitle', () => {
  it('自定义页面菜单加载后，使用菜单名称作为标题', () => {
    const route = {
      name: 'CustomPage',
      params: { id: 'scheduler' },
      meta: {
        title: 'Custom Page'
      }
    }

    expect(resolveRouteDocumentTitle(route, 'EzouAPI')).toBe('Custom Page - EzouAPI')
    expect(resolveRouteDocumentTitle(route, 'EzouAPI', [
      {
        id: 'scheduler',
        label: '账号调度器',
        icon_svg: '',
        url: 'https://example.com',
        visibility: 'admin',
        sort_order: 0
      }
    ])).toBe('账号调度器 - EzouAPI')
  })
})

describe('resolveRouteDocumentTitle - 商户品牌站', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  const route = { name: 'Home', params: {}, meta: { title: 'Dashboard' } }

  it('商户站且有 seoTitle 时，页签标题固定为 seoTitle（覆盖路由标题）', () => {
    const merchant = useMerchantStore()
    merchant.brand = { is_merchant_site: true, seo_title: 'BrandCo - AI Gateway' }

    expect(resolveRouteDocumentTitle(route, 'EzouAPI')).toBe('BrandCo - AI Gateway')
  })

  it('非商户站时仍走默认“路由标题 - 站点名”', () => {
    const merchant = useMerchantStore()
    merchant.brand = { is_merchant_site: false }

    expect(resolveRouteDocumentTitle(route, 'EzouAPI')).toBe('Dashboard - EzouAPI')
  })

  it('商户站但 seoTitle 为空时，回退默认路由标题', () => {
    const merchant = useMerchantStore()
    merchant.brand = { is_merchant_site: true }

    expect(resolveRouteDocumentTitle(route, 'EzouAPI')).toBe('Dashboard - EzouAPI')
  })
})
