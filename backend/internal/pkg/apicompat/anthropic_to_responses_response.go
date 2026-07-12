package apicompat

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Non-streaming: AnthropicResponse → ResponsesResponse
// ---------------------------------------------------------------------------

// AnthropicResponsesToolContext 汇总 /v1/responses → Anthropic 回退路径回程还原
// codex 0.14x 工具调用所需的请求侧上下文（fork 定制，对齐 chat 桥的
// CustomTools / ToolSearchDeclared / NamespaceTools 语义）：custom 工具的调用还原
// 为 custom_tool_call 项、tool_search 代理调用还原为 tool_search_call 项、namespace
// 子工具的摊平名调用还原为带 namespace 字段的 function_call 项。codex 只按这些项
// 类型路由，一律还原为 function_call 会报 unsupported call。
type AnthropicResponsesToolContext struct {
	CustomTools    map[string]bool
	ToolSearch     bool
	NamespaceTools map[string]NamespacedToolName
}

// NewAnthropicResponsesToolContext 从 Responses 请求的工具声明构造回程还原上下文，
// 复用 chat 桥的收集口径。没有任何需要还原的工具时返回 nil（nil 上下文按纯
// function_call 还原，与历史行为一致）。
func NewAnthropicResponsesToolContext(tools []ResponsesTool) *AnthropicResponsesToolContext {
	if len(tools) == 0 {
		return nil
	}
	ctx := &AnthropicResponsesToolContext{
		CustomTools:    CustomToolNames(tools),
		ToolSearch:     HasToolSearchTool(tools),
		NamespaceTools: NamespaceToolNames(tools),
	}
	if len(ctx.CustomTools) == 0 && !ctx.ToolSearch && len(ctx.NamespaceTools) == 0 {
		return nil
	}
	return ctx
}

// classifyToolUse 按请求声明判定 tool_use 应还原成的 Responses 项类型，返回
// (项类型, 还原后的工具名, namespace)。nil 上下文安全，等价于纯 function_call。
// 判定顺序与 chat 桥 announceChatToolItem 一致（请求方向的撞名拒绝保证无歧义）。
func (ctx *AnthropicResponsesToolContext) classifyToolUse(name string) (itemType, itemName, namespace string) {
	if ctx == nil {
		return "function_call", name, ""
	}
	if ctx.CustomTools[name] {
		return "custom_tool_call", name, ""
	}
	if ctx.ToolSearch && name == toolSearchProxyName {
		return "tool_search_call", name, ""
	}
	if ns, ok := ctx.NamespaceTools[name]; ok {
		return "function_call", ns.Name, ns.Namespace
	}
	return "function_call", name, ""
}

// AnthropicToResponsesResponse converts an Anthropic Messages response into a
// Responses API response. This is the reverse of ResponsesToAnthropic and
// enables Anthropic upstream responses to be returned in OpenAI Responses format.
func AnthropicToResponsesResponse(resp *AnthropicResponse) *ResponsesResponse {
	return AnthropicToResponsesResponseWithTools(resp, nil)
}

