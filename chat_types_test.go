package claudeagent

import (
	"context"
	"encoding/json"
	"testing"
)

func TestChatMessage_Roles(t *testing.T) {
	msgs := []ChatMessage{
		{Role: ChatRoleSystem, Content: "You are helpful."},
		{Role: ChatRoleUser, Content: "Hello"},
		{Role: ChatRoleAssistant, Content: "Hi!", ToolCalls: []ToolCall{
			{ID: "tc_1", Name: "search", Input: json.RawMessage(`{"q":"test"}`)},
		}},
		{Role: ChatRoleTool, Content: "result", ToolCallID: "tc_1"},
	}

	if msgs[0].Role != ChatRoleSystem {
		t.Errorf("expected system role, got %s", msgs[0].Role)
	}
	if msgs[2].Role != ChatRoleAssistant {
		t.Errorf("expected assistant role, got %s", msgs[2].Role)
	}
	if len(msgs[2].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(msgs[2].ToolCalls))
	}
	if msgs[2].ToolCalls[0].Name != "search" {
		t.Errorf("expected tool name 'search', got %s", msgs[2].ToolCalls[0].Name)
	}
	if msgs[3].ToolCallID != "tc_1" {
		t.Errorf("expected tool call ID 'tc_1', got %s", msgs[3].ToolCallID)
	}
}

func TestChatRequest_Construction(t *testing.T) {
	temp := 0.7
	req := ChatRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []ChatMessage{
			{Role: ChatRoleUser, Content: "Hello"},
		},
		Tools: []ToolDefinition{
			{Name: "search", Description: "Search the web"},
		},
		SystemPrompt: "Be helpful.",
		MaxTokens:    4096,
		Temperature:  &temp,
	}

	if req.Model != "claude-sonnet-4-20250514" {
		t.Errorf("unexpected model: %s", req.Model)
	}
	if len(req.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(req.Messages))
	}
	if len(req.Tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(req.Tools))
	}
	if req.Temperature == nil || *req.Temperature != 0.7 {
		t.Errorf("expected temperature 0.7")
	}
}

func TestChatResponse_ToolCallsAndUsage(t *testing.T) {
	resp := ChatResponse{
		Content: "Let me search.",
		ToolCalls: []ToolCall{
			{ID: "tc_1", Name: "search", Input: json.RawMessage(`{"q":"cats"}`)},
		},
		StopReason: "tool_use",
		Usage: ChatUsage{
			InputTokens:              100,
			OutputTokens:             20,
			CacheCreationInputTokens: 50, // Anthropic-specific
			CacheReadInputTokens:     10, // Anthropic-specific
		},
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.StopReason != "tool_use" {
		t.Errorf("expected 'tool_use', got %s", resp.StopReason)
	}
	if resp.Usage.CacheCreationInputTokens != 50 {
		t.Errorf("expected 50 cache creation tokens, got %d", resp.Usage.CacheCreationInputTokens)
	}
}

func TestChatUsage_NonAnthropicZerosCacheTokens(t *testing.T) {
	// Non-Anthropic providers should return zero for cache-specific fields.
	usage := ChatUsage{InputTokens: 100, OutputTokens: 50}
	if usage.CacheCreationInputTokens != 0 {
		t.Error("non-Anthropic provider should have 0 cache creation tokens")
	}
	if usage.CacheReadInputTokens != 0 {
		t.Error("non-Anthropic provider should have 0 cache read tokens")
	}
}

func TestChatStreamEvent_Types(t *testing.T) {
	events := []ChatStreamEvent{
		{Type: ChatStreamContentDelta, Content: "Hello"},
		{Type: ChatStreamToolUseStart, ToolCall: &ToolCall{ID: "tc_1", Name: "search"}},
		{Type: ChatStreamToolUseDelta, Content: `{"q":"test`},
		{Type: ChatStreamToolUseEnd, ToolCall: &ToolCall{ID: "tc_1", Name: "search"}},
	}

	if events[0].Type != ChatStreamContentDelta {
		t.Errorf("expected content_delta, got %s", events[0].Type)
	}
	if events[1].ToolCall.Name != "search" {
		t.Errorf("expected search tool, got %s", events[1].ToolCall.Name)
	}
}

func TestLLMProvider_MockImplementation(t *testing.T) {
	// Verify the interface is correctly shaped and usable end-to-end.
	// llmProviderFunc (defined below) implements LLMProvider — a compile error
	// here means the interface signature changed.
	var _ LLMProvider = llmProviderFunc(nil)

	called := make([]ChatStreamEvent, 0)
	impl := llmProviderFunc(func(ctx context.Context, req ChatRequest, onEvent ChatStreamCallback) (ChatResponse, error) {
		if onEvent != nil {
			// ToolUseStart: Input is nil (not yet accumulated).
			tcStart := ToolCall{ID: "tc_1", Name: "search"}
			onEvent(ChatStreamEvent{Type: ChatStreamToolUseStart, ToolCall: &tcStart})
			onEvent(ChatStreamEvent{Type: ChatStreamToolUseDelta, Content: `{"q":"test"}`})
			// ToolUseEnd: Input is fully accumulated. Use a separate value.
			tcEnd := ToolCall{ID: "tc_1", Name: "search", Input: json.RawMessage(`{"q":"test"}`)}
			onEvent(ChatStreamEvent{Type: ChatStreamToolUseEnd, ToolCall: &tcEnd})
		}
		return ChatResponse{
			ToolCalls:  []ToolCall{{ID: "tc_1", Name: "search", Input: json.RawMessage(`{"q":"test"}`)}},
			StopReason: "tool_use",
			Usage:      ChatUsage{InputTokens: 10, OutputTokens: 5},
		}, nil
	})

	resp, err := impl.Complete(context.Background(), ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: ChatRoleUser, Content: "search for cats"}},
	}, func(e ChatStreamEvent) {
		called = append(called, e)
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "search" {
		t.Errorf("expected 'search', got %s", resp.ToolCalls[0].Name)
	}
	if len(called) != 3 {
		t.Errorf("expected 3 stream events, got %d", len(called))
	}
	if called[0].Type != ChatStreamToolUseStart {
		t.Errorf("expected ToolUseStart first, got %s", called[0].Type)
	}
	// ToolUseStart: Input should be nil
	if called[0].ToolCall.Input != nil {
		t.Errorf("ToolUseStart should have nil Input")
	}
	// ToolUseEnd: Input should be populated
	if string(called[2].ToolCall.Input) != `{"q":"test"}` {
		t.Errorf("ToolUseEnd should have full Input, got %s", called[2].ToolCall.Input)
	}
	if resp.Usage.InputTokens != 10 {
		t.Errorf("expected 10 input tokens, got %d", resp.Usage.InputTokens)
	}
}

// llmProviderFunc adapts a function to the LLMProvider interface.
type llmProviderFunc func(ctx context.Context, req ChatRequest, onEvent ChatStreamCallback) (ChatResponse, error)

func (f llmProviderFunc) Complete(ctx context.Context, req ChatRequest, onEvent ChatStreamCallback) (ChatResponse, error) {
	return f(ctx, req, onEvent)
}
func (f llmProviderFunc) Name() string { return "func" }
