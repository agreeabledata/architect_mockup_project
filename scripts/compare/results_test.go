package main

import (
	"os"
	"path/filepath"
	"testing"

	"architect_mockup_project/harness"
)

// TestCommittedResults is the issue #35 gate: the scored tree is a full
// current-main dataset, and results/README.md is exactly what -from writes.
func TestCommittedResults(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Join("..", "..")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Error(err)
		}
	})

	dir := "results"
	runs, err := loadScored(dir)
	if err != nil {
		t.Fatal(err)
	}
	byModel := map[string]map[uint32]scoredRun{}
	for _, run := range runs {
		if byModel[run.model] == nil {
			byModel[run.model] = map[uint32]scoredRun{}
		}
		if _, dup := byModel[run.model][run.seed]; dup {
			t.Fatalf("duplicate %s seed %d", run.model, run.seed)
		}
		byModel[run.model][run.seed] = run
	}
	want := []string{"bot", "meta-llama/llama-3.3-70b-instruct", "mistralai/mistral-small-3.2-24b-instruct"}
	if len(byModel) != len(want) {
		got := make([]string, 0, len(byModel))
		for name := range byModel {
			got = append(got, name)
		}
		t.Fatalf("models = %v, want %v", got, want)
	}
	for _, model := range want {
		seeds, ok := byModel[model]
		if !ok {
			t.Fatalf("missing model %s", model)
		}
		for _, seed := range []uint32{1, 2, 3} {
			run, ok := seeds[seed]
			if !ok {
				t.Fatalf("missing %s seed %d", model, seed)
			}
			assertCommittedRun(t, model, seed, run.path)
		}
	}
	for _, name := range []string{
		"run_20260914T183126.275447000Z.jsonl",
		"run_20260914T184721.600107000Z.jsonl",
	} {
		if _, err := os.Stat(filepath.Join(dir, "archive", name)); err != nil {
			t.Errorf("archive %s: %v", name, err)
		}
	}

	body, err := buildFrom(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Fatalf("results/README.md drifted from compare -from (%d committed bytes, %d regenerated)", len(got), len(body))
	}
}

func assertCommittedRun(t *testing.T, model string, seed uint32, path string) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	log, err := harness.LoadTranscript(f)
	if err != nil {
		t.Fatal(err)
	}
	if log.Header["model"] != model {
		t.Errorf("%s model = %v", path, log.Header["model"])
	}
	if log.Header["engine_seed"] != float64(seed) {
		t.Errorf("%s engine_seed = %v", path, log.Header["engine_seed"])
	}
	day, ok := lastSuccessDay(log.Turns)
	if !ok || day != 50 {
		t.Errorf("%s last successful day = %d, ok %v", path, day, ok)
	}
	if model == "bot" {
		if log.Header["provider"] != "bot" {
			t.Errorf("%s provider = %v", path, log.Header["provider"])
		}
		if _, present := log.Header["prompt_version"]; present {
			t.Errorf("%s bot header has prompt_version", path)
		}
		if len(log.Calls) != 0 {
			t.Errorf("%s bot recorded %d calls", path, len(log.Calls))
		}
		return
	}
	if log.Header["prompt_version"] != "v3" {
		t.Errorf("%s prompt_version = %v", path, log.Header["prompt_version"])
	}
	hash, _ := log.Header["config_hash"].(string)
	if hash == "" {
		t.Errorf("%s missing config_hash", path)
	}
}
