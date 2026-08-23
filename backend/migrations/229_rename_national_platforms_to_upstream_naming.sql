-- 国产平台标识对齐上游命名：moonshot → kimi、glm → zhipu；qwen / seedance 下线。
--
-- 背景
--   本 fork 先于上游实现了国产 OpenAI 兼容供应商，平台标识取名 moonshot/glm/qwen/seedance；
--   上游（0.1.179）后来自行实现了同类平台，标识为 kimi/zhipu/deepseek。两套命名并存导致
--   每轮 main → local_dev 合并都要在平台枚举、调度、配额、前端徽章等处反复解冲突。
--   站长 2026-08-17 拍板改用上游命名：
--     moonshot → kimi      （Kimi 月之暗面 / Moonshot）
--     glm      → zhipu     （智谱 GLM / bigmodel）
--     deepseek → deepseek  （值不变，仅 Go 标识符改为 PlatformDeepseek，DB 无需动）
--     qwen     → 下线      （上游无此平台）
--     seedance → 下线      （上游无此平台）
--   终态平台集合（= service.AllowedQuotaPlatforms，8 个）：
--     anthropic / openai / gemini / antigravity / grok / kimi / zhipu / deepseek
--
-- 为什么顺序不能颠倒（本文件最关键的一条，两个方向都会致命）
--   方向 A（第 4 步）：ADD CONSTRAINT ... CHECK 会**立即校验存量行**。若先收紧 CHECK 再改数据，
--     存量 moonshot/glm/qwen/seedance 行会让 ADD CONSTRAINT 当场失败（SQLSTATE 23514）
--     → 事务型迁移整体回滚 → ApplyMigrations 报错 → 容器起不来，且重启无限复现同一错误。
--   方向 B（第 0 步）：数据转换时旧 CHECK **仍然在位**，而它的白名单里没有 kimi / zhipu
--     （158_user_platform_quota_restore_fork_platforms.sql 留下的终态是
--      anthropic/openai/gemini/antigravity/grok/deepseek/moonshot/glm/qwen/seedance 这 10 个旧名）
--     → 第 1 步那条把 moonshot 改成 kimi 的 UPDATE 会当场撞 23514，同样整体回滚。
--   所以严格编排为：先 DROP 旧 CHECK（第 0 步）→ 转换数据（第 1~3 步）→ 最后 ADD 新 CHECK（第 4 步）。
--
--   ⚠️ 这两个坑在空库上都测不出来：全新库里 user_platform_quotas 没有旧平台行，所有 UPDATE
--      影响 0 行，001→229 一路全绿。只有带存量数据的真实库才炸。别拿「集成测试通过」当安全证据。
--      挡这两条的是静态断言 migrations/national_platform_rename_migration_test.go。
--
-- 执行顺序的第三条（跨文件）
--   上游把「收紧到 8 平台」的 CHECK 放在 224_user_platform_quotas_add_cn_providers.sql。
--   runner 按文件名字典序执行（sort.Strings，见 internal/repository/migrations_runner.go），
--   224 会排在本文件之前 → 在还带存量旧平台行的生产库上先收紧、当场 23514。
--   本轮已用 git mv 把它重编号为 230_user_platform_quotas_add_cn_providers.sql，排到本文件之后。
--   同类事故本仓库已经发生过一次：见 157 的头注释（上游 157 把白名单收回 5 个平台，
--   本 fork 只能追加 158 恢复）。守卫：migrations/platform_check_migration_ordering_test.go。
--
-- 边界：只改「平台标识」，绝不碰模型名
--   moonshot/glm/kimi/qwen 这些字符串在本项目里有两种语境：
--     (a) 平台标识：accounts.platform / groups.platform / 按平台维度的配置与观测指标；
--     (b) 模型名：moonshot-v1-8k、kimi-k2、glm-4.6、channels.model_mapping **内层**的
--         src→dst 映射、channel_model_pricing.models 数组、181_prompt_audit.sql 里的
--         scanner_backend='qwen3guard-openai' ……
--   本迁移只动 (a)，(b) 一律原样保留。特别地：
--     - channels.model_mapping 是 {平台: {源模型: 目标模型}}（086_channel_platform_pricing.sql
--       把扁平结构升级成的嵌套结构），本迁移只改**外层平台键**，内层映射整体搬迁不做任何改写；
--     - channel_model_pricing / channel_account_stats_model_pricing 只改 platform 列，
--       models 数组（模型名）不动；
--     - channel_monitor_v2_config.platforms 里每项只改 "platform" 字段，"models" 数组不动。
--
-- 与参照工程（/Project/sub2api dev 分支的 226）的三处本仓库差异，均已核过 schema：
--   1) 本仓库 channels 表**没有 platform 列**（081_create_channels.sql 建表 + 后续 ALTER 均无），
--      平台维度只落在 channel_model_pricing.platform（086 新增）。故不含 `UPDATE channels SET platform`。
--   2) channel_monitor_v2_config.platforms 过滤后为空时回落**原值**而非 '[]'（理由见 2.3）。
--   3) composite_model_routes 不动：172 建表时 target_platform CHECK 只有
--      anthropic/openai/gemini/antigravity/grok 五个，从未含国产平台，227 是纯超集、无行会违约。
--
-- 不删只停用 / 只改名不删行
--   - accounts / groups 里的 qwen/seedance 行：**停用**（status='disabled'）而非删除，理由见 1.6；
--   - ops_* 与 channel_monitor_v2_* 历史观测表：只改名不删行，否则 moonshot 的历史曲线会凭空断掉。
--
-- 幂等：所有语句都带「WHERE 旧值」或 IF EXISTS 守卫，可重复执行且第二次为空操作。
--
-- 锁与缓存
--   - 事务型迁移，runner 已在事务开头 SET LOCAL lock_timeout='10s'，本文件无需自己设。
--   - 第 0 步的 DROP CONSTRAINT 把 ACCESS EXCLUSIVE 锁从事务开头持到提交（而非只在末尾），
--     user_platform_quotas 很小，可接受。
--   - 进程内缓存（ChannelService 渠道缓存、ErrorPassthroughService 规则缓存等）无需手工失效：
--     迁移在启动时先于服务开始接流量执行，缓存此时还是冷的。
--     groups 的 platform 变更另有 DB 侧兜底：193_group_profit_control_auth_cache_invalidation.sql
--     重定义的 enqueue_group_auth_cache_invalidation() 守卫里含
--     `OLD.platform IS NOT DISTINCT FROM NEW.platform`，改名会写 auth_cache_invalidation_outbox。


