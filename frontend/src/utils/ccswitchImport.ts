import type { GroupPlatform } from '@/types'

export const OPENAI_CC_SWITCH_CODEX_MODEL = 'gpt-5.5'
export const DEEPSEEK_CC_SWITCH_CODEX_MODEL = 'deepseek-v4-flash'
export const MOONSHOT_CC_SWITCH_CODEX_MODEL = 'kimi-for-coding'

export type CcSwitchClientType = 'claude' | 'gemini' | 'codex'

export interface CcSwitchImportConfig {
  app: string
  endpoint: string
  model?: string
}

export interface CcSwitchImportDeeplinkInput {
  baseUrl: string
  platform?: GroupPlatform | null
  clientType: CcSwitchClientType
  providerName: string
  apiKey: string
  usageScript: string
}

export function resolveCcSwitchImportConfig(
  platform: GroupPlatform | undefined | null,
  clientType: CcSwitchClientType,
  baseUrl: string
): CcSwitchImportConfig {
  switch (platform || 'anthropic') {
    case 'antigravity':
      return {
        app: clientType === 'gemini' ? 'gemini' : 'claude',
        endpoint: `${baseUrl}/antigravity`
      }
    case 'openai':
      if (clientType === 'codex') {
        return { app: 'codex', endpoint: baseUrl, model: OPENAI_CC_SWITCH_CODEX_MODEL }
      }
      return { app: 'claude', endpoint: baseUrl }
    case 'deepseek':
      if (clientType === 'codex') {
        return { app: 'codex', endpoint: baseUrl, model: DEEPSEEK_CC_SWITCH_CODEX_MODEL }
      }
      return { app: 'claude', endpoint: baseUrl }
    case 'moonshot':
      if (clientType === 'codex') {
        return { app: 'codex', endpoint: baseUrl, model: MOONSHOT_CC_SWITCH_CODEX_MODEL }
      }
      return { app: 'claude', endpoint: baseUrl }
    case 'gemini':
      return {
        app: 'gemini',
        endpoint: baseUrl
      }
    default:
      return {
        app: 'claude',
        endpoint: baseUrl
      }
  }
}

export function buildCcSwitchImportDeeplink(input: CcSwitchImportDeeplinkInput): string {
  const config = resolveCcSwitchImportConfig(input.platform, input.clientType, input.baseUrl)
  const entries: [string, string][] = [
    ['resource', 'provider'],
    ['app', config.app],
    ['name', input.providerName],
    ['homepage', input.baseUrl],
    ['endpoint', config.endpoint],
    ['apiKey', input.apiKey],
    ['configFormat', 'json'],
    ['usageEnabled', 'true'],
    ['usageScript', btoa(input.usageScript)],
    ['usageAutoInterval', '30']
  ]

  if (config.model) {
    entries.splice(2, 0, ['model', config.model])
  }

  return `ccswitch://v1/import?${new URLSearchParams(entries).toString()}`
}
