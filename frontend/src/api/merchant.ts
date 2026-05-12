/**
 * Merchant System API endpoints (RFC v1.13 Phase 5)
 *
 * Three groups of endpoints:
 *   1. /admin/merchants/*       — admin-only, manages merchants
 *   2. /merchant/*              — merchant owner self-service (JWT auth)
 *   3. /merchant_brand          — public, returns brand info for the requested host
 */

import { apiClient } from './client'
import type { PaginatedResponse } from '@/types'

// ==================== Domain types ====================

export type MerchantStatus = 'active' | 'suspended'

export interface NotifyEmailEntry {
  email: string
  disabled?: boolean
}

export interface Merchant {
  id: number
  owner_user_id: number
  name: string
  status: MerchantStatus
  low_balance_threshold: number
  notify_emails: NotifyEmailEntry[] | string[] | null
  // admin list 富对象字段
  domains?: string[]
  sub_user_count?: number
  owner_balance?: number
  balance_baseline?: number
  domain?: string | null
  site_name?: string | null
  site_logo?: string | null
  brand_color?: string | null
  custom_css?: string | null
  home_content?: string | null
  seo_title?: string | null
  seo_description?: string | null
  seo_keywords?: string | null
  created_at: string
  updated_at: string
}

export interface MerchantGroupMarkup {
  merchant_id: number
  group_id: number
  /** v2.0: 对外售价绝对倍率（base × sell_rate = sub_user 实付）。 */
  sell_rate: number
  created_at?: string
  updated_at?: string
}

export interface MerchantGroupCost {
  merchant_id: number
  group_id: number
  /** 拿货价绝对倍率，admin 配置。base × cost_rate = 平台从 sub_user 余额扣的部分。 */
  cost_rate: number
  created_at?: string
  updated_at?: string
}

export interface MerchantLedgerEntry {
  id: number
  merchant_id: number
  owner_user_id: number
  counterparty_user_id?: number | null
  direction: 'credit' | 'debit'
  amount: number
  balance_after: number
  source: string
  ref_type?: string | null
  ref_id?: number | null
  idempotency_key?: string | null
  metadata?: Record<string, unknown> | null
  created_at: string
}

export interface MerchantAuditLogEntry {
  id: number
  merchant_id: number
  admin_id?: number | null
  field: string
  old_value?: string | null
  new_value?: string | null
  reason?: string | null
  created_at: string
}

export interface MerchantInfo extends Merchant {
  // Alias for clarity in self-service contexts.
}

export interface MerchantBrand {
  is_merchant_site: boolean
  merchant_id?: number | null
  merchant_name?: string | null
  status?: MerchantStatus | null
  domain?: string | null
  site_name?: string | null
  site_logo?: string | null
  brand_color?: string | null
  custom_css?: string | null
  home_content?: string | null
  seo_title?: string | null
  seo_description?: string | null
  seo_keywords?: string | null
}

export interface MerchantDomain {
  id: number
  merchant_id: number
  domain: string
  site_name?: string
  site_logo?: string
  brand_color?: string
  custom_css?: string
  home_content?: string
  seo_title?: string
  seo_description?: string
  seo_keywords?: string
  verify_token?: string
  verified?: boolean
  verified_at?: string | null
  is_primary?: boolean
  created_at?: string
  updated_at?: string
  deleted_at?: string | null
}

// ==================== Request payloads ====================

export interface CreateMerchantPayload {
  owner_user_id: number
  name: string
  low_balance_threshold: number
  notify_emails: NotifyEmailEntry[] | string[]
  reason?: string
}

export interface AdjustAmountPayload {
  amount: number
  reason?: string
}

// ==================== Helpers ====================

interface ListParams {
  offset?: number
  limit?: number
}

function paginationParams(offset = 0, limit = 20): Record<string, number> {
  return { offset, limit }
}

// ==================== Admin endpoints ====================

/** GET /admin/merchants */
export async function adminMerchantList(
  status?: MerchantStatus,
  offset = 0,
  limit = 20,
  search?: string,
): Promise<PaginatedResponse<Merchant>> {
  const params: Record<string, string | number> = { ...paginationParams(offset, limit) }
  if (status) params.status = status
  if (search) params.q = search
  const { data } = await apiClient.get<PaginatedResponse<Merchant> | Merchant[]>(
    '/admin/merchants',
    { params },
  )
  // Backend may return a paginated response or a bare array. Normalize.
  if (Array.isArray(data)) {
    return {
      items: data,
      total: data.length,
      page: 1,
      page_size: data.length || limit,
      pages: 1,
    }
  }
  return data
}

