// Package fighter is a Go port of Stick-Figure-Showdown's Fighter class.
// Pure logic — no rendering, no libGDX. Drive it from tests, bots, or a sim loop.
package fighter

// Movement and combat constants (match Showdown defaults).
const (
	MovementSpeed     = 10.0
	MaxLife           = 100.0
	HitStrength       = 5.0
	BlockDamageFactor = 0.2

	// Default frame times for each animation (seconds per frame × 6 frames).
	// Used only for attack-active windows and "animation finished" checks.
	blockFrameTime = 0.05
	hurtFrameTime  = 0.03
	idleFrameTime  = 0.10
	kickFrameTime  = 0.05
	loseFrameTime  = 0.05
	punchFrameTime = 0.05
	walkFrameTime  = 0.08
	winFrameTime   = 0.05
	frameCount     = 6 // 2 rows × 3 cols
)

// State is the fighter's logic state (also used for render intent).
type State int

const (
	StateIdle State = iota
	StateWalk
	StateBlock
	StatePunch
	StateKick
	StateHurt
	StateWin
	StateLose
)

func (s State) String() string {
	switch s {
	case StateIdle:
		return "IDLE"
	case StateWalk:
		return "WALK"
	case StateBlock:
		return "BLOCK"
	case StatePunch:
		return "PUNCH"
	case StateKick:
		return "KICK"
	case StateHurt:
		return "HURT"
	case StateWin:
		return "WIN"
	case StateLose:
		return "LOSE"
	default:
		return "?"
	}
}

// Vec2 is a simple 2D point / direction.
type Vec2 struct {
	X, Y float64
}

// Fighter is the combat actor: move, punch, kick, block, take hits.
type Fighter struct {
	Name  string
	Color string // hex or label; rendering is left to the caller

	state            State
	stateTime        float64
	renderState      State
	renderStateTime  float64
	position         Vec2
	movementDir      Vec2
	life             float64
	facing           int // 1 = right, -1 = left
	madeContact      bool
	animDur          map[State]float64
}

// New creates a fighter. Call GetReady before the first round.
func New(name, color string) *Fighter {
	f := &Fighter{
		Name:  name,
		Color: color,
		animDur: map[State]float64{
			StateBlock: blockFrameTime * frameCount,
			StateHurt:  hurtFrameTime * frameCount,
			StateIdle:  idleFrameTime * frameCount,
			StateKick:  kickFrameTime * frameCount,
			StateLose:  loseFrameTime * frameCount,
			StatePunch: punchFrameTime * frameCount,
			StateWalk:  walkFrameTime * frameCount,
			StateWin:   winFrameTime * frameCount,
		},
	}
	f.GetReady(0, 0)
	return f
}

func (f *Fighter) Position() Vec2 { return f.position }
func (f *Fighter) Life() float64  { return f.life }
func (f *Fighter) State() State   { return f.state }
func (f *Fighter) Facing() int    { return f.facing }

// RenderState is the state the UI should draw this frame.
func (f *Fighter) RenderState() State { return f.renderState }

// RenderStateTime is how long that render state has been active.
func (f *Fighter) RenderStateTime() float64 { return f.renderStateTime }

// GetReady resets life, state, and position for a new round.
func (f *Fighter) GetReady(x, y float64) {
	f.state = StateIdle
	f.renderState = StateIdle
	f.stateTime = 0
	f.renderStateTime = 0
	f.position = Vec2{X: x, Y: y}
	f.movementDir = Vec2{}
	f.life = MaxLife
	f.madeContact = false
}

// Update advances animation timers and movement. Pass 0 to freeze (paused).
func (f *Fighter) Update(dt float64) {
	f.stateTime += dt
	if dt > 0 {
		f.renderState = f.state
		f.renderStateTime = f.stateTime
	}

	switch f.state {
	case StateWalk:
		f.position.X += f.movementDir.X * MovementSpeed * dt
		f.position.Y += f.movementDir.Y * MovementSpeed * dt
	case StatePunch, StateKick, StateHurt:
		if f.animFinished(f.state) {
			if f.movementDir.X != 0 || f.movementDir.Y != 0 {
				f.changeState(StateWalk)
			} else {
				f.changeState(StateIdle)
			}
		}
	}
}

