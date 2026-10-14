package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"architect_mockup_project/engine"
	"architect_mockup_project/harness"
)

// fightWinBalance is the krone balance at which WinsBattle flips. A successful
// fight whose balance-before (balance - earned) is below it lost on purpose.
const fightWinBalance = 150

// scoredRun is one committed full-game transcript.
type scoredRun struct {
	model string
	seed  uint32
	path  string
	stats harness.RunStats
	turns []harness.TurnRecord
}

// writeFrom rebuilds <dir>/README.md from committed transcripts. It does not
// run the bot or any model.
func writeFrom(dir, pricingPath string) error {
	body, err := buildFrom(dir, pricingPath)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "README.md"), []byte(body), 0o644)
}

// buildFrom is the README body writeFrom persists. A second call with the
// same transcripts returns the same bytes.
func buildFrom(dir, pricingPath string) (string, error) {
	runs, err := loadScored(dir)
	if err != nil {
		return "", err
	}
	botRuns, groups, err := groupRuns(runs)
	if err != nil {
		return "", err
	}
	priceCfg, err := harness.LoadConfig(pricingPath)
	if err != nil {
		return "", fmt.Errorf("pricing: %w", err)
	}

	botRow := aggregate("bot", seedsOf(botRuns), statsOf(botRuns), nil, 0)
	botRow.gap = 0
	rows := []row{botRow}
	for _, g := range groups {
		rows = append(rows, aggregate(g.model, seedsOf(g.runs), statsOf(g.runs), priceCfg, botRow.meanBal))
	}
	notes, err := readFailedNotes(dir)
	if err != nil {
		return "", err
	}
	return renderReadme(dir, rows, groups, indexBySeed(botRuns), notes), nil
}

func renderReadme(dir string, rows []row, groups []modelGroup, botBySeed map[uint32]scoredRun, failedNotes string) string {
	var b strings.Builder
	b.WriteString("# Results\n\n")
	fmt.Fprintf(&b, "Regenerate with `go run ./scripts/compare -from %s/`.\n\n", filepath.Clean(dir))
	b.WriteString(formatTable(rows))
	b.WriteString("\n## Analysis\n\n")
	b.WriteString(analysisParagraph(rows, groups, botBySeed))
	b.WriteString("\n")
	if failedNotes != "" {
		b.WriteString("\n## Failed runs\n\n")
		b.WriteString(failedNotes)
		if !strings.HasSuffix(failedNotes, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func analysisParagraph(rows []row, groups []modelGroup, botBySeed map[uint32]scoredRun) string {
	byName := make(map[string]row, len(rows))
	for _, r := range rows {
		byName[r.model] = r
	}
	parts := make([]string, 0, len(groups)+1)
	for _, g := range groups {
		r := byName[g.model]
		fights, fightSeeds := fightsUnder150(g.runs)
		refreshes, botRefreshes := 0, 0
		for _, run := range g.runs {
			refreshes += countAction(run.turns, engine.ActionRefresh)
			botRefreshes += countAction(botBySeed[run.seed].turns, engine.ActionRefresh)
		}
		parts = append(parts, fmt.Sprintf(
			"%s averaged %s (gap %s). It %s, refreshed the board %s versus the bot's %s on the same seeds, and recorded %s.",
			g.model,
			formatNum(r.meanBal),
			formatGap(r.gap),
			fightPhrase(fights, fightSeeds),
			timesPhrase(refreshes),
			timesPhrase(botRefreshes),
			invalidPhrase(r.invalid),
		))
	}
	if len(groups) == 0 {
		parts = append(parts, "The bot is the only scored row.")
	}
	parts = append(parts, "Every scored run reached day 50.")
	return strings.Join(parts, " ")
}

func fightPhrase(n int, seeds []uint32) string {
	if n == 0 {
		return "fought under 150 krones 0 times"
	}
	return fmt.Sprintf("fought under 150 krones %s (seeds %s)", timesPhrase(n), formatSeeds(seeds))
}

func timesPhrase(n int) string {
	if n == 1 {
		return "1 time"
	}
	return fmt.Sprintf("%d times", n)
}

func invalidPhrase(n int) string {
	if n == 1 {
		return "1 invalid action"
	}
	return fmt.Sprintf("%d invalid actions", n)
}

func fightsUnder150(runs []scoredRun) (int, []uint32) {
	var n int
	var seeds []uint32
	for _, run := range runs {
		c := 0
		for _, t := range run.turns {
			if t.Error != nil || t.Action != engine.ActionFight {
				continue
			}
			if t.Balance-t.Earned < fightWinBalance {
				c++
			}
		}
		if c == 0 {
			continue
		}
		n += c
		seeds = append(seeds, run.seed)
	}
	return n, seeds
}

func countAction(turns []harness.TurnRecord, action string) int {
	n := 0
	for _, t := range turns {
		if t.Error == nil && t.Action == action {
			n++
		}
	}
	return n
}

type modelGroup struct {
	model string
	runs  []scoredRun
}

func groupRuns(runs []scoredRun) ([]scoredRun, []modelGroup, error) {
	byModel := map[string][]scoredRun{}
	for _, run := range runs {
		byModel[run.model] = append(byModel[run.model], run)
	}
	botRuns, ok := byModel["bot"]
	if !ok {
		return nil, nil, fmt.Errorf("no bot transcripts")
	}
	sortRuns(botRuns)
	var names []string
	for name := range byModel {
		if name == "bot" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	groups := make([]modelGroup, 0, len(names))
	for _, name := range names {
		rs := byModel[name]
		sortRuns(rs)
		if !sameSeeds(botRuns, rs) {
			return nil, nil, fmt.Errorf("%s seeds %s do not match bot seeds %s", name, formatSeeds(seedsOf(rs)), formatSeeds(seedsOf(botRuns)))
		}
		groups = append(groups, modelGroup{model: name, runs: rs})
	}
	return botRuns, groups, nil
}

func sortRuns(runs []scoredRun) {
	sort.Slice(runs, func(i, j int) bool { return runs[i].seed < runs[j].seed })
}

func sameSeeds(a, b []scoredRun) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].seed != b[i].seed {
			return false
		}
	}
	return true
}

func seedsOf(runs []scoredRun) []uint32 {
	out := make([]uint32, len(runs))
	for i, run := range runs {
		out[i] = run.seed
	}
	return out
}

func statsOf(runs []scoredRun) []harness.RunStats {
	out := make([]harness.RunStats, len(runs))
	for i, run := range runs {
		out[i] = run.stats
	}
	return out
}

func indexBySeed(runs []scoredRun) map[uint32]scoredRun {
	out := make(map[uint32]scoredRun, len(runs))
	for _, run := range runs {
		out[run.seed] = run
	}
	return out
}

func loadScored(dir string) ([]scoredRun, error) {
	paths, err := scoredPaths(dir)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no seed-*.jsonl transcripts in %s", dir)
	}
	seen := map[string]string{}
	runs := make([]scoredRun, 0, len(paths))
	for _, path := range paths {
		run, err := readScored(path)
		if err != nil {
			return nil, err
		}
		key := run.model + "\x00" + strconv.FormatUint(uint64(run.seed), 10)
		if prev, ok := seen[key]; ok {
			return nil, fmt.Errorf("duplicate %s seed %d (%s and %s)", run.model, run.seed, prev, path)
		}
		seen[key] = path
		runs = append(runs, run)
	}
	return runs, nil
}

