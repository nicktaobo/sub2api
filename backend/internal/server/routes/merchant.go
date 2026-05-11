// MERCHANT-SYSTEM v1.0
// 商户系统路由：
//   - 公开品牌信息（基于 DomainDetect 中间件识别 host）
//   - JWT 受保护的商户 owner 自服务（/api/v1/merchant/*）

package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterMerchantBrandRoute 注册公开品牌信息 endpoint。
// 不需要 auth；DomainDetect 中间件已在 router 全局应用。
func RegisterMerchantBrandRoute(v1 *gin.RouterGroup, h *handler.Handlers) {
	if h == nil || h.MerchantBrand == nil {
		return
	}
	v1.GET("/merchant_brand", h.MerchantBrand.GetCurrent)
}

// RegisterMerchantOwnerRoutes 注册商户 owner 自服务路由（JWT auth）。
func RegisterMerchantOwnerRoutes(v1 *gin.RouterGroup, h *handler.Handlers, jwtAuth middleware.JWTAuthMiddleware) {
	if h == nil || h.Merchant == nil {
		return
	}
	mh := h.Merchant
	g := v1.Group("/merchant")
	g.Use(gin.HandlerFunc(jwtAuth))
	{
		g.GET("/info", mh.GetInfo)
		g.GET("/stats", mh.GetStats)
		g.GET("/sub_users", mh.ListSubUsers)
		g.POST("/pay", mh.PayToUser)
		g.GET("/ledger", mh.ListLedger)
		g.GET("/group_markups", mh.ListGroupMarkups)
		g.GET("/pricing_groups", mh.ListPricingGroups)
		g.PUT("/markup_default", mh.SetMarkupDefault)
		g.PUT("/group_markups", mh.SetGroupMarkup)
		g.DELETE("/group_markups/:group_id", mh.DeleteGroupMarkup)
		g.GET("/domains", mh.ListDomains)
		g.POST("/domains", mh.CreateDomain)
		g.PUT("/domains/:id", mh.UpdateDomain)
		g.POST("/domains/:id/verify", mh.VerifyDomain)
		g.DELETE("/domains/:id", mh.DeleteDomain)
		g.GET("/dns_setup", mh.DNSSetupInfo)
		g.GET("/withdrawals", mh.ListWithdrawals)
		g.POST("/withdrawals", mh.CreateWithdraw)
		if h.MerchantLogo != nil {
			g.POST("/upload/logo", h.MerchantLogo.Upload)
		}
	}
}

// RegisterMerchantAssetRoute 注册公开的商户静态资产路径（logo 等）。
// 顶层挂载（不在 /api/v1 下），无需 auth，前端 <img src> 直接引用。
func RegisterMerchantAssetRoute(r *gin.Engine, h *handler.Handlers) {
	if r == nil || h == nil || h.MerchantLogo == nil {
		return
	}
	r.GET("/merchant-assets/:merchant_id/:filename", h.MerchantLogo.Serve)
}
