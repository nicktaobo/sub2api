package repository

import (
	"context"
	"database/sql"
	"errors"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// affiliateConsumeOutboxRepository 邀请返利消费侧 outbox 的薄 repo。
// 与 merchant_outbox_repo 风格一致：插入路径走 usage_billing_repo 的内联 SQL（同事务），
// 这个 repo 只负责 worker 起停时的"还有没有积压"判断。
type affiliateConsumeOutboxRepository struct {
	db *sql.DB
}

func NewAffiliateConsumeOutboxRepository(_ *dbent.Client, db *sql.DB) service.AffiliateConsumeOutboxRepository {
	return &affiliateConsumeOutboxRepository{db: db}
}

func (r *affiliateConsumeOutboxRepository) HasPending(ctx context.Context) (bool, error) {
	if r == nil || r.db == nil {
		return false, errors.New("affiliate consume outbox repo db is nil")
	}
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM user_affiliate_consumption_outbox WHERE processed = FALSE LIMIT 1)
	`).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}
