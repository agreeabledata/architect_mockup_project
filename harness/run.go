package harness

import (
	"context"
	"fmt"

	"architect_mockup_project/engine"
)

// RunEpisode runs one agent episode against game and writes a JSONL transcript
// under transcriptDir. Compare calls this instead of shelling out to run_agent.
func RunEpisode(ctx context.Context, cfg *Config, game engine.Game, transcriptDir string) (Report, string, error) {
	prov, err := NewProvider(cfg)
	if err != nil {
		return Report{}, "", err
	}
	tr, err := NewTranscript(transcriptDir, cfg)
	if err != nil {
		return Report{}, "", err
	}
	defer tr.Close()
	agent := &Agent{Provider: prov, Config: cfg, Transcript: tr}
	report, err := agent.Run(ctx, game)
	return report, tr.Path(), err
}

// BotConfig is the dummy eval config written into a greedy-bot transcript header.
func BotConfig(seed uint32) *Config {
	s := int64(seed)
	return &Config{
		Provider:     "bot",
		Model:        "bot",
		Seed:         &s,
		IterationCap: 1,
	}
}

// RunBot drives the greedy baseline, recording one structured turn per action.
func RunBot(w *engine.World, tr *Transcript) error {
	startRemaining := w.Observe().LifespanDays
	turn := 0
	for i := 0; i < engine.BotStepCap; i++ {
		before := w.Observe()
		if before.Done || len(w.LegalActions()) == 0 {
			return nil
		}
		name, args, err := w.BotChoice()
		if err != nil {
			return err
		}
		outcome, err := w.Apply(name, args)
		turn++
		if tr != nil {
			tr.RecordTurn(newTurnRecord(turn, startRemaining, before, name, args, outcome, err))
		}
		if err != nil {
			return err
		}
	}
	return fmt.Errorf("bot: step cap hit")
}

// RunBotEpisode runs the greedy baseline on seed and writes a transcript.
func RunBotEpisode(seed uint32, transcriptDir string) (string, error) {
	tr, err := NewTranscript(transcriptDir, BotConfig(seed))
	if err != nil {
		return "", err
	}
	defer tr.Close()
	if err := RunBot(engine.New(seed), tr); err != nil {
		return tr.Path(), err
	}
	return tr.Path(), nil
}
