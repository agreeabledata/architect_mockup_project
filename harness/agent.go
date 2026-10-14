package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"architect_mockup_project/engine"
)

const reasoningParam = "reasoning"

// promptVersion hashes every run transcript to the prompt it used.
const promptVersion = "v4"

// contextTurns is how many turns of the conversation reach the model: the
// system prompt plus the last N turns. The fresh observation restates the
// world, so older turns add little; the full history stays in the transcript.
const contextTurns = 3

// BuildSystem returns the shared system prompt, identical across providers.
func BuildSystem() string {
	return `You are an adventurer in a small fantasy world. Each turn you are told
your situation and the actions available to you. Think briefly in free text about
the arithmetic of your options, then call exactly one action tool. Every action
spends days of your life, which you cannot get back. Spend them only where the
payoff is worth it.`
}

// BuildTurn renders one observation plus its legal actions as a user message.
func BuildTurn(state engine.State, actions []engine.Action) Message {
	var b strings.Builder
	b.WriteString(state.Narrative)
	b.WriteString("\n\nLegal actions:\n")
	for _, a := range actions {
		b.WriteString("- " + a.Name)
		if a.Description != "" {
			b.WriteString(": " + a.Description)
		}
		b.WriteString("\n")
	}
	return Message{Role: RoleUser, Content: b.String()}
}

// lastTurns windows a conversation down to its system message plus the last
// turns user turns. It slices only at user-message boundaries, so assistant
// tool calls and their results stay paired for the providers.
func lastTurns(msgs []Message, turns int) []Message {
	var users []int
	for i, m := range msgs {
		if m.Role == RoleUser {
			users = append(users, i)
		}
	}
	if len(users) <= turns {
		return msgs
	}
	start := users[len(users)-turns]
	out := make([]Message, 0, len(msgs)-start+1)
	return append(append(out, msgs[0]), msgs[start:]...)
}

func actionsToTools(actions []engine.Action) []Tool {
	tools := make([]Tool, 0, len(actions))
	for _, a := range actions {
		tools = append(tools, Tool{
			Name:        a.Name,
			Description: a.Description,
			Parameters:  withReasoning(a.Parameters),
		})
	}
	return tools
}

// withReasoning copies an engine action schema and requires a reasoning string
// so the model cannot call a tool without writing why. The copy is deliberate:
// LegalActions reuses its maps, and the engine must not learn this field.
func withReasoning(params map[string]any) map[string]any {
	out := map[string]any{"type": "object"}
	props := map[string]any{}
	required := []any{reasoningParam}
	if params != nil {
		if t, ok := params["type"]; ok {
			out["type"] = t
		}
		if p, ok := params["properties"].(map[string]any); ok {
			for k, v := range p {
				props[k] = v
			}
		}
		if req, ok := params["required"]; ok {
			required = prependRequired(req)
		}
		for k, v := range params {
			if k == "type" || k == "properties" || k == "required" {
				continue
			}
			out[k] = v
		}
	}
	props[reasoningParam] = map[string]any{
		"type":        "string",
		"description": "Why this action is the best use of remaining days, given the arithmetic of the options.",
	}
	out["properties"] = props
	out["required"] = required
	return out
}

func prependRequired(v any) []any {
	out := []any{reasoningParam}
	switch req := v.(type) {
	case []any:
		for _, x := range req {
			if s, ok := x.(string); ok && s == reasoningParam {
				continue
			}
			out = append(out, x)
		}
	case []string:
		for _, s := range req {
			if s == reasoningParam {
				continue
			}
			out = append(out, s)
		}
	}
	return out
}

// stripReasoning removes the harness-only reasoning field so Apply sees engine
// args. Malformed JSON is left alone for Apply to reject.
func stripReasoning(args json.RawMessage) json.RawMessage {
	if len(args) == 0 {
		return args
	}
	var m map[string]any
	if err := json.Unmarshal(args, &m); err != nil {
		return args
	}
	delete(m, reasoningParam)
	b, err := json.Marshal(m)
	if err != nil {
		return args
	}
	return b
}

// Agent drives one game run: observe -> chat -> apply until the run is terminal
// or the iteration cap is hit.
type Agent struct {
	Provider   Provider
	Config     *Config
	Transcript *Transcript
}

// Report is the per-run cost and outcome record.
type Report struct {
	FinalState engine.State
	Balance    int
	Iterations int
	ToolCalls  int
	Usage      Usage
	WallTime   time.Duration
	Reason     string // terminal | iteration_cap | decision_cap | no_action | provider_error
	// ProseReplies counts model replies with no tool call. Each one is nudged
	// and spends a retry; only a decision that ends on prose is no_action.
	ProseReplies int
}

// proseNudge follows a reply with no tool call. It is sent for the next
// attempt only and never enters the windowed history.
const proseNudge = "You replied without calling a tool. Choose your next action by calling exactly one of the tools."

// Cost returns the dollar cost of the run on the configured model.
func (r Report) Cost(c *Config) float64 {
	return c.Cost(c.Model, r.Usage.InputTokens, r.Usage.OutputTokens)
}

