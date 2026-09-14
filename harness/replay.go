package harness

import (
	"fmt"
	"io"
	"strings"
)

const replayActionWidth = 24

// Replay prints one terminal line per structured turn, in file order.
// On a truncated file it still returns the lines parsed so far plus the error.
func Replay(r io.Reader) ([]string, error) {
	log, err := LoadTranscript(r)
	if log == nil {
		return nil, err
	}
	lines := make([]string, len(log.Turns))
	for i, rec := range log.Turns {
		lines[i] = FormatTurn(rec, rec.Reasoning)
	}
	return lines, err
}

// FormatTurn renders one kind=="turn" record as a replay line. Success lines
// show pre→post day and balance (post-action is what TurnRecord stores).
func FormatTurn(rec TurnRecord, reasoning string) string {
	action := rec.Action
	if id := questIDArg(rec.Args); id != "" {
		action += " " + id
	}
	var b strings.Builder
	if rec.Error != nil {
		fmt.Fprintf(&b, "day %d  bal %d  %-*s  invalid: %s", rec.Day, rec.Balance, replayActionWidth, action, *rec.Error)
	} else {
		beforeDay := rec.Day - rec.DaysSpent
		beforeBal := rec.Balance - rec.Earned
		fmt.Fprintf(&b, "day %d→%d  bal %d→%d  %-*s  %+d  %s",
			beforeDay, rec.Day, beforeBal, rec.Balance, replayActionWidth, action, rec.Earned, daysPhrase(rec.DaysSpent))
	}
	if reasoning != "" {
		fmt.Fprintf(&b, "   reasoning: %q", reasoning)
	}
	return b.String()
}

func questIDArg(args map[string]any) string {
	if args == nil {
		return ""
	}
	s, _ := args["quest_id"].(string)
	return s
}

func daysPhrase(n int) string {
	if n == 1 {
		return "(1 day)"
	}
	return fmt.Sprintf("(%d days)", n)
}
