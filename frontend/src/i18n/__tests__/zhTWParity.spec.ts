import { describe, expect, it } from 'vitest'
import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'
import zhTWHand from '@/i18n/locales/zh-TW'
import zhTWFill from '@/i18n/locales/zh-TW.fill'
import { deepMergeMessages } from '@/i18n/mergeMessages'

function flattenKeys(obj: Record<string, any>, prefix = ''): string[] {
  const keys: string[] = []
  for (const [k, v] of Object.entries(obj)) {
    const fullKey = prefix ? `${prefix}.${k}` : k
    if (typeof v === 'object' && v !== null && !Array.isArray(v)) {
      keys.push(...flattenKeys(v, fullKey))
    } else {
      keys.push(fullKey)
    }
  }
  return keys
}

// The runtime loader (i18n/index.ts) serves zh-TW as this exact merge.
const merged = deepMergeMessages(zhTWFill as Record<string, any>, zhTWHand as Record<string, any>)
const mergedKeys = new Set(flattenKeys(merged))

describe('zh-TW locale parity', () => {
  it('covers every key present in the zh locale (no English/raw-key fallback)', () => {
    const missing = flattenKeys(zh).filter((k) => !mergedKeys.has(k))
    expect(missing).toEqual([])
  })

  it('covers every key present in the en locale', () => {
    const missing = flattenKeys(en).filter((k) => !mergedKeys.has(k))
    expect(missing).toEqual([])
  })

  it('fill only supplies keys the hand-curated file is missing (no shadowing)', () => {
    const handKeys = new Set(flattenKeys(zhTWHand as Record<string, any>))
    const shadowed = flattenKeys(zhTWFill as Record<string, any>).filter((k) => handKeys.has(k))
    expect(shadowed).toEqual([])
  })
})
