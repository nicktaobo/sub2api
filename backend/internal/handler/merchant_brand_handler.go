// MERCHANT-SYSTEM v1.0
// MerchantBrandHandler 公开 API：返回当前请求 host 对应的商户品牌信息。
// 前端 bootstrap 时调一次，根据返回值渲染 SEO / 站点名 / 颜色 / 自定义 HTML 等。
//
// RFC §3.4 / §4.1.2 / Phase 4.3 / Phase 4.4。

package handler

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// MerchantBrandHandler 商户品牌信息 handler。
type MerchantBrandHandler struct {
	merchantSvc *service.MerchantService
}

// NewMerchantBrandHandler DI 构造函数。
func NewMerchantBrandHandler(merchantSvc *service.MerchantService) *MerchantBrandHandler {
	return &MerchantBrandHandler{merchantSvc: merchantSvc}
}

type merchantBrandResponse struct {
	IsMerchantSite bool   `json:"is_merchant_site"`
	MerchantID     int64  `json:"merchant_id,omitempty"`
	MerchantName   string `json:"merchant_name,omitempty"`
	Status         string `json:"status,omitempty"`

	Domain         string `json:"domain,omitempty"`
	SiteName       string `json:"site_name,omitempty"`
	SiteLogo       string `json:"site_logo,omitempty"`
	BrandColor     string `json:"brand_color,omitempty"`
	CustomCSS      string `json:"custom_css,omitempty"`
	HomeContent    string `json:"home_content,omitempty"`
	SEOTitle       string `json:"seo_title,omitempty"`
	SEODescription string `json:"seo_description,omitempty"`
	SEOKeywords    string `json:"seo_keywords,omitempty"`
}

// GetCurrent 公开端点：根据 c.Request.Host 返回当前商户品牌信息（DomainDetect 中间件已查好）。
func (h *MerchantBrandHandler) GetCurrent(c *gin.Context) {
	mctx := middleware.MerchantFromContext(c)
	if mctx == nil || mctx.Merchant == nil {
		c.JSON(http.StatusOK, &merchantBrandResponse{IsMerchantSite: false})
		return
	}

	resp := &merchantBrandResponse{
		IsMerchantSite: true,
		MerchantID:     mctx.Merchant.ID,
		MerchantName:   mctx.Merchant.Name,
		Status:         mctx.Merchant.Status,
	}
	if mctx.Domain != nil {
		d := mctx.Domain
		resp.Domain = d.Domain
		resp.SiteName = d.SiteName
		resp.SiteLogo = d.SiteLogo
		resp.BrandColor = d.BrandColor
		resp.CustomCSS = d.CustomCSS
		// home_content 需要在 Phase 4.4 通过 sanitize 输出（v1.0 暂直接返回 raw，前端 v-html 时已用 sanitize 过滤）
		resp.HomeContent = d.HomeContent
		resp.SEOTitle = d.SEOTitle
		resp.SEODescription = d.SEODescription
		resp.SEOKeywords = d.SEOKeywords
	}
	c.JSON(http.StatusOK, resp)
}
