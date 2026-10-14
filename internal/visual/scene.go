// Package visual builds a presentation model from a finished run's structured
// transcript. It is shared by the HTML and Ebiten visualizers so both read the
// same frames; the engine stays headless and is only replayed to recover the
// quest board each turn (it never renders anything).
package visual

import (
	"encoding/json"

	"architect_mockup_project/engine"
	"architect_mockup_project/harness"
)

// Frame is one turn flattened from a harness.TurnRecord for a viewer.
type Frame struct {
	Turn      int    `json:"turn"`
	Day       int    `json:"day"`
	Balance   int    `json:"balance"`
	Action    string `json:"action"`
	QuestID   string `json:"quest_id,omitempty"`
	Earned    int    `json:"earned"`
	DaysSpent int    `json:"days_spent"`
	Error     string `json:"error,omitempty"`
	Reasoning string `json:"reasoning,omitempty"`

	// Board is the quest board as it stood before this turn's action, recovered
	// by replaying the engine. Nil when the header carries no engine_seed
	// (unreplayable), so a viewer can fall back to turn-only display.
	Board  []QuestView `json:"board,omitempty"`
	Legal  []string    `json:"legal,omitempty"`
	Active string      `json:"active,omitempty"`
}

// QuestView is one board entry as the agent could see it. Hidden arcane
// resonances stay hidden: Nodes[i] is only exposed where Revealed[i] is true.
type QuestView struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Title    string   `json:"title"`
	Status   string   `json:"status"`
	Days     int      `json:"days"`
	Payout   int      `json:"payout"`
	Active   bool     `json:"active,omitempty"`
	Mana     int      `json:"mana,omitempty"`
	NextNode int      `json:"next_node,omitempty"`
	Revealed []string `json:"revealed,omitempty"`
}

// Scene is everything a viewer needs: header summary plus one frame per turn.
type Scene struct {
	Model       string `json:"model"`
	Provider    string `json:"provider"`
	EngineSeed  int    `json:"engine_seed"`
	FinalBal    int    `json:"final_balance"`
	TotalEarned int    `json:"total_earned"`
	MaxDay      int    `json:"max_day"`
	// DivergedAt is the first turn whose replayed outcome (accepted or
	// rejected, balance, day) differs from the recorded one, e.g. because the
	// engine changed since the run. Frames from it on carry no Board. 0 = none.
	DivergedAt int     `json:"diverged_at,omitempty"`
	Frames     []Frame `json:"frames"`
}

// BuildScene turns a parsed transcript into the viewers' animation model. It
// reads only structured records and replays the engine to recover the board;
// it never parses observation prose.
func BuildScene(log *harness.TranscriptLog) Scene {
	sc := Scene{
		Model:    headerString(log.Header, "model"),
		Provider: headerString(log.Header, "provider"),
		MaxDay:   engine.StartingLifespanDays,
	}
	seed, hasSeed := log.Header["engine_seed"].(float64)
	if hasSeed {
		sc.EngineSeed = int(seed)
	}
	for _, t := range log.Turns {
		f := Frame{
			Turn:      t.Turn,
			Day:       t.Day,
			Balance:   t.Balance,
			Action:    t.Action,
			QuestID:   questID(t.Args),
			Earned:    t.Earned,
			DaysSpent: t.DaysSpent,
			Reasoning: t.Reasoning,
		}
		if t.Error != nil {
			f.Error = *t.Error
		} else {
			sc.TotalEarned += t.Earned
		}
		sc.Frames = append(sc.Frames, f)
	}
	if n := len(log.Turns); n > 0 {
		sc.FinalBal = log.Turns[n-1].Balance
	}
	if hasSeed {
		replayBoards(&sc, log.Turns)
	}
	return sc
}

// replayBoards re-applies each recorded action to a fresh engine so every
// frame carries the board and legal actions the agent saw. Each replayed
// outcome is checked against the record (error turns must be rejected again);
// at the first mismatch the boards stop, since every later one would be wrong.
func replayBoards(sc *Scene, turns []harness.TurnRecord) {
	w := engine.New(uint32(sc.EngineSeed))
	startRemaining := w.Observe().LifespanDays // as newTurnRecord measures day
	for i := range sc.Frames {
		sc.Frames[i].Board = boardView(w)
		sc.Frames[i].Legal = legalNames(w)
		sc.Frames[i].Active = activeID(sc.Frames[i].Board)
		t := turns[i]
		args, _ := json.Marshal(t.Args)
		out, err := w.Apply(t.Action, args)
		if (err != nil) != (t.Error != nil) ||
			err == nil && (out.Balance != t.Balance || out.Earned != t.Earned || startRemaining-out.LifespanDays != t.Day) {
			sc.DivergedAt = t.Turn
			return
		}
	}
}

func boardView(w *engine.World) []QuestView {
	quests := w.Board()
	out := make([]QuestView, 0, len(quests))
	for _, q := range quests {
		v := QuestView{
			ID:     q.ID,
			Kind:   string(q.Kind),
			Title:  q.Title,
			Status: string(q.Status),
			Days:   q.Days,
			Payout: q.Payout,
			Active: q.Status == engine.QuestActive,
		}
		if q.Kind == engine.QuestArcane {
			v.Mana = q.Mana
			v.NextNode = q.NextNode + 1
			v.Revealed = make([]string, engine.ArcaneNodes)
			for i := range v.Revealed {
				if q.Revealed[i] {
					v.Revealed[i] = string(q.Nodes[i])
				}
			}
		}
		out = append(out, v)
	}
	return out
}

func legalNames(w *engine.World) []string {
	acts := w.LegalActions()
	out := make([]string, len(acts))
	for i, a := range acts {
		out[i] = a.Name
	}
	return out
}

func activeID(board []QuestView) string {
	for _, q := range board {
		if q.Active {
			return q.ID
		}
	}
	return ""
}

func headerString(header map[string]any, key string) string {
	s, _ := header[key].(string)
	return s
}

func questID(args map[string]any) string {
	s, _ := args["quest_id"].(string)
	return s
}
