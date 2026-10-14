package harness

import (
	"context"
	"fmt"

	"architect_mockup_project/engine"
)

// RunEpisode runs one agent episode against game and writes a JSONL transcript
// under transcriptDir. Callers construct the Provider (compare injects a fake
// in tests). engineSeed is written to the transcript header and is independent
// of cfg.Seed (the model sampling seed).
func RunEpisode(ctx context.Context, prov Provider, cfg *Config, game engine.Game, transcriptDir string, engineSeed uint32) (Report, string, error) {
	tr, err := NewTranscript(transcriptDir, cfg, engineSeed)
	if err != nil {
		return Report{}, "", fmt.Errorf("transcript: %w", err)
	}
	defer tr.Close()
	agent := &Agent{Provider: prov, Config: cfg, Transcript: tr}
	report, err := agent.Run(ctx, game)
	return report, tr.Path(), err
}

// BotConfig is the dummy eval config for a greedy-bot transcript. The header
// writer recognizes provider "bot" and omits model-only fields.
func BotConfig() *Config {
	return &Config{
		Provider: "bot",
		Model:    "bot",
	}
}

// RunBot drives the greedy baseline, recording one structured turn per action.
// Same loop as engine.DriveBot; keep them in sync.
func RunBot(w *engine.World, tr *Transcript) error {
	startRemaining := w.Observe().HealthDays
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
			if recErr := tr.RecordTurn(newTurnRecord(turn, startRemaining, before, name, args, outcome, err)); recErr != nil {
				return recErr
			}
		}
		if err != nil {
			return err
		}
	}
	return fmt.Errorf("bot: step cap hit")
}

// RunBotEpisode runs the greedy baseline on seed and writes a transcript.
func RunBotEpisode(seed uint32, transcriptDir string) (string, error) {
	tr, err := NewTranscript(transcriptDir, BotConfig(), seed)
	if err != nil {
		return "", err
	}
	defer tr.Close()
	if err := RunBot(engine.New(seed), tr); err != nil {
		return tr.Path(), err
	}
	return tr.Path(), nil
}
