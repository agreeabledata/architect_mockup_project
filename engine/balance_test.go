package engine

import "testing"

// The whole eval hinges on one number: the balance at which fighting starts
// winning. If a payout or duel constant changes, this test says so.
func TestWinThresholdIs150(t *testing.T) {
	const want = 150
	for b := 0; b < want; b += 10 {
		if WinsBattle(b) {
			t.Fatalf("balance %d should lose", b)
		}
	}
	if !WinsBattle(want) {
		t.Fatalf("balance %d should win", want)
	}
}

// The advertised attack budget must stay non-binding, or it stops being a
// distractor and becomes a rule the engine has to implement.
func TestAttackBudgetNeverBinds(t *testing.T) {
	maxAttacks := (AgentHP + OpponentDamage - 1) / OpponentDamage
	if maxAttacks > AttackBudget {
		t.Fatalf("agent lands %d attacks, budget of %d would bind", maxAttacks, AttackBudget)
	}
}
