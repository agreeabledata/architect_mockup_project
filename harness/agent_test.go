package harness

import (
	"context"
	"encoding/json"
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
		{Content: "rest", ToolCalls: []ToolCall{{ID: "r1", Name: "rest", Arguments: json.RawMessage("{}")}}, StopReason: StopReasonToolCalls},
		{Content: "rest", ToolCalls: []ToolCall{{ID: "r2", Name: "rest", Arguments: json.RawMessage("{}")}}, StopReason: StopReasonToolCalls},
		{Content: "rest", ToolCalls: []ToolCall{{ID: "r3", Name: "rest", Arguments: json.RawMessage("{}")}}, StopReason: StopReasonToolCalls},
		{Content: "rest", ToolCalls: []ToolCall{{ID: "r4", Name: "rest", Arguments: json.RawMessage("{}")}}, StopReason: StopReasonToolCalls},
	}}
	agent := &Agent{Provider: prov, Config: testConfig(3)}
	report, err := agent.Run(context.Background(), stub.New())
	if err != nil {
		t.Fatal(err)
	}
	if report.Reason != "iteration_cap" {
		t.Errorf("reason = %q, want iteration_cap", report.Reason)
	}
	if report.Iterations != 3 {
		t.Errorf("iterations = %d, want 3", report.Iterations)
	}
	if prov.calls != 3 {
		t.Errorf("provider called %d times, want 3", prov.calls)
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
