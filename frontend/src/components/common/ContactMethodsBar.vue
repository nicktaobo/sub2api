<template>
  <div v-if="visibleMethods.length > 0" class="contact-bar" :class="containerClass">
    <span v-if="label" class="contact-bar__label">{{ label }}</span>
    <component
      :is="getTag(item)"
      v-for="item in visibleMethods"
      :key="item.id || item.label + item.value"
      :href="getHref(item) || undefined"
      :target="isLink(item) ? '_blank' : undefined"
      :rel="isLink(item) ? 'noopener noreferrer' : undefined"
      class="contact-bar__item"
      :title="`${item.label}: ${item.value}`"
      @click="handleClick(item, $event)"
    >
      <span class="contact-bar__icon" v-html="iconSvg(item.type)" />
      <span class="contact-bar__text">{{ item.label }}</span>
    </component>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useAppStore } from '@/stores'
import type { ContactMethod } from '@/types'

interface Props {
  variant?: 'inline' | 'card' | 'compact'
  label?: string
  /** override; default reads from app store */
  methods?: ContactMethod[]
}

const props = withDefaults(defineProps<Props>(), {
  variant: 'inline',
  label: '',
  methods: undefined,
})

const appStore = useAppStore()

const visibleMethods = computed<ContactMethod[]>(() => {
  const list = props.methods ?? appStore.cachedPublicSettings?.contact_methods ?? []
  return [...list]
    .filter((m) => m && m.label && m.value)
    .sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0))
})

const containerClass = computed(() => `contact-bar--${props.variant}`)

function isLink(item: ContactMethod): boolean {
  const v = (item.value || '').trim()
  if (item.type === 'telegram') return true
  if (item.type === 'email') return true
  return v.startsWith('http://') || v.startsWith('https://') || v.startsWith('mailto:')
}

function getTag(item: ContactMethod): string {
  return isLink(item) ? 'a' : 'button'
}

function getHref(item: ContactMethod): string {
  const v = (item.value || '').trim()
  if (!isLink(item)) return ''
  if (item.type === 'email') {
    return v.startsWith('mailto:') ? v : `mailto:${v}`
  }
  if (item.type === 'telegram' && !v.startsWith('http')) {
    const handle = v.replace(/^@/, '')
    return `https://t.me/${handle}`
  }
  return v
}

function handleClick(item: ContactMethod, _evt: MouseEvent): void {
  if (isLink(item)) return
  // Non-link types (wechat / wechat_work / qq / custom without URL): copy value
  const v = (item.value || '').trim()
  if (!v) return
  if (navigator.clipboard?.writeText) {
    navigator.clipboard.writeText(v).then(
      () => appStore.showSuccess?.(v),
      () => appStore.showInfo?.(v)
    )
  } else {
    appStore.showInfo?.(v)
  }
}

/* SVG icons – simple monoline, color via currentColor */
function iconSvg(type: string | undefined): string {
  switch (type) {
    case 'telegram':
      return TG_SVG
    case 'wechat_work':
      return WECOM_SVG
    case 'wechat':
      return WECHAT_SVG
    case 'qq':
      return QQ_SVG
    case 'email':
      return EMAIL_SVG
    default:
      return LINK_SVG
  }
}

