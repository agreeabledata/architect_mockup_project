package harness

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestTranscriptKinds(t *testing.T) {
	tr, path := testTranscript(t)
	if err := tr.Record([]Message{{Role: RoleUser, Content: "hi"}}, Result{Content: "ok"}); err != nil {
		t.Fatal(err)
	}
	if err := tr.RecordTurn(TurnRecord{
		Turn:      1,
		Day:       3,
		Balance:   12,
		Action:    "deliver_package",
		Earned:    12,
		DaysSpent: 3,
	}); err != nil {
		t.Fatal(err)
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}

	lines := readJSONL(t, path)
	if len(lines) != 3 {
		t.Fatalf("lines = %d, want 3 (header, call, turn)", len(lines))
	}
	if got := lines[0]["kind"]; got != kindHeader {
		t.Errorf("header kind = %v, want %s", got, kindHeader)
	}
	if got := lines[0]["engine_seed"]; got != float64(1) {
		t.Errorf("engine_seed = %v, want 1", got)
	}
	if got := lines[1]["kind"]; got != kindCall {
		t.Errorf("call kind = %v, want %s", got, kindCall)
	}
	if _, ok := lines[1]["messages"]; !ok {
		t.Error("call line missing messages")
	}

	turn := lines[2]
	if got := turn["kind"]; got != kindTurn {
		t.Errorf("turn kind = %v, want %s", got, kindTurn)
	}
	if turn["action"] != "deliver_package" {
		t.Errorf("action = %v", turn["action"])
	}
	if turn["earned"] != float64(12) {
		t.Errorf("earned = %v, want 12", turn["earned"])
	}
	if turn["days_spent"] != float64(3) {
		t.Errorf("days_spent = %v, want 3", turn["days_spent"])
	}
	if turn["error"] != nil {
		t.Errorf("error = %v, want null", turn["error"])
	}
	args, ok := turn["args"].(map[string]any)
	if !ok || len(args) != 0 {
		t.Errorf("args = %v, want {}", turn["args"])
	}
}

