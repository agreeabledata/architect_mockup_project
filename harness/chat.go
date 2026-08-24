// Package harness drives an agent through a game via a provider-neutral
// chat(messages, tools) interface.
package harness

import (
	"context"
	"encoding/json"
)

// Role identifies who authored a message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall is one structured action the model requested.
type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

// Message is one turn in the conversation.
type Message struct {
	Role       Role
	Content    string
	ToolCallID string     // set when Role == RoleTool
	ToolCalls  []ToolCall // set when Role == RoleAssistant
}

// Tool is a function the model may call, described with a JSON Schema.
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON Schema object; nil means no args
}

// Usage is normalized token accounting.
type Usage struct {
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
}

// StopReason says why generation ended.
type StopReason string

const (
	StopReasonStop      StopReason = "stop"
	StopReasonToolCalls StopReason = "tool_calls"
	StopReasonLength    StopReason = "length"
	StopReasonError     StopReason = "error"
)

// Result is a normalized provider response.
type Result struct {
	Content    string
	ToolCalls  []ToolCall
	StopReason StopReason
	Usage      Usage
}

// CallOptions are per-call decoding settings applied identically across
// providers. Models that reject a setting get it omitted (the divergence is
// logged by the caller).
type CallOptions struct {
	Model       string
	MaxTokens   int64
	Temperature *float64
	TopP        *float64
	Seed        *int64
}

// Provider is the neutral chat(messages, tools) seam. Implementations cover
// OpenAI-compatible endpoints (OpenAI, DeepSeek, Ollama) and Anthropic.
type Provider interface {
	Chat(ctx context.Context, messages []Message, tools []Tool, opts CallOptions) (Result, error)
}

func addUsage(a, b Usage) Usage {
	return Usage{
		InputTokens:  a.InputTokens + b.InputTokens,
		OutputTokens: a.OutputTokens + b.OutputTokens,
		TotalTokens:  a.TotalTokens + b.TotalTokens,
	}
}
