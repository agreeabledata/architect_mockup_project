# plan.md checklist

Status of the `engine.Game` port. Checked = done on `malcolm-engine-game`.

## Goal

- [x] Replace per-tick `Input` with the ratified `engine.Game` seam (`Observe` / `LegalActions` / `Apply`)
- [x] One `Apply("deliver", …)` resolves a whole delivery and debits its days

## Git

- [x] Leave `malcome-go-engine` untouched on origin
- [x] `git checkout -b malcolm-engine-game origin/davis-contracts-lifespan`
- [x] Port into monorepo `engine/` as new files
- [x] Do not rewrite `engine/engine.go` or `engine/balance.go`
- [x] Module path is `architect_mockup_project`
- [x] Open a PR into `main` (stacked on Davis’s #12: [#13](https://github.com/Flatiron-Apprentices/architect_mockup_project/pull/13))

## Target surface

- [x] `World` implements `Game` (`var _ Game = (*World)(nil)`)
- [x] `Observe()` returns `Narrative`, `Balance`, `LifespanDays`, `Done`
- [x] `LegalActions()` returns `Name`, `Description`, `Parameters`
- [x] `Apply(name, args)` returns `Outcome` (`Narrative`, `Balance`, `LifespanDays`, `Earned`, `Done`)
- [x] Invalid `Apply` returns an error and leaves the game unchanged
- [x] Economy consumed from `engine/balance.go` (not duplicated)

### Balance.go numbers used

- [x] `StartingLifespanDays` = 50
- [x] `StartingBalance` = 0
- [x] `CostRefreshBoard` = 1
- [x] `CostBattle` = 2
- [x] Deliveries 2 / 3 / 4 days → 60 / 70 / 100
- [x] `PayBattleWin` / `PayBattleLoss` = 120 / 0
- [x] `QuestsPerBoard` = 5, `MaxBattlesPerBoard` = 2
- [x] `AgentHP` / `OpponentHP` / `OpponentDamage` = 50 / 100 / 8
- [x] `AttackBudget` = 10 is prose only (not enforced)
- [x] `AttackPower` / `WinsBattle` from `balance.go` (win at 150 krones)

## Game rules

- [x] `engine.New(seed uint32) *World` — seed 0 remapped; same seed → same boards
- [x] Start: 50 days, 0 krone, one seeded board of 5 quests (≤2 battles)
- [x] Terminal: `Done` when `LifespanDays <= 0`; score is final balance
- [x] Delivery quest: 2–4 days, table payout, completes in one `Apply("deliver")`
- [x] Battle quest: 2 days, `WinsBattle(balance)`, fresh 50 vs 100 HP, no persistent HP
- [x] Deliveries are not platforming levels and do not contain a thug
- [x] `accept_quest` (`{"quest_id"}`) costs 0 days
- [x] `deliver` / `fight` / `refresh_board` as specified
- [x] No `rest`, no d-pad, no per-strike `PlayerStrike`
- [x] At most one active quest; no abandon; no refresh while active
- [x] After accept, legal actions are exactly `deliver` or `fight`
- [x] Unaffordable quests are not listed; `Apply` errors if forced
- [x] Unwinnable `fight` is still legal once accepted
- [x] `refresh_board` discards open quests, draws 5 new ones, costs 1 day, still ≤2 battles

### Narrative

- [x] Day index, remaining lifespan, krone balance, current attack power + formula
- [x] Opponent 100 HP, 8 dmg/round, agent 50 HP, “10 attacks” as flavor
- [x] Board lists id, type, days, payout, blurb
- [x] Active quest named, with the next action to complete it
- [x] `LegalActions` descriptions repeat cost and payout

## Files

- [x] `engine/world.go` — `World`, `New`, `Observe`, `LegalActions`, `Apply`
- [x] `engine/ledger.go` — days, krone, events (no persistent HP)
- [x] `engine/board.go` — quest type, `SeedBoard`, refresh
- [x] `engine/rng.go` — Mulberry32
- [x] `engine/bot.go` — decision baseline (best krone/day; skip battles unless `WinsBattle`)
- [x] `engine/world_test.go`, `board_test.go`, `bot_test.go` (plus `ledger_test.go`)
- [x] `scripts/run_bot/main.go` — print Observe → bot Apply until Done
- [x] Did **not** wire `scripts/run_agent` off the stub

## Tests

- [x] Compile-time `Game` implementation
- [x] Same seed → identical first board and identical refresh boards
- [x] Board constraints: 5 quests, ≤2 battles, delivery days {2,3,4} with 60/70/100, battles 2 days / 120
- [x] `Apply("deliver")` credits payout, debits days, `Earned` correct; second deliver errors and does not mutate
- [x] Fight at 0 krone loses (balance unchanged, 2 days gone)
- [x] Fight at 150 krone wins (+120, 2 days gone)
- [x] Invalid `Apply` errors and deep-equals the pre-Apply world
- [x] `LegalActions` hides unaffordable quests; includes unwinnable `fight` once accepted
- [x] Scripted bot on a fixed seed: ≥1 delivery, ≥1 battle after 150, `Done` at 0 days, `Balance == total Earned`
- [x] `go test ./...` green for engine, harness, and stub
- [x] No Ebiten in the test binary
- [x] No LLM code in the engine

## Implementation order

- [x] 1. Create `malcolm-engine-game` from `origin/davis-contracts-lifespan`; tests green before adding files
- [x] 2. `rng.go` + slim `ledger.go` + tests
- [x] 3. `board.go` generator + tests
- [x] 4. `world.go` implementing `Game`
- [x] 5. `world_test.go` (Apply invariants + 150-krone fight)
- [x] 6. Decision `bot.go` + full-run test
- [x] 7. `scripts/run_bot` text dump
- [x] 8a. `go test ./...`
- [x] 8b. Open PR ([#13](https://github.com/Flatiron-Apprentices/architect_mockup_project/pull/13))

## PR writeup (blocked on opening the PR)

- [x] Engine/framework: no game framework; Ebiten stays on `malcome-go-engine`
- [x] Kept: deterministic battle math, lifespan ledger, seeded board, Mulberry32
- [x] Dropped: physics, levels, per-frame `Input`, Ebiten client, fighter HUD
- [x] Constants aligned to `engine/balance.go`
- [x] `malcome-go-engine` is intentionally unmerged

## Out of scope (correctly not done)

- [x] Did not rebase or delete `malcome-go-engine`
- [x] Did not edit `engine/engine.go` or `engine/balance.go`
- [x] Did not switch `scripts/run_agent` from `stub.New()` to `engine.New()`
- [x] Did not add rest / shop / spending / HP persistence / visuals
- [x] Did not ship Davis’s official lazy baseline vs oracle solver
