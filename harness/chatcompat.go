package harness

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

// OpenAICompatProvider talks to any OpenAI-compatible Chat Completions endpoint
// (OpenAI, DeepSeek, OpenRouter, Ollama) via the official openai-go SDK.
type OpenAICompatProvider struct {
	client openai.Client
}

// NewOpenAICompatProvider builds a provider for a base URL and API key.
// baseURL empty = OpenAI default; key empty = local model (Ollama).
func NewOpenAICompatProvider(baseURL, apiKey string) (*OpenAICompatProvider, error) {
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	return &OpenAICompatProvider{client: openai.NewClient(opts...)}, nil
}

// Chat sends one normalized request to the chat completions endpoint.
func (p *OpenAICompatProvider) Chat(ctx context.Context, messages []Message, tools []Tool, opts CallOptions) (Result, error) {
	params := openai.ChatCompletionNewParams{
		Model:    opts.Model,
		Messages: make([]openai.ChatCompletionMessageParamUnion, 0, len(messages)),
	}
	if opts.MaxTokens > 0 {
		params.MaxTokens = openai.Int(opts.MaxTokens)
	}
	if opts.Temperature != nil {
		params.Temperature = openai.Float(*opts.Temperature)
	}
	if opts.TopP != nil {
		params.TopP = openai.Float(*opts.TopP)
	}
	if opts.Seed != nil {
		params.Seed = openai.Int(*opts.Seed)
	}
	for _, m := range messages {
		params.Messages = append(params.Messages, toOpenAIMessage(m))
	}
	for _, t := range tools {
		params.Tools = append(params.Tools, openai.ChatCompletionToolUnionParam{
			OfFunction: &openai.ChatCompletionFunctionToolParam{
				Function: shared.FunctionDefinitionParam{
					Name:        t.Name,
					Description: openai.String(t.Description),
					Parameters:  t.Parameters,
				},
			},
		})
	}

	comp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return Result{}, fmt.Errorf("chat completions: %w", err)
	}

	var res Result
	if len(comp.Choices) > 0 {
		msg := comp.Choices[0].Message
		res.Content = msg.Content
		for _, tc := range msg.ToolCalls {
			res.ToolCalls = append(res.ToolCalls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: json.RawMessage(tc.Function.Arguments),
			})
		}
		switch comp.Choices[0].FinishReason {
		case "tool_calls":
			res.StopReason = StopReasonToolCalls
		case "length":
			res.StopReason = StopReasonLength
		case "stop":
			res.StopReason = StopReasonStop
		default:
			res.StopReason = StopReasonError
		}
	}
	res.Usage = Usage{
		InputTokens:  comp.Usage.PromptTokens,
		OutputTokens: comp.Usage.CompletionTokens,
		TotalTokens:  comp.Usage.TotalTokens,
	}
	return res, nil
}

func toOpenAIMessage(m Message) openai.ChatCompletionMessageParamUnion {
	switch m.Role {
	case RoleSystem:
		return openai.SystemMessage(m.Content)
	case RoleUser:
		return openai.UserMessage(m.Content)
	case RoleTool:
		return openai.ToolMessage(m.Content, m.ToolCallID)
	case RoleAssistant:
		if len(m.ToolCalls) > 0 {
			var tcs []openai.ChatCompletionMessageToolCallUnionParam
			for _, tc := range m.ToolCalls {
				tcs = append(tcs, openai.ChatCompletionMessageToolCallUnionParam{
					OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
						ID:   tc.ID,
						Type: "function",
						Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
							Name:      tc.Name,
							Arguments: string(tc.Arguments),
						},
					},
				})
			}
			return openai.ChatCompletionMessageParamUnion{
				OfAssistant: &openai.ChatCompletionAssistantMessageParam{
					Content: openai.ChatCompletionAssistantMessageParamContentUnion{
						OfString: openai.String(m.Content),
					},
					ToolCalls: tcs,
				},
			}
		}
		return openai.AssistantMessage(m.Content)
	}
	return openai.UserMessage(m.Content)
}
