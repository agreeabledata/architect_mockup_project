package harness

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// AnthropicProvider talks to Claude via the official SDK, behind the same
// neutral chat interface.
type AnthropicProvider struct {
	client anthropic.Client
}

// NewAnthropicProvider builds a provider for an API key and optional base URL
// ("" = Anthropic default; override for tests/proxies). Empty key = read
// ANTHROPIC_API_KEY from the environment.
func NewAnthropicProvider(apiKey, baseURL string) *AnthropicProvider {
	var opts []option.RequestOption
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	return &AnthropicProvider{client: anthropic.NewClient(opts...)}
}

// Chat sends one normalized request to the Messages API.
func (p *AnthropicProvider) Chat(ctx context.Context, messages []Message, tools []Tool, opts CallOptions) (Result, error) {
	params := anthropic.MessageNewParams{
		Model:     opts.Model,
		MaxTokens: opts.MaxTokens,
		Messages:  make([]anthropic.MessageParam, 0, len(messages)),
	}
	if params.MaxTokens == 0 {
		params.MaxTokens = 1024
	}
	if opts.Temperature != nil {
		params.Temperature = anthropic.Float(*opts.Temperature)
	}
	if opts.TopP != nil {
		params.TopP = anthropic.Float(*opts.TopP)
	}
	for _, m := range messages {
		switch m.Role {
		case RoleSystem:
			var b strings.Builder
			b.WriteString(m.Content)
			params.System = append(params.System, anthropic.TextBlockParam{Text: b.String()})
		case RoleUser:
			if m.ToolCallID != "" {
				params.Messages = append(params.Messages, anthropic.NewUserMessage(
					anthropic.NewToolResultBlock(m.ToolCallID, m.Content, false),
				))
			} else {
				params.Messages = append(params.Messages, anthropic.NewUserMessage(
					anthropic.NewTextBlock(m.Content),
				))
			}
		case RoleAssistant:
			if len(m.ToolCalls) > 0 {
				blocks := make([]anthropic.ContentBlockParamUnion, 0, len(m.ToolCalls)+1)
				if m.Content != "" {
					blocks = append(blocks, anthropic.NewTextBlock(m.Content))
				}
				for _, tc := range m.ToolCalls {
					blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, tc.Arguments, tc.Name))
				}
				params.Messages = append(params.Messages, anthropic.NewAssistantMessage(blocks...))
			} else {
				params.Messages = append(params.Messages, anthropic.NewAssistantMessage(
					anthropic.NewTextBlock(m.Content),
				))
			}
		}
	}
	for _, t := range tools {
		params.Tools = append(params.Tools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name: t.Name,
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: t.Parameters["properties"],
					Required:   stringSlice(t.Parameters["required"]),
				},
			},
		})
	}

	msg, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return Result{}, fmt.Errorf("messages: %w", err)
	}

	res := Result{StopReason: StopReasonStop}
	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			res.Content += block.Text
		case "tool_use":
			res.ToolCalls = append(res.ToolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: block.Input,
			})
			res.StopReason = StopReasonToolCalls
		}
	}
	switch string(msg.StopReason) {
	case "tool_use":
		res.StopReason = StopReasonToolCalls
	case "max_tokens":
		res.StopReason = StopReasonLength
	case "end_turn", "stop_sequence":
		res.StopReason = StopReasonStop
	default:
		if res.StopReason == StopReasonStop {
			res.StopReason = StopReasonError
		}
	}
	u := msg.Usage
	res.Usage = Usage{
		InputTokens:  u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens,
		OutputTokens: u.OutputTokens,
		TotalTokens:  u.InputTokens + u.OutputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens,
	}
	return res, nil
}

func stringSlice(v any) []string {
	var out []string
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	for _, s := range raw {
		if str, ok := s.(string); ok {
			out = append(out, str)
		}
	}
	return out
}
