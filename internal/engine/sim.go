package engine

func CreateSim(seed uint32, driver Driver) *Sim {
	if seed == 0 {
		seed = 1
	}
	if driver == "" {
		driver = DriverPlayer
	}
	return &Sim{
		Seed:    seed,
		Mode:    ModeTitle,
		Driver:  driver,
		Ledger:  CreateLedger(),
		Board:   SeedBoard(seed),
		UIDirty: true,
	}
}

func dirty(s *Sim) { s.UIDirty = true }

func zoneOf(level LevelDef, kind ZoneKind) *Zone {
	for i := range level.Zones {
		if level.Zones[i].Kind == kind {
			return &level.Zones[i]
		}
	}
	return nil
}

func startRun(s *Sim, quest Quest) {
	level := GetLevel(quest.LevelID)
	p := MakePlayer(level.SpawnX, level.SpawnY)
	SnapToGround(&p, level.Platforms)
	encs := make([]Encounter, len(level.Encounters))
	for i, e := range level.Encounters {
		encs[i] = e
		encs[i].Cleared = false
	}
	coins := make([]Coin, len(level.Coins))
	for i, c := range level.Coins {
		coins[i] = c
		coins[i].Taken = false
	}
	s.Run = &RunState{
		LevelID: level.ID, QuestID: quest.ID, Player: p,
		CheckpointX: level.SpawnX, CheckpointY: level.SpawnY,
		Encounters: encs, Coins: coins,
	}
	for i := range s.Board {
		if s.Board[i].ID == quest.ID {
			s.Board[i].Status = QuestActive
		}
	}
	s.Ledger = DebitDays(s.Ledger, quest.TravelDays, s.Tick, "Travel to "+quest.District+".", "travel")
	s.Mode = ModeRun
	s.Battle = nil
	s.Result = nil
	dirty(s)
}

func failRun(s *Sim, body string) {
	questID := ""
	if s.Run != nil {
		questID = s.Run.QuestID
		for i := range s.Board {
			if s.Board[i].ID == questID {
				s.Board[i].Status = QuestFailed
			}
		}
	}
	s.Ledger = RestHP(s.Ledger)
	s.Run = nil
	s.Battle = nil
	s.Mode = ModeResult
	s.Result = &ResultState{Kind: "failed", Title: "Run failed", Body: body, QuestID: questID}
	if s.Ledger.Days <= 0 {
		s.Ended = "loss"
		s.Mode = ModeEnded
		s.Result = &ResultState{Kind: "failed", Title: "The ledger closes",
			Body: "No days remain. The page is full of debts.", QuestID: questID}
	}
	dirty(s)
}

func completeRun(s *Sim) {
	run := s.Run
	if run == nil {
		return
	}
	var quest *Quest
	for i := range s.Board {
		if s.Board[i].ID == run.QuestID {
			quest = &s.Board[i]
			break
		}
	}
	if quest == nil {
		return
	}
	s.Ledger = CreditKrone(s.Ledger, quest.RewardKrone, s.Tick, "Delivered "+quest.Title+".")
	s.Ledger = RestHP(s.Ledger)
	quest.Status = QuestComplete
	s.Run = nil
	s.Battle = nil
	remaining := 0
	for _, q := range s.Board {
		if q.Status != QuestComplete {
			remaining++
		}
	}
	if remaining == 0 {
		s.Ended = "win"
		s.Mode = ModeEnded
		s.Result = &ResultState{Kind: "campaign", Title: "The board is clear",
			Body: "Deliveries done. The ledger holds.", QuestID: quest.ID}
	} else {
		s.Mode = ModeResult
		s.Result = &ResultState{Kind: "delivered", Title: "Stamped",
			Body: "Parcel delivered. Jobs remain.", QuestID: quest.ID}
	}
	dirty(s)
}

func beginBattle(s *Sim, enc Encounter) {
	b := StartBattle(s.Ledger.Krone, s.Ledger.HP, int(enc.HP), enc.Name)
	s.Battle = &b
	s.Mode = ModeBattle
	dirty(s)
}

func finishBattle(s *Sim) {
	b := s.Battle
	run := s.Run
	if b == nil || run == nil {
		return
	}
	if b.Result == BattleWin {
		for i := range run.Encounters {
			if !run.Encounters[i].Cleared {
				run.Encounters[i].Cleared = true
				break
			}
		}
		s.Ledger = SetHP(s.Ledger, b.AgentHP)
		s.Battle = nil
		s.Mode = ModeRun
		s.Toast = b.OpponentName + " down."
		dirty(s)
		return
	}
	s.Ledger = SetHP(s.Ledger, 0)
	s.Ledger = DebitDays(s.Ledger, 3, s.Tick, "Defeated on the roofs.", "defeat")
	failRun(s, b.OpponentName+" still stood. Three days struck from the page.")
}

