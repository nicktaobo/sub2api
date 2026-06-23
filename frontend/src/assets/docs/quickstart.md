# ModelBoxs 快速接入说明

官网：https://modelboxs.com  
Base URL：`https://modelboxs.com/v1`  
认证方式：`Authorization: Bearer YOUR_API_KEY`

ModelBoxs 是一个多模型 AI API Gateway。用户可以使用一个 ModelBoxs API Key 调用 Claude、GPT、Gemini、GPT image、国产模型等多家模型能力，并兼容主流 OpenAI / Anthropic / Gemini 接入格式。

重要聲明：本站僅針對非中華人民共和國行政管轄區域範圍內使用者提供 AI Token 算力資源服務；平台不主動取得、儲存或記錄使用者的具體使用內容、Prompt 及業務資料。

## 1. 三步开始使用

1. 打开 `https://modelboxs.com` 注册或登录。
2. 在控制台创建 API Key。
3. 将原项目中的官方接口地址替换为 `https://modelboxs.com/v1`，并使用 ModelBoxs API Key。

## 2. OpenAI 兼容调用

适用于 GPT 5.5、GPT 5.x、GPT 4.x，以及支持 OpenAI 格式的 Claude、Gemini、国产模型。

```bash
export MODELBOXS_API_KEY="sk-your-api-key"

curl https://modelboxs.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MODELBOXS_API_KEY" \
  -d '{
    "model": "gpt-5.5",
    "messages": [
      {"role": "user", "content": "你好，请介绍一下 ModelBoxs。"}
    ],
    "stream": true
  }'
```

Python：

```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-your-api-key",
    base_url="https://modelboxs.com/v1",
)

response = client.chat.completions.create(
    model="gpt-5.5",
    messages=[{"role": "user", "content": "你好"}],
)

print(response.choices[0].message.content)
```

## 3. Claude 原生调用

适用于 Claude Opus 4.7、Claude Sonnet、Claude Haiku 等模型。

```bash
curl https://modelboxs.com/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: $MODELBOXS_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-opus-4.7",
    "max_tokens": 2048,
    "messages": [
      {"role": "user", "content": "请写一份 API 网关技术方案。"}
    ]
  }'
```

## 4. Gemini 原生调用

适用于 Gemini 3.1 Pro、Gemini 3.x/2.x 系列模型。

```bash
curl "https://modelboxs.com/v1beta/models/gemini-3.1-pro:generateContent?key=$MODELBOXS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [
      {
        "role": "user",
        "parts": [{"text": "请总结多模型 API 网关的优势。"}]
      }
    ]
  }'
```

## 5. GPT image-2 图片生成

```bash
curl https://modelboxs.com/v1/images/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MODELBOXS_API_KEY" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "一张专业的 AI API Gateway 产品主视觉",
    "size": "1536x1024",
    "quality": "medium",
    "n": 1
  }'
```

## 6. 第三方客户端配置

| 配置项 | 填写内容 |
| --- | --- |
| Provider | OpenAI Compatible |
| Base URL | `https://modelboxs.com/v1` |
| API Key | ModelBoxs 控制台创建的 Key |
| Model | 控制台展示的模型 ID |

如果客户端会自动追加 `/v1`，Base URL 请填写 `https://modelboxs.com`，避免变成 `/v1/v1`。

## 7. 注意事项

- `model` 必须填写控制台展示的准确模型 ID。
- 当前平台不提供操练台或 Playground，首次接入建议使用本文 cURL 示例进行最小化测试。
- 不要把 API Key 暴露在前端、公开仓库或客户端安装包中。
- 图片生成和长文本推理建议设置更长超时。
- 生产环境建议开启流式输出、重试、限流、日志记录和模型降级。
- 可用模型、价格、上下文长度和权限以 ModelBoxs 控制台为准。
- 使用服务前请阅读并遵守完整用户服务协议。

完整文档见：`ModelBoxs-用户使用与API接入文档.md`
