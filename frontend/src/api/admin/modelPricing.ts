/**
 * Admin 通用模型价格覆盖表 API（独立于渠道管理定价）。
 * 价格为每百万 token；currency ∈ {CNY,USD}；精确匹配优先、无则最长前缀回退。
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

export interface ModelPricingConfig {
  entries: ModelPricingEntry[]
}

export async function getModelPricing(): Promise<ModelPricingConfig> {
  const { data } = await apiClient.get<ModelPricingConfig>('/admin/model-pricing')
  return data
}

export async function updateModelPricing(req: ModelPricingConfig): Promise<ModelPricingConfig> {
  const { data } = await apiClient.put<ModelPricingConfig>('/admin/model-pricing', req)
  return data
}

export const modelPricingAPI = { getModelPricing, updateModelPricing }
export default modelPricingAPI
