-- MERCHANT-SYSTEM v1.0
-- 商户系统数据底座（RFC §4.1）
-- 6 张新表：merchants / merchant_domains / merchant_ledger / merchant_earnings_outbox /
--          merchant_audit_log / merchant_group_markups

-- ============================================================================
-- 1. merchants：商户主表
-- ============================================================================
CREATE TABLE IF NOT EXISTS merchants (
    id                       BIGSERIAL PRIMARY KEY,
    owner_user_id            BIGINT       NOT NULL UNIQUE REFERENCES users(id) ON DELETE RESTRICT,
    name                     VARCHAR(100) NOT NULL,
    status                   VARCHAR(20)  NOT NULL DEFAULT 'active',
    discount                 DECIMAL(6,4) NOT NULL DEFAULT 1.0000,
    user_markup_default      DECIMAL(6,4) NOT NULL DEFAULT 1.0000,
    owner_balance_baseline   DECIMAL(20,8) NOT NULL DEFAULT 0,
    low_balance_threshold    DECIMAL(20,8) NOT NULL DEFAULT 0,
    notify_emails            JSONB        NOT NULL DEFAULT '[]'::jsonb,
    created_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at               TIMESTAMPTZ,
    CONSTRAINT chk_merchants_status        CHECK (status IN ('active', 'suspended')),
    CONSTRAINT chk_merchants_discount_range CHECK (discount > 0 AND discount <= 1),
    CONSTRAINT chk_merchants_markup_default_range CHECK (user_markup_default >= 1)
);

