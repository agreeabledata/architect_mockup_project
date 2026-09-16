# Arcane Dispersal

Third quest type (issue #21). A spell-sequencing puzzle whose limiting resource is
**mana**. All numbers live in `engine/balance.go`.

## Cost and payout

| | |
|---|---|
| Turn cost | `CostArcane` = 3 days, win or lose |
| Payout | `PayArcane` = 150 krones on success, 0 on collapse |
| Per-board cap | `MaxArcanePerBoard` = 1 |

## Rules

- `ArcaneNodes` = 3 nodes, each keyed to a different resonance (fire / water /
  earth). Every resonance appears exactly once, but which node holds which is
  hidden. Nodes are disrupted in order.
- The agent starts a ritual with `ArcaneMana` = 8 mana.
- `cast_spell` costs `ManaSpell` = 2 mana. The correct resonance disrupts the
  current node; a wrong one backlashes for `ManaBacklash` = 2 extra.
- `scry` costs `ManaScry` = 1 mana and reveals one undisrupted node's resonance.
- The ritual succeeds when all nodes are disrupted. It collapses (pays 0) when too
  little mana remains to act — including when every remaining node is already
  revealed and casting is unaffordable.

Scrying every node and casting every spell costs 3 + 6 = 9 mana, over budget. The
intended line is to scry all but one node and deduce the last by elimination:
1 + 1 + 2 + 2 + 2 = 8 mana, exact.

## Tool surface

| Action | Args | Legal when |
|---|---|---|
| `scry` | `{"node": 1}` | active ritual; node undisrupted and unrevealed; ≥1 mana |
| `cast_spell` | `{"spell": "fire"}` | active ritual; ≥2 mana |

`spell` is one of `fire`, `water`, `earth`. Accepting is the shared
`accept_quest`; there is no give-up action — a dead ritual resolves itself.
