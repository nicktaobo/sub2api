-- MERCHANT-SYSTEM v3.0 cleanup
-- 商户「充值折扣」机制（merchants.discount + outbox source=user_recharge_share）
-- 整个废弃。商户唯一利润来源改为消费侧的 per-group sell_rate / cost_rate 倍率差。
--
-- 含义变化：
--   - owner 自充：实付 = 充值额（不再 ×discount）
--   - sub_user 充值：owner 不再分成（不写 outbox）
--
-- 注：merchant_audit_log 中 field='discount' 的历史行保留作为审计真相，
-- 不参与本次迁移；payment_audit_logs 中 MERCHANT_RECHARGE_SHARE_* 历史行同理。

-- 1. 删除 merchants.discount 字段及范围约束
ALTER TABLE merchants DROP CONSTRAINT IF EXISTS chk_merchants_discount_range;
ALTER TABLE merchants DROP COLUMN IF EXISTS discount;

-- 2. 收紧 merchant_earnings_outbox.source 枚举：移除 user_recharge_share
--    无真实商户接入，未处理的历史行直接删除（避免 worker 进入未识别 source）
ALTER TABLE merchant_earnings_outbox DROP CONSTRAINT IF EXISTS chk_merchant_outbox_source;
DELETE FROM merchant_earnings_outbox WHERE source = 'user_recharge_share';
ALTER TABLE merchant_earnings_outbox
    ADD CONSTRAINT chk_merchant_outbox_source
    CHECK (source IN ('user_markup_share', 'self_recharge'));
