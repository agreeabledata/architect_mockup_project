package harness

import (
	"fmt"
	"io"
	"strings"
)

const replayActionWidth = 18

// Replay prints one terminal line per structured turn, in file order.
func Replay(r io.Reader) ([]string, error) {
	log, err := LoadTranscript(r)
	if err != nil {
		return nil, err
	}
	reasonings := flattenReasonings(log.Calls)
	lines := make([]string, len(log.Turns))
	for i, rec := range log.Turns {
		reason := ""
		if i < len(reasonings) {
			reason = reasonings[i]
		}
		lines[i] = FormatTurn(rec, reason)
	}
	return lines, nil
}

func flattenReasonings(calls []CallLog) []string {
	var out []string
	for _, c := range calls {
		out = append(out, c.Reasonings...)
	}
	return out
}

// FormatTurn renders one kind=="turn" record as a replay line.
func FormatTurn(rec TurnRecord, reasoning string) string {
	action := rec.Action
	if id := questIDArg(rec.Args); id != "" {
		action += " " + id
	}
	var b strings.Builder
	fmt.Fprintf(&b, "day %d  bal %d  %-*s", rec.Day, rec.Balance, replayActionWidth, action)
	if rec.Error != nil {
		fmt.Fprintf(&b, "  invalid: %s", *rec.Error)
	} else {
		fmt.Fprintf(&b, "  %+d  %s", rec.Earned, daysPhrase(rec.DaysSpent))
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
