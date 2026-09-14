package harness

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestTranscriptKinds(t *testing.T) {
	tr, path := testTranscript(t)
	tr.Record([]Message{{Role: RoleUser, Content: "hi"}}, Result{Content: "ok"})
	tr.RecordTurn(TurnRecord{
		Turn:      1,
		Day:       3,
		Balance:   12,
		Action:    "deliver_package",
		Earned:    12,
		DaysSpent: 3,
	})
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
	tr.RecordTurn(TurnRecord{
		Turn:    1,
		Day:     0,
		Balance: 0,
		Action:  "fly",
		Error:   &msg,
	})
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
	if got := parseTurnArgs(json.RawMessage(`not json`)); len(got) != 0 {
		t.Errorf("invalid = %v, want {}", got)
	}
	got := parseTurnArgs(json.RawMessage(`{"quest_id":"q-d"}`))
	if got["quest_id"] != "q-d" {
		t.Errorf("quest_id = %v", got["quest_id"])
	}
}

func testTranscript(t *testing.T) (*Transcript, string) {
	t.Helper()
	tr, err := NewTranscript(t.TempDir(), testConfig(10))
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
