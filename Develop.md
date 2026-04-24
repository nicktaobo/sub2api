# Sub2API 开发与部署速查（Develop.md）

本文基于当前仓库内容，对 Sub2API 的架构、运行方式、部署方式做一个偏“工程视角”的梳理，便于二次开发与运维落地。

---

## 1. 项目架构（宏观）

Sub2API = 「Go 后端单体」+「Vue 管理后台前端」+「PostgreSQL」+「Redis」。

### 1.1 组件职责

- 后端（Go / Gin）
  - 提供 Web 管理后台 API（/api/v1/...）
  - 提供网关兼容接口（/v1/...，以及部分 /openai 前缀路由），承接用户 API Key 调用并转发到上游（OpenAI/Anthropic/Gemini/Antigravity）
  - 负责鉴权、分组/账号调度、计费与用量记录、并发与限流、支付、运维监控（Ops）等
  - 可选：内嵌前端静态资源（embed 构建标签），从同一个端口直接提供管理后台页面

- 前端（Vue 3 + Vite）
  - 管理后台 Web UI（账号/分组/渠道/用量/支付/系统设置等）
  - 开发模式下通过 Vite 代理把 /api /v1 /setup 转发到后端
  - 生产构建产物输出到后端目录，供后端 embed 进二进制或由后端直接静态托管