func (f *Fighter) animFinished(s State) bool {
	return f.stateTime >= f.animDur[s]
}

func (f *Fighter) FaceLeft()  { f.facing = -1 }
func (f *Fighter) FaceRight() { f.facing = 1 }

func (f *Fighter) changeState(s State) {
	f.state = s
	f.stateTime = 0
}

func (f *Fighter) setMovement(x, y float64) {
	f.movementDir = Vec2{X: x, Y: y}
	if f.state == StateWalk && x == 0 && y == 0 {
		f.changeState(StateIdle)
	} else if f.state == StateIdle && (x != 0 || y != 0) {
		f.changeState(StateWalk)
	}
}

func (f *Fighter) MoveLeft()  { f.setMovement(-1, f.movementDir.Y) }
func (f *Fighter) MoveRight() { f.setMovement(1, f.movementDir.Y) }
func (f *Fighter) MoveUp()    { f.setMovement(f.movementDir.X, 1) }
func (f *Fighter) MoveDown()  { f.setMovement(f.movementDir.X, -1) }

func (f *Fighter) StopMovingLeft() {
	if f.movementDir.X == -1 {
		f.setMovement(0, f.movementDir.Y)
	}
}
func (f *Fighter) StopMovingRight() {
	if f.movementDir.X == 1 {
		f.setMovement(0, f.movementDir.Y)
	}
}
func (f *Fighter) StopMovingUp() {
	if f.movementDir.Y == 1 {
		f.setMovement(f.movementDir.X, 0)
	}
}
func (f *Fighter) StopMovingDown() {
	if f.movementDir.Y == -1 {
		f.setMovement(f.movementDir.X, 0)
	}
}

// Block enters BLOCK from idle/walk.
func (f *Fighter) Block() {
	if f.state == StateIdle || f.state == StateWalk {
		f.changeState(StateBlock)
	}
}

// StopBlocking leaves BLOCK back to walk or idle.
func (f *Fighter) StopBlocking() {
	if f.state != StateBlock {
		return
	}
	if f.movementDir.X != 0 || f.movementDir.Y != 0 {
		f.changeState(StateWalk)
	} else {
		f.changeState(StateIdle)
	}
}

func (f *Fighter) IsBlocking() bool { return f.state == StateBlock }

// Punch starts a punch if idle/walking. Resets contact.
func (f *Fighter) Punch() {
	if f.state == StateIdle || f.state == StateWalk {
		f.changeState(StatePunch)
		f.madeContact = false
	}
}

// Kick starts a kick if idle/walking. Resets contact.
func (f *Fighter) Kick() {
	if f.state == StateIdle || f.state == StateWalk {
		f.changeState(StateKick)
		f.madeContact = false
	}
}

func (f *Fighter) MakeContact()     { f.madeContact = true }
func (f *Fighter) HasMadeContact() bool { return f.madeContact }

func (f *Fighter) IsAttacking() bool {
	return f.state == StatePunch || f.state == StateKick
}

// IsAttackActive is true only in the middle third of the punch/kick animation,
// and only before contact has been registered (Showdown hit window).
func (f *Fighter) IsAttackActive() bool {
	if f.madeContact {
		return false
	}
	switch f.state {
	case StatePunch:
		d := f.animDur[StatePunch]
		return f.stateTime > d*0.33 && f.stateTime < d*0.66
	case StateKick:
		d := f.animDur[StateKick]
		return f.stateTime > d*0.33 && f.stateTime < d*0.66
	default:
		return false
	}
}

// GetHit applies damage. Blocking multiplies by BlockDamageFactor.
// Non-blocking hits that don't kill enter HURT.
func (f *Fighter) GetHit(damage float64) {
	if f.state == StateHurt || f.state == StateWin || f.state == StateLose {
		return
	}
	if f.state == StateBlock {
		f.life -= damage * BlockDamageFactor
	} else {
		f.life -= damage
	}
	if f.life <= 0 {
		f.Lose()
		return
	}
	if f.state != StateBlock {
		f.changeState(StateHurt)
	}
}

func (f *Fighter) Lose() {
	f.changeState(StateLose)
	f.life = 0
}

func (f *Fighter) HasLost() bool { return f.state == StateLose }

func (f *Fighter) Win() { f.changeState(StateWin) }
