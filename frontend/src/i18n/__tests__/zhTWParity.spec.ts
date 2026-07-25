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

  // fill 按上游快照批量生成，上游删键时 fill 不会自动跟删，孤儿键会随每轮合并单调堆积。
  // 只校验 fill 层：手工层 zh-TW.ts 有历史沉淀的 stray key，纳入会直接红，且不是本测试要防的问题。
  it('fill contains no stray keys absent from both en and zh', () => {
    const upstreamKeys = new Set([...flattenKeys(en), ...flattenKeys(zh)])
    const stray = flattenKeys(zhTWFill as Record<string, any>).filter((k) => !upstreamKeys.has(k))
    expect(stray).toEqual([])
  })

  // 占位符丢失是 key 覆盖与预编译都抓不到的一类漂移：key 在、语法也合法，只是译文把
  // {limit} 之类写成了当时的字面值，界面从此永远显示旧常量。上游后来把写死的文案改成
  // 占位符时尤其容易中招，故按值比对占位符集合。
  it('translations keep every placeholder present in the source locale', () => {
    const placeholders = (value: string) =>
      new Set((value.match(/\{[a-zA-Z0-9_]+\}/g) ?? []).map((m) => m))
    const valueAt = (obj: Record<string, any>, path: string): unknown =>
      path.split('.').reduce<any>((acc, part) => (acc == null ? acc : acc[part]), obj)

    const drifted: string[] = []
    for (const key of flattenKeys(zhTWFill as Record<string, any>)) {
      const translated = valueAt(zhTWFill as Record<string, any>, key)
      const source = valueAt(zh as Record<string, any>, key) ?? valueAt(en as Record<string, any>, key)
      if (typeof translated !== 'string' || typeof source !== 'string') continue
      const expected = placeholders(source)
      const actual = placeholders(translated)
      const missing = [...expected].filter((p) => !actual.has(p))
      if (missing.length > 0) drifted.push(`${key}: missing ${missing.join(', ')}`)
    }
    expect(drifted).toEqual([])
  })
})