- PostgreSQL
  - 主业务数据：用户、分组、上游账号、API Key、用量日志、支付订单等
  - 迁移脚本位于 backend/migrations/*.sql（按文件名序）

- Redis
  - 缓存与限流/队列/调度辅助数据（例如认证限流的兜底策略、调度/快照缓存等）

### 1.2 目录结构（关键路径）

- backend/
  - cmd/server：后端主入口（启动、setup wizard、自检、HTTP server）
  - internal/
    - server/：Gin router 组装、路由注册
    - handler/：HTTP handlers（用户、管理后台、网关、支付、webhook 等）
    - service/：业务服务层（调度、计费、支付、设置、Ops 等）
    - repository/：数据访问（结合 ent 与 SQL 迁移）
    - config/：配置加载（viper：config.yaml + 环境变量）
    - web/：前端 dist 的 embed 与 HTML 注入（把公开配置注入到 index.html）
  - ent/：Ent ORM 代码（schema + 生成代码）
  - migrations/：PostgreSQL SQL 迁移脚本

- frontend/
  - src/：Vue 业务代码（api 封装、页面组件、测试）
  - vite.config.ts：开发代理与构建输出路径（输出到 backend/internal/web/dist）

- deploy/
  - docker-compose*.yml：多种 compose 模式（命名卷/本地目录/开发构建）
  - docker-deploy.sh：一键准备 compose 文件与 .env（并生成密钥）
  - install.sh：一键安装二进制到 Linux（systemd 托管）
  - sub2api.service：systemd service 示例
  - Caddyfile：反向代理示例（h2c/h1 回退、缓存、压缩等）
  - config.example.yaml：源码/二进制模式的示例配置
  - DATAMANAGEMENTD_CN.md：可选 datamanagementd 联动说明

---

## 2. 请求流与模块边界（工程视角）

### 2.1 后端启动流程

入口：backend/cmd/server/main.go

启动模式分三类：

1) `-version`：打印版本信息退出  
2) `-setup`：以 CLI 方式执行 setup  
3) 默认：正常启动（若首次启动且未完成初始化，则进入 setup 流程）

首次启动判定：

- setup 通过 data dir 下的 `config.yaml` 与 `.installed` 双重判断是否需要初始化：
  - DATA_DIR 环境变量优先
  - Docker 默认使用 `/app/data`
  - 其次当前目录

Docker 场景常见路径：

- `/app/data/config.yaml`
- `/app/data/.installed`

### 2.2 HTTP 路由分层

路由组装：backend/internal/server/router.go

- 全局中间件：
  - RequestLogger / Logger
  - CORS
  - SecurityHeaders（包含 CSP nonce 注入）
- 前端静态资源：
  - embed 构建时：后端通过 middleware 提供 SPA 静态文件，并对 index.html 注入公开配置（window.__APP_CONFIG__）

路由注册分组：

- 通用：
  - GET /health
  - GET /setup/status（正常模式固定返回 needs_setup=false，供前端检测重启后状态）
- 业务 API：
  - /api/v1/auth：注册/登录/2FA/验证码等（带 Redis 兜底限流，Redis 故障时 fail-close）
  - /api/v1/user：用户资料/绑定/通知邮箱/TOTP 等
  - /api/v1/admin：管理后台（需要 admin auth）
  - /api/v1/payment：用户支付相关 + webhook + admin 支付管理
- 网关：
  - /v1/...：Claude/OpenAI/Gemini 兼容入口（API Key 鉴权 + 分组要求 + 调度转发）

### 2.3 网关（/v1）高层链路

以 /v1/messages 为例（backend/internal/server/routes/gateway.go）：

1) RequestBodyLimit（请求体大小控制）
2) ClientRequestID
3) OpsErrorLogger（记录/聚合上游错误事件，供运维页面使用）
4) InboundEndpointMiddleware：将真实 URL 归一化为统一 endpoint（/v1/messages、/v1/responses、/v1beta/models 等）
5) APIKeyAuthMiddleware：解析并校验平台下发的 API Key（通常为 sk- 前缀）
6) RequireGroupAssignment：未分组 Key 的拦截（按 Anthropic/Google 兼容格式返回错误）
7) 进入 GatewayHandler / OpenAIGatewayHandler：
   - 根据“分组平台”决定走 Anthropic 兼容还是 OpenAI 兼容路径
   - 通过调度器选择上游账号（可能包含粘性会话/并发控制/速率控制）
   - 发起上游 HTTP/WS 请求并流式或非流式回传
   - 记录用量与计费数据（写入 Postgres，同时利用 Redis 做缓存/队列/聚合）

### 2.4 数据层（Postgres + Ent + SQL migrations）

- Ent schema：backend/ent/schema/*
- 迁移：backend/migrations/*.sql（按文件名顺序执行）
- Docker 自动初始化时会自动应用 migrations，并记录到 schema_migrations（详见 deploy/README.md 的说明）

---

## 3. 如何运行（本地开发 / 源码编译 / Docker）

下面按“你在开发机上怎么跑起来”来给出可复用的流程。

### 3.1 本地开发模式（热更新：后端 + 前端）

前置：

- Go（README_CN.md 里写的是 Go 1.21+；CI/徽标中出现过 1.25.x；Dockerfile 使用 1.26.x）
- Node.js 18+
- pnpm
- PostgreSQL 15+
- Redis 7+

后端启动（默认读取 config.yaml / env）：

```bash
cd backend
go run ./cmd/server
```

前端启动（Vite dev server，会把 /api /v1 /setup 代理到后端）：

```bash
cd frontend
pnpm install
pnpm run dev
```

说明：

- 前端开发代理目标通过 `VITE_DEV_PROXY_TARGET` 控制，默认 `http://localhost:8080`
- 前端 dev 端口通过 `VITE_DEV_PORT` 控制，默认 3000
- Vite 在开发模式下会尝试请求后端 `/api/v1/settings/public`，把公开配置注入到 index.html，减少闪烁

### 3.2 源码编译（产物包含管理后台页面）

前端构建（输出到后端内嵌目录）：

```bash
cd frontend
pnpm install
pnpm run build
```

后端构建（带 embed 标签，内嵌前端 dist）：

```bash
cd backend
go build -tags embed -o sub2api ./cmd/server
```

运行：

```bash
./sub2api
```

说明：

- 不加 `-tags embed` 编译时，二进制里不会包含前端页面；此时需要你自行提供前端静态资源托管方案

### 3.3 配置加载优先级（config.yaml + 环境变量）

配置读取逻辑（backend/internal/config/config.go）：

- config.yaml 搜索路径（高到低）：
  1. DATA_DIR 环境变量指定目录
  2. /app/data（Docker）
  3. 当前目录
  4. ./config
  5. /etc/sub2api
- 同时启用环境变量覆盖：key 的 `.` 会被替换为 `_`
  - 例如 `server.port` ⇢ `SERVER_PORT`
  - 例如 `security.url_allowlist.enabled` ⇢ `SECURITY_URL_ALLOWLIST_ENABLED`

### 3.4 初始化（Setup Wizard / Auto Setup）

首次启动时若没有 config.yaml 且没有 .installed：

- 非 Docker（AUTO_SETUP=false 或未设置）：进入 Web Setup Wizard（runSetupServer），浏览器访问 `http://<host>:<port>`
- Docker（AUTO_SETUP=true）：走 AutoSetupFromEnv
  - 从环境变量读数据库与 redis 连接信息
  - 自动创建/初始化数据库
  - 应用 migrations
  - 生成/补齐 JWT_SECRET 等关键配置（若你没提供）
  - 创建管理员账号（如果你没提供 ADMIN_PASSWORD，会在日志里打印生成密码）

---

## 4. 如何部署（生产）

项目内已经给出了两条主线：systemd 二进制部署与 Docker Compose 部署。

### 4.1 方式一：Docker Compose（推荐，最省心）

仓库提供了多份 compose：

- deploy/docker-compose.local.yml：数据落本地目录（便于备份与迁移）
- deploy/docker-compose.yml：数据用命名卷（更“纯 Docker”）
- deploy/docker-compose.dev.yml：从本地源码构建镜像用于开发验证

一键准备（会生成 .env 并写入随机密钥）：

```bash
mkdir -p sub2api-deploy && cd sub2api-deploy
curl -sSL https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/deploy/docker-deploy.sh | bash
docker compose up -d
```

关键点：

- 必须设置 `AUTO_SETUP=true`（compose 已默认设置），否则容器首次启动会进入 setup wizard，与你预期的“纯自动化”不一致
- `.env` 中最关键的三项（建议固定不变）：
  - POSTGRES_PASSWORD
  - JWT_SECRET（不固定会导致重启后用户登录态失效）
  - TOTP_ENCRYPTION_KEY（不固定会导致 2FA 失效）
- 数据目录（local 版本）：
  - ./data（应用数据：config.yaml、.installed 等）
  - ./postgres_data（PGDATA）
  - ./redis_data

常用运维命令（local 版本为例）：

```bash
docker compose -f docker-compose.local.yml ps
docker compose -f docker-compose.local.yml logs -f sub2api
docker compose -f docker-compose.local.yml pull
docker compose -f docker-compose.local.yml up -d
docker compose -f docker-compose.local.yml down
```

### 4.2 方式二：二进制 + systemd（适合传统服务器）

一键安装：

```bash
curl -sSL https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/deploy/install.sh | sudo bash
```

安装脚本会：

- 下载 release 二进制到 `/opt/sub2api`
- 安装 systemd unit（示例见 deploy/sub2api.service）
- 你启动服务后，通过 Web Setup Wizard 完成数据库/redis/管理员初始化

常用命令：

```bash
sudo systemctl start sub2api
sudo systemctl enable sub2api
sudo journalctl -u sub2api -f
```

### 4.3 反向代理（Caddy / Nginx）

生产建议放到反向代理后面（统一 TLS、压缩、静态缓存、WAF/CDN、真实 IP 透传）。

- Caddy 示例：deploy/Caddyfile
  - 已包含静态资源长期缓存、压缩、反代健康检查、header 透传等
  - 同时给出了 h2c/h1 回退的 transport 思路（便于 websocket/旧客户端）

Nginx 注意事项（与 Codex CLI / 粘性会话相关）：

- 需要在 `http {}` 块开启：
  - `underscores_in_headers on;`
  - 否则类似 `session_id` 的头会被丢弃，影响多账号粘性会话等能力

---

## 5. 可选组件：datamanagementd（数据管理）

管理后台存在“数据管理”模块，但它依赖宿主机额外进程 `datamanagementd`：

- 主进程固定探测 Unix Socket：`/tmp/sub2api-datamanagement.sock`
- 仅当 Socket 可连通且 health 正常时，“数据管理”才会启用
- datamanagementd 自己用 SQLite 持久化元数据（不依赖主库）
- Docker 部署时需把宿主机 socket 挂载到容器同路径

详细步骤见：deploy/DATAMANAGEMENTD_CN.md

注意：当前仓库快照中未看到 `datamanagement/` 目录；若你需要启用该能力，请以官方发布说明/脚本为准，确认 datamanagementd 源码或二进制的来源与版本匹配方式。

---

## 6. 开发者常用入口（定位代码）

- 后端入口与启动/初始化：
  - backend/cmd/server/main.go
  - backend/internal/setup/*
- 路由注册：
  - backend/internal/server/router.go
  - backend/internal/server/routes/*
- 网关入口（/v1）：
  - backend/internal/server/routes/gateway.go
  - backend/internal/handler/gateway_handler.go（以及 OpenAI 相关 handler）
- 配置加载：
  - backend/internal/config/config.go
- 前端构建输出路径：
  - frontend/vite.config.ts（outDir 指向 backend/internal/web/dist）
- Docker 镜像构建：
  - Dockerfile（多阶段：前端构建 → 后端 embed → 最小运行时镜像）


## 源码编译
```bash
# 2. 安装 pnpm（如果还没有安装）
npm install -g pnpm

# 3. 编译前端
cd frontend
pnpm install
pnpm run build
# 构建产物输出到 ../backend/internal/web/dist/

# 4. 编译后端（嵌入前端）
cd ../backend
go build -tags embed -o sub2api ./cmd/server

# 5. 创建配置文件
cp ../deploy/config.example.yaml ./config.yaml

# 6. 编辑配置
nano config.yaml
```