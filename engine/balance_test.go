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

// The arcane budget must force deduction: scrying every node and casting every
// spell is unaffordable, but scrying all but one and deducing the last fits.
// If mana, scry, or spell costs change, this says so.
func TestArcaneManaForcesDeduction(t *testing.T) {
	scryAll := ManaScry*ArcaneNodes + ManaSpell*ArcaneNodes
	if scryAll <= ArcaneMana {
		t.Fatalf("budget %d allows scrying every node (%d); no deduction needed", ArcaneMana, scryAll)
	}
	optimal := ManaScry*(ArcaneNodes-1) + ManaSpell*ArcaneNodes
	if optimal > ArcaneMana {
		t.Fatalf("budget %d cannot fund the optimal line (%d)", ArcaneMana, optimal)
	}
	if !arcaneSolvable(Quest{Mana: ArcaneMana}) {
		t.Fatal("a fresh ritual should be solvable")
	}
}
