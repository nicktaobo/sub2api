-- MERCHANT-SYSTEM v2.0 cleanup
-- 商户「默认消费倍率」(user_markup_default) v2.0 后已被 per-group cost_rate / sell_rate
-- 完全取代，消费计费链路上不再读取此列。这里彻底删除以消除歧义。
--
-- 注：merchant_audit_log 中 field='user_markup_default' 的历史行保留作为审计真相，
-- 不参与本次迁移。

ALTER TABLE merchants DROP CONSTRAINT IF EXISTS chk_merchants_markup_default_range;
ALTER TABLE merchants DROP COLUMN IF EXISTS user_markup_default;
