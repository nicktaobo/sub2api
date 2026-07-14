type Dict = Record<string, any>

/**
 * Deep-merge locale message objects. Values from `override` win over `base`;
 * nested plain objects are merged recursively. Used to layer the hand-curated
 * zh-TW.ts (override) on top of the auto-generated zh-TW.fill.ts (base) so the
 * curated translations always take precedence while the fill covers any gaps.
 */
export function deepMergeMessages<T extends Dict>(base: T, override: Dict): T {
  const out: Dict = { ...base }
  for (const [key, value] of Object.entries(override)) {
    const prev = out[key]
    if (
      prev && typeof prev === 'object' && !Array.isArray(prev) &&
      value && typeof value === 'object' && !Array.isArray(value)
    ) {
      out[key] = deepMergeMessages(prev, value)
    } else {
      out[key] = value
    }
  }
  return out as T
}