func TestRecordTurnRejected(t *testing.T) {
	tr, path := testTranscript(t)
	msg := `unknown action "fly"`
	if err := tr.RecordTurn(TurnRecord{
		Turn:    1,
		Day:     0,
		Balance: 0,
		Action:  "fly",
		Error:   &msg,
	}); err != nil {
		t.Fatal(err)
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	lines := readJSONL(t, path)
	turn := lines[1]
	if turn["kind"] != kindTurn {
		t.Errorf("kind = %v", turn["kind"])
	}
	if turn["error"] != msg {
		t.Errorf("error = %v, want %q", turn["error"], msg)
	}
	if turn["earned"] != float64(0) {
		t.Errorf("earned = %v, want 0", turn["earned"])
	}
}

func TestParseTurnArgs(t *testing.T) {
	if got := parseTurnArgs(nil); len(got) != 0 {
		t.Errorf("nil = %v, want {}", got)
	}
	if got := parseTurnArgs(json.RawMessage(`not json`)); got["args_raw"] != "not json" {
		t.Errorf("invalid = %v, want args_raw", got)
	}
	got := parseTurnArgs(json.RawMessage(`{"quest_id":"q-d"}`))
	if got["quest_id"] != "q-d" {
		t.Errorf("quest_id = %v", got["quest_id"])
	}
}

func testTranscript(t *testing.T) (*Transcript, string) {
	t.Helper()
	tr, err := NewTranscript(t.TempDir(), testConfig(10), 1)
	if err != nil {
		t.Fatal(err)
	}
	return tr, tr.Path()
}

func readJSONL(t *testing.T, path string) []map[string]any {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(body), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatalf("unmarshal %s: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

func TestRecordInvalidToolArgs(t *testing.T) {
	tr, path := testTranscript(t)
	if err := tr.Record(nil, Result{
		ToolCalls: []ToolCall{{ID: "c1", Name: "fly", Arguments: json.RawMessage(`{not json`)}},
		Usage:     Usage{InputTokens: 3, OutputTokens: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	lines := readJSONL(t, path)
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2 (header + call that must not be dropped)", len(lines))
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	log, err := LoadTranscript(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(log.Calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(log.Calls))
	}
	if log.Calls[0].Usage.InputTokens != 3 {
		t.Errorf("tokens = %d, want 3", log.Calls[0].Usage.InputTokens)
	}
}

func TestRecordLoadCallRoundTrip(t *testing.T) {
	tr, path := testTranscript(t)
	res := Result{
		ToolCalls: []ToolCall{{
			ID:        "c1",
			Name:      "deliver",
			Arguments: json.RawMessage(`{"reasoning":"short is 30/day"}`),
		}},
		Usage: Usage{InputTokens: 10, OutputTokens: 4, TotalTokens: 14},
	}
	if err := tr.Record([]Message{{Role: RoleUser, Content: "hi"}}, res); err != nil {
		t.Fatal(err)
	}
	if err := tr.RecordTurn(TurnRecord{
		Turn: 1, Day: 2, Balance: 60, Action: "deliver", Earned: 60, DaysSpent: 2,
	}); err != nil {
		t.Fatal(err)
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	log, err := LoadTranscript(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(log.Calls) != 1 || log.Calls[0].Usage.InputTokens != 10 || log.Calls[0].Usage.OutputTokens != 4 {
		t.Errorf("calls = %+v", log.Calls)
	}
	if len(log.Turns) != 1 {
		t.Fatalf("turns = %d", len(log.Turns))
	}
	if log.Turns[0].Reasoning != "short is 30/day" {
		t.Errorf("reasoning = %q", log.Turns[0].Reasoning)
	}
}

func TestLoadTranscriptPairsReasoningsByCall(t *testing.T) {
	src := `{"kind":"header"}
{"kind":"turn","turn":1,"day":0,"balance":0,"action":"fly","args":{},"earned":0,"days_spent":0,"error":"bad"}
{"kind":"call","result":{"tool_calls":[{"Name":"deliver","Arguments":{"reasoning":"take short"}}]}}
{"kind":"turn","turn":2,"day":2,"balance":60,"action":"deliver","args":{},"earned":60,"days_spent":2,"error":null}
`
	log, err := LoadTranscript(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(log.Turns) != 2 {
		t.Fatalf("turns = %d", len(log.Turns))
	}
	if log.Turns[0].Reasoning != "" {
		t.Errorf("turn 1 (no preceding call) got reasoning %q", log.Turns[0].Reasoning)
	}
	if log.Turns[1].Reasoning != "take short" {
		t.Errorf("turn 2 reasoning = %q, want take short", log.Turns[1].Reasoning)
	}
}

func TestLoadTranscriptPopsTwoReasoningsFromOneCall(t *testing.T) {
	src := `{"kind":"header"}
{"kind":"call","result":{"tool_calls":[{"Name":"accept_quest","Arguments":{"reasoning":"first"}},{"Name":"accept_quest","Arguments":{"reasoning":"second"}}]}}
{"kind":"turn","turn":1,"day":0,"balance":0,"action":"accept_quest","args":{},"earned":0,"days_spent":0,"error":null}
{"kind":"turn","turn":2,"day":0,"balance":0,"action":"accept_quest","args":{},"earned":0,"days_spent":0,"error":"already on a quest"}
`
	log, err := LoadTranscript(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(log.Turns) != 2 {
		t.Fatalf("turns = %d", len(log.Turns))
	}
	if log.Turns[0].Reasoning != "first" || log.Turns[1].Reasoning != "second" {
		t.Errorf("reasonings = %q, %q", log.Turns[0].Reasoning, log.Turns[1].Reasoning)
	}
}

func TestLoadTranscriptTruncated(t *testing.T) {
	src := `{"kind":"header","model":"m"}
{"kind":"turn","turn":1,"day":0,"balance":0,"action":"refresh_board","args":{},"earned":0,"days_spent":1,"error":null}
{"kind":"turn","turn":2,"day":1,"balanc`
	log, err := LoadTranscript(strings.NewReader(src))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "line 3") {
		t.Errorf("error = %v, want line 3", err)
	}
	if log == nil || len(log.Turns) != 1 {
		t.Fatalf("turns = %+v, want 1 parsed so far", log)
	}
	lines, rerr := Replay(strings.NewReader(src))
	if rerr == nil {
		t.Fatal("replay expected error")
	}
	if len(lines) != 1 {
		t.Fatalf("replay lines = %d, want 1", len(lines))
	}
}

func TestLoadTranscriptMissingKind(t *testing.T) {
	src := `{"messages":[{"Role":"user","Content":"hi"}]}
`
	_, err := LoadTranscript(strings.NewReader(src))
	if err == nil || !strings.Contains(err.Error(), "missing kind") {
		t.Fatalf("err = %v, want missing kind", err)
	}
	if !strings.Contains(err.Error(), "pre-#29") {
		t.Errorf("error should mention pre-#29: %v", err)
	}
}

// The header carries the model's price at run time, and Stats reads it back.
func TestHeaderRecordsPricing(t *testing.T) {
	cfg := &Config{Provider: "openrouter", Model: "m", Pricing: map[string]*Pricing{
		"m": {InputPerMillion: 0.1, OutputPerMillion: 0.32}, "local": nil,
	}}
	var buf bytes.Buffer
	b, err := json.Marshal(transcriptHeader(cfg, 1))
	if err != nil {
		t.Fatal(err)
	}
	buf.Write(append(b, '\n'))
	log, err := LoadTranscript(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if p := log.Stats().Pricing; p == nil || *p != (Pricing{InputPerMillion: 0.1, OutputPerMillion: 0.32}) {
		t.Errorf("pricing = %+v, want the run-time price", p)
	}
	cfg.Model = "local"
	if _, ok := transcriptHeader(cfg, 1)["pricing"]; ok {
		t.Error("an unpriced model must not record pricing")
	}
}
