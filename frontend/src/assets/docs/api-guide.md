# ModelBoxs 用户使用与 API 接入文档

文档版本：2026-05-18  
官网地址：https://modelboxs.com  
平台定位：AI API Gateway / Sub2API 中转与模型路由平台

ModelBoxs 为开发者和团队提供统一的 AI API 接入入口。你可以通过一个平台 Key 调用 Claude、GPT、Gemini、GPT Image、国产大模型等多家模型能力，并尽量保持与 OpenAI、Anthropic、Gemini 等主流 API 格式兼容。

> 重要聲明：本站僅針對非中華人民共和國行政管轄區域範圍內使用者提供 AI Token 算力資源服務；平台不主動取得、儲存或記錄使用者的具體使用內容、Prompt 及業務資料。

本文档分为两部分：

- 用户使用文档：注册、获取 API Key、查看模型、充值、使用记录与安全建议。
- 开发者接入文档：OpenAI 兼容格式、Claude/Anthropic 原生格式、Gemini 原生格式、图片生成、常见客户端与错误处理。

> 说明：本文中的模型名称用于展示接入方式。实际可用模型、上下文长度、价格、倍率、并发限制和是否支持流式输出，请以 ModelBoxs 控制台、模型列表接口和平台公告为准。

---

## 1. 快速开始

### 1.1 接入信息

| 项目 | 值 |
| --- | --- |
| 官网 | `https://modelboxs.com` |
| OpenAI 兼容 Base URL | `https://modelboxs.com/v1` |
| Anthropic/Claude Base URL | `https://modelboxs.com/v1` |
| Gemini 原生接口前缀 | `https://modelboxs.com/v1beta` |
| 认证方式 | `Authorization: Bearer YOUR_API_KEY` |
| Claude 认证方式 | `x-api-key: YOUR_API_KEY` 或 `Authorization: Bearer YOUR_API_KEY` |

### 1.2 三步完成第一次调用

1. 访问 `https://modelboxs.com` 注册或登录账号。
2. 进入控制台创建 API Key，并妥善保存。
3. 将你原来代码中的官方 `base_url` 替换为 `https://modelboxs.com/v1`，把 `api_key` 替换为 ModelBoxs 颁发的 Key。

最小 cURL 示例：

```bash
export MODELBOXS_API_KEY="sk-your-api-key"

curl https://modelboxs.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MODELBOXS_API_KEY" \
  -d '{
    "model": "gpt-5.5",
    "messages": [
      {"role": "user", "content": "你好，请用一句话介绍 ModelBoxs。"}
    ],
    "stream": true
  }'
```

---

## 2. 用户使用文档

### 2.1 注册与登录

打开 `https://modelboxs.com`，根据页面提示完成注册、登录和邮箱验证。登录后可进入控制台管理账户余额、API Key、调用记录、模型权限和充值信息。

### 2.2 创建 API Key

进入控制台后，找到「令牌」「API Key」或类似入口，创建新的 API Key。

建议配置：

| 配置项 | 建议 |
| --- | --- |
| Key 名称 | 使用业务名或项目名，例如 `prod-chatbot`、`dev-test` |
| 额度限制 | 生产环境建议设置单 Key 额度，避免泄露后造成过大损失 |
| 模型权限 | 只开放当前项目需要的模型 |
| IP 限制 | 如果控制台支持，生产服务建议绑定服务器出口 IP |
| 过期时间 | 临时测试 Key 建议设置过期时间 |

安全提醒：

- 不要把 API Key 写入前端代码、App 客户端或公开仓库。
- 生产环境请放在服务端环境变量、密钥管理服务或 CI/CD Secret 中。
- 怀疑泄露时请立即禁用旧 Key 并创建新 Key。

### 2.3 查询可用模型

你可以在控制台查看当前账号可用模型，也可以通过接口查询：

```bash
curl https://modelboxs.com/v1/models \
  -H "Authorization: Bearer $MODELBOXS_API_KEY"
```

平台接入模型包括但不限于：

