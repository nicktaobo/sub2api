-- MERCHANT-SYSTEM v1.0
-- users 表加 parent_merchant_id（RFC §4.2.1）
-- 应用层守住"一个 user 不能同时是 merchant.owner_user_id 和 users.parent_merchant_id"。

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS parent_merchant_id BIGINT
        REFERENCES merchants(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_users_parent_merchant
    ON users(parent_merchant_id)
    WHERE parent_merchant_id IS NOT NULL;

COMMENT ON COLUMN users.parent_merchant_id IS '子用户绑定的商户 id（NULL=普通用户/owner）';
