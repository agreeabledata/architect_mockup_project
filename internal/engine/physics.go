package engine

const Tick = 1.0 / 60.0

const (
	gravityUp      = 1650.0
	gravityDown    = 3400.0
	gravityApex    = 880.0
	apexThreshold  = 55.0
	jumpVel        = -575.0
	jumpCut        = 0.5
	maxFall        = 980.0
	runSpeed       = 245.0
	accelGround    = 2400.0
	accelAir       = 1200.0
	friction       = 2600.0
	coyoteTime     = 0.1
	jumpBuffer     = 0.14
	playerW        = 12.0
	playerH        = 26.0
	dropThroughSec = 0.18
)

func MakePlayer(x, y float64) Player {
	return Player{
		X: x, Y: y, W: playerW, H: playerH, Facing: 1,
		PrevBottom: y + playerH,
	}
}

func AABB(a, b Rect) bool {
	return a.X < b.X+b.W && a.X+a.W > b.X && a.Y < b.Y+b.H && a.Y+a.H > b.Y
}

func overlaps(ax, ay, aw, ah float64, b Platform) bool {
	return ax < b.X+b.W && ax+aw > b.X && ay < b.Y+b.H && ay+ah > b.Y
}

func supports(p *Player, plat Platform) bool {
	if p.DropThrough > 0 && plat.Kind == PlatformOneway {
		return false
	}
	if plat.Kind == PlatformOneway && p.VY < -20 {
		return false
	}
	feet := p.Y + p.H
	onX := p.X+2 < plat.X+plat.W && p.X+p.W-2 > plat.X
	return onX && feet >= plat.Y-2 && feet <= plat.Y+6 && p.Y < plat.Y
}

func SnapToGround(p *Player, platforms []Platform) {
	p.Grounded = false
	for _, plat := range platforms {
		if !supports(p, plat) {
			continue
		}
		p.Y = plat.Y - p.H
		p.VY = 0
		p.Grounded = true
		p.Coyote = coyoteTime
		return
	}
}

func StepPlayer(p *Player, in Input, platforms []Platform, dt float64) {
	p.PrevBottom = p.Y + p.H
	if in.MoveX > 0.15 {
		p.Facing = 1
	} else if in.MoveX < -0.15 {
		p.Facing = -1
	}

	accel := accelAir
	if p.Grounded {
		accel = accelGround
	}
	target := in.MoveX * runSpeed
	if abs(target) > 8 {
		if p.VX < target {
			p.VX = min(target, p.VX+accel*dt)
		} else if p.VX > target {
			p.VX = max(target, p.VX-accel*dt)
		}
	} else if p.Grounded {
		fr := friction * dt
		if abs(p.VX) <= fr {
			p.VX = 0
		} else {
			p.VX -= sign(p.VX) * fr
		}
	} else {
		p.VX *= 0.997
	}
	p.VX = clamp(p.VX, -runSpeed, runSpeed)

	if p.Grounded {
		p.Coyote = coyoteTime
	} else {
		p.Coyote = max(0, p.Coyote-dt)
	}

	if in.JumpPressed {
		p.JumpBuf = jumpBuffer
	} else {
		p.JumpBuf = max(0, p.JumpBuf-dt)
	}

	p.DropThrough = max(0, p.DropThrough-dt)
	if in.Down && in.JumpPressed && p.Grounded {
		p.DropThrough = dropThroughSec
		p.Coyote = 0
		p.JumpBuf = 0
		p.Grounded = false
	}

	canJump := (p.Grounded || p.Coyote > 0) && p.DropThrough <= 0
	if p.JumpBuf > 0 && canJump {
		p.VY = jumpVel
		p.Grounded = false
		p.Coyote = 0
		p.JumpBuf = 0
		p.JumpCutDone = false
	} else if !in.JumpHeld && p.VY < 0 && !p.JumpCutDone {
		p.VY *= jumpCut
		p.JumpCutDone = true
	}

	g := gravityDown
	if p.VY < 0 {
		g = gravityUp
	}
	if abs(p.VY) < apexThreshold {
		g = gravityApex
	}
	p.VY += g * dt
	if p.VY > maxFall {
		p.VY = maxFall
	}

	p.X += p.VX * dt
	resolveAxis(p, platforms, true)
	p.Y += p.VY * dt
	p.Grounded = false
	resolveAxis(p, platforms, false)

	if !p.Grounded && p.VY >= 0 {
		for _, plat := range platforms {
			if !supports(p, plat) {
				continue
			}
			p.Y = plat.Y - p.H
			p.VY = 0
			p.Grounded = true
			break
		}
	}
}

func resolveAxis(p *Player, platforms []Platform, axisX bool) {
	for _, plat := range platforms {
		if plat.Kind == PlatformOneway {
			if axisX {
				continue
			}
			if p.DropThrough > 0 || p.VY < 0 || p.PrevBottom > plat.Y+4 {
				continue
			}
		}
		if !overlaps(p.X, p.Y, p.W, p.H, plat) {
			continue
		}
		if axisX {
			if p.VX > 0 {
				p.X = plat.X - p.W
			} else if p.VX < 0 {
				p.X = plat.X + plat.W
			} else {
				cx := p.X + p.W/2
				pcx := plat.X + plat.W/2
				if cx < pcx {
					p.X = plat.X - p.W
				} else {
					p.X = plat.X + plat.W
				}
			}
			p.VX = 0
		} else if p.VY >= 0 {
			p.Y = plat.Y - p.H
			p.VY = 0
			p.Grounded = true
		} else {
			p.Y = plat.Y + plat.H
			p.VY = 0
		}
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	if v > 0 {
		return 1
	}
	return 0
}
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
