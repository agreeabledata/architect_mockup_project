package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestBotFundsThenFights(t *testing.T) {
	w := New(1)
	DriveBot(w)
	st := w.Observe()
	if !st.Done {
		t.Fatal("bot should exhaust the health")
	}
	if st.HealthDays != 0 {
		t.Fatalf("days left = %d", st.HealthDays)
	}
	if st.Balance != totalEarned(w.ledger.Events) {
		t.Fatalf("balance %d != earned %d", st.Balance, totalEarned(w.ledger.Events))
	}

	deliveries, wins := 0, 0
	for _, e := range w.ledger.Events {
		switch e.Kind {
		case "delivery":
			deliveries++
		case "battle_win":
			wins++
		}
	}
	if deliveries < 1 {
		t.Fatal("bot never completed a delivery")
	}
	if wins < 1 {
		t.Fatal("bot never won a battle after funding")
	}
}

func TestBotSolvesArcaneWithoutPeeking(t *testing.T) {
	w := New(1)
	plantArcane(w, "q-a", [ArcaneNodes]Resonance{ResonanceWater, ResonanceEarth, ResonanceFire})
	if _, err := w.Apply(ActionAccept, questArgs("q-a")); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2*ArcaneNodes+2 && w.activeID != ""; i++ {
		if _, err := w.StepBot(); err != nil {
			t.Fatal(err)
		}
	}
	if q := w.questByID("q-a"); q.Status != QuestComplete {
		t.Fatalf("ritual not complete: %s", q.Status)
	}
	if w.ledger.Krone != PayArcane {
		t.Fatalf("balance = %d, want %d", w.ledger.Krone, PayArcane)
	}
}

func TestBotDeterministicPerSeed(t *testing.T) {
	a, b := New(9), New(9)
	DriveBot(a)
	DriveBot(b)
	if a.Observe().Balance != b.Observe().Balance || a.Observe().HealthDays != b.Observe().HealthDays {
		t.Fatalf("same seed diverged: %+v vs %+v", a.Observe(), b.Observe())
	}
}

func TestBotDeterministic(t *testing.T) {
	for _, seed := range []uint32{1, 7, 42} {
		a, b := New(seed), New(seed)
		DriveBot(a)
		DriveBot(b)
		if !reflect.DeepEqual(a.ledger, b.ledger) {
			t.Fatalf("seed %d: ledgers differ", seed)
		}
	}
}

func TestBotSkipsUnwinnableBattles(t *testing.T) {
	w := New(1)
	w.board = []Quest{
		{ID: "b", Kind: QuestBattle, Title: "trap", Days: CostBattle, Payout: PayBattleWin, Status: QuestOpen},
		{ID: "d", Kind: QuestDelivery, Title: "safe", Days: 2, Payout: PayDeliveryShort, Status: QuestOpen},
	}
	out, err := w.StepBot()
	if err != nil {
		t.Fatal(err)
	}
	if w.activeID != "d" {
		t.Fatalf("active = %q, want delivery; outcome %q", w.activeID, out.Narrative)
	}
}

func TestBotChoiceNamesTheDelivery(t *testing.T) {
	w := New(1)
	w.board = []Quest{
		{ID: "b", Kind: QuestBattle, Title: "trap", Days: CostBattle, Payout: PayBattleWin, Status: QuestOpen},
		{ID: "d", Kind: QuestDelivery, Title: "safe", Days: 2, Payout: PayDeliveryShort, Status: QuestOpen},
	}
	name, args, err := w.BotChoice()
	if err != nil {
		t.Fatal(err)
	}
	if name != ActionAccept {
		t.Fatalf("name = %q, want %s", name, ActionAccept)
	}
	var m map[string]string
	if err := json.Unmarshal(args, &m); err != nil {
		t.Fatal(err)
	}
	if m["quest_id"] != "d" {
		t.Fatalf("quest_id = %q, want d", m["quest_id"])
	}
}
