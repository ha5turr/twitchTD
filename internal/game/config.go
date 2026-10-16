package game

// Config holds every balance number. Defaults live in DefaultConfig; a JSON
// file may override any subset of fields.
type Config struct {
	TickRate int `json:"tickRate"`

	BuildPhaseSec      float64 `json:"buildPhaseSec"`
	FirstBuildPhaseSec float64 `json:"firstBuildPhaseSec"`
	GameOverSec        float64 `json:"gameOverSec"`
	SpawnIntervalSec   float64 `json:"spawnIntervalSec"`

	WallsHP float64 `json:"wallsHP"`

	StartGold      int  `json:"startGold"`
	WalletCap      int  `json:"walletCap"`
	KeepGold       bool `json:"keepGold"` // keep gold between games instead of resetting
	TowerLimit     int  `json:"towerLimit"`
	SellRefundPct  int  `json:"sellRefundPct"`
	ActiveWindowMn int  `json:"activeWindowMin"`

	ChatGold        int     `json:"chatGold"`
	ChatCooldownSec float64 `json:"chatCooldownSec"`
	// Gold above the wallet cap goes to the common treasury and repairs walls.
	OverflowWallsPerGold float64 `json:"overflowWallsPerGold"`
	// Paid every Present Viewers tick to chatters who already have a mercenary.
	PresentGold int `json:"presentGold"`

	WorkerSpeed    float64 `json:"workerSpeed"` // cells per second
	BuildTimeSec   float64 `json:"buildTimeSec"`
	UpgradeTimeSec float64 `json:"upgradeTimeSec"`
	MaxOrders      int     `json:"maxOrders"`
	StunRadius     float64 `json:"stunRadius"` // cells
	StunRestSec    float64 `json:"stunRestSec"`

	ClassBonus    float64              `json:"classBonus"` // extra damage of a class's signature building
	Drinks        map[string]DrinkSpec `json:"drinks"`
	DrunkDecaySec float64              `json:"drunkDecaySec"` // seconds for one mug to wear off

	// Bots and service accounts whose messages are ignored entirely.
	IgnoreUsers []string `json:"ignoreUsers"`

	BaseHP        float64 `json:"baseHP"`
	HPGrowth      float64 `json:"hpGrowth"`
	ChatterHPCoef float64 `json:"chatterHPCoef"`
	BaseCount     int     `json:"baseCount"`
	CountPerWave  int     `json:"countPerWave"`
	BossHPMult    float64 `json:"bossHPMult"`
	BossEvery     int     `json:"bossEvery"`

	Enemies map[string]EnemySpec `json:"enemies"`
	Towers  map[string]TowerSpec `json:"towers"`

	// Webcam zone in chess cells, inclusive. Empty From disables it.
	WebcamFrom string `json:"webcamFrom"`
	WebcamTo   string `json:"webcamTo"`
}

type EnemySpec struct {
	HPMult  float64 `json:"hpMult"`
	Speed   float64 `json:"speed"` // cells per second
	Armor   float64 `json:"armor"` // 0..1, cut of physical damage
	WallDPS float64 `json:"wallDps"`
	Bounty  int     `json:"bounty"`
}

type TowerSpec struct {
	Name     string  `json:"name"`
	Cost     int     `json:"cost"`
	Damage   float64 `json:"damage"`
	Range    float64 `json:"range"`   // cells
	Rate     float64 `json:"rate"`    // shots per second
	ProjSpd  float64 `json:"projSpd"` // cells per second
	Magic    bool    `json:"magic"`   // ignores armor
	UpgMult  float64 `json:"upgMult"` // damage multiplier per level
	MaxLevel int     `json:"maxLevel"`
}

func DefaultConfig() Config {
	return Config{
		TickRate:             20,
		BuildPhaseSec:        35,
		FirstBuildPhaseSec:   60,
		GameOverSec:          18,
		SpawnIntervalSec:     0.9,
		WallsHP:              1000,
		StartGold:            120,
		WalletCap:            1000,
		TowerLimit:           5,
		SellRefundPct:        60,
		ActiveWindowMn:       10,
		ChatGold:             5,
		ChatCooldownSec:      45,
		OverflowWallsPerGold: 0.5,
		PresentGold:          10,
		WorkerSpeed:          2.6,
		BuildTimeSec:         3,
		UpgradeTimeSec:       2,
		MaxOrders:            3,
		StunRadius:           0.45,
		StunRestSec:          2,
		ClassBonus:           0.2,
		Drinks: map[string]DrinkSpec{
			"ale":    {Name: "Эль", Price: 10, Mugs: 1},
			"mead":   {Name: "Медовуха", Price: 25, Mugs: 2},
			"potion": {Name: "Чёрное зелье", Price: 60, Mugs: 4},
		},
		DrunkDecaySec: 90,
		IgnoreUsers:   []string{"nightbot", "streamelements", "streamlabs", "moobot", "fossabot", "wizebot", "sery_bot", "soundalerts"},
		BaseHP:        30,
		HPGrowth:      1.12,
		ChatterHPCoef: 0.25,
		BaseCount:     6,
		CountPerWave:  2,
		BossHPMult:    15,
		BossEvery:     5,
		Enemies: map[string]EnemySpec{
			"normal":  {HPMult: 1, Speed: 1.1, WallDPS: 6, Bounty: 4},
			"fast":    {HPMult: 0.55, Speed: 2.0, WallDPS: 4, Bounty: 3},
			"armored": {HPMult: 1.4, Speed: 0.8, Armor: 0.5, WallDPS: 9, Bounty: 6},
			"boss":    {HPMult: 1, Speed: 0.6, Armor: 0.2, WallDPS: 60, Bounty: 80},
		},
		Towers: map[string]TowerSpec{
			"arrow": {Name: "Пост лучников", Cost: 50, Damage: 9, Range: 2.6, Rate: 1.4, ProjSpd: 10, UpgMult: 1.45, MaxLevel: 5},
			"magic": {Name: "Тёмный обелиск", Cost: 80, Damage: 22, Range: 2.2, Rate: 0.6, ProjSpd: 7, Magic: true, UpgMult: 1.45, MaxLevel: 5},
		},
		WebcamFrom: "N1",
		WebcamTo:   "P5",
	}
}

func (c Config) TowerCost(kind string, level int) int {
	spec := c.Towers[kind]
	// Each upgrade costs the base price times the target level.
	return spec.Cost * level
}
