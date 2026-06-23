/**
 * Admin Profit API endpoints
 * 利润自动化核算：按 团队(分公司/销售) / 客户 / 代理(商户) 四个维度
 * 实时聚合 usage_logs，返回 营收/成本/毛利。
 */

import { apiClient } from '../client'

export type ProfitGroupBy = 'merchant' | 'user' | 'attribute'

export interface ProfitRow {
  key: string
  name: string
  revenue: number
  cost: number
  profit: number
  profit_rate: number
  request_count: number
}

export interface ProfitSummaryResponse {
  group_by: ProfitGroupBy
  attribute_id?: number
  start: string
  end: string
  total_revenue: number
  total_cost: number
  total_profit: number
  rows: ProfitRow[]
}

export interface ProfitSummaryParams {
  start?: string // RFC3339
  end?: string   // RFC3339
  group_by: ProfitGroupBy
  attribute_id?: number
  limit?: number
}

export async function getProfitSummary(
  params: ProfitSummaryParams
): Promise<ProfitSummaryResponse> {
  const { data } = await apiClient.get<ProfitSummaryResponse>('/admin/profit/summary', {
    params,
  })
  return data
}

export const profitAPI = {
  getSummary: getProfitSummary,
}

export default profitAPI
