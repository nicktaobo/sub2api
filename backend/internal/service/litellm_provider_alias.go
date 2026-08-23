package service

import "strings"

// litellmProviderAliases 把本站的平台标识映射到 LiteLLM 价目表里的 litellm_provider。
//
// 两套 ID 是各自独立演进的命名空间，不保证同名：
//   - 本站平台标识跟随上游 sub2api（见 AllowedQuotaPlatforms）；
//   - litellm_provider 跟随 LiteLLM 上游价目表，我们只是消费方，改不了。
//
// 目前唯一对不上的是 Kimi：LiteLLM 至今把月之暗面的模型标为 "moonshot"，
// litellm_provider="kimi" 一条都没有。本 fork 把平台标识从 moonshot 改名成 kimi 之后，
// 没有这层别名，ListAll("kimi") 会恒返回空 —— 公开模型广场 /models 与用户定价页的
// 兜底链（账号全透传 → 回落平台全表）拿不到任何模型名，Kimi 分组会显示成「无模型」。
//
// zhipu / deepseek 两侧同名，无需别名；这里只登记真正有分歧的。
// 今后 LiteLLM 若补上 "kimi" provider，这条别名仍然安全：它只放宽匹配，不排除新值。
var litellmProviderAliases = map[string]string{
	PlatformKimi: "moonshot",
}

// litellmProviderMatches 报告价目表条目的 litellm_provider 是否命中给定的平台过滤值。
// 平台标识本身与其 LiteLLM 别名都算命中，均大小写不敏感。
// 别名是单向的：用平台标识过滤会额外命中别名条目，反过来用 LiteLLM 的 provider 名
// （admin 渠道定价同步就是这么传的）不会被放宽。
func litellmProviderMatches(entryProvider, filter string) bool {
	if strings.EqualFold(entryProvider, filter) {
		return true
	}
	if alias, ok := litellmProviderAliases[strings.ToLower(strings.TrimSpace(filter))]; ok {
		return strings.EqualFold(entryProvider, alias)
	}
	return false
}
