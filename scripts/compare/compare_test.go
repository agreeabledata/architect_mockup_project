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
		{model: "bot", seeds: "1,2,3", meanBal: 1650, gap: 0, calls: 0, cost: 0},
		{model: "llama3.2", seeds: "1,2,3", meanBal: 1390, gap: -260, calls: 41, inTok: 50000, outTok: 2000, invalid: 4, cost: 0.0123},
	})
	if !strings.Contains(got, "| model | seeds | mean final balance | gap to bot | model calls | input tok | output tok | invalid actions | est. cost |") {
		t.Fatalf("missing header:\n%s", got)
	}
	if !strings.Contains(got, "| bot | 1,2,3 | 1650 | +0 | 0 | 0 | 0 | 0 | $0.0000 |") {
		t.Fatalf("bot row:\n%s", got)
	}
	if !strings.Contains(got, "| llama3.2 | 1,2,3 | 1390 | -260 | 41 | 50000 | 2000 | 4 | $0.0123 |") {
		t.Fatalf("model row:\n%s", got)
	}
}

func TestAggregateMeanAndCost(t *testing.T) {
	cfg := &harness.Config{Model: "m", Pricing: map[string]*harness.Pricing{
		"m": {InputPerMillion: 1, OutputPerMillion: 2},
	}}
	r := aggregate("m", []uint32{1, 2}, []harness.RunStats{
		{FinalBalance: 100, ModelCalls: 2, InputTokens: 500_000, OutputTokens: 0, InvalidActions: 1},
		{FinalBalance: 200, ModelCalls: 3, InputTokens: 0, OutputTokens: 500_000, InvalidActions: 0},
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
}

func TestCompareBotOnly(t *testing.T) {
	dir := t.TempDir()
	var out, errb bytes.Buffer
	if err := runCompare(context.Background(), []uint32{1}, nil, dir, &out, &errb); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "| bot | 1 | 1650 | +0 | 0 | 0 | 0 | 0 | $0.0000 |") {
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