/** POST /admin/merchants */
export async function adminMerchantCreate(payload: CreateMerchantPayload): Promise<Merchant> {
  const { data } = await apiClient.post<Merchant>('/admin/merchants', payload)
  return data
}

/** GET /admin/merchants/:id */
export async function adminMerchantGet(id: number): Promise<Merchant> {
  const { data } = await apiClient.get<Merchant>(`/admin/merchants/${id}`)
  return data
}

/** Admin 视角看某商户的全量统计（利润 / 本金 / 提现 / 子用户规模）。 */
export interface AdminMerchantStats {
  total_profit: number
  current_balance: number
  total_self_recharge: number
  total_pay_to_user: number
  total_refund_from_user: number
  total_withdrawn: number
  pending_withdraw: number
  sub_user_count: number
  sub_user_total_balance: number
  sub_user_total_recharge: number
}

/** GET /admin/merchants/:id/stats */
export async function adminMerchantStats(id: number): Promise<AdminMerchantStats> {
  const { data } = await apiClient.get<AdminMerchantStats>(`/admin/merchants/${id}/stats`)
  return data
}

/** PATCH /admin/merchants/:id/status */
export async function adminMerchantSetStatus(
  id: number,
  status: MerchantStatus,
  reason?: string,
): Promise<Merchant> {
  const { data } = await apiClient.patch<Merchant>(
    `/admin/merchants/${id}/status`,
    { status, reason },
  )
  return data
}

/** POST /admin/merchants/:id/recharge */
export async function adminMerchantRecharge(
  id: number,
  amount: number,
  reason?: string,
): Promise<MerchantLedgerEntry> {
  const { data } = await apiClient.post<MerchantLedgerEntry>(
    `/admin/merchants/${id}/recharge`,
    { amount, reason },
  )
  return data
}

/** POST /admin/merchants/:id/refund (reason required) */
export async function adminMerchantRefund(
  id: number,
  amount: number,
  reason: string,
): Promise<MerchantLedgerEntry> {
  const { data } = await apiClient.post<MerchantLedgerEntry>(
    `/admin/merchants/${id}/refund`,
    { amount, reason },
  )
  return data
}

/** GET /admin/merchants/:id/group_markups */
export async function adminMerchantListGroupMarkups(id: number): Promise<MerchantGroupMarkup[]> {
  const { data } = await apiClient.get<MerchantGroupMarkup[] | null>(
    `/admin/merchants/${id}/group_markups`,
  )
  return data || []
}

/** PUT /admin/merchants/:id/group_markups — 设商户分组对外售价（admin 代操作） */
export async function adminMerchantSetGroupMarkup(
  id: number,
  group_id: number,
  sell_rate: number,
  reason?: string,
): Promise<MerchantGroupMarkup> {
  const { data } = await apiClient.put<MerchantGroupMarkup>(
    `/admin/merchants/${id}/group_markups`,
    { group_id, sell_rate, reason },
  )
  return data
}

/** DELETE /admin/merchants/:id/group_markups/:group_id */
export async function adminMerchantDeleteGroupMarkup(
  id: number,
  group_id: number,
): Promise<{ message?: string }> {
  const { data } = await apiClient.delete<{ message?: string }>(
    `/admin/merchants/${id}/group_markups/${group_id}`,
  )
  return data || {}
}

/** GET /admin/merchants/:id/group_costs — 列出商户的分组拿货价配置 */
export async function adminMerchantListGroupCosts(id: number): Promise<MerchantGroupCost[]> {
  const { data } = await apiClient.get<MerchantGroupCost[] | null>(
    `/admin/merchants/${id}/group_costs`,
  )
  return data || []
}

/** PUT /admin/merchants/:id/group_costs — admin 设商户分组拿货价 */
export async function adminMerchantSetGroupCost(
  id: number,
  group_id: number,
  cost_rate: number,
  reason?: string,
): Promise<MerchantGroupCost> {
  const { data } = await apiClient.put<MerchantGroupCost>(
    `/admin/merchants/${id}/group_costs`,
    { group_id, cost_rate, reason },
  )
  return data
}

/** DELETE /admin/merchants/:id/group_costs/:group_id */
export async function adminMerchantDeleteGroupCost(
  id: number,
  group_id: number,
): Promise<{ message?: string }> {
  const { data } = await apiClient.delete<{ message?: string }>(
    `/admin/merchants/${id}/group_costs/${group_id}`,
  )
  return data || {}
}

