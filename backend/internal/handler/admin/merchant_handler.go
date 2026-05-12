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
	search := c.Query("q")
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	rows, total, err := h.merchantSvc.ListWithDetails(c.Request.Context(), status, search, offset, limit)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	response.Success(c, paginated(rows, total, offset, limit))
}

type createMerchantReq struct {
	OwnerUserID         int64    `json:"owner_user_id" binding:"required"`
	Name                string   `json:"name" binding:"required"`
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
	m, err := h.merchantSvc.CreateMerchant(c.Request.Context(), service.CreateMerchantInput{
		OwnerUserID:         req.OwnerUserID,
		Name:                req.Name,
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
// Status / Recharge / Refund
// ----------------------------------------------------------------------------

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

type setGroupSellRateReq struct {
	GroupID  int64   `json:"group_id" binding:"required"`
	SellRate float64 `json:"sell_rate" binding:"required"`
	Reason   string  `json:"reason"`
}

// PUT /admin/merchants/:id/group_markups — 设置某商户在某分组的对外售价倍率（绝对值）
func (h *MerchantHandler) SetGroupMarkup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	var req setGroupSellRateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.merchantSvc.SetGroupSellRate(c.Request.Context(), id, req.GroupID, req.SellRate, adminID(c), req.Reason); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// DELETE /admin/merchants/:id/group_markups/:group_id — 删除某分组售价配置
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
	if err := h.merchantSvc.DeleteGroupSellRate(c.Request.Context(), id, groupID, adminID(c), c.Query("reason")); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// GET /admin/merchants/:id/group_costs — 列出商户的所有分组拿货价配置
func (h *MerchantHandler) ListGroupCosts(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	rows, err := h.merchantSvc.ListGroupCosts(c.Request.Context(), id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if rows == nil {
		rows = []*service.MerchantGroupCost{}
	}
	response.Success(c, rows)
}

type setGroupCostRateReq struct {
	GroupID  int64   `json:"group_id" binding:"required"`
	CostRate float64 `json:"cost_rate" binding:"required"`
	Reason   string  `json:"reason"`
}

// PUT /admin/merchants/:id/group_costs — 设置商户在某分组的拿货价倍率（admin only）
func (h *MerchantHandler) SetGroupCost(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	var req setGroupCostRateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.merchantSvc.SetGroupCostRate(c.Request.Context(), id, req.GroupID, req.CostRate, adminID(c), req.Reason); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// DELETE /admin/merchants/:id/group_costs/:group_id — 删除商户某分组拿货价配置
func (h *MerchantHandler) DeleteGroupCost(c *gin.Context) {
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
	if err := h.merchantSvc.DeleteGroupCostRate(c.Request.Context(), id, groupID, adminID(c), c.Query("reason")); err != nil {
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

// ============================================================================
// 提现审核（admin）
// ============================================================================

// GET /admin/merchant_withdrawals
func (h *MerchantHandler) ListWithdrawals(c *gin.Context) {
	status := c.Query("status")
	merchantID, _ := strconv.ParseInt(c.DefaultQuery("merchant_id", "0"), 10, 64)
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	rows, total, err := h.merchantSvc.AdminListWithdrawals(c.Request.Context(), status, merchantID, offset, limit)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	response.Success(c, paginated(rows, total, offset, limit))
}

// POST /admin/merchant_withdrawals/:id/approve
func (h *MerchantHandler) ApproveWithdrawal(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	if err := h.merchantSvc.AdminApproveWithdrawal(c.Request.Context(), id, adminID(c)); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

type rejectWithdrawReq struct {
	Reason string `json:"reason"`
}

// POST /admin/merchant_withdrawals/:id/reject
func (h *MerchantHandler) RejectWithdrawal(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	var req rejectWithdrawReq
	_ = c.ShouldBindJSON(&req)
	if err := h.merchantSvc.AdminRejectWithdrawal(c.Request.Context(), id, adminID(c), req.Reason); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	response.Success(c, gin.H{"ok": true})
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
