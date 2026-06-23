// MERCHANT-SYSTEM v3.0
// 商户域名归属隔离（auth scope check）。
//
// 规则（v3.0 RFC §5.4）：
//   - 主站（请求无 merchant_ctx）只允许 user.parent_merchant_id IS NULL 登录。
//     普通用户、任何商户的 owner 都满足；商户子用户在此被拒。
//   - 商户域名（请求 merchant_ctx = M）只允许：
//       a) M 的子用户（user.parent_merchant_id = M.id）
//       b) M 的 owner（m.owner_user_id = user.id）
//     其他用户（主站普通用户、别的商户的子用户/owner）一律拒。
//   - 商户 status=suspended 时整站拒绝登录（哪怕是 owner 自己）。
//
// 用途：
//   - 登录入口（密码 / OAuth / 邮箱 token / refresh）在签发 JWT 前调用，阻止跨域名登录
//   - JWT 中间件解出用户后兜底校验，防止已签发的 token 被跨域名使用
//
// 这是一个纯函数，不需要任何 repo 依赖——MerchantContext 由 DomainDetect
// 中间件预先填好（含 *Merchant 对象，已带 owner_user_id 与 status）。

package service

import (
	"context"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// ErrUserDomainScopeMismatch 用户与当前请求域名不匹配（子用户跑到主站、跨商户访问等）。
var ErrUserDomainScopeMismatch = infraerrors.Forbidden(
	"USER_DOMAIN_MISMATCH",
	"user does not belong to this site; please access from the correct domain",
)

// ValidateUserDomainScope 按当前请求域名校验 user 是否有权在此登录/访问。
// merchantCfg.Enabled=false 时直接放行（商户系统关闭，整站等价主站）。
// user==nil 调用方应在上层先拒绝；此处只做归属校验。
func ValidateUserDomainScope(ctx context.Context, user *User, merchantEnabled bool) error {
	if user == nil {
		return nil
	}
	if !merchantEnabled {
		return nil
	}
	mctx := MerchantFromGoContext(ctx)

	// 主站
	if mctx == nil || mctx.Merchant == nil {
		if user.ParentMerchantID != nil {
			return ErrUserDomainScopeMismatch
		}
		return nil
	}

	// 商户域名：商户停用整站拒绝
	m := mctx.Merchant
	if m.Status != MerchantStatusActive || m.DeletedAt != nil {
		return ErrMerchantSuspended
	}

	// 子用户：parent_merchant_id 必须匹配当前 merchant
	if user.ParentMerchantID != nil {
		if *user.ParentMerchantID == m.ID {
			return nil
		}
		return ErrUserDomainScopeMismatch
	}

	// 非子用户：仅 owner 可在自己的商户域名登录
	if m.OwnerUserID == user.ID {
		return nil
	}
	return ErrUserDomainScopeMismatch
}
