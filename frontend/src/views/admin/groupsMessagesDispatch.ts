import type { GroupPlatform, OpenAIMessagesDispatchModelConfig } from "@/types";

export interface MessagesDispatchMappingRow {
  claude_model: string;
  target_model: string;
}

export interface MessagesDispatchFormState {
  allow_messages_dispatch: boolean;
  opus_mapped_model: string;
  sonnet_mapped_model: string;
  haiku_mapped_model: string;
  exact_model_mappings: MessagesDispatchMappingRow[];
}

function getDefaultModelsForPlatform(platform?: GroupPlatform | null): {
  opus: string;
  sonnet: string;
  haiku: string;
} {
  switch (platform) {
    case "deepseek":
      return { opus: "deepseek-v4-pro", sonnet: "deepseek-v4-pro", haiku: "deepseek-v4-flash" };
    case "moonshot":
      return { opus: "kimi-k2.6", sonnet: "kimi-k2.6", haiku: "kimi-k2.6" };
    case "glm":
      return { opus: "glm-4.6", sonnet: "glm-4.6", haiku: "glm-4.5-air" };
    case "qwen":
      return { opus: "qwen3-coder-plus", sonnet: "qwen3-coder-plus", haiku: "qwen-plus" };
    default:
      return { opus: "gpt-5.4", sonnet: "gpt-5.3-codex", haiku: "gpt-5.4-mini" };
  }
}

// 支持 Anthropic Messages API 调度（/v1/messages 派发）的平台白名单。
// 必须与后端 service.sanitizeGroupMessagesDispatchFields / defaultMessagesDispatchModels
// 的平台名单一致（注意：openAICompatPlatforms 另含 seedance，不在派发白名单内）。
const MESSAGES_DISPATCH_PLATFORMS: GroupPlatform[] = ["openai", "deepseek", "moonshot", "glm", "qwen"];

export function groupSupportsMessagesDispatch(platform?: GroupPlatform | null): boolean {
  return !!platform && MESSAGES_DISPATCH_PLATFORMS.includes(platform);
}

export function createDefaultMessagesDispatchFormState(
  platform?: GroupPlatform | null,
): MessagesDispatchFormState {
  const models = getDefaultModelsForPlatform(platform);
  return {
    allow_messages_dispatch: false,
    opus_mapped_model: models.opus,
    sonnet_mapped_model: models.sonnet,
    haiku_mapped_model: models.haiku,
    exact_model_mappings: [],
  };
}

export function messagesDispatchConfigToFormState(
  config?: OpenAIMessagesDispatchModelConfig | null,
  platform?: GroupPlatform | null,
): MessagesDispatchFormState {
  const defaults = createDefaultMessagesDispatchFormState(platform);
  const exactMappings = Object.entries(config?.exact_model_mappings || {})
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([claude_model, target_model]) => ({ claude_model, target_model }));

  return {
    allow_messages_dispatch: false,
    opus_mapped_model:
      config?.opus_mapped_model?.trim() || defaults.opus_mapped_model,
    sonnet_mapped_model:
      config?.sonnet_mapped_model?.trim() || defaults.sonnet_mapped_model,
    haiku_mapped_model:
      config?.haiku_mapped_model?.trim() || defaults.haiku_mapped_model,
    exact_model_mappings: exactMappings,
  };
}

export function messagesDispatchFormStateToConfig(
  state: MessagesDispatchFormState,
): OpenAIMessagesDispatchModelConfig {
  const exactModelMappings = Object.fromEntries(
    state.exact_model_mappings
      .map((row) => [row.claude_model.trim(), row.target_model.trim()] as const)
      .filter(([claudeModel, targetModel]) => claudeModel && targetModel),
  );

  return {
    opus_mapped_model: state.opus_mapped_model.trim(),
    sonnet_mapped_model: state.sonnet_mapped_model.trim(),
    haiku_mapped_model: state.haiku_mapped_model.trim(),
    exact_model_mappings: exactModelMappings,
  };
}

export function resetMessagesDispatchFormState(
  target: MessagesDispatchFormState,
  platform?: GroupPlatform | null,
): void {
  const defaults = createDefaultMessagesDispatchFormState(platform);
  target.allow_messages_dispatch = defaults.allow_messages_dispatch;
  target.opus_mapped_model = defaults.opus_mapped_model;
  target.sonnet_mapped_model = defaults.sonnet_mapped_model;
  target.haiku_mapped_model = defaults.haiku_mapped_model;
  target.exact_model_mappings = [];
}
