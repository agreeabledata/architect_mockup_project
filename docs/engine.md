# Engine rules

What the harness drives: `engine.New(seed)` returns a `World` that satisfies `engine.Game` (`Observe` / `LegalActions` / `Apply`). One LLM call is one `Apply`. Every number below is a constant in `engine/balance.go`; change it there, nowhere else.

Seed `0` becomes `1`. Same seed gives identical boards, refreshes, and outcomes.

## The six actions

| Action | Args | Cost | When legal |
|---|---|---|---|
| `accept_quest` | `{"quest_id": "..."}` | 0 days | no active quest; quest is open and affordable |
| `deliver` | none | 2, 3, or 4 days | active quest is a delivery |
| `fight` | none | 2 days | active quest is a battle |
| `scry` | `{"node": 1}` | 0 days, 1 mana | active arcane quest; node undisrupted and unrevealed |
| `cast_spell` | `{"spell": "fire"}` | 0 days, 2 mana | active arcane quest; at least 2 mana |
| `refresh_board` | none | 1 day | no active quest; at least 1 day left |

Rules people quiz:

- You may hold **one** quest. No abandon. Cannot refresh while holding one.
- After accept, the only legal actions are `deliver` / `fight`, or the arcane quest's `scry` / `cast_spell`.
- Unaffordable quests are hidden. Forcing them with `Apply` **errors and does not mutate**.
- Fighting while too poor **is legal** — that is the trap the eval scores.
- There is no `rest`, no d-pad, no per-punch combat.

## The three quest types

**Delivery** — courier packet. Distance is the cost (2 / 3 / 4 days). Pays 60 / 70 / 100. Short run is best per day (30/day vs 23 vs 25), so board order is a real decision. `Apply("deliver")` completes the whole job. No walking, no jumping.

**Battle** — its own quest type, not a thug on a courier route. Costs 2 days. Win pays 120, loss pays 0 (the wasted days are the penalty). Stamina does not persist: every fight is a fresh 50 stamina vs 100 HP.

**Arcane Dispersal** — a spell-sequencing puzzle under a mana budget. 3 nodes, each keyed to a different resonance (fire, water, earth) and **every resonance appears exactly once**, but which node holds which is hidden. Nodes must be disrupted in order. Each spell costs 2 mana; a wrong spell backlashes for 2 more. `scry` costs 1 mana and reveals one node's resonance. You start with 8 mana. Succeed and you are paid 150 krones; run dry and the ritual collapses, paying nothing. Either way it costs 3 days. The budget makes scrying every node (3 + 3 = 9 mana) unaffordable, so the agent must scry all but one node and **deduce the last by elimination** (2 + 6 = 8 mana, exact).

A board is **5 quests, at most 2 battles and at most 1 arcane ritual**. Refresh discards leftovers and draws a new five.

## The eval

The model is supposed to extract arithmetic from the fiction.

- Start: **50 days, 0 krones**
- Attack power = `max(1, floor(krones / 10))` — krones are never spent
- You: 50 stamina. Foe: 100 HP, 8 damage/round. You strike first
- **Fighting is a guaranteed loss below 150 krones, a guaranteed win at 150+**
- “10-attack budget” is **flavor only**. At these numbers you die or win in 7 rounds, so the engine does not track it
- Shape of a good run: fund on deliveries until 150, then farm battles (60/day vs 30/day for a short delivery). Supply is the limiter (≤2 battles per board)
- Arcane pays 50/day (150 over 3 days) but needs no krone investment; the skill is budgeting mana and deducing the last node. Supply is ≤1 ritual per board

`WinsBattle(balance)` and `AttackPower` live in `balance.go`. `world.go` calls `WinsBattle` for the outcome and only computes the round count for the story text. All arcane numbers (`CostArcane`, `PayArcane`, `ArcaneNodes`, `ArcaneMana`, `ManaScry`, `ManaSpell`, `ManaBacklash`, `MaxArcanePerBoard`) live in `balance.go` too; `TestArcaneManaForcesDeduction` pins the budget so deduction stays required.

