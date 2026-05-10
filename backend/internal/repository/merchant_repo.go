// MERCHANT-SYSTEM v1.0
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/merchant"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type merchantRepository struct {
	client *dbent.Client
	db     *sql.DB
}

func NewMerchantRepository(client *dbent.Client, db *sql.DB) service.MerchantRepository {
	return &merchantRepository{client: client, db: db}
}

func (r *merchantRepository) Create(ctx context.Context, m *service.Merchant) error {
	client := clientFromContext(ctx, r.client)
	if m.NotifyEmails == nil {
		m.NotifyEmails = []string{}
	}
	created, err := client.Merchant.Create().
		SetOwnerUserID(m.OwnerUserID).
		SetName(m.Name).
		SetStatus(orDefault(m.Status, service.MerchantStatusActive)).
		SetDiscount(m.Discount).
		SetUserMarkupDefault(m.UserMarkupDefault).
		SetOwnerBalanceBaseline(m.OwnerBalanceBaseline).
		SetLowBalanceThreshold(m.LowBalanceThreshold).
		SetNotifyEmails(m.NotifyEmails).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrMerchantNotFound, service.ErrMerchantOwnerConflict)
	}
	m.ID = created.ID
	m.CreatedAt = created.CreatedAt
	m.UpdatedAt = created.UpdatedAt
	return nil
}

func (r *merchantRepository) GetByID(ctx context.Context, id int64) (*service.Merchant, error) {
	client := clientFromContext(ctx, r.client)
	row, err := client.Merchant.Query().Where(merchant.IDEQ(id)).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrMerchantNotFound, nil)
	}
	return merchantEntityToService(row), nil
}

func (r *merchantRepository) GetByOwnerUserID(ctx context.Context, userID int64) (*service.Merchant, error) {
	client := clientFromContext(ctx, r.client)
	row, err := client.Merchant.Query().Where(merchant.OwnerUserIDEQ(userID)).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrMerchantNotFound, nil)
	}
	return merchantEntityToService(row), nil
}

// GetByDomain 根据 verified 域名查商户。未 verified 的域名不参与。
func (r *merchantRepository) GetByDomain(ctx context.Context, domain string) (*service.Merchant, error) {
	if r.db == nil {
		return nil, errors.New("merchant repository sql db is nil")
	}
	const q = `
		SELECT m.id, m.owner_user_id, m.name, m.status, m.discount, m.user_markup_default,
		       m.owner_balance_baseline, m.low_balance_threshold, m.notify_emails,
		       m.created_at, m.updated_at, m.deleted_at
		FROM merchants m
		JOIN merchant_domains d ON d.merchant_id = m.id
		WHERE d.domain = $1
		  AND d.verified = TRUE
		  AND d.deleted_at IS NULL
		  AND m.deleted_at IS NULL
		LIMIT 1
	`
	var (
		mm           service.Merchant
		notifyJSON   []byte
		deletedAt    sql.NullTime
	)
	err := r.db.QueryRowContext(ctx, q, strings.TrimSpace(domain)).Scan(
		&mm.ID, &mm.OwnerUserID, &mm.Name, &mm.Status, &mm.Discount, &mm.UserMarkupDefault,
		&mm.OwnerBalanceBaseline, &mm.LowBalanceThreshold, &notifyJSON,
		&mm.CreatedAt, &mm.UpdatedAt, &deletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrMerchantNotFound
	}
	if err != nil {
		return nil, err
	}
	if deletedAt.Valid {
		t := deletedAt.Time
		mm.DeletedAt = &t
	}
	mm.NotifyEmails = decodeStringJSONList(notifyJSON)
	return &mm, nil
}

func (r *merchantRepository) List(ctx context.Context, status string, offset, limit int) ([]*service.Merchant, int, error) {
	client := clientFromContext(ctx, r.client)
	q := client.Merchant.Query()
	if status != "" {
		q = q.Where(merchant.StatusEQ(status))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := q.Order(dbent.Desc(merchant.FieldCreatedAt)).Offset(offset).Limit(limit).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]*service.Merchant, 0, len(rows))
	for _, row := range rows {
		out = append(out, merchantEntityToService(row))
	}
	return out, total, nil
}

func (r *merchantRepository) Update(ctx context.Context, m *service.Merchant) error {
	client := clientFromContext(ctx, r.client)
	if m.NotifyEmails == nil {
		m.NotifyEmails = []string{}
	}
	_, err := client.Merchant.UpdateOneID(m.ID).
		SetName(m.Name).
		SetStatus(m.Status).
		SetDiscount(m.Discount).
		SetUserMarkupDefault(m.UserMarkupDefault).
		SetLowBalanceThreshold(m.LowBalanceThreshold).
		SetNotifyEmails(m.NotifyEmails).
		Save(ctx)
	return translatePersistenceError(err, service.ErrMerchantNotFound, nil)
}

func (r *merchantRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.Merchant.UpdateOneID(id).SetStatus(status).Save(ctx)
	return translatePersistenceError(err, service.ErrMerchantNotFound, nil)
}

func (r *merchantRepository) UpdateDiscount(ctx context.Context, id int64, discount float64) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.Merchant.UpdateOneID(id).SetDiscount(discount).Save(ctx)
	return translatePersistenceError(err, service.ErrMerchantNotFound, nil)
}

