package engine

import "fmt"

// QuestKind is a quest type: courier delivery, combat, or arcane ritual.
type QuestKind string

const (
	QuestDelivery QuestKind = "delivery"
	QuestBattle   QuestKind = "battle"
	QuestArcane   QuestKind = "arcane"
)

// Resonance is an arcane node's hidden element. The three form a permutation
// across a ritual's nodes, so the last is deducible once the others are known.
type Resonance string

const (
	ResonanceFire  Resonance = "fire"
	ResonanceWater Resonance = "water"
	ResonanceEarth Resonance = "earth"
)

// resonances is the base set every ritual permutes.
var resonances = [ArcaneNodes]Resonance{ResonanceFire, ResonanceWater, ResonanceEarth}

// QuestStatus tracks a posted job through accept and resolve.
type QuestStatus string

const (
	QuestOpen     QuestStatus = "open"
	QuestActive   QuestStatus = "active"
	QuestComplete QuestStatus = "complete"
)

// Quest is one job on the board.
type Quest struct {
	ID     string
	Kind   QuestKind
	Title  string
	Blurb  string
	Days   int
	Payout int
	Status QuestStatus

	// Arcane-only ritual state. Nodes is hidden from the agent until scried;
	// fixed arrays (not slices) keep Quest comparable so tests can diff boards.
	Nodes    [ArcaneNodes]Resonance
	Revealed [ArcaneNodes]bool
	NextNode int
	Mana     int
}

var deliveryTitles = []string{
	"Harbour parcel",
	"Clocktower packet",
	"Lantern-row dispatch",
	"Old Ward letter",
	"Night-market crate",
	"Pier 4 run",
	"High Spine packet",
}

var battleTitles = []string{
	"Bridge warband",
	"Roof thug",
	"Gate duelist",
	"High-spine raiders",
	"Harbour brutes",
}

var arcaneTitles = []string{
	"The Sunken Rite",
	"Emberwell Dispersal",
	"The Tidebound Circle",
	"Stonesong Unbinding",
}

// SeedBoard draws QuestsPerBoard jobs from rng, with at most MaxBattlesPerBoard
// battles and MaxArcanePerBoard arcane rituals. Delivery length is 2, 3, or 4
// days, paying the table in balance.go. Same (rng stream, seed, refresh) →
// same board.
func SeedBoard(rng func() float64, seed uint32, refresh int) []Quest {
	nBattles := randInt(rng, MaxBattlesPerBoard+1)
	nArcane := randInt(rng, MaxArcanePerBoard+1)
	order := shuffle(rng, QuestsPerBoard)
	kinds := make([]QuestKind, QuestsPerBoard)
	for i := range kinds {
		kinds[i] = QuestDelivery
	}
	for i := 0; i < nBattles; i++ {
		kinds[order[i]] = QuestBattle
	}
	for i := nBattles; i < nBattles+nArcane; i++ {
		kinds[order[i]] = QuestArcane
	}

	out := make([]Quest, 0, QuestsPerBoard)
	for i := 0; i < QuestsPerBoard; i++ {
		id := fmt.Sprintf("q-%d-%d-%d", seed, refresh, i)
		switch kinds[i] {
		case QuestBattle:
			title := battleTitles[randInt(rng, len(battleTitles))]
			out = append(out, Quest{
				ID:     id,
				Kind:   QuestBattle,
				Title:  title,
				Blurb:  battleBlurb(),
				Days:   CostBattle,
				Payout: PayBattleWin,
				Status: QuestOpen,
			})
		case QuestArcane:
			out = append(out, Quest{
				ID:     id,
				Kind:   QuestArcane,
				Title:  arcaneTitles[randInt(rng, len(arcaneTitles))],
				Blurb:  arcaneBlurb(),
				Days:   CostArcane,
				Payout: PayArcane,
				Status: QuestOpen,
				Nodes:  resonancePermutation(rng),
				Mana:   ArcaneMana,
			})
		default:
			days := MinDeliveryDays + randInt(rng, MaxDeliveryDays-MinDeliveryDays+1)
			title := deliveryTitles[randInt(rng, len(deliveryTitles))]
			out = append(out, Quest{
				ID:     id,
				Kind:   QuestDelivery,
				Title:  title,
				Blurb:  deliveryBlurb(days, payoutForDeliveryDays(days)),
				Days:   days,
				Payout: payoutForDeliveryDays(days),
				Status: QuestOpen,
			})
		}
	}
	return out
}

// resonancePermutation deals the three resonances into a fresh order.
func resonancePermutation(rng func() float64) [ArcaneNodes]Resonance {
	idx := shuffle(rng, ArcaneNodes)
	var out [ArcaneNodes]Resonance
	for i, j := range idx {
		out[i] = resonances[j]
	}
	return out
}

func payoutForDeliveryDays(days int) int {
	switch days {
	case 2:
		return PayDeliveryShort
	case 3:
		return PayDeliveryMedium
	default:
		return PayDeliveryLong
	}
}

func deliveryBlurb(days, pay int) string {
	return fmt.Sprintf("A sealed courier packet. %d days on the road, %d krones if it arrives.", days, pay)
}

func battleBlurb() string {
	return fmt.Sprintf(
		"A foe (%d HP) holds the road. Each round it deals %d damage to your %d HP. Tales speak of a %d-attack budget. Your attack power is max(1, floor(krones/10)). Winning pays %d krones; losing pays nothing.",
		OpponentHP, OpponentDamage, AgentHP, AttackBudget, PayBattleWin,
	)
}

// arcaneBlurb states the ritual's public rules. The node resonances are never
// here -- the agent must scry or deduce them.
func arcaneBlurb() string {
	return fmt.Sprintf(
		"A ritual of %d nodes, each keyed to a different resonance (fire, water, earth) -- every resonance appears exactly once, but which node holds which is hidden. Disrupt the nodes in order. Each spell costs %d mana; a wrong spell backlashes for %d more. Scrying a node costs %d mana and reveals its resonance. You start with %d mana. Succeed and you are paid %d krones; run dry and the ritual collapses, paying nothing. Either way it costs %d days.",
		ArcaneNodes, ManaSpell, ManaBacklash, ManaScry, ArcaneMana, PayArcane, CostArcane,
	)
}

func boardSeed(seed uint32, refresh int) uint32 {
	return seed + uint32(refresh)
}
