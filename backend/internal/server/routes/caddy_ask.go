// MERCHANT-SYSTEM v1.0
// Caddy on_demand_tls ask endpoint：让 Caddy 在为商户自定义域名签发 LE 证书前，
// 反查后端确认该域名是否为 verified 的商户域名。命中返回 200，其余 404。
//
// 仅允许 loopback 来源调用（Caddy 与 sub2api 同机部署）。

package routes

import (
	"net"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterCaddyAskRoute 注册 /internal/caddy/ask，挂在 gin.Engine 顶层（不进 /api/v1）。
func RegisterCaddyAskRoute(r *gin.Engine, cfg *config.Config, merchantSvc *service.MerchantService) {
	if merchantSvc == nil {
		return
	}
	r.GET("/internal/caddy/ask", func(c *gin.Context) {
		// 只接受本机回环来源（Caddy 同机调用）
		host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
		if err != nil {
			host = c.Request.RemoteAddr
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			c.Status(http.StatusForbidden)
			return
		}

		// 商户系统未启用时一律 404，避免 Caddy 因残留记录乱签证
		if cfg == nil || !cfg.Merchant.Enabled {
			c.Status(http.StatusNotFound)
			return
		}

		domain := strings.TrimSpace(strings.ToLower(c.Query("domain")))
		if domain == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		d, _ := merchantSvc.GetDomain(c.Request.Context(), domain)
		if d == nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Status(http.StatusOK)
	})
}
