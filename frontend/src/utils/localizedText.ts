/**
 * 解析"按 locale 取值"的字符串。
 *
 * 兼容两种格式：
 *   1. 普通字符串（旧值不变）。
 *   2. JSON 对象 {"en": "...", "zh-TW": "...", "zh": "..."}。
 *
 * 解析顺序：locale → locale 前缀 → 'en' → 任一非空 → 兜底 fallback。
 */
export function resolveLocalizedText(
  raw: string | null | undefined,
  locale: string,
  fallback = '',
): string {
  const text = (raw ?? '').trim()
  if (!text) {
    return fallback
  }
  if (text.startsWith('{')) {
    try {
      const parsed = JSON.parse(text)
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        const map = parsed as Record<string, unknown>
        const candidates = [locale, locale.split('-')[0], locale.split('_')[0], 'en']
        const seen = new Set<string>()
        for (const code of candidates) {
          const key = (code || '').trim()
          if (!key || seen.has(key)) continue
          seen.add(key)
          const value = map[key]
          if (typeof value === 'string' && value.trim()) {
            return value.trim()
          }
        }
        for (const key of Object.keys(map)) {
          const value = map[key]
          if (typeof value === 'string' && value.trim()) {
            return value.trim()
          }
        }
        return fallback
      }
    } catch {
      // not JSON, fall through to plain string
    }
  }
  return text
}
