# ModelBoxs Quick Start Guide

Website: https://modelboxs.com
Base URL: `https://modelboxs.com/v1`
Auth: `Authorization: Bearer YOUR_API_KEY`

ModelBoxs is a multi-model AI API gateway. With a single ModelBoxs API key you can call Claude, GPT, Gemini, GPT image, and major Chinese models, while staying compatible with the OpenAI, Anthropic, and Gemini request formats.

Important notice: This service is intended only for users outside the People's Republic of China. The platform does not actively collect, store, or record user prompts, conversations, or business data.

## 1. Get Started in Three Steps

1. Open `https://modelboxs.com` and sign up or log in.
2. Create an API key in the console.
3. Replace the official base URL in your existing project with `https://modelboxs.com/v1`, and switch the API key to your ModelBoxs key.

## 2. OpenAI-Compatible Calls

Works for GPT 5.5, GPT 5.x, GPT 4.x, as well as Claude, Gemini, and Chinese models that accept the OpenAI format.

```bash
export MODELBOXS_API_KEY="sk-your-api-key"

curl https://modelboxs.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MODELBOXS_API_KEY" \
  -d '{
    "model": "gpt-5.5",
    "messages": [
      {"role": "user", "content": "Hello, please introduce ModelBoxs."}
    ],
    "stream": true
  }'
```

Python:

```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-your-api-key",
    base_url="https://modelboxs.com/v1",
)

response = client.chat.completions.create(
    model="gpt-5.5",
    messages=[{"role": "user", "content": "Hello"}],
)

print(response.choices[0].message.content)
```

## 3. Claude Native Calls

Works for Claude Opus 4.7, Claude Sonnet, Claude Haiku, and similar models.

```bash
curl https://modelboxs.com/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: $MODELBOXS_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-opus-4.7",
    "max_tokens": 2048,
    "messages": [
      {"role": "user", "content": "Draft a technical proposal for an API gateway."}
    ]
  }'
```

## 4. Gemini Native Calls

Works for Gemini 3.1 Pro and the Gemini 3.x / 2.x series.

```bash
curl "https://modelboxs.com/v1beta/models/gemini-3.1-pro:generateContent?key=$MODELBOXS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [
      {
        "role": "user",
        "parts": [{"text": "Summarize the benefits of a multi-model API gateway."}]
      }
    ]
  }'
```

## 5. GPT image-2 Image Generation

```bash
curl https://modelboxs.com/v1/images/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MODELBOXS_API_KEY" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "A professional hero visual for an AI API Gateway product page",
    "size": "1536x1024",
    "quality": "medium",
    "n": 1
  }'
```

## 6. Third-Party Client Configuration

| Field | Value |
| --- | --- |
| Provider | OpenAI Compatible |
| Base URL | `https://modelboxs.com/v1` |
| API Key | Key created in the ModelBoxs console |
| Model | Model ID as shown in the console |

If the client automatically appends `/v1`, set the Base URL to `https://modelboxs.com` to avoid ending up with `/v1/v1`.

## 7. Notes

- `model` must match the model ID shown in the console exactly.
- The platform does not offer a Playground. For your first integration, run a minimal cURL test using the examples above.
- Never expose the API key in frontend code, public repositories, or client installers.
- For image generation and long-form inference, set a longer client timeout.
- In production, enable streaming, retries, rate limiting, request logging, and model fallback.
- Final model availability, pricing, context length, and permissions follow the ModelBoxs console.
- Read and accept the full Terms of Service before using the platform.

See the full reference at `ModelBoxs-用户使用与API接入文档.md`.
