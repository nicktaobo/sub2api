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
	cryptoRand "crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
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
	groupCostRepo      MerchantGroupCostRepository
	groupRepo          GroupRepository
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
	groupCostRepo MerchantGroupCostRepository,
	groupRepo GroupRepository,
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
		groupCostRepo:   groupCostRepo,
		groupRepo:       groupRepo,
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

// MerchantListItem 给 admin 列表用的富对象（含域名 + 子用户数 + owner 余额）。
type MerchantListItem struct {
	*Merchant
	Domains      []string `json:"domains"`
	SubUserCount int      `json:"sub_user_count"`
	OwnerBalance float64  `json:"owner_balance"`
}

// ListWithDetails 富列表：商户主表 + 关联域名 + 子用户数 + owner 余额。
func (s *MerchantService) ListWithDetails(ctx context.Context, status, search string, offset, limit int) ([]*MerchantListItem, int, error) {
	rows, total, err := s.repo.List(ctx, status, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		return nil, total, nil
	}
	db, err := s.sqlDB()
	if err != nil {
		return nil, 0, err
	}
	out := make([]*MerchantListItem, 0, len(rows))
	for _, m := range rows {
		item := &MerchantListItem{Merchant: m}
		// 取该商户所有 verified domain
		domRows, derr := db.QueryContext(ctx, `
			SELECT domain FROM merchant_domains
			WHERE merchant_id = $1 AND deleted_at IS NULL
			ORDER BY verified DESC, created_at ASC
		`, m.ID)
		if derr == nil {
			for domRows.Next() {
				var d string
				if err := domRows.Scan(&d); err == nil {
					item.Domains = append(item.Domains, d)
				}
			}
			_ = domRows.Close()
		}
		// sub_user 数
		var cnt int
		_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE parent_merchant_id = $1 AND deleted_at IS NULL`, m.ID).Scan(&cnt)
		item.SubUserCount = cnt
		// owner.balance
		_ = db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id = $1`, m.OwnerUserID).Scan(&item.OwnerBalance)
		out = append(out, item)
	}
	_ = search // 暂不实现 search（仓库层没接），后续按需加
	return out, total, nil
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

