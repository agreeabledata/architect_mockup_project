package engine

const StartDays = 24

func CreateLedger() Ledger {
	return Ledger{
		Days:  StartDays,
		Krone: 0,
		HP:    AgentStartHP,
		MaxHP: AgentStartHP,
		Events: []LedgerEvent{{
			T: 0, Kind: "new", Days: StartDays, Note: "Ledger opened. Twenty-four days on the page.",
		}},
	}
}

func DebitDays(l Ledger, n, tick int, note, kind string) Ledger {
	days := l.Days - n
	if days < 0 {
		days = 0
	}
	l.Days = days
	l.Events = append(l.Events, LedgerEvent{T: tick, Kind: kind, Days: -n, Note: note})
	return l
}

func CreditKrone(l Ledger, n, tick int, note string) Ledger {
	l.Krone += n
	l.Deliveries++
	l.Events = append(l.Events, LedgerEvent{T: tick, Kind: "delivery", Krone: n, Note: note})
	return l
}

func SetHP(l Ledger, hp int) Ledger {
	if hp < 0 {
		hp = 0
	}
	if hp > l.MaxHP {
		hp = l.MaxHP
	}
	l.HP = hp
	return l
}

func RestHP(l Ledger) Ledger {
	l.HP = l.MaxHP
	return l
}
