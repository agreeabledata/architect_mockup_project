// KRONE — pure Go desktop client (Ebitengine).
// Parity with the TypeScript/React build: title, board, run, Showdown battle HUD, results, bot.
package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/mfeinLearn/krone/internal/engine"
)

const (
	screenW = 960
	screenH = 540
	viewW   = 420.0
	viewH   = 236.0
)

var (
	colInk       = color.RGBA{0x0c, 0x0b, 0x0a, 0xff}
	colElev      = color.RGBA{0x14, 0x13, 0x11, 0xff}
	colParchment = color.RGBA{0xef, 0xe8, 0xdc, 0xff}
	colRust      = color.RGBA{0x8f, 0x3d, 0x36, 0xff}
	colRed       = color.RGBA{0xc0, 0x39, 0x2b, 0xff}
	colStone     = color.RGBA{0xb8, 0xbc, 0xc4, 0xff}
	colGold      = color.RGBA{0xc9, 0xa2, 0x27, 0xff}
	colMuted     = color.RGBA{0x7a, 0x76, 0x70, 0xff}
	colRoof      = color.RGBA{0x1c, 0x1b, 0x19, 0xff}
	colRoofTop   = color.RGBA{0x3a, 0x39, 0x36, 0xff}
	colSage      = color.RGBA{0x6f, 0x8a, 0x78, 0xff}
	colSkyTop    = color.RGBA{0x12, 0x11, 0x10, 0xff}
)

type punchPhase int

const (
	punchIdle punchPhase = iota
	punchAgent
	punchFoe
)

type Game struct {
	sim         *engine.Sim
	camX, camY  float64
	boardI      int
	time        float64
	banner      string // "", "DUEL", "FIGHT!"
	bannerUntil float64
	phase       punchPhase
	phaseUntil  float64
	shakeUntil  float64
	punching    bool
	battleKey   string
}

func NewGame() *Game {
	return &Game{sim: engine.CreateSim(uint32(time.Now().UnixNano()), engine.DriverPlayer)}
}

func (g *Game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	g.time += 1.0 / 60.0
	s := g.sim

	// bot demo: hold B on title
	if s.Mode == engine.ModeTitle && inpututil.IsKeyJustPressed(ebiten.KeyB) {
		s = engine.CreateSim(1, engine.DriverBot)
		engine.GoToBoard(s)
		g.sim = s
		return nil
	}

	if s.Driver == engine.DriverBot && s.Mode != engine.ModeEnded {
		engine.DriveBotTick(s)
		g.syncBattleFX()
		return nil
	}

	in := engine.ZeroInput
	if ebiten.IsKeyPressed(ebiten.KeyA) || ebiten.IsKeyPressed(ebiten.KeyLeft) {
		in.MoveX = -1
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) || ebiten.IsKeyPressed(ebiten.KeyRight) {
		in.MoveX = 1
	}
	in.JumpHeld = ebiten.IsKeyPressed(ebiten.KeySpace)
	in.JumpPressed = inpututil.IsKeyJustPressed(ebiten.KeySpace)
	in.Down = ebiten.IsKeyPressed(ebiten.KeyS) || ebiten.IsKeyPressed(ebiten.KeyDown)
	in.Interact = inpututil.IsKeyJustPressed(ebiten.KeyE)
	in.Confirm = inpututil.IsKeyJustPressed(ebiten.KeyEnter)
	in.Pause = inpututil.IsKeyJustPressed(ebiten.KeyP) || inpututil.IsKeyJustPressed(ebiten.KeyEscape)

	switch s.Mode {
	case engine.ModeTitle:
		if in.Confirm || in.Interact || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			engine.GoToBoard(s)
		}
	case engine.ModeBoard:
		n := len(s.Board)
		if n == 0 {
			break
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
			g.boardI = (g.boardI + 1) % n
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
			g.boardI = (g.boardI - 1 + n) % n
		}
		if in.Confirm || in.Interact {
			q := s.Board[g.boardI]
			if q.Status == engine.QuestOpen {
				engine.AcceptQuest(s, q.ID)
			}
		}
	case engine.ModeRun:
		if in.Pause {
			s.Mode = engine.ModePaused
			s.UIDirty = true
			break
		}
		engine.StepSim(s, in)
	case engine.ModePaused:
		if inpututil.IsKeyJustPressed(ebiten.KeyP) || in.Confirm {
			s.Mode = engine.ModeRun
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyQ) {
			s.Run = nil
			engine.GoToBoard(s)
		}
	case engine.ModeBattle:
		g.syncBattleFX()
		if g.punching {
			if g.time >= g.phaseUntil {
				if g.phase == punchAgent {
					engine.PlayerStrike(s)
					g.phase = punchFoe
					g.phaseUntil = g.time + 0.28
					g.shakeUntil = g.time + 0.18
				} else {
					g.phase = punchIdle
					g.punching = false
				}
			}
			break
		}
		// Fight must be finished with punches — no resolve/skip.
		if inpututil.IsKeyJustPressed(ebiten.KeyF) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			if s.Battle != nil && s.Battle.Result == engine.BattleOngoing {
				g.punching = true
				g.phase = punchAgent
				g.phaseUntil = g.time + 0.18
			}
		}
	case engine.ModeResult, engine.ModeEnded:
		if in.Confirm || in.Interact || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			if s.Mode == engine.ModeEnded {
				*g = *NewGame()
			} else {
				engine.DismissResult(s)
			}
		}
	}
	return nil
}

