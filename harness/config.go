package harness

import (
	"encoding/json"
	"os"
)

// Pricing is dollars per 1M tokens. Zero values mean "free" (local models).
type Pricing struct {
	InputPerMillion  float64 `json:"input_per_million"`
	OutputPerMillion float64 `json:"output_per_million"`
}

// Config is the run configuration, loaded from a JSON file. API keys come from
// the environment, never from the file.
type Config struct {
	Provider     string              `json:"provider"` // openai | deepseek | ollama | anthropic
	Model        string              `json:"model"`
	BaseURL      string              `json:"base_url"`    // empty = provider default (openai-compat only)
	APIKeyEnv    string              `json:"api_key_env"` // env var holding the key; empty = local model
	MaxTokens    int64               `json:"max_tokens"`
	Temperature  *float64            `json:"temperature"`
	TopP         *float64            `json:"top_p"`
	Seed         *int64              `json:"seed"`
	IterationCap int                 `json:"iteration_cap"` // hard per-decision cap; the main cost control
	Pricing      map[string]*Pricing `json:"pricing"`       // per-model; nil entry = free/local
}

// baseURLs are the OpenAI-compatible defaults per provider.
var baseURLs = map[string]string{
	"openai":   "https://api.openai.com/v1",
	"deepseek": "https://api.deepseek.com",
	"ollama":   "http://localhost:11434/v1",
}

// LoadConfig reads config from path and applies defaults.
func LoadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	if c.IterationCap == 0 {
		c.IterationCap = 10
	}
	return &c, nil
}

// APIKey returns the configured API key from the environment, or "" if none.
func (c *Config) APIKey() string {
	if c.APIKeyEnv == "" {
		return ""
	}
	return os.Getenv(c.APIKeyEnv)
}

// ResolveBaseURL returns the effective base URL for an openai-compat provider.
func (c *Config) ResolveBaseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return baseURLs[c.Provider]
}

// Cost returns the dollar cost of token usage on a model, or 0 for free/local.
func (c *Config) Cost(model string, in, out int64) float64 {
	p := c.Pricing[model]
	if p == nil {
		return 0
	}
	return float64(in)/1e6*p.InputPerMillion + float64(out)/1e6*p.OutputPerMillion
}
