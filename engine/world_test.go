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
	if st.HealthDays != StartingHealthDays {
		t.Fatalf("days = %d", st.HealthDays)
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
	if out.HealthDays != StartingHealthDays-2 {
		t.Fatalf("days = %d", out.HealthDays)
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
	if out.HealthDays != StartingHealthDays-CostBattle {
		t.Fatalf("days = %d", out.HealthDays)
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
	if out.HealthDays != StartingHealthDays-CostBattle {
		t.Fatalf("days = %d", out.HealthDays)
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

func TestArcaneScryRevealsAndSpendsMana(t *testing.T) {
	w := New(1)
	plantArcane(w, "q-a", [ArcaneNodes]Resonance{ResonanceFire, ResonanceWater, ResonanceEarth})
	if _, err := w.Apply(ActionAccept, questArgs("q-a")); err != nil {
		t.Fatal(err)
	}
	out, err := w.Apply(ActionScry, json.RawMessage(`{"node":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if out.Earned != 0 {
		t.Fatalf("scry earned = %d", out.Earned)
	}
	q := w.questByID("q-a")
	if !q.Revealed[0] {
		t.Fatal("node 1 not revealed")
	}
	if q.Mana != ArcaneMana-ManaScry {
		t.Fatalf("mana = %d", q.Mana)
	}
	before := w.clone()
	if _, err := w.Apply(ActionScry, json.RawMessage(`{"node":1}`)); err == nil {
		t.Fatal("re-scrying node 1 should error")
	}
	if !reflect.DeepEqual(w, before) {
		t.Fatal("failed scry mutated world")
	}
}

func TestArcaneSolvePaysFromBalance(t *testing.T) {
	w := New(1)
	plantArcane(w, "q-a", [ArcaneNodes]Resonance{ResonanceFire, ResonanceWater, ResonanceEarth})
	if _, err := w.Apply(ActionAccept, questArgs("q-a")); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		`{"node":1}`, `{"node":2}`,
	} {
		if _, err := w.Apply(ActionScry, json.RawMessage(raw)); err != nil {
			t.Fatal(err)
		}
	}
	out, err := w.Apply(ActionCast, json.RawMessage(`{"spell":"fire"}`))
	if err != nil {
		t.Fatal(err)
	}
	if out.Earned != 0 {
		t.Fatalf("intermediate cast earned = %d", out.Earned)
	}
	for _, raw := range []string{
		`{"spell":"water"}`, `{"spell":"earth"}`,
	} {
		out, err = w.Apply(ActionCast, json.RawMessage(raw))
		if err != nil {
			t.Fatal(err)
		}
	}
	if out.Earned != PayArcane || out.Balance != PayArcane {
		t.Fatalf("earned=%d balance=%d, want %d", out.Earned, out.Balance, PayArcane)
	}
	if out.HealthDays != StartingHealthDays-CostArcane {
		t.Fatalf("days = %d", out.HealthDays)
	}
	if q := w.questByID("q-a"); q.Status != QuestComplete {
		t.Fatalf("status = %s", q.Status)
	}
	if w.activeID != "" {
		t.Fatal("quest still active after success")
	}
}

func TestArcaneWrongCastBacklashes(t *testing.T) {
	w := New(1)
	plantArcane(w, "q-a", [ArcaneNodes]Resonance{ResonanceFire, ResonanceWater, ResonanceEarth})
	if _, err := w.Apply(ActionAccept, questArgs("q-a")); err != nil {
		t.Fatal(err)
	}
	out, err := w.Apply(ActionCast, json.RawMessage(`{"spell":"water"}`))
	if err != nil {
		t.Fatal(err)
	}
	if out.Earned != 0 {
		t.Fatalf("backlash earned = %d", out.Earned)
	}
	q := w.questByID("q-a")
	if q.NextNode != 0 {
		t.Fatalf("node advanced on wrong cast: %d", q.NextNode)
	}
	if want := ArcaneMana - ManaSpell - ManaBacklash; q.Mana != want {
		t.Fatalf("mana = %d, want %d", q.Mana, want)
	}
}

func TestArcaneCollapsePaysNothing(t *testing.T) {
	w := New(1)
	plantArcane(w, "q-a", [ArcaneNodes]Resonance{ResonanceFire, ResonanceWater, ResonanceEarth})
	if _, err := w.Apply(ActionAccept, questArgs("q-a")); err != nil {
		t.Fatal(err)
	}
	var out Outcome
	for i := 0; i < 3 && w.activeID != ""; i++ {
		var err error
		out, err = w.Apply(ActionCast, json.RawMessage(`{"spell":"water"}`))
		if err != nil {
			t.Fatal(err)
		}
	}
	if out.Earned != 0 || out.Balance != 0 {
		t.Fatalf("collapse earned=%d balance=%d", out.Earned, out.Balance)
	}
	if out.HealthDays != StartingHealthDays-CostArcane {
		t.Fatalf("days = %d", out.HealthDays)
	}
	if q := w.questByID("q-a"); q.Status != QuestComplete {
		t.Fatalf("status = %s", q.Status)
	}
}

func TestArcaneUnknownSpellNoMutation(t *testing.T) {
	w := New(1)
	plantArcane(w, "q-a", [ArcaneNodes]Resonance{ResonanceFire, ResonanceWater, ResonanceEarth})
	if _, err := w.Apply(ActionAccept, questArgs("q-a")); err != nil {
		t.Fatal(err)
	}
	before := w.clone()
	if _, err := w.Apply(ActionCast, json.RawMessage(`{"spell":"shadow"}`)); err == nil {
		t.Fatal("unknown resonance should error")
	}
	if !reflect.DeepEqual(w, before) {
		t.Fatal("failed cast mutated world")
	}
}

func plantArcane(w *World, id string, nodes [ArcaneNodes]Resonance) {
	w.board = []Quest{{
		ID: id, Kind: QuestArcane, Title: "test rite",
		Days: CostArcane, Payout: PayArcane, Status: QuestOpen,
		Blurb: arcaneBlurb(), Nodes: nodes, Mana: ArcaneMana,
	}}
	w.activeID = ""
}

func TestApplyAfterDoneNoMutation(t *testing.T) {
	w := New(1)
	w.ledger.Days = 0
	before := w.clone()
	if _, err := w.Apply(ActionAccept, questArgs("q-d")); err == nil {
		t.Fatal("accept after done should error")
	}
	if _, err := w.Apply(ActionRefresh, nil); err == nil {
		t.Fatal("refresh after done should error")
	}
	if !reflect.DeepEqual(w, before) {
		t.Fatal("apply after done mutated world")
	}
	if acts := w.LegalActions(); acts != nil {
		t.Fatalf("legal actions after done = %v, want nil", acts)
	}
}

func TestAcceptUnaffordableRejected(t *testing.T) {
	for _, days := range []int{MinDeliveryDays, 3, MaxDeliveryDays} {
		w := New(1)
		w.ledger.Days = days - 1
		plantDelivery(w, "q-d", days, payoutForDeliveryDays(days))
		before := w.clone()
		if _, err := w.Apply(ActionAccept, questArgs("q-d")); err == nil {
			t.Fatalf("%d-day quest with %d days left: want error", days, days-1)
		}
		if !reflect.DeepEqual(w, before) {
			t.Fatalf("%d-day quest mutated world", days)
		}
	}
}

func TestRunEndsExactlyAtZero(t *testing.T) {
	for _, days := range []int{MinDeliveryDays, 3, MaxDeliveryDays} {
		w := New(1)
		w.ledger.Days = days
		plantDelivery(w, "q-d", days, payoutForDeliveryDays(days))
		if _, err := w.Apply(ActionAccept, questArgs("q-d")); err != nil {
			t.Fatal(err)
		}
		out, err := w.Apply(ActionDeliver, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !out.Done {
			t.Fatalf("%d-day run: not done at 0 days", days)
		}
		if out.HealthDays != 0 {
			t.Fatalf("%d-day run: days = %d, want 0", days, out.HealthDays)
		}
		if out.Balance != payoutForDeliveryDays(days) {
			t.Fatalf("%d-day run: balance = %d", days, out.Balance)
		}
		if _, err := w.Apply(ActionDeliver, nil); err == nil {
			t.Fatal("apply after exact-zero ending should error")
		}
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

func TestBoardSnapshotIsCopy(t *testing.T) {
	w := New(1)
	got := w.Board()
	if len(got) != QuestsPerBoard {
		t.Fatalf("board size = %d, want %d", len(got), QuestsPerBoard)
	}
	if !boardsEqual(got, w.board) {
		t.Fatal("Board() does not match the world's board")
	}
	got[0].ID = "mutated"
	if w.board[0].ID == "mutated" {
		t.Fatal("mutating the returned board changed the world")
	}
}
