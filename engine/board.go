package engine

import "fmt"

// QuestKind is a v1 quest type: courier delivery or combat.
type QuestKind string

const (
	QuestDelivery QuestKind = "delivery"
	QuestBattle   QuestKind = "battle"
)

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

// SeedBoard draws QuestsPerBoard jobs from rng, with at most MaxBattlesPerBoard
// battles. Delivery length is 2, 3, or 4 days, paying the table in balance.go.
// Same (rng stream, seed, refresh) → same board.
func SeedBoard(rng func() float64, seed uint32, refresh int) []Quest {
	nBattles := randInt(rng, MaxBattlesPerBoard+1)
	order := shuffle(rng, QuestsPerBoard)
	isBattle := make([]bool, QuestsPerBoard)
	for i := 0; i < nBattles; i++ {
		isBattle[order[i]] = true
	}

	out := make([]Quest, 0, QuestsPerBoard)
	for i := 0; i < QuestsPerBoard; i++ {
		id := fmt.Sprintf("q-%d-%d-%d", seed, refresh, i)
		if isBattle[i] {
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
			continue
		}
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

func boardSeed(seed uint32, refresh int) uint32 {
	return seed + uint32(refresh)
}
