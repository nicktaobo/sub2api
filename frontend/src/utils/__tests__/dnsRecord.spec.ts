import { describe, expect, it } from 'vitest'
import { dnsRecordHosts, normalizeDomain, registrableDomain } from '@/utils/dnsRecord'

describe('normalizeDomain', () => {
  it('lowercases, trims and strips trailing dots', () => {
    expect(normalizeDomain('  AI.Fighting.WIN. ')).toBe('ai.fighting.win')
  })
})

describe('registrableDomain', () => {
  it('returns the domain itself for an apex domain', () => {
    expect(registrableDomain('fighting.win')).toBe('fighting.win')
  })

  it('strips the subdomain', () => {
    expect(registrableDomain('ai.fighting.win')).toBe('fighting.win')
    expect(registrableDomain('a.b.example.com')).toBe('example.com')
  })

  it('handles multi-label public suffixes', () => {
    expect(registrableDomain('example.co.uk')).toBe('example.co.uk')
    expect(registrableDomain('ai.example.co.uk')).toBe('example.co.uk')
    expect(registrableDomain('ai.example.com.cn')).toBe('example.com.cn')
  })
})

describe('dnsRecordHosts', () => {
  it('uses @ for an apex domain', () => {
    const r = dnsRecordHosts('fighting.win')
    expect(r.isApex).toBe(true)
    expect(r.zone).toBe('fighting.win')
    expect(r.aHost).toBe('@')
    expect(r.txtHost).toBe('_domain-verify')
    expect(r.txtFqdn).toBe('_domain-verify.fighting.win')
  })

  // 这是线上踩到的 bug：二级域名却让用户解析 @，会把根域指向服务器。
  it('uses the subdomain label for a subdomain, never @', () => {
    const r = dnsRecordHosts('ai.fighting.win')
    expect(r.isApex).toBe(false)
    expect(r.zone).toBe('fighting.win')
    expect(r.aHost).toBe('ai')
    expect(r.aFqdn).toBe('ai.fighting.win')
    expect(r.txtHost).toBe('_domain-verify.ai')
    // 后端 VerifyDomain 查询的就是这个 FQDN
    expect(r.txtFqdn).toBe('_domain-verify.ai.fighting.win')
  })

  it('handles deep subdomains', () => {
    const r = dnsRecordHosts('a.b.example.com')
    expect(r.aHost).toBe('a.b')
    expect(r.txtHost).toBe('_domain-verify.a.b')
  })

  it('handles subdomain under a multi-label suffix', () => {
    const r = dnsRecordHosts('ai.example.co.uk')
    expect(r.zone).toBe('example.co.uk')
    expect(r.aHost).toBe('ai')
    expect(r.txtHost).toBe('_domain-verify.ai')
  })

  it('respects a custom txt prefix', () => {
    expect(dnsRecordHosts('ai.fighting.win', '_verify').txtHost).toBe('_verify.ai')
  })
})
