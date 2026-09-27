// Command visualize opens a finished run's transcript in an Ebiten window and
// steps through it frame by frame. It shares the same harness.LoadTranscript
// parser and internal/visual model as scripts/replay_visual; the engine stays
// headless and is only replayed to recover the quest board each turn.
//
// With no path it opens the newest .jsonl in transcripts/.
//
//	go run ./scripts/visualize transcripts/run_….jsonl
//	go run ./scripts/visualize   # newest in transcripts/
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"architect_mockup_project/harness"
	"architect_mockup_project/internal/visual"
)

const (
	screenW = 960
	screenH = 560

	// Board card layout, shared by the board drawing and the token target math.
	boardX      = 40.0
	boardY      = 330.0
	cardW       = 168.0
	cardH       = 150.0
	cardGap     = 10.0
	tokenR      = 9.0
	tokenInsetY = 18.0 // token center above a card's bottom edge
)

var (
	colBG     = color.RGBA{0x12, 0x14, 0x1c, 0xff}
	colPanel  = color.RGBA{0x1a, 0x1d, 0x28, 0xff}
	colTrack  = color.RGBA{0x23, 0x26, 0x35, 0xff}
	colInk    = color.RGBA{0xe6, 0xe8, 0xef, 0xff}
	colMuted  = color.RGBA{0x8b, 0x90, 0xa0, 0xff}
	colGold   = color.RGBA{0xff, 0xd1, 0x66, 0xff}
	colGreen  = color.RGBA{0x5b, 0xd6, 0x7e, 0xff}
	colRed    = color.RGBA{0xe0, 0x5c, 0x5c, 0xff}
	colBlue   = color.RGBA{0x3b, 0x6e, 0xa5, 0xff}
	colPurple = color.RGBA{0x9b, 0x6e, 0xd6, 0xff}
	colCyan   = color.RGBA{0x4f, 0xc3, 0xc9, 0xff}
)

func main() {
	start := flag.Int("frame", 0, "turn index to show first (0-based)")
	shot := flag.String("shot", "", "render one frame to this PNG and exit")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: visualize [-frame n] [-shot out.png] [transcript.jsonl]")
	}
	flag.Parse()
	if flag.NArg() > 1 {
		flag.Usage()
		os.Exit(2)
	}

	in := flag.Arg(0)
	if in == "" {
		var err error
		in, err = newestTranscript("transcripts")
		if err != nil {
			fail("%v", err)
		}
		fmt.Fprintln(os.Stderr, "visualize: "+in)
	}
	f, err := os.Open(in)
	if err != nil {
		fail("%v", err)
	}
	log, loadErr := harness.LoadTranscript(f)
	f.Close()
	if loadErr != nil {
		fmt.Fprintf(os.Stderr, "visualize: %v\n", loadErr)
	}
	sc := visual.BuildScene(log)
	if len(sc.Frames) == 0 {
		fail("no turn records in %s", in)
	}

	g := newGame(sc)
	if *start > 0 && *start < len(sc.Frames) {
		g.v.idx = *start
	}
	g.shot = *shot

	ebiten.SetWindowSize(screenW, screenH)
	ebiten.SetWindowTitle("replay · " + sc.Model)
	if err := ebiten.RunGame(g); err != nil {
		fail("%v", err)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

// newestTranscript returns the most recently modified .jsonl in dir. Transcript
// filenames embed a UTC timestamp, so the name tiebreak keeps ties stable.
func newestTranscript(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("no transcripts in %s (run ./scripts/run_bot or pass a path): %w", dir, err)
	}
	best := ""
	var bestTime time.Time
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if best == "" || info.ModTime().After(bestTime) ||
			(info.ModTime().Equal(bestTime) && e.Name() > best) {
			best = e.Name()
			bestTime = info.ModTime()
		}
	}
	if best == "" {
		return "", fmt.Errorf("no .jsonl transcripts in %s (run ./scripts/run_bot or pass a path)", dir)
	}
	return filepath.Join(dir, best), nil
}

// viewer is the frame/playback state machine, kept free of Ebiten so it can be
// tested without opening a window.
type viewer struct {
	frames  []visual.Frame
	idx     int
	playing bool
	speed   int // Update ticks between automatic advances
	wait    int
}

func newViewer(frames []visual.Frame) *viewer {
	return &viewer{frames: frames, speed: 36}
}

