-- 142_group_models.sql
-- 「模型列表」展示页用的 admin 配置：每个 group 一份"对外宣传用"模型清单。
-- 跟计费完全无关——计费仍走 channels.supported_models / LiteLLM 路径。

CREATE TABLE IF NOT EXISTS group_models (
    id          BIGSERIAL PRIMARY KEY,
    group_id    BIGINT       NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    model       VARCHAR(200) NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_group_models_model_not_empty CHECK (length(trim(model)) > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_group_models_unique
    ON group_models(group_id, model);

CREATE INDEX IF NOT EXISTS idx_group_models_group
    ON group_models(group_id);

COMMENT ON TABLE  group_models IS '分组"展示用"模型列表（admin 配置，不参与计费）';
COMMENT ON COLUMN group_models.model IS '模型名（用户侧可见名）';
