# MEMORY.md checklist

Status of Malcolm’s original engine task and the review feedback. Checked = done on `malcolm-engine-game`.

## Original v1 scope

- [x] Malcolm owns the game engine; no LLM code in the engine
- [x] Engine is testable with scripted bots (no API keys)
- [x] Lifespan ledger
- [x] Seeded quest board
- [x] Deterministic battle resolver (outcome from visible stats — no luck)
- [x] Courier delivery quest
- [x] Combat power from the krone balance (no spending)
- [x] Start after the repo skeleton (engine lives in the monorepo `engine/` package)
- [x] Visuals optional — skipped; turns and outcomes print via `scripts/run_bot`
- [x] Engine/framework rationale written in a PR ([#13](https://github.com/Flatiron-Apprentices/architect_mockup_project/pull/13))

## What the review said was already right (platformer branch)

These were true on `malcome-go-engine` and still hold as the source of the port:

- [x] Battle constants match the spec (50 HP / 100 HP / 8 dmg / `max(1, coins/10)`)
- [x] Engine package has zero Ebiten imports
- [x] Ledger, seeded board, and Mulberry32 RNG are the v1 shape
- [x] Unit tests sit next to the logic

## The real problem (replace the interface)

- [x] Stop using per-frame controller input (`StepSim` + `Input{MoveX, JumpPressed, …}`)
- [x] Implement `engine.Game`: `Observe()` / `LegalActions()` / `Apply(name, args)`
- [x] Legal actions are `"accept_quest"`, `"deliver"`, `"fight"`, `"refresh_board"`
- [x] `Apply("deliver", …)` resolves the entire delivery and debits its days — no traversal, no jumping
- [x] Keep ledger / battle math / quests / rng; drop physics, levels, `cmd/krone`, fighter
- [x] Decision bot that chooses which quest to take and never touches a d-pad (replaced the coin-chasing jumper)

## Constants (align on port, do not rework the platformer branch)

- [x] `StartDays` 24 → `StartingLifespanDays` 50
- [x] Deliveries cost 2–4 days (not `TravelDays: 1` on every level)
- [x] Payouts from `engine/balance.go` (not per-level jittered rewards)
- [x] Consume Davis’s `engine/balance.go`; do not duplicate the table

## Git / process from the feedback

- [x] Keep the platformer branch (`malcome-go-engine`) — do not merge it
- [x] New work is mergeable with main’s history (branched from `origin/davis-contracts-lifespan` instead of rebasing the unrelated platformer)
- [x] Commit the engine port (`c6440c7` on `malcolm-engine-game`)
- [x] Open a PR into `main` (stacked on Davis’s #12 / `davis-contracts-lifespan`: [#13](https://github.com/Flatiron-Apprentices/architect_mockup_project/pull/13))
- [ ] Pair on Discord for the `engine.Game` port (optional; port is already implemented)
