package engine

func groundUnder(x, feet float64, platforms []Platform) bool {
	for _, plat := range platforms {
		if x < plat.X+1 || x > plat.X+plat.W-1 {
			continue
		}
		if feet >= plat.Y-5 && feet <= plat.Y+12 {
			return true
		}
	}
	return false
}

func ledgeJump(p Player, platforms []Platform) bool {
	if !p.Grounded {
		return false
	}
	dir := 1.0
	if p.Facing < 0 {
		dir = -1
	}
	probe := p.X + p.W/2 + dir*28
	return !groundUnder(probe, p.Y+p.H, platforms)
}

func nearestCoinX(run *RunState) (x float64, ok bool) {
	best := 1e12
	found := false
	px := run.Player.X
	for _, c := range run.Coins {
		if c.Taken {
			continue
		}
		d := c.X - px
		if d < 0 {
			d = -d
		}
		if d < best {
			best = d
			x = c.X
			found = true
		}
	}
	return x, found
}

func botInput(s *Sim) Input {
	in := ZeroInput
	switch s.Mode {
	case ModeTitle, ModeResult:
		in.Confirm = true
		in.Interact = true
		return in
	case ModeBattle:
		in.Confirm = true
		return in
	case ModeRun:
	default:
		return in
	}
	if s.Run == nil {
		return in
	}
	level := GetLevel(s.Run.LevelID)
	p := s.Run.Player
	in.MoveX = 1

	if cx, ok := nearestCoinX(s.Run); ok {
		if p.X+p.W < cx {
			in.MoveX = 1
		} else if p.X > cx+14 {
			in.MoveX = -1
		} else {
			in.MoveX = 1
		}
	} else {
		for i := range level.Zones {
			z := &level.Zones[i]
			if z.Kind != ZoneDropoff {
				continue
			}
			if p.X+p.W < z.X {
				in.MoveX = 1
			} else if p.X > z.X+z.W {
				in.MoveX = -1
			}
			if AABB(Rect{p.X, p.Y, p.W, p.H}, z.Rect) {
				in.Interact = true
				in.MoveX = 0
			}
		}
	}

	if ledgeJump(p, level.Platforms) && !in.Interact {
		in.JumpPressed = true
		in.JumpHeld = true
	}
	if p.VY < 0 {
		in.JumpHeld = true
	}
	return in
}

func DriveBotTick(s *Sim) {
	if s.Mode == ModeBoard {
		for _, q := range s.Board {
			if q.Status == QuestOpen {
				AcceptQuest(s, q.ID)
				return
			}
		}
		return
	}
	StepSim(s, botInput(s))
	if s.Mode == ModeResult {
		DismissResult(s)
	}
}
