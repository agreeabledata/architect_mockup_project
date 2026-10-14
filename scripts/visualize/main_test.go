package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"architect_mockup_project/internal/visual"
)

func TestViewerStepClampAndPlay(t *testing.T) {
	v := newViewer([]visual.Frame{{}, {}, {}})
	v.speed = 1

	v.prev()
	if v.idx != 0 {
		t.Fatalf("prev at 0 moved to %d", v.idx)
	}
	v.next()
	v.next()
	v.next()
	if v.idx != 2 {
		t.Fatalf("next past end = %d, want 2", v.idx)
	}

	v.toggle() // at last frame: restarts and plays
	if !v.playing || v.idx != 0 {
		t.Fatalf("toggle at end = playing %v idx %d, want true/0", v.playing, v.idx)
	}
	v.step()
	if v.idx != 1 {
		t.Fatalf("playing step = %d, want 1", v.idx)
	}
	v.toggle() // pause
	if v.playing {
		t.Fatal("second toggle should pause")
	}
	v.step()
	if v.idx != 1 {
		t.Fatalf("paused step moved to %d, want 1", v.idx)
	}
}

func TestViewerStopsAtEnd(t *testing.T) {
	v := newViewer([]visual.Frame{{}, {}})
	v.speed = 1
	v.toggle()
	v.step()
	v.step()
	if v.playing {
		t.Fatal("playing should stop at the last frame")
	}
	if v.idx != 1 {
		t.Fatalf("idx = %d, want 1", v.idx)
	}
	v.restart()
	if v.idx != 0 || v.playing {
		t.Fatalf("restart = idx %d playing %v", v.idx, v.playing)
	}
}

func TestWrapLimitsLines(t *testing.T) {
	lines := wrap(strings.Repeat("word ", 40), 20, 2)
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(lines))
	}
	if !strings.HasSuffix(lines[1], "…") {
		t.Fatalf("overflowed last line %q should end with an ellipsis", lines[1])
	}
	if got := wrap("short text", 20, 2); len(got) != 1 || got[0] != "short text" {
		t.Fatalf("short wrap = %q", got)
	}
}

func TestNewestTranscript(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	write := func(name string, mod time.Time) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(filepath.Join(dir, name), mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	write("run_a.jsonl", base)
	write("run_c.jsonl", base.Add(2*time.Hour))
	write("run_b.jsonl", base.Add(time.Hour))
	write("notes.txt", base.Add(10*time.Hour)) // wrong extension: ignored
	if err := os.Mkdir(filepath.Join(dir, "sub.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := newestTranscript(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "run_c.jsonl"); got != want {
		t.Fatalf("newest = %q, want %q", got, want)
	}

	if _, err := newestTranscript(t.TempDir()); err == nil {
		t.Fatal("empty dir should error")
	}
}

func TestTokenTarget(t *testing.T) {
	board := []visual.QuestView{{ID: "q0"}, {ID: "q1"}, {ID: "q2", Active: true}, {ID: "q3"}, {ID: "q4"}}
	x, y := tokenTarget(visual.Frame{Board: board})
	if wantX := boardX + 2*(cardW+cardGap) + cardW/2; x != wantX {
		t.Fatalf("active-quest x = %v, want %v", x, wantX)
	}
	if wantY := boardY + cardH - tokenInsetY; y != wantY {
		t.Fatalf("active-quest y = %v, want %v", y, wantY)
	}

	open := []visual.QuestView{{ID: "q0"}, {ID: "q1"}, {ID: "q2"}}
	x, _ = tokenTarget(visual.Frame{Board: open, QuestID: "q1"})
	if wantX := boardX + 1*(cardW+cardGap) + cardW/2; x != wantX {
		t.Fatalf("quest_id x = %v, want %v", x, wantX)
	}

	for _, f := range []visual.Frame{{Board: open}, {}} {
		x, y = tokenTarget(f)
		if x != boardX-20 || y != boardY+cardH/2 {
			t.Fatalf("home = (%v,%v), want (%v,%v)", x, y, boardX-20, boardY+cardH/2)
		}
	}
}

func TestAnimatorGlide(t *testing.T) {
	var a animator
	a.frame = -1
	a.retarget(100, 200, 0)
	if !a.init || a.x != 100 || a.y != 200 {
		t.Fatalf("first target should appear immediately: %+v", a)
	}

	a.retarget(300, 200, 1)
	if a.x != 100 || a.y != 200 || a.t != 0 {
		t.Fatalf("glide should start from the current position: %+v", a)
	}
	a.advance()
	if a.x <= 100 || a.x >= 300 {
		t.Fatalf("mid-glide x = %v, want between 100 and 300", a.x)
	}
	for i := 0; i < 100; i++ {
		a.advance()
	}
	if a.x != 300 || a.t != 1 {
		t.Fatalf("glide did not settle: x=%v t=%v", a.x, a.t)
	}

	a.retarget(999, 999, 1) // same frame: no-op
	if a.toX != 300 {
		t.Fatalf("retarget on same frame changed target to %v", a.toX)
	}
}

func TestLerp(t *testing.T) {
	if lerp(0, 10, 0) != 0 || lerp(0, 10, 1) != 10 || lerp(0, 10, 0.5) != 5 {
		t.Fatal("lerp endpoints/midpoint wrong")
	}
}
