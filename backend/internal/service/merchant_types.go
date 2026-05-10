// MERCHANT-SYSTEM v1.0
// 商户系统 service 层域类型 + repository 接口 + 错误定义。
//
// 设计原则（RFC §2）：
//   - 单一资金来源：merchant.owner_user_id 引用的 user.balance 即商户池子
//   - 比例只在事件发生时立即固化为金额快照（discount/markup 历史不可变）
//   - 所有写入路径有幂等键（idempotency_key UNIQUE）

package service

import (
	"context"
	"errors"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// ----------------------------------------------------------------------------
// 错误定义
// ----------------------------------------------------------------------------

var (
	ErrMerchantNotFound       = infraerrors.NotFound("MERCHANT_NOT_FOUND", "merchant not found")
	ErrMerchantSuspended      = infraerrors.Forbidden("MERCHANT_SUSPENDED", "merchant is suspended")
	ErrMerchantOwnerConflict  = infraerrors.Conflict("MERCHANT_OWNER_CONFLICT", "user is already the owner of another merchant")
	ErrMerchantInvalidParam   = infraerrors.BadRequest("MERCHANT_INVALID_PARAM", "merchant parameter out of range")
	ErrMerchantDomainNotFound = infraerrors.NotFound("MERCHANT_DOMAIN_NOT_FOUND", "merchant domain not found")
	ErrMerchantDomainConflict = infraerrors.Conflict("MERCHANT_DOMAIN_CONFLICT", "domain already used by another merchant")

	// errMerchantOutboxAlreadyExists 是 InsertIfNotExists 命中 idempotency_key 时返回的内部信号。
	// 调用方应 errors.Is 判断后视为成功语义（RFC §5.2.2 / §5.2.4）。
	ErrMerchantOutboxAlreadyExists = errors.New("merchant earnings outbox row already exists (idempotency key conflict)")
)

// ----------------------------------------------------------------------------
// 状态/枚举常量
// ----------------------------------------------------------------------------

const (
	MerchantStatusActive    = "active"
	MerchantStatusSuspended = "suspended"

	MerchantLedgerDirectionCredit = "credit"
	MerchantLedgerDirectionDebit  = "debit"

	// outbox/ledger source 枚举（RFC §4.2.2.3）
	MerchantSourceUserMarkupShare   = "user_markup_share"
	MerchantSourceUserRechargeShare = "user_recharge_share"
	MerchantSourceSelfRecharge      = "self_recharge"
	MerchantSourcePayToUser         = "pay_to_user"
	MerchantSourceRedeemCreate      = "redeem_create"
	MerchantSourceRedeemRefund      = "redeem_refund"
	MerchantSourceAdminRecharge     = "admin_recharge"
	MerchantSourceAdminRefund       = "admin_refund"
	MerchantSourceOwnerUsageDebit   = "owner_usage_debit"

	// ledger ref_type 枚举
	MerchantRefTypePaymentOrder      = "payment_order"
	MerchantRefTypeUsageBillingDedup = "usage_billing_dedup"
	MerchantRefTypeRedeemCode        = "redeem_code"
	MerchantRefTypeOutboxBatch       = "outbox_batch"

	// audit log field 枚举
	MerchantAuditFieldDiscount     = "discount"
	MerchantAuditFieldMarkupDef    = "user_markup_default"
	MerchantAuditFieldGroupMarkup  = "group_markup"
	MerchantAuditFieldStatus       = "status"
	MerchantAuditFieldDomainAdd    = "domain_add"
	MerchantAuditFieldDomainRemove = "domain_remove"
	MerchantAuditFieldDomainVerify = "domain_verify"
	MerchantAuditFieldUnbindUser   = "unbind_user"
)

// ----------------------------------------------------------------------------
// 域类型
// ----------------------------------------------------------------------------

// Merchant 商户主体（RFC §4.1.1）。
type Merchant struct {
	ID                   int64
	OwnerUserID          int64
	Name                 string
	Status               string
	Discount             float64
	UserMarkupDefault    float64
	OwnerBalanceBaseline float64
	LowBalanceThreshold  float64
	NotifyEmails         []string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	DeletedAt            *time.Time
}

// MerchantDomain 商户自定义域名 + 站点品牌（RFC §4.1.2）。
type MerchantDomain struct {
	ID              int64
	MerchantID      int64
	Domain          string
	SiteName        string
	SiteLogo        string
	BrandColor      string
	CustomCSS       string
	HomeContent     string
	SEOTitle        string
	SEODescription  string
	SEOKeywords     string
	VerifyToken     string
	Verified        bool
	VerifiedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

// MerchantLedgerEntry owner-only 资金流水（RFC §4.2.2）。
type MerchantLedgerEntry struct {
	ID                 int64
	MerchantID         int64
	OwnerUserID        int64
	CounterpartyUserID *int64
	Direction          string  // credit / debit
	Amount             float64 // 正数
	BalanceAfter       *float64
	IsAggregated       bool
	AggregatedCount    *int
	Source             string
	RefType            *string
	RefID              *int64
	IdempotencyKey     *string
	Note               *string
	CreatedAt          time.Time
}

// MerchantOutboxEntry 网关→worker 临时队列条目（RFC §4.1.3）。
type MerchantOutboxEntry struct {
	ID                 int64
	MerchantID         int64
	CounterpartyUserID *int64
	Amount             float64
	Source             string
	RefType            string
	RefID              int64
	IdempotencyKey     string
	Processed          bool
	ProcessedAt        *time.Time
	CreatedAt          time.Time
}

// MerchantAuditLogEntry 配置/操作审计（RFC §4.1.4）。
type MerchantAuditLogEntry struct {
	ID         int64
	MerchantID int64
	AdminID    *int64
	Field      string
	OldValue   *string
	NewValue   *string
	Reason     string
	CreatedAt  time.Time
}

// MerchantGroupMarkup 分组级 markup 覆盖（RFC §4.1.5）。
type MerchantGroupMarkup struct {
	ID         int64
	MerchantID int64
	GroupID    int64
	Markup     float64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ----------------------------------------------------------------------------
// 错误类型：MerchantBlockingError (RFC §5.2 / v1.11 P1 / v1.12 P1-#1)
// ----------------------------------------------------------------------------

// MerchantBlockingError 标记 merchant hook 在 INTENT 写入阶段失败，
// 必须阻塞 markCompleted 让订单 markFailed → admin retry 时进入 skipCompleted 重跑 hook。
//
// 仅 INTENT 阶段使用；outbox 阶段失败返回普通 error，由 caller 走非阻塞 + reconcile。
type MerchantBlockingError struct {
	Stage string // "intent_write"
	Err   error
}

func (e *MerchantBlockingError) Error() string {
	if e == nil || e.Err == nil {
		return "merchant blocking error"
	}
	return e.Err.Error()
}

func (e *MerchantBlockingError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// IsMerchantBlockingError 判断 err 是否为 INTENT 阶段失败需要阻塞的错误。
// 必须用 errors.As 判断（不能直接 ==），支持 fmt.Errorf %w 嵌套包装。
func IsMerchantBlockingError(err error) bool {
	var be *MerchantBlockingError
	return errors.As(err, &be)
}

// ----------------------------------------------------------------------------
// Repository 接口
// ----------------------------------------------------------------------------

// MerchantRepository 商户主表仓储接口。
type MerchantRepository interface {
	Create(ctx context.Context, m *Merchant) error
	GetByID(ctx context.Context, id int64) (*Merchant, error)
	GetByOwnerUserID(ctx context.Context, userID int64) (*Merchant, error)
	GetByDomain(ctx context.Context, domain string) (*Merchant, error)
	List(ctx context.Context, status string, offset, limit int) ([]*Merchant, int, error)
	Update(ctx context.Context, m *Merchant) error
	UpdateStatus(ctx context.Context, id int64, status string) error
	UpdateDiscount(ctx context.Context, id int64, discount float64) error
	UpdateMarkupDefault(ctx context.Context, id int64, markup float64) error
	SoftDelete(ctx context.Context, id int64) error

	// LookupMerchantIDForUser 按 user_id 反查 merchant_id（同时识别 sub_user 与 owner）。
	// 返回 0 表示不属于任何商户（普通主站用户）。RFC §5.2.1 Step 2.0。
	LookupMerchantIDForUser(ctx context.Context, userID int64) (int64, error)

	// LoadPricing 一次性加载某商户的 discount/markup_default + 所有 active group markups（用于 pricing cache）。
	LoadPricing(ctx context.Context, merchantID int64) (*CachedMerchantPricing, error)
}

// MerchantDomainRepository 域名仓储。
type MerchantDomainRepository interface {
	Create(ctx context.Context, d *MerchantDomain) error
	GetByDomain(ctx context.Context, domain string) (*MerchantDomain, error)
	GetByID(ctx context.Context, id int64) (*MerchantDomain, error)
	ListByMerchant(ctx context.Context, merchantID int64) ([]*MerchantDomain, error)
	Update(ctx context.Context, d *MerchantDomain) error
	MarkVerified(ctx context.Context, id int64) error
	SoftDelete(ctx context.Context, id int64) error
}

// MerchantLedgerRepository owner 钱包流水（同步路径直接写）。
type MerchantLedgerRepository interface {
	// Insert 同步写一条 ledger（service 事务内调用）。
	Insert(ctx context.Context, e *MerchantLedgerEntry) error
	// List 分页查询某商户的 ledger 流水。
	List(ctx context.Context, merchantID int64, offset, limit int) ([]*MerchantLedgerEntry, int, error)
}

// MerchantOutboxRepository 异步分润队列。
type MerchantOutboxRepository interface {
	// InsertIfNotExists 写入一行；若 idempotency_key 冲突返回 ErrMerchantOutboxAlreadyExists（视为成功）。
	// 必须从 ctx 取 *sql.Tx（如调用方在事务内），否则用默认 db。RFC §5.2.2。
	InsertIfNotExists(ctx context.Context, e *MerchantOutboxEntry) error
	// HasPending 检查是否还有 processed=false 行（worker 在 flag 关闭后的积压判断）。
	HasPending(ctx context.Context) (bool, error)
}

// MerchantAuditLogRepository 审计日志仓储。
type MerchantAuditLogRepository interface {
	Insert(ctx context.Context, e *MerchantAuditLogEntry) error
	ListByMerchant(ctx context.Context, merchantID int64, offset, limit int) ([]*MerchantAuditLogEntry, int, error)
}

// MerchantGroupMarkupRepository 分组级 markup 覆盖仓储。
type MerchantGroupMarkupRepository interface {
	Upsert(ctx context.Context, e *MerchantGroupMarkup) error
	Delete(ctx context.Context, merchantID, groupID int64) error
	ListByMerchant(ctx context.Context, merchantID int64) ([]*MerchantGroupMarkup, error)
}

// CachedMerchantPricing pricing hook 用的缓存对象（merchant 维度，不含 user 信息）。
// RFC §5.2.1 Step 2 / Step 2.1。
type CachedMerchantPricing struct {
	MerchantID        int64
	OwnerUserID       int64
	Status            string
	Discount          float64
	UserMarkupDefault float64
	GroupMarkups      map[int64]float64 // 已过滤 deleted_at 的 active groups
}
