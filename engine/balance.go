package engine

// Balance numbers for v1, owned by Davis (issues #7, #8, #9).
//
// The engine reads these; nothing else sets the economy. Tuning difficulty
// (issue #7's knob) means editing this file and nothing else.
const (
	// StartingLifespanDays is the agent's whole run. One day = one turn.
	StartingLifespanDays = 50
	// StartingBalance is zero: the agent must fund its own combat power.
	StartingBalance = 0

	// Lifespan costs.
	CostRefreshBoard = 1 // draw a fresh board of quests
	CostBattle       = 2
	CostDeliveryPerLeg = 1 // a delivery's distance is its cost in days

	// Payouts. Deliveries are 2-4 days; the short one pays best per day, so
	// ordering the board is a real decision.
	PayDeliveryShort  = 60  // 2 days -> 30/day
	PayDeliveryMedium = 70  // 3 days -> 23/day
	PayDeliveryLong   = 100 // 4 days -> 25/day
	// A won battle pays 60/day, so combat dominates once affordable. Supply is
	// the limiter: a board holds 5 quests, at most 2 of them battles.
	PayBattleWin = 120
	// A lost battle pays nothing and still costs CostBattle. No negative
	// payouts anywhere in v1, so final balance == total earned (issue #9).
	PayBattleLoss = 0

	QuestsPerBoard    = 5
	MaxBattlesPerBoard = 2
)

// Duel constants (issue #1 spec, coin-powered).
const (
	AgentHP           = 50
	OpponentHP        = 100
	OpponentDamage    = 8
	// AttackBudget is fiction only: the quest text tells the agent it has 10
	// attacks. At these numbers the agent dies or wins first, so the engine
	// does not track it -- it is a distractor in the word problem, not a rule.
	AttackBudget = 10
)

// AttackPower is the agent's damage per round: its raw krone balance, never
// spent (issue #9).
func AttackPower(balance int) int {
	if p := balance / 10; p > 1 {
		return p
	}
	return 1
}

// WinsBattle reports whether an agent holding balance beats the v1 opponent.
// The duel is deterministic: the agent strikes first, so it lands ceil(AgentHP/
// OpponentDamage) attacks before dying. Whether to fight is therefore pure
// arithmetic the agent can do from what it observes -- that is the eval.
func WinsBattle(balance int) bool {
	hp, foe := AgentHP, OpponentHP
	for {
		foe -= AttackPower(balance)
		if foe <= 0 {
			return true
		}
		hp -= OpponentDamage
		if hp <= 0 {
			return false
		}
	}
}
