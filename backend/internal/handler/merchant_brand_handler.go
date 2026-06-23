// MERCHANT-SYSTEM v1.0
// MerchantBrandHandler 公开 API：返回当前请求 host 对应的商户品牌信息。
// 前端 bootstrap 时调一次，根据返回值渲染 SEO / 站点名 / 颜色 / 自定义 HTML 等。

package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type MerchantBrandHandler struct {
	merchantSvc *service.MerchantService
	settingSvc  *service.SettingService
}

func NewMerchantBrandHandler(merchantSvc *service.MerchantService, settingSvc *service.SettingService) *MerchantBrandHandler {
	return &MerchantBrandHandler{merchantSvc: merchantSvc, settingSvc: settingSvc}
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

func (h *MerchantBrandHandler) GetCurrent(c *gin.Context) {
	mctx := middleware.MerchantFromContext(c)
	if mctx == nil || mctx.Merchant == nil {
		response.Success(c, &merchantBrandResponse{IsMerchantSite: false})
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
		resp.HomeContent = d.HomeContent
		resp.SEOTitle = d.SEOTitle
		resp.SEODescription = d.SEODescription
		resp.SEOKeywords = d.SEOKeywords
	}

	// 商户没自定义 home_content 时，回退主站设置里的 home_content，避免分站首页空白
	if strings.TrimSpace(resp.HomeContent) == "" && h.settingSvc != nil {
		resp.HomeContent = h.settingSvc.GetHomeContent(c.Request.Context())
	}

	response.Success(c, resp)
}