| 模型系列 | 示例 |
| --- | --- |
| Anthropic Claude | Claude Opus 4.7、Claude Sonnet/Haiku 等向下兼容型号 |
| OpenAI GPT | GPT 5.5、GPT 5.x、GPT 4.x、轻量与推理模型 |
| OpenAI Image | GPT image-2 及兼容图片生成模型 |
| Google Gemini | Gemini 3.1 Pro、Gemini 3.x/2.x 系列 |
| 国产模型 | DeepSeek、Qwen、Kimi、GLM、Hunyuan、Doubao 等 |

调用时 `model` 参数必须填写控制台展示的准确模型 ID。展示名和 API ID 可能不同。

### 2.4 接入前检查

当前平台不提供操练台或 Playground。首次接入时，建议通过本文档中的 cURL 示例进行最小化测试，并优先检查：

- API Key 是否已经创建并复制完整。
- 模型 ID 是否与控制台展示完全一致。
- 当前余额或套餐是否可用。
- Base URL 是否填写为 `https://modelboxs.com/v1`，或按客户端要求填写为 `https://modelboxs.com`。

### 2.5 计费与用量

ModelBoxs 通常按模型、输入 Token、输出 Token、图片张数或任务类型计费。不同模型价格不同，具体以控制台价格页为准。

建议使用方式：

- 测试阶段先使用低成本模型或较低图片质量。
- 长文本任务设置合理的 `max_tokens`，避免无意消耗。
- 生产服务建议记录每次调用的模型、输入长度、输出长度、请求 ID 和业务用户 ID。
- 定期查看「使用记录」「消费明细」或「日志」页面。

---

## 3. 开发者 API 接入

### 3.1 认证方式

OpenAI 兼容接口：

```http
Authorization: Bearer YOUR_API_KEY
Content-Type: application/json
```

Anthropic/Claude 原生接口：

```http
x-api-key: YOUR_API_KEY
anthropic-version: 2023-06-01
Content-Type: application/json
```

Gemini 原生接口常用 Query Key：

```text
https://modelboxs.com/v1beta/models/{model}:generateContent?key=YOUR_API_KEY
```

### 3.2 常用接口

| 类型 | 方法与路径 | 说明 |
| --- | --- | --- |
| 模型列表 | `GET /v1/models` | 查询当前 Key 可用模型 |
| 聊天补全 | `POST /v1/chat/completions` | OpenAI Chat Completions 格式 |
| Responses API | `POST /v1/responses` | OpenAI Responses 格式，适合新项目逐步迁移 |
| Claude Messages | `POST /v1/messages` | Anthropic Messages 原生格式 |
| Gemini Generate Content | `POST /v1beta/models/{model}:generateContent` | Gemini 原生格式 |
| 图片生成 | `POST /v1/images/generations` | GPT image-2 文生图 |
| 图片编辑 | `POST /v1/images/edits` | 图片编辑，视模型权限开放 |

说明：Embeddings、Rerank、音频等能力如后续开放，请以控制台、平台公告或对应专项文档为准，不建议用户在未确认开放前直接接入。

---

## 4. OpenAI 兼容格式

适用场景：

- 调用 GPT 5.5、GPT 5.x、GPT 4.x 等 OpenAI 系列模型。
- 调用支持 OpenAI Chat Completions 格式的 Claude、Gemini、国产模型。
- 接入 Cherry Studio、Chatbox、LobeChat、Open WebUI、Continue、Cline 等 OpenAI 兼容客户端。

### 4.1 Python SDK

安装：

```bash
pip install openai
```

代码示例：

```python
import os
from openai import OpenAI

client = OpenAI(
    api_key=os.environ["MODELBOXS_API_KEY"],
    base_url="https://modelboxs.com/v1",
    timeout=60.0,
)

stream = client.chat.completions.create(
    model="gpt-5.5",
    messages=[
        {"role": "system", "content": "你是一个专业、可靠的 AI 助手。"},
        {"role": "user", "content": "请用 Python 写一个快速排序。"},
    ],
    stream=True,
)

for chunk in stream:
    delta = chunk.choices[0].delta
    if delta.content:
        print(delta.content, end="", flush=True)
```

