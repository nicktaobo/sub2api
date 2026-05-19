# ModelBoxs User & API Integration Guide

Document version: 2026-05-18
Website: https://modelboxs.com
Platform positioning: AI API Gateway / Sub2API routing platform

ModelBoxs gives developers and teams a unified entry point for AI APIs. With a single platform key you can call Claude, GPT, Gemini, GPT Image, and major Chinese models, while staying compatible with the OpenAI, Anthropic, and Gemini request formats.

> Important notice: This service is intended only for users outside the People's Republic of China. The platform does not actively collect, store, or record user prompts, conversations, or business data.

This guide has two parts:

- User guide: registration, API key management, model listing, billing, usage logs, and security recommendations.
- Developer integration guide: OpenAI-compatible format, Claude/Anthropic native format, Gemini native format, image generation, third-party clients, and error handling.

> Note: Model names in this document are for integration illustration. Actual model availability, context length, pricing, billing multipliers, concurrency limits, and streaming support follow the ModelBoxs console, the model list API, and platform announcements.

---

## 1. Quick Start

### 1.1 Endpoints

| Item | Value |
| --- | --- |
| Website | `https://modelboxs.com` |
| OpenAI-compatible Base URL | `https://modelboxs.com/v1` |
| Anthropic/Claude Base URL | `https://modelboxs.com/v1` |
| Gemini native prefix | `https://modelboxs.com/v1beta` |
| Auth | `Authorization: Bearer YOUR_API_KEY` |
| Claude auth | `x-api-key: YOUR_API_KEY` or `Authorization: Bearer YOUR_API_KEY` |

### 1.2 First Call in Three Steps

1. Open `https://modelboxs.com` and sign up or log in.
2. Create an API key in the console and store it safely.
3. Replace your existing `base_url` with `https://modelboxs.com/v1`, and replace your `api_key` with the ModelBoxs key.

Minimal cURL example:

```bash
export MODELBOXS_API_KEY="sk-your-api-key"

curl https://modelboxs.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MODELBOXS_API_KEY" \
  -d '{
    "model": "gpt-5.5",
    "messages": [
      {"role": "user", "content": "Hello, introduce ModelBoxs in one sentence."}
    ],
    "stream": true
  }'
```

---

## 2. User Guide

### 2.1 Sign Up and Log In

Visit `https://modelboxs.com` and follow the on-screen steps to register, log in, and verify your email. Once logged in, the console gives you access to balance, API keys, usage logs, model permissions, and top-up records.

### 2.2 Create an API Key

In the console, find the "Tokens" or "API Keys" section and create a new key.

Recommended configuration:

| Field | Recommendation |
| --- | --- |
| Key name | Use a business or project name, e.g. `prod-chatbot`, `dev-test` |
| Quota limit | For production, set a per-key quota to limit blast radius if leaked |
| Model permissions | Grant only the models the project actually needs |
| IP restrictions | If the console supports it, bind production keys to your server's egress IP |
| Expiry | Set an expiry for temporary or test keys |

Security reminders:

- Never put the API key in frontend code, mobile installers, or public repositories.
- In production, keep keys in server-side environment variables, a secrets manager, or CI/CD secrets.
- If you suspect a leak, disable the old key immediately and create a new one.

### 2.3 List Available Models

You can view the models available to your account in the console, or via the API:

```bash
curl https://modelboxs.com/v1/models \
  -H "Authorization: Bearer $MODELBOXS_API_KEY"
```

Supported model families include but are not limited to:

| Family | Examples |
| --- | --- |
| Anthropic Claude | Claude Opus 4.7, Claude Sonnet/Haiku, and backward-compatible variants |
| OpenAI GPT | GPT 5.5, GPT 5.x, GPT 4.x, lightweight and reasoning models |
| OpenAI Image | GPT image-2 and compatible image-generation models |
| Google Gemini | Gemini 3.1 Pro, Gemini 3.x / 2.x series |
| Chinese models | DeepSeek, Qwen, Kimi, GLM, Hunyuan, Doubao, etc. |

When making a call, the `model` field must exactly match the model ID shown in the console. The display name and the API ID may differ.

