package harness

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	kindHeader = "header"
	kindCall   = "call"
	kindTurn   = "turn"
)

// TurnRecord is one applied action (including rejects), sourced from Observe
// and Outcome integers — never from narrative text. Day and Balance are the
// state after a successful action (balance already includes earned) and the
// unchanged pre-action state on reject.
type TurnRecord struct {
	Kind      string         `json:"kind"`
	Turn      int            `json:"turn"`
	Day       int            `json:"day"`
	Balance   int            `json:"balance"`
	Action    string         `json:"action"`
	Args      map[string]any `json:"args"`
	Earned    int            `json:"earned"`
	DaysSpent int            `json:"days_spent"`
	Error     *string        `json:"error"`
	// Reasoning is filled by LoadTranscript from the preceding call line; it is
	// not written by RecordTurn.
	Reasoning string `json:"-"`
}

// Transcript writes one JSONL file per run for audit, with a header hashing the
// eval config so every result is attributable to (model x prompt-version x seed).
type Transcript struct {
	w *bufio.Writer
	f *os.File
}

// NewTranscript creates a transcript file under dir with a config header.
// engineSeed is the world seed (engine.New); cfg.Seed is the model sampling seed.
func NewTranscript(dir string, cfg *Config, engineSeed uint32) (*Transcript, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	name := filepath.Join(dir, "run_"+time.Now().UTC().Format("20060102T150405.000000000Z")+".jsonl")
	f, err := os.Create(name)
	if err != nil {
		return nil, err
	}
	t := &Transcript{w: bufio.NewWriter(f), f: f}
	return t, t.write(transcriptHeader(cfg, engineSeed))
}

func transcriptHeader(cfg *Config, engineSeed uint32) map[string]any {
	if cfg != nil && cfg.Provider == "bot" {
		return map[string]any{
			"kind":        kindHeader,
			"provider":    "bot",
			"model":       "bot",
			"engine_seed": engineSeed,
		}
	}
	h := map[string]any{
		"kind":               kindHeader,
		"provider":           cfg.Provider,
		"model":              cfg.Model,
		"prompt_version":     promptVersion,
		"config_hash":        hashConfig(cfg),
		"seed":               cfg.Seed,
		"engine_seed":        engineSeed,
		"iteration_cap":      cfg.IterationCap,
		"decision_retry_cap": cfg.DecisionRetryCap,
		"context_turns":      contextTurns,
		"max_tokens":         cfg.MaxTokens,
	}
	// The price at run time travels with the run, so a later repricing cannot
	// re-price it. Unpriced models (missing or null) record nothing.
	if p := cfg.Pricing[cfg.Model]; p != nil {
		h["pricing"] = p
	}
	return h
}

func (t *Transcript) write(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := t.w.Write(b); err != nil {
		return err
	}
	return t.w.WriteByte('\n')
}

// Record appends one model call and its result to the transcript.
func (t *Transcript) Record(messages []Message, res Result) error {
	return t.write(map[string]any{
		"kind":     kindCall,
		"messages": messages,
		"result": map[string]any{
			"content":     res.Content,
			"tool_calls":  marshalableToolCalls(res.ToolCalls),
			"stop_reason": res.StopReason,
			"usage":       res.Usage,
		},
	})
}

// marshalableToolCalls copies tool calls so invalid Arguments still marshal.
// json.RawMessage holding non-JSON (a misbehaving model) would otherwise drop
// the whole call line.
func marshalableToolCalls(calls []ToolCall) []map[string]any {
	out := make([]map[string]any, len(calls))
	for i, tc := range calls {
		out[i] = map[string]any{
			"ID":        tc.ID,
			"Name":      tc.Name,
			"Arguments": marshalableArgs(tc.Arguments),
		}
	}
	return out
}

func marshalableArgs(raw json.RawMessage) any {
	if len(raw) == 0 || json.Valid(raw) {
		return raw
	}
	return string(raw)
}

// RecordTurn appends one applied action. Readers filter kind=="turn".
func (t *Transcript) RecordTurn(rec TurnRecord) error {
	rec.Kind = kindTurn
	if rec.Args == nil {
		rec.Args = map[string]any{}
	}
	return t.write(rec)
}

// parseTurnArgs decodes stripped engine args into a JSON object. Empty input
// becomes {}. Invalid JSON is kept as args_raw so the payload is not lost.
func parseTurnArgs(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return map[string]any{"args_raw": string(raw)}
	}
	return m
}

// Path returns the transcript file's path.
func (t *Transcript) Path() string {
	return t.f.Name()
}

// Close flushes and closes the file.
func (t *Transcript) Close() error {
	if err := t.w.Flush(); err != nil {
		t.f.Close()
		return err
	}
	return t.f.Close()
}

func hashConfig(c *Config) string {
	h := sha256.New()
	enc := json.NewEncoder(h)
	_ = enc.Encode(map[string]any{
		"provider":           c.Provider,
		"model":              c.Model,
		"base_url":           c.BaseURL,
		"seed":               c.Seed,
		"top_p":              c.TopP,
		"iteration_cap":      c.IterationCap,
		"decision_retry_cap": c.DecisionRetryCap,
	})
	return hex.EncodeToString(h.Sum(nil))
}