func (g *Game) syncBattleFX() {
	s := g.sim
	if s.Mode != engine.ModeBattle || s.Battle == nil {
		g.banner = ""
		g.battleKey = ""
		return
	}
	key := s.Battle.OpponentName
	if key != g.battleKey {
		g.battleKey = key
		g.banner = "DUEL"
		g.bannerUntil = g.time + 0.7
	}
	if g.banner == "DUEL" && g.time >= g.bannerUntil {
		g.banner = "FIGHT!"
		g.bannerUntil = g.time + 0.7
	} else if g.banner == "FIGHT!" && g.time >= g.bannerUntil {
		g.banner = ""
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenW, screenH
}

func main() {
	ebiten.SetWindowSize(screenW, screenH)
	ebiten.SetWindowTitle("KRONE")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := ebiten.RunGame(NewGame()); err != nil {
		log.Fatal(err)
	}
}

// --- drawing ---

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(colInk)
	s := g.sim
	shakeX := 0.0
	if g.time < g.shakeUntil {
		shakeX = math.Sin(g.time*80) * 4
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(shakeX, 0)
	// draw into world layer conceptually; for simplicity draw with offset applied in coords

	switch s.Mode {
	case engine.ModeTitle:
		g.drawSky(screen, 0)
		g.drawIdleRoofs(screen)
		g.drawTitleUI(screen)
	case engine.ModeBoard:
		g.drawSky(screen, 0)
		g.drawIdleRoofs(screen)
		g.drawBoardUI(screen)
	case engine.ModeRun, engine.ModePaused:
		g.drawRun(screen)
		g.drawRunHUD(screen)
		if s.Mode == engine.ModePaused {
			g.drawPauseUI(screen)
		}
	case engine.ModeBattle:
		g.drawBattle(screen)
	case engine.ModeResult, engine.ModeEnded:
		g.drawSky(screen, 0)
		g.drawResultUI(screen)
	}
	if s.Toast != "" {
		ebitenutil.DebugPrintAt(screen, s.Toast, 24, screenH-36)
	}
	_ = op
}

func (g *Game) drawSky(screen *ebiten.Image, camX float64) {
	vector.DrawFilledRect(screen, 0, 0, screenW, screenH, colSkyTop, false)
	// moon
	vector.DrawFilledCircle(screen, float32(screenW-120), 70, 22, colParchment, false)
	vector.DrawFilledCircle(screen, float32(screenW-112), 64, 17, colInk, false)
	// parallax blocks
	rng := engine.Mulberry32(g.sim.Seed + 99)
	shift := camX * 0.25
	for i := 0; i < 18; i++ {
		w := 40 + rng()*90
		h := 80 + rng()*160
		x := math.Mod(float64(i)*110-shift, float64(screenW)+160) - 80
		y := float64(screenH) - h + 40
		vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{0x16, 0x15, 0x13, 0xff}, false)
	}
}

