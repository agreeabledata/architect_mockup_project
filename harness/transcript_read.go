package harness

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// CallLog is the audit fields from one kind=="call" line: token usage and
// per-tool-call reasoning (kept on the call, stripped from the turn).
type CallLog struct {
	Usage      Usage
	Reasonings []string
	StopReason StopReason
}

// TranscriptLog is a parsed JSONL transcript. Replay and compare read this
// rather than scraping observation prose.
type TranscriptLog struct {
	Header map[string]any
	Calls  []CallLog
	Turns  []TurnRecord
}

// LoadTranscript decodes a JSONL transcript. Unknown kinds are skipped so
// older or newer files still yield their turn records. On a truncated or
// malformed line it returns the records parsed so far plus an error that
// includes the 1-based line number. Reasonings are paired structurally: each
// turn pops one from the most recent preceding call's queue.
func LoadTranscript(r io.Reader) (*TranscriptLog, error) {
	log := &TranscriptLog{Header: map[string]any{}}
	br := bufio.NewReader(r)
	var pending []string
	lineNo := 0
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			lineNo++
			line = bytes.TrimSuffix(line, []byte("\n"))
			line = bytes.TrimSuffix(line, []byte("\r"))
			if len(bytes.TrimSpace(line)) == 0 {
				if err == io.EOF {
					return log, nil
				}
				if err != nil {
					return log, fmt.Errorf("transcript: line %d: %w", lineNo, err)
				}
				continue
			}
			if perr := parseTranscriptLine(log, line, &pending); perr != nil {
				return log, fmt.Errorf("transcript: line %d: %w", lineNo, perr)
			}
		}
		if err == io.EOF {
			return log, nil
		}
		if err != nil {
			if lineNo == 0 {
				return log, err
			}
			return log, fmt.Errorf("transcript: line %d: %w", lineNo, err)
		}
	}
}

func parseTranscriptLine(log *TranscriptLog, raw []byte, pending *[]string) error {
	var meta struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return err
	}
	if meta.Kind == "" {
		return fmt.Errorf("missing kind (pre-#29 transcripts are not replayable)")
	}
	switch meta.Kind {
	case kindHeader:
		if err := json.Unmarshal(raw, &log.Header); err != nil {
			return err
		}
	case kindCall:
		call, err := parseCallLog(raw)
		if err != nil {
			return err
		}
		log.Calls = append(log.Calls, call)
		*pending = append([]string(nil), call.Reasonings...)
	case kindTurn:
		var rec TurnRecord
		if err := json.Unmarshal(raw, &rec); err != nil {
			return err
		}
		if rec.Args == nil {
			rec.Args = map[string]any{}
		}
		if len(*pending) > 0 {
			rec.Reasoning = (*pending)[0]
			*pending = (*pending)[1:]
		}
		log.Turns = append(log.Turns, rec)
	}
	return nil
}

func parseCallLog(raw json.RawMessage) (CallLog, error) {
	var rec struct {
		Result struct {
			ToolCalls  []json.RawMessage `json:"tool_calls"`
			Usage      Usage             `json:"usage"`
			StopReason StopReason        `json:"stop_reason"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &rec); err != nil {
		return CallLog{}, err
	}
	call := CallLog{Usage: rec.Result.Usage, StopReason: rec.Result.StopReason}
	for _, tc := range rec.Result.ToolCalls {
		call.Reasonings = append(call.Reasonings, reasoningFromToolCall(tc))
	}
	return call, nil
}

func reasoningFromToolCall(raw json.RawMessage) string {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	args := m["Arguments"]
	if args == nil {
		args = m["arguments"]
	}
	switch a := args.(type) {
	case map[string]any:
		s, _ := a[reasoningParam].(string)
		return s
	case string:
		return reasoningFromArgs(json.RawMessage(a))
	}
	return ""
}

func reasoningFromArgs(raw json.RawMessage) string {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	s, _ := m[reasoningParam].(string)
	return s
}

// StatsFromLog reads the comparison-table numbers off structured records.
func (l *TranscriptLog) Stats() RunStats {
	st := RunStats{}
	if l == nil {
		return st
	}
	if m, ok := l.Header["model"].(string); ok {
		st.Model = m
	}
	st.ModelCalls = len(l.Calls)
	for _, c := range l.Calls {
		st.InputTokens += c.Usage.InputTokens
		st.OutputTokens += c.Usage.OutputTokens
		if len(c.Reasonings) == 0 && c.StopReason != StopReasonError {
			st.ProseReplies++
		}
	}
	for _, t := range l.Turns {
		if t.Error != nil {
			st.InvalidActions++
		}
	}
	if n := len(l.Turns); n > 0 {
		st.FinalBalance = l.Turns[n-1].Balance
	}
	return st
}

// RunStats is one transcript's contribution to the model comparison table.
type RunStats struct {
	Model          string
	FinalBalance   int
	ModelCalls     int
	InputTokens    int64
	OutputTokens   int64
	InvalidActions int
	ProseReplies   int    // model replies with no tool call (each nudged); provider errors excluded
	Reason         string // terminal | iteration_cap | decision_cap | no_action | provider_error; empty for bot logs
}