-- ---------------------------------------------------------------------------
-- 0. 先卸掉旧 CHECK —— 必须排在任何数据转换之前（理由见头注释「方向 B」）
--    142 建表时 platform 列上是**内联匿名** CHECK，Postgres 自动命名为
--    user_platform_quotas_platform_check；154/155/157/158 之后都沿用该名字，
--    因此 DROP ... IF EXISTS 这一条即可覆盖所有历史形态，且在没有约束的库上是空操作。
-- ---------------------------------------------------------------------------
ALTER TABLE user_platform_quotas
    DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check;


-- ---------------------------------------------------------------------------
-- 1. 配置 / 业务表：platform 标识改名
-- ---------------------------------------------------------------------------

-- 1.1 上游账号。
UPDATE accounts SET platform = 'kimi'  WHERE platform = 'moonshot';
UPDATE accounts SET platform = 'zhipu' WHERE platform = 'glm';

-- 1.2 分组。platform 变化会触发 trg_groups_auth_cache_invalidation（见头注释「锁与缓存」），
--     受影响 api_key 的鉴权快照会被写进 auth_cache_invalidation_outbox，无需手工失效。
UPDATE groups SET platform = 'kimi'  WHERE platform = 'moonshot';
UPDATE groups SET platform = 'zhipu' WHERE platform = 'glm';

-- 1.3 渠道按平台维度的计价规则（channels 表本身无 platform 列，见头注释差异 1）。
--     只改 platform 列；models 数组是模型名，不动。
UPDATE channel_model_pricing SET platform = 'kimi'  WHERE platform = 'moonshot';
UPDATE channel_model_pricing SET platform = 'zhipu' WHERE platform = 'glm';

UPDATE channel_account_stats_model_pricing SET platform = 'kimi'  WHERE platform = 'moonshot';
UPDATE channel_account_stats_model_pricing SET platform = 'zhipu' WHERE platform = 'glm';

-- 1.4 channels.model_mapping：{平台: {源模型: 目标模型}}，只重命名**外层平台键**，
--     内层 map 整体搬迁（-> 取原值），一个字符都不改写。
--     若同时存在旧键和新键（理论上不会发生），保留已存在的新键、丢弃旧键，避免键冲突。
UPDATE channels
SET model_mapping = (
        (model_mapping - 'moonshot' - 'glm')
        || CASE WHEN model_mapping ? 'moonshot' AND NOT (model_mapping ? 'kimi')
                THEN jsonb_build_object('kimi', model_mapping -> 'moonshot')
                ELSE '{}'::jsonb END
        || CASE WHEN model_mapping ? 'glm' AND NOT (model_mapping ? 'zhipu')
                THEN jsonb_build_object('zhipu', model_mapping -> 'glm')
                ELSE '{}'::jsonb END
    )
