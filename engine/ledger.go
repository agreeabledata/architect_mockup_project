package engine

// LedgerEvent is one line on the lifespan page.
type LedgerEvent struct {
	Kind  string
	Days  int
	Krone int
	Note  string
}

// Ledger is the agent's remaining life and krone purse. HP is not stored:
// every duel is a fresh AgentHP vs OpponentHP (see WinsBattle).
type Ledger struct {
	Days   int
	Krone  int
	Events []LedgerEvent
}

func newLedger() Ledger {
	return Ledger{
		Days:  StartingLifespanDays,
		Krone: StartingBalance,
		Events: []LedgerEvent{{
			Kind: "new", Days: StartingLifespanDays,
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

func totalEarned(events []LedgerEvent) int {
	sum := 0
	for _, e := range events {
		if e.Krone > 0 {
			sum += e.Krone
		}
	}
	return sum
}
