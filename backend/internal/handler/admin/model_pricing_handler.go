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

// List 返回当前覆盖表。GET /api/v1/admin/model-pricing
func (h *ModelPricingHandler) List(c *gin.Context) {
	cfg, err := h.pricingService.GetModelPricingOverrides(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
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