说明：

- 对于 GPT 5.5 等较新的推理或长输出模型，建议优先使用 `stream: true`。
- 如果某个模型在控制台标注「必须流式」，非流式调用可能返回错误。
- 非 GPT 模型通常同时支持流式和非流式，具体以模型说明为准。

### 4.2 Node.js SDK

安装：

```bash
npm install openai
```

代码示例：

```javascript
import OpenAI from "openai";

const client = new OpenAI({
  apiKey: process.env.MODELBOXS_API_KEY,
  baseURL: "https://modelboxs.com/v1",
  timeout: 60_000,
});

const stream = await client.chat.completions.create({
  model: "gpt-5.5",
  messages: [
    { role: "system", content: "你是一个专业、可靠的 AI 助手。" },
    { role: "user", content: "请解释一下什么是 RAG。" },
  ],
  stream: true,
});

for await (const chunk of stream) {
  const text = chunk.choices[0]?.delta?.content;
  if (text) process.stdout.write(text);
}
```

### 4.3 cURL

```bash
curl https://modelboxs.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MODELBOXS_API_KEY" \
  -d '{
    "model": "gpt-5.5",
    "messages": [
      {"role": "user", "content": "给我一个 SaaS 产品定价页的文案框架。"}
    ],
    "temperature": 0.7,
    "max_tokens": 2000,
    "stream": true
  }'
```

### 4.4 调用国产模型

国产模型通常也可以通过 OpenAI 兼容格式调用，只需要把 `model` 替换为控制台展示的国产模型 ID。

```bash
curl https://modelboxs.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MODELBOXS_API_KEY" \
  -d '{
    "model": "请替换为控制台展示的国产模型ID",
    "messages": [
      {"role": "user", "content": "请总结这段中文合同的主要风险点。"}
    ],
    "stream": false
  }'
```

---

## 5. OpenAI Responses API

新项目如果已经使用 OpenAI Responses API，可以尝试使用 `/v1/responses`。

```bash
curl https://modelboxs.com/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MODELBOXS_API_KEY" \
  -d '{
    "model": "gpt-5.5",
    "input": "请给我一份 API 网关监控指标清单。"
  }'
```

Python 示例：

```python
import os
from openai import OpenAI

client = OpenAI(
    api_key=os.environ["MODELBOXS_API_KEY"],
    base_url="https://modelboxs.com/v1",
)

response = client.responses.create(
    model="gpt-5.5",
    input="请给我一份 API 网关监控指标清单。",
)

print(response.output_text)
```

如果你的 SDK 或业务框架暂不支持 Responses API，优先使用兼容性更广的 `/v1/chat/completions`。

---

## 6. Claude / Anthropic 原生格式

适用场景：

- 需要最大程度兼容 Anthropic 官方 SDK。
- 调用 Claude Opus 4.7、Claude Sonnet、Claude Haiku 等模型。
- 接入 Claude Code、Claude SDK 或使用 Anthropic Messages 格式的工具。

### 6.1 cURL

```bash
curl https://modelboxs.com/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: $MODELBOXS_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-opus-4.7",
    "max_tokens": 2048,
    "system": "你是一个资深软件架构师。",
    "messages": [
      {"role": "user", "content": "请为一个多模型 API 网关设计监控告警方案。"}
    ]
  }'
```

### 6.2 Python SDK

安装：

```bash
pip install anthropic
```

代码示例：

```python
import os
from anthropic import Anthropic

client = Anthropic(
    api_key=os.environ["MODELBOXS_API_KEY"],
    base_url="https://modelboxs.com/v1",
)

message = client.messages.create(
    model="claude-opus-4.7",
    max_tokens=2048,
    system="你是一个资深软件架构师。",
    messages=[
        {"role": "user", "content": "请解释 API Gateway 和模型路由的区别。"}
    ],
)

print(message.content[0].text)
```

### 6.3 Node.js SDK

安装：

```bash
npm install @anthropic-ai/sdk
```

代码示例：

