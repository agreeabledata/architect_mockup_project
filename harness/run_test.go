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
	if log.Header["engine_seed"] != float64(1) {
		t.Errorf("engine_seed = %v, want 1", log.Header["engine_seed"])
	}
	if _, ok := log.Header["prompt_version"]; ok {
		t.Errorf("bot header should omit prompt_version: %v", log.Header)
	}
	if _, ok := log.Header["iteration_cap"]; ok {
		t.Errorf("bot header should omit iteration_cap: %v", log.Header)
	}
	if _, ok := log.Header["config_hash"]; ok {
		t.Errorf("bot header should omit config_hash: %v", log.Header)
	}
	if len(log.Calls) != 0 {
		t.Errorf("bot recorded %d model calls, want 0", len(log.Calls))
	}
	if len(log.Turns) == 0 {
		t.Fatal("no turns")
	}
	st := log.Stats()
	if st.FinalBalance != 1640 {
		t.Errorf("final balance = %d, want 1640", st.FinalBalance)
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
