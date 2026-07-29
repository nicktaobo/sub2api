/**
 * 全站共享的 CNY/USD 汇率。
 *
 * `GET /api/v1/settings/fx-rate` 是公开路由（无 JWT），所以公开页匿名也能拿到；
 * 拉取失败时静默降级到 DEFAULT_CNY_PER_USD——汇率不是页面主体，绝不能因为它挂了
 * 让整页崩掉或空白。
 *
 * 模块级单例 + in-flight 去重：同一页里登录页/公开页/多个组件并发调用只发一次请求。
 */

import { ref } from 'vue'
import systemAPI from '@/api/system'
import { DEFAULT_CNY_PER_USD } from '@/utils/pricing'

const fxRate = ref<number>(DEFAULT_CNY_PER_USD)
/** true = 已成功从后端拿到真实汇率；false = 当前用的是兜底常量。 */
const fxLoaded = ref(false)

let inflight: Promise<number> | null = null

/**
 * ensureFxRate 保证汇率已拉取过一次。
 *   - 已加载成功且未强制刷新 → 直接返回当前值
 *   - 进行中 → 复用同一个 promise
 *   - 失败 → 保持兜底值，不 reject
 */
export function ensureFxRate(force = false): Promise<number> {
  if (fxLoaded.value && !force) return Promise.resolve(fxRate.value)
  if (inflight && !force) return inflight
  inflight = systemAPI
    .getFXRate()
    .then((fx) => {
      if (fx && fx.cny_per_usd > 0) {
        fxRate.value = fx.cny_per_usd
        fxLoaded.value = true
      }
      return fxRate.value
    })
    .catch(() => fxRate.value)
    .finally(() => {
      inflight = null
    })
  return inflight
}

export function useFxRate() {
  return { fxRate, fxLoaded, ensureFxRate }
}

/** 仅供测试重置模块级单例。 */
export function __resetFxRateForTest() {
  fxRate.value = DEFAULT_CNY_PER_USD
  fxLoaded.value = false
  inflight = null
}
