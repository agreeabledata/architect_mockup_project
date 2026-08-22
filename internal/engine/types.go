package engine

type Mode string

const (
	ModeTitle  Mode = "title"
	ModeBoard  Mode = "board"
	ModeRun    Mode = "run"
	ModeBattle Mode = "battle"
	ModeResult Mode = "result"
	ModePaused Mode = "paused"
	ModeEnded  Mode = "ended"
)

type Rect struct {
	X, Y, W, H float64
}

type PlatformKind string

const (
	PlatformSolid  PlatformKind = "solid"
	PlatformOneway PlatformKind = "oneway"
)

type Platform struct {
	Rect
	Kind PlatformKind
}

type ZoneKind string

const (
	ZonePickup     ZoneKind = "pickup"
	ZoneDropoff    ZoneKind = "dropoff"
	ZoneCheckpoint ZoneKind = "checkpoint"
)

type Zone struct {
	Rect
	Kind ZoneKind
}

type Encounter struct {
	Rect
	ID      string
	Name    string
	HP      float64
	Cleared bool
}

// Coin is a Mario-style collectible. Touching it grants Value krone.
type Coin struct {
	X, Y  float64
	Value int
	Taken bool
}

type Waypoint struct {
	X        float64
	Jump     bool
	Interact bool
}

type LevelDef struct {
	ID          string
	Name        string
	District    string
	Width       float64
	Height      float64
	DeathY      float64
	SpawnX      float64
	SpawnY      float64
	Platforms   []Platform
	Zones       []Zone
	Encounters  []Encounter // Cleared ignored in defs
	Coins       []Coin      // template; Taken ignored in defs
	Waypoints   []Waypoint
	RewardKrone int
	TravelDays  int
}

type QuestStatus string

const (
	QuestOpen     QuestStatus = "open"
	QuestActive   QuestStatus = "active"
	QuestComplete QuestStatus = "complete"
	QuestFailed   QuestStatus = "failed"
)

type Quest struct {
	ID          string
	LevelID     string
	Title       string
	District    string
	Blurb       string
	RewardKrone int
	TravelDays  int
	HasFoe      bool
	FoeName     string
	Status      QuestStatus
}

type Player struct {
	X, Y, VX, VY float64
	W, H         float64
	Facing       int // 1 or -1
	Grounded     bool
	Coyote       float64
	JumpBuf      float64
	DropThrough  float64
	PrevBottom   float64
	JumpCutDone  bool
}

type RunState struct {
	LevelID     string
	QuestID     string
	Player      Player
	Carrying    bool // deprecated; kept for bot compatibility
	Delivered   bool
	CheckpointX float64
	CheckpointY float64
	Encounters  []Encounter
	Coins       []Coin
	Falls       int
	Ticks       int
}

type BattleResult string

const (
	BattleOngoing BattleResult = "ongoing"
	BattleWin     BattleResult = "win"
	BattleLoss    BattleResult = "loss"
)

type BattleTurn struct {
	N          int
	Power      int
	OpponentHP int
	AgentHP    int
	Budget     int
	Phase      string
	Note       string
}

type Battle struct {
	Coins        int
	Power        int
	AgentHP      int
	OpponentHP   int
	OpponentName string
	Budget       int
	Result       BattleResult
	LossReason   string // "hp", "budget", or ""
	Log          []BattleTurn
}

type BattleForecast struct {
	Result     BattleResult
	Reason     string
	Turns      int
	AgentHP    int
	OpponentHP int
	Power      int
}

type LedgerEvent struct {
	T     int
	Kind  string
	Days  int
	Krone int
	Note  string
}

type Ledger struct {
	Days       int
	Krone      int
	HP         int
	MaxHP      int
	Deliveries int
	Events     []LedgerEvent
}

type ResultState struct {
	Kind    string
	Title   string
	Body    string
	QuestID string
}

type Driver string

const (
	DriverPlayer Driver = "player"
	DriverBot    Driver = "bot"
)

type Input struct {
	MoveX       float64
	JumpPressed bool
	JumpHeld    bool
	Down        bool
	Interact    bool
	Confirm     bool
	Pause       bool
}

var ZeroInput Input

type Sim struct {
	Seed    uint32
	Tick    int
	Mode    Mode
	Driver  Driver
	Ledger  Ledger
	Board   []Quest
	Run     *RunState
	Battle  *Battle
	Result  *ResultState
	Ended   string // "win", "loss", or ""
	Toast   string
	UIDirty bool
}