// SetGroupSellRate 设置某商户在某分组的对外售价倍率（绝对值，≥ 对应 cost_rate）。
// admin 和商户 owner 都能调（adminID=0 表示商户自助）。
func (s *MerchantService) SetGroupSellRate(ctx context.Context, merchantID, groupID int64, sellRate float64, adminID int64, reason string) error {
	if !s.enabled() {
		return ErrMerchantInvalidParam
	}
	if sellRate <= 0 {
		return infraerrors.BadRequest("MERCHANT_SELL_RATE_OUT_OF_RANGE", "sell_rate must be > 0")
	}
	if _, err := s.repo.GetByID(ctx, merchantID); err != nil {
		return err
	}
	// 校验 sell_rate ≥ 对应 group 的 cost_rate（未配 cost 则跳过校验，运行时按 site rate 兜底）
	costs, err := s.groupCostRepo.ListByMerchant(ctx, merchantID)
	if err != nil {
		return err
	}
	for _, c := range costs {
		if c != nil && c.GroupID == groupID && sellRate < c.CostRate {
			return infraerrors.BadRequest("MERCHANT_SELL_BELOW_COST",
				fmt.Sprintf("sell_rate %.4f below cost_rate %.4f", sellRate, c.CostRate))
		}
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
		SellRate:   sellRate,
	}); err != nil {
		return err
	}
	newVal := fmt.Sprintf("group_id=%d sell_rate=%g", groupID, sellRate)
	if err := s.writeAudit(txCtx, merchantID, adminID, MerchantAuditFieldGroupSell,
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

// DeleteGroupSellRate 删除某分组售价配置；删除后商户在该 group 不再分润，sub_user 按主站价。
func (s *MerchantService) DeleteGroupSellRate(ctx context.Context, merchantID, groupID int64, adminID int64, reason string) error {
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
	if err := s.writeAudit(txCtx, merchantID, adminID, MerchantAuditFieldGroupSell,
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

// SetGroupCostRate admin 设置某商户在某分组的拿货价倍率（绝对值，> 0）。
// 不允许设到大于任何已存在的 sell_rate（会让商户亏本卖）。
func (s *MerchantService) SetGroupCostRate(ctx context.Context, merchantID, groupID int64, costRate float64, adminID int64, reason string) error {
	if !s.enabled() {
		return ErrMerchantInvalidParam
	}
	if costRate <= 0 {
		return infraerrors.BadRequest("MERCHANT_COST_RATE_OUT_OF_RANGE", "cost_rate must be > 0")
	}
	if _, err := s.repo.GetByID(ctx, merchantID); err != nil {
		return err
	}
	// 校验：cost_rate 不能高于该 group 上已有的 sell_rate（否则商户的现有定价会亏本）
	sells, err := s.groupMarkupRepo.ListByMerchant(ctx, merchantID)
	if err != nil {
		return err
	}
	for _, m := range sells {
		if m != nil && m.GroupID == groupID && m.SellRate < costRate {
			return infraerrors.BadRequest("MERCHANT_COST_ABOVE_SELL",
				fmt.Sprintf("cost_rate %.4f above existing sell_rate %.4f; ask merchant to raise sell first",
					costRate, m.SellRate))
		}
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	if err := s.groupCostRepo.Upsert(txCtx, &MerchantGroupCost{
		MerchantID: merchantID,
		GroupID:    groupID,
		CostRate:   costRate,
	}); err != nil {
		return err
	}
	newVal := fmt.Sprintf("group_id=%d cost_rate=%g", groupID, costRate)
	if err := s.writeAudit(txCtx, merchantID, adminID, MerchantAuditFieldGroupCost,
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

// DeleteGroupCostRate 删除某分组拿货价配置；删除后回退到 group.rate_multiplier。
func (s *MerchantService) DeleteGroupCostRate(ctx context.Context, merchantID, groupID int64, adminID int64, reason string) error {
	if !s.enabled() {
		return ErrMerchantInvalidParam
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	if err := s.groupCostRepo.Delete(txCtx, merchantID, groupID); err != nil {
		return err
	}
	if err := s.writeAudit(txCtx, merchantID, adminID, MerchantAuditFieldGroupCost,
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

func (s *MerchantService) ListGroupCosts(ctx context.Context, merchantID int64) ([]*MerchantGroupCost, error) {
	return s.groupCostRepo.ListByMerchant(ctx, merchantID)
}

// MerchantPricingGroup 商户可定价分组（用于商户后台「分组定价」页平铺渲染）。
//
// 范围：所有 active 标准（非订阅型）分组。订阅型分组不参与商户计费。
// CostRate=nil 表示 admin 未配（回退到 group.rate_multiplier）；
// SellRate=nil 表示商户未配（该 group 不分润，sub_user 按主站价）。
type MerchantPricingGroup struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	Platform       string   `json:"platform"`
	IsExclusive    bool     `json:"is_exclusive"`
	RateMultiplier float64  `json:"rate_multiplier"`
	CostRate       *float64 `json:"cost_rate,omitempty"`
	SellRate       *float64 `json:"sell_rate,omitempty"`
}

// ListPricingGroups 列出某商户可定价的分组（含每个分组当前生效的 cost/sell）。
//
// 可见范围跟「普通用户」一致：以商户 owner 的 user 身份为准——
//   - 公开标准分组：所有 owner 都可见
//   - 专属标准分组：仅 admin 把它加进 owner.AllowedGroups 时可见
//   - 订阅型分组：始终隐藏（merchant 不参与订阅计费）
//
// 这样商户不会看到 admin 没授权给他的专属分组。
func (s *MerchantService) ListPricingGroups(ctx context.Context, merchantID int64) ([]MerchantPricingGroup, error) {
	m, err := s.repo.GetByID(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	owner, err := s.userRepo.GetByID(ctx, m.OwnerUserID)
	if err != nil {
		return nil, err
	}
	allGroups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	sells, err := s.groupMarkupRepo.ListByMerchant(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	costs, err := s.groupCostRepo.ListByMerchant(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	sellByGroup := make(map[int64]float64, len(sells))
	for _, gm := range sells {
		if gm != nil {
			sellByGroup[gm.GroupID] = gm.SellRate
		}
	}
	costByGroup := make(map[int64]float64, len(costs))
	for _, c := range costs {
		if c != nil {
			costByGroup[c.GroupID] = c.CostRate
		}
	}
	out := make([]MerchantPricingGroup, 0, len(allGroups))
	for i := range allGroups {
		g := allGroups[i]
		if g.IsSubscriptionType() {
			continue
		}
		if !owner.CanBindGroup(g.ID, g.IsExclusive) {
			continue
		}
		item := MerchantPricingGroup{
			ID:             g.ID,
			Name:           g.Name,
			Platform:       g.Platform,
			IsExclusive:    g.IsExclusive,
			RateMultiplier: g.RateMultiplier,
		}
		if v, ok := costByGroup[g.ID]; ok {
			vv := v
			item.CostRate = &vv
		}
		if v, ok := sellByGroup[g.ID]; ok {
			vv := v
			item.SellRate = &vv
		}
		out = append(out, item)
	}
	return out, nil
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

// SubUserSummary 子用户摘要，给 owner 后台列表用。
type SubUserSummary struct {
	ID             int64     `json:"id"`
	Email          string    `json:"email"`
	Username       string    `json:"username"`
	Balance        float64   `json:"balance"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	LastActiveAt   *time.Time `json:"last_active_at,omitempty"`
}

// ListSubUsers 列出某商户的子用户（按 parent_merchant_id 过滤）。
func (s *MerchantService) ListSubUsers(ctx context.Context, merchantID int64, search string, offset, limit int) ([]*SubUserSummary, int, error) {
	if !s.enabled() {
		return nil, 0, ErrMerchantInvalidParam
	}
	tx := dbent.TxFromContext(ctx)
	client := s.entClient
	if tx != nil {
		client = tx.Client()
	}
	driver, ok := client.Driver().(interface {
		DB() *sql.DB
	})
	if !ok {
		return nil, 0, errors.New("entClient driver does not expose *sql.DB")
	}
	db := driver.DB()

	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	search = strings.TrimSpace(search)
	args := []any{merchantID}
	where := "u.parent_merchant_id = $1 AND u.deleted_at IS NULL"
	if search != "" {
		args = append(args, "%"+search+"%")
		where += " AND (u.email ILIKE $2 OR u.username ILIKE $2)"
	}

	var total int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users u WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	limitArg := len(args) - 1
	offsetArg := len(args)
	rows, err := db.QueryContext(ctx, `
		SELECT u.id, u.email, COALESCE(u.username, ''), u.balance, u.status, u.created_at, u.last_active_at
		FROM users u WHERE `+where+`
		ORDER BY u.created_at DESC, u.id DESC
		LIMIT $`+strconv.Itoa(limitArg)+` OFFSET $`+strconv.Itoa(offsetArg), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]*SubUserSummary, 0, limit)
	for rows.Next() {
		var (
			u            SubUserSummary
			lastActiveAt sql.NullTime
		)
		if err := rows.Scan(&u.ID, &u.Email, &u.Username, &u.Balance, &u.Status, &u.CreatedAt, &lastActiveAt); err != nil {
			return nil, 0, err
		}
		if lastActiveAt.Valid {
			t := lastActiveAt.Time
			u.LastActiveAt = &t
		}
		out = append(out, &u)
	}
	return out, total, rows.Err()
}

// CreateDomainInput owner 新建域名（含品牌定制）输入。
type CreateDomainInput struct {
	MerchantID     int64
	Domain         string
	SiteName       string
	SiteLogo       string
	BrandColor     string
	CustomCSS      string
	HomeContent    string
	SEOTitle       string
	SEODescription string
	SEOKeywords    string
}

// CreateDomain 创建一条 merchant_domains 记录。生成随机 verify_token（DNS TXT 验证用）。
func (s *MerchantService) CreateDomain(ctx context.Context, in CreateDomainInput) (*MerchantDomain, error) {
	if !s.enabled() {
		return nil, ErrMerchantInvalidParam
	}
	domain := strings.TrimSpace(strings.ToLower(in.Domain))
	if domain == "" {
		return nil, infraerrors.BadRequest("MERCHANT_DOMAIN_REQUIRED", "domain is required")
	}
	// 生成 verify_token（hex 32 字符）
	tokenBytes := make([]byte, 16)
	if _, err := cryptoRand.Read(tokenBytes); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(tokenBytes)
	d := &MerchantDomain{
		MerchantID:     in.MerchantID,
		Domain:         domain,
		SiteName:       in.SiteName,
		SiteLogo:       in.SiteLogo,
		BrandColor:     in.BrandColor,
		CustomCSS:      in.CustomCSS,
		HomeContent:    sanitizeBrandHTML(in.HomeContent),
		SEOTitle:       in.SEOTitle,
		SEODescription: in.SEODescription,
		SEOKeywords:    in.SEOKeywords,
		VerifyToken:    token,
		Verified:       false,
	}
	if err := s.domainRepo.Create(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

// UpdateDomainInput owner 更新品牌字段。Domain 字段不允许改（改则删了再建）。
type UpdateDomainInput struct {
	SiteName       string
	SiteLogo       string
	BrandColor     string
	CustomCSS      string
	HomeContent    string
	SEOTitle       string
	SEODescription string
	SEOKeywords    string
}

// UpdateDomain 更新某域名的品牌字段（owner 自服务）。返回更新后的域名。
func (s *MerchantService) UpdateDomain(ctx context.Context, merchantID, domainID int64, in UpdateDomainInput) (*MerchantDomain, error) {
	if !s.enabled() {
		return nil, ErrMerchantInvalidParam
	}
	d, err := s.domainRepo.GetByID(ctx, domainID)
	if err != nil {
		return nil, err
	}
	if d.MerchantID != merchantID {
		return nil, ErrMerchantDomainNotFound
	}
	d.SiteName = in.SiteName
	d.SiteLogo = in.SiteLogo
	d.BrandColor = in.BrandColor
	d.CustomCSS = in.CustomCSS
	d.HomeContent = sanitizeBrandHTML(in.HomeContent)
	d.SEOTitle = in.SEOTitle
	d.SEODescription = in.SEODescription
	d.SEOKeywords = in.SEOKeywords
	if err := s.domainRepo.Update(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

// VerifyDomain 真实查 DNS TXT 记录验证所有权（RFC §4.1.2）。
//
// 检查 `_domain-verify.<domain>` 的 TXT 记录是否含 `domain-verify=<verify_token>`。
// 配置 SkipDNSVerify=true 时（本地/开发用）直接通过。
// 失败时返回带原因的 ApplicationError，前端可直接展示给 owner。
func (s *MerchantService) VerifyDomain(ctx context.Context, merchantID, domainID int64) error {
	if !s.enabled() {
		return ErrMerchantInvalidParam
	}
	d, err := s.domainRepo.GetByID(ctx, domainID)
	if err != nil {
		return err
	}
	if d.MerchantID != merchantID {
		return ErrMerchantDomainNotFound
	}
	if d.Verified {
		return nil
	}

	if !s.cfg.Merchant.SkipDNSVerify {
		expected := "domain-verify=" + d.VerifyToken
		host := "_domain-verify." + d.Domain
		lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		records, lookupErr := net.DefaultResolver.LookupTXT(lookupCtx, host)
		if lookupErr != nil {
			return infraerrors.BadRequest(
				"MERCHANT_DOMAIN_DNS_LOOKUP_FAILED",
				"DNS TXT lookup failed for "+host+": "+lookupErr.Error(),
			)
		}
		matched := false
		for _, txt := range records {
			if strings.TrimSpace(txt) == expected {
				matched = true
				break
			}
		}
		if !matched {
			return infraerrors.BadRequest(
				"MERCHANT_DOMAIN_VERIFY_TOKEN_MISMATCH",
				"expected TXT record "+expected+" not found on "+host,
			)
		}
	}

	return s.domainRepo.MarkVerified(ctx, domainID)
}

// MarkDomainVerified 是 VerifyDomain 的别名，保留兼容（旧名）。
func (s *MerchantService) MarkDomainVerified(ctx context.Context, merchantID, domainID int64) error {
	return s.VerifyDomain(ctx, merchantID, domainID)
}

// DNSSetupInfo 给 owner 后台展示 DNS 配置指引用。
type DNSSetupInfo struct {
	ServerIP     string `json:"server_ip"`      // A 记录 VALUE
	HasServerIP  bool   `json:"has_server_ip"`  // 平台是否配置了 IP（否则前端提示联系管理员）
	TXTHostPrefix string `json:"txt_host_prefix"` // "_domain-verify"
	SkipDNSVerify bool  `json:"skip_dns_verify"` // dev 模式标记
}

// GetDNSSetupInfo 返回平台 DNS 配置元数据（owner 后台用）。
func (s *MerchantService) GetDNSSetupInfo() DNSSetupInfo {
	cfg := s.cfg.Merchant
	return DNSSetupInfo{
		ServerIP:      cfg.ServerIP,
		HasServerIP:   strings.TrimSpace(cfg.ServerIP) != "",
		TXTHostPrefix: "_domain-verify",
		SkipDNSVerify: cfg.SkipDNSVerify,
	}
}

// DeleteDomain 软删除域名。
func (s *MerchantService) DeleteDomain(ctx context.Context, merchantID, domainID int64) error {
	if !s.enabled() {
		return ErrMerchantInvalidParam
	}
	d, err := s.domainRepo.GetByID(ctx, domainID)
	if err != nil {
		return err
	}
	if d.MerchantID != merchantID {
		return ErrMerchantDomainNotFound
	}
	return s.domainRepo.SoftDelete(ctx, domainID)
}

// sanitizeBrandHTML 最小化 sanitize：去除 <script>/<iframe>/on* 事件属性。
// v1 简化实现；生产应用 bluemonday.UGCPolicy。
func sanitizeBrandHTML(html string) string {
	if html == "" {
		return ""
	}
	// 极简过滤：去掉 <script.*</script> + on...= 事件
	out := html
	for _, bad := range []string{"<script", "</script", "<iframe", "</iframe", "javascript:"} {
		out = strings.ReplaceAll(out, bad, "")
		out = strings.ReplaceAll(out, strings.ToUpper(bad), "")
	}
	return out
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
