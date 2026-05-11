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

// GET /merchant/sub_users — 列出子用户
func (h *MerchantHandler) ListSubUsers(c *gin.Context) {
	m := h.resolveOwnerMerchant(c)
	if m == nil {
		return
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	search := c.Query("q")
	rows, total, err := h.merchantSvc.ListSubUsers(c.Request.Context(), m.ID, search, offset, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, paginatedOwner(rows, total, offset, limit))
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

// GET /merchant/pricing_groups — 列出商户可定价的分组（带每个分组当前生效 markup）
func (h *MerchantHandler) ListPricingGroups(c *gin.Context) {
	m := h.resolveOwnerMerchant(c)
	if m == nil {
		return
	}
	rows, err := h.merchantSvc.ListPricingGroups(c.Request.Context(), m.ID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if rows == nil {
		rows = []service.MerchantPricingGroup{}
	}
	response.Success(c, rows)
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

// 商户 owner 自助售价管理（v2.0：sell_rate 绝对倍率，无 default markup 概念）。
// admin 仍可代操作，audit 通过 admin_id 区分：owner 自助 adminID=0；admin 代操作填 admin user_id。
// 商户只能改 sell_rate，cost_rate 始终由 admin 控制。

type ownerSetGroupSellRateReq struct {
	GroupID  int64   `json:"group_id" binding:"required"`
	SellRate float64 `json:"sell_rate" binding:"required"`
	Reason   string  `json:"reason"`
}

// PUT /merchant/group_markups — 商户设置某分组对外售价倍率
func (h *MerchantHandler) SetGroupMarkup(c *gin.Context) {
	m := h.resolveOwnerMerchant(c)
	if m == nil {
		return
	}
	var req ownerSetGroupSellRateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.merchantSvc.SetGroupSellRate(c.Request.Context(), m.ID, req.GroupID, req.SellRate, 0, req.Reason); err != nil {
		if !response.ErrorFrom(c, err) {
			response.Error(c, http.StatusBadRequest, err.Error())
		}
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// DELETE /merchant/group_markups/:group_id — 商户删除某分组售价（删后该 group 不分润，sub_user 按主站价）
func (h *MerchantHandler) DeleteGroupMarkup(c *gin.Context) {
	m := h.resolveOwnerMerchant(c)
	if m == nil {
		return
	}
	groupID, err := strconv.ParseInt(c.Param("group_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid group_id")
		return
	}
	if err := h.merchantSvc.DeleteGroupSellRate(c.Request.Context(), m.ID, groupID, 0, c.Query("reason")); err != nil {
		if !response.ErrorFrom(c, err) {
			response.Error(c, http.StatusBadRequest, err.Error())
		}
		return
	}
	response.Success(c, gin.H{"ok": true})
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

type createDomainReq struct {
	Domain         string `json:"domain" binding:"required"`
	SiteName       string `json:"site_name"`
	SiteLogo       string `json:"site_logo"`
	BrandColor     string `json:"brand_color"`
	CustomCSS      string `json:"custom_css"`
	HomeContent    string `json:"home_content"`
	SEOTitle       string `json:"seo_title"`
	SEODescription string `json:"seo_description"`
	SEOKeywords    string `json:"seo_keywords"`
}

func (h *MerchantHandler) CreateDomain(c *gin.Context) {
	m := h.resolveOwnerMerchant(c)
	if m == nil {
		return
	}
	var req createDomainReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	d, err := h.merchantSvc.CreateDomain(c.Request.Context(), service.CreateDomainInput{
		MerchantID:     m.ID,
		Domain:         req.Domain,
		SiteName:       req.SiteName,
		SiteLogo:       req.SiteLogo,
		BrandColor:     req.BrandColor,
		CustomCSS:      req.CustomCSS,
		HomeContent:    req.HomeContent,
		SEOTitle:       req.SEOTitle,
		SEODescription: req.SEODescription,
		SEOKeywords:    req.SEOKeywords,
	})
	if err != nil {
		if !response.ErrorFrom(c, err) {
			response.Error(c, http.StatusBadRequest, err.Error())
		}
		return
	}
	response.Success(c, d)
}

type updateDomainReq struct {
	SiteName       string `json:"site_name"`
	SiteLogo       string `json:"site_logo"`
	BrandColor     string `json:"brand_color"`
	CustomCSS      string `json:"custom_css"`
	HomeContent    string `json:"home_content"`
	SEOTitle       string `json:"seo_title"`
	SEODescription string `json:"seo_description"`
	SEOKeywords    string `json:"seo_keywords"`
}

func (h *MerchantHandler) UpdateDomain(c *gin.Context) {
	m := h.resolveOwnerMerchant(c)
	if m == nil {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	var req updateDomainReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	d, err := h.merchantSvc.UpdateDomain(c.Request.Context(), m.ID, id, service.UpdateDomainInput{
		SiteName:       req.SiteName,
		SiteLogo:       req.SiteLogo,
		BrandColor:     req.BrandColor,
		CustomCSS:      req.CustomCSS,
		HomeContent:    req.HomeContent,
		SEOTitle:       req.SEOTitle,
		SEODescription: req.SEODescription,
		SEOKeywords:    req.SEOKeywords,
	})
	if err != nil {
		if !response.ErrorFrom(c, err) {
			response.Error(c, http.StatusBadRequest, err.Error())
		}
		return
	}
	response.Success(c, d)
}

func (h *MerchantHandler) VerifyDomain(c *gin.Context) {
	m := h.resolveOwnerMerchant(c)
	if m == nil {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	if err := h.merchantSvc.VerifyDomain(c.Request.Context(), m.ID, id); err != nil {
		if !response.ErrorFrom(c, err) {
			response.Error(c, http.StatusBadRequest, err.Error())
		}
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// DNSSetupInfo 返回平台域名配置元信息，给 owner 后台展示 DNS 步骤用。
func (h *MerchantHandler) DNSSetupInfo(c *gin.Context) {
	if _, ok := jwtUserID(c); !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	response.Success(c, h.merchantSvc.GetDNSSetupInfo())
}

// ============================================================================
// Stats + Withdrawal (owner)
// ============================================================================

// GET /merchant/stats
func (h *MerchantHandler) GetStats(c *gin.Context) {
	m := h.resolveOwnerMerchant(c)
	if m == nil {
		return
	}
	stats, err := h.merchantSvc.GetMerchantStats(c.Request.Context(), m.ID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, stats)
}

type createWithdrawReq struct {
	Amount         float64 `json:"amount" binding:"required"`
	PaymentMethod  string  `json:"payment_method" binding:"required"`
	PaymentAccount string  `json:"payment_account" binding:"required"`
	PaymentName    string  `json:"payment_name" binding:"required"`
	Note           string  `json:"note"`
}

// POST /merchant/withdrawals
func (h *MerchantHandler) CreateWithdraw(c *gin.Context) {
	m := h.resolveOwnerMerchant(c)
	if m == nil {
		return
	}
	var req createWithdrawReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	w, err := h.merchantSvc.CreateWithdrawRequest(c.Request.Context(), service.CreateWithdrawInput{
		MerchantID:     m.ID,
		Amount:         req.Amount,
		PaymentMethod:  req.PaymentMethod,
		PaymentAccount: req.PaymentAccount,
		PaymentName:    req.PaymentName,
		Note:           req.Note,
	})
	if err != nil {
		if !response.ErrorFrom(c, err) {
			response.Error(c, http.StatusBadRequest, err.Error())
		}
		return
	}
	response.Success(c, w)
}

// GET /merchant/withdrawals
func (h *MerchantHandler) ListWithdrawals(c *gin.Context) {
	m := h.resolveOwnerMerchant(c)
	if m == nil {
		return
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	status := c.Query("status")
	rows, total, err := h.merchantSvc.ListWithdrawRequests(c.Request.Context(), m.ID, status, offset, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, paginatedOwner(rows, total, offset, limit))
}

func (h *MerchantHandler) DeleteDomain(c *gin.Context) {
	m := h.resolveOwnerMerchant(c)
	if m == nil {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	if err := h.merchantSvc.DeleteDomain(c.Request.Context(), m.ID, id); err != nil {
		if !response.ErrorFrom(c, err) {
			response.Error(c, http.StatusBadRequest, err.Error())
		}
		return
	}
	response.Success(c, gin.H{"ok": true})
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
