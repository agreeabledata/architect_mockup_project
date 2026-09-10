package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"architect_mockup_project/stub"
)

// scriptedProvider returns results from a plan and records every request it
// sees, so tests can drive the loop deterministically and inspect it.
type scriptedProvider struct {
	plan    []Result
	calls   int
	gotMsgs [][]Message
	gotOpts []CallOptions
}

func (p *scriptedProvider) Chat(_ context.Context, msgs []Message, _ []Tool, opts CallOptions) (Result, error) {
	p.gotMsgs = append(p.gotMsgs, append([]Message(nil), msgs...))
	p.gotOpts = append(p.gotOpts, opts)
	res := p.plan[p.calls]
	p.calls++
	return res, nil
}

func testConfig(cap int) *Config {
	return &Config{Model: "test", IterationCap: cap}
}

func TestAgentFullRun(t *testing.T) {
	prov := &scriptedProvider{plan: []Result{
		{
			Content:    "I'll take the delivery.",
			ToolCalls:  []ToolCall{{ID: "c1", Name: "take_delivery", Arguments: json.RawMessage("{}")}},
			StopReason: StopReasonToolCalls,
			Usage:      Usage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
		},
		{
			Content:    "Delivering now.",
			ToolCalls:  []ToolCall{{ID: "c2", Name: "deliver_package", Arguments: json.RawMessage("{}")}},
			StopReason: StopReasonToolCalls,
			Usage:      Usage{InputTokens: 20, OutputTokens: 6, TotalTokens: 26},
		},
		{
			Content:    "The fight is a bad deal; I'm done.",
			StopReason: StopReasonStop,
			Usage:      Usage{InputTokens: 30, OutputTokens: 7, TotalTokens: 37},
		},
	}}
	agent := &Agent{Provider: prov, Config: testConfig(10)}
	report, err := agent.Run(context.Background(), stub.New())
	if err != nil {
		t.Fatal(err)
	}

	if report.Balance != 12 {
		t.Errorf("balance = %d, want 12", report.Balance)
	}
	if report.Iterations != 3 {
		t.Errorf("iterations = %d, want 3", report.Iterations)
	}
	if report.ToolCalls != 2 {
		t.Errorf("tool calls = %d, want 2", report.ToolCalls)
	}
	if report.Reason != "no_action" {
		t.Errorf("reason = %q, want no_action", report.Reason)
	}
	want := Usage{InputTokens: 60, OutputTokens: 18, TotalTokens: 78}
	if report.Usage != want {
		t.Errorf("usage = %+v, want %+v", report.Usage, want)
	}
	if got := report.Cost(&Config{Model: "test", Pricing: map[string]*Pricing{"test": {InputPerMillion: 1, OutputPerMillion: 2}}}); got != 6e-5+3.6e-5 {
		t.Errorf("cost = %v", got)
	}

	// The engine's outcome was fed back as a tool message on the next call.
	last := prov.gotMsgs[len(prov.gotMsgs)-1]
	var found bool
	for _, m := range last {
		if m.Role == RoleTool && m.ToolCallID == "c2" {
			found = true
		}
	}
	if !found {
		t.Error("deliver_package result was not fed back as a tool message")
	}
}

func TestAgentIterationCap(t *testing.T) {
	prov := &scriptedProvider{plan: []Result{
		{Content: "I'll take the delivery.", ToolCalls: []ToolCall{{ID: "c1", Name: "take_delivery", Arguments: json.RawMessage("{}")}}, StopReason: StopReasonToolCalls},
		{Content: "Delivering now.", ToolCalls: []ToolCall{{ID: "c2", Name: "deliver_package", Arguments: json.RawMessage("{}")}}, StopReason: StopReasonToolCalls},
	}}
	agent := &Agent{Provider: prov, Config: testConfig(2)}
	report, err := agent.Run(context.Background(), stub.New())
	if err != nil {
		t.Fatal(err)
	}
	if report.Reason != "iteration_cap" {
		t.Errorf("reason = %q, want iteration_cap", report.Reason)
	}
	if report.Iterations != 2 {
		t.Errorf("iterations = %d, want 2", report.Iterations)
	}
	if prov.calls != 2 {
		t.Errorf("provider called %d times, want 2", prov.calls)
	}
	// The cap check precedes Observe, so Balance must be re-read after the
	// last applied action, not left at the pre-delivery value.
	if report.Balance != 12 {
		t.Errorf("balance = %d, want 12 (engine state after deliver)", report.Balance)
	}
}

func TestAgentMalformedAction(t *testing.T) {
	prov := &scriptedProvider{plan: []Result{
		{
			Content:    "fly away",
			ToolCalls:  []ToolCall{{ID: "bad", Name: "fly", Arguments: json.RawMessage("{}")}},
			StopReason: StopReasonToolCalls,
		},
		{Content: "Resting.", ToolCalls: []ToolCall{{ID: "r", Name: "rest", Arguments: json.RawMessage("{}")}}, StopReason: StopReasonToolCalls},
		{Content: "Done.", StopReason: StopReasonStop},
	}}
	agent := &Agent{Provider: prov, Config: testConfig(10)}
	report, err := agent.Run(context.Background(), stub.New())
	if err != nil {
		t.Fatal(err)
	}
	if report.Iterations != 3 {
		t.Errorf("iterations = %d, want 3 (loop must survive a bad action)", report.Iterations)
	}
	var fedBack bool
	for _, m := range prov.gotMsgs[1] {
		if m.Role == RoleTool && m.ToolCallID == "bad" && m.Content == "Invalid action: unknown action \"fly\"" {
			fedBack = true
		}
	}
	if !fedBack {
		t.Error("engine error was not fed back as a tool message")
	}
}

