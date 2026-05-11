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
  discount: number
  user_markup_default: number
  low_balance_threshold: number
  notify_emails: NotifyEmailEntry[] | string[] | null
  balance_baseline?: number
  owner_balance?: number
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
  markup: number
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
  actor_user_id: number
  actor_email?: string | null
  actor_username?: string | null
  action: string
  reason?: string | null
  before?: Record<string, unknown> | null
  after?: Record<string, unknown> | null
  metadata?: Record<string, unknown> | null
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
  discount: number
  user_markup_default: number
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
): Promise<PaginatedResponse<Merchant>> {
  const params: Record<string, string | number> = { ...paginationParams(offset, limit) }
  if (status) params.status = status
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

/** PATCH /admin/merchants/:id/discount */
export async function adminMerchantSetDiscount(
  id: number,
  discount: number,
  reason?: string,
): Promise<Merchant> {
  const { data } = await apiClient.patch<Merchant>(
    `/admin/merchants/${id}/discount`,
    { discount, reason },
  )
  return data
}

/** PATCH /admin/merchants/:id/markup_default */
export async function adminMerchantSetMarkupDefault(
  id: number,
  markup: number,
  reason?: string,
): Promise<Merchant> {
  const { data } = await apiClient.patch<Merchant>(
    `/admin/merchants/${id}/markup_default`,
    { user_markup_default: markup, markup, reason },
  )
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

/** PUT /admin/merchants/:id/group_markups */
export async function adminMerchantSetGroupMarkup(
  id: number,
  group_id: number,
  markup: number,
  reason?: string,
): Promise<MerchantGroupMarkup> {
  const { data } = await apiClient.put<MerchantGroupMarkup>(
    `/admin/merchants/${id}/group_markups`,
    { group_id, markup, reason },
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

/** DELETE /merchant/domains/:id */
export async function merchantDeleteDomain(id: number): Promise<void> {
  await apiClient.delete(`/merchant/domains/${id}`)
}

/** GET /merchant/audit_log */
export async function merchantListAuditLog(
  offset = 0,
  limit = 20,
): Promise<PaginatedResponse<MerchantAuditLogEntry>> {
  const { data } = await apiClient.get<PaginatedResponse<MerchantAuditLogEntry> | MerchantAuditLogEntry[]>(
    '/merchant/audit_log',
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
  adminSetDiscount: adminMerchantSetDiscount,
  adminSetMarkupDefault: adminMerchantSetMarkupDefault,
  adminSetStatus: adminMerchantSetStatus,
  adminRecharge: adminMerchantRecharge,
  adminRefund: adminMerchantRefund,
  adminListGroupMarkups: adminMerchantListGroupMarkups,
  adminSetGroupMarkup: adminMerchantSetGroupMarkup,
  adminDeleteGroupMarkup: adminMerchantDeleteGroupMarkup,
  adminListAuditLog: adminMerchantListAuditLog,
  adminListLedger: adminMerchantListLedger,
  adminUnbindUser: adminMerchantUnbindUser,
  // merchant owner
  info: merchantInfo,
  payToUser: merchantPayToUser,
  listLedger: merchantListLedger,
  listGroupMarkups: merchantListGroupMarkups,
  listDomains: merchantListDomains,
  createDomain: merchantCreateDomain,
  updateDomain: merchantUpdateDomain,
  verifyDomain: merchantVerifyDomain,
  deleteDomain: merchantDeleteDomain,
  listAuditLog: merchantListAuditLog,
  // public
  brand: merchantBrand,
}

// Export ListParams to satisfy unused-import linters in callers that import the type.
export type { ListParams }

export default merchantAPI
