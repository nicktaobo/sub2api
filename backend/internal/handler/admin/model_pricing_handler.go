package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ModelPricingHandler 管理“通用模型价格覆盖表”（独立于渠道管理定价）。
type ModelPricingHandler struct {
	pricingService *service.PricingService
}

func NewModelPricingHandler(pricingService *service.PricingService) *ModelPricingHandler {
	return &ModelPricingHandler{pricingService: pricingService}
}

// List 返回覆盖表 + 内置全量表(只读)。GET /api/v1/admin/model-pricing
// entries=可编辑覆盖项;builtin=Claude/GPT/Gemini/国产 等内置默认价(只读展示)。
func (h *ModelPricingHandler) List(c *gin.Context) {
	cfg, err := h.pricingService.GetModelPricingView(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

// RefreshPreview 拉取远程最新价表与当前内置表做 diff,返回预览(不落库)。
// POST /api/v1/admin/model-pricing/refresh/preview
func (h *ModelPricingHandler) RefreshPreview(c *gin.Context) {
	preview, err := h.pricingService.PreviewRemotePricing(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, preview)
}

// RefreshApply 管理员确认后落库刷新内置全量表(走 ForceUpdate 同步链路)。
// POST /api/v1/admin/model-pricing/refresh/apply
//
// 注:ForceUpdate 内部用自带的 30s background context(见 downloadPricingData),
// 刻意不绑定请求 ctx——价表落盘是原子写,不应因管理员浏览器断连而中途取消。
func (h *ModelPricingHandler) RefreshApply(c *gin.Context) {
	if err := h.pricingService.ForceUpdate(); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, h.pricingService.GetStatus())
}

// Update 整表覆盖写入并即时生效。PUT /api/v1/admin/model-pricing
func (h *ModelPricingHandler) Update(c *gin.Context) {
	var req service.ModelPricingOverridesDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	cfg, err := h.pricingService.UpdateModelPricingOverrides(c.Request.Context(), &req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}
