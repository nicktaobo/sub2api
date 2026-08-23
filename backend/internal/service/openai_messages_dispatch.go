package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

const (
	defaultOpenAIMessagesDispatchOpusMappedModel   = "gpt-5.4"
	defaultOpenAIMessagesDispatchSonnetMappedModel = "gpt-5.3-codex"
	defaultOpenAIMessagesDispatchHaikuMappedModel  = "gpt-5.4-mini"
)

func normalizeOpenAIMessagesDispatchMappedModel(model string) string {
	model = NormalizeOpenAICompatRequestedModel(strings.TrimSpace(model))
	return strings.TrimSpace(model)
}

func normalizeOpenAIMessagesDispatchModelConfig(cfg OpenAIMessagesDispatchModelConfig) OpenAIMessagesDispatchModelConfig {
	out := OpenAIMessagesDispatchModelConfig{
		OpusMappedModel:   normalizeOpenAIMessagesDispatchMappedModel(cfg.OpusMappedModel),
		SonnetMappedModel: normalizeOpenAIMessagesDispatchMappedModel(cfg.SonnetMappedModel),
		HaikuMappedModel:  normalizeOpenAIMessagesDispatchMappedModel(cfg.HaikuMappedModel),
	}

	if len(cfg.ExactModelMappings) > 0 {
		out.ExactModelMappings = make(map[string]string, len(cfg.ExactModelMappings))
		for requestedModel, mappedModel := range cfg.ExactModelMappings {
			requestedModel = strings.TrimSpace(requestedModel)
			mappedModel = normalizeOpenAIMessagesDispatchMappedModel(mappedModel)
			if requestedModel == "" || mappedModel == "" {
				continue
			}
			out.ExactModelMappings[requestedModel] = mappedModel
		}
		if len(out.ExactModelMappings) == 0 {
			out.ExactModelMappings = nil
		}
	}

	return out
}

func claudeMessagesDispatchFamily(model string) string {
	normalized := strings.ToLower(strings.TrimSpace(model))
	if !strings.HasPrefix(normalized, "claude") {
		return ""
	}
	switch {
	case strings.Contains(normalized, "opus"):
		return "opus"
	case strings.Contains(normalized, "sonnet"):
		return "sonnet"
	case strings.Contains(normalized, "haiku"):
		return "haiku"
	default:
		return ""
	}
}

func (g *Group) ResolveMessagesDispatchModel(requestedModel string) string {
	if g == nil {
		return ""
	}
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		return ""
	}

	if g.Platform == PlatformGrok {
		if claudeMessagesDispatchFamily(requestedModel) == "" {
			return ""
		}
		opts := xai.RuntimeModelMappingOptions()
		if !opts.EnableCrossClientMap {
			return ""
		}
		return xai.ModelMappingWithOptions(opts)["claude-*"]
	}

	// 与上游的差异（有意保留）：上游在此处对国产供应商无条件 `return ""`，前提是
	// 其 sanitize 会把 CN 分组的调度配置整个置空、模型改写全交给账号级 model_mapping。
	// 本 fork 的 sanitizeGroupMessagesDispatchFields 保留 CN 分组的分组级映射，
	// 本站 CN 分组也依赖这里把 claude-* 翻成各家自己的型号，直接照抄会打掉这条链路。
	// 下面的 defaultMessagesDispatchModels 已按平台给出兜底，不会把 openai 专属的
	// gpt-5.x 发给国产上游。

	cfg := normalizeOpenAIMessagesDispatchModelConfig(g.MessagesDispatchModelConfig)
	if mappedModel := strings.TrimSpace(cfg.ExactModelMappings[requestedModel]); mappedModel != "" {
		return mappedModel
	}

	defaultOpus, defaultSonnet, defaultHaiku := g.defaultMessagesDispatchModels()

	switch claudeMessagesDispatchFamily(requestedModel) {
	case "opus":
		if mappedModel := strings.TrimSpace(cfg.OpusMappedModel); mappedModel != "" {
			return mappedModel
		}
		return defaultOpus
	case "sonnet":
		if mappedModel := strings.TrimSpace(cfg.SonnetMappedModel); mappedModel != "" {
			return mappedModel
		}
		return defaultSonnet
	case "haiku":
		if mappedModel := strings.TrimSpace(cfg.HaikuMappedModel); mappedModel != "" {
			return mappedModel
		}
		return defaultHaiku
	default:
		return ""
	}
}

func (g *Group) defaultMessagesDispatchModels() (opus, sonnet, haiku string) {
	switch g.Platform {
	case PlatformDeepseek:
		return "deepseek-v4-pro", "deepseek-v4-pro", "deepseek-v4-flash"
	case PlatformKimi:
		return "kimi-k2.6", "kimi-k2.6", "kimi-k2.6"
	case PlatformZhipu:
		return "glm-4.6", "glm-4.6", "glm-4.5-air"
	default:
		return defaultOpenAIMessagesDispatchOpusMappedModel,
			defaultOpenAIMessagesDispatchSonnetMappedModel,
			defaultOpenAIMessagesDispatchHaikuMappedModel
	}
}

func sanitizeGroupMessagesDispatchFields(g *Group) {
	// openai 分组的调度配置完整保留；composite 只保留开关（映射交给复合路由解析出的目标平台）。
	if g == nil || g.Platform == PlatformOpenAI {
		return
	}
	if g.Platform != PlatformComposite {
		g.AllowMessagesDispatch = false
	}
	g.DefaultMappedModel = ""
	// 国产供应商分组保留 MessagesDispatchModelConfig：/v1/messages 上的 claude-* 要靠
	// 它翻成 kimi-* / glm-* / deepseek-*（详见 ResolveMessagesDispatchModel 的注释）。
	// AllowMessagesDispatch 跟随上游置 false —— handler 的
	// allowOpenAICompatibleMessagesDispatch 对 CN 分组直接豁免该开关，清不清都放行，
	// 跟随上游可减少下一轮合并冲突。
	if IsCNProvider(g.Platform) {
		return
	}
	g.MessagesDispatchModelConfig = OpenAIMessagesDispatchModelConfig{}
}
