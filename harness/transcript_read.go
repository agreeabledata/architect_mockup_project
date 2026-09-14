package harness

import (
	"encoding/json"
	"io"
)

// CallLog is the audit fields from one kind=="call" line: token usage and
// per-tool-call reasoning (kept on the call, stripped from the turn).
type CallLog struct {
	Usage      Usage
	Reasonings []string
}

// TranscriptLog is a parsed JSONL transcript. Replay and compare read this
// rather than scraping observation prose.
type TranscriptLog struct {
	Header map[string]any
	Calls  []CallLog
	Turns  []TurnRecord
}

// LoadTranscript decodes a JSONL transcript. Unknown kinds are skipped so
// older or newer files still yield their turn records.
func LoadTranscript(r io.Reader) (*TranscriptLog, error) {
	log := &TranscriptLog{Header: map[string]any{}}
	dec := json.NewDecoder(r)
	for {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				return log, nil
			}
			return nil, err
		}
		var meta struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(raw, &meta); err != nil {
			return nil, err
		}
		switch meta.Kind {
		case kindHeader:
			if err := json.Unmarshal(raw, &log.Header); err != nil {
				return nil, err
			}
		case kindCall:
			call, err := parseCallLog(raw)
			if err != nil {
				return nil, err
			}
			log.Calls = append(log.Calls, call)
		case kindTurn:
			var rec TurnRecord
			if err := json.Unmarshal(raw, &rec); err != nil {
				return nil, err
			}
			if rec.Args == nil {
				rec.Args = map[string]any{}
			}
			log.Turns = append(log.Turns, rec)
		}
	}
}

func parseCallLog(raw json.RawMessage) (CallLog, error) {
	var rec struct {
		Result struct {
			ToolCalls []json.RawMessage `json:"tool_calls"`
			Usage     Usage             `json:"usage"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &rec); err != nil {
		return CallLog{}, err
	}
	call := CallLog{Usage: rec.Result.Usage}
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
}
