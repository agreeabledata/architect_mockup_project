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
