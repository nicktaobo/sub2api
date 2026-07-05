// 利润核算 service：按 团队(分公司/销售) / 客户 / 代理(商户) 四个维度
// 实时聚合 usage_logs，输出 营收/成本/毛利。
//
// 利润口径：
//   营收 = actual_cost                                      （计费倍率后的应收）
//   成本 = COALESCE(account_stats_cost, total_cost)
//          × COALESCE(account_rate_multiplier, 1)           （账号侧真实采购成本）
//   毛利 = 营收 - 成本
//
// 成本公式与 dashboard / usage 等已有报表完全一致：调高账号的 rate_multiplier
// 等价于"账号采购单价变贵"，毛利随之下降；调低反之。不引入新字段、不分摊。
//
// 归属继承：sub_user 的归属继承自其商户 owner 的属性值；owner 和独立用户读自身。

package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
)

// ProfitGroupBy 利润报表的分组维度。
type ProfitGroupBy string

const (
	ProfitGroupByMerchant  ProfitGroupBy = "merchant"  // 代理
	ProfitGroupByUser      ProfitGroupBy = "user"      // 客户
	ProfitGroupByAttribute ProfitGroupBy = "attribute" // 团队：分公司 / 销售（按 attribute_id 区分）
)

// ProfitRow 一行聚合结果。
type ProfitRow struct {
	Key          string  `json:"key"`           // 维度键（merchant_id / user_id / attr_value）
	Name         string  `json:"name"`          // 显示名
	Revenue      float64 `json:"revenue"`       // 营收（actual_cost）
	Cost         float64 `json:"cost"`          // 成本（total_cost）
	Profit       float64 `json:"profit"`        // 毛利
	ProfitRate   float64 `json:"profit_rate"`   // 毛利率（profit / revenue），revenue=0 时为 0
	RequestCount int64   `json:"request_count"` // 请求数
}

// ProfitSummaryQuery 查询参数。
type ProfitSummaryQuery struct {
	Start       time.Time
	End         time.Time
	GroupBy     ProfitGroupBy
	AttributeID int64 // GroupBy=attribute 时必填，对应分公司或销售的 user_attribute_definitions.id
	Limit       int   // 仅 user 维度生效（防止千万级用户全返），默认 200
}

// ProfitSummary 总览 + 明细。
type ProfitSummary struct {
	GroupBy      ProfitGroupBy `json:"group_by"`
	AttributeID  int64         `json:"attribute_id,omitempty"`
	Start        time.Time     `json:"start"`
	End          time.Time     `json:"end"`
	TotalRevenue float64       `json:"total_revenue"`
	TotalCost    float64       `json:"total_cost"`
	TotalProfit  float64       `json:"total_profit"`
	Rows         []ProfitRow   `json:"rows"`
}

// ProfitService 利润核算只读 service。
type ProfitService struct {
	entClient *dbent.Client
}

func NewProfitService(entClient *dbent.Client) *ProfitService {
	return &ProfitService{entClient: entClient}
}

func (s *ProfitService) db() (*sql.DB, error) {
	if s == nil || s.entClient == nil {
		return nil, errors.New("profit service: ent client nil")
	}
	drv, ok := s.entClient.Driver().(*entsql.Driver)
	if !ok {
		return nil, errors.New("profit service: ent driver is not *entsql.Driver")
	}
	return drv.DB(), nil
}

// Summary 主查询入口。
func (s *ProfitService) Summary(ctx context.Context, q ProfitSummaryQuery) (*ProfitSummary, error) {
	if q.End.IsZero() {
		q.End = time.Now()
	}
	if q.Start.IsZero() {
		q.Start = q.End.AddDate(0, 0, -30)
	}
	if !q.End.After(q.Start) {
		return nil, fmt.Errorf("profit summary: end must be after start")
	}
	if q.Limit <= 0 || q.Limit > 1000 {
		q.Limit = 200
	}

	switch q.GroupBy {
	case ProfitGroupByMerchant:
		return s.summaryByMerchant(ctx, q)
	case ProfitGroupByUser:
		return s.summaryByUser(ctx, q)
	case ProfitGroupByAttribute:
		if q.AttributeID <= 0 {
			return nil, fmt.Errorf("profit summary: attribute_id required for group_by=attribute")
		}
		return s.summaryByAttribute(ctx, q)
	default:
		return nil, fmt.Errorf("profit summary: invalid group_by %q", q.GroupBy)
	}
}

const unassignedKey = "__unassigned__"