/** GET /admin/merchants/:id/audit_log */
export async function adminMerchantListAuditLog(
  id: number,
  offset = 0,
  limit = 20,
): Promise<PaginatedResponse<MerchantAuditLogEntry>> {
  const { data } = await apiClient.get<PaginatedResponse<MerchantAuditLogEntry> | MerchantAuditLogEntry[]>(
    `/admin/merchants/${id}/audit_log`,
    { params: paginationParams(offset, limit) },
  )
  if (Array.isArray(data)) {
    return {
      items: data,
      total: data.length,
      page: 1,
      page_size: data.length || limit,
      pages: 1,
    }
  }
  return data
}

/** GET /admin/merchants/:id/ledger */
export async function adminMerchantListLedger(
  id: number,
  offset = 0,
  limit = 20,
): Promise<PaginatedResponse<MerchantLedgerEntry>> {
  const { data } = await apiClient.get<PaginatedResponse<MerchantLedgerEntry> | MerchantLedgerEntry[]>(
    `/admin/merchants/${id}/ledger`,
    { params: paginationParams(offset, limit) },
  )
  if (Array.isArray(data)) {
    return {
      items: data,
      total: data.length,
      page: 1,
      page_size: data.length || limit,
      pages: 1,
    }
  }
  return data
}

/** POST /admin/merchants/unbind_user/:user_id */
export async function adminMerchantUnbindUser(
  user_id: number,
  reason?: string,
): Promise<{ message?: string }> {
  const { data } = await apiClient.post<{ message?: string }>(
    `/admin/merchants/unbind_user/${user_id}`,
    { reason },
  )
  return data || {}
}

// ==================== Merchant owner endpoints ====================

/** GET /merchant/info */
export async function merchantInfo(): Promise<MerchantInfo> {
  const { data } = await apiClient.get<MerchantInfo>('/merchant/info')
  return data
}

export interface SubUserSummary {
  id: number
  email: string
  username: string
  balance: number
  status: string
  created_at: string
  last_active_at?: string | null
}

/** GET /merchant/sub_users */
export async function merchantListSubUsers(
  q?: string,
  offset = 0,
  limit = 20,
): Promise<PaginatedResponse<SubUserSummary>> {
  const params: Record<string, string | number> = { ...paginationParams(offset, limit) }
  if (q) params.q = q
  const { data } = await apiClient.get<PaginatedResponse<SubUserSummary> | SubUserSummary[]>(
    '/merchant/sub_users',
    { params },
  )
  if (Array.isArray(data)) {
    return { items: data, total: data.length, page: 1, page_size: data.length || limit, pages: 1 }
  }
  return data
}

/** POST /merchant/pay */
export async function merchantPayToUser(
  sub_user_id: number,
  amount: number,
  reason?: string,
): Promise<MerchantLedgerEntry> {
  const { data } = await apiClient.post<MerchantLedgerEntry>('/merchant/pay', {
    sub_user_id,
    amount,
    reason,
  })
  return data
}

/** POST /merchant/refund — 商户从子用户撤回余额（sub_user.balance → owner.balance） */
export async function merchantRefundFromUser(
  sub_user_id: number,
  amount: number,
  reason?: string,
): Promise<void> {
  await apiClient.post('/merchant/refund', { sub_user_id, amount, reason })
}

/** GET /merchant/ledger */
export async function merchantListLedger(
  offset = 0,
  limit = 20,
): Promise<PaginatedResponse<MerchantLedgerEntry>> {
  const { data } = await apiClient.get<PaginatedResponse<MerchantLedgerEntry> | MerchantLedgerEntry[]>(
    '/merchant/ledger',
    { params: paginationParams(offset, limit) },
  )
  if (Array.isArray(data)) {
    return {
      items: data,
      total: data.length,
      page: 1,
      page_size: data.length || limit,
      pages: 1,
    }
  }
  return data
}

/** GET /merchant/group_markups */
export async function merchantListGroupMarkups(): Promise<MerchantGroupMarkup[]> {
  const { data } = await apiClient.get<MerchantGroupMarkup[] | null>('/merchant/group_markups')
  return data || []
}

/**
 * 商户可定价的分组（v2.0 模型）。
 * - cost_rate: admin 配的拿货价（绝对倍率）；缺失时按 rate_multiplier 兜底
 * - sell_rate: 商户配的对外售价（绝对倍率）；缺失时该 group 不分润，sub_user 按主站价
 */