// AnthropicToResponsesResponseWithTools 同 AnthropicToResponsesResponse，但按
// 请求侧工具上下文还原 codex 工具调用形态（fork 定制，见
// AnthropicResponsesToolContext）。toolCtx 为 nil 时行为与原函数一致。
func AnthropicToResponsesResponseWithTools(resp *AnthropicResponse, toolCtx *AnthropicResponsesToolContext) *ResponsesResponse {
	id := resp.ID
	if id == "" {
		id = generateResponsesID()
	}

	out := &ResponsesResponse{
		ID:     id,
		Object: "response",
		Model:  resp.Model,
	}

	var outputs []ResponsesOutput
	var msgParts []ResponsesContentPart

	for _, block := range resp.Content {
		switch block.Type {
		case "thinking":
			if block.Thinking != "" {
				outputs = append(outputs, ResponsesOutput{
					Type: "reasoning",
					ID:   generateItemID(),
					Summary: []ResponsesSummary{{
						Type: "summary_text",
						Text: block.Thinking,
					}},
				})
			}
		case "text":
			if block.Text != "" {
				msgParts = append(msgParts, ResponsesContentPart{
					Type: "output_text",
					Text: block.Text,
				})
			}
		case "tool_use":
			args := "{}"
			if len(block.Input) > 0 {
				args = string(block.Input)
			}
			outputs = append(outputs, responsesOutputForAnthropicToolUse(block, args, toolCtx))
		}
	}

	// Assemble message output item from text parts
	if len(msgParts) > 0 {
		outputs = append(outputs, ResponsesOutput{
			Type:    "message",
			ID:      generateItemID(),
			Role:    "assistant",
			Content: msgParts,
			Status:  "completed",
		})
	}

	if len(outputs) == 0 {
		outputs = append(outputs, ResponsesOutput{
			Type:    "message",
			ID:      generateItemID(),
			Role:    "assistant",
			Content: []ResponsesContentPart{{Type: "output_text", Text: ""}},
			Status:  "completed",
		})
	}
	out.Output = outputs

	// Map stop_reason → status
	out.Status = anthropicStopReasonToResponsesStatus(resp.StopReason, resp.Content)
	if out.Status == "incomplete" {
		out.IncompleteDetails = &ResponsesIncompleteDetails{Reason: "max_output_tokens"}
	}

	// Usage
	// Anthropic's input_tokens excludes cache_read/cache_creation, while OpenAI
	// Responses' input_tokens is the total including cached tokens. Add them back
	// when converting so downstream consumers see OpenAI semantics.
	totalInputTokens := resp.Usage.InputTokens +
		resp.Usage.CacheReadInputTokens +
		resp.Usage.CacheCreationInputTokens
	out.Usage = &ResponsesUsage{
		InputTokens:              totalInputTokens,
		OutputTokens:             resp.Usage.OutputTokens,
		TotalTokens:              totalInputTokens + resp.Usage.OutputTokens,
		CacheCreationInputTokens: resp.Usage.CacheCreationInputTokens,
	}
	if resp.Usage.CacheReadInputTokens > 0 {
		out.Usage.InputTokensDetails = &ResponsesInputTokensDetails{
			CachedTokens: resp.Usage.CacheReadInputTokens,
		}
	}

	return out
}

// responsesOutputForAnthropicToolUse 把一个 tool_use 块按请求侧工具上下文还原为
// Responses 输出项（fork 定制，见 AnthropicResponsesToolContext）。
func responsesOutputForAnthropicToolUse(block AnthropicContentBlock, args string, toolCtx *AnthropicResponsesToolContext) ResponsesOutput {
	itemType, name, namespace := toolCtx.classifyToolUse(block.Name)
	out := ResponsesOutput{
		Type:   itemType,
		ID:     generateItemID(),
		CallID: toResponsesCallID(block.ID),
		Status: "completed",
	}
	switch itemType {
	case "custom_tool_call":
		// custom 调用的 arguments 是降级 schema 包裹的 {"input": ...}，还原为
		// 自由文本输入（见 extractCustomToolCallInput）。
		out.Name = name
		out.Input = extractCustomToolCallInput(args)
	case "tool_search_call":
		// codex 只在项类型为 tool_search_call 时执行 tool search，arguments 原样回传。
		out.Arguments = args
	default:
		out.Name = name
		out.Namespace = namespace
		out.Arguments = args
	}
	return out
}

// anthropicStopReasonToResponsesStatus maps Anthropic stop_reason to Responses status.
func anthropicStopReasonToResponsesStatus(stopReason string, blocks []AnthropicContentBlock) string {
	switch stopReason {
	case "max_tokens":
		return "incomplete"
	case "end_turn", "tool_use", "stop_sequence":
		return "completed"
	default:
		return "completed"
	}
}

// ---------------------------------------------------------------------------
// Streaming: AnthropicStreamEvent → []ResponsesStreamEvent (stateful converter)
// ---------------------------------------------------------------------------