### 2.4 Pre-Integration Checklist

The platform does not offer a Playground. For your first integration, run a minimal cURL test using the examples in this guide and verify:

- The API key is created and copied in full.
- The model ID matches the console exactly.
- Your balance or plan is active.
- The base URL is set to `https://modelboxs.com/v1`, or `https://modelboxs.com` if the client appends `/v1` automatically.

### 2.5 Billing and Usage

ModelBoxs typically bills by model, input tokens, output tokens, image count, or task type. Pricing varies per model — see the console pricing page for the source of truth.

Recommendations:

- During testing, use low-cost models or lower image-quality settings.
- For long-context tasks, set a reasonable `max_tokens` to avoid unintentional spend.
- In production, log the model, input length, output length, request ID, and your business user ID for each call.
- Review the "Usage", "Spending", or "Logs" page periodically.

---

## 3. Developer API Integration

### 3.1 Authentication

OpenAI-compatible endpoints:

```http
Authorization: Bearer YOUR_API_KEY
Content-Type: application/json
```

Anthropic / Claude native endpoint:

```http
x-api-key: YOUR_API_KEY
anthropic-version: 2023-06-01
Content-Type: application/json
```

Gemini native endpoint typically uses a query parameter:

```text
https://modelboxs.com/v1beta/models/{model}:generateContent?key=YOUR_API_KEY
```

### 3.2 Common Endpoints

| Type | Method & Path | Description |
| --- | --- | --- |
| Models list | `GET /v1/models` | List models available to the current key |
| Chat completions | `POST /v1/chat/completions` | OpenAI Chat Completions format |
| Responses API | `POST /v1/responses` | OpenAI Responses format, useful for newer projects |
| Claude Messages | `POST /v1/messages` | Anthropic Messages native format |
| Gemini generateContent | `POST /v1beta/models/{model}:generateContent` | Gemini native format |
| Image generation | `POST /v1/images/generations` | GPT image-2 text-to-image |
| Image editing | `POST /v1/images/edits` | Image edits, subject to model permissions |

Note: Embeddings, Rerank, and audio capabilities will be documented separately if and when they are released. Do not integrate them before they are confirmed open.

---

## 4. OpenAI-Compatible Format

Best for:

- Calling OpenAI models (GPT 5.5, GPT 5.x, GPT 4.x).
- Calling Claude, Gemini, and Chinese models that accept the OpenAI Chat Completions format.
- Integrating with OpenAI-compatible clients such as Cherry Studio, Chatbox, LobeChat, Open WebUI, Continue, and Cline.

### 4.1 Python SDK

Install:

```bash
pip install openai
```

Example:

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
        {"role": "system", "content": "You are a professional, reliable AI assistant."},
        {"role": "user", "content": "Write a quicksort in Python."},
    ],
    stream=True,
)

for chunk in stream:
    delta = chunk.choices[0].delta
    if delta.content:
        print(delta.content, end="", flush=True)
```

Notes:

- For newer reasoning or long-output models like GPT 5.5, prefer `stream: true`.
- If a model is marked "streaming required" in the console, non-streaming calls may return an error.
- Non-GPT models generally support both streaming and non-streaming — check the model description.

### 4.2 Node.js SDK

Install:

```bash
npm install openai
```

Example:

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
    { role: "system", content: "You are a professional, reliable AI assistant." },
    { role: "user", content: "Explain what RAG is." },
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
      {"role": "user", "content": "Draft a pricing-page copy framework for a SaaS product."}
    ],
    "temperature": 0.7,
    "max_tokens": 2000,
    "stream": true
  }'
```

### 4.4 Calling Chinese Models

Chinese models are typically callable through the OpenAI-compatible format. Just set `model` to the Chinese model ID shown in the console.

```bash
curl https://modelboxs.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MODELBOXS_API_KEY" \
  -d '{
    "model": "replace-with-your-chinese-model-id",
    "messages": [
      {"role": "user", "content": "Summarize the main risk points in this Chinese contract."}
    ],
    "stream": false
  }'
```

---

## 5. OpenAI Responses API

If your project already uses the OpenAI Responses API, try `/v1/responses`.

