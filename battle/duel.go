// Package battle runs a two-fighter duel using the Showdown fighter model.
// Contact distance and hit application match GameScreen logic.
package battle

import (
	"math"

	"github.com/mfeinLearn/krone/fighter"
)

const (
	ContactDistanceX = 7.5
	ContactDistanceY = 1.5
)

// Duel is a live exchange between two fighters on a ring plane.
type Duel struct {
	Player   *fighter.Fighter
	Opponent *fighter.Fighter
	Over     bool
	Winner   string // "player", "opponent", or ""
}

func NewDuel(playerName, opponentName string) *Duel {
	return &Duel{
		Player:   fighter.New(playerName, "#efe8dc"),
		Opponent: fighter.New(opponentName, "#b8bcc4"),
	}
}

// StartRound places fighters and clears win/lose.
func (d *Duel) StartRound(playerX, opponentX, y float64) {
	d.Player.GetReady(playerX, y)
	d.Opponent.GetReady(opponentX, y)
	d.Player.FaceRight()
	d.Opponent.FaceLeft()
	d.Over = false
	d.Winner = ""
}

// FaceEachOther flips facing so each looks toward the other.
func (d *Duel) FaceEachOther() {
	if d.Player.Position().X <= d.Opponent.Position().X {
		d.Player.FaceRight()
		d.Opponent.FaceLeft()
	} else {
		d.Player.FaceLeft()
		d.Opponent.FaceRight()
	}
}

func WithinContact(a, b fighter.Vec2) bool {
	return math.Abs(a.X-b.X) <= ContactDistanceX &&
		math.Abs(a.Y-b.Y) <= ContactDistanceY
}

// Step advances both fighters by dt and resolves active attacks on contact.
func (d *Duel) Step(dt float64) {
	if d.Over {
		return
	}
	d.Player.Update(dt)
	d.Opponent.Update(dt)
	d.FaceEachOther()

	if !WithinContact(d.Player.Position(), d.Opponent.Position()) {
		return
	}

	if d.Player.IsAttackActive() {
		d.Opponent.GetHit(fighter.HitStrength)
		d.Player.MakeContact()
		if d.Opponent.HasLost() {
			d.Player.Win()
			d.Over = true
			d.Winner = "player"
			return
		}
	}
	if d.Opponent.IsAttackActive() {
		d.Player.GetHit(fighter.HitStrength)
		d.Opponent.MakeContact()
		if d.Player.HasLost() {
			d.Opponent.Win()
			d.Over = true
			d.Winner = "opponent"
		}
	}
}