// AnthropicEventToResponsesState tracks state for converting a sequence of
// Anthropic SSE events into Responses SSE events.
type AnthropicEventToResponsesState struct {
	ResponseID     string
	Model          string
	Created        int64
	SequenceNumber int

	// CreatedSent tracks whether response.created has been emitted.
	CreatedSent bool
	// CompletedSent tracks whether the terminal event has been emitted.
	CompletedSent bool

	// Current output tracking
	OutputIndex     int
	CurrentItemID   string
	CurrentItemType string // "message" | "function_call" | "custom_tool_call" | "tool_search_call" | "reasoning"

	// For message output: accumulate text parts
	ContentIndex int

	// For function_call: track per-output info
	CurrentCallID string
	CurrentName   string
	// fork 定制：namespace 子工具调用还原出的归属命名空间（见 ToolContext）。
	CurrentNamespace string
	// fork 定制：累积当前工具调用的 input_json_delta。custom 调用的 input 与
	// tool_search 调用的 arguments 无法增量还原，收尾时一次性下发；function_call
	// 的 output_item.done 也需带全量 arguments 供 codex 物化调用。
	CurrentArgs strings.Builder

	// ToolContext 是请求侧工具上下文（fork 定制，见 AnthropicResponsesToolContext）。
	// nil 时所有 tool_use 按 function_call 还原，与历史行为一致。
	ToolContext *AnthropicResponsesToolContext

	// Usage from message_start / message_delta. InputTokens here follows
	// Anthropic semantics (excludes cached tokens); they are added back when
	// emitting the OpenAI Responses usage.
	InputTokens              int
	OutputTokens             int
	CacheReadInputTokens     int
	CacheCreationInputTokens int
}

// NewAnthropicEventToResponsesState returns an initialised stream state.
func NewAnthropicEventToResponsesState() *AnthropicEventToResponsesState {
	return &AnthropicEventToResponsesState{
		Created: time.Now().Unix(),
	}
}

// AnthropicEventToResponsesEvents converts a single Anthropic SSE event into
// zero or more Responses SSE events, updating state as it goes.
func AnthropicEventToResponsesEvents(
	evt *AnthropicStreamEvent,
	state *AnthropicEventToResponsesState,
) []ResponsesStreamEvent {
	switch evt.Type {
	case "message_start":
		return anthToResHandleMessageStart(evt, state)
	case "content_block_start":
		return anthToResHandleContentBlockStart(evt, state)
	case "content_block_delta":
		return anthToResHandleContentBlockDelta(evt, state)
	case "content_block_stop":
		return anthToResHandleContentBlockStop(evt, state)
	case "message_delta":
		return anthToResHandleMessageDelta(evt, state)
	case "message_stop":
		return anthToResHandleMessageStop(state)
	default:
		return nil
	}
}

// FinalizeAnthropicResponsesStream emits synthetic termination events if the
// stream ended without a proper message_stop.
func FinalizeAnthropicResponsesStream(state *AnthropicEventToResponsesState) []ResponsesStreamEvent {
	if !state.CreatedSent || state.CompletedSent {
		return nil
	}

	var events []ResponsesStreamEvent

	// Close any open item
	events = append(events, closeCurrentResponsesItem(state)...)

	// Emit response.completed
	events = append(events, makeResponsesCompletedEvent(state, "completed", nil))
	state.CompletedSent = true
	return events
}

// ResponsesEventToSSE formats a ResponsesStreamEvent as an SSE data line.
func ResponsesEventToSSE(evt ResponsesStreamEvent) (string, error) {
	data, err := json.Marshal(evt)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("event: %s\ndata: %s\n\n", evt.Type, data), nil
}

// --- internal handlers ---

func anthToResHandleMessageStart(evt *AnthropicStreamEvent, state *AnthropicEventToResponsesState) []ResponsesStreamEvent {
	if evt.Message != nil {
		state.ResponseID = evt.Message.ID
		if state.Model == "" {
			state.Model = evt.Message.Model
		}
		if evt.Message.Usage.InputTokens > 0 {
			state.InputTokens = evt.Message.Usage.InputTokens
		}
		if evt.Message.Usage.CacheReadInputTokens > 0 {
			state.CacheReadInputTokens = evt.Message.Usage.CacheReadInputTokens
		}
		if evt.Message.Usage.CacheCreationInputTokens > 0 {
			state.CacheCreationInputTokens = evt.Message.Usage.CacheCreationInputTokens
		}
	}

	if state.CreatedSent {
		return nil
	}
	state.CreatedSent = true

	// Emit response.created
	return []ResponsesStreamEvent{makeResponsesCreatedEvent(state)}
}

