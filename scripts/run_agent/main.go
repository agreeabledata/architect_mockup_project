// Command run_agent runs one agent episode against the stub engine and prints
// the cost record. Requires a config JSON (see config.example.json) and, for
// hosted providers, the API key in the environment variable named in the config.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"architect_mockup_project/harness"
	"architect_mockup_project/stub"
)

func main() {
	configPath := flag.String("config", "config.json", "path to run config JSON")
	flag.Parse()

	cfg, err := harness.LoadConfig(*configPath)
	if err != nil {
		fail("config: %v", err)
	}
	prov, err := newProvider(cfg)
	if err != nil {
		fail("%v", err)
	}
	tr, err := harness.NewTranscript("transcripts", cfg)
	if err != nil {
		fail("transcript: %v", err)
	}
	defer tr.Close()

	agent := &harness.Agent{Provider: prov, Config: cfg, Transcript: tr}
	report, err := agent.Run(context.Background(), stub.New())
	if err != nil {
		fail("run: %v", err)
	}

	fmt.Printf("provider      %s (%s)\n", cfg.Provider, cfg.Model)
	fmt.Printf("final balance %d krones\n", report.Balance)
	fmt.Printf("iterations    %d  (cap %d)\n", report.Iterations, cfg.IterationCap)
	fmt.Printf("tool calls    %d\n", report.ToolCalls)
	fmt.Printf("tokens        %d in / %d out / %d total\n", report.Usage.InputTokens, report.Usage.OutputTokens, report.Usage.TotalTokens)
	fmt.Printf("wall time     %s\n", report.WallTime)
	fmt.Printf("cost          $%.4f\n", report.Cost(cfg))
	fmt.Printf("reason        %s\n", report.Reason)
	fmt.Printf("transcript    %s\n", tr.Path())
}

func newProvider(cfg *harness.Config) (harness.Provider, error) {
	switch cfg.Provider {
	case "openai", "deepseek", "ollama":
		return harness.NewOpenAICompatProvider(cfg.ResolveBaseURL(), cfg.APIKey())
	case "anthropic":
		return harness.NewAnthropicProvider(cfg.APIKey(), ""), nil
	}
	return nil, fmt.Errorf("unknown provider %q (want openai|deepseek|ollama|anthropic)", cfg.Provider)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