func (g *Game) drawIdleRoofs(screen *ebiten.Image) {
	y := float32(screenH) * 0.62
	vector.DrawFilledRect(screen, -20, y+14, 160, 200, colRoof, false)
	vector.DrawFilledRect(screen, 150, y, 220, 200, colRoof, false)
	vector.DrawFilledRect(screen, 380, y-24, 180, 230, colRoof, false)
	vector.DrawFilledRect(screen, 580, y+10, 140, 200, colRoof, false)
	vector.DrawFilledRect(screen, 740, y-8, 260, 220, colRoof, false)
	vector.DrawFilledRect(screen, 150, y, 220, 4, colRoofTop, false)
	vector.DrawFilledRect(screen, 380, y-24, 180, 4, colRoofTop, false)
	bob := math.Sin(g.time*2.5) * 3
	drawStick(screen, 420, float64(y)+bob, 2.8, 1, g.time, false, colParchment, false, false)
}

func (g *Game) drawTitleUI(screen *ebiten.Image) {
	ebitenutil.DebugPrintAt(screen, "A COURIER LEDGER", 48, screenH-200)
	ebitenutil.DebugPrintAt(screen, "KRONE", 48, screenH-175)
	ebitenutil.DebugPrintAt(screen, "Deliver before the ledger closes.", 48, screenH-140)
	ebitenutil.DebugPrintAt(screen, "Combat power is read from krone — never spent. No luck.", 48, screenH-120)
	ebitenutil.DebugPrintAt(screen, "Enter — Open the board", 48, screenH-80)
	ebitenutil.DebugPrintAt(screen, "B — Watch a courier (bot)", 48, screenH-60)
	ebitenutil.DebugPrintAt(screen, "A/D move · Space jump · touch coins · E stamp door", 48, screenH-36)
}

func (g *Game) drawBoardUI(screen *ebiten.Image) {
	s := g.sim
	// dim overlay
	vector.DrawFilledRect(screen, 0, 0, screenW, screenH, color.RGBA{0, 0, 0, 180}, false)
	ebitenutil.DebugPrintAt(screen, "QUEST BOARD", 40, 24)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Days %d   Krone %d   Power %d   HP %d",
		s.Ledger.Days, s.Ledger.Krone, engine.AttackPower(s.Ledger.Krone), s.Ledger.HP), 40, 48)

	for i, q := range s.Board {
		y := 90 + i*90
		selected := i == g.boardI
		bg := colElev
		if selected {
			bg = color.RGBA{0x2a, 0x28, 0x24, 0xff}
		}
		vector.DrawFilledRect(screen, 36, float32(y), screenW-72, 80, bg, false)
		if selected {
			vector.StrokeRect(screen, 36, float32(y), screenW-72, 80, 2, colParchment, false)
		}
		status := string(q.Status)
		line1 := fmt.Sprintf("%s  ·  %s  ·  +%d krone  ·  %d day  [%s]", q.Title, q.District, q.RewardKrone, q.TravelDays, status)
		ebitenutil.DebugPrintAt(screen, line1, 52, y+12)
		ebitenutil.DebugPrintAt(screen, q.Blurb, 52, y+36)
		note := "No combat on this run."
		if q.HasFoe {
			f := engine.Forecast(s.Ledger.Krone, s.Ledger.HP, 100)
			if f.Result == engine.BattleWin {
				note = fmt.Sprintf("Forecast: WIN in %d strikes · power %d", f.Turns, f.Power)
			} else {
				need := engine.KroneToWin(s.Ledger.HP, 100)
				note = fmt.Sprintf("Forecast: LOSS (%s) · need %d krone · power %d", f.Reason, need, f.Power)
			}
		}
		ebitenutil.DebugPrintAt(screen, note, 52, y+56)
	}
	ebitenutil.DebugPrintAt(screen, "W/S select · Enter take job", 40, screenH-28)
}

