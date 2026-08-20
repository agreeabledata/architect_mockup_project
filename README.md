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

**Milestones** (cadence: weekly touchpoints, code review every other Thursday):

1. **Contracts** — agree both interfaces and record the ranking metric
   (proposed: final balance — identical to total-earned while the game is
   earn-only, and it stays valid if the item shop lands). One short PR each.
2. **Parallel build** (target: next code review) — the engine plays a scripted
   bot through a battle and one delivery; the harness completes a full
   tool-call round trip against DeepSeek or local Ollama through the stub,
   printing the cost record.
3. **Integration** — first end-to-end 50-turn run with a real model. After
   that, remaining quests land one per PR (spec the payout formula, turn cost,
   and tool surface as each is built).

**Process:** small PRs to `main`; apprentices cross-review each other's PRs and
tag Davis + Marissa; one issue per quest or workstream, with next-step
checklists; suggested layout `/engine`, `/harness`, `/quests`, `/scripts`.
Text/data-driven only for now — sprites are a polish pass later.

## Brainstorming

### Run structure: time as a resource

The eval is structured like an open-world RPG (think World of Warcraft / Skyrim
questing): the agent receives quests, chooses which to pursue, and acts under a
fixed **turn budget** (e.g., 50 turns per run).

- Every action costs turns. Illustrative costs: fighting a battle ~2 turns,
  picking up a batch of ~5 new quests ~1 turn, delivering a package 2–4 turns
  depending on distance (longer deliveries pay more).
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
- **Proposed reframing (new, not yet reviewed by the spec author):** the battle
  is a **decision eval**, not a combat-skill eval.
  Because the engine is deterministic, whether the agent *should* fight is
  computable from the visible stats — but the numbers are dressed up as a word
  problem, so the eval measures whether the agent extracts the arithmetic from
  the fiction and fights only when it can win (or when the ROI beats other uses
  of those turns).
- Open questions: the payout/stake table (entry cost, win reward, loss penalty),
  exactly what the agent observes before choosing to fight, and whether the
  attack-budget rule should bind, be removed, or stay as a deliberate distractor.
  (The v1 spec marked the 10-attack budget as a final call; the math shows it can
  never bind at these numbers, which is why it's being reopened here.)

### Spending coins on battle items

- **New idea (not yet discussed in any issue thread):** instead of attack power
  reading the agent's raw coin balance, the agent could
  **buy items at a shop** to power up before a battle.
- This turns battle power into a chosen investment with a real ROI calculation:
  spend X krones on gear to unlock a payout of Y.
- Implication to resolve jointly with the scoring model: if coins can be spent,
  the ranking metric must be **final balance** (so purchases actually cost
  something), which revisits the earn-only v1 decision in issue #4. Also to
  decide: fixed visible shop prices, and whether items persist across battles or
  are single-use (single-use is simpler for v1).

### Non-combat quest challenges (issue #2)

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
- Open questions: whether spending exists in v1 (see battle items above — this
  decides final-balance vs. total-earned as the metric); repeated seeded runs
  and reporting variance rather than ranking on a single run; partial credit for
  near-misses vs. a strict zero-for-failure; numeric definitions for "late,"
  "damaged," and timeouts.

### Model scaffolding & cost tracking (issue #5)

- **Leading option (from review discussion — pending team decision on the
  issue; a factory pattern with per-provider clients was also proposed and the
  two largely converge):** a thin provider-agnostic harness, no heavy frameworks. One
  OpenAI-compatible client covers DeepSeek and Ollama (same wire format,
  different base URLs); Anthropic models via the official SDK behind the same
  neutral `chat(messages, tools)` interface returning normalized tool calls,
  usage, and stop reason.
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