// TestAgentDecisionRetryCap: a model that only emits invalid actions on one
// decision is cut off at the per-decision retry cap, not allowed to churn the
// whole per-run budget.
func TestAgentDecisionRetryCap(t *testing.T) {
	prov := &scriptedProvider{plan: []Result{
		{Content: "fly away", ToolCalls: []ToolCall{{ID: "b1", Name: "fly", Arguments: json.RawMessage("{}")}}, StopReason: StopReasonToolCalls},
		{Content: "fly away", ToolCalls: []ToolCall{{ID: "b2", Name: "fly", Arguments: json.RawMessage("{}")}}, StopReason: StopReasonToolCalls},
		{Content: "fly away", ToolCalls: []ToolCall{{ID: "b3", Name: "fly", Arguments: json.RawMessage("{}")}}, StopReason: StopReasonToolCalls},
		{Content: "fly away", ToolCalls: []ToolCall{{ID: "b4", Name: "fly", Arguments: json.RawMessage("{}")}}, StopReason: StopReasonToolCalls},
	}}
	agent := &Agent{Provider: prov, Config: testConfig(10)} // DecisionRetryCap 0 -> default 3
	report, err := agent.Run(context.Background(), stub.New())
	if err != nil {
		t.Fatal(err)
	}
	if report.Reason != "decision_cap" {
		t.Errorf("reason = %q, want decision_cap", report.Reason)
	}
	if report.Iterations != 3 {
		t.Errorf("iterations = %d, want 3 (retry cap stops the decision)", report.Iterations)
	}
	if prov.calls != 3 {
		t.Errorf("provider called %d times, want 3", prov.calls)
	}
	// Retries must not re-send a fresh observation; the invalid-action error
	// is the next attempt's prompt.
	var users int
	for _, m := range prov.gotMsgs[2] {
		if m.Role == RoleUser {
			users++
		}
	}
	if users != 1 {
		t.Errorf("call 3 has %d user messages, want 1 (no re-observation on retry)", users)
	}
}

func TestLastTurns(t *testing.T) {
	msgs := []Message{
		{Role: RoleSystem, Content: "sys"},
		{Role: RoleUser, Content: "u1"},
		{Role: RoleAssistant, Content: "a1"},
		{Role: RoleTool, ToolCallID: "1", Content: "t1"},
		{Role: RoleUser, Content: "u2"},
		{Role: RoleAssistant, Content: "a2"},
		{Role: RoleUser, Content: "u3"},
		{Role: RoleAssistant, Content: "a3"},
	}

	if got := lastTurns(msgs, 5); len(got) != len(msgs) {
		t.Errorf("short history trimmed: %d, want %d", len(got), len(msgs))
	}

	got := lastTurns(msgs, 2)
	if len(got) != 5 || got[0].Role != RoleSystem {
		t.Fatalf("window = %+v", got)
	}
	if got[1].Role != RoleUser || got[1].Content != "u2" {
		t.Errorf("window must start at the 2nd-last user turn, got %+v", got[1])
	}
	for _, m := range got {
		if m.Content == "u1" || m.Content == "t1" {
			t.Errorf("stale turn survived: %+v", m)
		}
	}
}

// TestAgentWindowedContext: once the run exceeds contextTurns decisions, the
// provider sees only the system prompt plus the last few turns, not the whole
// conversation.
func TestAgentWindowedContext(t *testing.T) {
	var plan []Result
	for i := 0; i < 5; i++ {
		plan = append(plan, Result{Content: "Rest.", ToolCalls: []ToolCall{{ID: fmt.Sprintf("r%d", i), Name: "rest", Arguments: json.RawMessage("{}")}}, StopReason: StopReasonToolCalls})
	}
	plan = append(plan, Result{Content: "Done.", StopReason: StopReasonStop})
	prov := &scriptedProvider{plan: plan}
	agent := &Agent{Provider: prov, Config: testConfig(10)}
	if _, err := agent.Run(context.Background(), stub.New()); err != nil {
		t.Fatal(err)
	}
	if prov.calls != 6 {
		t.Fatalf("provider called %d times, want 6", prov.calls)
	}

	last := prov.gotMsgs[len(prov.gotMsgs)-1]
	if last[0].Role != RoleSystem {
		t.Fatalf("window must start with the system prompt")
	}
	var users int
	for _, m := range last {
		if m.Role == RoleUser {
			users++
		}
	}
	if users != contextTurns {
		t.Errorf("final call has %d user turns, want %d", users, contextTurns)
	}
	// The oldest observation must be gone from the window, not just trimmed to
	// a fixed length by coincidence.
	firstTurn := ""
	for _, m := range prov.gotMsgs[0] {
		if m.Role == RoleUser {
			firstTurn = m.Content
			break
		}
	}
	for _, m := range last {
		if m.Role == RoleUser && m.Content == firstTurn {
			t.Error("oldest turn still reaches the model")
		}
	}
}
