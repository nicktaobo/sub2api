// MERCHANT-SYSTEM v1.0
// MerchantService 商户主体的 CRUD + 资金链路同步路径写入。
//
// 设计核心：
//   - 所有改动 discount/markup/status 写 merchant_audit_log + 主动 invalidate pricing cache
//   - PayToUser/AdminRecharge/AdminRefund/RedeemCreate/RedeemRefund：单事务内
//     扣/加 owner.balance + 写 merchant_ledger（同步路径）
//   - 不直接持有 PaymentService（避免循环依赖）

package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// MerchantService 商户系统的核心 service。
type MerchantService struct {
	cfg                *config.Config
	entClient          *dbent.Client
	repo               MerchantRepository
	domainRepo         MerchantDomainRepository
	ledgerRepo         MerchantLedgerRepository
	auditLogRepo       MerchantAuditLogRepository
	groupMarkupRepo    MerchantGroupMarkupRepository
	userRepo           UserRepository
	pricingService     *MerchantPricingService // 用于失效缓存
}

// NewMerchantService DI 构造函数。
func NewMerchantService(
	cfg *config.Config,
	entClient *dbent.Client,
	repo MerchantRepository,
	domainRepo MerchantDomainRepository,
	ledgerRepo MerchantLedgerRepository,
	auditLogRepo MerchantAuditLogRepository,
	groupMarkupRepo MerchantGroupMarkupRepository,
	userRepo UserRepository,
	pricingService *MerchantPricingService,
) *MerchantService {
	return &MerchantService{
		cfg:             cfg,
		entClient:       entClient,
		repo:            repo,
		domainRepo:      domainRepo,
		ledgerRepo:      ledgerRepo,
		auditLogRepo:    auditLogRepo,
		groupMarkupRepo: groupMarkupRepo,
		userRepo:        userRepo,
		pricingService:  pricingService,
	}
}

// ----------------------------------------------------------------------------
// Feature flag short-circuit helper
// ----------------------------------------------------------------------------

func (s *MerchantService) enabled() bool {
	return s != nil && s.cfg != nil && s.cfg.Merchant.Enabled
}

// ----------------------------------------------------------------------------
// 内部 helper：事务内读 owner.balance
// ----------------------------------------------------------------------------
// readOwnerBalanceInTx 走事务的 ent client 直读用户表：现有 userRepository.GetByID 不接受 tx context
// （读默认 client），事务内调用会拿到 commit 前的旧值——导致 merchant_ledger.balance_after 滞后一个事务。
// 这里改用 ent 的事务 client（dbuser.IDEQ）保证读到 +/- amount 的最新值。
func (s *MerchantService) readOwnerBalanceInTx(ctx context.Context, ownerUserID int64) (float64, error) {
	tx := dbent.TxFromContext(ctx)
	if tx == nil {
		return 0, errors.New("readOwnerBalanceInTx: no transaction in context")
	}
	m, err := tx.Client().User.Query().Where(dbuser.IDEQ(ownerUserID)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return 0, ErrUserNotFound
		}
		return 0, err
	}
	return m.Balance, nil
}

// ----------------------------------------------------------------------------
// Read APIs
// ----------------------------------------------------------------------------

func (s *MerchantService) GetByID(ctx context.Context, id int64) (*Merchant, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *MerchantService) GetByOwnerUserID(ctx context.Context, userID int64) (*Merchant, error) {
	return s.repo.GetByOwnerUserID(ctx, userID)
}

func (s *MerchantService) GetByDomain(ctx context.Context, domain string) (*Merchant, error) {
	return s.repo.GetByDomain(ctx, strings.TrimSpace(strings.ToLower(domain)))
}

func (s *MerchantService) List(ctx context.Context, status string, offset, limit int) ([]*Merchant, int, error) {
	return s.repo.List(ctx, status, offset, limit)
}

// ----------------------------------------------------------------------------
// CreateMerchant
// ----------------------------------------------------------------------------

// CreateMerchantInput 开通商户参数（admin 操作）。
type CreateMerchantInput struct {
	OwnerUserID         int64
	Name                string
	Discount            float64 // ∈ (0, 1]
	UserMarkupDefault   float64 // ≥ 1
	LowBalanceThreshold float64
	NotifyEmails        []string
	AdminID             int64
	Reason              string
}

