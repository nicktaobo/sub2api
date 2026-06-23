-- MERCHANT-SYSTEM v1.0
-- redeem_codes 表加 created_by_merchant_id（RFC §5.3.2）
-- admin 生成 = NULL；商户 owner 生成 = merchant.id。

ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS created_by_merchant_id BIGINT
        REFERENCES merchants(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_redeem_codes_created_by_merchant_id
    ON redeem_codes(created_by_merchant_id)
    WHERE created_by_merchant_id IS NOT NULL;

COMMENT ON COLUMN redeem_codes.created_by_merchant_id IS '商户 owner 出资生成的兑换码记录商户 id（admin 生成时为 NULL）';
