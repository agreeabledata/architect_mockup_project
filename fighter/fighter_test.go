package fighter

import (
	"testing"
)

func TestGetReady(t *testing.T) {
	f := New("Courier", "#efe8dc")
	f.GetReady(16, 15)
	if f.Life() != MaxLife {
		t.Fatalf("life=%v want %v", f.Life(), MaxLife)
	}
	if f.State() != StateIdle {
		t.Fatalf("state=%v want IDLE", f.State())
	}
	if f.Position().X != 16 || f.Position().Y != 15 {
		t.Fatalf("pos=%v", f.Position())
	}
}

func TestPunchActiveWindow(t *testing.T) {
	f := New("You", "#fff")
	f.GetReady(0, 0)
	f.Punch()
	if !f.IsAttacking() {
		t.Fatal("should be attacking after Punch")
	}
	// too early
	if f.IsAttackActive() {
		t.Fatal("attack should not be active at t=0")
	}
	// middle third of punch duration (0.05 * 6 = 0.30s → window ~0.10..0.20)
	f.Update(0.12)
	if !f.IsAttackActive() {
		t.Fatalf("attack should be active at t=%v", f.stateTime)
	}
	f.MakeContact()
	if f.IsAttackActive() {
		t.Fatal("attack inactive after contact")
	}
}

func TestBlockReducesDamage(t *testing.T) {
	f := New("You", "#fff")
	f.GetReady(0, 0)
	f.Block()
	f.GetHit(HitStrength)
	want := MaxLife - HitStrength*BlockDamageFactor
	if f.Life() != want {
		t.Fatalf("life=%v want %v", f.Life(), want)
	}
	if f.State() != StateBlock {
		t.Fatalf("should stay blocking, got %v", f.State())
	}
}

func TestUnblockedHitHurts(t *testing.T) {
	f := New("You", "#fff")
	f.GetReady(0, 0)
	f.GetHit(HitStrength)
	if f.State() != StateHurt {
		t.Fatalf("state=%v want HURT", f.State())
	}
	if f.Life() != MaxLife-HitStrength {
		t.Fatalf("life=%v", f.Life())
	}
}

func TestKill(t *testing.T) {
	f := New("Foe", "#aaa")
	f.GetReady(0, 0)
	for i := 0; i < 25; i++ {
		if f.HasLost() {
			break
		}
		// leave hurt so next hit can apply
		if f.State() == StateHurt {
			f.Update(f.animDur[StateHurt] + 0.01)
		}
		f.GetHit(HitStrength)
	}
	if !f.HasLost() {
		t.Fatalf("should have lost, life=%v state=%v", f.Life(), f.State())
	}
}

func TestWalk(t *testing.T) {
	f := New("You", "#fff")
	f.GetReady(0, 0)
	f.MoveRight()
	f.Update(1.0)
	if f.Position().X != MovementSpeed {
		t.Fatalf("x=%v want %v", f.Position().X, MovementSpeed)
	}
	if f.State() != StateWalk {
		t.Fatalf("state=%v", f.State())
	}
}
