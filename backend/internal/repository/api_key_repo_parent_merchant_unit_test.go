package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 回归：GetByKeyForAuth 的 WithUser Select 必须包含 parent_merchant_id，
// 否则 merchant 子用户经鉴权路径拿到的 User.ParentMerchantID 恒为 nil，
// 中间件 merchant suspended 拦截与 batch_image merchant 子用户守卫全部失效。
func TestAPIKeyRepository_GetByKeyForAuth_PreservesParentMerchantID_SQLite(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()

	owner := mustCreateAPIKeyRepoUser(t, ctx, client, "getbykey-auth-merchant-owner@test.com")
	merchant, err := client.Merchant.Create().
		SetOwnerUserID(owner.ID).
		SetName("m-auth-unit").
		Save(ctx)
	require.NoError(t, err)

	subUser, err := client.User.Create().
		SetEmail("getbykey-auth-merchant-sub@test.com").
		SetPasswordHash("test-password-hash").
		SetRole(service.RoleUser).
		SetStatus(service.StatusActive).
		SetParentMerchantID(merchant.ID).
		Save(ctx)
	require.NoError(t, err)

	key := &service.APIKey{
		UserID: subUser.ID,
		Key:    "sk-getbykey-auth-merchant-unit",
		Name:   "Merchant Sub-User Key Unit",
		Status: service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, key))

	got, err := repo.GetByKeyForAuth(ctx, key.Key)
	require.NoError(t, err)
	require.NotNil(t, got.User)
	require.NotNil(t, got.User.ParentMerchantID, "GetByKeyForAuth 漏选 parent_merchant_id 列")
	require.Equal(t, merchant.ID, *got.User.ParentMerchantID)
}
