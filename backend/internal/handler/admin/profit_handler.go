// admin.ProfitHandler 利润自动化核算 API：按 团队(分公司/销售) / 客户 / 代理(商户)
// 四个维度实时聚合 usage_logs，输出营收/成本/毛利。
//
// 路由：GET /admin/profit/summary

package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type ProfitHandler struct {
	profitSvc *service.ProfitService
}

func NewProfitHandler(profitSvc *service.ProfitService) *ProfitHandler {
	return &ProfitHandler{profitSvc: profitSvc}
}

// Summary GET /admin/profit/summary
//
// Query 参数：
//   - start, end       RFC3339 时间；默认 end=now, start=end-30d
//   - group_by         merchant | user | attribute（必填）
//   - attribute_id     group_by=attribute 时必填，对应分公司或销售的属性定义 ID
//   - limit            user 维度生效，默认 200，最大 1000
func (h *ProfitHandler) Summary(c *gin.Context) {
	q := service.ProfitSummaryQuery{
		GroupBy: service.ProfitGroupBy(c.Query("group_by")),
	}

	if v := c.Query("start"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			response.BadRequest(c, "invalid start: "+err.Error())
			return
		}
		q.Start = t
	}
	if v := c.Query("end"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			response.BadRequest(c, "invalid end: "+err.Error())
			return
		}
		q.End = t
	}
	if v := c.Query("attribute_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			response.BadRequest(c, "invalid attribute_id")
			return
		}
		q.AttributeID = id
	}
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			response.BadRequest(c, "invalid limit")
			return
		}
		q.Limit = n
	}

	out, err := h.profitSvc.Summary(c.Request.Context(), q)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, out)
}
