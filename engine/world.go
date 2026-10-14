package engine

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Action names the harness exposes as tools.
const (
	ActionAccept  = "accept_quest"
	ActionDeliver = "deliver"
	ActionFight   = "fight"
	ActionScry    = "scry"
	ActionCast    = "cast_spell"
	ActionRefresh = "refresh_board"
)

// World is the v1 decision-level engine: a seeded quest board, a health
// ledger, and a deterministic duel. It implements Game. There is no physics
// and no per-frame input — one Apply is one whole decision.
type World struct {
	seed         uint32
	refreshCount int
	ledger       Ledger
	board        []Quest
	activeID     string
	done         bool
}

var _ Game = (*World)(nil)

// New returns a fresh run. Seed 0 is remapped to 1. Same seed → same boards,
// refreshes, and outcomes.
func New(seed uint32) *World {
	if seed == 0 {
		seed = 1
	}
	w := &World{
		seed:   seed,
		ledger: newLedger(),
	}
	w.board = SeedBoard(Mulberry32(boardSeed(seed, 0)), seed, 0)
	return w
}

// Board returns a copy of the current quest board, active quest included.
// Read-only and additive: it is not part of the Game seam, and callers cannot
// mutate the world through the returned slice. Used by the #23 visualizer to
// replay what the agent saw each turn.
func (w *World) Board() []Quest {
	return append([]Quest(nil), w.board...)
}

// Observe implements Game.
func (w *World) Observe() State {
	return State{
		Narrative:  w.narrative(),
		Balance:    w.ledger.Krone,
		HealthDays: w.ledger.Days,
		Done:       w.isDone(),
	}
}

// LegalActions implements Game.
func (w *World) LegalActions() []Action {
	if w.isDone() {
		return nil
	}
	if w.activeID != "" {
		q := w.questByID(w.activeID)
		if q == nil {
			return nil
		}
		if q.Kind == QuestBattle {
			return []Action{{
				Name: ActionFight,
				Description: fmt.Sprintf(
					"Fight the posted foe. Costs %d days. Winning pays %d krones; losing pays %d. Outcome is determined by your attack power max(1, floor(krones/10)) against a %d HP foe dealing %d per round to your %d stamina.",
					CostBattle, PayBattleWin, PayBattleLoss, OpponentHP, OpponentDamage, AgentStamina,
				),
			}}
		}
		if q.Kind == QuestArcane {
			return w.arcaneActions(q)
		}
		return []Action{{
			Name: ActionDeliver,
			Description: fmt.Sprintf(
				"Deliver %q. Resolves the whole run: costs %d days, pays %d krones. No travel to act out.",
				q.Title, q.Days, q.Payout,
			),
		}}
	}

	var acts []Action
	if ids := w.acceptableIDs(); len(ids) > 0 {
		acts = append(acts, Action{
			Name:        ActionAccept,
			Description: "Accept one open quest from the board. Costs no days; completing it later spends its listed days. You may hold only one quest at a time.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"quest_id": map[string]any{
						"type":        "string",
						"enum":        ids,
						"description": "ID of an open quest whose cost you can still afford",
					},
				},
				"required": []any{"quest_id"},
			},
		})
	}
	if w.ledger.Days >= CostRefreshBoard {
		acts = append(acts, Action{
			Name: ActionRefresh,
			Description: fmt.Sprintf(
				"Discard the remaining open jobs and draw a fresh board of %d quests (at most %d battles). Costs %d day.",
				QuestsPerBoard, MaxBattlesPerBoard, CostRefreshBoard,
			),
		})
	}
	return acts
}

// Apply implements Game. Invalid names or arguments return an error and leave
// the world unchanged.
func (w *World) Apply(name string, args json.RawMessage) (Outcome, error) {
	if w.isDone() {
		return Outcome{}, fmt.Errorf("run is over")
	}
	switch name {
	case ActionAccept:
		return w.applyAccept(args)
	case ActionDeliver:
		return w.applyDeliver()
	case ActionFight:
		return w.applyFight()
	case ActionScry:
		return w.applyScry(args)
	case ActionCast:
		return w.applyCast(args)
	case ActionRefresh:
		return w.applyRefresh()
	default:
		return Outcome{}, fmt.Errorf("unknown action %q", name)
	}
}

func (w *World) applyAccept(raw json.RawMessage) (Outcome, error) {
	if w.activeID != "" {
		return Outcome{}, fmt.Errorf("already on a quest")
	}
	id, err := parseQuestID(raw)
	if err != nil {
		return Outcome{}, err
	}
	q := w.questByID(id)
	if q == nil || q.Status != QuestOpen {
		return Outcome{}, fmt.Errorf("no open quest %q", id)
	}
	if q.Days > w.ledger.Days {
		return Outcome{}, fmt.Errorf("not enough days to take %q", id)
	}
	q.Status = QuestActive
	w.activeID = q.ID
	msg := fmt.Sprintf("You accept %q (%s, %d days, %d krones). Complete it next.", q.Title, q.Kind, q.Days, q.Payout)
	return w.outcome(0, msg), nil
}

