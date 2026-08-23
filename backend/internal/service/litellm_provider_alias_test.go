package service

import "testing"

// 平台标识 moonshot 改名成 kimi 后，LiteLLM 价目表里 Kimi 系模型的 litellm_provider
// 仍然是 "moonshot"。没有别名层，ListAll("kimi") 恒空 → 公开模型广场 /models 与
// 用户定价页的兜底链（账号全透传 → 回落平台全表）拿不到模型名 → Kimi 分组显示为空。
func TestLiteLLMProviderMatchesKimiAlias(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		entryProvider string
		filter        string
		want          bool
	}{
		{"kimi 过滤命中 LiteLLM 的 moonshot", "moonshot", "kimi", true},
		{"大小写不敏感", "MoonShot", "KIMI", true},
		{"带空白的过滤值", "moonshot", "  kimi  ", true},
		{"LiteLLM 将来补上 kimi 也命中", "kimi", "kimi", true},
		{"zhipu 两侧同名", "zhipu", "zhipu", true},
		{"deepseek 两侧同名", "deepseek", "deepseek", true},

		// 别名单向：admin 渠道定价同步传的是 LiteLLM provider 名，不能被放宽。
		{"别名不反向生效：moonshot 过滤不吃 kimi 条目", "kimi", "moonshot", false},
		{"别名不泄漏到别的平台", "moonshot", "zhipu", false},
		{"无关 provider 不命中", "anthropic", "kimi", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := litellmProviderMatches(tc.entryProvider, tc.filter); got != tc.want {
				t.Errorf("litellmProviderMatches(%q, %q) = %v, want %v",
					tc.entryProvider, tc.filter, got, tc.want)
			}
		})
	}
}

// 别名表的键必须是真实平台标识，否则是死条目（平台改名/下线后没人清理）。
func TestLiteLLMProviderAliasKeysAreRealPlatforms(t *testing.T) {
	t.Parallel()

	for platform := range litellmProviderAliases {
		if !IsAllowedQuotaPlatform(platform) {
			t.Errorf("别名表里的 %q 不在 AllowedQuotaPlatforms 中，是死条目", platform)
		}
	}
}

// 端到端守住真实兜底链：PricingService.ListAll(平台标识) 必须能列出 Kimi 系模型。
// 这是公开模型广场 /models 与定价页 getLiteLLMModels 的唯一数据来源。
func TestPricingServiceListAllResolvesKimiViaMoonshotEntries(t *testing.T) {
	t.Parallel()

	s := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"kimi-k2-0905-preview":  {LiteLLMProvider: "moonshot", Mode: "chat"},
			"moonshot-v1-8k":        {LiteLLMProvider: "moonshot", Mode: "chat"},
			"glm-4.6":               {LiteLLMProvider: "zhipu", Mode: "chat"},
			"deepseek-chat":         {LiteLLMProvider: "deepseek", Mode: "chat"},
			"gpt-5.4":               {LiteLLMProvider: "openai", Mode: "chat"},
			"moonshot-embedding-v1": {LiteLLMProvider: "moonshot", Mode: "embedding"},
		},
	}

	names := func(entries []LiteLLMModelEntry) []string {
		out := make([]string, 0, len(entries))
		for _, e := range entries {
			out = append(out, e.Model)
		}
		return out
	}

	got := names(s.ListAll(PlatformKimi))
	want := []string{"kimi-k2-0905-preview", "moonshot-v1-8k"} // 已按字母序；embedding 条目被 mode 过滤掉
	if len(got) != len(want) {
		t.Fatalf("ListAll(%q) = %v, want %v", PlatformKimi, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ListAll(%q) = %v, want %v", PlatformKimi, got, want)
		}
	}

	if got := names(s.ListAll(PlatformZhipu)); len(got) != 1 || got[0] != "glm-4.6" {
		t.Errorf("ListAll(%q) = %v, want [glm-4.6]", PlatformZhipu, got)
	}
	if got := names(s.ListAll(PlatformDeepseek)); len(got) != 1 || got[0] != "deepseek-chat" {
		t.Errorf("ListAll(%q) = %v, want [deepseek-chat]", PlatformDeepseek, got)
	}
	// 别名不能把 openai 的条目也捞进来。
	if got := names(s.ListAll(PlatformOpenAI)); len(got) != 1 || got[0] != "gpt-5.4" {
		t.Errorf("ListAll(%q) = %v, want [gpt-5.4]", PlatformOpenAI, got)
	}
}