WHERE model_mapping IS NOT NULL
  AND jsonb_typeof(model_mapping) = 'object'
  AND (model_mapping ?| ARRAY['moonshot', 'glm']);

-- 1.5 user_platform_quotas。
--     userplatformquota_user_id_platform_uq 是 `WHERE deleted_at IS NULL` 的**部分**唯一索引
--     （142_user_platform_quotas.sql），所以：
--       - 先删掉「改名后会和已有活跃行撞唯一索引」的旧行。正常库里不该有 kimi/zhipu 行，
--         这段只是保证重跑 / 混合数据态下不会因唯一冲突中止整个迁移；
--       - 改名 UPDATE **不加 deleted_at 过滤**：CHECK 对软删行同样生效，漏掉它们第 4 步就会失败；
--       - qwen/seedance 的预填行只能**硬删**：软删（置 deleted_at）不满足 CHECK——
--         约束对软删行同样生效——而 230 会无条件重建 8 平台白名单，留一行就起不来。
--         这些行是注册时 GetDefaultPlatformQuotas 按 AllowedQuotaPlatforms 自动预填的
--         **配额配置**，不是账单或用量记录（用量落在 usage_logs，且该表没有 platform 列、
--         按 account_id 关联），删除不影响任何历史计费与对账。
--         ⚠️ 未在生产库实测过存量行数：若某用户手工给 qwen/seedance 配过限额，该配置会丢失。
--         鉴于这两个平台随本轮一起下线，配置本身已无意义。
DELETE FROM user_platform_quotas q
WHERE q.platform IN ('moonshot', 'glm')
  AND q.deleted_at IS NULL
  AND EXISTS (
        SELECT 1
        FROM user_platform_quotas k
        WHERE k.user_id = q.user_id
          AND k.deleted_at IS NULL
          AND k.platform = CASE q.platform WHEN 'moonshot' THEN 'kimi' ELSE 'zhipu' END
  );

UPDATE user_platform_quotas
SET platform = CASE platform WHEN 'moonshot' THEN 'kimi' ELSE 'zhipu' END,
    updated_at = NOW()
WHERE platform IN ('moonshot', 'glm');

DELETE FROM user_platform_quotas WHERE platform IN ('qwen', 'seedance');

-- 1.6 残留的 qwen / seedance 账号与分组必须**停用**，否则升级后会把 API Key 发给错误的上游。
--
--   本轮改动从 routes.isOpenAICompatPlatform / service.openAICompatPlatforms 里摘掉了
--   qwen/seedance。此后 platform='qwen' 的分组在 /v1/messages、/chat/completions 上都不再命中
--   OpenAI 兼容分支，会 fall through 到 **Anthropic 网关**；而这条路径上没有平台白名单兜底：
--     service.matchingPlatforms('qwen')（internal/service/channel_service.go:358）原样返回
--       ['qwen'] —— 只有 composite 才展开成白名单，具体平台一律返回自身；
--     账号没配 model_mapping 就视为「支持所有模型」→ 被选中；
--     取 BaseURL 时走 Account.GetBaseURL()（internal/service/account.go:960），
--       credentials.base_url 为空时**默认回落 https://api.anthropic.com**
--       （旧代码是靠随平台一起删除的 GetQwenBaseURL 回落到 DashScope 的）。
--   净效果：阿里云百炼 / 火山方舟的 API Key 会被当作 x-api-key 发给 Anthropic —— 凭证外泄。
--
--   停用而非删除：status='disabled' 已足以阻断（可调度账号查询硬性要求 status='active'），
--   且完全可逆——站长把平台迁走后自行清理或改配到别的平台即可；
--   删除不可逆，账号/分组还挂着用量、订单与外键，不适合在迁移里替站长做这个决定。
--   只动 active 行，重跑为空操作。
UPDATE accounts
SET status = 'disabled', updated_at = NOW()
WHERE platform IN ('qwen', 'seedance')
  AND status = 'active';

UPDATE groups
SET status = 'disabled', updated_at = NOW()
WHERE platform IN ('qwen', 'seedance')
  AND status = 'active';