func (w *World) applyDeliver() (Outcome, error) {
	q := w.questByID(w.activeID)
	if q == nil || q.Status != QuestActive || q.Kind != QuestDelivery {
		return Outcome{}, fmt.Errorf("no delivery in hand")
	}
	if q.Days > w.ledger.Days {
		return Outcome{}, fmt.Errorf("not enough days to deliver")
	}
	pay := q.Payout
	title := q.Title
	days := q.Days
	w.ledger = w.ledger.creditKrone(pay, "delivery", "Delivered "+title+".")
	w.ledger = w.ledger.debitDays(days, "travel", fmt.Sprintf("Road to complete %q.", title))
	q.Status = QuestComplete
	w.activeID = ""
	w.markDoneIfSpent()
	msg := fmt.Sprintf("You deliver %q and are paid %d krones. %d days pass.", title, pay, days)
	return w.outcome(pay, msg), nil
}

func (w *World) applyFight() (Outcome, error) {
	q := w.questByID(w.activeID)
	if q == nil || q.Status != QuestActive || q.Kind != QuestBattle {
		return Outcome{}, fmt.Errorf("no battle posted")
	}
	if CostBattle > w.ledger.Days {
		return Outcome{}, fmt.Errorf("not enough days to fight")
	}
	title := q.Title
	won, story := duelStory(w.ledger.Krone)
	earned := PayBattleLoss
	kind := "battle_loss"
	note := "Defeated: " + title + "."
	if won {
		earned = PayBattleWin
		kind = "battle_win"
		note = "Won: " + title + "."
	}
	w.ledger = w.ledger.creditKrone(earned, kind, note)
	w.ledger = w.ledger.debitDays(CostBattle, "battle", fmt.Sprintf("Fought %q.", title))
	q.Status = QuestComplete
	w.activeID = ""
	w.markDoneIfSpent()
	return w.outcome(earned, story), nil
}

// arcaneActions lists the ritual's legal moves: scry an undisrupted, unrevealed
// node, and cast any of the three resonance spells at the current node.
func (w *World) arcaneActions(q *Quest) []Action {
	var acts []Action
	if nodes := scryableNodes(q); len(nodes) > 0 && q.Mana >= ManaScry {
		acts = append(acts, Action{
			Name: ActionScry,
			Description: fmt.Sprintf(
				"Scry one undisrupted node for %d mana, revealing its hidden resonance. %d mana remains.",
				ManaScry, q.Mana,
			),
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"node": map[string]any{
						"type":        "integer",
						"enum":        nodes,
						"description": "1-based node number to scry",
					},
				},
				"required": []any{"node"},
			},
		})
	}
	if q.Mana >= ManaSpell {
		acts = append(acts, Action{
			Name: ActionCast,
			Description: fmt.Sprintf(
				"Cast a resonance spell at node %d. Costs %d mana; a wrong resonance backlashes for %d more. Disrupt all %d nodes to finish.",
				q.NextNode+1, ManaSpell, ManaBacklash, ArcaneNodes,
			),
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"spell": map[string]any{
						"type":        "string",
						"enum":        []string{string(ResonanceFire), string(ResonanceWater), string(ResonanceEarth)},
						"description": "resonance to cast at the current node",
					},
				},
				"required": []any{"spell"},
			},
		})
	}
	return acts
}

// scryableNodes returns the 1-based numbers of undisrupted nodes not yet
// revealed.
func scryableNodes(q *Quest) []int {
	var nodes []int
	for i := q.NextNode; i < ArcaneNodes; i++ {
		if !q.Revealed[i] {
			nodes = append(nodes, i+1)
		}
	}
	return nodes
}

// arcaneStuck reports whether no ritual action remains affordable: too little
// mana to cast, and either too little to scry or nothing left to scry. A stuck
// ritual collapses.
func (w *World) arcaneStuck(q *Quest) bool {
	if q.Mana >= ManaSpell {
		return false
	}
	if q.Mana >= ManaScry && len(scryableNodes(q)) > 0 {
		return false
	}
	return true
}

