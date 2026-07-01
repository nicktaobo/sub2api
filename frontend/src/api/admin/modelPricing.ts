/**
 * Admin 通用模型价格覆盖表 API（独立于渠道管理定价）。
 * 价格为每百万 token；currency ∈ {CNY,USD}；精确匹配优先、无则最长前缀回退。
 * List 同时返回只读的内置全量表(builtin),并支持一键拉取最新价(refresh preview/apply)。
 */

import { apiClient } from '../client'

export interface ModelPricingEntry {
  model: string
  currency: 'CNY' | 'USD'
  input: number
  output: number
  cache: number
  has_cache: boolean
  enabled: boolean
}

// 内置全量价目表的一条只读记录(②国产¥表 + ③LiteLLM JSON 摊平,每百万 token)。
export interface BuiltinPricingEntry {
  model: string
  currency: 'CNY' | 'USD'
  input: number
  output: number
  cache: number
  has_cache: boolean
  source: string // cny:<plat> | litellm | <provider>
}

export interface ModelPricingConfig {
  entries: ModelPricingEntry[]
  builtin?: BuiltinPricingEntry[]
}

// 一键拉价 diff(每百万 token,USD 口径)。input/output/cache-read 三项。
export interface PricingChange {
  model: string
  kind: 'added' | 'removed' | 'changed'
  old_input: number
  new_input: number
  old_output: number
  new_output: number
  old_cache: number
  new_cache: number
}

export interface PricingRefreshPreview {
  remote_url: string
  current_count: number
  remote_count: number
  added: number
  removed: number
  changed: number
  changes: PricingChange[]
  truncated: boolean
}

export async function getModelPricing(): Promise<ModelPricingConfig> {
  const { data } = await apiClient.get<ModelPricingConfig>('/admin/model-pricing')
  return data
}

export async function updateModelPricing(req: { entries: ModelPricingEntry[] }): Promise<ModelPricingConfig> {
  const { data } = await apiClient.put<ModelPricingConfig>('/admin/model-pricing', req)
  return data
}

// 拉取远程最新价表与当前内置表做 diff,不落库。
export async function refreshPreview(): Promise<PricingRefreshPreview> {
  const { data } = await apiClient.post<PricingRefreshPreview>('/admin/model-pricing/refresh/preview')
  return data
}

// 确认后落库刷新内置全量表(返回值无消费点,落库后前端重新拉 List)。
export async function refreshApply(): Promise<void> {
  await apiClient.post('/admin/model-pricing/refresh/apply')
}

export const modelPricingAPI = { getModelPricing, updateModelPricing, refreshPreview, refreshApply }
export default modelPricingAPI