func (v *viewer) next() {
	if v.idx < len(v.frames)-1 {
		v.idx++
	}
}

func (v *viewer) prev() {
	if v.idx > 0 {
		v.idx--
	}
}

func (v *viewer) toggle() {
	if len(v.frames) == 0 {
		return
	}
	if !v.playing && v.idx == len(v.frames)-1 {
		v.idx = 0
	}
	v.playing = !v.playing
	v.wait = 0
}

func (v *viewer) restart() {
	v.idx = 0
	v.playing = false
	v.wait = 0
}

func (v *viewer) step() {
	if !v.playing || len(v.frames) == 0 {
		return
	}
	v.wait++
	if v.wait < v.speed {
		return
	}
	v.wait = 0
	if v.idx >= len(v.frames)-1 {
		v.playing = false
		return
	}
	v.idx++
}

// questIndex is the board slot the turn points at: the active quest, else the
// card named by the turn's quest_id arg, else -1 (home).
func questIndex(f visual.Frame) int {
	for i, q := range f.Board {
		if q.Active {
			return i
		}
	}
	if f.QuestID != "" {
		for i, q := range f.Board {
			if q.ID == f.QuestID {
				return i
			}
		}
	}
	return -1
}

// tokenTarget is the center of the cosmetic agent token for a frame: on the
// card it points at, or in the left gutter when the turn names no card.
func tokenTarget(f visual.Frame) (float64, float64) {
	if i := questIndex(f); i >= 0 {
		return boardX + float64(i)*(cardW+cardGap) + cardW/2, boardY + cardH - tokenInsetY
	}
	return boardX - 20, boardY + cardH/2
}

// animator glides the cosmetic token between targets. Pure state math, so it
// is testable without Ebiten.
type animator struct {
	x, y         float64 // current rendered position
	fromX, fromY float64
	toX, toY     float64
	t            float64 // 0..1 glide progress
	frame        int     // frame the current target belongs to (-1 = none)
	init         bool
}

const animStep = 1.0 / 30.0 // ~0.5s per glide at 60 TPS

func (a *animator) retarget(tx, ty float64, frame int) {
	if a.init && a.frame == frame {
		return
	}
	if a.init {
		a.fromX, a.fromY = a.x, a.y
	} else {
		a.fromX, a.fromY = tx, ty
	}
	a.toX, a.toY = tx, ty
	a.t = 0
	a.frame = frame
	a.init = true
	a.x, a.y = a.fromX, a.fromY
}

func (a *animator) advance() {
	if !a.init || a.t >= 1 {
		return
	}
	a.t += animStep
	if a.t > 1 {
		a.t = 1
	}
	a.x = lerp(a.fromX, a.toX, a.t)
	a.y = lerp(a.fromY, a.toY, a.t)
}

func (a *animator) snap() {
	if a.init {
		a.x, a.y, a.t = a.toX, a.toY, 1
	}
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

type game struct {
	sc    visual.Scene
	v     *viewer
	a     animator
	tick  int
	shot  string // if set, render one frame to this path and quit
	saved bool
}

func newGame(sc visual.Scene) *game {
	g := &game{sc: sc, v: newViewer(sc.Frames)}
	g.a.frame = -1
	return g
}

func (g *game) Update() error {
	if g.saved {
		return ebiten.Termination
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) ||
		inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) ||
		inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		g.v.playing = false
		g.v.next()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) || inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		g.v.playing = false
		g.v.prev()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		g.v.toggle()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		g.v.restart()
	}
	g.v.step()
	g.tick++

	if len(g.sc.Frames) > 0 {
		tx, ty := tokenTarget(g.sc.Frames[g.v.idx])
		g.a.retarget(tx, ty, g.v.idx)
		if g.shot != "" {
			g.a.snap()
		} else {
			g.a.advance()
		}
	}
	return nil
}

func (g *game) Layout(int, int) (int, int) {
	return screenW, screenH
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(colBG)
	g.drawHeader(screen)

	if len(g.sc.Frames) == 0 {
		ebitenutil.DebugPrintAt(screen, "no turn records in this transcript", 40, 140)
		return
	}
	f := g.sc.Frames[g.v.idx]

	g.drawBars(screen, f)
	g.drawAction(screen, f)
	g.drawBoard(screen, f)
	g.drawAgent(screen, f)
	g.drawTurnLog(screen)
	g.drawControls(screen)
	ebitenutil.DebugPrintAt(screen,
		"agent token is cosmetic; the transcript records decisions, not positions", 40, 481)

	if g.shot != "" && !g.saved {
		g.writeShot(screen)
		g.saved = true
	}
}