```javascript
import Anthropic from "@anthropic-ai/sdk";

const client = new Anthropic({
  apiKey: process.env.MODELBOXS_API_KEY,
  baseURL: "https://modelboxs.com/v1",
});

const message = await client.messages.create({
  model: "claude-opus-4.7",
  max_tokens: 2048,
  system: "你是一个资深软件架构师。",
  messages: [
    { role: "user", content: "请给出一个高可用 API 中转站的架构清单。" },
  ],
});

console.log(message.content[0].text);
```

### 6.4 Claude Messages 注意事项

- `system` 是独立字段，不要放进 `messages` 数组。
- `messages` 中通常只使用 `user` 和 `assistant` 两种角色。
- 多轮对话建议保持 `user` 与 `assistant` 交替。
- 图片输入、多模态能力是否可用取决于模型和账号权限。

---

## 7. Gemini 原生格式

适用场景：

- 调用 Gemini 3.1 Pro、Gemini 3.x/2.x 系列模型。
- 已有项目使用 Google Gemini `generateContent` 格式。

### 7.1 cURL

```bash
curl "https://modelboxs.com/v1beta/models/gemini-3.1-pro:generateContent?key=$MODELBOXS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [
      {
        "role": "user",
        "parts": [
          {"text": "请用表格比较 GPT、Claude 和 Gemini 的适用场景。"}
        ]
      }
    ],
    "generationConfig": {
      "temperature": 0.7,
      "maxOutputTokens": 2048
    }
  }'
```

### 7.2 OpenAI 兼容方式调用 Gemini

如果你的业务统一使用 OpenAI 格式，也可以使用 `/v1/chat/completions` 调用 Gemini 系列模型：

```bash
curl https://modelboxs.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MODELBOXS_API_KEY" \
  -d '{
    "model": "gemini-3.1-pro",
    "messages": [
      {"role": "user", "content": "请生成一份产品需求文档大纲。"}
    ],
    "stream": false
  }'
```

---

## 8. GPT image-2 图片生成

适用场景：

- 文生图、海报、产品概念图、Logo 草图、视觉方案。
- 已兼容 OpenAI Images API 的项目。

### 8.1 cURL

```bash
curl https://modelboxs.com/v1/images/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MODELBOXS_API_KEY" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "为 ModelBoxs AI API Gateway 生成一张科技感官网主视觉，干净、专业、适合 B2B SaaS。",
    "size": "1536x1024",
    "quality": "medium",
    "n": 1
  }'
```

### 8.2 常用参数

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `model` | string | 是 | 图片模型 ID，例如 `gpt-image-2` |
| `prompt` | string | 是 | 图片生成提示词 |
| `size` | string | 否 | 常见值：`1024x1024`、`1024x1536`、`1536x1024`、`2048x2048`、`3840x2160`、`auto` |
| `quality` | string | 否 | 常见值：`low`、`medium`、`high` |
| `n` | integer | 否 | 生成张数，通常从 `1` 开始 |

说明：

- 图片生成不是聊天接口，不要调用 `/v1/chat/completions`。
- 高分辨率和高质量模式耗时更长，客户端超时建议设置为 300 秒。
- 具体尺寸、质量和价格以控制台为准。

### 8.3 Python 保存图片

```python
import base64
import os
from openai import OpenAI

client = OpenAI(
    api_key=os.environ["MODELBOXS_API_KEY"],
    base_url="https://modelboxs.com/v1",
    timeout=300.0,
)

result = client.images.generate(
    model="gpt-image-2",
    prompt="一张用于 AI API Gateway 官网的专业产品主视觉",
    size="1536x1024",
    quality="medium",
    n=1,
)

image = result.data[0]

if getattr(image, "b64_json", None):
    with open("modelboxs-image.png", "wb") as f:
        f.write(base64.b64decode(image.b64_json))
else:
    print(image.url)
```

---

## 9. 第三方客户端配置

大多数支持 OpenAI Compatible Provider 的客户端都可以接入 ModelBoxs。