CREATE INDEX IF NOT EXISTS idx_merchants_status ON merchants(status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_merchants_deleted_at ON merchants(deleted_at);

COMMENT ON TABLE  merchants IS '商户主表（RFC §4.1.1）';
COMMENT ON COLUMN merchants.owner_user_id IS '商户拥有者 user.id；UNIQUE 保证一个 user 最多是一个商户的 owner';
COMMENT ON COLUMN merchants.discount IS '充值环节比例 (0,1]：owner 实付 amount × discount；sub_user 充值 owner 得 (1-discount)×amount';
COMMENT ON COLUMN merchants.user_markup_default IS '消费倍率商户级兜底 ≥1，分组未配置 markup 时使用';
COMMENT ON COLUMN merchants.owner_balance_baseline IS '开通商户时 owner.balance 快照，对账等式基线';

-- ============================================================================
-- 2. merchant_domains：商户域名 + 站点品牌
-- ============================================================================
CREATE TABLE IF NOT EXISTS merchant_domains (
    id                BIGSERIAL PRIMARY KEY,
    merchant_id       BIGINT       NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    domain            VARCHAR(255) NOT NULL,
    site_name         VARCHAR(100) NOT NULL DEFAULT '',
    site_logo         TEXT         NOT NULL DEFAULT '',
    brand_color       VARCHAR(20)  NOT NULL DEFAULT '',
    custom_css        TEXT         NOT NULL DEFAULT '',
    home_content      TEXT         NOT NULL DEFAULT '',
    seo_title         VARCHAR(255) NOT NULL DEFAULT '',
    seo_description   TEXT         NOT NULL DEFAULT '',
    seo_keywords      VARCHAR(500) NOT NULL DEFAULT '',
    verify_token      VARCHAR(64)  NOT NULL DEFAULT '',
    verified          BOOLEAN      NOT NULL DEFAULT FALSE,
    verified_at       TIMESTAMPTZ,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ
);

-- partial unique：软删除后域名可重新启用（RFC §4.1.2）
CREATE UNIQUE INDEX IF NOT EXISTS idx_merchant_domains_domain_active
    ON merchant_domains(domain) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_merchant_domains_merchant
    ON merchant_domains(merchant_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_merchant_domains_verified
    ON merchant_domains(verified, domain) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_merchant_domains_deleted_at ON merchant_domains(deleted_at);

COMMENT ON TABLE merchant_domains IS '商户自定义域名 + 站点品牌（RFC §4.1.2）';

-- ============================================================================
-- 3. merchant_ledger：商户 owner 钱包资金流水（永久保留）
-- ============================================================================
CREATE TABLE IF NOT EXISTS merchant_ledger (
    id                    BIGSERIAL PRIMARY KEY,
    merchant_id           BIGINT       NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    owner_user_id         BIGINT       NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    counterparty_user_id  BIGINT       REFERENCES users(id) ON DELETE SET NULL,
    direction             VARCHAR(10)  NOT NULL,
    amount                DECIMAL(20,8) NOT NULL,
    balance_after         DECIMAL(20,8),
    is_aggregated         BOOLEAN      NOT NULL DEFAULT FALSE,
    aggregated_count      INTEGER,
    source                VARCHAR(40)  NOT NULL,
    ref_type              VARCHAR(40),
    ref_id                BIGINT,
    idempotency_key       VARCHAR(120) UNIQUE,
    note                  TEXT,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_merchant_ledger_direction CHECK (direction IN ('credit', 'debit')),
    CONSTRAINT chk_merchant_ledger_amount    CHECK (amount > 0)
);

CREATE INDEX IF NOT EXISTS idx_merchant_ledger_merchant
    ON merchant_ledger(merchant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_merchant_ledger_owner
    ON merchant_ledger(owner_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_merchant_ledger_counterparty
    ON merchant_ledger(counterparty_user_id, created_at DESC)
    WHERE counterparty_user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_merchant_ledger_source_ref
    ON merchant_ledger(source, ref_type, ref_id);

COMMENT ON TABLE  merchant_ledger IS '商户 owner 钱包流水（owner-only，sub_user 余额变动不在此记账）（RFC §4.2.2）';
COMMENT ON COLUMN merchant_ledger.owner_user_id IS '余额变动主体（恒为 merchant.owner_user_id）';
COMMENT ON COLUMN merchant_ledger.counterparty_user_id IS '交易对手（如 sub_user.id 或 NULL）';
COMMENT ON COLUMN merchant_ledger.balance_after IS '同步路径行有值；聚合路径行 NULL';

-- ============================================================================
-- 4. merchant_earnings_outbox：网关→worker 临时队列（30 天保留）
-- ============================================================================
CREATE TABLE IF NOT EXISTS merchant_earnings_outbox (
    id                   BIGSERIAL PRIMARY KEY,
    merchant_id          BIGINT       NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    counterparty_user_id BIGINT       REFERENCES users(id) ON DELETE SET NULL,
    amount               DECIMAL(20,8) NOT NULL,
    source               VARCHAR(40)  NOT NULL,
    ref_type             VARCHAR(40)  NOT NULL,
    ref_id               BIGINT       NOT NULL,
    idempotency_key      VARCHAR(100) NOT NULL UNIQUE,
    processed            BOOLEAN      NOT NULL DEFAULT FALSE,
    processed_at         TIMESTAMPTZ,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_merchant_outbox_amount CHECK (amount > 0),
    CONSTRAINT chk_merchant_outbox_source CHECK (source IN ('user_markup_share', 'user_recharge_share', 'self_recharge'))
);

CREATE INDEX IF NOT EXISTS idx_outbox_pending
    ON merchant_earnings_outbox(processed, created_at)
    WHERE processed = FALSE;
CREATE INDEX IF NOT EXISTS idx_outbox_merchant
    ON merchant_earnings_outbox(merchant_id, created_at DESC);

COMMENT ON TABLE merchant_earnings_outbox IS '商户分润短期缓冲队列（非流水，processed=true 后保留 30 天）（RFC §4.1.3）';

-- ============================================================================
-- 5. merchant_audit_log：商户配置/操作审计（永久保留）
-- ============================================================================
CREATE TABLE IF NOT EXISTS merchant_audit_log (
    id                BIGSERIAL PRIMARY KEY,
    merchant_id       BIGINT       NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    admin_id          BIGINT,
    field             VARCHAR(50)  NOT NULL,
    old_value         TEXT,
    new_value         TEXT,
    reason            TEXT         NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_merchant ON merchant_audit_log(merchant_id, created_at DESC);

COMMENT ON TABLE merchant_audit_log IS '商户配置/操作审计日志（永久保留）（RFC §4.1.4）';

-- ============================================================================
-- 6. merchant_group_markups：分组级 markup 覆盖
-- ============================================================================
CREATE TABLE IF NOT EXISTS merchant_group_markups (
    id            BIGSERIAL PRIMARY KEY,
    merchant_id   BIGINT       NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    group_id      BIGINT       NOT NULL REFERENCES groups(id)    ON DELETE CASCADE,
    markup        DECIMAL(6,4) NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_group_markup_range CHECK (markup >= 1)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_merchant_group_markups_unique
    ON merchant_group_markups(merchant_id, group_id);

COMMENT ON TABLE merchant_group_markups IS '商户分组级 markup 覆盖（RFC §4.1.5）';