// Run executes the full agent loop against a game. IterationCap bounds the
// total model calls for the run; a decision whose actions the engine rejects
// retries up to DecisionRetryCap times instead of churning the budget.
func (a *Agent) Run(ctx context.Context, g engine.Game) (Report, error) {
	start := time.Now()
	msgs := []Message{{Role: RoleSystem, Content: BuildSystem()}}
	retryCap := a.Config.DecisionRetryCapOrDefault()
	var report Report
	startRemaining := g.Observe().LifespanDays
decisions:
	for {
		if report.Iterations >= a.Config.IterationCap {
			report.Reason = "iteration_cap"
			break
		}
		state := g.Observe()
		report.FinalState = state
		report.Balance = state.Balance
		if state.Done {
			report.Reason = "terminal"
			break
		}
		actions := g.LegalActions()
		msgs = append(msgs, BuildTurn(state, actions))

		// One decision: chat until an action applies, or the retry cap gives
		// up. A rejected action retries without a fresh observation -- the
		// tool-error feedback is the next attempt's prompt.
		decided := false
		var nudge []Message // the prose reply and proseNudge, for the next attempt only
		for attempt := 0; attempt < retryCap; attempt++ {
			if report.Iterations >= a.Config.IterationCap {
				report.Reason = "iteration_cap"
				break decisions
			}
			window := lastTurns(msgs, contextTurns)
			res, err := a.Provider.Chat(ctx, append(window[:len(window):len(window)], nudge...), actionsToTools(actions), CallOptions{
				Model:       a.Config.Model,
				MaxTokens:   a.Config.MaxTokens,
				Temperature: a.Config.Temperature,
				TopP:        a.Config.TopP,
				Seed:        a.Config.Seed,
			})
			if err != nil {
				return report, fmt.Errorf("agent: %w", err)
			}
			report.Iterations++
			report.Usage = addUsage(report.Usage, res.Usage)
			if a.Transcript != nil {
				if err := a.Transcript.Record(append(msgs[:len(msgs):len(msgs)], nudge...), res); err != nil {
					return report, fmt.Errorf("transcript: %w", err)
				}
			}

			assistant := Message{Role: RoleAssistant, Content: res.Content, ToolCalls: res.ToolCalls}
			if len(res.ToolCalls) == 0 {
				if res.StopReason == StopReasonError {
					report.Reason = "provider_error"
					break decisions
				}
				report.ProseReplies++
				nudge = []Message{{Role: RoleUser, Content: proseNudge}}
				if res.Content != "" { // an empty assistant text block is a 400 on Anthropic
					nudge = append([]Message{assistant}, nudge...)
				}
				continue
			}
			nudge = nil
			msgs = append(msgs, assistant)
			for _, tc := range res.ToolCalls {
				report.ToolCalls++
				before := g.Observe()
				stripped := stripReasoning(tc.Arguments)
				outcome, err := g.Apply(tc.Name, stripped)
				if a.Transcript != nil {
					if recErr := a.Transcript.RecordTurn(newTurnRecord(report.ToolCalls, startRemaining, before, tc.Name, stripped, outcome, err)); recErr != nil {
						return report, fmt.Errorf("transcript: %w", recErr)
					}
				}
				if err != nil {
					outcome = engine.Outcome{Narrative: fmt.Sprintf("Invalid action: %v", err)}
				} else {
					decided = true
				}
				msgs = append(msgs, Message{Role: RoleTool, ToolCallID: tc.ID, Content: outcome.Narrative})
			}
			if decided {
				continue decisions
			}
		}
		// The decision's last attempt decides the label: prose means the model
		// would not act; otherwise its actions kept being rejected. Hitting the
		// iteration cap mid-decision is labeled iteration_cap above instead.
		report.Reason = "decision_cap"
		if nudge != nil {
			report.Reason = "no_action"
		}
		break
	}
	// The cap check precedes Observe, so the last applied action is not yet
	// reflected; re-observe once so Balance/FinalState describe the end state.
	s := g.Observe()
	report.FinalState, report.Balance = s, s.Balance
	report.WallTime = time.Since(start)
	return report, nil
}

// newTurnRecord builds a structured turn from Observe/Outcome integers, not
// narrative text. A rejected Apply uses the pre-action state and sets error.
func newTurnRecord(turn, startRemaining int, before engine.State, action string, args json.RawMessage, outcome engine.Outcome, err error) TurnRecord {
	rec := TurnRecord{
		Turn:   turn,
		Action: action,
		Args:   parseTurnArgs(args),
	}
	if err != nil {
		msg := err.Error()
		rec.Error = &msg
		rec.Day = startRemaining - before.LifespanDays
		rec.Balance = before.Balance
		return rec
	}
	rec.Day = startRemaining - outcome.LifespanDays
	rec.Balance = outcome.Balance
	rec.Earned = outcome.Earned
	rec.DaysSpent = before.LifespanDays - outcome.LifespanDays
	return rec
}
