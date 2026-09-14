package harness

import (
	"strings"
	"testing"
)

func TestFormatTurnIssueExample(t *testing.T) {
	invalid := "already on a quest"
	tests := []struct {
		name      string
		rec       TurnRecord
		reasoning string
		want      string
	}{
		{
			name: "deliver",
			rec: TurnRecord{
				Day: 14, Balance: 380, Action: "deliver",
				Args: map[string]any{}, Earned: 100, DaysSpent: 3,
			},
			want: "day 14  bal 380  deliver             +100  (3 days)",
		},
		{
			name: "fight with reasoning",
			rec: TurnRecord{
				Day: 17, Balance: 480, Action: "fight",
				Args: map[string]any{}, Earned: 120, DaysSpent: 2,
			},
			reasoning: "balance over 150, fight unlocked",
			want:      `day 17  bal 480  fight               +120  (2 days)   reasoning: "balance over 150, fight unlocked"`,
		},
		{
			name: "rejected accept",
			rec: TurnRecord{
				Day: 19, Balance: 600, Action: "accept_quest",
				Args:  map[string]any{"quest_id": "q7"},
				Error: &invalid,
			},
			want: "day 19  bal 600  accept_quest q7     invalid: already on a quest",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTurn(tt.rec, tt.reasoning)
			if got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestFormatTurnOneDay(t *testing.T) {
	got := FormatTurn(TurnRecord{
		Day: 1, Balance: 0, Action: "refresh_board",
		Args: map[string]any{}, Earned: 0, DaysSpent: 1,
	}, "")
	want := "day 1  bal 0  refresh_board       +0  (1 day)"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestReplayReadsTurnRecords(t *testing.T) {
	const src = `
{"kind":"header","provider":"test","model":"test-model","seed":1}
{"kind":"call","result":{"tool_calls":[{"Name":"deliver","Arguments":{}}],"usage":{"InputTokens":10,"OutputTokens":2,"TotalTokens":12}}}
{"kind":"turn","turn":1,"day":14,"balance":380,"action":"deliver","args":{},"earned":100,"days_spent":3,"error":null}
{"kind":"call","result":{"tool_calls":[{"Name":"fight","Arguments":{"reasoning":"balance over 150, fight unlocked"}}],"usage":{"InputTokens":11,"OutputTokens":3,"TotalTokens":14}}}
{"kind":"turn","turn":2,"day":17,"balance":480,"action":"fight","args":{},"earned":120,"days_spent":2,"error":null}
{"kind":"call","result":{"tool_calls":[{"Name":"accept_quest","Arguments":{"quest_id":"q7"}}],"usage":{"InputTokens":12,"OutputTokens":4,"TotalTokens":16}}}
{"kind":"turn","turn":3,"day":19,"balance":600,"action":"accept_quest","args":{"quest_id":"q7"},"earned":0,"days_spent":0,"error":"already on a quest"}
`
	lines, err := Replay(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"day 14  bal 380  deliver             +100  (3 days)",
		`day 17  bal 480  fight               +120  (2 days)   reasoning: "balance over 150, fight unlocked"`,
		"day 19  bal 600  accept_quest q7     invalid: already on a quest",
	}
	if len(lines) != len(want) {
		t.Fatalf("lines = %d, want %d: %#v", len(lines), len(want), lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d:\n got %s\nwant %s", i, lines[i], want[i])
		}
	}

	log, err := LoadTranscript(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	st := log.Stats()
	if st.Model != "test-model" {
		t.Errorf("model = %q", st.Model)
	}
	if st.FinalBalance != 600 {
		t.Errorf("final balance = %d, want 600", st.FinalBalance)
	}
	if st.ModelCalls != 3 {
		t.Errorf("model calls = %d, want 3", st.ModelCalls)
	}
	if st.InputTokens != 33 || st.OutputTokens != 9 {
		t.Errorf("tokens = %d/%d, want 33/9", st.InputTokens, st.OutputTokens)
	}
	if st.InvalidActions != 1 {
		t.Errorf("invalid = %d, want 1", st.InvalidActions)
	}
}

func TestReplayBotTurnsHaveNoReasoning(t *testing.T) {
	const src = `
{"kind":"header","provider":"bot","model":"bot"}
{"kind":"turn","turn":1,"day":0,"balance":0,"action":"accept_quest","args":{"quest_id":"q-1-0-0"},"earned":0,"days_spent":0,"error":null}
{"kind":"turn","turn":2,"day":2,"balance":60,"action":"deliver","args":{},"earned":60,"days_spent":2,"error":null}
`
	lines, err := Replay(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(lines))
	}
	if strings.Contains(lines[0], "reasoning") {
		t.Errorf("bot line has reasoning: %s", lines[0])
	}
	if !strings.Contains(lines[0], "accept_quest q-1-0-0") {
		t.Errorf("missing quest id: %s", lines[0])
	}
	if !strings.Contains(lines[1], "+60  (2 days)") {
		t.Errorf("deliver line: %s", lines[1])
	}
}

func TestLoadTranscriptSkipsUnknownKinds(t *testing.T) {
	const src = `
{"kind":"header","model":"x"}
{"kind":"future","n":1}
{"kind":"turn","turn":1,"day":0,"balance":0,"action":"refresh_board","args":{},"earned":0,"days_spent":1,"error":null}
`
	log, err := LoadTranscript(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(log.Turns) != 1 || log.Turns[0].Action != "refresh_board" {
		t.Fatalf("turns = %+v", log.Turns)
	}
}

func TestReplayEmpty(t *testing.T) {
	lines, err := Replay(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 0 {
		t.Errorf("lines = %v, want empty", lines)
	}
}
