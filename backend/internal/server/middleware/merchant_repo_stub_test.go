package middleware

import (
	"context"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// MERCHANT-SYSTEM 停用商户守卫（fork 定制，api_key_auth.go/api_key_auth_google.go 的高频合并冲突点）
// 的共享测试替身。
//
// 刻意不加 build tag：孪生守卫测试所在的 api_key_auth_google_test.go 无 tag，会在
// unit 与 integration 下都被编译；若本文件限定 //go:build unit，integration 会编译失败。

// stubMerchantRepo 只实现 GetByID —— 守卫只调这一个方法，其余方法存在仅为满足接口。
// 任何一个非 GetByID 方法被中间件调用都意味着守卫逻辑跑偏，因此统一返回 not implemented。
type stubMerchantRepo struct {
	getByID func(ctx context.Context, id int64) (*service.Merchant, error)
}

func (r *stubMerchantRepo) Create(ctx context.Context, m *service.Merchant) error {
	return errors.New("not implemented")
}

func (r *stubMerchantRepo) GetByID(ctx context.Context, id int64) (*service.Merchant, error) {
	if r.getByID != nil {
		return r.getByID(ctx, id)
	}
	return nil, errors.New("not implemented")
}

func (r *stubMerchantRepo) GetByOwnerUserID(ctx context.Context, userID int64) (*service.Merchant, error) {
	return nil, errors.New("not implemented")
}

func (r *stubMerchantRepo) GetByDomain(ctx context.Context, domain string) (*service.Merchant, error) {
	return nil, errors.New("not implemented")
}

func (r *stubMerchantRepo) List(ctx context.Context, status string, offset, limit int) ([]*service.Merchant, int, error) {
	return nil, 0, errors.New("not implemented")
}

func (r *stubMerchantRepo) Update(ctx context.Context, m *service.Merchant) error {
	return errors.New("not implemented")
}

func (r *stubMerchantRepo) UpdateStatus(ctx context.Context, id int64, status string) error {
	return errors.New("not implemented")
}

func (r *stubMerchantRepo) SoftDelete(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

func (r *stubMerchantRepo) LookupMerchantIDForUser(ctx context.Context, userID int64) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *stubMerchantRepo) LoadPricing(ctx context.Context, merchantID int64) (*service.CachedMerchantPricing, error) {
	return nil, errors.New("not implemented")
}

// newSuspendedMerchantRepo 返回一个把所有 merchant 都视为 suspended 的仓储。
func newSuspendedMerchantRepo() *stubMerchantRepo {
	return &stubMerchantRepo{
		getByID: func(_ context.Context, id int64) (*service.Merchant, error) {
			return &service.Merchant{ID: id, Status: service.MerchantStatusSuspended}, nil
		},
	}
}
