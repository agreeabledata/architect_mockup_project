package stub

import (
	"testing"

	"architect_mockup_project/engine"
)

func TestStubDelivery(t *testing.T) {
	s := New()
	if s.Observe().Done {
		t.Fatal("run should not start done")
	}
	if _, err := s.Apply("take_delivery", nil); err != nil {
		t.Fatal(err)
	}
	out, err := s.Apply("deliver_package", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Balance != deliveryPayout {
		t.Errorf("balance = %d, want %d", out.Balance, deliveryPayout)
	}
	if out.Earned != deliveryPayout {
		t.Errorf("earned = %d, want %d", out.Earned, deliveryPayout)
	}
	if s.Observe().HealthDays != startHealth-pickupCost-deliveryCost {
		t.Errorf("health = %d", s.Observe().HealthDays)
	}
}

// The duel is the word problem: whether it wins must match the arithmetic the
// narrative states (attack = max(1, floor(krones/10)), 50 stamina vs 8 dmg/round).
func TestStubFightMath(t *testing.T) {
	rich := New()
	rich.balance = 150 // attack 15, kills in 7 rounds, dies in 7 -> wins
	out, err := rich.Apply("fight", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Balance != 150+fightPayout {
		t.Errorf("rich balance = %d, want %d", out.Balance, 150+fightPayout)
	}

	poor := New() // attack 1, can never win
	out, err = poor.Apply("fight", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Balance != -engine.DuelLossPenalty {
		t.Errorf("poor balance = %d, want %d", out.Balance, -engine.DuelLossPenalty)
	}
}

func TestStubInvalidAction(t *testing.T) {
	s := New()
	if _, err := s.Apply("deliver_package", nil); err == nil {
		t.Fatal("delivering without a package should error")
	}
	if _, err := s.Apply("nonsense", nil); err == nil {
		t.Fatal("unknown action should error")
	}
}
