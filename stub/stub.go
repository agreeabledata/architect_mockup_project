// Package stub provides a hardcoded fake engine with two fake quests so the
// harness can develop before the real engine lands. Deterministic, no LLM code.
package stub

import (
	"encoding/json"
	"fmt"
	"strings"

	"architect_mockup_project/engine"
)

const (
	startHealth    = 50
	agentStamina   = 50
	opponentHP     = 100
	opponentDmg    = 8
	pickupCost     = 1
	deliveryCost   = 3
	deliveryPayout = 12
	fightCost      = 2
	fightPayout    = 30
	restCost       = 1
)

// Stub is a deterministic two-quest world: a courier delivery and one duel.
// The duel is a word problem — winning depends on extracting the arithmetic
// (attack power = max(1, floor(balance/10))) from the fiction.
type Stub struct {
	day           int
	health        int
	balance       int
	deliveryTaken bool
	deliveryDone  bool
	fought        bool
	over          bool
}

// New returns a fresh stub run.
func New() *Stub {
	return &Stub{health: startHealth}
}

// Observe implements engine.Game.
func (s *Stub) Observe() engine.State {
	return engine.State{
		Narrative:  s.narrative(),
		Balance:    s.balance,
		HealthDays: s.health,
		Done:       s.over,
	}
}

// LegalActions implements engine.Game.
func (s *Stub) LegalActions() []engine.Action {
	var acts []engine.Action
	if !s.deliveryDone {
		if s.deliveryTaken {
			acts = append(acts, engine.Action{
				Name:        "deliver_package",
				Description: fmt.Sprintf("Deliver the package across town. Costs %d days, pays %d krones.", deliveryCost, deliveryPayout),
			})
		} else {
			acts = append(acts, engine.Action{
				Name:        "take_delivery",
				Description: fmt.Sprintf("Pick up the courier package. Costs %d day.", pickupCost),
			})
		}
	}
	if !s.fought {
		acts = append(acts, engine.Action{
			Name:        "fight",
			Description: "Challenge the goblin warband. Costs 2 days; winning depends on your attack power.",
		})
	}
	acts = append(acts, engine.Action{
		Name:        "rest",
		Description: fmt.Sprintf("Do nothing and gather your strength. Costs %d day.", restCost),
	})
	return acts
}

// Apply implements engine.Game.
func (s *Stub) Apply(name string, args json.RawMessage) (engine.Outcome, error) {
	before := s.balance
	switch name {
	case "take_delivery":
		if s.deliveryTaken || s.deliveryDone {
			return engine.Outcome{}, fmt.Errorf("no delivery available to take")
		}
		s.deliveryTaken = true
		s.spend(pickupCost)
		return s.outcome(before, fmt.Sprintf("You pick up the package. %d days pass.", pickupCost)), nil

	case "deliver_package":
		if !s.deliveryTaken {
			return engine.Outcome{}, fmt.Errorf("you have no package to deliver")
		}
		s.deliveryDone = true
		s.balance += deliveryPayout
		s.spend(deliveryCost)
		return s.outcome(before, fmt.Sprintf("You deliver the package and are paid %d krones.", deliveryPayout)), nil

	case "fight":
		if s.fought {
			return engine.Outcome{}, fmt.Errorf("the warband is gone")
		}
		s.fought = true
		dmg := s.attack()
		roundsToWin := ceilDiv(opponentHP, dmg)
		roundsToDie := ceilDiv(agentStamina, opponentDmg)
		var msg string
		if roundsToWin <= roundsToDie {
			s.balance += fightPayout
			msg = fmt.Sprintf("Your attack power %d defeats the warband in %d rounds. You earn %d krones.", dmg, roundsToWin, fightPayout)
		} else {
			s.balance -= engine.DuelLossPenalty
			msg = fmt.Sprintf("Your attack power %d is too weak; the warband overpowers you. You lose %d krones.", dmg, engine.DuelLossPenalty)
		}
		s.spend(fightCost)
		return s.outcome(before, msg), nil

	case "rest":
		s.spend(restCost)
		return s.outcome(before, "You rest and take stock of the road ahead."), nil
	}
	return engine.Outcome{}, fmt.Errorf("unknown action %q", name)
}

func (s *Stub) attack() int {
	if d := s.balance / 10; d > 0 {
		return d
	}
	return 1
}

func (s *Stub) spend(days int) {
	s.day += days
	s.health -= days
	if s.health <= 0 {
		s.over = true
	}
}

func (s *Stub) outcome(before int, msg string) engine.Outcome {
	return engine.Outcome{
		Narrative:  msg,
		Balance:    s.balance,
		HealthDays: s.health,
		Earned:     s.balance - before,
		Done:       s.over,
	}
}

func (s *Stub) narrative() string {
	var sb []string
	sb = append(sb, fmt.Sprintf("Day %d. You have %d health remaining (every day spent costs 1 health) and %d krones.", s.day, s.health, s.balance))
	if !s.deliveryDone {
		if s.deliveryTaken {
			sb = append(sb, fmt.Sprintf("You carry the courier's package, worth %d krones if delivered.", deliveryPayout))
		} else {
			sb = append(sb, fmt.Sprintf("A courier job waits at the guild: pick up a package and deliver it across town for %d krones.", deliveryPayout))
		}
	}
	if !s.fought {
		sb = append(sb, fmt.Sprintf("A goblin warband (%d HP) blocks the bridge. Each round it deals %d damage to your %d stamina. Your attack power is max(1, floor(krones/10)). Winning pays %d krones, losing costs %d.", opponentHP, opponentDmg, agentStamina, fightPayout, engine.DuelLossPenalty))
	}
	return strings.Join(sb, " ")
}

func ceilDiv(a, b int) int {
	return (a + b - 1) / b
}
