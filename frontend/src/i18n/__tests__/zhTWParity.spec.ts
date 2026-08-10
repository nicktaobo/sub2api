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

  // 同层重复键会被 JS 静默合并（后写覆盖先写），import 之后从对象上完全查不出来，
  // 而本仓库 eslint 未开 no-dupe-keys —— 也就是说这条风险此前没有任何自动守卫。
  // fill 层按上游快照批量补齐、一次动辄几百个 key，插错嵌套位置制造重复键很容易发生，
  // 后果是把既有译文静默顶掉。故直接解析源文件 AST 比对同层键名。
  it('fill has no duplicate sibling keys (JS would silently drop the earlier one)', async () => {
    const ts = await import('typescript')
    const { readFileSync } = await import('node:fs')
    const { resolve } = await import('node:path')
    // vitest 下 import.meta.url 不一定是 file: scheme，按项目根拼路径更稳。
    const fillPath = resolve(process.cwd(), 'src/i18n/locales/zh-TW.fill.ts')
    const source = ts.createSourceFile(
      fillPath,
      readFileSync(fillPath, 'utf8'),
      ts.ScriptTarget.Latest,
      true
    )

    const duplicates: string[] = []
    const walk = (node: any, path: string) => {
      if (ts.isObjectLiteralExpression(node)) {
        const seen = new Set<string>()
        for (const prop of node.properties) {
          if (!ts.isPropertyAssignment(prop)) continue
          const name = ts.isIdentifier(prop.name) || ts.isStringLiteral(prop.name) ? prop.name.text : null
          if (name == null) continue
          const full = path ? `${path}.${name}` : name
          if (seen.has(name)) duplicates.push(full)
          seen.add(name)
          walk(prop.initializer, full)
        }
        return
      }
      ts.forEachChild(node, (child: any) => walk(child, path))
    }
    walk(source, '')
    expect(duplicates).toEqual([])
  })
})
