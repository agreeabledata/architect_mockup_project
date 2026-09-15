package main

import (
	"bytes"
	"context"
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
	if !strings.Contains(got, "| bot | 1 | 1650 | +0 | 1/1 | 0 | 0 | 0 | 0 | $0.0000 |") {
		t.Fatalf("table:\n%s\nstderr:\n%s", got, errb.String())
	}
	if !strings.Contains(errb.String(), "bot  seed 1  balance 1650") {
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
	mean := float64(1650+want2) / 2
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
	if err := os.WriteFile(cfgPath, []byte(`{"provider":"ollama","model":"fake-model","seed":99,"iteration_cap":1}`), 0o644); err != nil {
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
