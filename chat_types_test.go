package claudeagent

import (
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

func TestChatResponse_StopReasons(t *testing.T) {
	for _, reason := range []string{"end_turn", "tool_use", "max_tokens"} {
		resp := ChatResponse{StopReason: reason}
		if resp.StopReason != reason {
			t.Errorf("expected %s, got %s", reason, resp.StopReason)
		}
	}
}

func TestChatUsage_ZeroForNonAnthropic(t *testing.T) {
	usage := ChatUsage{InputTokens: 100, OutputTokens: 50}
	if usage.CacheCreationInputTokens != 0 {
		t.Error("expected 0 cache creation tokens")
	}
	if usage.CacheReadInputTokens != 0 {
		t.Error("expected 0 cache read tokens")
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

func TestChatStreamCallback_Nil(t *testing.T) {
	// Nil callback should be safe to check.
	var cb ChatStreamCallback
	if cb != nil {
		t.Error("expected nil callback")
	}
}
