package engine

func solid(x, y, w float64) Platform {
	return Platform{Rect: Rect{X: x, Y: y, W: w, H: 22}, Kind: PlatformSolid}
}
func oneway(x, y, w float64) Platform {
	return Platform{Rect: Rect{X: x, Y: y, W: w, H: 8}, Kind: PlatformOneway}
}

// coinRow places value-krone coins along a platform top (Mario-style trail).
func coinRow(x0, y, spacing float64, n, value int) []Coin {
	out := make([]Coin, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, Coin{X: x0 + float64(i)*spacing, Y: y, Value: value})
	}
	return out
}

func joinCoins(rows ...[]Coin) []Coin {
	var out []Coin
	for _, r := range rows {
		out = append(out, r...)
	}
	return out
}

const roofY = 280.0

var Levels = map[string]LevelDef{
	"pier4": {
		ID: "pier4", Name: "Pier 4", District: "Harbour",
		Width: 1780, Height: 360, DeathY: 430,
		SpawnX: 48, SpawnY: roofY - 26,
		Platforms: []Platform{
			solid(0, roofY, 310), solid(390, roofY, 200), solid(670, 252, 230),
			solid(980, roofY, 210), solid(1270, roofY, 460),
		},
		Zones: []Zone{
			{Kind: ZoneDropoff, Rect: Rect{1520, roofY - 36, 40, 36}},
			{Kind: ZoneCheckpoint, Rect: Rect{700, 252 - 8, 24, 8}},
		},
		Coins: joinCoins(
			coinRow(80, roofY-28, 36, 5, 5),
			coinRow(420, roofY-28, 36, 4, 5),
			coinRow(700, 252-28, 36, 5, 10),
			coinRow(1000, roofY-28, 36, 4, 5),
			coinRow(1320, roofY-28, 36, 6, 5),
		),
		RewardKrone: 40, TravelDays: 1,
	},
	"clocktower": {
		ID: "clocktower", Name: "Clocktower Run", District: "Old Ward",
		Width: 2100, Height: 360, DeathY: 430,
		SpawnX: 40, SpawnY: roofY - 26,
		Platforms: []Platform{
			solid(0, roofY, 260), solid(340, 250, 180), oneway(560, 210, 140),
			solid(740, roofY, 240), solid(1060, 248, 200), solid(1350, roofY, 180),
			solid(1620, roofY, 430),
		},
		Zones: []Zone{
			{Kind: ZoneDropoff, Rect: Rect{1860, roofY - 36, 40, 36}},
			{Kind: ZoneCheckpoint, Rect: Rect{780, roofY - 8, 24, 8}},
		},
		Coins: joinCoins(
			coinRow(60, roofY-28, 36, 4, 5),
			coinRow(360, 250-28, 36, 4, 10),
			coinRow(580, 210-28, 36, 3, 10),
			coinRow(780, roofY-28, 36, 5, 5),
			coinRow(1080, 248-28, 36, 4, 10),
			coinRow(1380, roofY-28, 36, 3, 5),
			coinRow(1680, roofY-28, 36, 5, 5),
		),
		RewardKrone: 55, TravelDays: 1,
	},
	"nightmarket": {
		ID: "nightmarket", Name: "Night Market", District: "Lantern Row",
		Width: 2320, Height: 360, DeathY: 430,
		SpawnX: 36, SpawnY: roofY - 26,
		Platforms: []Platform{
			solid(0, roofY, 240), solid(320, roofY, 160), oneway(520, 236, 160),
			solid(720, 200, 220), solid(1000, roofY, 200), oneway(1240, 240, 150),
			solid(1440, roofY, 220), solid(1760, 250, 180), solid(2020, roofY, 280),
		},
		Zones: []Zone{
			{Kind: ZoneDropoff, Rect: Rect{2160, roofY - 36, 40, 36}},
			{Kind: ZoneCheckpoint, Rect: Rect{1040, roofY - 8, 24, 8}},
		},
		Coins: joinCoins(
			coinRow(50, roofY-28, 36, 4, 5),
			coinRow(340, roofY-28, 36, 3, 5),
			coinRow(540, 236-28, 36, 3, 10),
			coinRow(740, 200-28, 36, 5, 10),
			coinRow(1020, roofY-28, 36, 4, 5),
			coinRow(1260, 240-28, 36, 3, 10),
			coinRow(1480, roofY-28, 36, 4, 5),
			coinRow(1780, 250-28, 36, 4, 10),
			coinRow(2060, roofY-28, 36, 4, 5),
		),
		RewardKrone: 70, TravelDays: 1,
	},
	"roofduel": {
		ID: "roofduel", Name: "Roof Duel", District: "High Spine",
		Width: 1960, Height: 360, DeathY: 430,
		SpawnX: 40, SpawnY: roofY - 26,
		Platforms: []Platform{
			solid(0, roofY, 280), solid(360, roofY, 240), solid(680, roofY, 520),
			solid(1280, 250, 200), solid(1560, roofY, 380),
		},
		Zones: []Zone{
			{Kind: ZoneDropoff, Rect: Rect{1760, roofY - 36, 40, 36}},
			{Kind: ZoneCheckpoint, Rect: Rect{700, roofY - 8, 24, 8}},
		},
		// Enough coins on the route that collecting them can fund a win vs the thug.
		Coins: joinCoins(
			coinRow(60, roofY-28, 36, 5, 10),
			coinRow(380, roofY-28, 36, 5, 10),
			coinRow(700, roofY-28, 36, 8, 10),
			coinRow(1000, roofY-28, 36, 6, 15),
			coinRow(1300, 250-28, 36, 4, 15),
			coinRow(1600, roofY-28, 36, 5, 10),
		),
		Encounters: []Encounter{
			{ID: "thug-1", Name: "Roof thug", HP: 100, Rect: Rect{880, roofY - 30, 18, 30}},
		},
		RewardKrone: 90, TravelDays: 1,
	},
}

var LevelOrder = []string{"pier4", "clocktower", "nightmarket", "roofduel"}

func GetLevel(id string) LevelDef {
	lvl, ok := Levels[id]
	if !ok {
		panic("unknown level: " + id)
	}
	return lvl
}

// FoeVisible is true when the player can forecast a win against the foe.
// Until then the opponent stays hidden on this stage.
func FoeVisible(krone, agentHP int, enc Encounter) bool {
	hp := int(enc.HP)
	if hp <= 0 {
		hp = OpponentStartHP
	}
	return Forecast(krone, agentHP, hp).Result == BattleWin
}
