package claudeagent

// ChatRole represents a message role in the chat completions format.
type ChatRole string

const (
	ChatRoleSystem    ChatRole = "system"
	ChatRoleUser      ChatRole = "user"
	ChatRoleAssistant ChatRole = "assistant"
	ChatRoleTool      ChatRole = "tool"
)

// ChatMessage is a provider-agnostic conversation message used by APIAgent.
// The agentic loop maintains a []ChatMessage history; providers convert to their native format.
//
// Note: ConversationMessage (agent.go) serves the same purpose for the CLI-based Agent.
// Both types are intentionally separate — the CLI agent uses string Role; ChatMessage uses typed ChatRole.
type ChatMessage struct {
	// Role of the message sender.
	Role ChatRole
	// Content is the text content of the message.
	Content string
	// ToolCalls are tool invocations requested by the assistant.
	// Only set when Role is ChatRoleAssistant.
	ToolCalls []ToolCall
	// ToolCallID links a tool result back to the tool call that produced it.
	// Only set when Role is ChatRoleTool.
	ToolCallID string
	// IsError indicates the tool result is an error.
	// Only set when Role is ChatRoleTool.
	IsError bool
}

// ChatRequest is a provider-agnostic request to an LLM.
type ChatRequest struct {
	// Model identifier (e.g., "claude-sonnet-4-20250514", "gpt-4o", "mistral-large-latest").
	Model string
	// Messages is the conversation history.
	Messages []ChatMessage
	// Tools available for the model to call.
	Tools []ToolDefinition
	// SystemPrompt is a plain text system prompt.
	SystemPrompt string
	// SystemBlocks provides structured system prompt blocks with cache control.
	// Anthropic-specific; other providers concatenate block text into a single system message.
	SystemBlocks []SystemPromptBlock
	// MaxTokens is the maximum number of tokens to generate.
	MaxTokens int
	// Temperature controls randomness. Nil uses the provider's default.
	Temperature *float64
}

// ChatResponse is a provider-agnostic response from an LLM.
type ChatResponse struct {
	// Content is the text content of the assistant's response.
	Content string
	// ToolCalls are tool invocations requested by the assistant.
	ToolCalls []ToolCall
	// StopReason indicates why the model stopped generating.
	// Normalized to: "end_turn", "tool_use", "max_tokens".
	StopReason string
	// Usage contains token consumption metrics.
	Usage ChatUsage
}

// ChatUsage contains token usage information from an LLM response.
type ChatUsage struct {
	InputTokens  int
	OutputTokens int
	// CacheCreationInputTokens is Anthropic-specific; 0 for other providers.
	CacheCreationInputTokens int
	// CacheReadInputTokens is Anthropic-specific; 0 for other providers.
	CacheReadInputTokens int
}

// ChatStreamEventType identifies the kind of streaming event from a provider.
type ChatStreamEventType string

const (
	// ChatStreamContentDelta is a text chunk from the assistant.
	ChatStreamContentDelta ChatStreamEventType = "content_delta"
	// ChatStreamToolUseStart indicates the model is starting a tool call.
	ChatStreamToolUseStart ChatStreamEventType = "tool_use_start"
	// ChatStreamToolUseDelta is a partial JSON chunk of tool input.
	ChatStreamToolUseDelta ChatStreamEventType = "tool_use_delta"
	// ChatStreamToolUseEnd indicates the tool call input is complete.
	ChatStreamToolUseEnd ChatStreamEventType = "tool_use_end"
)

// ChatStreamEvent carries a streaming delta from a provider.
type ChatStreamEvent struct {
	// Type identifies the event kind.
	Type ChatStreamEventType
	// Content carries text for ContentDelta and ToolUseDelta events.
	Content string
	// ToolCall carries tool information for ToolUseStart and ToolUseEnd events.
	// On ToolUseStart: ID and Name are set; Input is nil (not yet accumulated).
	// On ToolUseEnd: ID, Name, and fully-accumulated Input are all set.
	ToolCall *ToolCall
}

// ChatStreamCallback receives streaming events during LLM completion.
// Providers call this as deltas arrive. May be nil to skip streaming.
type ChatStreamCallback func(event ChatStreamEvent)
