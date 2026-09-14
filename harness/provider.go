package harness

import "fmt"

// NewProvider builds the chat provider named in cfg.
func NewProvider(cfg *Config) (Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil config")
	}
	switch cfg.Provider {
	case "openai", "deepseek", "openrouter", "ollama":
		return NewOpenAICompatProvider(cfg.ResolveBaseURL(), cfg.APIKey)
	case "anthropic":
		return NewAnthropicProvider(cfg.APIKey, ""), nil
	}
	return nil, fmt.Errorf("unknown provider %q (want openai|deepseek|openrouter|ollama|anthropic)", cfg.Provider)
}
