# KRONE walkthrough (Malcolm’s crib sheet)

Use this if someone asks what you built. Current branch: `malcolm-engine-game`. PR: [#13](https://github.com/Flatiron-Apprentices/architect_mockup_project/pull/13), stacked on Davis’s contracts PR [#12](https://github.com/Flatiron-Apprentices/architect_mockup_project/pull/12).

## What it is, in one breath

KRONE is an eval world, not a game you play with a controller. An LLM (or a scripted bot) is an adventurer with **50 days of life** and **0 krones**. It picks courier deliveries and fights off a seeded board. Final krone balance is the score. Combat power reads the raw balance and **never spends it**.

You own the **engine**. Max owns the **harness** (the LLM loop). Davis owns the **contract** (`Observe` / `LegalActions` / `Apply`) and the **economy numbers**. The engine has **zero LLM code**.

## Who owns which files

| Path | Owner | What it is |
|---|---|---|
| `engine/engine.go` | Davis | `Game` interface. Do not edit. |
| `engine/balance.go` | Davis | Every economy number. Do not edit. |
| `engine/world.go` | Malcolm | Real game: `New`, `Observe`, `LegalActions`, `Apply` |
| `engine/ledger.go` | Malcolm | Days + krones + event log |
| `engine/board.go` | Malcolm | Seeded 5-quest board |
| `engine/rng.go` | Malcolm | Mulberry32 (same seed → same run) |
| `engine/bot.go` | Malcolm | Decision baseline, not a jumper |
| `harness/` | Max | LLM client + agent loop |
| `stub/` | Max | Fake two-quest engine so Max never waited on you |
| `scripts/run_agent` | Max | Still calls `stub.New()`, not `engine.New` |
| `scripts/run_bot` | Malcolm | Prints a full bot run |
| `malcome-go-engine` branch | Malcolm | Ebiten rooftop demo; **intentionally unmerged** |

## How a run works

```
engine.New(seed)
        │
        ▼
   Observe()  →  narrative, balance, days left, done?
        │
   LegalActions()  →  accept_quest / deliver / fight / refresh_board
        │
   Apply(name, json args)  →  one whole decision (not one frame)
        │
        └── loop until days == 0  (Done = true). Score = balance.
```

That is the same loop the harness will drive. One LLM call ≈ one `Apply`. There is no 60 fps input.

Constructor: `engine.New(seed uint32) *World`. Seed `0` becomes `1`. Same seed → identical boards, refreshes, and outcomes.

## The four actions

| Action | Args | Cost | When legal |
|---|---|---|---|
| `accept_quest` | `{"quest_id": "..."}` | 0 days | no active quest; quest is open and affordable |
| `deliver` | none | 2, 3, or 4 days | active quest is a delivery |
| `fight` | none | 2 days | active quest is a battle |
| `refresh_board` | none | 1 day | no active quest; at least 1 day left |

Rules people quiz:

- You may hold **one** quest. No abandon. Cannot refresh while holding one.
- After accept, the only legal action is `deliver` or `fight`.
- Unaffordable quests are hidden. Forcing them with `Apply` **errors and does not mutate**.
- Fighting while too poor **is legal** — that is the trap the eval scores.
- There is no `rest`, no d-pad, no per-punch combat.

## The two quest types

**Delivery** — courier packet. Distance is the cost (2 / 3 / 4 days). Pays 60 / 70 / 100. Short run is best per day (30/day vs 23 vs 25), so board order is a real decision. `Apply("deliver")` completes the whole job. No walking, no jumping.

**Battle** — its own quest type, not a thug on a courier route. Costs 2 days. Win pays 120, loss pays 0 (the wasted days are the penalty). HP does not persist: every fight is a fresh 50 vs 100.

A board is **5 quests, at most 2 battles**. Refresh discards leftovers and draws a new five.

## The eval (memorize this)

The model is supposed to extract arithmetic from the fiction.

- Start: **50 days, 0 krones**
- Attack power = `max(1, floor(krones / 10))` — krones are never spent
- You: 50 HP. Foe: 100 HP, 8 damage/round. You strike first
- **Fighting is a guaranteed loss below 150 krones, a guaranteed win at 150+**
- “10-attack budget” is **flavor only**. At these numbers you die or win in 7 rounds, so the engine does not track it
- Shape of a good run: fund on deliveries until 150, then farm battles (60/day vs 30/day for a short delivery). Supply is the limiter (≤2 battles per board)

`WinsBattle(balance)` and `AttackPower` live in Davis’s `balance.go`. `world.go` calls them; it does not reimplement the math.

## File map if someone says “show me the code”

Start here:

1. `engine/engine.go` — the three methods the harness sees
2. `engine/world.go` — the whole game loop and action handlers
3. `engine/balance.go` — why 150 / 50 days / 120 payout
4. `engine/bot.go` — what “scripted bot” means
5. `engine/world_test.go` + `bot_test.go` — the acceptance tests

Bot policy (`StepBot`): if a quest is already accepted, finish it. Else take the affordable job with the **best krones per day**, skipping battles unless `WinsBattle`. Else refresh.

## What we used to have, and why it changed

First version was an Ebiten rooftop platformer (`malcome-go-engine`). Ledger, battle math, seeded board, and tests were right. The interface was wrong:

```
StepSim(s, Input{MoveX, JumpPressed, JumpHeld, ...})  // 60 times a second
```

A model cannot hold-jump. And if a run fails from a missed ledge, the eval is scoring platforming, not “is this fight winnable at 150?”

**Kept:** ledger idea, battle constants, Mulberry32, seeded board idea.  
**Dropped:** `physics.go`, `levels.go`, per-frame `Input`, `cmd/krone`, fighter HUD.  
**Framework choice:** no game framework for the harness engine. Ebiten stays on the demo branch.

That branch shares **no git history** with `main`, so it cannot merge. We did not rebase it. We branched `malcolm-engine-game` from Davis’s contracts branch instead.

## What we did *not* do (say this if asked)

- Did not wire `scripts/run_agent` to `engine.New` — still `stub.New()`. That’s integration (Davis / Max).
- Did not edit `engine.go` or `balance.go`.
- Did not add a shop, spending, rest, or persistent HP.
- Did not ship Davis’s official lazy-baseline vs oracle. Our bot is only a test driver.
- There is no human prompt (`accept_quest` / `deliver` typed by hand). Human readout is `go run ./scripts/run_bot`.

## Commands

```bash
# this branch
go test ./...
go run ./scripts/run_bot -seed 1

# rooftop demo (other branch)
git checkout malcome-go-engine
go run ./cmd/krone
```

On seed 1 the bot funds on deliveries, then farms battles, and finishes around **1650 krones / 0 days**.

## Likely questions, short answers

**“Can a model play it?”**  
Yes, through `Game`. Observe → tools from LegalActions → one Apply. Not yet hooked to `run_agent`.

**“Where is the luck?”**  
Nowhere in combat. Board contents are seeded RNG, fully reproducible.

**“Why 150?”**  
50 HP, foe 100 HP / 8 dmg. You get 7 strikes before you die. Need `ceil(100 / power) ≤ 7` → power 15 → `floor(krones/10) = 15` → 150 krones.

**“Does fighting spend coins?”**  
No. Issue #9. Balance is both score and XP.

**“What if the model takes a fight at 0 krones?”**  
Legal. It loses, pays 0, burns 2 days. That’s the eval working.

**“Why not merge the platformer?”**  
Unrelated git history, and it scores the wrong skill. Keep it as a human demo.

## What the LLM actually does in the game

The LLM is the adventurer. Each turn it reads a short story (days left, krones, the board) and picks **one** tool: take a job, deliver it, fight, or redraw the board. That’s the whole game — no walking, no jumping.

A typical run looks like this:

1. **Wake up broke.** 50 days, 0 krones. Attack power is 1, so a fight is suicide.
2. **Read the board.** Five jobs: courier runs (2–4 days, 60/70/100 krones) and maybe a fight (2 days, 120 if you win).
3. **Fund up.** It should take short deliveries first (30 krones/day, best rate while poor) until it hits **150 krones**. That’s when `floor(krones/10)` becomes 15 and the 100 HP foe dies before you do.
4. **Then farm fights.** A won battle pays 120 for 2 days (60/day) — about double a delivery. At most two fights per board, so it will refresh when the good ones are gone.
5. **Clock runs out.** Days hit 0. Score = krones in the purse.

What the eval is actually scoring:

- Does it extract “I win at 150” from the flavor text (50 HP, foe 100 HP, 8 dmg/round, power = krones/10)?
- Does it skip fights before that? Taking one at 0 krones is *allowed* — it just wastes 2 days and earns nothing. That’s a trap.
- Does it notice the short delivery is better per day than the long one (30 vs 25 vs 23)?
- Does it spend life where the payoff is worth it, instead of refreshing for no reason?

The “10-attack budget” in the story is a distractor. A model that treats it as a real rule is chasing fiction; the engine never checks it.

It is **not** hooked up yet. Max’s `run_agent` still talks to a fake stub. After Davis swaps in `engine.New(seed)`, the model will play exactly this loop on its own.

## Is it just adding and taking away numbers?

Yes. Under the story, it is a resource puzzle: add krones, subtract days, stop when days hit 0. Highest purse wins.

The only “combat” is more of that math. Your hit power is `krones / 10`. Below 150 you lose the fight (0 pay, still lose 2 days). At 150+ you win (+120, still lose 2 days). No dice, no aiming, no walking.

What the eval cares about is whether the model **notices** that from the flavor text and spends life on the better rate — short deliveries until 150, then fights — instead of taking a hopeless duel or a worse courier run.

## What are days in the game?

**Days are the adventurer’s remaining life** — the turn budget. You start with 50. When they hit 0, the run is over and the score is however many krones you have.

Every real action spends some:

- Refresh the board: 1 day
- Fight: 2 days
- Delivery: 2, 3, or 4 days (how far the packet has to go)
- Accepting a quest: 0 (you’re just picking; the cost is when you actually do it)

A “day” is in-game time, not a clock and not an LLM call. The harness can take as many model turns as it wants on one decision; the engine only cares that you spent a day of life.

That’s why the eval is opportunity cost: a day spent on a bad fight or a slow delivery is a day you cannot spend on something that pays more.

### Is a day a minute, an hour, etc.?

It isn’t a minute or an hour. A “day” is just the name for **one turn of life**.

The fiction is “you have 50 days left to live.” Mechanically that is a counter of 50. Take a 2-day delivery and the counter becomes 48. There is no clock, no calendar, and no mapping to real time.

The harness’s LLM calls are separate: the model can think for seconds or a minute on one choice, and that still costs whatever days that action lists — 1 to refresh, 2 to fight, 2–4 to deliver.

## What is Mulberry32?

**Mulberry32** is a tiny seeded random-number generator. You give it a number (the seed); it spits out a stream of values in `[0, 1)`. Same seed → same stream every time.

It is not a dice roll in the live sense. It is fake-random on purpose so two agents with seed `1` see the **same quest board**, the same refreshes, and can be compared fairly.

In this engine it only shuffles the board: how many fights (0–2), which slots they occupy, delivery length (2/3/4 days), and flavor names. Combat itself does not use it.

Go’s default `math/rand` would also work, but Mulberry32 is a few lines, has no extra package, and is easy to reproduce. If the seed is `0`, the code substitutes a fixed nonzero constant so the stream is never stuck.

## Why isn’t Ebiten used?

Yes — because an LLM cannot hold down the jump button (and the rest of a gamepad).

Ebiten is a 60-frames-a-second game loop. It wants “move, jump, hold jump for height” every tick. An LLM call takes about a second, and “hold jump” is not a decision a model makes.

Even if you slowed it down, a missed ledge would fail the run. Then you’d be scoring platforming, not “does it know a fight is winnable at 150 krones?”

So the harness engine has no Ebiten and no jump button. One `Apply` is a whole delivery or fight. The rooftop demo still uses Ebiten on `malcome-go-engine` for humans.

## Is there a game board? Like a 2D grid?

No 2D grid. Nothing to walk around on in this version.

“Board” here means a **list of five jobs** (like a quest notice board): courier runs and maybe a fight, with a cost and a payout. The model picks one. There are no tiles, rooms, or coordinates.

The old Ebiten demo on `malcome-go-engine` *did* have a side-view rooftop (platforms, coins, a dropoff). That map is not part of the eval engine. A delivery is just “spend 2–4 days, gain 60/70/100 krones.”

## Does the LLM only look at the words and act on what it reads?

Yes. The model never sees a map, a sprite, or a controller. It reads text, then calls one tool.

Each turn the harness hands it something like:

- A **story** (`Observe().Narrative`): days left, krones, attack-power formula, the five jobs with costs and payouts.
- The same numbers as fields (`Balance`, `LifespanDays`).
- A **menu of tools** (`LegalActions`): `accept_quest`, `deliver`, `fight`, `refresh_board`, each with a short description.

It thinks in prose, then picks exactly one tool (for `accept_quest`, it also passes a `quest_id`). The engine applies that and writes the next paragraph. Repeat until days hit 0.

So it is acting on words (and the tool list derived from those words). If it does the arithmetic hiding in the flavor text, it funds to 150 and then fights. If it just vibes off the story, it might take a hopeless duel. That is the whole eval.

## How long does a token (word) take to generate from the game?

The game does not generate tokens, and it does not wait on a clock to “print” words.

`Observe()` is ordinary Go: it builds a string and returns it immediately (milliseconds or less). Those words are not streamed from a model. There is no “one token every X ms” inside the engine.

Tokens show up later, on **Max’s harness**, when the LLM *replies* to that story. That whole reply (think + tool call) is often on the order of **about a second**, but that is the model’s latency, not a game rule. One slow or fast reply still costs the same in-game days.

So: game text = instant. Model tokens = however long the provider takes. Those two clocks are not connected.

## Where is the story coming from? Is it already stored in the game?

The story is **not** a stored novel, and an LLM does not write it. The engine pastes a **template** together every time `Observe()` runs.

The sentences live in Go (`world.go` / `board.go`): “You have X days… Y krones… attack power is max(1, floor(krones/10))…” It fills in the current numbers, then lists the five jobs.

Job **names** *are* stored — short lists like “Harbour parcel” and “Roof thug.” Mulberry32 picks which names appear. The **blurb** is also a template: “A sealed courier packet. 3 days on the road, 70 krones if it arrives.”

So: canned phrases + live stats. Same seed → same names and numbers. The model only *reads* that paragraph; it does not generate the world’s text.

## Reading a `run_bot` log (`go run ./scripts/run_bot`)

This printout is the **scripted bot** playing, not an LLM. Each `======== step N ========` block is: the story it sees (`Observe`), then the one action it took (`Apply`). Accepting a job costs **0 days**, so the “Day” counter only moves when it actually delivers, fights, or refreshes.

### Step 0 — broke, pick the best rate

50 days, **0 krones**, attack power **1**. A fight would lose.

Rates on the board:

| Job | Pay / days | Per day | Take it? |
|---|---|---|---|
| Night-market crate | 60 / 2 | **30** | yes (best) |
| Lantern-row / Old Ward | 100 / 4 | 25 | later |
| Harbour parcel | 70 / 3 | ~23 | later |
| Gate duelist | 120 / 2 | 60 | **no** — power 1 cannot win |

It `accept_quest` the crate. Days still 50. `earned=0` because accept does not pay.

### Step 1 — do the job

Status is `[active]`. Only legal move is `deliver`.  
**+60 krones, −2 days.** Now 60 / 48. Power becomes `floor(60/10) = 6` (still too weak to fight).

### Steps 2–3 — next-best delivery

Still under 150, so it still skips the duelist. Best remaining rate is 25/day, and **Lantern-row** is listed first, so it takes that 4-day / 100 run (not Harbour).  
**+100, −4 days** → **160 krones, 44 days.** Power **16**.

160 ≥ 150, so fights are now winnable.

### Steps 4–5 — first fight

Gate duelist is 60 krones/day, better than any leftover delivery. It accepts, then `fight`.  
Power 16 kills 100 HP in 7 rounds (`16×7 = 112`). You would have died on round 7 if it went longer (50 HP / 8 dmg), so this is the exact win threshold.  
**+120, −2 days** → **280 / 42.**

### Steps 6–9 — finish the leftover packets

No more fights on this board. It drains Old Ward (+100, −4) then Harbour (+70, −3).  
**450 krones, 35 days.** Every slot is `[complete]`.

### Step 10 — refresh

Nothing left to take, so it spends **1 day** and draws a new five. IDs go from `q-1-0-*` to `q-1-1-*` (seed 1, refresh 1).

### Steps 11–14 — farm fights

New board has **two** battles (the max). Both pay 60/day. It wins the first (power 45, 3 rounds), then the second (power 57, 2 rounds).  
450 → 570 → **690 krones**, 34 → 32 → **30 days.**

### Step 15 — back to deliveries

Fights on this board are done. Best leftover is Pier 4 at 30/day (60/2), better than the 3-day 70 (~23). It accepts that next.

**Pattern:** fund on the fattest delivery rate until 150 → take every fight → when the board is empty, refresh → repeat until days hit 0.

An LLM would see the **same paragraphs**. If it does that arithmetic, it plays like this. If it takes Gate duelist at step 0, it burns 2 days for 0 krones — that is the trap.