export interface MerchantPricingGroup {
  id: number
  name: string
  platform: string
  is_exclusive: boolean
  rate_multiplier: number
  cost_rate?: number | null
  sell_rate?: number | null
}

/** GET /merchant/pricing_groups — 商户可定价分组列表（所有 active 非订阅分组） */
export async function merchantListPricingGroups(): Promise<MerchantPricingGroup[]> {
  const { data } = await apiClient.get<MerchantPricingGroup[] | null>('/merchant/pricing_groups')
  return data || []
}

/** PUT /merchant/group_markups — 商户 owner 自助设置某分组对外售价倍率（绝对值，≥ cost_rate） */
export async function merchantSetGroupMarkup(
  group_id: number,
  sell_rate: number,
  reason?: string,
): Promise<void> {
  await apiClient.put('/merchant/group_markups', { group_id, sell_rate, reason })
}

/** DELETE /merchant/group_markups/:group_id — 商户 owner 删除分组售价，回到不分润状态 */
export async function merchantDeleteGroupMarkup(group_id: number, reason?: string): Promise<void> {
  await apiClient.delete(`/merchant/group_markups/${group_id}`, {
    params: reason ? { reason } : undefined,
  })
}

/** GET /merchant/domains */
export async function merchantListDomains(): Promise<MerchantDomain[]> {
  const { data } = await apiClient.get<MerchantDomain[] | null>('/merchant/domains')
  return data || []
}

export interface DomainBrandPayload {
  domain?: string
  site_name?: string
  site_logo?: string
  brand_color?: string
  custom_css?: string
  home_content?: string
  seo_title?: string
  seo_description?: string
  seo_keywords?: string
}

/** POST /merchant/domains */
export async function merchantCreateDomain(payload: DomainBrandPayload): Promise<MerchantDomain> {
  const { data } = await apiClient.post<MerchantDomain>('/merchant/domains', payload)
  return data
}

/** PUT /merchant/domains/:id */
export async function merchantUpdateDomain(
  id: number,
  payload: DomainBrandPayload,
): Promise<MerchantDomain> {
  const { data } = await apiClient.put<MerchantDomain>(`/merchant/domains/${id}`, payload)
  return data
}

/** POST /merchant/domains/:id/verify */
export async function merchantVerifyDomain(id: number): Promise<void> {
  await apiClient.post(`/merchant/domains/${id}/verify`)
}