func (g *Game) drawRun(screen *ebiten.Image) {
	s := g.sim
	run := s.Run
	if run == nil {
		return
	}
	level := engine.GetLevel(run.LevelID)
	p := run.Player
	targetX := p.X + p.W/2 - viewW*0.38
	targetY := p.Y - viewH*0.55
	g.camX += (targetX - g.camX) * 0.15
	g.camY += (targetY - g.camY) * 0.12
	if g.camX < 0 {
		g.camX = 0
	}

	scale := float64(screenW) / viewW
	if float64(screenH)/viewH < scale {
		scale = float64(screenH) / viewH
	}
	ox := (float64(screenW) - viewW*scale) / 2
	oy := (float64(screenH) - viewH*scale) / 2
	wx := func(x float64) float64 { return ox + (x-g.camX)*scale }
	wy := func(y float64) float64 { return oy + (y-g.camY)*scale }

	g.drawSky(screen, g.camX)

	// void
	vector.DrawFilledRect(screen, 0, float32(wy(level.DeathY)), screenW, screenH, color.RGBA{0x0a, 0x09, 0x08, 0xff}, false)

	for _, plat := range level.Platforms {
		x, y := wx(plat.X), wy(plat.Y)
		if plat.Kind == engine.PlatformOneway {
			vector.DrawFilledRect(screen, float32(x), float32(y), float32(plat.W*scale), float32(4*scale), color.RGBA{0x4a, 0x48, 0x43, 0xff}, false)
		} else {
			vector.DrawFilledRect(screen, float32(x), float32(y), float32(plat.W*scale), float32((plat.H+40)*scale), colRoof, false)
			vector.DrawFilledRect(screen, float32(x), float32(y), float32(plat.W*scale), float32(3*scale), colRoofTop, false)
		}
	}
	for _, z := range level.Zones {
		x, y := wx(z.X), wy(z.Y)
		if z.Kind == engine.ZoneDropoff {
			vector.StrokeRect(screen, float32(x), float32(y), float32(z.W*scale), float32(z.H*scale), 2, colSage, false)
		}
	}
	// Mario-style coins
	for _, c := range run.Coins {
		if c.Taken {
			continue
		}
		cx := wx(c.X + 7)
		cy := wy(c.Y + 7)
		r := float32(6 * scale)
		bob := float32(math.Sin(g.time*6+c.X*0.05) * 2 * scale)
		vector.DrawFilledCircle(screen, float32(cx), float32(cy)+bob, r, colGold, false)
		vector.DrawFilledCircle(screen, float32(cx), float32(cy)+bob, r*0.45, color.RGBA{0xff, 0xe0, 0x80, 0xff}, false)
	}
	for _, enc := range run.Encounters {
		if enc.Cleared {
			continue
		}
		if !engine.FoeVisible(s.Ledger.Krone, s.Ledger.HP, enc) {
			continue
		}
		drawStick(screen, wx(enc.X), wy(enc.Y+enc.H), scale*1.1, -1, g.time, false, colRust, true, false)
	}
	drawStick(screen, wx(p.X), wy(p.Y+p.H), scale, float64(p.Facing), g.time,
		math.Abs(p.VX) > 20 && p.Grounded, colParchment, false, false)
}

func (g *Game) drawRunHUD(screen *ebiten.Image) {
	s := g.sim
	// chip bar like TS Hud
	chips := []string{
		fmt.Sprintf("DAYS %d", s.Ledger.Days),
		fmt.Sprintf("KRONE %d", s.Ledger.Krone),
		fmt.Sprintf("PWR %d", engine.AttackPower(s.Ledger.Krone)),
		fmt.Sprintf("HP %d", s.Ledger.HP),
	}
	x := float32(12)
	for _, c := range chips {
		vector.DrawFilledRect(screen, x, 10, 90, 36, colElev, false)
		vector.StrokeRect(screen, x, 10, 90, 36, 1, color.RGBA{0x2a, 0x29, 0x26, 0xff}, false)
		ebitenutil.DebugPrintAt(screen, c, int(x)+8, 20)
		x += 94
	}
	if s.Run != nil {
		level := engine.GetLevel(s.Run.LevelID)
		left := 0
		for _, c := range s.Run.Coins {
			if !c.Taken {
				left++
			}
		}
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s · coins left %d", level.District, left), screenW-240, 18)
	}
	ebitenutil.DebugPrintAt(screen, "A/D · Space · E stamp door · P pause", 12, screenH-24)
}

