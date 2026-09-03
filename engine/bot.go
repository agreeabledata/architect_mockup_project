package engine

import (
	"encoding/json"
	"fmt"
)

const botStepCap = 256

// StepBot applies one legal action using the decision baseline: execute an
// already-accepted quest; otherwise take the affordable job with the best
// krones-per-day (skipping battles unless WinsBattle); otherwise refresh.
func (w *World) StepBot() (Outcome, error) {
	if w.isDone() {
		return Outcome{}, fmt.Errorf("bot: run is over")
	}
	if w.activeID != "" {
		q := w.questByID(w.activeID)
		if q == nil {
			return Outcome{}, fmt.Errorf("bot: active quest missing")
		}
		if q.Kind == QuestBattle {
			return w.Apply(ActionFight, nil)
		}
		return w.Apply(ActionDeliver, nil)
	}

	bestI := -1
	bestRate := -1.0
	for i, q := range w.board {
		if q.Status != QuestOpen || q.Days > w.ledger.Days {
			continue
		}
		if q.Kind == QuestBattle && !WinsBattle(w.ledger.Krone) {
			continue
		}
		rate := float64(q.Payout) / float64(q.Days)
		if rate > bestRate {
			bestRate = rate
			bestI = i
		}
	}
	if bestI >= 0 {
		args, err := json.Marshal(map[string]string{"quest_id": w.board[bestI].ID})
		if err != nil {
			return Outcome{}, err
		}
		return w.Apply(ActionAccept, args)
	}
	return w.Apply(ActionRefresh, nil)
}

// DriveBot runs StepBot until the ledger closes or no legal action remains.
func DriveBot(w *World) {
	for i := 0; i < botStepCap; i++ {
		if w.Observe().Done || len(w.LegalActions()) == 0 {
			return
		}
		if _, err := w.StepBot(); err != nil {
			return
		}
	}
}
