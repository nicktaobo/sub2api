// MERCHANT-SYSTEM v3.0
// ValidateUserDomainScope 单元测试：覆盖 7 个核心场景。

package service

import (
	"context"
	"errors"
	"testing"
)

func ptrInt64(v int64) *int64 { return &v }

func TestValidateUserDomainScope_MerchantSystemDisabled(t *testing.T) {
	// 商户系统关闭：任何用户在任何域名都放行。
	ctx := WithMerchantInGoContext(context.Background(), &MerchantContext{
		Merchant: &Merchant{ID: 1, Status: MerchantStatusActive, OwnerUserID: 100},
	})
	u := &User{ID: 999, ParentMerchantID: ptrInt64(2)} // 即使是 merchant 2 的 sub_user
	if err := ValidateUserDomainScope(ctx, u, false); err != nil {
		t.Fatalf("merchant disabled should allow all, got %v", err)
	}
}

func TestValidateUserDomainScope_MainSite_PlainUser(t *testing.T) {
	// 主站：普通用户（parent=NULL）放行。
	u := &User{ID: 1}
	if err := ValidateUserDomainScope(context.Background(), u, true); err != nil {
		t.Fatalf("plain user on main site should be allowed, got %v", err)
	}
}

func TestValidateUserDomainScope_MainSite_SubUser(t *testing.T) {
	// 主站：子用户拒绝。
	u := &User{ID: 5, ParentMerchantID: ptrInt64(1)}
	err := ValidateUserDomainScope(context.Background(), u, true)
	if !errors.Is(err, ErrUserDomainScopeMismatch) {
		t.Fatalf("sub-user on main site should be rejected with ErrUserDomainScopeMismatch, got %v", err)
	}
}

func TestValidateUserDomainScope_MerchantDomain_OwnSubUser(t *testing.T) {
	// 商户 M1 域名：M1 的 sub_user 放行。
	ctx := WithMerchantInGoContext(context.Background(), &MerchantContext{
		Merchant: &Merchant{ID: 1, Status: MerchantStatusActive, OwnerUserID: 100},
	})
	u := &User{ID: 5, ParentMerchantID: ptrInt64(1)}
	if err := ValidateUserDomainScope(ctx, u, true); err != nil {
		t.Fatalf("M1 sub-user on M1 domain should be allowed, got %v", err)
	}
}

func TestValidateUserDomainScope_MerchantDomain_OtherMerchantSubUser(t *testing.T) {
	// 商户 M1 域名：M2 的 sub_user 拒绝。
	ctx := WithMerchantInGoContext(context.Background(), &MerchantContext{
		Merchant: &Merchant{ID: 1, Status: MerchantStatusActive, OwnerUserID: 100},
	})
	u := &User{ID: 5, ParentMerchantID: ptrInt64(2)} // belongs to M2
	err := ValidateUserDomainScope(ctx, u, true)
	if !errors.Is(err, ErrUserDomainScopeMismatch) {
		t.Fatalf("M2 sub-user on M1 domain should be rejected, got %v", err)
	}
}

func TestValidateUserDomainScope_MerchantDomain_Owner(t *testing.T) {
	// 商户 M1 域名：M1 的 owner（自己是普通用户，但 m.OwnerUserID == user.ID）放行。
	ctx := WithMerchantInGoContext(context.Background(), &MerchantContext{
		Merchant: &Merchant{ID: 1, Status: MerchantStatusActive, OwnerUserID: 100},
	})
	u := &User{ID: 100} // owner
	if err := ValidateUserDomainScope(ctx, u, true); err != nil {
		t.Fatalf("owner on own merchant domain should be allowed, got %v", err)
	}
}

func TestValidateUserDomainScope_MerchantDomain_PlainUser(t *testing.T) {
	// 商户 M1 域名：主站普通用户（parent=NULL，且不是 M1 owner）拒绝。
	ctx := WithMerchantInGoContext(context.Background(), &MerchantContext{
		Merchant: &Merchant{ID: 1, Status: MerchantStatusActive, OwnerUserID: 100},
	})
	u := &User{ID: 999}
	err := ValidateUserDomainScope(ctx, u, true)
	if !errors.Is(err, ErrUserDomainScopeMismatch) {
		t.Fatalf("plain main-site user on merchant domain should be rejected, got %v", err)
	}
}

func TestValidateUserDomainScope_MerchantSuspended(t *testing.T) {
	// 商户停用：哪怕是该商户自己的 owner 或 sub_user 也拒绝（整站关闭登录）。
	ctx := WithMerchantInGoContext(context.Background(), &MerchantContext{
		Merchant: &Merchant{ID: 1, Status: MerchantStatusSuspended, OwnerUserID: 100},
	})
	subUser := &User{ID: 5, ParentMerchantID: ptrInt64(1)}
	if err := ValidateUserDomainScope(ctx, subUser, true); err == nil || !errors.Is(err, ErrMerchantSuspended) {
		t.Fatalf("suspended merchant should reject sub-user, got %v", err)
	}
	owner := &User{ID: 100}
	if err := ValidateUserDomainScope(ctx, owner, true); err == nil || !errors.Is(err, ErrMerchantSuspended) {
		t.Fatalf("suspended merchant should reject owner, got %v", err)
	}
}