func (g *Game) drawPauseUI(screen *ebiten.Image) {
	vector.DrawFilledRect(screen, 0, 0, screenW, screenH, color.RGBA{0, 0, 0, 160}, false)
	ebitenutil.DebugPrintAt(screen, "PAUSED", screenW/2-40, screenH/2-40)
	ebitenutil.DebugPrintAt(screen, "P — resume   Q — abandon to board", screenW/2-120, screenH/2)
}

func (g *Game) drawResultUI(screen *ebiten.Image) {
	vector.DrawFilledRect(screen, 0, 0, screenW, screenH, color.RGBA{0, 0, 0, 180}, false)
	s := g.sim
	if s.Result != nil {
		ebitenutil.DebugPrintAt(screen, s.Result.Title, 80, screenH/2-40)
		ebitenutil.DebugPrintAt(screen, s.Result.Body, 80, screenH/2)
	}
	ebitenutil.DebugPrintAt(screen, "Enter — continue", 80, screenH/2+50)
}

func (g *Game) drawBattle(screen *ebiten.Image) {
	s := g.sim
	b := s.Battle
	if b == nil {
		return
	}
	vector.DrawFilledRect(screen, 0, 0, screenW, screenH, color.RGBA{0x0a, 0x09, 0x08, 0xff}, false)

	power := engine.AttackPower(b.Coins)
	strikesUsed := 10 - b.Budget
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("STRIKES: %d / 10", strikesUsed), 24, 16)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("POWER: %d", power), screenW-120, 16)

	// gold-framed HP bars (Showdown)
	maxHP := float64(s.Ledger.MaxHP)
	if maxHP < 1 {
		maxHP = 50
	}
	agentPct := float64(b.AgentHP) / maxHP
	if agentPct < 0 {
		agentPct = 0
	}
	foePct := float64(b.OpponentHP) / 100
	if foePct < 0 {
		foePct = 0
	}

	// left bar
	vector.DrawFilledRect(screen, 24, 48, 320, 36, colGold, false)
	vector.DrawFilledRect(screen, 28, 52, 312, 28, color.RGBA{0x1a, 0x12, 0x08, 0xff}, false)
	vector.DrawFilledRect(screen, 28, 52, float32(312*agentPct), 28, colRed, false)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("YOU  %d/%d", b.AgentHP, s.Ledger.MaxHP), 36, 58)

	// center budget
	budgetColor := colParchment
	if b.Budget <= 3 {
		budgetColor = colRed
	}
	_ = budgetColor
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%02d", b.Budget), screenW/2-12, 52)

	// right bar (fill from right)
	vector.DrawFilledRect(screen, float32(screenW-344), 48, 320, 36, colGold, false)
	vector.DrawFilledRect(screen, float32(screenW-340), 52, 312, 28, color.RGBA{0x1a, 0x12, 0x08, 0xff}, false)
	fw := 312 * foePct
	vector.DrawFilledRect(screen, float32(screenW-28)-float32(fw), 52, float32(fw), 28, colRed, false)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d/100  %s", b.OpponentHP, b.OpponentName), screenW-330, 58)

	// ring
	vector.StrokeRect(screen, float32(screenW)*0.15, float32(screenH)*0.55, float32(screenW)*0.7, 4, 1, color.RGBA{0xef, 0xe8, 0xdc, 40}, false)

	agentPhase := "idle"
	foePhase := "idle"
	if g.phase == punchAgent {
		agentPhase = "punch"
		foePhase = "hurt"
	} else if g.phase == punchFoe {
		agentPhase = "hurt"
		foePhase = "punch"
	}
	drawBattleStick(screen, screenW*0.28, screenH*0.72, 3.2, 1, agentPhase, colParchment, false)
	drawBattleStick(screen, screenW*0.72, screenH*0.72, 3.2, -1, foePhase, colStone, true)

	if g.banner != "" {
		ebitenutil.DebugPrintAt(screen, g.banner, screenW/2-40, screenH/3)
	} else if len(b.Log) > 0 {
		ebitenutil.DebugPrintAt(screen, b.Log[len(b.Log)-1].Note, 40, screenH*82/100)
	}

	if b.Result == engine.BattleOngoing {
		ebitenutil.DebugPrintAt(screen, "F / Space — PUNCH  (fight until one falls)", 40, screenH-40)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Power %d from krone · every punch lands · no luck", power), 40, screenH-22)
	} else if b.Result == engine.BattleWin {
		ebitenutil.DebugPrintAt(screen, "YOU WON!", screenW/2-40, screenH-50)
	} else {
		ebitenutil.DebugPrintAt(screen, "YOU LOST", screenW/2-40, screenH-50)
	}
}