func (s *MerchantService) CreateMerchant(ctx context.Context, in CreateMerchantInput) (*Merchant, error) {
	if !s.enabled() {
		return nil, ErrMerchantInvalidParam
	}
	if err := validateDiscount(in.Discount); err != nil {
		return nil, err
	}
	if err := validateMarkup(in.UserMarkupDefault); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, infraerrors.BadRequest("MERCHANT_NAME_REQUIRED", "merchant name is required")
	}

	owner, err := s.userRepo.GetByID(ctx, in.OwnerUserID)
	if err != nil {
		return nil, err
	}

	// 应用层守住：owner 不能同时是 sub_user
	if owner.ParentMerchantID != nil {
		return nil, ErrMerchantOwnerConflict
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	m := &Merchant{
		OwnerUserID:          in.OwnerUserID,
		Name:                 strings.TrimSpace(in.Name),
		Status:               MerchantStatusActive,
		Discount:             in.Discount,
		UserMarkupDefault:    in.UserMarkupDefault,
		OwnerBalanceBaseline: owner.Balance, // 开通时快照
		LowBalanceThreshold:  in.LowBalanceThreshold,
		NotifyEmails:         in.NotifyEmails,
	}
	if err := s.repo.Create(txCtx, m); err != nil {
		return nil, err
	}

	// 写一条 audit
	adminID := nullableAdminID(in.AdminID)
	reason := in.Reason
	newVal := fmt.Sprintf("merchant_id=%d owner_user_id=%d discount=%g markup_default=%g",
		m.ID, m.OwnerUserID, m.Discount, m.UserMarkupDefault)
	if err := s.auditLogRepo.Insert(txCtx, &MerchantAuditLogEntry{
		MerchantID: m.ID,
		AdminID:    adminID,
		Field:      "merchant_create",
		NewValue:   &newVal,
		Reason:     reason,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	// 缓存失效（owner 此后被识别为商户）
	if s.pricingService != nil {
		s.pricingService.InvalidateUser(in.OwnerUserID)
		s.pricingService.InvalidateMerchant(m.ID)
	}
	return m, nil
}

// ----------------------------------------------------------------------------
// 配置变更：discount / markup_default / status / group markup
// ----------------------------------------------------------------------------

// SetDiscount admin 修改 discount。
func (s *MerchantService) SetDiscount(ctx context.Context, merchantID int64, newDiscount float64, adminID int64, reason string) error {
	if !s.enabled() {
		return ErrMerchantInvalidParam
	}
	if err := validateDiscount(newDiscount); err != nil {
		return err
	}
	old, err := s.repo.GetByID(ctx, merchantID)
	if err != nil {
		return err
	}
	if old.Discount == newDiscount {
		return nil
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	if err := s.repo.UpdateDiscount(txCtx, merchantID, newDiscount); err != nil {
		return err
	}
	if err := s.writeAudit(txCtx, merchantID, adminID, MerchantAuditFieldDiscount,
		fmt.Sprintf("%g", old.Discount), fmt.Sprintf("%g", newDiscount), reason); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if s.pricingService != nil {
		s.pricingService.InvalidateMerchant(merchantID)
	}
	return nil
}

// SetMarkupDefault admin 修改商户级 markup 兜底。
func (s *MerchantService) SetMarkupDefault(ctx context.Context, merchantID int64, newMarkup float64, adminID int64, reason string) error {
	if !s.enabled() {
		return ErrMerchantInvalidParam
	}
	if err := validateMarkup(newMarkup); err != nil {
		return err
	}
	old, err := s.repo.GetByID(ctx, merchantID)
	if err != nil {
		return err
	}
	if old.UserMarkupDefault == newMarkup {
		return nil
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	if err := s.repo.UpdateMarkupDefault(txCtx, merchantID, newMarkup); err != nil {
		return err
	}
	if err := s.writeAudit(txCtx, merchantID, adminID, MerchantAuditFieldMarkupDef,
		fmt.Sprintf("%g", old.UserMarkupDefault), fmt.Sprintf("%g", newMarkup), reason); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if s.pricingService != nil {
		s.pricingService.InvalidateMerchant(merchantID)
	}
	return nil
}

// UpdateStatus admin 切换 active/suspended。
func (s *MerchantService) UpdateStatus(ctx context.Context, merchantID int64, newStatus string, adminID int64, reason string) error {
	if !s.enabled() {
		return ErrMerchantInvalidParam
	}
	if newStatus != MerchantStatusActive && newStatus != MerchantStatusSuspended {
		return ErrMerchantInvalidParam
	}
	old, err := s.repo.GetByID(ctx, merchantID)
	if err != nil {
		return err
	}
	if old.Status == newStatus {
		return nil
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	if err := s.repo.UpdateStatus(txCtx, merchantID, newStatus); err != nil {
		return err
	}
	if err := s.writeAudit(txCtx, merchantID, adminID, MerchantAuditFieldStatus,
		old.Status, newStatus, reason); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if s.pricingService != nil {
		s.pricingService.InvalidateMerchant(merchantID)
	}
	return nil
}

// SetGroupMarkup admin 设置某商户在某分组的 markup 覆盖。
func (s *MerchantService) SetGroupMarkup(ctx context.Context, merchantID, groupID int64, markup float64, adminID int64, reason string) error {
	if !s.enabled() {
		return ErrMerchantInvalidParam
	}
	if err := validateMarkup(markup); err != nil {
		return err
	}
	if _, err := s.repo.GetByID(ctx, merchantID); err != nil {
		return err
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	if err := s.groupMarkupRepo.Upsert(txCtx, &MerchantGroupMarkup{
		MerchantID: merchantID,
		GroupID:    groupID,
		Markup:     markup,
	}); err != nil {
		return err
	}
	newVal := fmt.Sprintf("group_id=%d markup=%g", groupID, markup)
	if err := s.writeAudit(txCtx, merchantID, adminID, MerchantAuditFieldGroupMarkup,
		"", newVal, reason); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if s.pricingService != nil {
		s.pricingService.InvalidateMerchant(merchantID)
	}
	return nil
}

// DeleteGroupMarkup admin 删除某分组覆盖（fallback 到 default）。
func (s *MerchantService) DeleteGroupMarkup(ctx context.Context, merchantID, groupID int64, adminID int64, reason string) error {
	if !s.enabled() {
		return ErrMerchantInvalidParam
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	if err := s.groupMarkupRepo.Delete(txCtx, merchantID, groupID); err != nil {
		return err
	}
	if err := s.writeAudit(txCtx, merchantID, adminID, MerchantAuditFieldGroupMarkup,
		fmt.Sprintf("group_id=%d", groupID), "(deleted)", reason); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if s.pricingService != nil {
		s.pricingService.InvalidateMerchant(merchantID)
	}
	return nil
}

func (s *MerchantService) ListGroupMarkups(ctx context.Context, merchantID int64) ([]*MerchantGroupMarkup, error) {
	return s.groupMarkupRepo.ListByMerchant(ctx, merchantID)
}

// ----------------------------------------------------------------------------
// PayToUser：商户给子用户充值（owner.balance → sub_user.balance）
// 单事务：扣 owner / 加 sub / 写 ledger ×1（debit/pay_to_user）
// ----------------------------------------------------------------------------

// PayToUser RFC §5.3.3 单边 owner debit。
func (s *MerchantService) PayToUser(ctx context.Context, merchantID, subUserID int64, amount float64, adminID int64, reason string) error {
	if !s.enabled() {
		return ErrMerchantInvalidParam
	}
	if amount <= 0 {
		return ErrMerchantInvalidParam
	}
	m, err := s.repo.GetByID(ctx, merchantID)
	if err != nil {
		return err
	}
	if m.Status != MerchantStatusActive {
		return ErrMerchantSuspended
	}
	subUser, err := s.userRepo.GetByID(ctx, subUserID)
	if err != nil {
		return err
	}
	if subUser.ParentMerchantID == nil || *subUser.ParentMerchantID != merchantID {
		return infraerrors.BadRequest("MERCHANT_USER_NOT_SUB", "user is not a sub-user of this merchant")
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	// 扣 owner.balance
	if err := s.userRepo.DeductBalance(txCtx, m.OwnerUserID, amount); err != nil {
		return err
	}
	// 加 sub_user.balance（不写 merchant_ledger 这一侧）
	if err := s.userRepo.UpdateBalance(txCtx, subUserID, amount); err != nil {
		return err
	}
	// 重新读 owner.balance 作为 balance_after（必须 tx 内读）
	bal, err := s.readOwnerBalanceInTx(txCtx, m.OwnerUserID)
	if err != nil {
		return err
	}
	idem := fmt.Sprintf("pay_to_user:%d:%d:%d", merchantID, subUserID, time.Now().UnixNano())
	refType := MerchantRefTypePaymentOrder // 概念上不指向 payment_order；标记为通用
	_ = refType                             // ledger ref_type/ref_id 留 NULL（PayToUser 不创建 payment_order）
	cuID := subUserID
	if err := s.ledgerRepo.Insert(txCtx, &MerchantLedgerEntry{
		MerchantID:         merchantID,
		OwnerUserID:        m.OwnerUserID,
		CounterpartyUserID: &cuID,
		Direction:          MerchantLedgerDirectionDebit,
		Amount:             amount,
		BalanceAfter:       &bal,
		Source:             MerchantSourcePayToUser,
		IdempotencyKey:     &idem,
	}); err != nil {
		return err
	}
	// 审计
	auditNew := fmt.Sprintf("sub_user_id=%d amount=%g", subUserID, amount)
	if err := s.writeAudit(txCtx, merchantID, adminID, "pay_to_user", "", auditNew, reason); err != nil {
		return err
	}

	return tx.Commit()
}

// ----------------------------------------------------------------------------
// AdminRecharge / AdminRefund
// ----------------------------------------------------------------------------

// AdminRecharge admin 给商户 owner 充值（绕过支付通道）。
func (s *MerchantService) AdminRecharge(ctx context.Context, merchantID int64, amount float64, adminID int64, reason string) error {
	if !s.enabled() {
		return ErrMerchantInvalidParam
	}
	if amount <= 0 {
		return ErrMerchantInvalidParam
	}
	m, err := s.repo.GetByID(ctx, merchantID)
	if err != nil {
		return err
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	if err := s.userRepo.UpdateBalance(txCtx, m.OwnerUserID, amount); err != nil {
		return err
	}
	bal, err := s.readOwnerBalanceInTx(txCtx, m.OwnerUserID)
	if err != nil {
		return err
	}
	idem := fmt.Sprintf("admin_recharge:%d:%d", merchantID, time.Now().UnixNano())
	if err := s.ledgerRepo.Insert(txCtx, &MerchantLedgerEntry{
		MerchantID:     merchantID,
		OwnerUserID:    m.OwnerUserID,
		Direction:      MerchantLedgerDirectionCredit,
		Amount:         amount,
		BalanceAfter:   &bal,
		Source:         MerchantSourceAdminRecharge,
		IdempotencyKey: &idem,
	}); err != nil {
		return err
	}
	auditNew := fmt.Sprintf("amount=%g", amount)
	if err := s.writeAudit(txCtx, merchantID, adminID, "admin_recharge", "", auditNew, reason); err != nil {
		return err
	}
	return tx.Commit()
}

// AdminRefund admin 从商户 owner 扣款。
func (s *MerchantService) AdminRefund(ctx context.Context, merchantID int64, amount float64, adminID int64, reason string) error {
	if !s.enabled() {
		return ErrMerchantInvalidParam
	}
	if amount <= 0 {
		return ErrMerchantInvalidParam
	}
	if strings.TrimSpace(reason) == "" {
		return infraerrors.BadRequest("MERCHANT_REFUND_REASON_REQUIRED", "refund reason is required")
	}
	m, err := s.repo.GetByID(ctx, merchantID)
	if err != nil {
		return err
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	if err := s.userRepo.DeductBalance(txCtx, m.OwnerUserID, amount); err != nil {
		return err
	}
	bal, err := s.readOwnerBalanceInTx(txCtx, m.OwnerUserID)
	if err != nil {
		return err
	}
	idem := fmt.Sprintf("admin_refund:%d:%d", merchantID, time.Now().UnixNano())
	if err := s.ledgerRepo.Insert(txCtx, &MerchantLedgerEntry{
		MerchantID:     merchantID,
		OwnerUserID:    m.OwnerUserID,
		Direction:      MerchantLedgerDirectionDebit,
		Amount:         amount,
		BalanceAfter:   &bal,
		Source:         MerchantSourceAdminRefund,
		IdempotencyKey: &idem,
	}); err != nil {
		return err
	}
	auditNew := fmt.Sprintf("amount=%g", amount)
	if err := s.writeAudit(txCtx, merchantID, adminID, "admin_refund", "", auditNew, reason); err != nil {
		return err
	}
	return tx.Commit()
}

// ----------------------------------------------------------------------------
// Sub-user binding
// ----------------------------------------------------------------------------

// BindSubUser 把 user 绑定到 merchant（注册时 / admin 操作）。
func (s *MerchantService) BindSubUser(ctx context.Context, merchantID, userID int64) error {
	if !s.enabled() {
		return ErrMerchantInvalidParam
	}
	m, err := s.repo.GetByID(ctx, merchantID)
	if err != nil {
		return err
	}
	if m.OwnerUserID == userID {
		return ErrMerchantOwnerConflict
	}
	if err := s.setParentMerchantID(ctx, userID, &merchantID); err != nil {
		return err
	}
	if s.pricingService != nil {
		s.pricingService.InvalidateUser(userID)
	}
	return nil
}

// UnbindSubUser admin 解绑 sub_user 回主站（RFC §5.3.4）。
func (s *MerchantService) UnbindSubUser(ctx context.Context, userID int64, adminID int64, reason string) error {
	if !s.enabled() {
		return ErrMerchantInvalidParam
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.ParentMerchantID == nil {
		return nil
	}
	merchantID := *user.ParentMerchantID
	if err := s.setParentMerchantID(ctx, userID, nil); err != nil {
		return err
	}
	// 写一条审计
	if err := s.writeAudit(ctx, merchantID, adminID, MerchantAuditFieldUnbindUser,
		strconv.FormatInt(userID, 10), "(unbound)", reason); err != nil {
		return err
	}
	if s.pricingService != nil {
		s.pricingService.InvalidateUser(userID)
	}
	return nil
}

// setParentMerchantID 直接 SQL 更新 users.parent_merchant_id（避免动 user_repo Update）。
func (s *MerchantService) setParentMerchantID(ctx context.Context, userID int64, merchantID *int64) error {
	client := s.entClient
	if tx := dbent.TxFromContext(ctx); tx != nil {
		client = tx.Client()
	}
	upd := client.User.UpdateOneID(userID)
	if merchantID == nil {
		upd = upd.ClearParentMerchantID()
	} else {
		upd = upd.SetParentMerchantID(*merchantID)
	}
	_, err := upd.Save(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return ErrUserNotFound
		}
		return err
	}
	return nil
}

// ----------------------------------------------------------------------------
// Audit helpers
// ----------------------------------------------------------------------------

func (s *MerchantService) writeAudit(ctx context.Context, merchantID, adminID int64, field, oldVal, newVal, reason string) error {
	entry := &MerchantAuditLogEntry{
		MerchantID: merchantID,
		AdminID:    nullableAdminID(adminID),
		Field:      field,
		Reason:     reason,
	}
	if oldVal != "" {
		v := oldVal
		entry.OldValue = &v
	}
	if newVal != "" {
		v := newVal
		entry.NewValue = &v
	}
	return s.auditLogRepo.Insert(ctx, entry)
}

func (s *MerchantService) ListAuditLog(ctx context.Context, merchantID int64, offset, limit int) ([]*MerchantAuditLogEntry, int, error) {
	return s.auditLogRepo.ListByMerchant(ctx, merchantID, offset, limit)
}

// GetDomain 按 domain 字符串查 verified domain 详情（DomainDetect 中间件用）。
// 返回未 verified / 不存在 / 软删除域名时返回 nil（不报错）。
func (s *MerchantService) GetDomain(ctx context.Context, domain string) (*MerchantDomain, error) {
	d, err := s.domainRepo.GetByDomain(ctx, strings.TrimSpace(strings.ToLower(domain)))
	if err != nil {
		return nil, nil // 静默：域名查不到不阻塞业务
	}
	if d == nil || !d.Verified || d.DeletedAt != nil {
		return nil, nil
	}
	return d, nil
}

func (s *MerchantService) ListDomains(ctx context.Context, merchantID int64) ([]*MerchantDomain, error) {
	return s.domainRepo.ListByMerchant(ctx, merchantID)
}

func (s *MerchantService) ListLedger(ctx context.Context, merchantID int64, offset, limit int) ([]*MerchantLedgerEntry, int, error) {
	return s.ledgerRepo.List(ctx, merchantID, offset, limit)
}

// ----------------------------------------------------------------------------
// 验证
// ----------------------------------------------------------------------------

func validateDiscount(discount float64) error {
	if discount <= 0 || discount > 1 {
		return infraerrors.BadRequest("MERCHANT_DISCOUNT_OUT_OF_RANGE",
			"discount must be in (0, 1]")
	}
	return nil
}

func validateMarkup(markup float64) error {
	if markup < 1 {
		return infraerrors.BadRequest("MERCHANT_MARKUP_OUT_OF_RANGE",
			"markup must be ≥ 1 (use 1.0 to disable markup)")
	}
	return nil
}

func nullableAdminID(adminID int64) *int64 {
	if adminID <= 0 {
		return nil
	}
	v := adminID
	return &v
}

// 占位防止 errors unused
var _ = errors.Is
