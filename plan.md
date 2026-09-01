# Port KRONE to `engine.Game` (decision-level, not per-frame)

The review is right: the ledger, battle math, seeded board, and tests are the v1 engine. The problem is the interface. `StepSim(s, Input{MoveX, JumpPressed, ...})` is a 60 Hz gamepad, which a model cannot drive and which scores platforming instead of quest decisions.

Replace the per-tick controller with the ratified `engine.Game` seam already on the monorepo (`Observe` / `LegalActions` / `Apply`). One `Apply("deliver", …)` resolves a whole delivery and debits its days — no traversal, no jumping.

## What is already true

Current branch `malcome-go-engine` is an Ebiten platformer. The good pieces:

- `internal/engine/battle.go` — `AgentStartHP 50`, `OpponentStartHP 100`, `OpponentDamage 8`, `AttackPower = max(1, coins/10)`, no spending. Matches the spec.
- `internal/engine` has **zero** Ebiten imports (client is `cmd/krone` only).
- `ledger.go`, `SeedBoard`, `rng.go` (Mulberry32) are the right shape.
- Unit tests sit next to the logic.

What must go for the harness engine: `physics.go`, `levels.go`, `sim.go` tick loop, `bot.go` (d-pad autopilot), `cmd/krone`, `fighter/`, `battle/`.

Keep `malcome-go-engine` as-is. It is the human-playable rooftop demo. Do not try to merge it.

## Git: do not rebase this branch onto main

`malcome-go-engine` shares **no commit** with `origin/main` (unrelated histories: one commit `91b7053` vs the monorepo starting at `7bdf206`). A rebase cannot produce a mergeable PR.

```
origin/main                      cfe06bc  init (engine.Game DRAFT, no balance.go)
origin/davis-contracts-lifespan  867e107  ratify contract + engine/balance.go
origin/malcome-go-engine         91b7053  Ebiten platformer (this workspace)
```

**Branch from Davis, not from this working tree.**

1. Leave `malcome-go-engine` untouched on origin.
2. `git checkout -b malcolm-engine-game origin/davis-contracts-lifespan`
3. Port logic into the monorepo `engine/` package as new files. Do not rewrite `engine/engine.go` or `engine/balance.go` — Davis owns those.
4. Open a PR into `main` (stacked on Davis’s contracts PR if it is not merged yet).

If `davis-contracts-lifespan` has already landed on `main` by the time we start, branch from `origin/main` instead. We need `engine/balance.go`; it is not on `origin/main` yet.

Module path on the target is `architect_mockup_project`, not `github.com/mfeinLearn/krone`.

## Target surface (already ratified)

```go
// engine/engine.go — do not edit
type Game interface {
    Observe() State
    LegalActions() []Action
    Apply(name string, args json.RawMessage) (Outcome, error)
}
```

`State`: `Narrative`, `Balance`, `LifespanDays`, `Done`.
`Outcome`: those plus `Earned`.
`Action`: `Name`, `Description`, `Parameters` (JSON Schema, or nil).

Invalid `Apply` returns an error and **must leave the game unchanged**.

Economy lives only in `engine/balance.go`. Consume these; do not duplicate:

| Constant | Value | Meaning |
|---|---|---|
| `StartingLifespanDays` | 50 | was `StartDays = 24` |
| `StartingBalance` | 0 | |
| `CostRefreshBoard` | 1 | |
| `CostBattle` | 2 | |
| `CostDeliveryPerLeg` | 1 | days = distance (2–4) |
| `PayDeliveryShort/Medium/Long` | 60 / 70 / 100 | 2 / 3 / 4 day runs |
| `PayBattleWin` / `PayBattleLoss` | 120 / 0 | loss still costs 2 days |
| `QuestsPerBoard` | 5 | |
| `MaxBattlesPerBoard` | 2 | |
| `AgentHP` / `OpponentHP` / `OpponentDamage` | 50 / 100 / 8 | |
| `AttackBudget` | 10 | **prose distractor only** — do not enforce |
| `AttackPower(balance)` | `max(1, balance/10)` | already in `balance.go` |
| `WinsBattle(balance)` | win iff balance ≥ 150 | already in `balance.go` |

