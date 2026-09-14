package harness

import (
	"os"
	"testing"
)

func TestRunBotEpisodeSeed1(t *testing.T) {
	path, err := RunBotEpisode(1, t.TempDir())
	if err != nil {
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
	if log.Header["model"] != "bot" || log.Header["provider"] != "bot" {
		t.Errorf("header = %v", log.Header)
	}
	if len(log.Calls) != 0 {
		t.Errorf("bot recorded %d model calls, want 0", len(log.Calls))
	}
	if len(log.Turns) == 0 {
		t.Fatal("no turns")
	}
	st := log.Stats()
	if st.FinalBalance != 1650 {
		t.Errorf("final balance = %d, want 1650", st.FinalBalance)
	}
	if st.InvalidActions != 0 {
		t.Errorf("invalid actions = %d, want 0", st.InvalidActions)
	}
	if st.ModelCalls != 0 || st.InputTokens != 0 {
		t.Errorf("calls/tokens = %d %d", st.ModelCalls, st.InputTokens)
	}

	replay, err := Replay(mustOpen(t, path))
	if err != nil {
		t.Fatal(err)
	}
	if len(replay) != len(log.Turns) {
		t.Errorf("replay lines = %d, turns = %d", len(replay), len(log.Turns))
	}
	last := replay[len(replay)-1]
	if last == "" {
		t.Error("empty last replay line")
	}
}

func mustOpen(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}
