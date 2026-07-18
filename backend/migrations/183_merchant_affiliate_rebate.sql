-- MERCHANT-AFFILIATE v1.0（代理下级邀请返利，单层）
-- 让商户（代理）白标站下的子用户也能邀请别人，被邀请人在同一商户下消费时，
-- 邀请人从**商户利润(markup)**里拿返利（不占平台，不跨商户）。
--
-- 与平台级邀请（user_affiliates / user_affiliate_consumption_outbox）完全隔离：
--   - 商户子用户注册时不写 user_affiliates.inviter_id（平台 hook 天然不对他们生效），
--     改绑到本表 merchant_affiliate_bindings，避免"平台 + 商户"双拿返利。
--   - 返利资金来源是商户利润：消费 hook 把 merchant_earnings_outbox 的利润减去返利额，
--     返利额进本表 outbox，worker 入账到邀请人余额。守恒：商户净利 + 邀请人返利 = 原利润。

-- ============================================================================
-- 1. merchants：新增本商户的下级邀请返利开关与比例
-- ============================================================================
ALTER TABLE merchants
    ADD COLUMN IF NOT EXISTS affiliate_rebate_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS affiliate_rebate_rate_percent DECIMAL(5,2) NOT NULL DEFAULT 0;

-- 幂等：迁移可能被重复执行（ADD CONSTRAINT 无 IF NOT EXISTS），先 DROP 再 ADD，与 141/142 一致。
ALTER TABLE merchants
    DROP CONSTRAINT IF EXISTS chk_merchants_aff_rebate_rate_range;
ALTER TABLE merchants
    ADD CONSTRAINT chk_merchants_aff_rebate_rate_range
    CHECK (affiliate_rebate_rate_percent >= 0 AND affiliate_rebate_rate_percent <= 100);

COMMENT ON COLUMN merchants.affiliate_rebate_enabled IS '本商户是否开启下级邀请返利（默认关）';
COMMENT ON COLUMN merchants.affiliate_rebate_rate_percent IS '邀请人从被邀请人产生的商户利润里拿多少 %（0-100）';

-- ============================================================================
-- 2. merchant_affiliate_bindings：商户内的单层邀请绑定（invitee → inviter）
-- ============================================================================
CREATE TABLE IF NOT EXISTS merchant_affiliate_bindings (
    id               BIGSERIAL PRIMARY KEY,
    merchant_id      BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    invitee_user_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    inviter_user_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_merchant_aff_not_self CHECK (invitee_user_id <> inviter_user_id)
);

-- 一个被邀请人在一个商户下最多绑定一个邀请人（单层，不可改绑）。
CREATE UNIQUE INDEX IF NOT EXISTS uq_merchant_aff_binding_invitee
    ON merchant_affiliate_bindings (invitee_user_id);

-- 按邀请人反查其下线（后台展示 / 统计）。
CREATE INDEX IF NOT EXISTS idx_merchant_aff_binding_inviter
    ON merchant_affiliate_bindings (merchant_id, inviter_user_id);

COMMENT ON TABLE  merchant_affiliate_bindings IS '商户内单层邀请绑定；invitee 在该商户下的唯一邀请人';
COMMENT ON COLUMN merchant_affiliate_bindings.merchant_id IS '所属商户；绑定时校验 inviter/invitee 同属该商户';
COMMENT ON COLUMN merchant_affiliate_bindings.invitee_user_id IS '被邀请人（产生消费的人），全局唯一，绑定后不可改';
COMMENT ON COLUMN merchant_affiliate_bindings.inviter_user_id IS '邀请人（拿返利的人）';

-- ============================================================================
-- 3. merchant_affiliate_consumption_outbox：返利异步入账队列（仿 143 / merchant_earnings_outbox）
-- ============================================================================
CREATE TABLE IF NOT EXISTS merchant_affiliate_consumption_outbox (
    id               BIGSERIAL PRIMARY KEY,
    merchant_id      BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    inviter_user_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitee_user_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount           DECIMAL(20,8) NOT NULL CHECK (amount > 0),
    ref_type         VARCHAR(32) NOT NULL DEFAULT 'usage_billing_dedup',
    ref_id           BIGINT NOT NULL,
    idempotency_key  VARCHAR(128) NOT NULL,
    processed        BOOLEAN NOT NULL DEFAULT FALSE,
    processed_at     TIMESTAMPTZ NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_merchant_aff_outbox_idemkey
    ON merchant_affiliate_consumption_outbox (idempotency_key);

-- worker 轮询：未处理行按时间顺序拉；部分索引避免被已处理行膨胀。
CREATE INDEX IF NOT EXISTS idx_merchant_aff_outbox_pending
    ON merchant_affiliate_consumption_outbox (created_at, id)
    WHERE processed = FALSE;

COMMENT ON TABLE  merchant_affiliate_consumption_outbox IS '商户下级邀请返利消费侧异步入账队列（返利从商户利润切出）';
COMMENT ON COLUMN merchant_affiliate_consumption_outbox.amount IS '待入账返利额 = 该次消费的商户利润 × 商户返利比例';
COMMENT ON COLUMN merchant_affiliate_consumption_outbox.idempotency_key IS '幂等键，固定 merchant_aff:{dedupID}';
COMMENT ON COLUMN merchant_affiliate_consumption_outbox.ref_id IS '关联 usage_billing_dedup.id，便于回溯';
