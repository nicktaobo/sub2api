// MERCHANT-SYSTEM v1.0
// admin.MerchantHandler 平台管理员的商户管理 API。

package admin

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
}

func NewMerchantHandler(merchantSvc *service.MerchantService) *MerchantHandler {
	return &MerchantHandler{merchantSvc: merchantSvc}
}

func adminID(c *gin.Context) int64 {
	v, ok := c.Get(string(middleware.ContextKeyUser))
	if !ok {
		return 0
	}
	switch t := v.(type) {
	case middleware.AuthSubject:
		return t.UserID
	case *middleware.AuthSubject:
		return t.UserID
	}
	return 0
}

func writeError(c *gin.Context, err error, fallbackStatus int) {
	if !response.ErrorFrom(c, err) {
		response.Error(c, fallbackStatus, err.Error())
	}
}

// paginated 把 (rows, total, offset, limit) 包装成前端 PaginatedResponse 形态。
func paginated[T any](rows []T, total int, offset, limit int) gin.H {
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

// ----------------------------------------------------------------------------
// List / Create / Get
// ----------------------------------------------------------------------------

func (h *MerchantHandler) List(c *gin.Context) {
	status := c.Query("status")
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	rows, total, err := h.merchantSvc.List(c.Request.Context(), status, offset, limit)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	response.Success(c, paginated(rows, total, offset, limit))
}

type createMerchantReq struct {
	OwnerUserID         int64    `json:"owner_user_id" binding:"required"`
	Name                string   `json:"name" binding:"required"`
	Discount            float64  `json:"discount"`
	UserMarkupDefault   float64  `json:"user_markup_default"`
	LowBalanceThreshold float64  `json:"low_balance_threshold"`
	NotifyEmails        []string `json:"notify_emails"`
	Reason              string   `json:"reason"`
}

func (h *MerchantHandler) Create(c *gin.Context) {
	var req createMerchantReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if req.Discount == 0 {
		req.Discount = 1.0
	}
	if req.UserMarkupDefault == 0 {
		req.UserMarkupDefault = 1.0
	}
	m, err := h.merchantSvc.CreateMerchant(c.Request.Context(), service.CreateMerchantInput{
		OwnerUserID:         req.OwnerUserID,
		Name:                req.Name,
		Discount:            req.Discount,
		UserMarkupDefault:   req.UserMarkupDefault,
		LowBalanceThreshold: req.LowBalanceThreshold,
		NotifyEmails:        req.NotifyEmails,
		AdminID:             adminID(c),
		Reason:              req.Reason,
	})
	if err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	response.Success(c, m)
}

func (h *MerchantHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	m, err := h.merchantSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		writeError(c, err, http.StatusNotFound)
		return
	}
	response.Success(c, m)
}

// ----------------------------------------------------------------------------
// Discount / Markup / Status / Recharge / Refund
// ----------------------------------------------------------------------------

type setDiscountReq struct {
	Discount float64 `json:"discount" binding:"required"`
	Reason   string  `json:"reason"`
}

func (h *MerchantHandler) SetDiscount(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	var req setDiscountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.merchantSvc.SetDiscount(c.Request.Context(), id, req.Discount, adminID(c), req.Reason); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

type setMarkupReq struct {
	Markup float64 `json:"markup" binding:"required"`
	Reason string  `json:"reason"`
}

func (h *MerchantHandler) SetMarkupDefault(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	var req setMarkupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.merchantSvc.SetMarkupDefault(c.Request.Context(), id, req.Markup, adminID(c), req.Reason); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

type setStatusReq struct {
	Status string `json:"status" binding:"required"`
	Reason string `json:"reason"`
}

func (h *MerchantHandler) SetStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	var req setStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.merchantSvc.UpdateStatus(c.Request.Context(), id, req.Status, adminID(c), req.Reason); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

type rechargeReq struct {
	Amount float64 `json:"amount" binding:"required"`
	Reason string  `json:"reason"`
}

func (h *MerchantHandler) Recharge(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	var req rechargeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.merchantSvc.AdminRecharge(c.Request.Context(), id, req.Amount, adminID(c), req.Reason); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func (h *MerchantHandler) Refund(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	var req rechargeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.merchantSvc.AdminRefund(c.Request.Context(), id, req.Amount, adminID(c), req.Reason); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// ----------------------------------------------------------------------------
// Group markup
// ----------------------------------------------------------------------------

func (h *MerchantHandler) ListGroupMarkups(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	rows, err := h.merchantSvc.ListGroupMarkups(c.Request.Context(), id)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	if rows == nil {
		rows = []*service.MerchantGroupMarkup{}
	}
	response.Success(c, rows)
}

type setGroupMarkupReq struct {
	GroupID int64   `json:"group_id" binding:"required"`
	Markup  float64 `json:"markup" binding:"required"`
	Reason  string  `json:"reason"`
}

func (h *MerchantHandler) SetGroupMarkup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	var req setGroupMarkupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.merchantSvc.SetGroupMarkup(c.Request.Context(), id, req.GroupID, req.Markup, adminID(c), req.Reason); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func (h *MerchantHandler) DeleteGroupMarkup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	groupID, err := strconv.ParseInt(c.Param("group_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid group_id")
		return
	}
	if err := h.merchantSvc.DeleteGroupMarkup(c.Request.Context(), id, groupID, adminID(c), c.Query("reason")); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// ----------------------------------------------------------------------------
// Audit log + Ledger + Unbind
// ----------------------------------------------------------------------------

func (h *MerchantHandler) ListAuditLog(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	rows, total, err := h.merchantSvc.ListAuditLog(c.Request.Context(), id, offset, limit)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	response.Success(c, paginated(rows, total, offset, limit))
}

func (h *MerchantHandler) ListLedger(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	rows, total, err := h.merchantSvc.ListLedger(c.Request.Context(), id, offset, limit)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	response.Success(c, paginated(rows, total, offset, limit))
}

type unbindReq struct {
	Reason string `json:"reason"`
}

func (h *MerchantHandler) UnbindSubUser(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid user_id")
		return
	}
	var req unbindReq
	_ = c.ShouldBindJSON(&req)
	if err := h.merchantSvc.UnbindSubUser(c.Request.Context(), userID, adminID(c), req.Reason); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	response.Success(c, gin.H{"ok": true})
}