func anthToResHandleContentBlockStart(evt *AnthropicStreamEvent, state *AnthropicEventToResponsesState) []ResponsesStreamEvent {
	if evt.ContentBlock == nil {
		return nil
	}

	var events []ResponsesStreamEvent

	switch evt.ContentBlock.Type {
	case "thinking":
		state.CurrentItemID = generateItemID()
		state.CurrentItemType = "reasoning"
		state.ContentIndex = 0

		events = append(events, makeResponsesEvent(state, "response.output_item.added", &ResponsesStreamEvent{
			OutputIndex: state.OutputIndex,
			Item: &ResponsesOutput{
				Type: "reasoning",
				ID:   state.CurrentItemID,
			},
		}))

	case "text":
		// If we don't have an open message item, open one
		if state.CurrentItemType != "message" {
			state.CurrentItemID = generateItemID()
			state.CurrentItemType = "message"
			state.ContentIndex = 0

			events = append(events, makeResponsesEvent(state, "response.output_item.added", &ResponsesStreamEvent{
				OutputIndex: state.OutputIndex,
				Item: &ResponsesOutput{
					Type:   "message",
					ID:     state.CurrentItemID,
					Role:   "assistant",
					Status: "in_progress",
				},
			}))
		}

	case "tool_use":
		// Close previous item if any
		events = append(events, closeCurrentResponsesItem(state)...)

		// fork 定制：按请求声明的工具类型还原 codex 工具调用形态（对齐 chat 桥
		// announceChatToolItem，见 AnthropicResponsesToolContext）。
		itemType, name, namespace := state.ToolContext.classifyToolUse(evt.ContentBlock.Name)
		state.CurrentItemID = generateItemID()
		state.CurrentItemType = itemType
		state.CurrentCallID = toResponsesCallID(evt.ContentBlock.ID)
		state.CurrentName = name
		state.CurrentNamespace = namespace
		state.CurrentArgs.Reset()

		events = append(events, makeResponsesEvent(state, "response.output_item.added", &ResponsesStreamEvent{
			OutputIndex: state.OutputIndex,
			Item: &ResponsesOutput{
				Type:      itemType,
				ID:        state.CurrentItemID,
				CallID:    state.CurrentCallID,
				Name:      state.CurrentName,
				Namespace: state.CurrentNamespace,
				Status:    "in_progress",
			},
		}))
	}

	return events
}

func anthToResHandleContentBlockDelta(evt *AnthropicStreamEvent, state *AnthropicEventToResponsesState) []ResponsesStreamEvent {
	if evt.Delta == nil {
		return nil
	}

	switch evt.Delta.Type {
	case "text_delta":
		if evt.Delta.Text == "" {
			return nil
		}
		return []ResponsesStreamEvent{makeResponsesEvent(state, "response.output_text.delta", &ResponsesStreamEvent{
			OutputIndex:  state.OutputIndex,
			ContentIndex: state.ContentIndex,
			Delta:        evt.Delta.Text,
			ItemID:       state.CurrentItemID,
		})}

	case "thinking_delta":
		if evt.Delta.Thinking == "" {
			return nil
		}
		return []ResponsesStreamEvent{makeResponsesEvent(state, "response.reasoning_summary_text.delta", &ResponsesStreamEvent{
			OutputIndex:  state.OutputIndex,
			SummaryIndex: 0,
			Delta:        evt.Delta.Thinking,
			ItemID:       state.CurrentItemID,
		})}

	case "input_json_delta":
		if evt.Delta.PartialJSON == "" {
			return nil
		}
		// fork 定制：累积全量参数（收尾项需要）；custom 调用的 input 与
		// tool_search 的 arguments 无法增量还原，流中不产出增量事件，收尾时
		// 一次性下发（见 anthToResHandleContentBlockStop）。
		_, _ = state.CurrentArgs.WriteString(evt.Delta.PartialJSON)
		if state.CurrentItemType == "custom_tool_call" || state.CurrentItemType == "tool_search_call" {
			return nil
		}
		return []ResponsesStreamEvent{makeResponsesEvent(state, "response.function_call_arguments.delta", &ResponsesStreamEvent{
			OutputIndex: state.OutputIndex,
			Delta:       evt.Delta.PartialJSON,
			ItemID:      state.CurrentItemID,
			CallID:      state.CurrentCallID,
			Name:        state.CurrentName,
		})}

	case "signature_delta":
		// Anthropic signature deltas have no Responses equivalent; skip
		return nil
	}

	return nil
}