func (w *World) applyScry(raw json.RawMessage) (Outcome, error) {
	q := w.questByID(w.activeID)
	if q == nil || q.Status != QuestActive || q.Kind != QuestArcane {
		return Outcome{}, fmt.Errorf("no ritual in progress")
	}
	node, err := parseScryNode(raw)
	if err != nil {
		return Outcome{}, err
	}
	i := node - 1
	if i < q.NextNode || i >= ArcaneNodes {
		return Outcome{}, fmt.Errorf("node %d is not an undisrupted node", node)
	}
	if q.Revealed[i] {
		return Outcome{}, fmt.Errorf("node %d is already scried", node)
	}
	if q.Mana < ManaScry {
		return Outcome{}, fmt.Errorf("not enough mana to scry")
	}
	q.Mana -= ManaScry
	q.Revealed[i] = true
	msg := fmt.Sprintf("You scry node %d: it resonates with %s. %d mana remains.", node, q.Nodes[i], q.Mana)
	if w.arcaneStuck(q) {
		return w.arcaneCollapse(q, msg+" Too little mana remains to act; the ritual collapses."), nil
	}
	return w.outcome(0, msg), nil
}

func (w *World) applyCast(raw json.RawMessage) (Outcome, error) {
	q := w.questByID(w.activeID)
	if q == nil || q.Status != QuestActive || q.Kind != QuestArcane {
		return Outcome{}, fmt.Errorf("no ritual in progress")
	}
	spell, err := parseCastSpell(raw)
	if err != nil {
		return Outcome{}, err
	}
	if q.Mana < ManaSpell {
		return Outcome{}, fmt.Errorf("not enough mana to cast")
	}
	node := q.NextNode + 1
	if Resonance(spell) == q.Nodes[q.NextNode] {
		q.Mana -= ManaSpell
		q.NextNode++
		if q.NextNode == ArcaneNodes {
			msg := fmt.Sprintf("Your %s spell disrupts node %d. All %d nodes fall and the ritual is undone.", spell, node, ArcaneNodes)
			return w.arcaneSuccess(q, msg), nil
		}
		msg := fmt.Sprintf("Your %s spell disrupts node %d. %d mana remains; node %d is next.", spell, node, q.Mana, q.NextNode+1)
		if w.arcaneStuck(q) {
			return w.arcaneCollapse(q, msg+" Too little mana remains to act; the ritual collapses."), nil
		}
		return w.outcome(0, msg), nil
	}
	q.Mana -= ManaSpell + ManaBacklash
	msg := fmt.Sprintf("The %s spell is wrong; node %d backlashes, draining %d mana.", spell, node, ManaSpell+ManaBacklash)
	if w.arcaneStuck(q) {
		return w.arcaneCollapse(q, msg+" The ritual collapses."), nil
	}
	return w.outcome(0, fmt.Sprintf("%s %d mana remains.", msg, q.Mana)), nil
}

func (w *World) arcaneSuccess(q *Quest, msg string) Outcome {
	q.Status = QuestComplete
	w.activeID = ""
	w.ledger = w.ledger.creditKrone(PayArcane, "arcane", "Dispersed "+q.Title+".")
	w.ledger = w.ledger.debitDays(CostArcane, "ritual", fmt.Sprintf("Completed %q.", q.Title))
	w.markDoneIfSpent()
	return w.outcome(PayArcane, fmt.Sprintf("%s You are paid %d krones; %d days pass.", msg, PayArcane, CostArcane))
}

func (w *World) arcaneCollapse(q *Quest, msg string) Outcome {
	q.Status = QuestComplete
	w.activeID = ""
	w.ledger = w.ledger.debitDays(CostArcane, "ritual", fmt.Sprintf("Ritual collapsed: %q.", q.Title))
	w.markDoneIfSpent()
	return w.outcome(0, fmt.Sprintf("%s The purse is unchanged; %d days pass.", msg, CostArcane))
}

func (w *World) applyRefresh() (Outcome, error) {
	if w.activeID != "" {
		return Outcome{}, fmt.Errorf("cannot refresh while a quest is active")
	}
	if w.ledger.Days < CostRefreshBoard {
		return Outcome{}, fmt.Errorf("not enough days to refresh the board")
	}
	w.ledger = w.ledger.debitDays(CostRefreshBoard, "refresh", "Drew a fresh quest board.")
	w.refreshCount++
	w.board = SeedBoard(Mulberry32(boardSeed(w.seed, w.refreshCount)), w.seed, w.refreshCount)
	w.markDoneIfSpent()
	msg := fmt.Sprintf("A day spent at the board. %d new jobs are posted.", len(w.board))
	return w.outcome(0, msg), nil
}

func (w *World) isDone() bool {
	return w.done || w.ledger.Days <= 0
}

func (w *World) markDoneIfSpent() {
	if w.ledger.Days <= 0 {
		w.done = true
		w.ledger.Days = 0
	}
}

