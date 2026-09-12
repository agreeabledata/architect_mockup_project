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
const promptVersion = "v2"

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
	Reason     string // terminal | iteration_cap | no_action
}

// Cost returns the dollar cost of the run on the configured model.
func (r Report) Cost(c *Config) float64 {
	return c.Cost(c.Model, r.Usage.InputTokens, r.Usage.OutputTokens)
}

// Run executes the full agent loop against a game.
func (a *Agent) Run(ctx context.Context, g engine.Game) (Report, error) {
	start := time.Now()
	msgs := []Message{{Role: RoleSystem, Content: BuildSystem()}}
	var report Report
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

		res, err := a.Provider.Chat(ctx, msgs, actionsToTools(actions), CallOptions{
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
			a.Transcript.Record(msgs, res)
		}

		assistant := Message{Role: RoleAssistant, Content: res.Content, ToolCalls: res.ToolCalls}
		msgs = append(msgs, assistant)
		if len(res.ToolCalls) == 0 {
			report.Reason = "no_action"
			break
		}
		for _, tc := range res.ToolCalls {
			report.ToolCalls++
			outcome, err := g.Apply(tc.Name, stripReasoning(tc.Arguments))
			if err != nil {
				outcome = engine.Outcome{Narrative: fmt.Sprintf("Invalid action: %v", err)}
			}
			msgs = append(msgs, Message{Role: RoleTool, ToolCallID: tc.ID, Content: outcome.Narrative})
		}
	}
	report.WallTime = time.Since(start)
	return report, nil
}