func anthToResHandleContentBlockStop(evt *AnthropicStreamEvent, state *AnthropicEventToResponsesState) []ResponsesStreamEvent {
	switch state.CurrentItemType {
	case "reasoning":
		// Emit reasoning summary done + output item done
		events := []ResponsesStreamEvent{
			makeResponsesEvent(state, "response.reasoning_summary_text.done", &ResponsesStreamEvent{
				OutputIndex:  state.OutputIndex,
				SummaryIndex: 0,
				ItemID:       state.CurrentItemID,
			}),
		}
		events = append(events, closeCurrentResponsesItem(state)...)
		return events

	case "function_call":
		// Emit function_call_arguments.done + output item done
		events := []ResponsesStreamEvent{
			makeResponsesEvent(state, "response.function_call_arguments.done", &ResponsesStreamEvent{
				OutputIndex: state.OutputIndex,
				ItemID:      state.CurrentItemID,
				CallID:      state.CurrentCallID,
				Name:        state.CurrentName,
				Arguments:   currentResponsesToolArguments(state),
			}),
		}
		events = append(events, closeCurrentResponsesItem(state)...)
		return events

	case "custom_tool_call":
		// fork 定制：custom 调用按 custom_tool_call 生命周期收尾，input 在此处
		// 一次性下发（流中不产出增量，对齐 chat 桥 closeChatToolItems）。
		input := extractCustomToolCallInput(currentResponsesToolArguments(state))
		var events []ResponsesStreamEvent
		if input != "" {
			events = append(events, makeResponsesEvent(state, "response.custom_tool_call_input.delta", &ResponsesStreamEvent{
				OutputIndex: state.OutputIndex,
				ItemID:      state.CurrentItemID,
				Delta:       input,
			}))
		}
		events = append(events, makeResponsesEvent(state, "response.custom_tool_call_input.done", &ResponsesStreamEvent{
			OutputIndex: state.OutputIndex,
			ItemID:      state.CurrentItemID,
			CallID:      state.CurrentCallID,
			Name:        state.CurrentName,
			Input:       input,
		}))
		events = append(events, closeCurrentResponsesItem(state)...)
		return events

	case "tool_search_call":
		// fork 定制：tool_search 调用按 tool_search_call 项收尾，codex 从
		// output_item.done 物化该调用（无参数增量事件，对齐 chat 桥）。
		return closeCurrentResponsesItem(state)

	case "message":
		// Emit output_text.done (text block is done, but message item stays open for potential more blocks)
		return []ResponsesStreamEvent{
			makeResponsesEvent(state, "response.output_text.done", &ResponsesStreamEvent{
				OutputIndex:  state.OutputIndex,
				ContentIndex: state.ContentIndex,
				ItemID:       state.CurrentItemID,
			}),
		}
	}

	return nil
}

func anthToResHandleMessageDelta(evt *AnthropicStreamEvent, state *AnthropicEventToResponsesState) []ResponsesStreamEvent {
	// Update usage
	if evt.Usage != nil {
		state.OutputTokens = evt.Usage.OutputTokens
		if evt.Usage.InputTokens > 0 {
			state.InputTokens = evt.Usage.InputTokens
		}
		if evt.Usage.CacheReadInputTokens > 0 {
			state.CacheReadInputTokens = evt.Usage.CacheReadInputTokens
		}
		if evt.Usage.CacheCreationInputTokens > 0 {
			state.CacheCreationInputTokens = evt.Usage.CacheCreationInputTokens
		}
	}

	return nil
}

func anthToResHandleMessageStop(state *AnthropicEventToResponsesState) []ResponsesStreamEvent {
	if state.CompletedSent {
		return nil
	}

	var events []ResponsesStreamEvent

	// Close any open item
	events = append(events, closeCurrentResponsesItem(state)...)

	// Determine status
	status := "completed"
	var incompleteDetails *ResponsesIncompleteDetails

	// Emit response.completed
	events = append(events, makeResponsesCompletedEvent(state, status, incompleteDetails))
	state.CompletedSent = true
	return events
}

// --- helper functions ---