const TG_SVG = `<svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M9.78 15.36 9.6 19a.62.62 0 0 0 1 .47l2.34-2.06 4.84 3.55c.88.49 1.51.23 1.74-.82l3.15-14.78c.32-1.46-.52-2.04-1.4-1.72L1.6 9.97c-1.39.55-1.37 1.34-.25 1.69l4.96 1.55 11.51-7.27c.54-.34 1.04-.16.64.22z"/></svg>`
const WECOM_SVG = `<svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M12 2C6.48 2 2 5.96 2 10.85c0 2.74 1.46 5.19 3.74 6.85L4.5 21l3.8-2.1c1.16.3 2.4.45 3.7.45 5.52 0 10-3.96 10-8.85S17.52 2 12 2zM8.7 9.55a1.15 1.15 0 1 1 0-2.3 1.15 1.15 0 0 1 0 2.3zm6.6 0a1.15 1.15 0 1 1 0-2.3 1.15 1.15 0 0 1 0 2.3z"/></svg>`
const WECHAT_SVG = `<svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M8.7 4C4.95 4 2 6.55 2 9.8c0 1.85.96 3.5 2.5 4.6L4 16.5l2.45-1.32c.7.18 1.46.27 2.25.27.18 0 .36 0 .54-.02-.18-.5-.29-1.04-.29-1.6 0-3.07 2.86-5.55 6.4-5.55.2 0 .38.01.58.02C15.42 5.6 12.34 4 8.7 4zm-2.5 3.6a.9.9 0 1 1 0 1.8.9.9 0 0 1 0-1.8zm5 0a.9.9 0 1 1 0 1.8.9.9 0 0 1 0-1.8zm9.8 8.7c0-2.74-2.5-4.95-5.6-4.95s-5.6 2.21-5.6 4.95c0 2.75 2.5 4.95 5.6 4.95.62 0 1.22-.09 1.78-.25l1.95 1.05-.46-1.78c1.43-.93 2.33-2.35 2.33-3.97zm-7.4-.7a.75.75 0 1 1 0-1.5.75.75 0 0 1 0 1.5zm3.6 0a.75.75 0 1 1 0-1.5.75.75 0 0 1 0 1.5z"/></svg>`
const QQ_SVG = `<svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M12 2C7.6 2 5.4 5.4 5.4 8.9c0 1.4.4 2.6 1 3.6-.7.5-1.7 1.4-2 2.3-.4 1.2.7 1.6 1.4.7.2-.3.5-.6.8-.9.5 1.6 1.7 3 2.4 3.5-.4.2-1.7.6-1.5 1.6.1.5.7.7 1.4.7h6.2c.7 0 1.3-.2 1.4-.7.2-1-1.1-1.4-1.5-1.6.7-.5 1.9-1.9 2.4-3.5.3.3.6.6.8.9.7.9 1.8.5 1.4-.7-.3-.9-1.3-1.8-2-2.3.6-1 1-2.2 1-3.6C18.6 5.4 16.4 2 12 2z"/></svg>`
const EMAIL_SVG = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="5" width="18" height="14" rx="2"/><path d="m3 7 9 6 9-6"/></svg>`
const LINK_SVG = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M10 13a5 5 0 0 0 7 0l3-3a5 5 0 0 0-7-7l-1 1"/><path d="M14 11a5 5 0 0 0-7 0l-3 3a5 5 0 0 0 7 7l1-1"/></svg>`
</script>

<style scoped>
.contact-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.contact-bar__label {
  font-size: 12.5px;
  color: rgb(107 114 128);
}
.dark .contact-bar__label {
  color: rgb(156 163 175);
}
.contact-bar__item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 999px;
  background: rgb(243 244 246);
  color: rgb(55 65 81);
  font-size: 13px;
  font-weight: 500;
  line-height: 1;
  cursor: pointer;
  text-decoration: none;
  border: 1px solid transparent;
  transition: background 0.15s, color 0.15s, transform 0.15s, border-color 0.15s;
}
.contact-bar__item:hover {
  background: rgb(229 231 235);
  color: rgb(17 24 39);
  transform: translateY(-1px);
}
.dark .contact-bar__item {
  background: rgba(255, 255, 255, 0.06);
  color: rgba(255, 255, 255, 0.78);
  border-color: rgba(255, 255, 255, 0.08);
}
.dark .contact-bar__item:hover {
  background: rgba(255, 255, 255, 0.12);
  color: #fff;
}
.contact-bar__icon {
  display: inline-flex;
  width: 16px;
  height: 16px;
}
.contact-bar__icon :deep(svg) {
  width: 100%;
  height: 100%;
}
.contact-bar__text {
  white-space: nowrap;
}

/* Card variant – larger, used on homepage section */
.contact-bar--card {
  gap: 16px;
}
.contact-bar--card .contact-bar__item {
  padding: 14px 20px;
  font-size: 14.5px;
  background: #fff;
  color: rgb(31 41 55);
  border: 1px solid rgb(229 231 235);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
}
.contact-bar--card .contact-bar__item:hover {
  border-color: rgb(0 102 255);
  color: rgb(0 102 255);
  box-shadow: 0 8px 24px rgba(0, 102, 255, 0.15);
}
.contact-bar--card .contact-bar__icon {
  width: 20px;
  height: 20px;
}

/* Compact variant – used in header dropdown */
.contact-bar--compact {
  flex-direction: column;
  align-items: stretch;
  gap: 4px;
}
.contact-bar--compact .contact-bar__item {
  width: 100%;
  justify-content: flex-start;
  padding: 8px 12px;
  background: transparent;
  border: none;
  border-radius: 6px;
}
.contact-bar--compact .contact-bar__item:hover {
  background: rgb(243 244 246);
  transform: none;
}
.dark .contact-bar--compact .contact-bar__item:hover {
  background: rgba(255, 255, 255, 0.06);
}
</style>
