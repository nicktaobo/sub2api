package service

import "strings"

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
	case PlatformDeepSeek:
		return "deepseek-v4-pro", "deepseek-v4-pro", "deepseek-v4-flash"
	case PlatformMoonshot:
		return "kimi-k2.6", "kimi-k2.6", "kimi-k2.6"
	case PlatformGLM:
		return "glm-4.6", "glm-4.6", "glm-4.5-air"
	case PlatformQwen:
		return "qwen3-coder-plus", "qwen3-coder-plus", "qwen-plus"
	default:
		return defaultOpenAIMessagesDispatchOpusMappedModel,
			defaultOpenAIMessagesDispatchSonnetMappedModel,
			defaultOpenAIMessagesDispatchHaikuMappedModel
	}
}

func sanitizeGroupMessagesDispatchFields(g *Group) {
	if g == nil {
		return
	}
	switch g.Platform {
	case PlatformOpenAI, PlatformDeepSeek, PlatformMoonshot, PlatformGLM, PlatformQwen:
		return
	default:
		g.AllowMessagesDispatch = false
		g.DefaultMappedModel = ""
		g.MessagesDispatchModelConfig = OpenAIMessagesDispatchModelConfig{}
	}
}
