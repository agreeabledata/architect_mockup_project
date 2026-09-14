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
	if cfg.DecisionRetryCap != 3 {
		t.Errorf("decision retry cap = %d, want default 3", cfg.DecisionRetryCap)
	}
	if got := cfg.ResolveBaseURL(); got != "http://localhost:11434/v1" {
		t.Errorf("ollama base URL = %q", got)
	}
}

func TestResolveBaseURL(t *testing.T) {
	tests := []struct {
		provider string
		baseURL  string
		want     string
	}{
		{provider: "openrouter", want: "https://openrouter.ai/api/v1"},
		{provider: "openrouter", baseURL: "http://localhost:8080/v1", want: "http://localhost:8080/v1"},
		{provider: "deepseek", want: "https://api.deepseek.com"},
		{provider: "anthropic", want: ""}, // not openai-compat, so no default
	}
	for _, tt := range tests {
		cfg := &Config{Provider: tt.provider, BaseURL: tt.baseURL}
		if got := cfg.ResolveBaseURL(); got != tt.want {
			t.Errorf("ResolveBaseURL(%q, base_url=%q) = %q, want %q", tt.provider, tt.baseURL, got, tt.want)
		}
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
	if !cfg.Priced("m") {
		t.Error("m should be priced")
	}
	if cfg.Priced("local") {
		t.Error("missing model must not be Priced")
	}
	cfg.Pricing["free"] = &Pricing{}
	if !cfg.Priced("free") {
		t.Error("explicit zero-rate entry is priced (free, not n/a)")
	}
}
