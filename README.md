# Architect Mockup Project

An RPG-style evaluation environment: LLM agents run quests in a small game world,
earn an in-game currency (**krones**), and are compared by how much they earn under
the same constraints. The point is to measure agent decision-making — planning,
tool use, resource management, and opportunity cost — with an objective score.

Work is tracked in GitHub Issues. Design ideas live in the Brainstorming section
below — **add new ideas directly to this section** (small PRs are fine), and open
an issue when an idea is ready to be worked on.

## Implementation plan

The brainstorming phase is complete (issues #1–#5, captured below). Implementation
splits along the **engine/harness seam** — the two workstreams touch only through
two small interfaces, so they proceed in parallel without blocking each other:

- **Game engine — Malcolm (mfeinLearn).** Pure and deterministic, no LLM code
  anywhere: the turn ledger, seeded quest board, battle resolver, and the first
  quest (Courier — deterministic, simplest). Testable end-to-end with scripted
  bots and unit tests; no API keys needed.
- **Model harness — Max (maxpeterkaya).** The OpenAI-compatible client
  (DeepSeek + Ollama via base URL), the agent runner loop, and cost tracking
  (normalized token usage per run + a pricing config). Developed against a
  hardcoded stub engine with two fake quests, so it never waits on the real one.
- **Contracts, scoring & integration — Davis.** The two interface definitions
  (the engine's `observe / legalActions / apply` surface, and the neutral
  `chat(messages, tools)` types), the scoring/balance numbers, and later the
  scripted baseline bots that sanity-check quest balance.

**Settled at the 2026-08-20 office hours** (issues #6–#11):

- **Everything is Go, in one monorepo** — engine and harness are packages, not
  separate repos. Max bootstraps the skeleton first (#6); everything else waits on it.
- **v1 ships exactly two quest types: combat and the Courier delivery** (#7). The
  rest land after the first end-to-end run works.
- **No spending in v1** (#9): krones are both the score and the XP — combat power
  reads the raw balance, so earning is progression. Metric is final balance.
- **The turn budget is the agent's lifespan** (#8): every action spends life from a
  fixed allotment. Cost table still to be set.

**Milestones** (cadence: weekly touchpoints, code review every other Thursday):

0. **Repo skeleton** (#6) — Go monorepo, package layout, `go test ./...` passes.
   Blocks 1 and 2.
1. **Contracts** — **done.** `engine/engine.go` is the ratified engine seam
   (`Observe / LegalActions / Apply`) and `harness/chat.go` the neutral model
   seam; `engine/balance.go` holds every economy number. Ranking metric is final
   balance (identical to total-earned while the game is earn-only, and still
   valid if an item shop ever lands). Changes to either seam go through a PR.
2. **Parallel build** — **done.** Engine (#13): headless `engine.Game`, rules in
   `docs/engine.md`, scripted bot via `go run ./scripts/run_bot -seed 1`.
   Harness (#11): both clients, agent loop, cost record, stub round trip.
3. **Integration** — **done 2026-09-02.** First end-to-end 50-day run:
   `llama3.2:3b` on Ollama, seed 1, 1390 krones in 41 model calls (greedy bot:
   1650). The model fought at 0 and 70 krones and lost both — the trap works.
   **Office hours 2026-09-03:** that run recorded 41 decisions with zero
   rationale. Every tool schema the harness sends now requires a `reasoning`
   string; it is stripped before `Apply` and kept in the transcript.
   Next: remaining quests land one per PR (spec the payout formula, day cost,
   and tool surface as each is built).

**Running it:**

```sh
cp config.example.json config.json   # pick a provider; local Ollama needs no key
go run ./scripts/run_agent -seed 1   # real engine; add -stub for the stub
go run ./scripts/run_bot -seed 1     # scripted baseline, no model
go run ./scripts/replay transcripts/run_….jsonl   # one line per turn, from kind=="turn"
go run ./scripts/compare -seeds 1,2,3 -configs bot,config.json
```

`iteration_cap` bounds model calls per run; a full game takes ~40–50, so raise
it from the example's 10 before expecting a terminal run. Within one decision a
rejected action retries up to `decision_retry_cap` (default 3) calls; the model
sees only the system prompt plus the last 3 turns (full history is kept in the
transcript). The windowed context changed the model inputs, so `prompt_version`
is now `v3` and `config_hash` inputs changed with it: transcripts from before
that bump are **not comparable** to runs of the same model and seed after it.

Transcripts are JSONL with a `kind` on every line (`header`, `call`, `turn`).
`scripts/replay` prints one terminal line per `kind=="turn"` record. `scripts/compare`
runs the greedy bot as the baseline, then each config on each seed, and prints a
markdown table whose numbers come from those structured records — not the prose.

**Running against OpenRouter.** OpenRouter speaks the OpenAI-compatible Chat
Completions API, so it reuses the existing client — no new provider code. Set
`"provider": "openrouter"` and `"api_key": "<your OpenRouter key>"`, leave
`base_url` empty (it defaults to `https://openrouter.ai/api/v1`), and run
`go run ./scripts/run_agent`. This is the cheapest way to compare several
vendors' models under one key and one code path. Note that OpenRouter model IDs
are `vendor/model` — e.g. `meta-llama/llama-3.3-70b-instruct`, not
`llama-3.3-70b-instruct` — and that the same string is the `pricing` lookup key,
so a model with no matching entry reports $0.00. OpenRouter's per-model rates
depend on which upstream provider it routes to, so confirm the current numbers
rather than trusting the example config's.

**Process:** small PRs to `main`; apprentices cross-review each other's PRs and
tag Davis + Marissa; one issue per quest or workstream, with next-step
checklists; suggested layout `/engine`, `/harness`, `/quests`, `/scripts`.
Text/data-driven by default; visuals are optional and only if they help the
author think while coding — if any, 2D with circles and squares plus text labels,
no art. Engine/framework choice is the engine author's call, with the rationale
in the PR.

## Brainstorming

### Run structure: lifespan as the resource

The eval is structured like an open-world RPG (think World of Warcraft / Skyrim
questing): the agent receives quests, chooses which to pursue, and acts under a
fixed **lifespan** (~50 turns, one turn = one in-game day). Framing the budget as
the agent's life rather than a neutral turn counter makes the opportunity cost
explicit — taking a quest spends life the agent cannot get back.

- Every action costs days of lifespan. **Settled (#8), and the numbers now live in
  `engine/balance.go` — that file is the only place the economy is defined:**
  50 days of lifespan, starting balance 0. A battle costs 2 days; a delivery costs
  its distance, 2–4 days; drawing a fresh board of 5 quests costs 1 day.
- **Payouts (#8):** deliveries pay 60 / 70 / 100 for short / medium / long, so the
  short run pays best per day and ordering the board is a real decision. A won
  battle pays 120 (60/day); a lost one pays 0 and still costs its 2 days. Nothing
  in v1 has a negative payout, so final balance is exactly total earned (#9).
- **Combat unlocks at 150 krones** — below that the deterministic duel is
  unwinnable, so the agent must fund itself on deliveries before it can fight. Once
  it can, combat pays roughly double a delivery per day, and the limiter is supply:
  a board holds 5 quests, at most 2 of them battles. The run therefore has a shape —
  fund up, then farm — that the agent has to work out from the arithmetic.
- The core eval signal is **opportunity cost**: does the agent make good calls
  about which quests to take, when to acquire more quests vs. execute the ones it
  has, and when a payoff justifies the turns spent.
- Every quest spec therefore needs two numbers: a **krone payout** and a
  **turn cost**. Balancing krones-per-turn across quest types is the main design
  work — if one quest type dominates, the optimal strategy collapses to spamming it.
- The quest board and world should be seeded so every agent faces identical
  offerings for a given seed; scores can then be reported as a fraction of the
  optimal (or a strong baseline) plan for that seed.
- "Turn" is in-game time, separate from LLM iterations. The harness keeps its own
  per-decision iteration cap for cost control (see scaffolding below).

### Battle / combat mechanics (issue #1)

- v1 spec ("Single Battle Rules — Coin-Powered", by mfeinLearn — see BATTLE.md on
  the issue): a fixed-turn duel — agent 50 HP with a 10-attack budget vs. opponent
  100 HP, opponent deals a flat 8 damage per round, agent attack power
  `max(1, floor(coins / 10))`. Fully deterministic, no randomness.
- **Reframing (agreed 2026-08-20, spec author present):** the battle
  is a **decision eval**, not a combat-skill eval.
  Because the engine is deterministic, whether the agent *should* fight is
  computable from the visible stats — but the numbers are dressed up as a word
  problem, so the eval measures whether the agent extracts the arithmetic from
  the fiction and fights only when it can win (or when the ROI beats other uses
  of those turns).
- **Settled:** entry cost 2 days, win pays 120, a loss pays 0 (the wasted days are
  the penalty — no negative payouts in v1). The agent observes its own balance, its
  remaining lifespan, and the opponent's HP and damage; the win condition is
  derivable from those and nothing else. The 10-attack budget **stays in the quest
  prose as a deliberate distractor but is not implemented** — the agent dies or wins
  within 7 attacks at these numbers, so tracking it would be dead code.
  `engine.WinsBattle` and its test pin the 150-krone threshold and assert the budget
  stays non-binding.

### Spending coins on battle items — deferred past v1 (#9)

Decided on 2026-08-20: **v1 has no shop and no spending.** Attack power reads the
raw krone balance, which makes krones double as XP. Revisit after the first
end-to-end run; the idea as originally written:

- Instead of attack power reading the agent's raw coin balance, the agent could
  **buy items at a shop** to power up before a battle.
- This turns battle power into a chosen investment with a real ROI calculation:
  spend X krones on gear to unlock a payout of Y.
- Implication to resolve jointly with the scoring model: if coins can be spent,
  the ranking metric must be **final balance** (so purchases actually cost
  something), which revisits the earn-only v1 decision in issue #4. Also to
  decide: fixed visible shop prices, and whether items persist across battles or
  are single-use (single-use is simpler for v1).

### Non-combat quest challenges (issue #2)

**v1 takes the Courier only** (#7). The Merchant's Bargain is liked but needs an
objective scoring model before it can be an eval — negotiation is subjective as
written; Arcane Dispersal is the likely next one in; Survival Trek is deferred.

- Four quest writeups: **The Courier's Errand** (timed delivery with package
  condition), **The Merchant's Bargain** (negotiation against a hidden minimum
  price), **The Arcane Dispersal** (spell-sequencing puzzle under a mana budget,
  with paid scrying), **The Survival Trek** (zone traversal managing food, water,
  and morale). Design principle: each quest stresses exactly one limiting
  resource (time, money, mana, supplies).
- Open questions: per-quest krone payouts and turn costs; the tool surface each
  quest exposes (actions, observations, hidden state); differentiating Courier
  (deterministic, full-information planning) from Trek (stochastic,
  partial-information risk management); how the agent learns the correct order
  in Arcane Dispersal.
- Future ideas: an investigation quest (pay per clue, commit to a conclusion);
  a two-faction diplomacy quest (parked until "trust" has a numeric model).

### Eval scoring model (issue #4)

- Proposal: agents accumulate krones across the run; payouts reward quality, not
  just completion (earliness and condition for delivery, budget saved for
  negotiation, leftover mana, surplus rations); failure earns zero; highest
  total wins.
- Under the turn economy, krones are already the common unit — the balancing
  question is krones-per-turn across quest types rather than post-hoc
  normalization.
- **Settled:** no spending in v1, and the metric is final balance (#9).
- Open questions: repeated seeded runs
  and reporting variance rather than ranking on a single run; partial credit for
  near-misses vs. a strict zero-for-failure; numeric definitions for "late,"
  "damaged," and timeouts.

### Model scaffolding & cost tracking (issue #5)

- **Decided (#11):** a thin provider-agnostic harness in Go, no heavy frameworks.
  One OpenAI-compatible client covers DeepSeek and Ollama — confirmed on the call
  that Ollama exposes OpenAI-compatible endpoints, so it is one SDK with a
  different base URL; Anthropic models via the official SDK behind the same
  neutral `chat(messages, tools)` interface returning normalized tool calls,
  usage, and stop reason. Built against a stub engine so it never waits on the
  real one. **Confirmed in practice at milestone 2:** no Go Ollama SDK was
  needed — the OpenAI-compatible client ran the first full tool-call round trip
  against a local Ollama instance using only `base_url`.
- Cost tracking: every call returns normalized token usage; the runner
  accumulates per quest run alongside iteration count and wall time; dollar cost
  from a per-model pricing config (null for local models). Headline efficiency
  metrics: iterations-to-success, success rate, tokens per run, krones per run.
- A hard per-decision iteration cap is both the main cost control and a natural
  failure condition.
- Fairness rules: identical prompts, tools, and caps for every model, and
  identical decoding settings where the provider supports them (some current
  models reject sampling parameters — divergences get logged); one uniform
  malformed-tool-call policy; full per-run transcripts logged for audit.
- **Settled 2026-09-03:** add a required `reasoning` string to every tool the
  harness sends (`actionsToTools` in `harness/agent.go`). Strip it before
  `Apply` so the engine never sees it; leave the raw call in the transcript.
  Small models skip free-text content and jump to the tool call; the schema
  field is the scaffolding that forces a recorded rationale per decision.

### Eval best-practices notes (from a survey of agent benchmarks)

Distilled from published agent evals (BALROG, τ-bench, ARC-AGI-3, TALES,
GSM-Symbolic) and eval-methodology guidance from Anthropic and METR:

- Report **pass^k** ("succeeds k out of k tries") per quest type alongside mean
  score — consistency is a separate axis from capability.
- **Regenerate combat arithmetic from templates per seed** (fresh names and
  numbers), so no fixed instance can be memorized; spot-check score stability
  across re-instantiations.
- Ship a scripted **"lazy baseline" bot** (e.g., always take the cheapest safe
  quest) and a per-seed **oracle solver**; leaderboard numbers only mean
  something between those two anchors — a lazy bot scoring near the models
  means the economy is exploitable.
- Seed **trap quests** (negative expected krones-per-turn) so declining is
  sometimes the correct move and decision quality is scored, not just execution.
- Let models emit **free-text reasoning before the structured action** — never
  demand schema-first output (format constraints measurably degrade reasoning,
  especially for small local models).
- Report **malformed-action rate per provider** as a first-class metric, to
  separate harness artifacts from model capability.
- **Variance-triage rule:** if per-seed variance exceeds a threshold, review
  transcripts and classify failures before trusting the number.
- **Hash and log the full eval config** (prompt version, model, decoding
  settings, seed) into each transcript header, so every result is attributable
  to (model × prompt-version × seed).

```
provider      openrouter (meta-llama/llama-3.3-70b-instruct)
engine        seed 1
final balance 330 krones
iterations    12  (cap 60)
tool calls    11
tokens        22682 in / 975 out / 23657 total
wall time     1m15.050710863s
cost          $0.0033
reason        no_action
transcript    transcripts/run_20260914T184721.600107000Z.jsonl
```
