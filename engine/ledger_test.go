package engine

import "testing"

func TestNewLedgerStart(t *testing.T) {
	l := newLedger()
	if l.Days != StartingLifespanDays {
		t.Fatalf("days = %d, want %d", l.Days, StartingLifespanDays)
	}
	if l.Krone != StartingBalance {
		t.Fatalf("krone = %d, want %d", l.Krone, StartingBalance)
	}
}

func TestDebitDaysClampsAtZero(t *testing.T) {
	l := newLedger()
	l = l.debitDays(StartingLifespanDays+10, "travel", "overspend")
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
