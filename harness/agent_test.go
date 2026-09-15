package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"architect_mockup_project/engine"
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

func TestActionsToToolsInjectsReasoning(t *testing.T) {
	t.Run("nil params", func(t *testing.T) {
		tools := actionsToTools([]engine.Action{{Name: "rest", Description: "do nothing"}})
		if len(tools) != 1 {
			t.Fatalf("len(tools) = %d, want 1", len(tools))
		}
		assertReasoningSchema(t, tools[0].Parameters)
		if _, ok := tools[0].Parameters["properties"].(map[string]any)["quest_id"]; ok {
			t.Fatal("nil-param tool should only have reasoning")
		}
		if got := requiredStrings(t, tools[0].Parameters); !slices.Equal(got, []string{reasoningParam}) {
			t.Errorf("required = %v, want [%s]", got, reasoningParam)
		}
	})

	t.Run("accept_quest keeps quest_id", func(t *testing.T) {
		orig := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"quest_id": map[string]any{
					"type": "string",
					"enum": []string{"q-d"},
				},
			},
			"required": []any{"quest_id"},
		}
		tools := actionsToTools([]engine.Action{{Name: "accept_quest", Parameters: orig}})
		params := tools[0].Parameters
		assertReasoningSchema(t, params)
		props := params["properties"].(map[string]any)
		qid, ok := props["quest_id"].(map[string]any)
		if !ok {
			t.Fatal("quest_id property missing")
		}
		enum, _ := qid["enum"].([]string)
		if !slices.Equal(enum, []string{"q-d"}) {
			t.Errorf("quest_id enum = %v, want [q-d]", enum)
		}
		if got := requiredStrings(t, params); !slices.Equal(got, []string{reasoningParam, "quest_id"}) {
			t.Errorf("required = %v, want [%s quest_id]", got, reasoningParam)
		}
	})

	t.Run("does not mutate engine.World", func(t *testing.T) {
		w := engine.New(1)
		_ = actionsToTools(w.LegalActions())
		for _, a := range w.LegalActions() {
			if schemaMentionsReasoning(a.Parameters) {
				t.Errorf("engine %s schema gained reasoning: %+v", a.Name, a.Parameters)
			}
			if a.Name != "accept_quest" {
				if a.Parameters != nil {
					t.Errorf("engine %s Parameters = %v, want nil", a.Name, a.Parameters)
				}
				continue
			}
			req := requiredStrings(t, a.Parameters)
			if !slices.Equal(req, []string{"quest_id"}) {
				t.Errorf("engine accept_quest required = %v, want [quest_id]", req)
			}
		}
	})
}

func schemaMentionsReasoning(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		if _, ok := x[reasoningParam]; ok {
			return true
		}
		for _, child := range x {
			if schemaMentionsReasoning(child) {
				return true
			}
		}
	case []any:
		for _, child := range x {
			if s, ok := child.(string); ok && s == reasoningParam {
				return true
			}
			if schemaMentionsReasoning(child) {
				return true
			}
		}
	case []string:
		return slices.Contains(x, reasoningParam)
	}
	return false
}

func TestStripReasoning(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := stripReasoning(nil); got != nil {
			t.Errorf("stripReasoning(nil) = %s, want nil", got)
		}
	})
	t.Run("reasoning only", func(t *testing.T) {
		got := stripReasoning(json.RawMessage(`{"reasoning":"short is 30/day"}`))
		assertJSONMap(t, got, map[string]any{})
	})
	t.Run("reasoning and quest_id", func(t *testing.T) {
		got := stripReasoning(json.RawMessage(`{"reasoning":"short is 30/day","quest_id":"q-d"}`))
		assertJSONMap(t, got, map[string]any{"quest_id": "q-d"})
	})
}