| 客户端/工具 | Provider 类型 | Base URL | API Key | Model |
| --- | --- | --- | --- | --- |
| Cherry Studio | OpenAI Compatible | `https://modelboxs.com/v1` | ModelBoxs Key | 控制台模型 ID |
| Chatbox | OpenAI API Compatible | `https://modelboxs.com/v1` | ModelBoxs Key | 控制台模型 ID |
| LobeChat | OpenAI Compatible | `https://modelboxs.com/v1` | ModelBoxs Key | 控制台模型 ID |
| Open WebUI | OpenAI Compatible | `https://modelboxs.com/v1` | ModelBoxs Key | 控制台模型 ID |
| Continue / Cline | OpenAI Compatible 或 Anthropic | `https://modelboxs.com/v1` | ModelBoxs Key | 控制台模型 ID |
| Claude Code | Anthropic Compatible | `https://modelboxs.com/v1` | ModelBoxs Key | Claude 模型 ID |

通用配置原则：

- Provider 选择 OpenAI Compatible 时，Base URL 填 `https://modelboxs.com/v1`。
- Provider 选择 Anthropic Compatible 时，Base URL 仍填 `https://modelboxs.com/v1`。
- Model 字段必须手动填写控制台显示的模型 ID。
- 如果客户端自动在 URL 后拼接 `/v1`，则 Base URL 只填 `https://modelboxs.com`，避免出现 `/v1/v1`。

---

## 10. 生产环境最佳实践

### 10.1 Key 管理

- 每个环境使用不同 Key：`dev`、`staging`、`prod` 分开。
- 每个业务线使用不同 Key，便于统计和风控。
- 生产 Key 不进入前端，不进入日志，不进入截图。
- 定期轮换 Key，离职、外包交接或仓库泄露后立即重置。

### 10.2 超时与重试

推荐设置：

| 场景 | 超时建议 |
| --- | --- |
| 普通文本对话 | 30-60 秒 |
| 长文本/推理模型 | 120-300 秒 |
| 图片生成 | 300 秒 |
| 批量任务 | 任务级超时 + 单请求超时 |

重试建议：

- 对 `429`、`500`、`502`、`503`、`504` 使用指数退避重试。
- 对 `400`、`401`、`403` 不要盲目重试，应检查参数、Key、模型权限或余额。
- 流式输出中断时，应由业务决定是否整段重试，避免重复扣费或重复生成。

### 10.3 模型降级

生产服务建议配置模型降级链：

```text
主模型：gpt-5.5
降级 1：claude-opus-4.7
降级 2：gemini-3.1-pro
降级 3：国产高性价比模型
```

实际降级顺序应根据任务类型、价格、延迟、上下文长度和输出风格确定。

### 10.4 日志字段

建议记录以下字段，便于排查问题：

| 字段 | 说明 |
| --- | --- |
| `request_id` | 平台或业务生成的请求 ID |
| `user_id` | 业务侧用户 ID，避免记录隐私明文 |
| `model` | 实际调用模型 |
| `endpoint` | 调用接口 |
| `status_code` | HTTP 状态码 |
| `latency_ms` | 请求耗时 |
| `prompt_tokens` | 输入 Token |
| `completion_tokens` | 输出 Token |
| `total_tokens` | 总 Token |
| `error_type` | 错误类型 |

---

## 11. 常见错误与排查

| 状态码 | 常见原因 | 处理方式 |
| --- | --- | --- |
| `400` | 请求参数错误、消息格式错误、模型不支持该参数 | 检查 JSON、`model`、`messages`、`max_tokens` 等 |
| `401` | API Key 无效、未传认证头 | 检查 Key 是否正确，认证头是否为 `Bearer` |
| `403` | 模型无权限、账户受限、风控拦截 | 检查模型权限、账户状态和平台规则 |
| `404` | 路径错误或模型不存在 | 确认是否漏写 `/v1`，用 `/v1/models` 查询模型 ID |
| `429` | 请求过快、并发过高、额度不足 | 降低并发，增加退避重试，检查余额或限额 |
| `500` | 平台或上游临时异常 | 稍后重试，保留请求 ID 联系支持 |
| `502/503/504` | 上游不可用、超时、网关异常 | 重试或切换备用模型 |