// writeShot reads the rendered framebuffer straight from Ebiten and writes it
// as a PNG. Unlike an X/Wayland screenshot it is independent of the compositor,
// so it also works headless in CI.
func (g *game) writeShot(screen *ebiten.Image) {
	b := screen.Bounds()
	w, h := b.Dx(), b.Dy()
	buf := make([]byte, 4*w*h)
	screen.ReadPixels(buf)
	img := &image.RGBA{Pix: buf, Stride: 4 * w, Rect: image.Rect(0, 0, w, h)}
	f, err := os.Create(g.shot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "visualize: %v\n", err)
		return
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		fmt.Fprintf(os.Stderr, "visualize: %v\n", err)
	}
}

func (g *game) drawHeader(screen *ebiten.Image) {
	ebitenutil.DebugPrintAt(screen, g.sc.Model+"  ·  "+g.sc.Provider, 40, 28)
	line := fmt.Sprintf("engine seed %d   ·   final %d krones   ·   earned %d   ·   turn %d/%d",
		g.sc.EngineSeed, g.sc.FinalBal, g.sc.TotalEarned, g.v.idx+1, len(g.sc.Frames))
	ebitenutil.DebugPrintAt(screen, line, 40, 50)
}

func (g *game) drawBars(screen *ebiten.Image, f visual.Frame) {
	tx, tw, ty := float32(40), float32(screenW-80), float32(92)

	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("lifespan   day %d / %d", f.Day, g.sc.MaxDay), 40, int(ty)-18)
	vector.FillRect(screen, tx, ty, tw, 12, colTrack, false)
	prog := float32(f.Day) / float32(g.sc.MaxDay)
	if prog > 1 {
		prog = 1
	}
	vector.FillRect(screen, tx, ty, tw*prog, 12, colBlue, false)
	px := tx + tw*prog
	vector.FillRect(screen, px-7, ty-7, 14, 26, colGold, false)

	by := float32(142)
	maxBal := 1
	for _, fr := range g.sc.Frames {
		if fr.Balance > maxBal {
			maxBal = fr.Balance
		}
	}
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("balance   %d krones", f.Balance), 40, int(by)-18)
	vector.FillRect(screen, tx, by, tw, 16, colTrack, false)
	bar := colGreen
	if f.Error != "" {
		bar = colRed
	}
	vector.FillRect(screen, tx, by, tw*float32(f.Balance)/float32(maxBal), 16, bar, false)
}

func (g *game) drawAction(screen *ebiten.Image, f visual.Frame) {
	const x, y, w, h = float32(40), float32(190), float32(screenW - 80), float32(120)
	vector.FillRect(screen, x, y, w, h, colPanel, false)
	border := actionColor(f)
	vector.StrokeRect(screen, x, y, w, h, 2, border, false)

	label := f.Action
	if f.QuestID != "" {
		label += " " + f.QuestID
	}
	ebitenutil.DebugPrintAt(screen, label, 56, 206)

	if f.Error != "" {
		ebitenutil.DebugPrintAt(screen, "invalid: "+f.Error, 56, 230)
	} else {
		ebitenutil.DebugPrintAt(screen,
			fmt.Sprintf("+%d krones   ·   %d day(s)", f.Earned, f.DaysSpent), 56, 230)
	}

	reasoning := f.Reasoning
	if reasoning == "" {
		reasoning = "(no reasoning recorded)"
	}
	for i, line := range wrap(reasoning, 130, 2) {
		ebitenutil.DebugPrintAt(screen, line, 56, 256+i*16)
	}
}

