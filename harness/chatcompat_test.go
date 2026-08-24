package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAICompatChat(t *testing.T) {
	var gotReq map[string]any
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q, want /chat/completions", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"x","object":"chat.completion","model":"fake","choices":[{"index":0,"message":{"role":"assistant","content":"Resting.","tool_calls":[{"id":"call_1","type":"function","function":{"name":"rest","arguments":"{}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":11,"completion_tokens":3,"total_tokens":14}}`)
	}))
	defer srv.Close()

	prov, err := NewOpenAICompatProvider(srv.URL, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	res, err := prov.Chat(context.Background(),
		[]Message{
			{Role: RoleSystem, Content: "sys"},
			{Role: RoleUser, Content: "hi"},
		},
		[]Tool{{Name: "rest", Description: "do nothing", Parameters: map[string]any{"type": "object"}}},
		CallOptions{Model: "m", MaxTokens: 5, Temperature: ptr(0.5)},
	)
	if err != nil {
		t.Fatal(err)
	}

	if res.Content != "Resting." {
		t.Errorf("content = %q", res.Content)
	}
	if len(res.ToolCalls) != 1 || res.ToolCalls[0].Name != "rest" || res.ToolCalls[0].ID != "call_1" {
		t.Errorf("tool calls = %+v", res.ToolCalls)
	}
	if res.StopReason != StopReasonToolCalls {
		t.Errorf("stop reason = %q", res.StopReason)
	}
	want := Usage{11, 3, 14}
	if res.Usage != want {
		t.Errorf("usage = %+v, want %+v", res.Usage, want)
	}

	if gotAuth != "Bearer test-key" {
		t.Errorf("authorization = %q", gotAuth)
	}
	if gotReq["model"] != "m" {
		t.Errorf("model = %v", gotReq["model"])
	}
	msgs, _ := gotReq["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("messages = %+v", gotReq["messages"])
	}
	first := msgs[0].(map[string]any)
	if first["role"] != "system" || first["content"] != "sys" {
		t.Errorf("first message = %+v", first)
	}
	tools, _ := gotReq["tools"].([]any)
	fn := tools[0].(map[string]any)["function"].(map[string]any)
	if fn["name"] != "rest" {
		t.Errorf("tool function = %+v", fn)
	}
	if gotReq["temperature"] != 0.5 || gotReq["max_tokens"] != float64(5) {
		t.Errorf("decoding params = %+v", gotReq)
	}
}

func TestOpenAICompatRoundTripMessage(t *testing.T) {
	// A request carrying assistant tool calls + tool results must serialize
	// into the wire format both ways.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		raw, _ := json.Marshal(req["messages"])
		if !strings.Contains(string(raw), `"tool_calls"`) {
			t.Errorf("assistant tool calls not serialized: %s", raw)
		}
		if !strings.Contains(string(raw), `"tool_call_id":"c1"`) {
			t.Errorf("tool result not serialized: %s", raw)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"x","object":"chat.completion","model":"m","choices":[{"index":0,"message":{"role":"assistant","content":""},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer srv.Close()

	prov, _ := NewOpenAICompatProvider(srv.URL, "k")
	_, err := prov.Chat(context.Background(), []Message{
		{Role: RoleAssistant, Content: "thinking", ToolCalls: []ToolCall{{ID: "c1", Name: "rest", Arguments: json.RawMessage(`{}`)}}},
		{Role: RoleTool, ToolCallID: "c1", Content: "you rest"},
	}, nil, CallOptions{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
}

func ptr[T any](v T) *T { return &v }
