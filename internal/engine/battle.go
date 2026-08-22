package engine

const (
	AttackBudget    = 10
	AgentStartHP    = 50
	OpponentStartHP = 100
	OpponentDamage  = 8
)

func AttackPower(coins int) int {
	if coins < 0 {
		coins = 0
	}
	p := coins / 10
	if p < 1 {
		return 1
	}
	return p
}

func StartBattle(coins int, agentHP, opponentHP int, opponentName string) Battle {
	if agentHP <= 0 {
		agentHP = AgentStartHP
	}
	if opponentHP <= 0 {
		opponentHP = OpponentStartHP
	}
	if opponentName == "" {
		opponentName = "Roof thug"
	}
	if coins < 0 {
		coins = 0
	}
	return Battle{
		Coins:        coins,
		Power:        AttackPower(coins),
		AgentHP:      agentHP,
		OpponentHP:   opponentHP,
		OpponentName: opponentName,
		Budget:       AttackBudget,
		Result:       BattleOngoing,
	}
}

func StepBattle(b Battle) Battle {
	if b.Result != BattleOngoing {
		return b
	}
	power := AttackPower(b.Coins)
	n := len(b.Log) + 1
	oppHP := b.OpponentHP - power
	agentHP := b.AgentHP
	budget := b.Budget - 1
	result := BattleOngoing
	lossReason := ""
	phase := "agent"
	note := ""

	if oppHP <= 0 {
		oppHP = 0
		result = BattleWin
		phase = "end"
		note = "Strike: foe down. Win."
	} else {
		agentHP -= OpponentDamage
		phase = "opponent"
		note = "Strike: foe answers."
		if agentHP <= 0 {
			agentHP = 0
			result = BattleLoss
			lossReason = "hp"
			phase = "end"
			note = "Strike: you fall."
		} else if budget <= 0 {
			result = BattleLoss
			lossReason = "budget"
			phase = "end"
			note = "Strike: budget spent."
		}
	}

	turn := BattleTurn{
		N: n, Power: power, OpponentHP: oppHP, AgentHP: agentHP,
		Budget: budget, Phase: phase, Note: note,
	}
	b.Power = power
	b.OpponentHP = oppHP
	b.AgentHP = agentHP
	b.Budget = budget
	b.Result = result
	b.LossReason = lossReason
	b.Log = append(b.Log, turn)
	return b
}

func ResolveBattle(b Battle) Battle {
	for b.Result == BattleOngoing {
		b = StepBattle(b)
	}
	return b
}

func Forecast(coins, agentHP, opponentHP int) BattleForecast {
	if agentHP <= 0 {
		agentHP = AgentStartHP
	}
	if opponentHP <= 0 {
		opponentHP = OpponentStartHP
	}
	resolved := ResolveBattle(StartBattle(coins, agentHP, opponentHP, ""))
	reason := "agent_down"
	if resolved.Result == BattleWin {
		reason = "opponent_down"
	} else if resolved.LossReason == "budget" {
		reason = "budget"
	}
	res := BattleLoss
	if resolved.Result == BattleWin {
		res = BattleWin
	}
	return BattleForecast{
		Result: res, Reason: reason, Turns: len(resolved.Log),
		AgentHP: resolved.AgentHP, OpponentHP: resolved.OpponentHP, Power: resolved.Power,
	}
}

func KroneToWin(agentHP, opponentHP int) int {
	for coins := 0; coins <= 5000; coins += 10 {
		if Forecast(coins, agentHP, opponentHP).Result == BattleWin {
			return coins
		}
	}
	return 5000
}
