// Command compare runs the greedy bot and one or more model configs on the
// same seeds and prints a markdown comparison table.
//
//	go run ./scripts/compare -seeds 1,2,3 -configs bot,ollama.json,deepseek.json
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"architect_mockup_project/engine"
	"architect_mockup_project/harness"
)

func main() {
	seedsFlag := flag.String("seeds", "1", "comma-separated engine seeds")
	configsFlag := flag.String("configs", "", "comma-separated config paths; bot is always the first row")
	dir := flag.String("transcripts", "transcripts", "directory for per-run transcripts")
	flag.Parse()

	seeds, err := parseSeeds(*seedsFlag)
	if err != nil {
		fail("%v", err)
	}
	if err := runCompare(context.Background(), seeds, parseConfigs(*configsFlag), *dir, os.Stdout, os.Stderr); err != nil {
		fail("%v", err)
	}
}

func runCompare(ctx context.Context, seeds []uint32, configs []string, dir string, stdout, stderr io.Writer) error {
	botStats := make([]harness.RunStats, 0, len(seeds))
	for _, seed := range seeds {
		path, err := harness.RunBotEpisode(seed, dir)
		if err != nil {
			return fmt.Errorf("bot seed %d: %w", seed, err)
		}
		st, err := statsFromPath(path)
		if err != nil {
			return err
		}
		botStats = append(botStats, st)
		fmt.Fprintf(stderr, "bot  seed %d  balance %d  %s\n", seed, st.FinalBalance, path)
	}
	botRow := aggregate("bot", seeds, botStats, nil, 0)
	botRow.gap = 0
	rows := []row{botRow}

	for _, cfgPath := range configs {
		if cfgPath == "bot" {
			continue
		}
		cfg, err := harness.LoadConfig(cfgPath)
		if err != nil {
			return fmt.Errorf("%s: %w", cfgPath, err)
		}
		stats := make([]harness.RunStats, 0, len(seeds))
		for _, seed := range seeds {
			s := int64(seed)
			cfg.Seed = &s
			_, path, err := harness.RunEpisode(ctx, cfg, engine.New(seed), dir)
			if err != nil {
				return fmt.Errorf("%s seed %d: %w", cfgPath, seed, err)
			}
			st, err := statsFromPath(path)
			if err != nil {
				return err
			}
			stats = append(stats, st)
			fmt.Fprintf(stderr, "%s  seed %d  balance %d  %s\n", cfg.Model, seed, st.FinalBalance, path)
		}
		rows = append(rows, aggregate(cfg.Model, seeds, stats, cfg, botRow.meanBal))
	}
	_, err := io.WriteString(stdout, formatTable(rows))
	return err
}

func statsFromPath(path string) (harness.RunStats, error) {
	f, err := os.Open(path)
	if err != nil {
		return harness.RunStats{}, err
	}
	defer f.Close()
	log, err := harness.LoadTranscript(f)
	if err != nil {
		return harness.RunStats{}, err
	}
	return log.Stats(), nil
}

type row struct {
	model   string
	seeds   string
	meanBal float64
	gap     float64
	calls   int
	inTok   int64
	outTok  int64
	invalid int
	cost    float64
}

func aggregate(model string, seeds []uint32, stats []harness.RunStats, cfg *harness.Config, botMean float64) row {
	r := row{model: model, seeds: formatSeeds(seeds)}
	var bal int
	for _, st := range stats {
		bal += st.FinalBalance
		r.calls += st.ModelCalls
		r.inTok += st.InputTokens
		r.outTok += st.OutputTokens
		r.invalid += st.InvalidActions
	}
	if n := len(stats); n > 0 {
		r.meanBal = float64(bal) / float64(n)
	}
	r.gap = r.meanBal - botMean
	if cfg != nil {
		r.cost = cfg.Cost(cfg.Model, r.inTok, r.outTok)
	}
	return r
}

func formatTable(rows []row) string {
	var b strings.Builder
	b.WriteString("| model | seeds | mean final balance | gap to bot | model calls | input tok | output tok | invalid actions | est. cost |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %d | %d | %d | %d | $%.4f |\n",
			r.model, r.seeds, formatNum(r.meanBal), formatGap(r.gap),
			r.calls, r.inTok, r.outTok, r.invalid, r.cost)
	}
	return b.String()
}

func formatNum(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%.0f", f)
	}
	return fmt.Sprintf("%.1f", f)
}

func formatGap(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%+.0f", f)
	}
	return fmt.Sprintf("%+.1f", f)
}

func parseSeeds(s string) ([]uint32, error) {
	parts := splitCSV(s)
	if len(parts) == 0 {
		return nil, fmt.Errorf("no seeds")
	}
	out := make([]uint32, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.ParseUint(p, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("seed %q: %w", p, err)
		}
		out = append(out, uint32(n))
	}
	return out, nil
}

func parseConfigs(s string) []string {
	return splitCSV(s)
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func formatSeeds(seeds []uint32) string {
	parts := make([]string, len(seeds))
	for i, s := range seeds {
		parts[i] = strconv.FormatUint(uint64(s), 10)
	}
	return strings.Join(parts, ",")
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