-- ---------------------------------------------------------------------------
-- 2. JSON 配置里的平台标识
-- ---------------------------------------------------------------------------

-- 2.1 settings 表里的默认平台配额：
--       key = 'default_platform_quotas'                       （系统层，SettingKeyDefaultPlatformQuotas）
--       key LIKE 'auth_source_default_%_platform_quotas'      （各登录来源层，见 domain_constants.go:704）
--     值是 {平台: {daily/weekly/monthly}} 的 JSON 对象（TEXT 列，存的是 JSON 文本）。
--     必须一起改，否则：
--       (a) 新注册用户会按旧 map 重新写入 moonshot/qwen 平台的配额行 → 撞第 4 步的 CHECK；
--           auth_service 那条写入是 fail-open 的（仅 warn log），结果是新用户**一条配额都没有**
--           = 全平台无限额（157/224 头注释记载的同型事故）；
--       (b) 后台保存设置时撞 validateDefaultPlatformQuotaMap 的白名单校验直接报错。
--     逐行处理并对非法 JSON 容错跳过，避免个别脏值把整个迁移（进而把启动）拖挂。
DO $$
DECLARE
    r         RECORD;
    parsed    JSONB;
    rewritten JSONB;
BEGIN
    FOR r IN
        SELECT id, key, value
        FROM settings
        WHERE key = 'default_platform_quotas'
           OR key LIKE 'auth\_source\_default\_%\_platform\_quotas'
    LOOP
        IF r.value IS NULL OR btrim(r.value) = '' THEN
            CONTINUE;
        END IF;

        BEGIN
            parsed := r.value::jsonb;
        EXCEPTION WHEN others THEN
            RAISE NOTICE '229: skip settings key % (value is not valid JSON)', r.key;
            CONTINUE;
        END;

        IF jsonb_typeof(parsed) <> 'object'
           OR NOT (parsed ?| ARRAY['moonshot', 'glm', 'qwen', 'seedance']) THEN
            CONTINUE;
        END IF;

        SELECT COALESCE(
                   jsonb_object_agg(
                       CASE e.key
                           WHEN 'moonshot' THEN 'kimi'
                           WHEN 'glm'      THEN 'zhipu'
                           ELSE e.key
                       END,
                       e.value
                   ),
                   '{}'::jsonb
               )
          INTO rewritten
          FROM jsonb_each(parsed) AS e
         WHERE e.key NOT IN ('qwen', 'seedance');

        UPDATE settings
        SET value = rewritten::text,
            updated_at = NOW()
        WHERE id = r.id;
    END LOOP;
END $$;

-- 2.2 error_passthrough_rules.platforms：平台标识字符串数组（服务端按小写比较）。
--     改名 moonshot/glm，并剔除 qwen/seedance；保持原有顺序，与模型名无关。
--
--     ⚠️ COALESCE 的兜底是 platforms（**原值**）而不是 '[]'：空数组在这张表里不是
--     「不匹配任何平台」，而是 ErrorPassthroughService.platformMatchesCached()
--     （internal/service/error_passthrough_service.go:325）里的
--         if len(rule.lowerPlatforms) == 0 { return true }
--     ——**匹配所有平台**。若某条规则的 platforms 恰好只有 qwen/seedance，剔除后留下空数组，
--     这条原本只作用于一个已下线平台的规则会静默升级成全平台生效，改变 Anthropic/OpenAI
--     等主力平台的错误透传行为（该不该把上游错误体透给客户端、该不该跳过监控/触发故障转移）。
--     jsonb_agg 在空集上返回 NULL，因此这里让 COALESCE 回落到原值：整条规则原样保留。
--     保留是安全的——不会再有任何请求带 platform='qwen'，规则在运行时恒不命中，
--     管理员也能在后台看到它引用了已下线平台并自行清理。
--     （只含 moonshot/glm 的规则不受影响：那是改名不是剔除，聚合结果非空。）
UPDATE error_passthrough_rules
SET platforms = COALESCE(
        (
            SELECT jsonb_agg(
                       CASE lower(e.value)
                           WHEN 'moonshot' THEN 'kimi'
                           WHEN 'glm'      THEN 'zhipu'
                           ELSE e.value
                       END
                       ORDER BY e.ord
                   )
            FROM jsonb_array_elements_text(platforms) WITH ORDINALITY AS e(value, ord)
            WHERE lower(e.value) NOT IN ('qwen', 'seedance')
        ),
        platforms
    ),
    updated_at = NOW()
