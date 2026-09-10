package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestAnthropicChat(t *testing.T) {
	var gotReq map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path = %q, want /v1/messages", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "test-key" {
			t.Errorf("x-api-key = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"msg_1","type":"message","role":"assistant","model":"claude","content":[{"type":"text","text":"Let me rest."},{"type":"tool_use","id":"tool_1","name":"rest","input":{}}],"stop_reason":"tool_use","usage":{"input_tokens":10,"output_tokens":4,"cache_read_input_tokens":2,"cache_creation_input_tokens":0}}`)
	}))
	defer srv.Close()

	prov := NewAnthropicProvider("test-key", srv.URL)
	res, err := prov.Chat(context.Background(),
		[]Message{
			{Role: RoleSystem, Content: "sys"},
			{Role: RoleUser, Content: "hi"},
		},
		[]Tool{{Name: "rest", Description: "do nothing", Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"mode": map[string]any{"type": "string"}},
			"required":   []string{"mode"},
		}}},
		CallOptions{Model: "claude-x", MaxTokens: 64},
	)
	if err != nil {
		t.Fatal(err)
	}

	if res.Content != "Let me rest." {
		t.Errorf("content = %q", res.Content)
	}
	if len(res.ToolCalls) != 1 || res.ToolCalls[0].Name != "rest" || res.ToolCalls[0].ID != "tool_1" {
		t.Errorf("tool calls = %+v", res.ToolCalls)
	}
	if res.StopReason != StopReasonToolCalls {
		t.Errorf("stop reason = %q", res.StopReason)
	}
	want := Usage{12, 4, 16}
	if res.Usage != want {
		t.Errorf("usage = %+v, want %+v", res.Usage, want)
	}

	if gotReq["model"] != "claude-x" || gotReq["max_tokens"] != float64(64) {
		t.Errorf("params = %+v", gotReq)
	}
	sys, _ := gotReq["system"].([]any)
	if len(sys) != 1 || sys[0].(map[string]any)["text"] != "sys" {
		t.Errorf("system = %+v", gotReq["system"])
	}
	tools, _ := gotReq["tools"].([]any)
	if tools[0].(map[string]any)["name"] != "rest" {
		t.Errorf("tools = %+v", gotReq["tools"])
	}
	schema := tools[0].(map[string]any)["input_schema"].(map[string]any)
	required, _ := schema["required"].([]any)
	if len(required) != 1 || required[0] != "mode" {
		t.Errorf("required = %+v, want [mode]", required)
	}
}

func TestStringSlice(t *testing.T) {
	cases := []struct {
		in   any
		want []string
	}{
		{[]string{"a", "b"}, []string{"a", "b"}},
		{[]any{"a", "b"}, []string{"a", "b"}},
		{[]any{"a", 7, "b"}, []string{"a", "b"}},
		{nil, nil},
		{"a", nil},
	}
	for _, c := range cases {
		if got := stringSlice(c.in); !slices.Equal(got, c.want) {
			t.Errorf("stringSlice(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestAnthropicToolRoundTrip(t *testing.T) {
	var gotReq map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"m","type":"message","role":"assistant","model":"claude","content":[{"type":"text","text":""}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}`)
	}))
	defer srv.Close()

	prov := NewAnthropicProvider("k", srv.URL)
	_, err := prov.Chat(context.Background(), []Message{
		{Role: RoleAssistant, Content: "thinking", ToolCalls: []ToolCall{{ID: "c1", Name: "rest", Arguments: json.RawMessage(`{}`)}}},
		{Role: RoleTool, ToolCallID: "c1", Content: "you rest"},
	}, []Tool{{Name: "rest", Description: "do nothing", Parameters: map[string]any{"type": "object"}}}, CallOptions{Model: "m", MaxTokens: 8})
	if err != nil {
		t.Fatal(err)
	}

	// Anthropic 400s a tool_use with no following tool_result, so assert the
	// result survives on the wire, not merely that the body serializes.
	var found bool
	for _, m := range gotReq["messages"].([]any) {
		content, _ := m.(map[string]any)["content"].([]any)
		for _, b := range content {
			block, _ := b.(map[string]any)
			if block["type"] == "tool_result" && block["tool_use_id"] == "c1" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("no tool_result for c1 in wire messages: %+v", gotReq["messages"])
	}
}
