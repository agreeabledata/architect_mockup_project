package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"architect_mockup_project/engine"
	"architect_mockup_project/harness"
)

func TestParseSeeds(t *testing.T) {
	got, err := parseSeeds("1,2,3")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Fatalf("got %v", got)
	}
	got, err = parseSeeds(" 1, 2 ")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1] != 2 {
		t.Fatalf("got %v", got)
	}
	if _, err := parseSeeds(""); err == nil {
		t.Fatal("expected error")
	}
	if _, err := parseSeeds("nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestFormatTable(t *testing.T) {
	got := formatTable([]row{
		{model: "bot", seeds: "1,2,3", meanBal: 1650, gap: 0, finished: 3, total: 3, costOK: true},
		{model: "llama3.2", seeds: "1,2,3", meanBal: 1390, gap: -260, finished: 3, total: 3, calls: 41, inTok: 50000, outTok: 2000, invalid: 4, cost: 0.0123, costOK: true},
	})
	if !strings.Contains(got, "| model | seeds | mean final balance | gap to bot | finished | model calls | input tok | output tok | invalid actions | est. cost |") {
		t.Fatalf("missing header:\n%s", got)
	}
	if !strings.Contains(got, "| bot | 1,2,3 | 1650 | +0 | 3/3 | 0 | 0 | 0 | 0 | $0.0000 |") {
		t.Fatalf("bot row:\n%s", got)
	}
	if !strings.Contains(got, "| llama3.2 | 1,2,3 | 1390 | -260 | 3/3 | 41 | 50000 | 2000 | 4 | $0.0123 |") {
		t.Fatalf("model row:\n%s", got)
	}
}

func TestAggregateMeanAndCost(t *testing.T) {
	cfg := &harness.Config{Model: "m", Pricing: map[string]*harness.Pricing{
		"m": {InputPerMillion: 1, OutputPerMillion: 2},
	}}
	r := aggregate("m", []uint32{1, 2}, []harness.RunStats{
		{FinalBalance: 100, ModelCalls: 2, InputTokens: 500_000, OutputTokens: 0, InvalidActions: 1, Reason: "terminal"},
		{FinalBalance: 200, ModelCalls: 3, InputTokens: 0, OutputTokens: 500_000, InvalidActions: 0, Reason: "terminal"},
	}, cfg, 180)
	if r.meanBal != 150 {
		t.Errorf("mean = %v, want 150", r.meanBal)
	}
	if r.gap != -30 {
		t.Errorf("gap = %v, want -30", r.gap)
	}
	if r.calls != 5 || r.invalid != 1 {
		t.Errorf("calls/invalid = %d/%d", r.calls, r.invalid)
	}
	if r.cost != 0.5+1.0 {
		t.Errorf("cost = %v, want 1.5", r.cost)
	}
	if !r.costOK || r.finished != 2 || r.total != 2 {
		t.Errorf("costOK/finished = %v %d/%d", r.costOK, r.finished, r.total)
	}

	unpriced := aggregate("x", []uint32{1}, []harness.RunStats{
		{FinalBalance: 10, Reason: "no_action"},
	}, &harness.Config{Model: "x"}, 0)
	if unpriced.costOK {
		t.Error("missing pricing must not be costOK")
	}
	if formatCost(unpriced) != "n/a" {
		t.Errorf("cost cell = %s, want n/a", formatCost(unpriced))
	}
	if unpriced.finished != 0 || unpriced.nonterm != "no_action" {
		t.Errorf("finished/nonterm = %d %q", unpriced.finished, unpriced.nonterm)
	}
}