func TestAgentStripsReasoningBeforeApply(t *testing.T) {
	raw := json.RawMessage(`{"reasoning":"short is 30/day","quest_id":"q-d"}`)
	prov := &scriptedProvider{plan: []Result{
		{
			Content:    "Take the short run.",
			ToolCalls:  []ToolCall{{ID: "c1", Name: "accept_quest", Arguments: raw}},
			StopReason: StopReasonToolCalls,
		},
		{Content: "Done.", StopReason: StopReasonStop},
	}}
	dir := t.TempDir()
	tr, err := NewTranscript(dir, testConfig(10), 0)
	if err != nil {
		t.Fatal(err)
	}
	g := &recordingGame{}
	agent := &Agent{Provider: prov, Config: testConfig(10), Transcript: tr}
	if _, err := agent.Run(context.Background(), g); err != nil {
		t.Fatal(err)
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}

	if len(g.applies) != 1 {
		t.Fatalf("Apply called %d times, want 1", len(g.applies))
	}
	assertJSONMap(t, g.applies[0].args, map[string]any{"quest_id": "q-d"})
	if bytes.Contains(g.applies[0].args, []byte(reasoningParam)) {
		t.Errorf("Apply args still contain reasoning: %s", g.applies[0].args)
	}

	last := prov.gotMsgs[len(prov.gotMsgs)-1]
	var kept bool
	for _, m := range last {
		if m.Role != RoleAssistant {
			continue
		}
		for _, tc := range m.ToolCalls {
			if string(tc.Arguments) == string(raw) {
				kept = true
			}
		}
	}
	if !kept {
		t.Error("conversation history lost the original reasoning arguments")
	}

	var callKept, turnStripped bool
	for _, line := range readJSONL(t, tr.Path()) {
		switch line["kind"] {
		case kindCall:
			raw, _ := json.Marshal(line)
			if bytes.Contains(raw, []byte(`"reasoning":"short is 30/day"`)) {
				callKept = true
			}
		case kindTurn:
			args, _ := line["args"].(map[string]any)
			if args["quest_id"] != "q-d" {
				t.Errorf("turn args = %v, want quest_id q-d", args)
			}
			if _, ok := args[reasoningParam]; ok {
				t.Errorf("turn args still contain reasoning: %v", args)
			}
			turnStripped = true
		}
	}
	if !callKept {
		t.Error("call line did not keep reasoning")
	}
	if !turnStripped {
		t.Error("missing turn line with stripped args")
	}
}

func assertReasoningSchema(t *testing.T, params map[string]any) {
	t.Helper()
	if params == nil {
		t.Fatal("Parameters is nil")
	}
	if params["type"] != "object" {
		t.Errorf("type = %v, want object", params["type"])
	}
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties is %T, want map", params["properties"])
	}
	r, ok := props[reasoningParam].(map[string]any)
	if !ok {
		t.Fatal("reasoning property missing")
	}
	if r["type"] != "string" {
		t.Errorf("reasoning type = %v, want string", r["type"])
	}
}

func requiredStrings(t *testing.T, params map[string]any) []string {
	t.Helper()
	switch req := params["required"].(type) {
	case []any:
		out := make([]string, len(req))
		for i, x := range req {
			s, ok := x.(string)
			if !ok {
				t.Fatalf("required[%d] = %T, want string", i, x)
			}
			out[i] = s
		}
		return out
	case []string:
		return req
	default:
		t.Fatalf("required is %T", params["required"])
		return nil
	}
}

func assertJSONMap(t *testing.T, raw json.RawMessage, want map[string]any) {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	if len(got) != len(want) {
		t.Fatalf("json %s: got %d keys, want %d (%v)", raw, len(got), len(want), want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("json %s: %s = %v, want %v", raw, k, got[k], v)
		}
	}
}

type applyCall struct {
	name string
	args json.RawMessage
}

type recordingGame struct {
	applies []applyCall
}

func (g *recordingGame) Observe() engine.State {
	return engine.State{Narrative: "a board"}
}

func (g *recordingGame) LegalActions() []engine.Action {
	return []engine.Action{{
		Name: "accept_quest",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"quest_id": map[string]any{
					"type": "string",
					"enum": []string{"q-d"},
				},
			},
			"required": []any{"quest_id"},
		},
	}}
}

