package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"architect_mockup_project/harness"
)

const fixture = `
{"kind":"header","provider":"test","model":"test-model","engine_seed":1}
{"kind":"call","result":{"tool_calls":[{"Name":"deliver","Arguments":{}}],"usage":{"InputTokens":10,"OutputTokens":2,"TotalTokens":12}}}
{"kind":"turn","turn":1,"day":14,"balance":380,"action":"deliver","args":{},"earned":100,"days_spent":3,"error":null}
{"kind":"call","result":{"tool_calls":[{"Name":"accept_quest","Arguments":{"quest_id":"q7","reasoning":"already on a quest"}}],"usage":{"InputTokens":11,"OutputTokens":3,"TotalTokens":14}}}
{"kind":"turn","turn":2,"day":14,"balance":380,"action":"accept_quest","args":{"quest_id":"q7"},"earned":0,"days_spent":0,"error":"already on a quest"}
`

func TestBuildScene(t *testing.T) {
	log, err := harness.LoadTranscript(strings.NewReader(fixture))
	if err != nil {
		t.Fatal(err)
	}
	sc := buildScene(log)
	if sc.Model != "test-model" || sc.Provider != "test" {
		t.Errorf("header = %q/%q", sc.Model, sc.Provider)
	}
	if sc.EngineSeed != 1 || sc.MaxDay != 50 {
		t.Errorf("seed/maxDay = %d/%d", sc.EngineSeed, sc.MaxDay)
	}
	if sc.FinalBal != 380 {
		t.Errorf("final balance = %d, want 380", sc.FinalBal)
	}
	if sc.TotalEarned != 100 {
		t.Errorf("total earned = %d, want 100 (rejects must not count)", sc.TotalEarned)
	}
	if len(sc.Frames) != 2 {
		t.Fatalf("frames = %d, want 2", len(sc.Frames))
	}
	if f := sc.Frames[0]; f.Action != "deliver" || f.Day != 14 || f.Earned != 100 || f.DaysSpent != 3 || f.QuestID != "" {
		t.Errorf("frame 0 = %+v", f)
	}
	if f := sc.Frames[1]; f.QuestID != "q7" || f.Error != "already on a quest" || f.Reasoning != "already on a quest" {
		t.Errorf("frame 1 = %+v", f)
	}
}

func TestWriteHTMLEmbedsScene(t *testing.T) {
	sc := buildScene(mustLog(t, fixture))
	var b strings.Builder
	if err := writeHTML(&b, sc); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "<canvas") {
		t.Fatal("no canvas element")
	}

	const marker = "const SCENE = "
	i := strings.Index(out, marker)
	if i < 0 {
		t.Fatal("no embedded SCENE")
	}
	rest := out[i+len(marker):]
	nl := strings.IndexByte(rest, '\n')
	if nl < 0 {
		t.Fatal("SCENE line not terminated")
	}
	raw := strings.TrimRight(rest[:nl], "; \t")
	var got scene
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("embedded scene is not valid JSON: %v", err)
	}
	if len(got.Frames) != 2 || got.Frames[1].Reasoning != "already on a quest" {
		t.Fatalf("round-trip frames = %+v", got.Frames)
	}
}

// A "</script>" in model reasoning must not terminate the script block.
func TestWriteHTMLEscapesReasoning(t *testing.T) {
	log := mustLog(t, `
{"kind":"header","provider":"p","model":"m"}
{"kind":"call","result":{"tool_calls":[{"Name":"deliver","Arguments":{"reasoning":"</script><script>alert(1)</script>"}}]}}
{"kind":"turn","turn":1,"day":1,"balance":1,"action":"deliver","args":{},"earned":1,"days_spent":1,"error":null}
`)
	var b strings.Builder
	if err := writeHTML(&b, buildScene(log)); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "</script><script>alert(1)") {
		t.Fatal("reasoning broke out of the script block")
	}
}

func TestSceneFromBotTranscript(t *testing.T) {
	path, err := harness.RunBotEpisode(1, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	log, err := harness.LoadTranscript(f)
	if err != nil {
		t.Fatal(err)
	}
	sc := buildScene(log)
	if len(sc.Frames) == 0 {
		t.Fatal("no frames from a real bot run")
	}
	if sc.FinalBal != 1820 {
		t.Errorf("final balance = %d, want the seed-1 bot baseline 1820", sc.FinalBal)
	}
}

func mustLog(t *testing.T, src string) *harness.TranscriptLog {
	t.Helper()
	log, err := harness.LoadTranscript(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	return log
}
