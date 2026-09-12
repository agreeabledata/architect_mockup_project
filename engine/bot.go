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
		switch q.Kind {
		case QuestBattle:
			return w.Apply(ActionFight, nil)
		case QuestArcane:
			return w.stepArcane(q)
		default:
			return w.Apply(ActionDeliver, nil)
		}
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
		if q.Kind == QuestArcane && !arcaneSolvable(q) {
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

// stepArcane plays the ritual without peeking at hidden resonances: scry the
// current node unless it is known or deducible to the last remaining
// resonance, then cast. The mana budget in balance.go makes scry-until-
// deducible the optimal line.
func (w *World) stepArcane(q *Quest) (Outcome, error) {
	if q.Revealed[q.NextNode] {
		return w.castResonance(q.Nodes[q.NextNode])
	}
	if rem := w.remainingResonances(q); len(rem) == 1 {
		return w.castResonance(rem[0])
	}
	if q.Mana >= ManaScry {
		args, err := json.Marshal(map[string]int{"node": q.NextNode + 1})
		if err != nil {
			return Outcome{}, err
		}
		return w.Apply(ActionScry, args)
	}
	return w.castResonance(w.remainingResonances(q)[0])
}

func (w *World) castResonance(r Resonance) (Outcome, error) {
	args, err := json.Marshal(map[string]string{"spell": string(r)})
	if err != nil {
		return Outcome{}, err
	}
	return w.Apply(ActionCast, args)
}

// remainingResonances returns the resonances not yet disrupted. Only already-
// cast nodes are read, so the bot never peeks at hidden state.
func (w *World) remainingResonances(q *Quest) []Resonance {
	used := map[Resonance]bool{}
	for i := 0; i < q.NextNode; i++ {
		used[q.Nodes[i]] = true
	}
	var rem []Resonance
	for _, r := range resonances {
		if !used[r] {
			rem = append(rem, r)
		}
	}
	return rem
}

// arcaneSolvable reports whether the ritual's mana covers the worst-case
// no-peek line: scry all but one node, then cast every spell.
func arcaneSolvable(q Quest) bool {
	return q.Mana >= ManaScry*(ArcaneNodes-1)+ManaSpell*ArcaneNodes
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