func (r *merchantRepository) UpdateMarkupDefault(ctx context.Context, id int64, markup float64) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.Merchant.UpdateOneID(id).SetUserMarkupDefault(markup).Save(ctx)
	return translatePersistenceError(err, service.ErrMerchantNotFound, nil)
}

func (r *merchantRepository) SoftDelete(ctx context.Context, id int64) error {
	client := clientFromContext(ctx, r.client)
	return client.Merchant.DeleteOneID(id).Exec(ctx)
}

// LookupMerchantIDForUser 同时识别 sub_user 和 owner（RFC §5.2.1 Step 2.0）。
// owner 路径**不过滤 status**——suspended owner 自用 API 仍要写 owner_usage_debit ledger。
func (r *merchantRepository) LookupMerchantIDForUser(ctx context.Context, userID int64) (int64, error) {
	if r.db == nil {
		return 0, errors.New("merchant repository sql db is nil")
	}
	const q = `
		SELECT merchant_id FROM (
			SELECT u.parent_merchant_id AS merchant_id
			FROM users u
			WHERE u.id = $1
			  AND u.parent_merchant_id IS NOT NULL
			  AND u.deleted_at IS NULL

			UNION ALL

			SELECT m.id AS merchant_id
			FROM merchants m
			WHERE m.owner_user_id = $1
			  AND m.deleted_at IS NULL
		) t
		LIMIT 1
	`
	var mid int64
	err := r.db.QueryRowContext(ctx, q, userID).Scan(&mid)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return mid, nil
}

// LoadPricing 一次性加载 merchant + 所有 active group_markups（已过滤 deleted_at）。
// RFC §4.1.5 / §5.2.1 Step 2.1 / Finding 7。
func (r *merchantRepository) LoadPricing(ctx context.Context, merchantID int64) (*service.CachedMerchantPricing, error) {
	client := clientFromContext(ctx, r.client)
	m, err := client.Merchant.Query().
		Where(merchant.IDEQ(merchantID)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	if m.DeletedAt != nil {
		return nil, nil
	}

	// 加载 group markups + JOIN groups 过滤软删除
	if r.db == nil {
		return nil, errors.New("merchant repository sql db is nil")
	}
	const groupSQL = `
		SELECT mgm.group_id, mgm.markup
		FROM merchant_group_markups mgm
		JOIN groups g ON g.id = mgm.group_id
		WHERE mgm.merchant_id = $1
		  AND g.deleted_at IS NULL
	`
	rows, err := r.db.QueryContext(ctx, groupSQL, merchantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	gm := make(map[int64]float64, 16)
	for rows.Next() {
		var gid int64
		var markup float64
		if err := rows.Scan(&gid, &markup); err != nil {
			return nil, err
		}
		gm[gid] = markup
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &service.CachedMerchantPricing{
		MerchantID:        m.ID,
		OwnerUserID:       m.OwnerUserID,
		Status:            m.Status,
		Discount:          m.Discount,
		UserMarkupDefault: m.UserMarkupDefault,
		GroupMarkups:      gm,
	}, nil
}

// ----- entity → service mapper -----

func merchantEntityToService(m *dbent.Merchant) *service.Merchant {
	if m == nil {
		return nil
	}
	out := &service.Merchant{
		ID:                   m.ID,
		OwnerUserID:          m.OwnerUserID,
		Name:                 m.Name,
		Status:               m.Status,
		Discount:             m.Discount,
		UserMarkupDefault:    m.UserMarkupDefault,
		OwnerBalanceBaseline: m.OwnerBalanceBaseline,
		LowBalanceThreshold:  m.LowBalanceThreshold,
		NotifyEmails:         append([]string{}, m.NotifyEmails...),
		CreatedAt:            m.CreatedAt,
		UpdatedAt:            m.UpdatedAt,
	}
	if m.DeletedAt != nil {
		t := *m.DeletedAt
		out.DeletedAt = &t
	}
	return out
}

func merchantDomainEntityToService(d *dbent.MerchantDomain) *service.MerchantDomain {
	if d == nil {
		return nil
	}
	out := &service.MerchantDomain{
		ID:             d.ID,
		MerchantID:     d.MerchantID,
		Domain:         d.Domain,
		SiteName:       d.SiteName,
		SiteLogo:       d.SiteLogo,
		BrandColor:     d.BrandColor,
		CustomCSS:      d.CustomCSS,
		HomeContent:    d.HomeContent,
		SEOTitle:       d.SeoTitle,
		SEODescription: d.SeoDescription,
		SEOKeywords:    d.SeoKeywords,
		VerifyToken:    d.VerifyToken,
		Verified:       d.Verified,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
	if d.VerifiedAt != nil {
		t := *d.VerifiedAt
		out.VerifiedAt = &t
	}
	if d.DeletedAt != nil {
		t := *d.DeletedAt
		out.DeletedAt = &t
	}
	return out
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// decodeStringJSONList 安全地把 JSONB []byte 解码成 []string；空/无效都返回 []string{}。
func decodeStringJSONList(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	out := []string{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return []string{}
	}
	return out
}
