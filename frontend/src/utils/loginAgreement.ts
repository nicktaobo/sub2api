import type { LoginAgreementDocument } from '@/types'

export type LoginAgreementIcon = 'document' | 'shield' | 'globe' | 'cog'

/**
 * 按当前 locale 解析协议文档的标题与正文。
 *
 * 解析顺序：
 *   1. doc.i18n[locale]
 *   2. doc.i18n[locale 的语言前缀, e.g. 'zh' for 'zh-TW']
 *   3. doc.i18n[any en variant]
 *   4. doc.title / doc.content_md（缺省）
 *
 * Title 与 content_md 各自独立回退，便于管理员只翻译其中一项。
 */
export function resolveLoginAgreementDocumentLocale(
  doc: LoginAgreementDocument,
  locale: string,
): { title: string; content_md: string } {
  const i18n = doc.i18n || {}
  const candidates = uniq([
    locale,
    locale.split('-')[0],
    locale.split('_')[0],
    'en',
  ])

  let title = ''
  let content = ''
  for (const code of candidates) {
    const entry = i18n[code]
    if (!entry) continue
    if (!title && entry.title?.trim()) {
      title = entry.title.trim()
    }
    if (!content && entry.content_md?.trim()) {
      content = entry.content_md.trim()
    }
    if (title && content) break
  }

  return {
    title: title || (doc.title || '').trim(),
    content_md: content || (doc.content_md || '').trim(),
  }
}

/**
 * 协议文档是否对当前 locale 有可显示的标题。
 * 用于隐藏没有对应翻译且无回退的条目。
 */
export function hasLoginAgreementTitle(doc: LoginAgreementDocument, locale: string): boolean {
  return Boolean(resolveLoginAgreementDocumentLocale(doc, locale).title)
}

/**
 * 协议文档是否在任意 locale 上有可显示的标题。
 * 父级路由用它过滤出展示候选；具体 locale 的回退由渲染组件处理。
 */
export function hasAnyLoginAgreementTitle(doc: LoginAgreementDocument): boolean {
  if (doc.title?.trim()) {
    return true
  }
  const i18n = doc.i18n || {}
  for (const key of Object.keys(i18n)) {
    if (i18n[key]?.title?.trim()) {
      return true
    }
  }
  return false
}

/**
 * 根据 doc.id 选图标；id 缺失时再用标题里的关键字做兜底匹配。
 */
export function resolveLoginAgreementDocumentIcon(
  id: string | undefined,
  title: string | undefined,
): LoginAgreementIcon {
  const key = (id || '').trim().toLowerCase()
  if (key) {
    if (key.includes('privacy') || key.includes('policy')) return 'shield'
    if (key.includes('region') || key.includes('country') || key.includes('geo')) return 'globe'
    if (key.includes('specific') || key.includes('service-specific')) return 'cog'
    if (key.includes('terms') || key.includes('tos') || key.includes('agreement')) return 'document'
  }
  const t = (title || '').toLowerCase()
  if (t.includes('privacy') || t.includes('policy') || t.includes('政策') || t.includes('隱私') || t.includes('隐私')) {
    return 'shield'
  }
  if (t.includes('region') || t.includes('country') || t.includes('地區') || t.includes('地区') || t.includes('國家') || t.includes('国家')) {
    return 'globe'
  }
  if (t.includes('specific') || t.includes('特定')) {
    return 'cog'
  }
  return 'document'
}

function uniq(items: string[]): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const item of items) {
    const v = (item || '').trim()
    if (!v || seen.has(v)) continue
    seen.add(v)
    out.push(v)
  }
  return out
}
