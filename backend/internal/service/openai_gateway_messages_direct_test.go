package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// normalizeAnthropicDirectInputUsage 的口径契约：
//   - DeepSeek（语义已实测：input_tokens 只报缓存未命中数）无条件加回 cache_read；
//   - 其他平台（Kimi 等，语义未实测）仅在 input_tokens < cache_read（明显为未命中
//     口径）时加回，总量口径上游不得双重计费。
func TestNormalizeAnthropicDirectInputUsage(t *testing.T) {
	t.Run("DeepSeek 未命中口径无条件加回", func(t *testing.T) {
		// 实测样例：3015 token prompt → input=71, cache_read=2944
		u := OpenAIUsage{InputTokens: 71, CacheReadInputTokens: 2944}
		normalizeAnthropicDirectInputUsage(PlatformDeepSeek, &u)
		require.Equal(t, 3015, u.InputTokens)
	})

	t.Run("DeepSeek 新增内容超过缓存前缀也加回", func(t *testing.T) {
		u := OpenAIUsage{InputTokens: 5000, CacheReadInputTokens: 2944}
		normalizeAnthropicDirectInputUsage(PlatformDeepSeek, &u)
		require.Equal(t, 7944, u.InputTokens, "条件判断 (input < cache_read) 会在此漏计")
	})

	t.Run("Moonshot 总量口径不得双重计费", func(t *testing.T) {
		// 若 Kimi 按 Anthropic 总量口径上报（input 已含全部输入），不应再加回
		u := OpenAIUsage{InputTokens: 3015, CacheReadInputTokens: 2944}
		normalizeAnthropicDirectInputUsage(PlatformMoonshot, &u)
		require.Equal(t, 3015, u.InputTokens)
	})

	t.Run("Moonshot 明显未命中口径仍加回", func(t *testing.T) {
		u := OpenAIUsage{InputTokens: 71, CacheReadInputTokens: 2944}
		normalizeAnthropicDirectInputUsage(PlatformMoonshot, &u)
		require.Equal(t, 3015, u.InputTokens)
	})

	t.Run("无缓存命中为 no-op", func(t *testing.T) {
		u := OpenAIUsage{InputTokens: 100}
		normalizeAnthropicDirectInputUsage(PlatformDeepSeek, &u)
		require.Equal(t, 100, u.InputTokens)
	})
}

// buildAnthropicDirectMessagesURL 的逐平台 URL 约定契约。
// GLM 端点根因上游而异：智谱官方/z.ai 在 /api/anthropic 下，NewAPI 中转在根。
func TestBuildAnthropicDirectMessagesURL(t *testing.T) {
	apikey := func(platform, baseURL string) *Account {
		return &Account{Platform: platform, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": baseURL}}
	}
	cases := []struct {
		name    string
		account *Account
		want    string
	}{
		{"DeepSeek 默认", &Account{Platform: PlatformDeepSeek, Type: AccountTypeAPIKey}, "https://api.deepseek.com/anthropic/v1/messages"},
		{"Moonshot 默认", &Account{Platform: PlatformMoonshot, Type: AccountTypeAPIKey}, "https://api.kimi.com/coding/v1/messages"},
		{"GLM 官方默认 base 补 /api/anthropic", &Account{Platform: PlatformGLM, Type: AccountTypeAPIKey}, "https://open.bigmodel.cn/api/anthropic/v1/messages"},
		{"GLM 官方 paas 根也归一到 /api/anthropic", apikey(PlatformGLM, "https://open.bigmodel.cn/api/paas/v4"), "https://open.bigmodel.cn/api/anthropic/v1/messages"},
		{"GLM 官方已含 /api/anthropic 不重复", apikey(PlatformGLM, "https://open.bigmodel.cn/api/anthropic"), "https://open.bigmodel.cn/api/anthropic/v1/messages"},
		{"GLM z.ai 补 /api/anthropic", apikey(PlatformGLM, "https://api.z.ai"), "https://api.z.ai/api/anthropic/v1/messages"},
		{"GLM NewAPI 中转根直挂 /v1/messages", apikey(PlatformGLM, "https://relay.orbitai.cc"), "https://relay.orbitai.cc/v1/messages"},
		{"GLM 中转 base 带 /v1 归一", apikey(PlatformGLM, "https://relay.orbitai.cc/v1"), "https://relay.orbitai.cc/v1/messages"},
		{"GLM 中转 base 带末尾斜杠", apikey(PlatformGLM, "https://relay.orbitai.cc/"), "https://relay.orbitai.cc/v1/messages"},
		{"Qwen 官方默认 base → claude-code-proxy", &Account{Platform: PlatformQwen, Type: AccountTypeAPIKey}, "https://dashscope.aliyuncs.com/api/v2/apps/claude-code-proxy/v1/messages"},
		{"Qwen 显式 compatible-mode → claude-code-proxy", apikey(PlatformQwen, "https://dashscope.aliyuncs.com/compatible-mode/v1"), "https://dashscope.aliyuncs.com/api/v2/apps/claude-code-proxy/v1/messages"},
		{"Qwen 中转 host 剥 compatible-mode/v1", apikey(PlatformQwen, "https://relay.example.com/compatible-mode/v1"), "https://relay.example.com/v1/messages"},
		{"Qwen 中转 host 根直挂", apikey(PlatformQwen, "https://relay.example.com"), "https://relay.example.com/v1/messages"},
		{"未支持平台返回空", &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, buildAnthropicDirectMessagesURL(tc.account))
		})
	}
}