// summaryByMerchant 按代理(商户)分组：
// 子用户归属其 parent_merchant_id；merchant owner 自身消费归属其作为 owner 的商户；
// 独立用户（既非 owner 也非 sub_user）归入 __unassigned__。
func (s *ProfitService) summaryByMerchant(ctx context.Context, q ProfitSummaryQuery) (*ProfitSummary, error) {
	db, err := s.db()
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, `
		WITH user_merchant AS (
			SELECT u.id AS user_id,
			       COALESCE(u.parent_merchant_id,
			                (SELECT m.id FROM merchants m WHERE m.owner_user_id = u.id AND m.deleted_at IS NULL LIMIT 1)
			       ) AS merchant_id
			FROM users u
			WHERE u.deleted_at IS NULL
		)
		SELECT um.merchant_id,
		       COALESCE(m.name, '') AS merchant_name,
		       COUNT(ul.id) AS req_cnt,
		       COALESCE(SUM(ul.actual_cost), 0)::float8 AS revenue,
		       COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost)
		                    * COALESCE(ul.account_rate_multiplier, 1)), 0)::float8 AS cost
		FROM usage_logs ul
		JOIN user_merchant um ON um.user_id = ul.user_id
		LEFT JOIN merchants m ON m.id = um.merchant_id
		WHERE ul.created_at >= $1 AND ul.created_at < $2
		GROUP BY um.merchant_id, m.name
		ORDER BY revenue DESC
	`, q.Start, q.End)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := &ProfitSummary{GroupBy: q.GroupBy, Start: q.Start, End: q.End}
	for rows.Next() {
		var mID sql.NullInt64
		var mName string
		var reqCnt int64
		var rev, cost float64
		if err := rows.Scan(&mID, &mName, &reqCnt, &rev, &cost); err != nil {
			return nil, err
		}
		key, name := unassignedKey, ""
		if mID.Valid {
			key = fmt.Sprintf("%d", mID.Int64)
			name = mName
		}
		out.Rows = append(out.Rows, buildRow(key, name, rev, cost, reqCnt))
		out.TotalRevenue += rev
		out.TotalCost += cost
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out.TotalProfit = out.TotalRevenue - out.TotalCost
	return out, nil
}

// summaryByUser 按客户(用户)分组：每个产生过消费的 user 一行。
// 大数据量下用 LIMIT 防爆，前端可按 revenue 降序看 Top N。
func (s *ProfitService) summaryByUser(ctx context.Context, q ProfitSummaryQuery) (*ProfitSummary, error) {
	db, err := s.db()
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, `
		SELECT u.id,
		       COALESCE(NULLIF(u.username, ''), u.email) AS display_name,
		       COUNT(ul.id) AS req_cnt,
		       COALESCE(SUM(ul.actual_cost), 0)::float8 AS revenue,
		       COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost)
		                    * COALESCE(ul.account_rate_multiplier, 1)), 0)::float8 AS cost
		FROM usage_logs ul
		JOIN users u ON u.id = ul.user_id
		WHERE ul.created_at >= $1 AND ul.created_at < $2
		GROUP BY u.id, u.username, u.email
		ORDER BY revenue DESC
		LIMIT $3
	`, q.Start, q.End, q.Limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := &ProfitSummary{GroupBy: q.GroupBy, Start: q.Start, End: q.End}
	for rows.Next() {
		var id int64
		var name string
		var reqCnt int64
		var rev, cost float64
		if err := rows.Scan(&id, &name, &reqCnt, &rev, &cost); err != nil {
			return nil, err
		}
		out.Rows = append(out.Rows, buildRow(fmt.Sprintf("%d", id), name, rev, cost, reqCnt))
		out.TotalRevenue += rev
		out.TotalCost += cost
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out.TotalProfit = out.TotalRevenue - out.TotalCost
	return out, nil
}

// summaryByAttribute 按某个用户属性(分公司/销售)分组：
// 1) 每个 user 解析"有效归属用户"：sub_user → 其商户 owner；其他 → 自身。
// 2) 读该归属用户在指定 attribute_id 上的 value，空值归 __unassigned__。
// 3) 按 value 分组聚合。
func (s *ProfitService) summaryByAttribute(ctx context.Context, q ProfitSummaryQuery) (*ProfitSummary, error) {
	db, err := s.db()
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, `
		WITH user_owner AS (
			SELECT u.id AS user_id,
			       COALESCE(
			         (SELECT m.owner_user_id FROM merchants m WHERE m.id = u.parent_merchant_id AND m.deleted_at IS NULL),
			         u.id
			       ) AS owner_user_id
			FROM users u
			WHERE u.deleted_at IS NULL
		)
		SELECT COALESCE(NULLIF(uav.value, ''), '') AS attr_value,
		       COUNT(ul.id) AS req_cnt,
		       COALESCE(SUM(ul.actual_cost), 0)::float8 AS revenue,
		       COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost)
		                    * COALESCE(ul.account_rate_multiplier, 1)), 0)::float8 AS cost
		FROM usage_logs ul
		JOIN user_owner uo ON uo.user_id = ul.user_id
		LEFT JOIN user_attribute_values uav
		       ON uav.user_id = uo.owner_user_id AND uav.attribute_id = $3
		WHERE ul.created_at >= $1 AND ul.created_at < $2
		GROUP BY attr_value
		ORDER BY revenue DESC
	`, q.Start, q.End, q.AttributeID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := &ProfitSummary{GroupBy: q.GroupBy, AttributeID: q.AttributeID, Start: q.Start, End: q.End}
	for rows.Next() {
		var val string
		var reqCnt int64
		var rev, cost float64
		if err := rows.Scan(&val, &reqCnt, &rev, &cost); err != nil {
			return nil, err
		}
		key := strings.TrimSpace(val)
		name := key
		if key == "" {
			key = unassignedKey
			name = ""
		}
		out.Rows = append(out.Rows, buildRow(key, name, rev, cost, reqCnt))
		out.TotalRevenue += rev
		out.TotalCost += cost
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out.TotalProfit = out.TotalRevenue - out.TotalCost
	return out, nil
}

func buildRow(key, name string, revenue, cost float64, reqCnt int64) ProfitRow {
	profit := revenue - cost
	var rate float64
	if revenue > 0 {
		rate = profit / revenue
	}
	return ProfitRow{
		Key:          key,
		Name:         name,
		Revenue:      revenue,
		Cost:         cost,
		Profit:       profit,
		ProfitRate:   rate,
		RequestCount: reqCnt,
	}
}
