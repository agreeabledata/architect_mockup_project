# Architect Mockup Project

An RPG-style evaluation environment: LLM agents run quests in a small game world,
earn an in-game currency (**krones**), and are compared by how much they earn under
the same constraints. The point is to measure agent decision-making — planning,
tool use, resource management, and opportunity cost — with an objective score.

Work is tracked in GitHub Issues. Design ideas live in the Brainstorming section
below — **add new ideas directly to this section** (small PRs are fine), and open
an issue when an idea is ready to be worked on.

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

- v1 spec ("Single Battle Rules — Coin-Powered"): a fixed-turn duel — agent
  50 HP vs. opponent 100 HP, opponent deals a flat 8 damage per round, agent
  attack power derived from its coin balance. Fully deterministic, no randomness.
- Current framing: the battle is a **decision eval**, not a combat-skill eval.
  Because the engine is deterministic, whether the agent *should* fight is
  computable from the visible stats — but the numbers are dressed up as a word
  problem, so the eval measures whether the agent extracts the arithmetic from
  the fiction and fights only when it can win (or when the ROI beats other uses
  of those turns).
- Open questions: the payout/stake table (entry cost, win reward, loss penalty),
  exactly what the agent observes before choosing to fight, and whether the
  attack-budget rule should bind, be removed, or stay as a deliberate distractor.

### Spending coins on battle items

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

- Direction: a thin provider-agnostic harness, no heavy frameworks. One
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
- Fairness rules: identical prompts, tools, temperature, and caps for every
  model; one uniform malformed-tool-call policy; full per-run transcripts logged
  for audit.