func (g *game) drawBoard(screen *ebiten.Image, f visual.Frame) {
	const x0, y, cw, ch, gap = float32(boardX), float32(boardY), float32(cardW), float32(cardH), float32(cardGap)
	ebitenutil.DebugPrintAt(screen, "quest board (as the agent saw it)", 40, int(y)-18)
	if f.Board == nil {
		ebitenutil.DebugPrintAt(screen, "board unavailable (no engine_seed in header)", 40, int(y)+16)
		return
	}
	for i, q := range f.Board {
		x := x0 + float32(i)*(cw+gap)
		fill := questFill(q.Kind)
		if q.Status == "complete" {
			fill = colPanel
		}
		vector.FillRect(screen, x, y, cw, ch, fill, false)
		if q.Active {
			vector.StrokeRect(screen, x, y, cw, ch, 2, colGold, false)
		}
		txt := 0
		put := func(s string) {
			ebitenutil.DebugPrintAt(screen, s, int(x)+8, int(y)+8+txt*16)
			txt++
		}
		put(q.Kind)
		for _, line := range wrap(q.Title, 24, 2) {
			put(line)
		}
		put(fmt.Sprintf("%d days / %d kr", q.Days, q.Payout))
		put(q.Status)
		if q.Kind == "arcane" {
			put(fmt.Sprintf("mana %d  node %d", q.Mana, q.NextNode))
			put("seen " + join(q.Revealed))
		}
	}
}

// drawAgent draws the cosmetic token and its target ring. Animation only: the
// transcript carries decisions, not positions.
func (g *game) drawAgent(screen *ebiten.Image, f visual.Frame) {
	if f.Board == nil {
		return
	}
	accent := actionColor(f)

	vector.StrokeCircle(screen, float32(g.a.toX), float32(g.a.toY),
		float32(14+12*g.a.t), 2, accent, true)

	y := g.a.y
	if g.v.playing {
		y += math.Sin(float64(g.tick)/18) * 2
	}
	vector.DrawFilledCircle(screen, float32(g.a.x), float32(y), tokenR, colInk, true)
	vector.StrokeCircle(screen, float32(g.a.x), float32(y), tokenR, 2, accent, true)
}

func (g *game) drawTurnLog(screen *ebiten.Image) {
	ebitenutil.DebugPrintAt(screen, "turn log", 40, 498)
	start := g.v.idx - 2
	if start < 0 {
		start = 0
	}
	for i, r := 0, 0; start+i <= g.v.idx; i++ {
		t := g.sc.Frames[start+i]
		line := fmt.Sprintf("day %d   bal %d   %s", t.Day, t.Balance, t.Action)
		if t.QuestID != "" {
			line += " " + t.QuestID
		}
		if t.Error != "" {
			line += "   invalid"
		} else {
			line += fmt.Sprintf("   +%d", t.Earned)
		}
		ebitenutil.DebugPrintAt(screen, line, 40, 518+r*16)
		r++
	}
}

func (g *game) drawControls(screen *ebiten.Image) {
	state := "paused"
	if g.v.playing {
		state = "playing"
	}
	ebitenutil.DebugPrintAt(screen,
		"[←/→] step  [P] play/pause  [R] restart  [esc] quit   ("+state+")", 460, 498)
}

func actionColor(f visual.Frame) color.Color {
	if f.Error != "" {
		return colRed
	}
	switch f.Action {
	case "fight":
		if f.Earned > 0 {
			return colGreen
		}
		return colRed
	case "scry":
		return colCyan
	case "cast_spell":
		return colPurple
	default:
		return colGreen
	}
}

func questFill(kind string) color.Color {
	switch kind {
	case "battle":
		return color.RGBA{0x4a, 0x22, 0x24, 0xff}
	case "arcane":
		return color.RGBA{0x2e, 0x24, 0x46, 0xff}
	default:
		return color.RGBA{0x1f, 0x2c, 0x3a, 0xff}
	}
}

func join(revealed []string) string {
	out := "—"
	for i, r := range revealed {
		if r == "" {
			continue
		}
		if out == "—" {
			out = ""
		} else {
			out += ","
		}
		out += fmt.Sprintf("%d:%s", i+1, r)
	}
	return out
}

// wrap breaks text into at most maxLines monospace lines of <= maxChars runes.
// The built-in bitmap font is fixed-width, so rune count is a good enough
// measure; the last line gets an ellipsis if it overflows.
func wrap(s string, maxChars, maxLines int) []string {
	words := splitWords(s)
	var lines []string
	line := ""
	for _, w := range words {
		if line != "" && len(line)+1+len(w) > maxChars {
			lines = append(lines, line)
			line = ""
		}
		if line == "" {
			line = w
		} else {
			line += " " + w
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines[maxLines-1] += "…"
	}
	return lines
}

func splitWords(s string) []string {
	var out []string
	word := ""
	for _, r := range s {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			if word != "" {
				out = append(out, word)
				word = ""
			}
			continue
		}
		word += string(r)
	}
	if word != "" {
		out = append(out, word)
	}
	return out
}