func drawStick(screen *ebiten.Image, feetX, feetY, scale, facing, t float64, walking bool, c color.Color, foe, carrying bool) {
	swing := 0.15 * math.Sin(t)
	if walking {
		swing = math.Sin(t * 8)
	}
	dir := facing
	if dir == 0 {
		dir = 1
	}
	cx := float32(feetX + 6*scale)
	feet := float32(feetY)
	hip := feet - float32(14*scale)
	headY := hip - float32(11*scale)
	r := float32(4.2 * scale)
	if foe {
		r = float32(5 * scale)
	}
	lw := float32(2 * scale)
	vector.DrawFilledCircle(screen, cx, headY, r, c, false)
	vector.StrokeLine(screen, cx, headY+r, cx, hip, lw, c, false)
	vector.StrokeLine(screen, cx, headY+float32(6*scale), cx-float32(dir*(6+swing*3)*scale), headY+float32(14*scale), lw, c, false)
	if carrying {
		vector.StrokeLine(screen, cx, headY+float32(6*scale), cx+float32(dir*9*scale), headY+float32(4*scale), lw, c, false)
		vector.DrawFilledRect(screen, cx+float32(dir*8*scale)-float32(4*scale), headY-float32(2*scale), float32(16*scale), float32(12*scale), colParchment, false)
	} else {
		vector.StrokeLine(screen, cx, headY+float32(6*scale), cx+float32(dir*(6-swing*3)*scale), headY+float32(14*scale), lw, c, false)
	}
	vector.StrokeLine(screen, cx, hip, cx-float32(dir*(3+swing*5)*scale), feet, lw, c, false)
	vector.StrokeLine(screen, cx, hip, cx+float32(dir*(4-swing*5)*scale), feet, lw, c, false)
	if foe {
		vector.StrokeLine(screen, cx+float32(dir*6*scale), hip-float32(4*scale), cx+float32(dir*14*scale), hip-float32(10*scale), lw, c, false)
	}
}

func drawBattleStick(screen *ebiten.Image, feetX, feetY, scale, facing float64, phase string, c color.Color, foe bool) {
	lean := 0.0
	if phase == "hurt" {
		lean = -facing * 8 * scale
	} else if phase == "punch" {
		lean = facing * 6 * scale
	}
	cx := float32(feetX + lean)
	feet := float32(feetY)
	hip := feet - float32(40*scale)
	headY := hip - float32(32*scale)
	r := float32(12 * scale)
	lw := float32(3.2 * scale)
	vector.DrawFilledCircle(screen, cx, headY, r, c, false)
	vector.StrokeLine(screen, cx, headY+r, cx, hip, lw, c, false)
	// back arm
	vector.StrokeLine(screen, cx, headY+float32(16*scale), cx-float32(facing*18*scale), headY+float32(36*scale), lw, c, false)
	// punch arm
	if phase == "punch" {
		vector.StrokeLine(screen, cx, headY+float32(16*scale), cx+float32(facing*34*scale), headY+float32(8*scale), float32(4*scale), c, false)
	} else {
		vector.StrokeLine(screen, cx, headY+float32(16*scale), cx+float32(facing*16*scale), headY+float32(36*scale), lw, c, false)
	}
	hurtY := feet
	if phase == "hurt" {
		hurtY = feet - float32(4*scale)
	}
	vector.StrokeLine(screen, cx, hip, cx-float32(facing*12*scale), hurtY, lw, c, false)
	vector.StrokeLine(screen, cx, hip, cx+float32(facing*14*scale), feet, lw, c, false)
	if foe {
		vector.StrokeLine(screen, cx+float32(facing*14*scale), headY+float32(36*scale), cx+float32(facing*28*scale), headY+float32(18*scale), float32(2.4*scale), c, false)
	}
}
