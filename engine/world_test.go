package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestNewImplementsGame(t *testing.T) {
	var g Game = New(1)
	st := g.Observe()
	if st.Done {
		t.Fatal("fresh run should not be done")
	}
	if st.LifespanDays != StartingLifespanDays {
		t.Fatalf("days = %d", st.LifespanDays)
	}
	if st.Balance != StartingBalance {
		t.Fatalf("balance = %d", st.Balance)
	}
	if st.Narrative == "" {
		t.Fatal("empty narrative")
	}
	if len(g.LegalActions()) == 0 {
		t.Fatal("expected legal actions on a fresh board")
	}
}

func TestSameSeedSameBoardsAndRefreshes(t *testing.T) {
	a, b := New(7), New(7)
	if !boardsEqual(a.board, b.board) {
		t.Fatal("first boards differ")
	}
	if _, err := a.Apply(ActionRefresh, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Apply(ActionRefresh, nil); err != nil {
		t.Fatal(err)
	}
	if !boardsEqual(a.board, b.board) {
		t.Fatal("refresh boards differ")
	}
}

func TestApplyDeliver(t *testing.T) {
	w := New(1)
	plantDelivery(w, "q-d", 2, PayDeliveryShort)
	if _, err := w.Apply(ActionAccept, questArgs("q-d")); err != nil {
		t.Fatal(err)
	}
	out, err := w.Apply(ActionDeliver, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Earned != PayDeliveryShort {
		t.Fatalf("earned = %d", out.Earned)
	}
	if out.Balance != PayDeliveryShort {
		t.Fatalf("balance = %d", out.Balance)
	}
	if out.LifespanDays != StartingLifespanDays-2 {
		t.Fatalf("days = %d", out.LifespanDays)
	}
	before := w.clone()
	if _, err := w.Apply(ActionDeliver, nil); err == nil {
		t.Fatal("second deliver should error")
	}
	if !reflect.DeepEqual(w, before) {
		t.Fatal("failed deliver mutated world")
	}
}

func TestFightLossAtZero(t *testing.T) {
	w := New(1)
	plantBattle(w, "q-b")
	if _, err := w.Apply(ActionAccept, questArgs("q-b")); err != nil {
		t.Fatal(err)
	}
	out, err := w.Apply(ActionFight, nil)
	if err != nil {
		t.Fatal(err)
	}
	if WinsBattle(0) {
		t.Fatal("0 krone should lose")
	}
	if out.Balance != 0 {
		t.Fatalf("balance changed: %d", out.Balance)
	}
	if out.Earned != PayBattleLoss {
		t.Fatalf("earned = %d", out.Earned)
	}
	if out.LifespanDays != StartingLifespanDays-CostBattle {
		t.Fatalf("days = %d", out.LifespanDays)
	}
}

func TestFightOutcomeAgreesWithWinsBattle(t *testing.T) {
	for _, bal := range []int{0, 140, 149, 150, 151, 1000} {
		w := New(1)
		w.ledger.Krone = bal
		plantBattle(w, "q-b")
		if _, err := w.Apply(ActionAccept, questArgs("q-b")); err != nil {
			t.Fatal(err)
		}
		out, err := w.Apply(ActionFight, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := out.Earned == PayBattleWin, WinsBattle(bal); got != want {
			t.Fatalf("balance %d: engine won=%v, WinsBattle=%v", bal, got, want)
		}
	}
}

func TestFightWinAt150(t *testing.T) {
	w := New(1)
	w.ledger.Krone = 150
	plantBattle(w, "q-b")
	if _, err := w.Apply(ActionAccept, questArgs("q-b")); err != nil {
		t.Fatal(err)
	}
	out, err := w.Apply(ActionFight, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Balance != 150+PayBattleWin {
		t.Fatalf("balance = %d", out.Balance)
	}
	if out.Earned != PayBattleWin {
		t.Fatalf("earned = %d", out.Earned)
	}
	if out.LifespanDays != StartingLifespanDays-CostBattle {
		t.Fatalf("days = %d", out.LifespanDays)
	}
}

func TestInvalidApplyNoMutation(t *testing.T) {
	w := New(1)
	before := w.clone()
	cases := []struct {
		name string
		args json.RawMessage
	}{
		{"nonsense", nil},
		{ActionDeliver, nil},
		{ActionFight, nil},
		{ActionAccept, json.RawMessage(`{}`)},
		{ActionAccept, json.RawMessage(`{"quest_id":"nope"}`)},
		{ActionAccept, json.RawMessage(`not-json`)},
		{ActionAccept, nil},
	}
	for _, c := range cases {
		if _, err := w.Apply(c.name, c.args); err == nil {
			t.Fatalf("%s: want error", c.name)
		}
		if !reflect.DeepEqual(w, before) {
			t.Fatalf("%s mutated world", c.name)
		}
	}
}

func TestAcceptWhileActiveNoMutation(t *testing.T) {
	w := New(1)
	plantDelivery(w, "q-d", 2, PayDeliveryShort)
	if _, err := w.Apply(ActionAccept, questArgs("q-d")); err != nil {
		t.Fatal(err)
	}
	before := w.clone()
	if _, err := w.Apply(ActionAccept, questArgs("q-d")); err == nil {
		t.Fatal("second accept should error")
	}
	if !reflect.DeepEqual(w, before) {
		t.Fatal("second accept mutated world")
	}
	if _, err := w.Apply(ActionRefresh, nil); err == nil {
		t.Fatal("refresh while active should error")
	}
	if !reflect.DeepEqual(w, before) {
		t.Fatal("refresh while active mutated world")
	}
}

func TestLegalActionsSchemaHasNoReasoning(t *testing.T) {
	w := New(1)
	acts := w.LegalActions()
	assertNoReasoning(t, acts)
	var sawAccept, sawRefresh bool
	for _, a := range acts {
		switch a.Name {
		case ActionAccept:
			sawAccept = true
			prop, _ := a.Parameters["properties"].(map[string]any)
			if len(prop) != 1 {
				t.Errorf("accept_quest properties = %v, want only quest_id", prop)
			}
			if _, ok := prop["quest_id"]; !ok {
				t.Fatal("accept_quest missing quest_id")
			}
			req, _ := a.Parameters["required"].([]any)
			if len(req) != 1 || req[0] != "quest_id" {
				t.Errorf("accept_quest required = %v, want [quest_id]", req)
			}
		case ActionRefresh:
			sawRefresh = true
			if a.Parameters != nil {
				t.Errorf("refresh_board Parameters = %v, want nil", a.Parameters)
			}
		}
	}
	if !sawAccept {
		t.Fatal("expected accept_quest on a fresh board")
	}
	if !sawRefresh {
		t.Fatal("expected refresh_board on a fresh board")
	}

	plantDelivery(w, "q-d", 2, PayDeliveryShort)
	if _, err := w.Apply(ActionAccept, questArgs("q-d")); err != nil {
		t.Fatal(err)
	}
	acts = w.LegalActions()
	assertNoReasoning(t, acts)
	if len(acts) != 1 || acts[0].Name != ActionDeliver || acts[0].Parameters != nil {
		t.Errorf("deliver = %+v, want a nil-param deliver", acts)
	}

	w = New(1)
	plantBattle(w, "q-b")
	if _, err := w.Apply(ActionAccept, questArgs("q-b")); err != nil {
		t.Fatal(err)
	}
	acts = w.LegalActions()
	assertNoReasoning(t, acts)
	if len(acts) != 1 || acts[0].Name != ActionFight || acts[0].Parameters != nil {
		t.Errorf("fight = %+v, want a nil-param fight", acts)
	}
}

func assertNoReasoning(t *testing.T, acts []Action) {
	t.Helper()
	for _, a := range acts {
		if schemaHasReasoning(a.Parameters) {
			t.Errorf("%s schema includes reasoning: %+v", a.Name, a.Parameters)
		}
	}
}

func schemaHasReasoning(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		if _, ok := x["reasoning"]; ok {
			return true
		}
		for _, child := range x {
			if schemaHasReasoning(child) {
				return true
			}
		}
	case []any:
		for _, child := range x {
			if s, ok := child.(string); ok && s == "reasoning" {
				return true
			}
			if schemaHasReasoning(child) {
				return true
			}
		}
	case []string:
		for _, s := range x {
			if s == "reasoning" {
				return true
			}
		}
	}
	return false
}

func TestLegalActionsHidesUnaffordable(t *testing.T) {
	w := New(1)
	w.board = []Quest{
		{ID: "cheap", Kind: QuestDelivery, Title: "short", Days: 2, Payout: PayDeliveryShort, Status: QuestOpen},
		{ID: "dear", Kind: QuestDelivery, Title: "long", Days: 4, Payout: PayDeliveryLong, Status: QuestOpen},
	}
	w.ledger.Days = 3
	ids := acceptEnum(t, w)
	if len(ids) != 1 || ids[0] != "cheap" {
		t.Fatalf("enum = %v, want [cheap]", ids)
	}
	w.ledger.Days = 1
	if got := acceptEnum(t, w); got != nil {
		t.Fatalf("expected no accept_quest, enum=%v", got)
	}
	if !hasAction(w, ActionRefresh) {
		t.Fatal("refresh should still be legal with 1 day")
	}
}

func TestUnwinnableFightIsLegal(t *testing.T) {
	w := New(1)
	w.ledger.Krone = 0
	plantBattle(w, "q-b")
	if _, err := w.Apply(ActionAccept, questArgs("q-b")); err != nil {
		t.Fatal(err)
	}
	if !hasAction(w, ActionFight) {
		t.Fatal("fight must be legal even when unwinnable")
	}
	if hasAction(w, ActionRefresh) || hasAction(w, ActionAccept) {
		t.Fatal("only fight should be legal while a battle is active")
	}
}

func plantDelivery(w *World, id string, days, pay int) {
	w.board = []Quest{{
		ID: id, Kind: QuestDelivery, Title: "test parcel",
		Days: days, Payout: pay, Status: QuestOpen,
		Blurb: deliveryBlurb(days, pay),
	}}
	w.activeID = ""
}

func plantBattle(w *World, id string) {
	w.board = []Quest{{
		ID: id, Kind: QuestBattle, Title: "test foe",
		Days: CostBattle, Payout: PayBattleWin, Status: QuestOpen,
		Blurb: battleBlurb(),
	}}
	w.activeID = ""
}

func questArgs(id string) json.RawMessage {
	raw, err := json.Marshal(map[string]string{"quest_id": id})
	if err != nil {
		panic(err)
	}
	return raw
}

func hasAction(w *World, name string) bool {
	for _, a := range w.LegalActions() {
		if a.Name == name {
			return true
		}
	}
	return false
}

func acceptEnum(t *testing.T, w *World) []string {
	t.Helper()
	for _, a := range w.LegalActions() {
		if a.Name != ActionAccept {
			continue
		}
		prop, _ := a.Parameters["properties"].(map[string]any)
		qid, _ := prop["quest_id"].(map[string]any)
		enum, _ := qid["enum"].([]string)
		return enum
	}
	return nil
}

func (w *World) clone() *World {
	c := *w
	c.board = append([]Quest(nil), w.board...)
	c.ledger.Events = append([]LedgerEvent(nil), w.ledger.Events...)
	return &c
}
