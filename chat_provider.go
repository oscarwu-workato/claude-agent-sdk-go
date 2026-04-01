package claudeagent

import "context"

// LLMProvider abstracts an LLM chat completions endpoint.
// Implementations handle message format conversion, streaming, and
// provider-specific features (e.g., prompt caching for Anthropic).
//
// The APIAgent's agentic loop calls Complete on each turn, passing
// provider-agnostic ChatMessages and receiving a ChatResponse.
type LLMProvider interface {
	// Complete sends a chat request and returns the full response.
	// onEvent, if non-nil, is called with streaming deltas as they arrive.
	// Streaming providers call onEvent for each content/tool chunk as it arrives.
	// Non-streaming providers may ignore onEvent entirely; the ChatResponse is
	// always the authoritative result regardless of whether onEvent was called.
	Complete(ctx context.Context, req ChatRequest, onEvent ChatStreamCallback) (ChatResponse, error)

	// Name returns a human-readable provider name (e.g., "anthropic", "openai").
	Name() string
}
