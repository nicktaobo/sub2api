// MERCHANT-SYSTEM v1.0
// MerchantHandler 商户 owner 自服务 API（需 JWT 认证 + 商户身份校验）。
// 提供：仪表盘 / 子用户管理 / 给子用户充值（PayToUser）/ 流水 / 域名管理 / 分组定价查看。
//
// 路由：/api/v1/merchant/*
// RFC §5.2 / Phase 5.2。

package handler

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type MerchantHandler struct {
	merchantSvc *service.MerchantService
	userSvc     *service.UserService
}

func NewMerchantHandler(merchantSvc *service.MerchantService, userSvc *service.UserService) *MerchantHandler {
	return &MerchantHandler{merchantSvc: merchantSvc, userSvc: userSvc}
}

// resolveOwnerMerchant 取当前 JWT 用户对应的商户。失败时返回 nil + 已写入响应。
func (h *MerchantHandler) resolveOwnerMerchant(c *gin.Context) *service.Merchant {
	userID, ok := jwtUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil
	}
	m, err := h.merchantSvc.GetByOwnerUserID(c.Request.Context(), userID)
	if err != nil || m == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "not_a_merchant_owner"})
		return nil
	}
	return m
}

// jwtUserID 从 gin.Context 取当前 JWT 用户 id（middleware 已注入）。
func jwtUserID(c *gin.Context) (int64, bool) {
	v, ok := c.Get(string(middleware.ContextKeyUser))
	if !ok {
		return 0, false
	}
	switch t := v.(type) {
	case middleware.AuthSubject:
		return t.UserID, true
	case *middleware.AuthSubject:
		return t.UserID, true
	}
	return 0, false
}

// ----------------------------------------------------------------------------
// GET /api/v1/merchant/info — 商户基本信息
// ----------------------------------------------------------------------------

func (h *MerchantHandler) GetInfo(c *gin.Context) {
	m := h.resolveOwnerMerchant(c)
	if m == nil {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":                     m.ID,
		"name":                   m.Name,
		"status":                 m.Status,
		"discount":               m.Discount,
		"user_markup_default":    m.UserMarkupDefault,
		"owner_balance_baseline": m.OwnerBalanceBaseline,
		"low_balance_threshold":  m.LowBalanceThreshold,
		"created_at":             m.CreatedAt,
	})
}

// ----------------------------------------------------------------------------
// POST /api/v1/merchant/pay — owner→sub_user 转账
// ----------------------------------------------------------------------------

type payToUserReq struct {
	SubUserID int64   `json:"sub_user_id" binding:"required"`
	Amount    float64 `json:"amount" binding:"required"`
	Reason    string  `json:"reason"`
}

func (h *MerchantHandler) PayToUser(c *gin.Context) {
	m := h.resolveOwnerMerchant(c)
	if m == nil {
		return
	}
	var req payToUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.merchantSvc.PayToUser(c.Request.Context(), m.ID, req.SubUserID, req.Amount, 0, req.Reason); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ----------------------------------------------------------------------------
// GET /api/v1/merchant/ledger — 商户流水
// ----------------------------------------------------------------------------

func (h *MerchantHandler) ListLedger(c *gin.Context) {
	m := h.resolveOwnerMerchant(c)
	if m == nil {
		return
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	rows, total, err := h.merchantSvc.ListLedger(c.Request.Context(), m.ID, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rows": rows, "total": total})
}

// ----------------------------------------------------------------------------
// GET /api/v1/merchant/group_markups — 商户分组定价（只读）
// ----------------------------------------------------------------------------

func (h *MerchantHandler) ListGroupMarkups(c *gin.Context) {
	m := h.resolveOwnerMerchant(c)
	if m == nil {
		return
	}
	rows, err := h.merchantSvc.ListGroupMarkups(c.Request.Context(), m.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"default_markup": m.UserMarkupDefault,
		"overrides":      rows,
	})
}

// ----------------------------------------------------------------------------
// GET /api/v1/merchant/domains — 列出商户域名
// ----------------------------------------------------------------------------

func (h *MerchantHandler) ListDomains(c *gin.Context) {
	m := h.resolveOwnerMerchant(c)
	if m == nil {
		return
	}
	rows, err := h.merchantSvc.ListDomains(c.Request.Context(), m.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rows": rows})
}

// ----------------------------------------------------------------------------
// GET /api/v1/merchant/audit_log — 配置变更审计
// ----------------------------------------------------------------------------

func (h *MerchantHandler) ListAuditLog(c *gin.Context) {
	m := h.resolveOwnerMerchant(c)
	if m == nil {
		return
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	rows, total, err := h.merchantSvc.ListAuditLog(c.Request.Context(), m.ID, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rows": rows, "total": total})
}