func (g *recordingGame) Apply(name string, args json.RawMessage) (engine.Outcome, error) {
	copied := append(json.RawMessage(nil), args...)
	g.applies = append(g.applies, applyCall{name: name, args: copied})
	return engine.Outcome{Narrative: "ok"}, nil
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
	tr, err := NewTranscript(t.TempDir(), testConfig(10), 0)
	if err != nil {
		t.Fatal(err)
	}
	agent := &Agent{Provider: prov, Config: testConfig(10), Transcript: tr}
	report, err := agent.Run(context.Background(), stub.New())
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.Close(); err != nil {
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

	turns := readTurns(t, tr.Path())
	if len(turns) < 1 {
		t.Fatal("want at least the rejected fly turn")
	}
	fly := turns[0]
	if fly.Action != "fly" {
		t.Errorf("action = %q, want fly", fly.Action)
	}
	if fly.Error == nil || *fly.Error != `unknown action "fly"` {
		t.Errorf("error = %v, want unknown action \"fly\"", fly.Error)
	}
	if fly.Earned != 0 || fly.DaysSpent != 0 || fly.Balance != 0 || fly.Day != 0 {
		t.Errorf("rejected turn mutated numbers: %+v", fly)
	}
}

func TestAgentTranscriptTurns(t *testing.T) {
	prov := &scriptedProvider{plan: []Result{
		{
			ToolCalls:  []ToolCall{{ID: "c1", Name: "take_delivery", Arguments: json.RawMessage("{}")}},
			StopReason: StopReasonToolCalls,
		},
		{
			ToolCalls:  []ToolCall{{ID: "c2", Name: "deliver_package", Arguments: json.RawMessage("{}")}},
			StopReason: StopReasonToolCalls,
		},
		{StopReason: StopReasonStop},
	}}
	tr, err := NewTranscript(t.TempDir(), testConfig(10), 0)
	if err != nil {
		t.Fatal(err)
	}
	agent := &Agent{Provider: prov, Config: testConfig(10), Transcript: tr}
	if _, err := agent.Run(context.Background(), stub.New()); err != nil {
		t.Fatal(err)
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}

	turns := readTurns(t, tr.Path())
	if len(turns) != 2 {
		t.Fatalf("turns = %d, want 2", len(turns))
	}

	pick := turns[0]
	if pick.Action != "take_delivery" || pick.Turn != 1 {
		t.Errorf("first turn = %+v", pick)
	}
	if pick.Earned != 0 || pick.DaysSpent != 1 || pick.Balance != 0 || pick.Day != 1 {
		t.Errorf("pickup numbers = earned %d days %d balance %d day %d", pick.Earned, pick.DaysSpent, pick.Balance, pick.Day)
	}
	if pick.Error != nil {
		t.Errorf("pickup error = %v, want null", pick.Error)
	}
	if len(pick.Args) != 0 {
		t.Errorf("pickup args = %v, want {}", pick.Args)
	}

	del := turns[1]
	if del.Action != "deliver_package" || del.Turn != 2 {
		t.Errorf("second turn = %+v", del)
	}
	// stub delivery pays 12 and costs 3 days; assert Outcome ints, not narrative.
	if del.Earned != 12 || del.DaysSpent != 3 || del.Balance != 12 || del.Day != 4 {
		t.Errorf("deliver numbers = earned %d days %d balance %d day %d", del.Earned, del.DaysSpent, del.Balance, del.Day)
	}
	if del.Error != nil {
		t.Errorf("deliver error = %v, want null", del.Error)
	}
}

func readTurns(t *testing.T, path string) []TurnRecord {
	t.Helper()
	var turns []TurnRecord
	for _, line := range readJSONL(t, path) {
		if line["kind"] != kindTurn {
			continue
		}
		raw, err := json.Marshal(line)
		if err != nil {
			t.Fatal(err)
		}
		var rec TurnRecord
		if err := json.Unmarshal(raw, &rec); err != nil {
			t.Fatal(err)
		}
		turns = append(turns, rec)
	}
	return turns
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

func TestAgentRejectedTurnUsesPreActionState(t *testing.T) {
	prov := &scriptedProvider{plan: []Result{
		{ToolCalls: []ToolCall{{ID: "t", Name: "take_delivery", Arguments: json.RawMessage("{}")}}, StopReason: StopReasonToolCalls},
		{ToolCalls: []ToolCall{{ID: "f", Name: "fly", Arguments: json.RawMessage("{}")}}, StopReason: StopReasonToolCalls},
		{ToolCalls: []ToolCall{{ID: "r", Name: "rest", Arguments: json.RawMessage("{}")}}, StopReason: StopReasonToolCalls},
		{StopReason: StopReasonStop},
	}}
	tr, err := NewTranscript(t.TempDir(), testConfig(10), 0)
	if err != nil {
		t.Fatal(err)
	}
	agent := &Agent{Provider: prov, Config: testConfig(10), Transcript: tr}
	if _, err := agent.Run(context.Background(), stub.New()); err != nil {
		t.Fatal(err)
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	turns := readTurns(t, tr.Path())
	var fly *TurnRecord
	for i := range turns {
		if turns[i].Action == "fly" {
			fly = &turns[i]
			break
		}
	}
	if fly == nil {
		t.Fatal("missing fly turn")
	}
	if fly.Error == nil {
		t.Fatal("fly should be rejected")
	}
	if fly.Day != 1 || fly.Balance != 0 {
		t.Errorf("rejected fly at day %d bal %d, want post-pickup pre-action day 1 bal 0", fly.Day, fly.Balance)
	}
}

func TestTranscriptEngineAcceptDeliver(t *testing.T) {
	name, args, err := engine.New(1).BotChoice()
	if err != nil {
		t.Fatal(err)
	}
	if name != engine.ActionAccept {
		t.Fatalf("seed 1 first choice = %s, want %s", name, engine.ActionAccept)
	}

	oracle := engine.New(1)
	acceptOut, err := oracle.Apply(name, args)
	if err != nil {
		t.Fatal(err)
	}
	beforeDeliver := oracle.Observe()
	deliverOut, err := oracle.Apply(engine.ActionDeliver, nil)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := json.Unmarshal(args, &m); err != nil {
		t.Fatal(err)
	}
	m[reasoningParam] = "greedy short"
	withReason, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	prov := &scriptedProvider{plan: []Result{
		{ToolCalls: []ToolCall{{ID: "a", Name: engine.ActionAccept, Arguments: withReason}}, StopReason: StopReasonToolCalls},
		{ToolCalls: []ToolCall{{ID: "d", Name: engine.ActionDeliver, Arguments: json.RawMessage(`{"reasoning":"complete"}`)}}, StopReason: StopReasonToolCalls},
		{StopReason: StopReasonStop},
	}}
	tr, err := NewTranscript(t.TempDir(), testConfig(10), 1)
	if err != nil {
		t.Fatal(err)
	}
	agent := &Agent{Provider: prov, Config: testConfig(10), Transcript: tr}
	if _, err := agent.Run(context.Background(), engine.New(1)); err != nil {
		t.Fatal(err)
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	turns := readTurns(t, tr.Path())
	if len(turns) < 2 {
		t.Fatalf("turns = %d, want at least 2", len(turns))
	}
	acc, del := turns[0], turns[1]
	if acc.Action != engine.ActionAccept {
		t.Errorf("turn 1 action = %s", acc.Action)
	}
	if acc.Earned != acceptOut.Earned || acc.Balance != acceptOut.Balance || acc.Day != engine.StartingLifespanDays-acceptOut.LifespanDays {
		t.Errorf("accept record = %+v, outcome = %+v", acc, acceptOut)
	}
	if del.Action != engine.ActionDeliver {
		t.Errorf("turn 2 action = %s", del.Action)
	}
	if del.Earned != deliverOut.Earned {
		t.Errorf("earned = %d, want %d (quest payout)", del.Earned, deliverOut.Earned)
	}
	if del.DaysSpent != beforeDeliver.LifespanDays-deliverOut.LifespanDays {
		t.Errorf("days_spent = %d, want %d", del.DaysSpent, beforeDeliver.LifespanDays-deliverOut.LifespanDays)
	}
	if del.Day != engine.StartingLifespanDays-deliverOut.LifespanDays {
		t.Errorf("day = %d, want %d", del.Day, engine.StartingLifespanDays-deliverOut.LifespanDays)
	}
	if del.Balance != deliverOut.Balance {
		t.Errorf("balance = %d, want %d", del.Balance, deliverOut.Balance)
	}
}

type twoCallGame struct {
	applies int
}

func (g *twoCallGame) Observe() engine.State {
	if g.applies >= 1 {
		return engine.State{Narrative: "over", Done: true, LifespanDays: 0}
	}
	return engine.State{Narrative: "go", LifespanDays: 50}
}

func (g *twoCallGame) LegalActions() []engine.Action {
	return []engine.Action{{Name: "refresh_board"}}
}

func (g *twoCallGame) Apply(name string, args json.RawMessage) (engine.Outcome, error) {
	g.applies++
	if g.applies == 1 {
		return engine.Outcome{Narrative: "ok", Done: true, LifespanDays: 0}, nil
	}
	return engine.Outcome{}, fmt.Errorf("run is over")
}

func TestAgentAppliesEveryToolCallInResponse(t *testing.T) {
	prov := &scriptedProvider{plan: []Result{{
		ToolCalls: []ToolCall{
			{ID: "1", Name: "refresh_board", Arguments: json.RawMessage(`{}`)},
			{ID: "2", Name: "refresh_board", Arguments: json.RawMessage(`{}`)},
		},
		StopReason: StopReasonToolCalls,
	}}}
	tr, err := NewTranscript(t.TempDir(), testConfig(10), 0)
	if err != nil {
		t.Fatal(err)
	}
	g := &twoCallGame{}
	agent := &Agent{Provider: prov, Config: testConfig(10), Transcript: tr}
	if _, err := agent.Run(context.Background(), g); err != nil {
		t.Fatal(err)
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	if g.applies != 2 {
		t.Errorf("Apply called %d times, want 2", g.applies)
	}
	turns := readTurns(t, tr.Path())
	if len(turns) != 2 {
		t.Fatalf("turns = %d, want 2", len(turns))
	}
	if turns[1].Error == nil || *turns[1].Error != "run is over" {
		t.Errorf("second turn error = %v, want run is over", turns[1].Error)
	}
}