/** POST /merchant/upload/logo (multipart/form-data, field "file") → { url } */
export async function merchantUploadLogo(file: File): Promise<{ url: string }> {
  const form = new FormData()
  form.append('file', file)
  const { data } = await apiClient.post<{ url: string }>('/merchant/upload/logo', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return data
}

/** DELETE /merchant/domains/:id */
export async function merchantDeleteDomain(id: number): Promise<void> {
  await apiClient.delete(`/merchant/domains/${id}`)
}

export interface DNSSetupInfo {
  server_ip: string
  has_server_ip: boolean
  txt_host_prefix: string
  skip_dns_verify: boolean
}

export interface MerchantStats {
  total_recharge: number
  total_share: number
  withdrawn_amount: number
  pending_withdraw: number
  available_balance: number
}

export interface WithdrawRequest {
  id: number
  merchant_id: number
  amount: number
  status: 'pending' | 'approved' | 'paid' | 'rejected'
  payment_method: string
  payment_account: string
  payment_name: string
  note: string
  admin_id?: number | null
  reject_reason?: string
  ledger_id?: number | null
  created_at: string
  processed_at?: string | null
}

export interface CreateWithdrawPayload {
  amount: number
  payment_method: string
  payment_account: string
  payment_name: string
  note?: string
}

/** GET /merchant/dns_setup */
export async function merchantDNSSetup(): Promise<DNSSetupInfo> {
  const { data } = await apiClient.get<DNSSetupInfo>('/merchant/dns_setup')
  return data
}

/** GET /merchant/stats */
export async function merchantStats(): Promise<MerchantStats> {
  const { data } = await apiClient.get<MerchantStats>('/merchant/stats')
  return data
}

/** GET /merchant/withdrawals */
export async function merchantListWithdrawals(
  status?: string,
  offset = 0,
  limit = 20,
): Promise<PaginatedResponse<WithdrawRequest>> {
  const params: Record<string, string | number> = { ...paginationParams(offset, limit) }
  if (status) params.status = status
  const { data } = await apiClient.get<PaginatedResponse<WithdrawRequest> | WithdrawRequest[]>(
    '/merchant/withdrawals',
    { params },
  )
  if (Array.isArray(data)) {
    return { items: data, total: data.length, page: 1, page_size: data.length || limit, pages: 1 }
  }
  return data
}

/** POST /merchant/withdrawals */
export async function merchantCreateWithdrawal(payload: CreateWithdrawPayload): Promise<WithdrawRequest> {
  const { data } = await apiClient.post<WithdrawRequest>('/merchant/withdrawals', payload)
  return data
}

/** GET /admin/merchant_withdrawals */
export async function adminListWithdrawals(
  status?: string,
  merchantId?: number,
  offset = 0,
  limit = 50,
): Promise<PaginatedResponse<WithdrawRequest>> {
  const params: Record<string, string | number> = { ...paginationParams(offset, limit) }
  if (status) params.status = status
  if (merchantId) params.merchant_id = merchantId
  const { data } = await apiClient.get<PaginatedResponse<WithdrawRequest> | WithdrawRequest[]>(
    '/admin/merchant_withdrawals',
    { params },
  )
  if (Array.isArray(data)) {
    return { items: data, total: data.length, page: 1, page_size: data.length || limit, pages: 1 }
  }
  return data
}

/** POST /admin/merchant_withdrawals/:id/approve */
export async function adminApproveWithdrawal(id: number): Promise<void> {
  await apiClient.post(`/admin/merchant_withdrawals/${id}/approve`)
}

/** POST /admin/merchant_withdrawals/:id/reject */
export async function adminRejectWithdrawal(id: number, reason?: string): Promise<void> {
  await apiClient.post(`/admin/merchant_withdrawals/${id}/reject`, { reason: reason || '' })
}

// 注：owner 端不再开放 audit log（admin 端 /admin/merchants/:id/audit_log 仍保留）。
// 商户关心的资金事件已在 /merchant/ledger 完整展示。

// ==================== Public endpoint ====================

/** GET /merchant_brand */
export async function merchantBrand(): Promise<MerchantBrand> {
  const { data } = await apiClient.get<MerchantBrand>('/merchant_brand')
  return data
}

// ==================== Aggregated export ====================

export const merchantAPI = {
  // admin
  adminList: adminMerchantList,
  adminCreate: adminMerchantCreate,
  adminGet: adminMerchantGet,
  adminStats: adminMerchantStats,
  adminSetStatus: adminMerchantSetStatus,
  adminRecharge: adminMerchantRecharge,
  adminRefund: adminMerchantRefund,
  adminListGroupMarkups: adminMerchantListGroupMarkups,
  adminSetGroupMarkup: adminMerchantSetGroupMarkup,
  adminDeleteGroupMarkup: adminMerchantDeleteGroupMarkup,
  adminListGroupCosts: adminMerchantListGroupCosts,
  adminSetGroupCost: adminMerchantSetGroupCost,
  adminDeleteGroupCost: adminMerchantDeleteGroupCost,
  adminListAuditLog: adminMerchantListAuditLog,
  adminListLedger: adminMerchantListLedger,
  adminUnbindUser: adminMerchantUnbindUser,
  // merchant owner
  info: merchantInfo,
  listSubUsers: merchantListSubUsers,
  payToUser: merchantPayToUser,
  refundFromUser: merchantRefundFromUser,
  listLedger: merchantListLedger,
  listGroupMarkups: merchantListGroupMarkups,
  listPricingGroups: merchantListPricingGroups,
  setGroupMarkup: merchantSetGroupMarkup,
  deleteGroupMarkup: merchantDeleteGroupMarkup,
  listDomains: merchantListDomains,
  createDomain: merchantCreateDomain,
  updateDomain: merchantUpdateDomain,
  verifyDomain: merchantVerifyDomain,
  deleteDomain: merchantDeleteDomain,
  uploadLogo: merchantUploadLogo,
  dnsSetup: merchantDNSSetup,
  stats: merchantStats,
  listWithdrawals: merchantListWithdrawals,
  createWithdrawal: merchantCreateWithdrawal,
  // admin
  adminListWithdrawals: adminListWithdrawals,
  adminApproveWithdrawal: adminApproveWithdrawal,
  adminRejectWithdrawal: adminRejectWithdrawal,
  // public
  brand: merchantBrand,
}

// Export ListParams to satisfy unused-import linters in callers that import the type.
export type { ListParams }

export default merchantAPI
