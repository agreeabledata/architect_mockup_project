// Package engine defines the game surface the agent harness drives.
//
// DRAFT: this is the engine/harness seam. The definition is owned by Davis
// (reviewed in milestone 1); the harness and stub build against it now.
package engine

import "encoding/json"

// Action describes one legal action the agent may take this turn.
type Action struct {
	// Name is the action identifier the harness exposes to the model.
	Name string
	// Description is prose the model reads when choosing.
	Description string
	// Parameters is a JSON Schema object describing the arguments. Nil means no args.
	Parameters map[string]any
}

// State is everything the agent observes about the world this turn.
type State struct {
	// Narrative is prose describing the current situation.
	Narrative string
	// Balance is the agent's krone balance (the score in v1).
	Balance int
	// LifespanDays is how many days of life remain.
	LifespanDays int
	// Done reports whether the run has reached a terminal state.
	Done bool
}

// Outcome describes the result of applying one action.
type Outcome struct {
	// Narrative is prose describing what happened and what the agent sees now.
	Narrative string
	// Balance is the balance after the action.
	Balance int
	// LifespanDays is the remaining lifespan after the action.
	LifespanDays int
	// Earned is the change in balance from this action.
	Earned int
	// Done reports whether the action ended the run.
	Done bool
}

// Game is the deterministic engine surface the harness drives. The engine owns
// the lifespan ledger; the harness owns its own per-decision iteration cap.
type Game interface {
	// Observe returns the current state the agent sees.
	Observe() State
	// LegalActions lists the actions available right now.
	LegalActions() []Action
	// Apply performs an action. Invalid names or arguments return a non-nil
	// error and leave the game unchanged.
	Apply(name string, args json.RawMessage) (Outcome, error)
}