Combat unlocks at 150 krones. Below that the duel is a guaranteed loss. That is the eval.

## Game rules

`World` (or `Krone`) is the `engine.Game` implementation. Constructor: `engine.New(seed uint32) *World`. Same seed → identical boards, refreshes, and outcomes.

### Lifecycle

- Start: 50 days, 0 krone, one seeded board of 5 quests (≤2 battles).
- Terminal: `LifespanDays <= 0` after an action (`Done == true`). No other win/loss flag — the score is final balance.

### Quest types

Two types only (issue #7):

- **delivery** — cost 2, 3, or 4 days; pays 60 / 70 / 100. `Apply("deliver")` completes it in one call.
- **battle** — cost 2 days; pays 120 on win, 0 on loss. Outcome is `WinsBattle(balance)` (fresh 50 vs 100 HP every fight; HP does not persist). Attack budget is mentioned in narrative, never checked.

Deliveries are **not** platforming levels and do **not** contain a thug. Combat is its own quest type, not an encounter on a courier route. That is the shape change vs the current `roofduel` level.

### Actions

| Name | Args | Cost | When legal |
|---|---|---|---|
| `accept_quest` | `{"quest_id": string}` | 0 days | no active quest; id is `open` on the current board; remaining days ≥ that quest’s cost |
| `deliver` | none | quest distance (2–4) | active quest is a delivery |
| `fight` | none | 2 | active quest is a battle |
| `refresh_board` | none | 1 | no active quest; remaining days ≥ 1 |

No `rest`. No d-pad. No per-strike `PlayerStrike`.

`accept_quest` is selection only (commitment). It does not debit days. Execute (`deliver` / `fight`) is what spends life and pays out. You may hold **at most one** active quest; you cannot refresh or accept another until it is resolved. No abandon.

After `accept_quest`, `LegalActions` is exactly `deliver` or `fight`. After resolve, the quest is complete, the slot is empty, and the agent is back on the board.

A quest whose cost exceeds remaining days is not listed (and `Apply` errors if forced). Fighting while unwinnable **is** legal — that is the trap the eval is scoring.

### `refresh_board`

Discard remaining `open` quests. Draw a new 5-quest board from `Mulberry32(seed + refreshCount)` (or equivalent: one RNG stream from the original seed, advanced in place — either is fine as long as it is deterministic). Still ≤2 battles. Cost 1 day. If that hits 0 days, the run ends.

### Narrative (this is the word problem)

`Observe().Narrative` must give the model everything needed to do the arithmetic, dressed as fiction:

- Day index, remaining lifespan, krone balance, current attack power, and the formula `max(1, floor(krones/10))`.
- Opponent: 100 HP, 8 damage/round, agent 50 HP, “10 attacks” as flavor.
- The board: each quest’s id, type, days, payout, and a short blurb. Battle quests should mention the duel numbers again.
- If a quest is active, say so and that the next action is to complete it.

`LegalActions` descriptions should repeat cost and payout so the harness tools are self-contained.

## What to port vs rewrite

| Keep (port into `engine/`) | Drop (stay on `malcome-go-engine`) |
|---|---|
| `rng.go` Mulberry32 | `physics.go` |
| Ledger idea (days, krone, event log) | `levels.go` (platforms, coins, zones) |
| Battle constants — **use Davis’s**, delete ours | `sim.go` `StepSim` / `Input` |
| `WinsBattle` / `AttackPower` already on Davis | `bot.go` coin-chasing jumper |
| Seeded board **idea** — rewrite generator | `cmd/krone` Ebiten client |
| Deterministic tests | `fighter/`, `battle/` Showdown HUD |

Ledger rewrite: drop persistent HP, `RestHP`, fall penalties, coin pickups. Fields: `Days`, `Krone`, `Events`. `CreateLedger` uses `StartingLifespanDays` / `StartingBalance`.

Battle rewrite: do not keep `StepBattle` budget-loss. Call `WinsBattle(balance)` (and optionally simulate the round loop **only** to write narrative). Never fail on `AttackBudget`. Never spend krone.

Board rewrite: `SeedBoard` today iterates `LevelOrder` and jitters `lvl.RewardKrone`. Replace with a generator that emits 5 quests from the payout table, with 0–2 battles.

## File plan (all under `engine/`, package `engine`)

Do not touch `engine.go` or `balance.go`.

- `world.go` — `World` struct, `New(seed)`, `Observe`, `LegalActions`, `Apply`. Compile-time check: `var _ Game = (*World)(nil)`.
- `ledger.go` — debit days, credit krone, events. Port and slim.
- `board.go` — quest type, `SeedBoard(rng)`, refresh. Port the idea, new generator.
- `rng.go` — copy Mulberry32.
- `bot.go` — **decision** baseline, not physics. Picks the affordable quest with the best krone/day; skips battles unless `WinsBattle(balance)`; refreshes when nothing qualifies. Used by tests, not by the harness.
- `world_test.go`, `board_test.go`, `bot_test.go` — see below.
- `scripts/run_bot/main.go` (optional, small) — print `Observe` → bot `Apply` until `Done`, so a reviewer can watch a run without Ebiten.

Do **not** wire `scripts/run_agent` off the stub in this PR. That is Davis’s integration milestone. Shipping `engine.New(seed)` that implements `Game` is enough for Max to swap later.

## Tests (this is the acceptance bar)

Milestone 2: “the engine plays a scripted bot through a battle and one delivery.”

1. **Compile-time `Game` implementation.**
2. **Same seed → identical first board and identical refresh boards.**
3. **Board constraints:** always 5 quests, ≤2 battles, delivery days in {2,3,4} with matching 60/70/100 payouts, battles cost 2 / pay 120.
4. **`Apply("deliver")`** credits the table payout, debits the distance, sets `Earned` correctly, does not change balance on a second illegal deliver.
5. **Fight at 0 krone loses** (balance unchanged, 2 days gone, `PayBattleLoss == 0`).
6. **Fight at 150 krone wins** (+120, 2 days gone). Aligns with existing `TestWinThresholdIs150`.
7. **Invalid `Apply`** (unknown name, bad json, wrong quest id, deliver with no active delivery, accept while already active) returns error and deep-equals the pre-Apply world.
8. **LegalActions** never includes a quest the agent cannot afford; does include an unwinnable `fight` once a battle quest is accepted.
9. **Bot run:** scripted bot on a fixed seed completes at least one delivery and, once it has ≥150, one battle. Assert `Done` with `LifespanDays == 0` (or bot stops when no legal action remains) and `Balance == total Earned` (no negative payouts).
10. **`go test ./...`** still green for harness + stub. We must not break Max.

No Ebiten in the test binary. No LLM code.

## PR writeup (rationale the original issue asked for)

State in the PR body:

- Engine/framework: **no game framework.** The harness needs a decision API, not a 2D loop. Ebiten was the right call for a human demo and remains on `malcome-go-engine`.
- Kept: deterministic battle math, lifespan ledger, seeded board, Mulberry32.
- Dropped: physics, levels, per-frame `Input`, Ebiten client, fighter HUD — they put platforming skill in the eval score.
- Constants aligned to `engine/balance.go` (50 days, delivery 2–4, payout table, 5-quest board, ≤2 battles).
- `malcome-go-engine` is intentionally unmerged.

## Implementation order

1. Create `malcolm-engine-game` from `origin/davis-contracts-lifespan`. Confirm `go test ./...` is green on the contracts branch before adding files.
2. Copy `rng.go`. Add slim `ledger.go` + tests.
3. Add `board.go` generator + tests (seed, size, battle cap, payout table).
4. Add `world.go` implementing `Game` (`accept_quest` / `deliver` / `fight` / `refresh_board`).
5. Add `world_test.go` covering Apply invariants and the 150-krone fight.
6. Replace physics bot with decision `bot.go` + a full-run test.
7. Optional `scripts/run_bot` text dump.
8. `go test ./...`. Open PR.

## Out of scope

- Rebasing or deleting `malcome-go-engine`.
- Editing `engine/engine.go` or `engine/balance.go`.
- Switching `scripts/run_agent` from `stub.New()` to `engine.New()` (integration; Davis).
- Rest/shop/spending/HP persistence/visuals.
- Official “lazy baseline” vs “oracle solver” (Davis’s later scoring work). Our bot is only a test driver that never touches a d-pad.
