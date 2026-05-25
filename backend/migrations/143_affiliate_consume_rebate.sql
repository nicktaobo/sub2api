-- 邀请返利：消费返利扩展（阶段 1 基础设施）。
-- 1) groups.affiliate_rebate_excluded —— 标记某个分组不参与邀请返利分成（消费场景）。
-- 2) user_affiliate_ledger.source_type —— 区分返利来源：recharge / consume / legacy；transfer 行保持 NULL。
-- 3) user_affiliate_consumption_outbox —— 仿 merchant_earnings_outbox 的异步分润队列。

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS affiliate_rebate_excluded BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN groups.affiliate_rebate_excluded IS '该分组不参与邀请返利（消费侧），默认参与';

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS source_type VARCHAR(16) NULL;

COMMENT ON COLUMN user_affiliate_ledger.source_type IS 'accrue 行来源：recharge / consume / legacy；transfer 行保持 NULL';

-- 回填存量 accrue 行：有订单关联 → recharge；其余 → legacy（无法可靠区分）。
UPDATE user_affiliate_ledger
SET source_type = CASE WHEN source_order_id IS NOT NULL THEN 'recharge' ELSE 'legacy' END
WHERE action = 'accrue' AND source_type IS NULL;

CREATE TABLE IF NOT EXISTS user_affiliate_consumption_outbox (
    id BIGSERIAL PRIMARY KEY,
    inviter_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitee_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount DECIMAL(20,8) NOT NULL CHECK (amount > 0),
    ref_type VARCHAR(32) NOT NULL DEFAULT 'usage_billing_dedup',
    ref_id BIGINT NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    processed BOOLEAN NOT NULL DEFAULT FALSE,
    processed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_aff_consume_outbox_idemkey
    ON user_affiliate_consumption_outbox (idempotency_key);

-- worker 轮询：未处理行，按时间顺序拉。部分索引避免被已处理行膨胀。
CREATE INDEX IF NOT EXISTS idx_aff_consume_outbox_pending
    ON user_affiliate_consumption_outbox (created_at, id)
    WHERE processed = FALSE;

COMMENT ON TABLE user_affiliate_consumption_outbox IS '邀请返利消费侧异步分润队列（仿 merchant_earnings_outbox）';
COMMENT ON COLUMN user_affiliate_consumption_outbox.inviter_id IS '邀请人（拿返利的人）';
COMMENT ON COLUMN user_affiliate_consumption_outbox.invitee_user_id IS '被邀请人（产生消费的人）';
COMMENT ON COLUMN user_affiliate_consumption_outbox.amount IS '建议入账金额（实际入账时仍会受 per-invitee cap / duration 截断）';
COMMENT ON COLUMN user_affiliate_consumption_outbox.ref_type IS '关联类型，固定 usage_billing_dedup';
COMMENT ON COLUMN user_affiliate_consumption_outbox.ref_id IS '关联 usage_billing_dedup.id，便于回溯哪次消费';
COMMENT ON COLUMN user_affiliate_consumption_outbox.idempotency_key IS '幂等键，固定 aff_consume:{dedupID}';