```bash
curl https://modelboxs.com/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MODELBOXS_API_KEY" \
  -d '{
    "model": "gpt-5.5",
    "input": "Give me a list of monitoring metrics for an API gateway."
  }'
```

Python:

```python
import os
from openai import OpenAI

client = OpenAI(
    api_key=os.environ["MODELBOXS_API_KEY"],
    base_url="https://modelboxs.com/v1",
)

response = client.responses.create(
    model="gpt-5.5",
    input="Give me a list of monitoring metrics for an API gateway.",
)

print(response.output_text)
```

If your SDK or framework does not yet support the Responses API, prefer the more widely compatible `/v1/chat/completions`.

---

## 6. Claude / Anthropic Native Format

Best for:

- Maximum compatibility with the official Anthropic SDK.
- Calling Claude Opus 4.7, Claude Sonnet, Claude Haiku, etc.
- Integrating with Claude Code, the Claude SDK, or tools that speak the Anthropic Messages format.

### 6.1 cURL

```bash
curl https://modelboxs.com/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: $MODELBOXS_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-opus-4.7",
    "max_tokens": 2048,
    "system": "You are a senior software architect.",
    "messages": [
      {"role": "user", "content": "Design a monitoring and alerting plan for a multi-model API gateway."}
    ]
  }'
```

### 6.2 Python SDK

Install:

```bash
pip install anthropic
```

Example:

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
    system="You are a senior software architect.",
    messages=[
        {"role": "user", "content": "Explain the difference between an API Gateway and a model router."}
    ],
)

print(message.content[0].text)
```

### 6.3 Node.js SDK

Install:

```bash
npm install @anthropic-ai/sdk
```

Example:

```javascript
import Anthropic from "@anthropic-ai/sdk";

const client = new Anthropic({
  apiKey: process.env.MODELBOXS_API_KEY,
  baseURL: "https://modelboxs.com/v1",
});

const message = await client.messages.create({
  model: "claude-opus-4.7",
  max_tokens: 2048,
  system: "You are a senior software architect.",
  messages: [
    { role: "user", content: "Give me a high-availability architecture checklist for an API relay." },
  ],
});

console.log(message.content[0].text);
```

### 6.4 Claude Messages Notes

- `system` is a top-level field. Do not put it inside the `messages` array.
- `messages` typically only uses the `user` and `assistant` roles.
- For multi-turn conversations, keep `user` and `assistant` alternating.
- Image input and multimodal capability depend on the model and your account permissions.

---

## 7. Gemini Native Format

Best for:

- Calling Gemini 3.1 Pro and the Gemini 3.x / 2.x series.
- Projects that already use Google Gemini's `generateContent` format.

### 7.1 cURL

```bash
curl "https://modelboxs.com/v1beta/models/gemini-3.1-pro:generateContent?key=$MODELBOXS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [
      {
        "role": "user",
        "parts": [
          {"text": "Compare GPT, Claude, and Gemini in a table by use case."}
        ]
      }
    ],
    "generationConfig": {
      "temperature": 0.7,
      "maxOutputTokens": 2048
    }
  }'
```

### 7.2 Calling Gemini in OpenAI-Compatible Mode

If your stack is standardized on the OpenAI format, you can also call Gemini models through `/v1/chat/completions`:

```bash
curl https://modelboxs.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MODELBOXS_API_KEY" \
  -d '{
    "model": "gemini-3.1-pro",
    "messages": [
      {"role": "user", "content": "Generate an outline for a product requirements document."}
    ],
    "stream": false
  }'
```

---

## 8. GPT image-2 Image Generation

Best for:

- Text-to-image, posters, product concept art, logo drafts, visual proposals.
- Projects that already use the OpenAI Images API.

### 8.1 cURL

```bash
curl https://modelboxs.com/v1/images/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MODELBOXS_API_KEY" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "A clean, professional B2B-SaaS hero visual for the ModelBoxs AI API Gateway homepage.",
    "size": "1536x1024",
    "quality": "medium",
    "n": 1
  }'
