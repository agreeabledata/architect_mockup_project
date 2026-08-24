package harness

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"provider":"ollama","model":"llama3.2"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.IterationCap != 10 {
		t.Errorf("iteration cap = %d, want default 10", cfg.IterationCap)
	}
	if got := cfg.ResolveBaseURL(); got != "http://localhost:11434/v1" {
		t.Errorf("ollama base URL = %q", got)
	}
}

func TestConfigCost(t *testing.T) {
	cfg := &Config{Model: "m", Pricing: map[string]*Pricing{
		"m": {InputPerMillion: 1, OutputPerMillion: 2},
	}}
	if got := cfg.Cost("m", 500_000, 500_000); got != 0.5+1.0 {
		t.Errorf("cost = %v", got)
	}
	if got := cfg.Cost("local", 500_000, 500_000); got != 0 {
		t.Errorf("local cost = %v, want 0", got)
	}
}

func TestConfigAPIKeyEnv(t *testing.T) {
	t.Setenv("TEST_HARNESS_KEY", "sekret")
	cfg := &Config{APIKeyEnv: "TEST_HARNESS_KEY"}
	if got := cfg.APIKey(); got != "sekret" {
		t.Errorf("key = %q", got)
	}
}