func TestCompareBotOnly(t *testing.T) {
	dir := t.TempDir()
	var out, errb bytes.Buffer
	if err := runCompare(context.Background(), []uint32{1}, nil, dir, &out, &errb); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "| bot | 1 | 1820 | +0 | 1/1 | 0 | 0 | 0 | 0 | $0.0000 |") {
		t.Fatalf("table:\n%s\nstderr:\n%s", got, errb.String())
	}
	if !strings.Contains(errb.String(), "bot  seed 1  balance 1820") {
		t.Errorf("stderr = %s", errb.String())
	}

	w := engine.New(2)
	engine.DriveBot(w)
	want2 := w.Observe().Balance

	out.Reset()
	errb.Reset()
	if err := runCompare(context.Background(), []uint32{1, 2}, []string{"bot"}, dir, &out, &errb); err != nil {
		t.Fatal(err)
	}
	mean := float64(1820+want2) / 2
	wantRow := "| bot | 1,2 | " + formatNum(mean) + " | +0 |"
	if !strings.Contains(out.String(), wantRow) {
		t.Fatalf("two-seed table:\n%s\nwant substring %s", out.String(), wantRow)
	}
}

func TestCompareUnknownConfig(t *testing.T) {
	err := runCompare(context.Background(), []uint32{1}, []string{filepath.Join(t.TempDir(), "missing.json")}, t.TempDir(), io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCompareReadsTranscriptNumbers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fake.jsonl")
	body := `{"kind":"header","model":"m"}
{"kind":"call","result":{"tool_calls":[{"Name":"fight","Arguments":{}}],"usage":{"InputTokens":10,"OutputTokens":4,"TotalTokens":14}}}
{"kind":"turn","turn":1,"day":2,"balance":100,"action":"fight","args":{},"earned":100,"days_spent":2,"error":null}
{"kind":"turn","turn":2,"day":2,"balance":100,"action":"fly","args":{},"earned":0,"days_spent":0,"error":"unknown action"}
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := statsFromPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.FinalBalance != 100 || st.ModelCalls != 1 || st.InputTokens != 10 || st.OutputTokens != 4 || st.InvalidActions != 1 {
		t.Errorf("stats = %+v", st)
	}
}

type stubProvider struct{}

func (stubProvider) Chat(_ context.Context, _ []harness.Message, _ []harness.Tool, _ harness.CallOptions) (harness.Result, error) {
	return harness.Result{Content: "nope", StopReason: harness.StopReasonStop}, nil
}

func TestCompareModelPathNoAction(t *testing.T) {
	orig := newProvider
	t.Cleanup(func() { newProvider = orig })
	newProvider = func(*harness.Config) (harness.Provider, error) {
		return stubProvider{}, nil
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "m.json")
	if err := os.WriteFile(cfgPath, []byte(`{"provider":"ollama","model":"fake-model","seed":99,"iteration_cap":10}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	if err := runCompare(context.Background(), []uint32{1}, []string{cfgPath}, dir, &out, &errb); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "fake-model") {
		t.Fatalf("table missing model:\n%s", got)
	}
	if !strings.Contains(got, "| 0/1 |") {
		t.Fatalf("finished column:\n%s", got)
	}
	if !strings.Contains(got, "n/a") {
		t.Fatalf("unpriced cost:\n%s", got)
	}
	if !strings.Contains(got, "ended at no_action") {
		t.Fatalf("footnote:\n%s", got)
	}
	if !strings.Contains(errb.String(), "no_action") {
		t.Errorf("stderr = %s", errb.String())
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var sawModel bool
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		f, err := os.Open(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		log, err := harness.LoadTranscript(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if log.Header["model"] != "fake-model" {
			continue
		}
		sawModel = true
		if log.Header["seed"] != float64(99) {
			t.Errorf("sampling seed clobbered: %v (want 99)", log.Header["seed"])
		}
		if log.Header["engine_seed"] != float64(1) {
			t.Errorf("engine_seed = %v, want 1", log.Header["engine_seed"])
		}
	}
	if !sawModel {
		t.Fatal("no model transcript written")
	}
}

func TestBuildFromTableAndAnalysis(t *testing.T) {
	dir := t.TempDir()
	writeSeed(t, filepath.Join(dir, "bot", "seed-1.jsonl"), "bot", 1, []string{
		turnLine(2, 0, "refresh_board", 0, false),
		turnLine(50, 1000, "deliver", 60, false),
	})
	writeSeed(t, filepath.Join(dir, "bot", "seed-2.jsonl"), "bot", 2, []string{
		turnLine(50, 2000, "deliver", 60, false),
	})
	writeSeed(t, filepath.Join(dir, "priced", "seed-1.jsonl"), "m", 1, []string{
		callLine(1_000_000, 0),
		turnLine(2, 70, "fight", 0, false),
		turnLine(4, 70, "fly", 0, true),
		turnLine(50, 800, "refresh_board", 0, false),
	})
	writeSeed(t, filepath.Join(dir, "priced", "seed-2.jsonl"), "m", 2, []string{
		turnLine(50, 900, "refresh_board", 0, false),
	})
	writeSeed(t, filepath.Join(dir, "unpriced", "seed-1.jsonl"), "x", 1, []string{
		turnLine(50, 1, "deliver", 1, false),
	})
	writeSeed(t, filepath.Join(dir, "unpriced", "seed-2.jsonl"), "x", 2, []string{
		turnLine(50, 1, "deliver", 1, false),
	})
	// These would fail the day-50 check if -from scored them.
	writeSeed(t, filepath.Join(dir, "failed", "m", "seed-9.jsonl"), "m", 9, []string{
		turnLine(10, 99999, "deliver", 1, false),
	})
	writeSeed(t, filepath.Join(dir, "archive", "seed-1.jsonl"), "old", 1, []string{
		turnLine(3, 99999, "deliver", 1, false),
	})
	if err := os.WriteFile(filepath.Join(dir, "failed", "NOTES.md"), []byte("m seed 9: no_action\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pricing := filepath.Join(dir, "pricing.json")
	if err := os.WriteFile(pricing, []byte(`{"pricing":{"m":{"input_per_million":0,"output_per_million":0}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := buildFrom(dir, pricing)
	if err != nil {
		t.Fatal(err)
	}
	again, err := buildFrom(dir, pricing)
	if err != nil {
		t.Fatal(err)
	}
	if got != again {
		t.Fatal("buildFrom is not stable")
	}
	for _, want := range []string{
		"| bot | 1,2 | 1500 | +0 | 2/2 | 0 | 0 | 0 | 0 | $0.0000 |",
		"| m | 1,2 | 850 | -650 | 2/2 | 1 | 1000000 | 0 | 1 | $0.0000 |",
		"| x | 1,2 | 1 | -1499 | 2/2 | 0 | 0 | 0 | 0 | n/a |",
		"m averaged 850 (gap -650). It fought under 150 krones 1 time (seeds 1), refreshed the board 2 times versus the bot's 1 time on the same seeds, and recorded 1 invalid action.",
		"x averaged 1 (gap -1499). It fought under 150 krones 0 times, refreshed the board 0 times versus the bot's 1 time on the same seeds, and recorded 0 invalid actions.",
		"Every scored run reached day 50.",
		"## Failed runs\n\nm seed 9: no_action\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q\n%s", want, got)
		}
	}
	if strings.Contains(got, "99999") {
		t.Fatalf("unscored transcript leaked into the README:\n%s", got)
	}
	if strings.Count(got, "\n## Analysis\n") != 1 {
		t.Fatalf("analysis heading:\n%s", got)
	}
}

func TestBuildFromRejectsIncompleteDuplicateAndMismatch(t *testing.T) {
	pricing := func(dir string) string {
		t.Helper()
		path := filepath.Join(dir, "pricing.json")
		if err := os.WriteFile(path, []byte(`{"pricing":{}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	t.Run("short", func(t *testing.T) {
		dir := t.TempDir()
		writeSeed(t, filepath.Join(dir, "bot", "seed-1.jsonl"), "bot", 1, []string{
			turnLine(16, 330, "refresh_board", 0, false),
		})
		_, err := buildFrom(dir, pricing(dir))
		if err == nil || !strings.Contains(err.Error(), "move it under failed/") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		dir := t.TempDir()
		body := []string{turnLine(50, 1, "deliver", 1, false)}
		writeSeed(t, filepath.Join(dir, "bot", "seed-1.jsonl"), "bot", 1, body)
		writeSeed(t, filepath.Join(dir, "copy", "seed-1.jsonl"), "bot", 1, body)
		_, err := buildFrom(dir, pricing(dir))
		if err == nil || !strings.Contains(err.Error(), "duplicate bot seed 1") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("filename", func(t *testing.T) {
		dir := t.TempDir()
		writeSeed(t, filepath.Join(dir, "bot", "seed-2.jsonl"), "bot", 1, []string{
			turnLine(50, 1, "deliver", 1, false),
		})
		_, err := buildFrom(dir, pricing(dir))
		if err == nil || !strings.Contains(err.Error(), "does not match filename seed") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("seed set", func(t *testing.T) {
		dir := t.TempDir()
		writeSeed(t, filepath.Join(dir, "bot", "seed-1.jsonl"), "bot", 1, []string{
			turnLine(50, 1, "deliver", 1, false),
		})
		writeSeed(t, filepath.Join(dir, "m", "seed-2.jsonl"), "m", 2, []string{
			turnLine(50, 1, "deliver", 1, false),
		})
		_, err := buildFrom(dir, pricing(dir))
		if err == nil || !strings.Contains(err.Error(), "do not match bot seeds") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("no bot", func(t *testing.T) {
		dir := t.TempDir()
		writeSeed(t, filepath.Join(dir, "m", "seed-1.jsonl"), "m", 1, []string{
			turnLine(50, 1, "deliver", 1, false),
		})
		_, err := buildFrom(dir, pricing(dir))
		if err == nil || !strings.Contains(err.Error(), "no bot transcripts") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestWriteFrom(t *testing.T) {
	dir := t.TempDir()
	writeSeed(t, filepath.Join(dir, "bot", "seed-1.jsonl"), "bot", 1, []string{
		turnLine(50, 1820, "deliver", 60, false),
	})
	pricing := filepath.Join(dir, "pricing.json")
	if err := os.WriteFile(pricing, []byte(`{"pricing":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeFrom(dir, pricing); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "The bot is the only scored row. Every scored run reached day 50.\n") {
		t.Fatalf("readme:\n%s", b)
	}
}

func writeSeed(t *testing.T, path, model string, seed int, lines []string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "{\"kind\":\"header\",\"provider\":%q,\"model\":%q,\"engine_seed\":%d}\n", model, model, seed)
	for _, line := range lines {
		b.WriteString(line)
		if !strings.HasSuffix(line, "\n") {
			b.WriteByte('\n')
		}
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func turnLine(day, balance int, action string, earned int, invalid bool) string {
	errField := "null"
	if invalid {
		errField = `"unknown action"`
	}
	return fmt.Sprintf(`{"kind":"turn","turn":1,"day":%d,"balance":%d,"action":%q,"args":{},"earned":%d,"days_spent":1,"error":%s}`, day, balance, action, earned, errField)
}

func callLine(in, out int) string {
	return fmt.Sprintf(`{"kind":"call","result":{"tool_calls":[],"usage":{"InputTokens":%d,"OutputTokens":%d,"TotalTokens":%d}}}`, in, out, in+out)
}

// A run's recorded price wins over the config's, so repricing the config
// cannot re-price it; runs without one fall back to the config.
func TestAggregateUsesRecordedPricing(t *testing.T) {
	cfg := &harness.Config{Pricing: map[string]*harness.Pricing{"m": {InputPerMillion: 100}}}
	r := aggregate("m", []uint32{1, 2}, []harness.RunStats{
		{InputTokens: 1_000_000, Pricing: &harness.Pricing{InputPerMillion: 1}},
		{InputTokens: 1_000_000},
	}, cfg, 0)
	if !r.costOK || r.cost != 1+100 {
		t.Errorf("cost = %v (ok=%v), want 101: recorded 1 + config fallback 100", r.cost, r.costOK)
	}
	recordedOnly := aggregate("m", []uint32{1}, []harness.RunStats{
		{InputTokens: 1_000_000, Pricing: &harness.Pricing{InputPerMillion: 1}},
	}, &harness.Config{}, 0)
	if !recordedOnly.costOK || recordedOnly.cost != 1 {
		t.Errorf("cost = %v (ok=%v), want 1 with no config entry", recordedOnly.cost, recordedOnly.costOK)
	}
}
