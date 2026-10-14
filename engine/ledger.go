package engine

// LedgerEvent is one line on the health page.
type LedgerEvent struct {
	Kind  string
	Days  int
	Krone int
	Note  string
}

// Ledger is the agent's remaining health (days) and krone purse. Combat
// stamina is not stored: every duel is a fresh AgentStamina vs OpponentHP (see
// WinsBattle).
type Ledger struct {
	Days   int
	Krone  int
	Events []LedgerEvent
}

func newLedger() Ledger {
	return Ledger{
		Days:  StartingHealthDays,
		Krone: StartingBalance,
		Events: []LedgerEvent{{
			Kind: "new", Days: StartingHealthDays,
			Note: "Ledger opened. Fifty days on the page.",
		}},
	}
}

func (l Ledger) debitDays(n int, kind, note string) Ledger {
	days := l.Days - n
	if days < 0 {
		days = 0
	}
	l.Days = days
	l.Events = append(append([]LedgerEvent(nil), l.Events...), LedgerEvent{
		Kind: kind, Days: -n, Note: note,
	})
	return l
}

func (l Ledger) creditKrone(n int, kind, note string) Ledger {
	l.Krone += n
	l.Events = append(append([]LedgerEvent(nil), l.Events...), LedgerEvent{
		Kind: kind, Krone: n, Note: note,
	})
	return l
}
