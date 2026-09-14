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
// and Outcome integers — never from narrative text.
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
}

// Transcript writes one JSONL file per run for audit, with a header hashing the
// eval config so every result is attributable to (model x prompt-version x seed).
type Transcript struct {
    w *bufio.Writer
    f *os.File
}

// NewTranscript creates a transcript file under dir with a config header.
func NewTranscript(dir string, cfg *Config) (*Transcript, error) {
    if err := os.MkdirAll(dir, 0o755); err != nil {
        return nil, err
    }
    name := filepath.Join(dir, "run_"+time.Now().UTC().Format("20060102T150405.000000000Z")+".jsonl")
    f, err := os.Create(name)
    if err != nil {
        return nil, err
    }
    t := &Transcript{w: bufio.NewWriter(f), f: f}
    header := map[string]any{
        "kind":               kindHeader,
        "provider":           cfg.Provider,
        "model":              cfg.Model,
        "prompt_version":     promptVersion,
        "config_hash":        hashConfig(cfg),
        "seed":               cfg.Seed,
        "iteration_cap":      cfg.IterationCap,
        "decision_retry_cap": cfg.DecisionRetryCap,
        "context_turns":      contextTurns,
        "max_tokens":         cfg.MaxTokens,
    }
    return t, t.write(header)
}

func (t *Transcript) write(v any) error {
    b, err := json.Marshal(v)
    if err != nil {
        return err
    }
    t.w.Write(b)
    return t.w.WriteByte('\n')
}

// Record appends one model call and its result to the transcript.
func (t *Transcript) Record(messages []Message, res Result) {
    _ = t.write(map[string]any{
        "kind":     kindCall,
        "messages": messages,
        "result": map[string]any{
            "content":     res.Content,
            "tool_calls":  res.ToolCalls,
            "stop_reason": res.StopReason,
            "usage":       res.Usage,
        },
    })
}

// RecordTurn appends one applied action. Readers filter kind=="turn".
func (t *Transcript) RecordTurn(rec TurnRecord) {
    rec.Kind = kindTurn
    if rec.Args == nil {
        rec.Args = map[string]any{}
    }
    _ = t.write(rec)
}

// parseTurnArgs decodes stripped engine args into a JSON object. Empty or
// invalid input becomes {}.
func parseTurnArgs(raw json.RawMessage) map[string]any {
    if len(raw) == 0 {
        return map[string]any{}
    }
    var m map[string]any
    if err := json.Unmarshal(raw, &m); err != nil || m == nil {
        return map[string]any{}
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