func (w *World) outcome(earned int, msg string) Outcome {
	return Outcome{
		Narrative:  msg,
		Balance:    w.ledger.Krone,
		HealthDays: w.ledger.Days,
		Earned:     earned,
		Done:       w.isDone(),
	}
}

func (w *World) questByID(id string) *Quest {
	if id == "" {
		return nil
	}
	for i := range w.board {
		if w.board[i].ID == id {
			return &w.board[i]
		}
	}
	return nil
}

func (w *World) acceptableIDs() []string {
	var ids []string
	for _, q := range w.board {
		if q.Status == QuestOpen && q.Days <= w.ledger.Days {
			ids = append(ids, q.ID)
		}
	}
	return ids
}

func parseQuestID(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("accept_quest requires {\"quest_id\": \"...\"}")
	}
	var args struct {
		QuestID string `json:"quest_id"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", fmt.Errorf("accept_quest args: %w", err)
	}
	if args.QuestID == "" {
		return "", fmt.Errorf("accept_quest requires quest_id")
	}
	return args.QuestID, nil
}

func parseScryNode(raw json.RawMessage) (int, error) {
	if len(raw) == 0 {
		return 0, fmt.Errorf("scry requires {\"node\": 1}")
	}
	var args struct {
		Node int `json:"node"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return 0, fmt.Errorf("scry args: %w", err)
	}
	return args.Node, nil
}

func parseCastSpell(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("cast_spell requires {\"spell\": \"fire\"}")
	}
	var args struct {
		Spell string `json:"spell"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", fmt.Errorf("cast_spell args: %w", err)
	}
	switch Resonance(args.Spell) {
	case ResonanceFire, ResonanceWater, ResonanceEarth:
		return args.Spell, nil
	}
	return "", fmt.Errorf("unknown resonance %q", args.Spell)
}

func (w *World) narrative() string {
	var b strings.Builder
	elapsed := StartingHealthDays - w.ledger.Days
	if elapsed < 0 {
		elapsed = 0
	}
	power := AttackPower(w.ledger.Krone)
	fmt.Fprintf(&b, "Day %d. You have %d health remaining (every day spent costs 1 health) and %d krones. Your attack power is max(1, floor(krones/10)), currently %d.\n\n",
		elapsed, w.ledger.Days, w.ledger.Krone, power)
	fmt.Fprintf(&b, "A posted foe has %d HP and deals %d damage each round to your %d stamina. The tales speak of a %d-attack budget; your power comes from the krones you hold, which are never spent. Combat pays %d krones on a win and %d on a loss, and costs %d days.\n\n",
		OpponentHP, OpponentDamage, AgentStamina, AttackBudget, PayBattleWin, PayBattleLoss, CostBattle)

	if w.isDone() {
		b.WriteString("The ledger is closed. No days remain.\n")
		return b.String()
	}

	b.WriteString("The quest board:\n")
	for _, q := range w.board {
		fmt.Fprintf(&b, "- %s [%s] %s (%s, %d days, %d krones): %s\n",
			q.ID, q.Status, q.Title, q.Kind, q.Days, q.Payout, q.Blurb)
	}
	b.WriteByte('\n')
	if q := w.questByID(w.activeID); q != nil {
		if q.Kind == QuestArcane {
			fmt.Fprintf(&b, "You hold %q. Node %d of %d is next; %d mana remains.\n",
				q.Title, q.NextNode+1, ArcaneNodes, q.Mana)
			for i := q.NextNode; i < ArcaneNodes; i++ {
				if q.Revealed[i] {
					fmt.Fprintf(&b, "- node %d resonates with %s.\n", i+1, q.Nodes[i])
				}
			}
		} else {
			verb := ActionDeliver
			if q.Kind == QuestBattle {
				verb = ActionFight
			}
			fmt.Fprintf(&b, "You hold %q. The next action is %s.\n", q.Title, verb)
		}
	} else {
		b.WriteString("No job is in your hands. Accept a quest, or spend a day to refresh the board.\n")
	}
	return b.String()
}

// duelStory narrates a fight. The outcome is WinsBattle from balance.go -- the
// engine never re-derives it; only the round count for the prose is computed here.
func duelStory(balance int) (won bool, story string) {
	power := AttackPower(balance)
	if WinsBattle(balance) {
		rounds := (OpponentHP + power - 1) / power
		return true, fmt.Sprintf("Your attack power %d fells the foe in %d rounds. You earn %d krones. %d days pass.", power, rounds, PayBattleWin, CostBattle)
	}
	rounds := (AgentStamina + OpponentDamage - 1) / OpponentDamage
	return false, fmt.Sprintf("Your attack power %d is too weak; after %d rounds you fall. The purse is unchanged. %d days pass.", power, rounds, CostBattle)
}
