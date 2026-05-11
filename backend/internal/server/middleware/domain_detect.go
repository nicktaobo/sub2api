// MERCHANT-SYSTEM v1.0
// DomainDetect 中间件：识别请求 host 是否属于某个 verified 商户域名，
// 若是则注入 *MerchantContext 到 gin.Context，供后续 handler / 登录域校验使用。
//
// RFC §3.4 / §4.1.2 / Phase 4.1。

package middleware

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// MerchantContext 是 service.MerchantContext 的本地别名，便于现有代码使用 middleware.MerchantContext。
//
// 实际定义在 service 包（含 context.Context 传播 helper），避免循环 import。
type MerchantContext = service.MerchantContext

// merchantDomainCache 简单的 TTL 缓存：domain → *MerchantContext。
type merchantDomainCache struct {
	mu      sync.RWMutex
	entries map[string]merchantDomainCacheEntry
	ttl     time.Duration
}

type merchantDomainCacheEntry struct {
	ctx       *MerchantContext
	expiresAt time.Time
}

func newMerchantDomainCache(ttl time.Duration) *merchantDomainCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &merchantDomainCache{entries: make(map[string]merchantDomainCacheEntry), ttl: ttl}
}

func (c *merchantDomainCache) Get(host string) (*MerchantContext, bool) {
	c.mu.RLock()
	e, ok := c.entries[host]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.ctx, true
}

func (c *merchantDomainCache) Set(host string, ctx *MerchantContext) {
	c.mu.Lock()
	c.entries[host] = merchantDomainCacheEntry{ctx: ctx, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

// Invalidate 清掉某个 host 的缓存（merchant 改 domain / status 时调）。
func (c *merchantDomainCache) Invalidate(host string) {
	c.mu.Lock()
	delete(c.entries, strings.ToLower(strings.TrimSpace(host)))
	c.mu.Unlock()
}

// DomainDetectMiddleware 创建 DomainDetect 中间件。
//
// 在 feature flag 关闭时短路（直接 c.Next()）。
// 找到 verified 商户域名时注入 *MerchantContext；否则不注入（普通主站）。
//
// 注意：使用 c.Request.Host 作为查找键（不含 scheme），并 trim 端口。
func DomainDetectMiddleware(cfg *config.Config, merchantSvc *service.MerchantService) gin.HandlerFunc {
	cache := newMerchantDomainCache(5 * time.Minute)
	return func(c *gin.Context) {
		if cfg == nil || !cfg.Merchant.Enabled || merchantSvc == nil {
			c.Next()
			return
		}
		host := stripPort(strings.ToLower(c.Request.Host))
		if host == "" {
			c.Next()
			return
		}
		mctx, ok := cache.Get(host)
		if !ok {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 500*time.Millisecond)
			m, err := merchantSvc.GetByDomain(ctx, host)
			if err == nil && m != nil {
				mctx = &MerchantContext{Merchant: m}
				if d, derr := merchantSvc.GetDomain(ctx, host); derr == nil && d != nil {
					mctx.Domain = d
				}
			}
			cancel()
			cache.Set(host, mctx) // 缓存找不到的结果（避免每次查 DB）
		}
		if mctx != nil && mctx.Merchant != nil {
			c.Set(string(ContextKeyMerchant), mctx)
			// 同时把商户上下文注入 *http.Request 的 context.Context，让 service/repo
			// 层在仅持有 ctx 的场景（user 创建、邮件发送等）也能取到。
			c.Request = c.Request.WithContext(service.WithMerchantInGoContext(c.Request.Context(), mctx))
		}
		c.Next()
	}
}

// MerchantFromContext 从 gin.Context 取当前商户上下文。
// 普通主站（无商户域名）返回 nil。
func MerchantFromContext(c *gin.Context) *MerchantContext {
	if c == nil {
		return nil
	}
	v, exists := c.Get(string(ContextKeyMerchant))
	if !exists {
		return nil
	}
	mctx, _ := v.(*MerchantContext)
	return mctx
}

// stripPort 去掉 host 的端口部分（"foo.com:8080" → "foo.com"）。
func stripPort(host string) string {
	if i := strings.LastIndex(host, ":"); i >= 0 {
		// 不能简单截：可能是 IPv6（含多个冒号），但 modelboxs 商户域名一律不可能是 IPv6
		// 判断 i 之后是否纯数字
		port := host[i+1:]
		allDigits := port != ""
		for _, r := range port {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return host[:i]
		}
	}
	return host
}

