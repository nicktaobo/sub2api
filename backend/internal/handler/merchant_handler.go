// MERCHANT-SYSTEM v1.0
// MerchantHandler 商户 owner 自服务 API（需 JWT 认证 + 商户身份校验）。

package handler

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
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

func (h *MerchantHandler) resolveOwnerMerchant(c *gin.Context) *service.Merchant {
	userID, ok := jwtUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return nil
	}
	m, err := h.merchantSvc.GetByOwnerUserID(c.Request.Context(), userID)
	if err != nil || m == nil {
		response.Forbidden(c, "not a merchant owner")
		return nil
	}
	return m
}

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

func (h *MerchantHandler) GetInfo(c *gin.Context) {
	m := h.resolveOwnerMerchant(c)
	if m == nil {
		return
	}
	response.Success(c, gin.H{
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
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.merchantSvc.PayToUser(c.Request.Context(), m.ID, req.SubUserID, req.Amount, 0, req.Reason); err != nil {
		if !response.ErrorFrom(c, err) {
			response.Error(c, http.StatusBadRequest, err.Error())
		}
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func (h *MerchantHandler) ListLedger(c *gin.Context) {
	m := h.resolveOwnerMerchant(c)
	if m == nil {
		return
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	rows, total, err := h.merchantSvc.ListLedger(c.Request.Context(), m.ID, offset, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, paginatedOwner(rows, total, offset, limit))
}

func (h *MerchantHandler) ListGroupMarkups(c *gin.Context) {
	m := h.resolveOwnerMerchant(c)
	if m == nil {
		return
	}
	rows, err := h.merchantSvc.ListGroupMarkups(c.Request.Context(), m.ID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if rows == nil {
		rows = []*service.MerchantGroupMarkup{}
	}
	response.Success(c, rows)
}

func (h *MerchantHandler) ListDomains(c *gin.Context) {
	m := h.resolveOwnerMerchant(c)
	if m == nil {
		return
	}
	rows, err := h.merchantSvc.ListDomains(c.Request.Context(), m.ID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if rows == nil {
		rows = []*service.MerchantDomain{}
	}
	response.Success(c, rows)
}

func (h *MerchantHandler) ListAuditLog(c *gin.Context) {
	m := h.resolveOwnerMerchant(c)
	if m == nil {
		return
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	rows, total, err := h.merchantSvc.ListAuditLog(c.Request.Context(), m.ID, offset, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, paginatedOwner(rows, total, offset, limit))
}

// paginatedOwner 与 admin.paginated 同形态。这里复制一份避免跨包依赖。
func paginatedOwner[T any](rows []T, total int, offset, limit int) gin.H {
	if limit <= 0 {
		limit = 50
	}
	page := offset/limit + 1
	pages := 1
	if total > 0 {
		pages = (total + limit - 1) / limit
	}
	return gin.H{
		"items":     rows,
		"total":     total,
		"page":      page,
		"page_size": limit,
		"pages":     pages,
	}
}