func readScored(path string) (scoredRun, error) {
	f, err := os.Open(path)
	if err != nil {
		return scoredRun{}, err
	}
	defer f.Close()
	log, err := harness.LoadTranscript(f)
	if err != nil {
		return scoredRun{}, fmt.Errorf("%s: %w", path, err)
	}
	model, _ := log.Header["model"].(string)
	if model == "" {
		return scoredRun{}, fmt.Errorf("%s: header missing model", path)
	}
	seed, ok := headerEngineSeed(log.Header)
	if !ok {
		return scoredRun{}, fmt.Errorf("%s: header missing engine_seed; move it under failed/ or archive/", path)
	}
	nameSeed, err := seedFromName(filepath.Base(path))
	if err != nil {
		return scoredRun{}, fmt.Errorf("%s: %w", path, err)
	}
	if nameSeed != seed {
		return scoredRun{}, fmt.Errorf("%s: engine_seed %d does not match filename seed %d", path, seed, nameSeed)
	}
	day, ok := lastSuccessDay(log.Turns)
	if !ok {
		return scoredRun{}, fmt.Errorf("%s: no successful turn; move it under failed/", path)
	}
	if day != engine.StartingLifespanDays {
		return scoredRun{}, fmt.Errorf("%s: last successful day is %d, want %d; move it under failed/", path, day, engine.StartingLifespanDays)
	}
	st := log.Stats()
	st.Reason = "terminal"
	return scoredRun{model: model, seed: seed, path: path, stats: st, turns: log.Turns}, nil
}

func headerEngineSeed(header map[string]any) (uint32, bool) {
	v, ok := header["engine_seed"]
	if !ok || v == nil {
		return 0, false
	}
	n, ok := v.(float64)
	if !ok || n < 0 || n != float64(uint32(n)) {
		return 0, false
	}
	return uint32(n), true
}

func seedFromName(name string) (uint32, error) {
	base := strings.TrimSuffix(name, ".jsonl")
	rest, ok := strings.CutPrefix(base, "seed-")
	if !ok || rest == "" {
		return 0, fmt.Errorf("filename %s is not seed-<n>.jsonl", name)
	}
	n, err := strconv.ParseUint(rest, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("filename %s: %w", name, err)
	}
	return uint32(n), nil
}

func lastSuccessDay(turns []harness.TurnRecord) (int, bool) {
	for i := len(turns) - 1; i >= 0; i-- {
		if turns[i].Error == nil {
			return turns[i].Day, true
		}
	}
	return 0, false
}

// scoredPaths lists seed-*.jsonl files under dir, skipping failed/ and archive/.
func scoredPaths(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != dir && (d.Name() == "failed" || d.Name() == "archive") {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, "seed-") && strings.HasSuffix(name, ".jsonl") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func readFailedNotes(dir string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, "failed", "NOTES.md"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}
