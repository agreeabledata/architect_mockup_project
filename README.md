# KRONE (pure Go)

Rooftop courier platformer — full parity with the TypeScript/React build, rewritten in Go + Ebitengine.

- Lifespan ledger, seeded quest board, four courier runs
- Deterministic coin-powered battle (`power = max(1, floor(krone/10))`, never spent)
- Showdown-style battle HUD (gold bars, punch animation, DUEL / FIGHT!)
- Stick-figure courier, platforms, one-way planks, bot demo

## Requirements

- Go 1.22+
- CGO + OpenGL (macOS / Windows / Linux desktop)

## Run

```bash
cd krone-go
go mod tidy
go run ./cmd/krone
```

## Controls

| Key | Action |
|-----|--------|
| Enter / E / Space | Confirm, open board |
| B (title) | Watch a courier (bot) |
| W / S | Select job on board |
| A / D | Move |
| Space | Jump (hold for height) |
| S + Space | Drop through one-way plank |
| E | Pick up parcel / stamp dropoff |
| F / Space | Punch in battle |
| Enter | Resolve battle |
| P | Pause / resume |
| Q (paused) | Abandon run → board |
| Esc | Quit |

## Tests

```bash
go test ./internal/engine/...
go test ./fighter/... ./battle/...
```

## Layout

```text
cmd/krone/           Ebitengine client (title, board, run, battle, HUD)
internal/engine/     physics, levels, ledger, battle, sim, bot
fighter/             Stick-Figure-Showdown Fighter port
battle/              live duel helper
```
