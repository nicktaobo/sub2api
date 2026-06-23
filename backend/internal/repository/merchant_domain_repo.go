// MERCHANT-SYSTEM v1.0
package repository

import (
	"context"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/merchantdomain"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type merchantDomainRepository struct {
	client *dbent.Client
}

func NewMerchantDomainRepository(client *dbent.Client) service.MerchantDomainRepository {
	return &merchantDomainRepository{client: client}
}

func (r *merchantDomainRepository) Create(ctx context.Context, d *service.MerchantDomain) error {
	client := clientFromContext(ctx, r.client)
	created, err := client.MerchantDomain.Create().
		SetMerchantID(d.MerchantID).
		SetDomain(d.Domain).
		SetSiteName(d.SiteName).
		SetSiteLogo(d.SiteLogo).
		SetBrandColor(d.BrandColor).
		SetCustomCSS(d.CustomCSS).
		SetHomeContent(d.HomeContent).
		SetSeoTitle(d.SEOTitle).
		SetSeoDescription(d.SEODescription).
		SetSeoKeywords(d.SEOKeywords).
		SetVerifyToken(d.VerifyToken).
		SetVerified(d.Verified).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrMerchantDomainNotFound, service.ErrMerchantDomainConflict)
	}
	d.ID = created.ID
	d.CreatedAt = created.CreatedAt
	d.UpdatedAt = created.UpdatedAt
	return nil
}

func (r *merchantDomainRepository) GetByDomain(ctx context.Context, domain string) (*service.MerchantDomain, error) {
	client := clientFromContext(ctx, r.client)
	row, err := client.MerchantDomain.Query().Where(merchantdomain.DomainEQ(domain)).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrMerchantDomainNotFound, nil)
	}
	return merchantDomainEntityToService(row), nil
}

func (r *merchantDomainRepository) GetByID(ctx context.Context, id int64) (*service.MerchantDomain, error) {
	client := clientFromContext(ctx, r.client)
	row, err := client.MerchantDomain.Query().Where(merchantdomain.IDEQ(id)).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrMerchantDomainNotFound, nil)
	}
	return merchantDomainEntityToService(row), nil
}

func (r *merchantDomainRepository) ListByMerchant(ctx context.Context, merchantID int64) ([]*service.MerchantDomain, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.MerchantDomain.Query().
		Where(merchantdomain.MerchantIDEQ(merchantID)).
		Order(dbent.Desc(merchantdomain.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*service.MerchantDomain, 0, len(rows))
	for _, row := range rows {
		out = append(out, merchantDomainEntityToService(row))
	}
	return out, nil
}

func (r *merchantDomainRepository) Update(ctx context.Context, d *service.MerchantDomain) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.MerchantDomain.UpdateOneID(d.ID).
		SetSiteName(d.SiteName).
		SetSiteLogo(d.SiteLogo).
		SetBrandColor(d.BrandColor).
		SetCustomCSS(d.CustomCSS).
		SetHomeContent(d.HomeContent).
		SetSeoTitle(d.SEOTitle).
		SetSeoDescription(d.SEODescription).
		SetSeoKeywords(d.SEOKeywords).
		Save(ctx)
	return translatePersistenceError(err, service.ErrMerchantDomainNotFound, nil)
}

func (r *merchantDomainRepository) MarkVerified(ctx context.Context, id int64) error {
	client := clientFromContext(ctx, r.client)
	now := time.Now()
	_, err := client.MerchantDomain.UpdateOneID(id).
		SetVerified(true).
		SetVerifiedAt(now).
		Save(ctx)
	return translatePersistenceError(err, service.ErrMerchantDomainNotFound, nil)
}

func (r *merchantDomainRepository) SoftDelete(ctx context.Context, id int64) error {
	client := clientFromContext(ctx, r.client)
	return client.MerchantDomain.DeleteOneID(id).Exec(ctx)
}
