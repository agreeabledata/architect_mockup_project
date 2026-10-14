package visual

import (
	"os"
	"strings"
	"testing"

	"architect_mockup_project/engine"
	"architect_mockup_project/harness"
)

// TestReplayBoardMatchesEngine replays a real bot run two ways and requires the
// visualizer's per-turn board to agree with the engine step for step: same
// quest IDs and same active quest. This is the proof that the replayed board is
// honest, not a reconstruction that can drift.
func TestReplayBoardMatchesEngine(t *testing.T) {
	dir := t.TempDir()
	path, err := harness.RunBotEpisode(1, dir)
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

	sc := BuildScene(log)
	if len(sc.Frames) == 0 {
		t.Fatal("no frames from a real bot run")
	}
	if sc.DivergedAt != 0 {
		t.Errorf("a fresh bot run diverged from its own replay at turn %d", sc.DivergedAt)
	}
	if sc.FinalBal != 1820 {
		t.Errorf("final balance = %d, want the seed-1 bot baseline 1820", sc.FinalBal)
	}

	w := engine.New(1)
	for i, fr := range sc.Frames {
		if fr.Board == nil {
			t.Fatalf("frame %d: no board (engine_seed missing from header)", i)
		}
		if got, want := questIDs(fr.Board), engineQuestIDs(w.Board()); !equal(got, want) {
			t.Fatalf("frame %d board IDs = %v, engine has %v", i, got, want)
		}
		if fr.Active != engineActiveID(w.Board()) {
			t.Fatalf("frame %d active = %q, engine has %q", i, fr.Active, engineActiveID(w.Board()))
		}
		if len(fr.Legal) == 0 {
			t.Fatalf("frame %d: no legal actions recorded", i)
		}
		if _, err := w.StepBot(); err != nil {
			t.Fatal(err)
		}
	}

	changed := 0
	for i := 1; i < len(sc.Frames); i++ {
		if !equal(questIDs(sc.Frames[i-1].Board), questIDs(sc.Frames[i].Board)) {
			changed++
		}
	}
	if changed == 0 {
		t.Error("board never changed; expected at least one refresh in a seed-1 bot run")
	}
}

// A rejected action must not move the board on the following frame.
func TestRejectedActionLeavesBoardUnchanged(t *testing.T) {
	log, err := harness.LoadTranscript(strings.NewReader(`
{"kind":"header","provider":"bot","model":"bot","engine_seed":1}
{"kind":"turn","turn":1,"day":0,"balance":0,"action":"accept_quest","args":{"quest_id":"nope"},"earned":0,"days_spent":0,"error":"no open quest \"nope\""}
{"kind":"turn","turn":2,"day":1,"balance":0,"action":"refresh_board","args":{},"earned":0,"days_spent":1,"error":null}
`))
	if err != nil {
		t.Fatal(err)
	}
	sc := BuildScene(log)
	if len(sc.Frames) != 2 {
		t.Fatalf("frames = %d, want 2", len(sc.Frames))
	}
	if !equal(questIDs(sc.Frames[0].Board), questIDs(sc.Frames[1].Board)) {
		t.Fatal("a rejected action changed the board shown on the next frame")
	}
	if sc.DivergedAt != 0 {
		t.Errorf("diverged at turn %d; the record matches the engine", sc.DivergedAt)
	}
}

// A record the engine no longer reproduces is flagged at its turn, and later
// frames drop the board rather than show one the agent never saw.
func TestReplayFlagsDivergence(t *testing.T) {
	for name, turn2 := range map[string]string{
		"balance":  `"day":1,"balance":5,"action":"refresh_board","args":{},"earned":5,"days_spent":1,"error":null`,
		"day":      `"day":3,"balance":0,"action":"refresh_board","args":{},"earned":0,"days_spent":3,"error":null`,
		"rejected": `"day":1,"balance":0,"action":"refresh_board","args":{},"earned":0,"days_spent":0,"error":"rejected"`,
		"accepted": `"day":0,"balance":0,"action":"accept_quest","args":{"quest_id":"nope"},"earned":0,"days_spent":1,"error":null`,
	} {
		log, err := harness.LoadTranscript(strings.NewReader(`
{"kind":"header","provider":"bot","model":"bot","engine_seed":1}
{"kind":"turn","turn":1,"day":1,"balance":0,"action":"refresh_board","args":{},"earned":0,"days_spent":1,"error":null}
{"kind":"turn","turn":2,` + turn2 + `}
{"kind":"turn","turn":3,"day":2,"balance":0,"action":"refresh_board","args":{},"earned":0,"days_spent":1,"error":null}
`))
		if err != nil {
			t.Fatal(err)
		}
		sc := BuildScene(log)
		if sc.DivergedAt != 2 {
			t.Errorf("%s: diverged at %d, want 2", name, sc.DivergedAt)
		}
		if sc.Frames[1].Board == nil || sc.Frames[2].Board != nil {
			t.Errorf("%s: want a board through the diverging turn and none after it", name)
		}
	}
}

func questIDs(board []QuestView) []string {
	out := make([]string, len(board))
	for i, q := range board {
		out[i] = q.ID
	}
	return out
}

func engineQuestIDs(board []engine.Quest) []string {
	out := make([]string, len(board))
	for i, q := range board {
		out[i] = q.ID
	}
	return out
}

func engineActiveID(board []engine.Quest) string {
	for _, q := range board {
		if q.Status == engine.QuestActive {
			return q.ID
		}
	}
	return ""
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
