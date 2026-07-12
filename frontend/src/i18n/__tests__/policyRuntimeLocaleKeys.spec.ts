import { describe, expect, it } from 'vitest'
import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'
import zhTW from '@/i18n/locales/zh-TW'

// 回归测试：合并上游后曾出现 i18n key 错位（key 定义在别的块里），页面渲染裸 key。
// - admin.settings.openaiFastPolicy.userIds 等 5 个 key 曾被放进 betaPolicy 块
// - admin.ops.runtime.metricThresholds 等 9 个 key 曾只存在于 settings 块
function resolveKey(obj: Record<string, any>, path: string): unknown {
  return path.split('.').reduce<any>((acc, part) => (acc == null ? undefined : acc[part]), obj)
}

const locales: Array<[string, Record<string, any>]> = [
  ['en', en],
  ['zh', zh],
  ['zh-TW', zhTW],
]

describe('openaiFastPolicy user-id locale keys (SettingsView.vue)', () => {
  const requiredKeys = [
    'admin.settings.openaiFastPolicy.userIds',
    'admin.settings.openaiFastPolicy.userIdsHint',
    'admin.settings.openaiFastPolicy.userIdPlaceholder',
    'admin.settings.openaiFastPolicy.addUserId',
    'admin.settings.openaiFastPolicy.removeUserId',
  ]

  for (const [name, messages] of locales) {
    for (const key of requiredKeys) {
      it(`${name} locale resolves ${key}`, () => {
        expect(resolveKey(messages, key), `${name}: ${key}`).toBeTypeOf('string')
      })
    }
  }
})

describe('ops runtime metric-threshold locale keys (OpsRuntimeSettingsCard.vue)', () => {
  const requiredKeys = [
    'admin.ops.runtime.metricThresholds',
    'admin.ops.runtime.metricThresholdsHint',
    'admin.ops.runtime.slaMinPercent',
    'admin.ops.runtime.slaMinPercentHint',
    'admin.ops.runtime.ttftP99MaxMs',
    'admin.ops.runtime.ttftP99MaxMsHint',
    'admin.ops.runtime.requestErrorRateMaxPercent',
    'admin.ops.runtime.requestErrorRateMaxPercentHint',
    'admin.ops.runtime.upstreamErrorRateMaxPercent',
    'admin.ops.runtime.upstreamErrorRateMaxPercentHint',
  ]

  for (const [name, messages] of locales) {
    for (const key of requiredKeys) {
      it(`${name} locale resolves ${key}`, () => {
        expect(resolveKey(messages, key), `${name}: ${key}`).toBeTypeOf('string')
      })
    }
  }
})