### 11.1 为什么提示模型不存在？

常见原因：

- `model` 填的是展示名，不是 API ID。
- 当前 Key 没有该模型权限。
- 模型已升级、下线或改名。
- 客户端写死了旧模型 ID。

建议先调用：

```bash
curl https://modelboxs.com/v1/models \
  -H "Authorization: Bearer $MODELBOXS_API_KEY"
```

### 11.2 为什么请求 404？

检查 Base URL：

- SDK 的 `base_url` 应为 `https://modelboxs.com/v1`。
- 如果客户端会自动追加 `/v1`，则填写 `https://modelboxs.com`。
- 不要把聊天请求发到图片接口，也不要把图片请求发到聊天接口。

### 11.3 为什么流式输出解析失败？

流式响应使用 SSE 格式，客户端需要逐行解析 `data:`。如果你使用官方 SDK，通常不需要自己解析；如果使用自定义 HTTP 客户端，请确认没有把流式响应当成普通 JSON 一次性解析。

---

## 12. 合規與用戶服務協議

### 12.1 平台聲明

本站僅針對非中華人民共和國行政管轄區域範圍內使用者提供 AI Token 算力資源服務；平台不主動取得、儲存或記錄使用者的具體使用內容、Prompt 及業務資料。

### 12.2 用戶服務協議

歡迎使用本平台提供之 AI Token 算力與 API 轉接服務。

使用者於註冊、登入、建立 API Key 或使用本平台服務前，請務必詳閱本協議。使用本服務即視為已閱讀、理解並同意本協議全部內容。

#### 12.2.1 服務性質

本平台為國際化 AI API 路由與算力資源平台，主要提供：

- AI Token 算力資源；
- API 介面轉接；
- 模型路由與負載調度；
- 開發者技術支援服務。

本平台不直接訓練、微調、生產或擁有第三方 AI 模型，相關模型能力均來自第三方模型服務供應商。

#### 12.2.2 服務對象與地區限制

本平台目前不面向中華人民共和國境內用戶提供服務。

使用者應自行確認其所在地、網路環境、付款方式及使用行為符合所在地法律法規及相關國際合規要求。

若平台判定使用者屬受限制地區、受制裁對象、高風險主體或存在合規風險，平台有權拒絕服務、限制功能或終止帳戶。

#### 12.2.3 使用者責任與禁止行為

使用者應合法、合規使用本平台服務，不得利用平台從事包括但不限於：

- 違法、暴力、色情、仇恨、詐騙等內容生成；
- 網路攻擊、漏洞利用、爬蟲濫用或惡意消耗資源；
- 洗錢、非法支付或其他違法商業活動；
- 侵害第三方智慧財產權、隱私權或資料權益；
- 繞過平台風控、限流、計費或安全機制；
- 批量濫發、垃圾行銷、虛假客服等濫用行為；
- 違反第三方模型服務商政策之行為。

平台有權對異常流量、高風險請求或違規行為進行限制、封禁或終止服務。

#### 12.2.4 資料、隱私與日誌政策

本平台重視使用者隱私與資料安全。

除維持服務運行、安全風控、計費統計與法律合規所必要之技術資訊外，平台原則上不主動長期保存使用者 Prompt、上傳內容、模型輸出或業務資料。

本平台不將使用者資料用於：

- AI 模型訓練；
- 模型微調；
- 廣告行銷；
- 商業出售或資料交易。

但於以下情況，平台可能進行必要的安全稽核與日誌分析：

- 系統安全事件；
- 攻擊與濫用行為；
- 異常資源消耗；
- 法律、監管或司法要求。

部分服務可能由第三方模型供應商處理，相關資料流轉與保留規則，亦可能受第三方服務政策約束。

#### 12.2.5 第三方模型與供應商說明

本平台部分能力來自第三方 AI 模型與雲服務供應商，包括但不限於：

- OpenAI
- Anthropic
- Google
- xAI
- DeepSeek

使用者理解並同意：

