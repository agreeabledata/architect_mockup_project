package battle

import (
	"testing"

	"github.com/mfeinLearn/krone/fighter"
)

func TestPunchConnectsInWindow(t *testing.T) {
	d := NewDuel("You", "Thug")
	d.StartRound(16, 20, 15) // within contact X distance

	d.Player.Punch()
	// advance into active window (~0.12s of 0.30s punch)
	d.Step(0.12)

	if d.Opponent.Life() >= fighter.MaxLife {
		t.Fatalf("opponent should have taken damage, life=%v", d.Opponent.Life())
	}
	if d.Player.HasMadeContact() != true {
		t.Fatal("player should have made contact")
	}
}

func TestBlockMitigates(t *testing.T) {
	d := NewDuel("You", "Thug")
	d.StartRound(16, 20, 15)
	d.Opponent.Block()
	d.Player.Punch()
	d.Step(0.12)

	want := fighter.MaxLife - fighter.HitStrength*fighter.BlockDamageFactor
	if d.Opponent.Life() != want {
		t.Fatalf("life=%v want %v", d.Opponent.Life(), want)
	}
}