func closeCurrentResponsesItem(state *AnthropicEventToResponsesState) []ResponsesStreamEvent {
	if state.CurrentItemType == "" {
		return nil
	}

	item := &ResponsesOutput{
		Type:   state.CurrentItemType,
		ID:     state.CurrentItemID,
		Status: "completed",
	}
	// fork 定制：工具调用项的 output_item.done 带完整 call 载荷，codex 从该事件
	// 物化调用（custom/tool_search 流中不产出增量，全量只在这里下发）。
	switch state.CurrentItemType {
	case "function_call":
		item.CallID = state.CurrentCallID
		item.Name = state.CurrentName
		item.Namespace = state.CurrentNamespace
		item.Arguments = currentResponsesToolArguments(state)
	case "custom_tool_call":
		item.CallID = state.CurrentCallID
		item.Name = state.CurrentName
		item.Input = extractCustomToolCallInput(currentResponsesToolArguments(state))
	case "tool_search_call":
		item.CallID = state.CurrentCallID
		item.Arguments = currentResponsesToolArguments(state)
	}

	// Reset
	state.CurrentItemType = ""
	state.CurrentItemID = ""
	state.CurrentCallID = ""
	state.CurrentName = ""
	state.CurrentNamespace = ""
	state.CurrentArgs.Reset()
	state.OutputIndex++
	state.ContentIndex = 0

	return []ResponsesStreamEvent{makeResponsesEvent(state, "response.output_item.done", &ResponsesStreamEvent{
		OutputIndex: state.OutputIndex - 1, // Use the index before increment
		Item:        item,
	})}
}

// currentResponsesToolArguments 返回当前工具调用累积的全量 arguments，空时兜底 "{}"。
func currentResponsesToolArguments(state *AnthropicEventToResponsesState) string {
	args := strings.TrimSpace(state.CurrentArgs.String())
	if args == "" {
		return "{}"
	}
	return args
}

func makeResponsesCreatedEvent(state *AnthropicEventToResponsesState) ResponsesStreamEvent {
	seq := state.SequenceNumber
	state.SequenceNumber++
	return ResponsesStreamEvent{
		Type:           "response.created",
		SequenceNumber: seq,
		Response: &ResponsesResponse{
			ID:     state.ResponseID,
			Object: "response",
			Model:  state.Model,
			Status: "in_progress",
			Output: []ResponsesOutput{},
		},
	}
}

func makeResponsesCompletedEvent(
	state *AnthropicEventToResponsesState,
	status string,
	incompleteDetails *ResponsesIncompleteDetails,
) ResponsesStreamEvent {
	seq := state.SequenceNumber
	state.SequenceNumber++

	// Anthropic's input_tokens excludes cache_read/cache_creation; add them
	// back to match OpenAI Responses semantics where input_tokens is the total.
	totalInputTokens := state.InputTokens + state.CacheReadInputTokens + state.CacheCreationInputTokens
	usage := &ResponsesUsage{
		InputTokens:              totalInputTokens,
		OutputTokens:             state.OutputTokens,
		TotalTokens:              totalInputTokens + state.OutputTokens,
		CacheCreationInputTokens: state.CacheCreationInputTokens,
	}
	if state.CacheReadInputTokens > 0 {
		usage.InputTokensDetails = &ResponsesInputTokensDetails{
			CachedTokens: state.CacheReadInputTokens,
		}
	}

	return ResponsesStreamEvent{
		Type:           "response.completed",
		SequenceNumber: seq,
		Response: &ResponsesResponse{
			ID:                state.ResponseID,
			Object:            "response",
			Model:             state.Model,
			Status:            status,
			Output:            []ResponsesOutput{}, // Simplified; full output tracking would add complexity
			Usage:             usage,
			IncompleteDetails: incompleteDetails,
		},
	}
}

func makeResponsesEvent(state *AnthropicEventToResponsesState, eventType string, template *ResponsesStreamEvent) ResponsesStreamEvent {
	seq := state.SequenceNumber
	state.SequenceNumber++

	evt := *template
	evt.Type = eventType
	evt.SequenceNumber = seq
	return evt
}

func generateResponsesID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return "resp_" + hex.EncodeToString(b)
}

func generateItemID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return "item_" + hex.EncodeToString(b)
}