```

### 8.2 Common Parameters

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `model` | string | yes | Image model ID, e.g. `gpt-image-2` |
| `prompt` | string | yes | Image-generation prompt |
| `size` | string | no | Common values: `1024x1024`, `1024x1536`, `1536x1024`, `2048x2048`, `3840x2160`, `auto` |
| `quality` | string | no | Common values: `low`, `medium`, `high` |
| `n` | integer | no | Image count, usually start at `1` |

Notes:

- Image generation is NOT a chat endpoint. Do not call `/v1/chat/completions`.
- High-resolution and high-quality modes take longer — set the client timeout to about 300 seconds.
- Final sizes, quality tiers, and pricing follow the console.

### 8.3 Saving Images in Python

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
    prompt="A professional product hero visual for an AI API Gateway homepage",
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

## 9. Third-Party Client Configuration

Most clients that support an OpenAI Compatible provider can integrate with ModelBoxs.

| Client / Tool | Provider type | Base URL | API Key | Model |
| --- | --- | --- | --- | --- |
| Cherry Studio | OpenAI Compatible | `https://modelboxs.com/v1` | ModelBoxs key | Console model ID |
| Chatbox | OpenAI API Compatible | `https://modelboxs.com/v1` | ModelBoxs key | Console model ID |
| LobeChat | OpenAI Compatible | `https://modelboxs.com/v1` | ModelBoxs key | Console model ID |
| Open WebUI | OpenAI Compatible | `https://modelboxs.com/v1` | ModelBoxs key | Console model ID |
| Continue / Cline | OpenAI Compatible or Anthropic | `https://modelboxs.com/v1` | ModelBoxs key | Console model ID |
| Claude Code | Anthropic Compatible | `https://modelboxs.com/v1` | ModelBoxs key | Claude model ID |

General rules:

- When the provider is OpenAI Compatible, set Base URL to `https://modelboxs.com/v1`.
- When the provider is Anthropic Compatible, the Base URL is still `https://modelboxs.com/v1`.
- The Model field must be filled in manually with the model ID shown in the console.
- If the client appends `/v1` automatically, set Base URL to `https://modelboxs.com` to avoid `/v1/v1`.

---

## 10. Production Best Practices

### 10.1 Key Management

- Use a different key per environment: `dev`, `staging`, `prod`.
- Use a different key per business line for clearer attribution and risk control.
- Production keys never go to the frontend, logs, or screenshots.
- Rotate keys regularly, and immediately after an employee leaves, a contractor handoff, or a repo leak.

### 10.2 Timeouts and Retries

Recommended timeouts:

| Scenario | Timeout |
| --- | --- |
| Standard text chat | 30–60 s |
| Long context / reasoning models | 120–300 s |
| Image generation | 300 s |
| Batch jobs | Task-level timeout + per-request timeout |

Retry guidelines:

- Retry `429`, `500`, `502`, `503`, `504` with exponential backoff.
- Do NOT blindly retry `400`, `401`, `403`. Check the request payload, key, model permissions, or balance instead.
- When a streaming response is interrupted, let the business decide whether to retry the whole request to avoid double-billing or duplicate generation.

### 10.3 Model Fallback

Configure a fallback chain for production:

```text
Primary:    gpt-5.5
Fallback 1: claude-opus-4.7
Fallback 2: gemini-3.1-pro
Fallback 3: a cost-effective Chinese model
```

The actual fallback order should reflect task type, price, latency, context length, and output style.

### 10.4 Logging Fields

For each call, log fields like:

| Field | Description |
| --- | --- |
| `request_id` | Platform or business request ID |
| `user_id` | Business user ID — avoid logging PII verbatim |
| `model` | Model actually called |
| `endpoint` | Endpoint hit |
| `status_code` | HTTP status code |
| `latency_ms` | Request latency |
| `prompt_tokens` | Input tokens |
| `completion_tokens` | Output tokens |
| `total_tokens` | Total tokens |
| `error_type` | Error category |

---

## 11. Common Errors and Troubleshooting

