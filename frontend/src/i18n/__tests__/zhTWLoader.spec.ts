import { beforeAll, describe, expect, it } from 'vitest'
import { i18n, loadLocaleMessages } from '@/i18n'

// Exercises the real runtime loader in i18n/index.ts: it dynamically imports
// both ./locales/zh-TW (hand) and ./locales/zh-TW.fill and deep-merges them,
// then registers the result via setLocaleMessage. We assert against the
// registered message tree — the exact data vue-i18n serves for zh-TW — proving
// the reported keys resolve to Traditional Chinese instead of English/raw keys.
describe('zh-TW runtime loader', () => {
  let msg: Record<string, any>
  beforeAll(async () => {
    await loadLocaleMessages('zh-TW')
    msg = i18n.global.getLocaleMessage('zh-TW') as Record<string, any>
  })

  const get = (path: string) => path.split('.').reduce<any>((o, k) => (o == null ? o : o[k]), msg)

  const cases: Array<[string, string]> = [
    // from the reported screenshots — were English fallback / raw keys before
    ['keys.columnSettings', '列設定'],
    ['keys.currentConcurrency', '當前併發'],
    ['admin.proxies.fallbackMode', '失敗回退'],
    ['admin.proxies.fallbackNone', '不回退'],
    ['admin.proxies.fallbackProxy', '指定備用代理'],
    ['admin.proxies.fallbackDirect', '回退直連'],
  ]

  for (const [path, expected] of cases) {
    it(`resolves ${path} to Traditional Chinese`, () => {
      expect(get(path)).toBe(expected)
    })
  }

  it('keeps hand-curated values winning over the fill layer', () => {
    // keys.deleteKey is defined in the hand file; the loader must not shadow it.
    expect(get('keys.deleteKey')).toBe('刪除金鑰')
  })

  it('preserves interpolation placeholders in filled keys', () => {
    expect(get('admin.accounts.dataImportIgnoredFiles')).toBe('已忽略 {count} 個非 JSON 檔案')
  })
})
