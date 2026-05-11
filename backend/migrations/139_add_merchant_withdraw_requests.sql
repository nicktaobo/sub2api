-- MERCHANT-SYSTEM v1.0
-- 商户提现申请表
CREATE TABLE IF NOT EXISTS merchant_withdraw_requests (
    id              BIGSERIAL PRIMARY KEY,
    merchant_id     BIGINT       NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    amount          DECIMAL(20,8) NOT NULL,
    status          VARCHAR(20)  NOT NULL DEFAULT 'pending',
    payment_method  VARCHAR(20)  NOT NULL,
    payment_account VARCHAR(255) NOT NULL,
    payment_name    VARCHAR(100) NOT NULL,
    note            TEXT         NOT NULL DEFAULT '',
    admin_id        BIGINT,
    reject_reason   TEXT         NOT NULL DEFAULT '',
    ledger_id       BIGINT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    processed_at    TIMESTAMPTZ,
    CONSTRAINT chk_merchant_withdraw_status        CHECK (status IN ('pending', 'approved', 'paid', 'rejected')),
    CONSTRAINT chk_merchant_withdraw_amount        CHECK (amount > 0),
    CONSTRAINT chk_merchant_withdraw_payment_method CHECK (payment_method IN ('alipay', 'wechat', 'bank', 'usdt', 'other'))
);

CREATE INDEX IF NOT EXISTS idx_merchant_withdraw_merchant
    ON merchant_withdraw_requests(merchant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_merchant_withdraw_status
    ON merchant_withdraw_requests(status, created_at DESC);

COMMENT ON TABLE  merchant_withdraw_requests IS '商户提现申请';
COMMENT ON COLUMN merchant_withdraw_requests.status IS 'pending=待审核 / approved=已批准未打款 / paid=已打款 / rejected=已拒绝';
COMMENT ON COLUMN merchant_withdraw_requests.ledger_id IS '批准后写入 merchant_ledger 的 id';