| Status | Common causes | What to do |
| --- | --- | --- |
| `400` | Bad parameters, malformed messages, unsupported parameter for this model | Check JSON, `model`, `messages`, `max_tokens`, etc. |
| `401` | Invalid API key or missing auth header | Check that the key is correct and the header is `Bearer` |
| `403` | No permission for that model, account restricted, blocked by risk control | Check model permissions, account status, and platform rules |
| `404` | Wrong path or unknown model | Make sure you didn't drop `/v1`; verify the model ID via `/v1/models` |
| `429` | Too fast, too much concurrency, or quota exceeded | Reduce concurrency, add backoff, check balance/limits |
| `500` | Temporary platform or upstream issue | Retry later, keep the request ID for support |
| `502/503/504` | Upstream unavailable, timeout, or gateway error | Retry or switch to a backup model |

### 11.1 Why am I getting "model not found"?

Common causes:

- `model` is set to the display name, not the API ID.
- The current key does not have permission for that model.
- The model was upgraded, deprecated, or renamed.
- The client has a hardcoded old model ID.

Start by listing models:

```bash
curl https://modelboxs.com/v1/models \
  -H "Authorization: Bearer $MODELBOXS_API_KEY"
```

### 11.2 Why is my request returning 404?

Check the Base URL:

- The SDK's `base_url` should be `https://modelboxs.com/v1`.
- If the client appends `/v1` automatically, set Base URL to `https://modelboxs.com`.
- Do not send chat requests to the image endpoint and vice versa.

### 11.3 Why is my streaming response failing to parse?

Streaming responses use the SSE format and need to be parsed line by line (`data:` lines). Official SDKs handle this for you; with a custom HTTP client, make sure you are not parsing the streamed body as a single JSON document.

---

## 12. Compliance and Terms of Service

### 12.1 Platform Statement

This service is intended only for users outside the People's Republic of China. The platform does not actively collect, store, or record user prompts, conversations, or business data.

### 12.2 Terms of Service

Welcome to the platform's AI token compute and API relay service.

By signing up, logging in, creating an API key, or otherwise using the service, you confirm that you have read, understood, and agree to these Terms.

#### 12.2.1 Nature of Service

The platform is an international AI API routing and compute service that provides:

- AI token compute resources;
- API relay;
- Model routing and load balancing;
- Developer technical support.

The platform does not train, fine-tune, produce, or own any third-party AI model. Model capabilities come from third-party providers.

#### 12.2.2 Audience and Regional Restrictions

The platform does not currently serve users inside the People's Republic of China.

Users are responsible for ensuring that their location, network environment, payment method, and usage comply with local laws and applicable international compliance rules.

If the platform determines that a user is in a restricted region, on a sanctions list, a high-risk subject, or otherwise carries compliance risk, the platform may refuse service, restrict features, or terminate the account.

#### 12.2.3 User Responsibilities and Prohibited Use

Users must use the service lawfully and may not use it to:

- Generate illegal, violent, sexual, hateful, or fraudulent content;
- Conduct cyber attacks, exploit vulnerabilities, abuse scraping, or maliciously exhaust resources;
- Engage in money laundering, illegal payments, or other unlawful commerce;
- Infringe third-party intellectual property, privacy, or data rights;
- Bypass the platform's risk-control, rate-limit, billing, or security mechanisms;
- Spam, mass-broadcast, run fake customer service, or otherwise abuse the service;
- Violate any third-party model provider's policy.

The platform may rate-limit, ban, or terminate service for traffic that looks anomalous, high-risk, or non-compliant.

#### 12.2.4 Data, Privacy, and Logging

User privacy and data security are a priority.

Aside from technical information required to keep the service running, enforce security, bill users, and meet legal requirements, the platform does not actively retain user prompts, uploads, model outputs, or business data for the long term.

The platform does NOT use user data for:

- Training AI models;
- Fine-tuning models;
- Advertising;
- Commercial sale or data trading.

The platform MAY perform necessary auditing and log analysis in cases such as:

- System security incidents;
- Attacks or abuse;
- Abnormal resource consumption;
- Legal, regulatory, or judicial requests.

Some calls are processed by third-party model providers, whose policies govern data flow and retention on their side.

#### 12.2.5 Third-Party Models and Providers

Some capabilities come from third-party AI and cloud providers, including but not limited to:

