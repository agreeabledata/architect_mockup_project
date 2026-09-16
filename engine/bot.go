package engine

import (
	"encoding/json"
	"fmt"
)

// BotStepCap bounds a scripted baseline run. Compare and DriveBot share it.
const BotStepCap = 256

// BotChoice returns the greedy baseline's next action without applying it.
func (w *World) BotChoice() (string, json.RawMessage, error) {
	if w.isDone() {
		return "", nil, fmt.Errorf("bot: run is over")
	}
	if w.activeID != "" {
		q := w.questByID(w.activeID)
		if q == nil {
			return "", nil, fmt.Errorf("bot: active quest missing")
		}
		switch q.Kind {
		case QuestBattle:
			return ActionFight, nil, nil
		case QuestArcane:
			return w.stepArcane(q)
		default:
			return ActionDeliver, nil, nil
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
			return "", nil, err
		}
		return ActionAccept, args, nil
	}
	return ActionRefresh, nil, nil
}

// StepBot applies one legal action using the decision baseline: execute an
// already-accepted quest; otherwise take the affordable job with the best
// krones-per-day (skipping battles unless WinsBattle); otherwise refresh.
func (w *World) StepBot() (Outcome, error) {
	name, args, err := w.BotChoice()
	if err != nil {
		return Outcome{}, err
	}
	return w.Apply(name, args)
}

// stepArcane chooses the ritual's next move without applying it: scry the
// current node unless it is known or deducible to the last remaining
// resonance, then cast. The mana budget in balance.go makes scry-until-
// deducible the optimal line.
func (w *World) stepArcane(q *Quest) (string, json.RawMessage, error) {
	if q.Revealed[q.NextNode] {
		return castArgs(q.Nodes[q.NextNode])
	}
	if rem := w.remainingResonances(q); len(rem) == 1 {
		return castArgs(rem[0])
	}
	if q.Mana >= ManaScry {
		args, err := json.Marshal(map[string]int{"node": q.NextNode + 1})
		if err != nil {
			return "", nil, err
		}
		return ActionScry, args, nil
	}
	return castArgs(w.remainingResonances(q)[0])
}

func castArgs(r Resonance) (string, json.RawMessage, error) {
	args, err := json.Marshal(map[string]string{"spell": string(r)})
	if err != nil {
		return "", nil, err
	}
	return ActionCast, args, nil
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
// harness.RunBot is the recording counterpart (same loop, plus transcripts).
func DriveBot(w *World) {
	for i := 0; i < BotStepCap; i++ {
		if w.Observe().Done || len(w.LegalActions()) == 0 {
			return
		}
		if _, err := w.StepBot(); err != nil {
			return
		}
	}
}