- 第三方模型之輸出內容、穩定性、上下文保留、政策限制與可用性，不受本平台完全控制；
- 上游供應商可能隨時調整價格、限額、能力或政策；
- 因第三方服務異常、中斷、封鎖、政策調整或不可抗力造成之損失，本平台不承擔直接責任。

#### 12.2.6 AI 生成內容說明

AI 模型輸出內容具有機率性、不確定性與幻覺風險。

平台不保證生成內容之：

- 真實性；
- 準確性；
- 合法性；
- 完整性；
- 適用性；
- 不侵權性。

使用者應自行判斷並承擔因使用 AI 生成內容所產生之一切風險與責任。

若法律或監管要求，平台有權對 AI 生成內容進行必要標識、風控或限制。

#### 12.2.7 API Key 與帳戶安全

使用者應妥善保管帳戶、API Key、Access Token 與相關憑證。

因使用者自身原因導致之：

- Key 外洩；
- 帳戶被盜；
- 非授權調用；
- 資源濫用；
- 費用損失；

均由使用者自行承擔責任。

平台有權對異常請求、高風險 IP、可疑流量或攻擊行為進行限制、凍結或封禁。

#### 12.2.8 計費、儲值與退款

平台依公示之價格與計費規則提供服務。

Token 消耗、API 呼叫次數、頻寬與資源統計，以平台系統記錄為準。

除法律另有規定外：

- 已消耗之資源原則上不予退款；
- 因第三方模型異常、網路波動或不可抗力導致之部分失敗請求，平台可依實際情況進行調整或補償；
- 平台有權依營運需要調整價格、套餐或資源策略。

#### 12.2.9 服務可用性

平台將盡力維持服務穩定與高可用性，但不保證服務絕對不中斷。

平台有權基於：

- 系統維護；
- 容量調整；
- 安全風控；
- 上游供應變更；
- 法律合規要求；

對部分模型、介面或功能進行升級、限流、暫停或下線。

#### 12.2.10 協議變更

平台有權依業務發展、法律法規或合規要求，對本協議進行更新。

更新後內容於平台公告後生效，使用者繼續使用服務即視為接受更新後協議。

#### 12.2.11 協議終止

若使用者存在以下情況，平台有權暫停或終止服務：

- 違反本協議；
- 存在違法或高風險行為；
- 濫用平台資源；
- 影響平台安全或穩定運行；
- 違反第三方供應商政策。

平台並保留追究相關責任之權利。

#### 12.2.12 其他說明

本協議之訂定、執行與解釋，依適用法律原則及國際商業慣例進行。

若本協議部分條款被認定無效，不影響其他條款之效力。

### 12.3 使用規範摘要

使用 ModelBoxs 時，請遵守：

- ModelBoxs 平台服務協議與公告。
- 所在地區適用法律法規。
- 上游模型服務商的使用政策。
- 資料隱私、版權、內容安全和行業監管要求。

禁止將平台用於違法、欺詐、攻擊、繞過安全限制、垃圾信息、侵權、惡意爬取、批量濫用或其他高風險用途。平台可能對異常請求、高風險 Key、異常 IP 或違規內容進行限制、暫停或終止服務。

---

## 13. 给用户的一句话接入说明

如果你已经会用 OpenAI SDK，只需要改两处：

```python
client = OpenAI(
    api_key="ModelBoxs 控制台创建的 API Key",
    base_url="https://modelboxs.com/v1",
)
```

然后把 `model` 改成 ModelBoxs 控制台显示的模型 ID，例如 GPT 5.5、Claude Opus 4.7、Gemini 3.1 Pro、GPT image-2 或对应国产模型 ID，即可开始调用。

---

## 14. 参考资料

- ModelBoxs 官网：https://modelboxs.com
- New API 使用 API 文档：https://docs.newapi.pro/zh/docs/guide/feature-guide/user/api
- ApiLink API 概览：https://apilink.cc/api.html
- ApiLink OpenAI 兼容 API：https://apilink.cc/api__openai.html
- ApiLink Anthropic API：https://apilink.cc/api__anthropic.html
- ApiLink GPT-Image-2 图片生成：https://apilink.cc/api__gpt-image-2.html
