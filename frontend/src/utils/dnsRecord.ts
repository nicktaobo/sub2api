/**
 * DNS 记录 HOST 计算。
 *
 * 绝大多数 DNS 服务商控制台里的 HOST/主机记录字段是**相对于解析区域（zone）**的：
 *   - 根域 example.com        → A 记录 HOST 填 `@`
 *   - 子域 ai.example.com     → A 记录 HOST 填 `ai`（填 `@` 会把根域指错，是危险操作）
 * TXT 同理：`_domain-verify.ai.example.com` 在区域 example.com 下应填 `_domain-verify.ai`。
 *
 * 判断「哪一段是注册域」需要 public suffix list；这里用常见多段后缀白名单做
 * best-effort 判断，并同时给出完整 FQDN，让用户按自家服务商的要求二选一。
 */

/** 常见的多段公共后缀（不完整，够覆盖商户自带域名的绝大多数场景）。 */
const MULTI_LABEL_SUFFIXES = new Set([
  'co.uk', 'org.uk', 'me.uk', 'ac.uk', 'gov.uk',
  'com.cn', 'net.cn', 'org.cn', 'gov.cn', 'edu.cn', 'ac.cn',
  'com.hk', 'org.hk', 'com.tw', 'com.sg', 'com.my',
  'com.au', 'net.au', 'org.au',
  'com.br', 'com.mx', 'com.ar', 'com.co',
  'co.jp', 'or.jp', 'ne.jp', 'ac.jp',
  'co.kr', 'or.kr',
  'co.nz', 'co.za', 'co.in', 'com.tr', 'com.ua', 'com.ru',
])

/** normalizeDomain 统一小写、去首尾空白与结尾的点。 */
export function normalizeDomain(domain: string): string {
  return (domain || '').trim().toLowerCase().replace(/\.+$/, '')
}

/**
 * registrableDomain 返回域名所属的解析区域（注册域）。
 * example.com → example.com；ai.example.com → example.com；a.b.co.uk → b.co.uk
 */
export function registrableDomain(domain: string): string {
  const d = normalizeDomain(domain)
  if (!d) return ''
  const labels = d.split('.')
  if (labels.length <= 2) return d
  const lastTwo = labels.slice(-2).join('.')
  if (MULTI_LABEL_SUFFIXES.has(lastTwo)) {
    return labels.slice(-3).join('.')
  }
  return lastTwo
}

export interface DnsRecordHosts {
  /** 解析区域（用户应该在这个域下添加记录）。 */
  zone: string
  /** 是否是根域（HOST 填 @）。 */
  isApex: boolean
  /** A 记录 HOST：根域为 `@`，子域为子域前缀。 */
  aHost: string
  /** A 记录的完整域名写法（部分服务商要求填完整域名）。 */
  aFqdn: string
  /** TXT 记录 HOST（相对区域）。 */
  txtHost: string
  /** TXT 记录完整域名写法（后端实际查询的名字）。 */
  txtFqdn: string
}

/**
 * dnsRecordHosts 计算接入某域名所需的 A / TXT 记录 HOST。
 * txtPrefix 默认 `_domain-verify`，与后端 VerifyDomain 查询的名字保持一致。
 */
export function dnsRecordHosts(domain: string, txtPrefix = '_domain-verify'): DnsRecordHosts {
  const d = normalizeDomain(domain)
  const zone = registrableDomain(d)
  const isApex = !d || d === zone
  // 子域前缀 = 去掉 ".<zone>" 后剩下的部分
  const sub = isApex ? '' : d.slice(0, Math.max(0, d.length - zone.length - 1))
  return {
    zone,
    isApex,
    aHost: isApex ? '@' : sub,
    aFqdn: d,
    txtHost: isApex ? txtPrefix : `${txtPrefix}.${sub}`,
    txtFqdn: d ? `${txtPrefix}.${d}` : txtPrefix,
  }
}