func tickRun(s *Sim, in Input) {
	run := s.Run
	if run == nil {
		return
	}
	level := GetLevel(run.LevelID)
	p := &run.Player
	run.Ticks++
	StepPlayer(p, in, level.Platforms, Tick)

	if p.X < 0 {
		p.X = 0
		p.VX = 0
	}
	if p.X+p.W > level.Width {
		p.X = level.Width - p.W
		p.VX = 0
	}

	if cp := zoneOf(level, ZoneCheckpoint); cp != nil && AABB(Rect{p.X, p.Y, p.W, p.H}, cp.Rect) {
		run.CheckpointX, run.CheckpointY = p.X, p.Y
	}

	if p.Y > level.DeathY {
		run.Falls++
		s.Ledger = DebitDays(s.Ledger, 1, s.Tick, "Fell from the roofs.", "fall")
		if s.Ledger.Days <= 0 {
			failRun(s, "A fall, and the last day gone.")
			return
		}
		p.X, p.Y = run.CheckpointX, run.CheckpointY
		p.VX, p.VY = 0, 0
		p.Grounded = true
		s.Toast = "Fell. One day struck."
		dirty(s)
		return
	}

	// Mario-style coins: touch to collect → krone.
	pr := Rect{p.X, p.Y, p.W, p.H}
	for i := range run.Coins {
		c := &run.Coins[i]
		if c.Taken {
			continue
		}
		cr := Rect{c.X, c.Y, 14, 14}
		if AABB(pr, cr) {
			c.Taken = true
			val := c.Value
			if val <= 0 {
				val = 5
			}
			s.Ledger.Krone += val
			s.Ledger.Events = append(s.Ledger.Events, LedgerEvent{
				T: s.Tick, Kind: "coin", Krone: val, Note: "Coin collected.",
			})
			s.Toast = "+" + itoa(val) + " krone"
			dirty(s)
		}
	}

	for _, enc := range run.Encounters {
		if enc.Cleared {
			continue
		}
		if !FoeVisible(s.Ledger.Krone, s.Ledger.HP, enc) {
			continue
		}
		if AABB(pr, enc.Rect) {
			beginBattle(s, enc)
			return
		}
	}

	drop := zoneOf(level, ZoneDropoff)
	if drop != nil && AABB(pr, drop.Rect) && (in.Interact || in.Confirm) {
		run.Delivered = true
		completeRun(s)
	}
}

func tickBattle(s *Sim, in Input) {
	if s.Battle == nil || s.Battle.Result != BattleOngoing {
		return
	}
	// Bots may resolve instantly; players must punch through (PlayerStrike).
	if s.Driver == DriverBot {
		b := ResolveBattle(*s.Battle)
		s.Battle = &b
		dirty(s)
		if s.Battle.Result != BattleOngoing {
			finishBattle(s)
		}
		return
	}
	// Player path: only explicit single strikes via PlayerStrike — no Enter skip.
}

func AcceptQuest(s *Sim, questID string) {
	for _, q := range s.Board {
		if q.ID == questID && q.Status == QuestOpen {
			if s.Ledger.Days < q.TravelDays {
				s.Toast = "Not enough days to travel."
				dirty(s)
				return
			}
			startRun(s, q)
			return
		}
	}
}

func GoToBoard(s *Sim) {
	s.Mode = ModeBoard
	s.Result = nil
	dirty(s)
}

func DismissResult(s *Sim) {
	if s.Mode == ModeResult {
		s.Mode = ModeBoard
		s.Result = nil
		dirty(s)
	}
}

func PlayerStrike(s *Sim) {
	if s.Mode != ModeBattle || s.Battle == nil || s.Battle.Result != BattleOngoing {
		return
	}
	b := StepBattle(*s.Battle)
	s.Battle = &b
	dirty(s)
	if s.Battle.Result != BattleOngoing {
		finishBattle(s)
	}
}

// PlayerResolve is disabled for player fights (no skip). Bots use ResolveBattle in tickBattle.
func PlayerResolve(s *Sim) {
	// no-op: battles must be finished with strikes
}

func StepSim(s *Sim, in Input) {
	s.Tick++
	if s.Toast != "" && s.Tick%180 == 0 {
		s.Toast = ""
	}
	switch s.Mode {
	case ModeTitle:
		if in.Confirm || in.Interact {
			s.Mode = ModeBoard
			dirty(s)
		}
	case ModeEnded:
		return
	case ModeResult:
		if in.Confirm || in.Interact {
			s.Mode = ModeBoard
			s.Result = nil
			dirty(s)
		}
	case ModeRun:
		tickRun(s, in)
	case ModeBattle:
		tickBattle(s, in)
	case ModePaused:
		if in.Pause {
			s.Mode = ModeRun
			dirty(s)
		}
	}
}
