package engine

import "testing"

func TestAttackPower(t *testing.T) {
	cases := []struct{ coins, want int }{
		{0, 1}, {9, 1}, {10, 1}, {19, 1}, {20, 2}, {50, 5}, {100, 10}, {150, 15},
	}
	for _, c := range cases {
		if got := AttackPower(c.coins); got != c.want {
			t.Fatalf("%d coins → %d want %d", c.coins, got, c.want)
		}
	}
}

func TestBattleNoSpend(t *testing.T) {
	b := ResolveBattle(StartBattle(150, 50, 100, ""))
	if b.Coins != 150 {
		t.Fatalf("coins spent: %d", b.Coins)
	}
	if b.Result != BattleWin {
		t.Fatalf("result=%s", b.Result)
	}
}

func TestForecastLossAtZero(t *testing.T) {
	f := Forecast(0, 50, 100)
	if f.Result != BattleLoss {
		t.Fatalf("want loss, got %s", f.Result)
	}
}
