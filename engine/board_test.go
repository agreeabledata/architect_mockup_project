package engine

import "testing"

func TestSeedBoardConstraints(t *testing.T) {
	for seed := uint32(1); seed <= 80; seed++ {
		for refresh := 0; refresh < 4; refresh++ {
			board := SeedBoard(Mulberry32(boardSeed(seed, refresh)), seed, refresh)
			if len(board) != QuestsPerBoard {
				t.Fatalf("seed %d refresh %d: len=%d want %d", seed, refresh, len(board), QuestsPerBoard)
			}
			battles := 0
			arcanes := 0
			seen := map[string]bool{}
			for _, q := range board {
				if seen[q.ID] {
					t.Fatalf("duplicate id %s", q.ID)
				}
				seen[q.ID] = true
				if q.Status != QuestOpen {
					t.Fatalf("status %s", q.Status)
				}
				switch q.Kind {
				case QuestBattle:
					battles++
					if q.Days != CostBattle {
						t.Fatalf("battle days %d", q.Days)
					}
					if q.Payout != PayBattleWin {
						t.Fatalf("battle payout %d", q.Payout)
					}
				case QuestArcane:
					arcanes++
					if q.Days != CostArcane {
						t.Fatalf("arcane days %d", q.Days)
					}
					if q.Payout != PayArcane {
						t.Fatalf("arcane payout %d", q.Payout)
					}
					if q.Mana != ArcaneMana {
						t.Fatalf("arcane mana %d", q.Mana)
					}
					if !isPermutation(q.Nodes) {
						t.Fatalf("arcane nodes %v not a permutation", q.Nodes)
					}
				case QuestDelivery:
					if q.Days < 2 || q.Days > 4 {
						t.Fatalf("delivery days %d", q.Days)
					}
					if q.Payout != payoutForDeliveryDays(q.Days) {
						t.Fatalf("delivery %d days paid %d", q.Days, q.Payout)
					}
				default:
					t.Fatalf("unknown kind %s", q.Kind)
				}
			}
			if battles > MaxBattlesPerBoard {
				t.Fatalf("seed %d refresh %d: %d battles", seed, refresh, battles)
			}
			if arcanes > MaxArcanePerBoard {
				t.Fatalf("seed %d refresh %d: %d arcane", seed, refresh, arcanes)
			}
		}
	}
}

func TestSeedBoardDeterministic(t *testing.T) {
	const seed uint32 = 42
	a := SeedBoard(Mulberry32(boardSeed(seed, 0)), seed, 0)
	b := SeedBoard(Mulberry32(boardSeed(seed, 0)), seed, 0)
	if !boardsEqual(a, b) {
		t.Fatal("same seed produced different first boards")
	}
	c := SeedBoard(Mulberry32(boardSeed(seed, 1)), seed, 1)
	d := SeedBoard(Mulberry32(boardSeed(seed, 1)), seed, 1)
	if !boardsEqual(c, d) {
		t.Fatal("same seed produced different refresh boards")
	}
	if boardsEqual(a, c) {
		t.Fatal("refresh board matched the first board; stream is stuck")
	}
}

func isPermutation(nodes [ArcaneNodes]Resonance) bool {
	seen := map[Resonance]bool{}
	for _, r := range nodes {
		if seen[r] {
			return false
		}
		seen[r] = true
	}
	return len(seen) == ArcaneNodes
}

func boardsEqual(a, b []Quest) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
