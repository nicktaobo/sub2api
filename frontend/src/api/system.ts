/**
 * Public system endpoints (no auth required).
 */
import { apiClient } from './client'

export interface FXRate {
  cny_per_usd: number
  last_updated: string | null
}

/** GET /settings/fx-rate — current CNY/USD rate (refreshed hourly on backend). */
export async function getFXRate(): Promise<FXRate> {
  const { data } = await apiClient.get<FXRate>('/settings/fx-rate')
  return data
}

export const systemAPI = { getFXRate }
export default systemAPI
