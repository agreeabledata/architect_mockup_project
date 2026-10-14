package engine

import "testing"

func TestNewLedgerStart(t *testing.T) {
	l := newLedger()
	if l.Days != StartingHealthDays {
		t.Fatalf("days = %d, want %d", l.Days, StartingHealthDays)
	}
	if l.Krone != StartingBalance {
		t.Fatalf("krone = %d, want %d", l.Krone, StartingBalance)
	}
}

func TestDebitDaysClampsAtZero(t *testing.T) {
	l := newLedger()
	l = l.debitDays(StartingHealthDays+10, "travel", "overspend")
	if l.Days != 0 {
		t.Fatalf("days = %d, want 0", l.Days)
	}
}

func TestCreditKroneAdds(t *testing.T) {
	l := newLedger()
	l = l.creditKrone(PayDeliveryShort, "delivery", "ok")
	if l.Krone != PayDeliveryShort {
		t.Fatalf("krone = %d, want %d", l.Krone, PayDeliveryShort)
	}
	if totalEarned(l.Events) != PayDeliveryShort {
		t.Fatalf("earned = %d", totalEarned(l.Events))
	}
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