WHERE jsonb_typeof(platforms) = 'array'
  AND EXISTS (
        SELECT 1
        FROM jsonb_array_elements_text(platforms) AS t(value)
        WHERE lower(t.value) IN ('moonshot', 'glm', 'qwen', 'seedance')
  );

-- 2.3 channel_monitor_v2_config.platforms：[{"platform":..,"enabled":..,"models":[..]}]。
--     只改每项的 "platform" 字段并剔除 qwen/seedance 项；"models" 是模型名，原样保留。
--     同时 version + 1：该列是后台保存用的乐观锁版本号，带外改动后让仍停留在旧版本的
--     管理页保存时冲突失败、强制刷新，避免把旧平台列表覆盖回来。
--
--     兜底同样回落**原值**而非 '[]'（与参照工程不同，是本仓库刻意的保守选择）：
--     这张表是单行配置（id=1），platforms 为空 = 监控一个平台都不采。若某个部署把 platforms
--     裁剪到只剩 qwen/seedance，剔除后写 '[]' 会静默把渠道监控整个关掉、图表全空且无人察觉。
--     保留原值最坏只是留下两个永远采不到数据的平台项——normalizeChannelMonitorV2Config
--     （internal/service/channel_monitor_v2.go:744）只做小写/去重/排序，不校验平台白名单，
--     所以保留 qwen/seedance 项不会让后台保存报错。
UPDATE channel_monitor_v2_config c
SET platforms = COALESCE(
        (
            SELECT jsonb_agg(
                       CASE e.value ->> 'platform'
                           WHEN 'moonshot' THEN jsonb_set(e.value, '{platform}', '"kimi"'::jsonb)
                           WHEN 'glm'      THEN jsonb_set(e.value, '{platform}', '"zhipu"'::jsonb)
                           ELSE e.value
                       END
                       ORDER BY e.ord
                   )
            FROM jsonb_array_elements(c.platforms) WITH ORDINALITY AS e(value, ord)
            WHERE COALESCE(e.value ->> 'platform', '') NOT IN ('qwen', 'seedance')
        ),
        c.platforms
    ),
    version = c.version + 1,
    updated_at = NOW()
WHERE jsonb_typeof(c.platforms) = 'array'
  AND EXISTS (
        SELECT 1
        FROM jsonb_array_elements(c.platforms) AS t(value)
        WHERE t.value ->> 'platform' IN ('moonshot', 'glm', 'qwen', 'seedance')
  );

-- 2.4 ops_alert_rules.filters->>'platform'：告警规则的平台作用域
--     （internal/service/ops_alert_evaluator_service.go:391 parseOpsAlertRuleScope 读这个键）。
--     只改名；指向 qwen/seedance 的规则原样保留——已下线平台不会再产生指标，规则恒不触发。
UPDATE ops_alert_rules
SET filters = jsonb_set(
        filters,
        '{platform}',
        -- ::text 显式定型：CASE 的两个分支都是 unknown 字面量，虽然按 PG 的 CASE 类型
        -- 解析规则会落到 text，但 to_jsonb 是多态函数，显式加 cast 才不依赖那条推导。
        to_jsonb((CASE filters ->> 'platform' WHEN 'moonshot' THEN 'kimi' ELSE 'zhipu' END)::text)
    ),
    updated_at = NOW()
WHERE filters IS NOT NULL
  AND jsonb_typeof(filters) = 'object'
  AND filters ->> 'platform' IN ('moonshot', 'glm');


