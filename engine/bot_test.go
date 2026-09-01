package engine

import "testing"

func TestBotFundsThenFights(t *testing.T) {
	w := New(1)
	DriveBot(w)
	st := w.Observe()
	if !st.Done {
		t.Fatal("bot should exhaust the lifespan")
	}
	if st.LifespanDays != 0 {
		t.Fatalf("days left = %d", st.LifespanDays)
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