- OpenAI
- Anthropic
- Google
- xAI
- DeepSeek

Users understand and accept that:

- The output, stability, context retention, policy limits, and availability of third-party models are not fully under the platform's control;
- Upstream providers may change pricing, limits, capabilities, or policies at any time;
- The platform is not directly liable for losses caused by third-party outages, blocking, policy changes, or force majeure.

#### 12.2.6 AI-Generated Content

AI model output is probabilistic, uncertain, and prone to hallucination.

The platform does NOT guarantee that generated content is:

- True;
- Accurate;
- Lawful;
- Complete;
- Fit for purpose;
- Free of infringement.

Users are responsible for evaluating AI-generated output and bear all risk and responsibility associated with its use.

Where required by law or regulation, the platform may add labels, risk controls, or limits to AI-generated content.

#### 12.2.7 API Keys and Account Security

Users must keep their accounts, API keys, access tokens, and other credentials safe.

The user is solely responsible for losses arising from:

- Key leaks;
- Account compromise;
- Unauthorized calls;
- Resource abuse;
- Charges incurred.

The platform may rate-limit, freeze, or ban any abnormal requests, high-risk IPs, suspicious traffic, or attack behavior.

#### 12.2.8 Billing, Top-Ups, and Refunds

The platform bills according to its published pricing and rules.

Token consumption, API call counts, bandwidth, and other resource use are measured by the platform's system records.

Unless required by law:

- Consumed resources are non-refundable;
- For partial failures caused by third-party model issues, network jitter, or force majeure, the platform may adjust or compensate at its discretion;
- The platform may adjust prices, plans, or resource policies as the business requires.

#### 12.2.9 Service Availability

The platform makes a best effort to keep the service stable and highly available but does not guarantee uninterrupted service.

The platform may upgrade, rate-limit, pause, or remove certain models, endpoints, or features for:

- System maintenance;
- Capacity changes;
- Security and risk control;
- Upstream supply changes;
- Legal and compliance reasons.

#### 12.2.10 Changes to the Terms

The platform may update these Terms as the business, law, or compliance environment evolves.

Updated Terms take effect when posted. Continued use of the service constitutes acceptance of the updated Terms.

#### 12.2.11 Termination

The platform may suspend or terminate service for users who:

- Breach these Terms;
- Engage in illegal or high-risk behavior;
- Abuse platform resources;
- Threaten platform security or stability;
- Violate a third-party provider's policy.

The platform also reserves the right to pursue legal remedies.

#### 12.2.12 Miscellaneous

These Terms are interpreted and enforced according to applicable law and international commercial custom.

If any provision is held invalid, the remaining provisions remain in effect.

### 12.3 Acceptable-Use Summary

When using ModelBoxs, comply with:

- ModelBoxs platform Terms of Service and announcements.
- The laws of your jurisdiction.
- The acceptable-use policies of the upstream model providers.
- Data privacy, copyright, content safety, and industry-specific regulations.

You may not use the platform for illegal activity, fraud, attacks, bypassing security limits, spam, infringement, malicious scraping, mass abuse, or other high-risk purposes. The platform may rate-limit, suspend, or terminate service in response to anomalous requests, high-risk keys, suspicious IPs, or violations.

---

## 13. The Shortest Possible Integration Note

If you already use the OpenAI SDK, you only need to change two things:

```python
client = OpenAI(
    api_key="your ModelBoxs API key",
    base_url="https://modelboxs.com/v1",
)
```

Then set `model` to a model ID shown in the console — for example GPT 5.5, Claude Opus 4.7, Gemini 3.1 Pro, GPT image-2, or a Chinese model ID — and you are ready to call.

---

## 14. References

- ModelBoxs website: https://modelboxs.com
- New API user docs: https://docs.newapi.pro/zh/docs/guide/feature-guide/user/api
- ApiLink API overview: https://apilink.cc/api.html
- ApiLink OpenAI-compatible API: https://apilink.cc/api__openai.html
- ApiLink Anthropic API: https://apilink.cc/api__anthropic.html
- ApiLink GPT-Image-2 image generation: https://apilink.cc/api__gpt-image-2.html