-- ---------------------------------------------------------------------------
-- 3. 观测 / 历史数据表：只改名，不删行
--     ops_* 与 channel_monitor_v2_* 存的是历史观测数据。改名是为了让旧数据在新命名下
--     仍能按平台聚合出来（否则 moonshot 的历史曲线会凭空断掉）；qwen/seedance 的历史点
--     即便存在也保留原值，删掉只会让历史不可读，且这些表都没有平台白名单约束。
--
--     usage_logs 本身没有 platform 列，聚合时靠
--       COALESCE(NULLIF(groups.platform, ''), accounts.platform, '')
--     现取（见 026_ops_metrics_aggregation_tables.sql 的注释）。第 1.1/1.2 步改完
--     accounts/groups 之后，历史 usage_logs 会自动落到新平台名下，与这里改名后的
--     预聚合表口径一致——两侧必须一起改，只改一侧会让同一段历史在两张图上对不上。
--
--     这些表大多带含 platform 的主键 / 唯一索引（例如
--     channel_monitor_v2_metrics_1m 的 PRIMARY KEY (bucket_start, platform, group_id, model)、
--     ops_metrics_hourly 的 idx_ops_metrics_hourly_unique_dim），
--     理论上不会与新命名撞键（新命名的行只可能在本迁移之后产生）；万一撞上
--     （例如 schema_migrations 被清空后在混合数据上重跑），捕获 unique_violation
--     跳过该表而不是让启动失败——历史观测数据不值得赌一次起不来的部署。
--
--     to_regclass / information_schema 双重存在性判断：历史部署可能裁剪过监控模块。
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    t    TEXT;
    tbls TEXT[] := ARRAY[
        'ops_metrics_hourly',
        'ops_metrics_daily',
        'ops_error_logs',
        'ops_system_metrics',
        'ops_system_logs',
        'ops_alert_silences',
        'channel_monitor_v2_metrics_1m',
        'channel_monitor_v2_user_metrics_1m',
        'channel_monitor_v2_error_metrics_1m',
        'channel_monitor_v2_latency_histograms_1m',
        'channel_monitor_v2_metrics_rollup',
        'channel_monitor_v2_user_metrics_rollup',
        'channel_monitor_v2_error_metrics_rollup',
        'channel_monitor_v2_latency_histograms_rollup'
    ];
BEGIN
    FOREACH t IN ARRAY tbls LOOP
        IF to_regclass(t) IS NULL THEN
            CONTINUE;
        END IF;
        IF NOT EXISTS (
            SELECT 1
            FROM information_schema.columns
            WHERE table_schema = current_schema()
              AND table_name = t
              AND column_name = 'platform'
        ) THEN
            CONTINUE;
        END IF;

        BEGIN
            EXECUTE format(
                'UPDATE %I SET platform = CASE platform WHEN %L THEN %L ELSE %L END '
                || 'WHERE platform IN (%L, %L)',
                t, 'moonshot', 'kimi', 'zhipu', 'moonshot', 'glm'
            );
        EXCEPTION WHEN unique_violation THEN
            RAISE NOTICE '229: skip platform rename on % (unique_violation: new-name rows already exist)', t;
        END;
    END LOOP;
END $$;


-- ---------------------------------------------------------------------------
-- 4. 收紧 user_platform_quotas.platform 的 CHECK（必须在数据转换之后）
--     终态 = 上游 5 平台 + 保留的 3 个国产平台 = 8 个：
--       anthropic / openai / gemini / antigravity / grok / kimi / zhipu / deepseek
--     历史沿革：142 建表 4 平台 → 154 放宽到 9 → 155 补 grok（10）→ 157（上游）误收回 5
--       → 158 恢复 10（含 moonshot/glm/qwen/seedance）→ 本迁移收敛为 8（新命名）。
--     已发布迁移受 checksum 保护、不能原地改，只能像这样用新迁移覆盖旧 CHECK。
--     这份名单必须与 internal/service/domain_constants.go 的 AllowedQuotaPlatforms
--     和 ent/schema/user_platform_quota.go 的 Validate 三处逐字一致；
--     漂移守卫见 internal/service/quota_platform_check_parity_test.go。
-- ---------------------------------------------------------------------------
-- 预检：ADD CONSTRAINT 会立即校验存量行，失败会让整个迁移回滚、应用起不来。
-- 这里先把「不在新白名单内的残留值」变成一条能直接看懂的报错，而不是裸的约束冲突。
-- 正常路径下第 1.5 步已经把 moonshot/glm 改名、qwen/seedance 删掉，这里应当为空。
DO $$
DECLARE
    leftover TEXT;
BEGIN
    SELECT string_agg(DISTINCT platform, ', ')
      INTO leftover
      FROM user_platform_quotas
     WHERE platform NOT IN (
               'anthropic', 'openai', 'gemini', 'antigravity', 'grok',
               'kimi', 'zhipu', 'deepseek'
           );
    IF leftover IS NOT NULL THEN
        RAISE EXCEPTION
            '229: user_platform_quotas 仍有不在新白名单内的 platform 值 (%)，请人工处理后再重跑迁移',
            leftover;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'user_platform_quotas_platform_check'
          AND conrelid = 'user_platform_quotas'::regclass
    ) THEN
        ALTER TABLE user_platform_quotas
            ADD CONSTRAINT user_platform_quotas_platform_check
            CHECK (platform IN (
                'anthropic', 'openai', 'gemini', 'antigravity', 'grok',
                'kimi', 'zhipu', 'deepseek'
            ));
    END IF;
END $$;
