-- MERCHANT-SYSTEM v2.0
-- 商户分组定价改为 cost/sell 绝对倍率模型。
--
-- 改动：
--   1. 重命名 merchant_group_markups.markup → sell_rate（语义从「加价倍率」变为「对外绝对售价倍率」）
--   2. 旧约束 markup >= 1（加价底线）改为 sell_rate > 0（绝对值底线，运行时再校验 ≥ cost_rate）
--   3. 新建 merchant_group_costs 表（admin 配商户拿货价，绝对倍率）

-- ---- 1. merchant_group_markups: markup → sell_rate ----
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'merchant_group_markups' AND column_name = 'markup'
    ) THEN
        ALTER TABLE merchant_group_markups RENAME COLUMN markup TO sell_rate;
    END IF;
END $$;

ALTER TABLE merchant_group_markups DROP CONSTRAINT IF EXISTS chk_group_markup_range;
ALTER TABLE merchant_group_markups DROP CONSTRAINT IF EXISTS chk_group_sell_rate_positive;
ALTER TABLE merchant_group_markups ADD CONSTRAINT chk_group_sell_rate_positive CHECK (sell_rate > 0);

COMMENT ON COLUMN merchant_group_markups.sell_rate IS '商户对外绝对售价倍率，sub_user 实付 = base × sell_rate';

-- ---- 2. merchant_group_costs 新表 ----
CREATE TABLE IF NOT EXISTS merchant_group_costs (
    id           BIGSERIAL PRIMARY KEY,
    merchant_id  BIGINT       NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    group_id     BIGINT       NOT NULL,
    cost_rate    DECIMAL(6,4) NOT NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_group_cost_rate_positive CHECK (cost_rate > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_merchant_group_costs_unique
    ON merchant_group_costs(merchant_id, group_id);

COMMENT ON TABLE  merchant_group_costs IS '商户分组拿货价（admin 配置，绝对倍率）';
COMMENT ON COLUMN merchant_group_costs.cost_rate IS '商户在该分组上的拿货倍率，平台从 sub_user 余额扣 base × cost_rate';
