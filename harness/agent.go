package harness

import (
	"context"
	"fmt"
	"strings"
	"time"

	"architect_mockup_project/engine"
)

// promptVersion hashes every run transcript to the prompt it used.
const promptVersion = "v2"

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
		tools = append(tools, Tool{Name: a.Name, Description: a.Description, Parameters: a.Parameters})
	}
	return tools
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
	Reason     string // terminal | iteration_cap | decision_cap | no_action
}

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
	retryCap := a.Config.DecisionRetryCap
	if retryCap < 1 {
		retryCap = 3
	}
	var report Report
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
		for attempt := 0; attempt < retryCap; attempt++ {
			if report.Iterations >= a.Config.IterationCap {
				report.Reason = "iteration_cap"
				break decisions
			}
			res, err := a.Provider.Chat(ctx, lastTurns(msgs, contextTurns), actionsToTools(actions), CallOptions{
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
				break decisions
			}
			for _, tc := range res.ToolCalls {
				report.ToolCalls++
				outcome, err := g.Apply(tc.Name, tc.Arguments)
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
		report.Reason = "decision_cap"
		break
	}
	report.WallTime = time.Since(start)
	return report, nil
}
