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
	GateDPS float64 `json:"gateDps"` // guards on the walls hurt each enemy at the gate

	StartGold    int  `json:"startGold"`
	WalletCap    int  `json:"walletCap"`
	KeepGold     bool `json:"keepGold"`     // keep gold between games instead of resetting
	KeepTreasury bool `json:"keepTreasury"` // keep the treasury between games instead of emptying it
	TowerLimit   int  `json:"towerLimit"`
	// The limit grows by one after every boss wave held, up to
	// TowerLimitMax; a party of SmallTeamSize players or fewer builds
	// SmallTeamBonus more on top.
	TowerLimitMax int `json:"towerLimitMax"`
	// A player who first joins a game from GrantFromWave on gets a one-off
	// grant of GrantPct of the median the others have invested, spendable
	// only on buildings and upgrades.
	GrantFromWave  int     `json:"grantFromWave"`
	GrantPct       float64 `json:"grantPct"`
	SmallTeamSize  int     `json:"smallTeamSize"`
	SmallTeamBonus int     `json:"smallTeamBonus"`
	// StreamerPlays makes the broadcaster's own chat lines play by the
	// viewers' rules (gold, limits, a mercenary in the tavern). The panel
	// keeps its free, instant building either way.
	StreamerPlays  bool `json:"streamerPlays"`
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

	ClassBonus float64 `json:"classBonus"` // extra damage of a class's signature building

	// Support buildings and barricades (hotfix "We Deeply sorry").
	ShrineLimit       int     `json:"shrineLimit"`       // shrines one player may own; 0 = no limit
	ShrineLimitCleric int     `json:"shrineLimitCleric"` // the same for a cleric, whose signature building it is
	ShrineAssistPct   float64 `json:"shrineAssistPct"`   // a shrine takes this part of its share of the damage in every kill's bounty
	DrumAssistPct     float64 `json:"drumAssistPct"`     // share of a kill's bounty for each drum that sped up the shot
	ShrineWallRepair  float64 `json:"shrineWallRepair"`  // share of every barricade's HP a shrine mends after a wave, per level
	WallHoldGold      float64 `json:"wallHoldGold"`      // gold per second for each enemy a barricade holds
	WallRepairShare   float64 `json:"wallRepairShare"`   // price of a full repair as a share of the barricade's last price
	WallRepairCDSec   float64 `json:"wallRepairCdSec"`   // a barricade can be mended again only this long after the last repair
	WallHoldMax       float64 `json:"wallHoldMax"`       // gold one barricade may pay for holding per wave (grows with rewards); 0 = no cap
	SiegeRageSec      float64 `json:"siegeRageSec"`      // an enemy held this long by a barricade flies into a rage: no healing, harder blows
	SiegeRagePerSec   float64 `json:"siegeRagePerSec"`   // a raging enemy's blows on the barricade grow by this share per second
	WallRefundPct     int     `json:"wallRefundPct"`     // refund when a barricade is broken
	WallRefundClass   int     `json:"wallRefundClass"`   // the same for the Воитель

	// Class passives (rogues.go).
	RogueEverySec     float64 `json:"rogueEverySec"`
	RogueStealPct     float64 `json:"rogueStealPct"` // share of the victim's gold
	RogueStealMax     int     `json:"rogueStealMax"`
	DruidEverySec     float64 `json:"druidEverySec"`
	DruidPantherSec   float64 `json:"druidPantherSec"`
	DruidPantherSpeed float64 `json:"druidPantherSpeed"` // cells per second
	DruidMaul         float64 `json:"druidMaul"`         // damage per bite, in base enemy HP of the wave

	// Class skills (heroes.go), all in seconds or in base enemy HP of the wave.
	WarriorEverySec    float64 `json:"warriorEverySec"`
	WarriorSpeed       float64 `json:"warriorSpeed"` // cells per second along the path
	WarriorPush        float64 `json:"warriorPush"`  // cells each enemy is shoved back
	MageEverySec       float64 `json:"mageEverySec"`
	MeteorCount        int     `json:"meteorCount"`
	MeteorDmg          float64 `json:"meteorDmg"`
	MeteorRadius       float64 `json:"meteorRadius"`
	MeteorBurn         float64 `json:"meteorBurn"` // burn per second
	MeteorBurnSec      float64 `json:"meteorBurnSec"`
	BardEverySec       float64 `json:"bardEverySec"`
	BardMusicSec       float64 `json:"bardMusicSec"`
	NoteDmg            float64 `json:"noteDmg"`
	NoteRate           float64 `json:"noteRate"` // notes per second
	NoteRange          float64 `json:"noteRange"`
	RogueStepEverySec  float64 `json:"rogueStepEverySec"`
	RogueStepSec       float64 `json:"rogueStepSec"` // seconds the rogue stays out
	RogueStepDmg       float64 `json:"rogueStepDmg"`
	RogueFanDmg        float64 `json:"rogueFanDmg"`
	RogueFanRadius     float64 `json:"rogueFanRadius"`
	ClericEverySec     float64 `json:"clericEverySec"`
	PillarDmg          float64 `json:"pillarDmg"`
	PillarStunSec      float64 `json:"pillarStunSec"`
	PillarRadius       float64 `json:"pillarRadius"` // one cell
	PyroEverySec       float64 `json:"pyroEverySec"`
	PathFireDmg        float64 `json:"pathFireDmg"`
	PathFireBurn       float64 `json:"pathFireBurn"` // burn per second
	PathFireBurnSec    float64 `json:"pathFireBurnSec"`
	BossNetImmuneSec   float64 `json:"bossNetImmuneSec"` // after a net a boss shakes off new ones this long
	RangerEverySec     float64 `json:"rangerEverySec"`
	WarlockEverySec    float64 `json:"warlockEverySec"`  // mass curse: every enemy on the map cursed at once
	WarlockCurseSec    float64 `json:"warlockCurseSec"`  // for this long
	WarlockCurseAmp    float64 `json:"warlockCurseAmp"`  // taking this much more damage (bosses half)
	RangerRangeBonus   float64 `json:"rangerRangeBonus"` // a ranger's ballistas reach this much further
	CritBase           float64 `json:"critBase"`         // chance of a ranger's ballista to land a critical hit
	CritPerBoss        float64 `json:"critPerBoss"`      // more after every boss wave held
	CritMax            float64 `json:"critMax"`
	CritMult           float64 `json:"critMult"`         // a critical hit's damage multiplier
	ClericShrineDmg    float64 `json:"clericShrineDmg"`  // the pillar of light hits this much harder per shrine of the cleric
	ClericShrineStun   float64 `json:"clericShrineStun"` // and stuns this many seconds longer
	PyroGateEverySec   float64 `json:"pyroGateEverySec"` // the pyromancer mans a brazier on the wall by the door
	PyroGateSec        float64 `json:"pyroGateSec"`      // for this long
	BearEverySec       float64 `json:"bearEverySec"`     // the druid's bear form
	BearSpeed          float64 `json:"bearSpeed"`
	BearFightSec       float64 `json:"bearFightSec"`
	BearReach          float64 `json:"bearReach"`          // cells around the bear it mauls
	BearDPS            float64 `json:"bearDps"`            // per second, share of the wave's base HP
	MageFeastEverySec  float64 `json:"mageFeastEverySec"`  // the mage's feast on the tavern tables
	ShamanWolfEverySec float64 `json:"shamanWolfEverySec"` // the shaman's two spectral wolves
	WolfSpeed          float64 `json:"wolfSpeed"`
	WolfDmg            float64 `json:"wolfDmg"`      // each bite, share of the wave's base HP
	PactEverySec       float64 `json:"pactEverySec"` // blood pact: the warlock's second skill
	PactWallsPct       float64 `json:"pactWallsPct"` // share of the tavern walls' max HP it costs
	PactBoost          float64 `json:"pactBoost"`    // extra damage every building on the map deals
	PactSec            float64 `json:"pactSec"`      // for this long
	PactMinWalls       float64 `json:"pactMinWalls"` // no pact while the walls are below this share
	ShamanEverySec     float64 `json:"shamanEverySec"`
	QuakeStunSec       float64 `json:"quakeStunSec"`    // ground enemies stunned by the earthquake (bosses half)
	CurseShardPct      float64 `json:"curseShardPct"`   // a cursed kill pays the altar's owner this share of the bounty
	CurseSoulChance    float64 `json:"curseSoulChance"` // and gives him a soul with this chance
	ArrowRainDmg       float64 `json:"arrowRainDmg"`
	ArrowRainRadius    float64 `json:"arrowRainRadius"`

	// Traps (hotfix 2).
	TrapBleed      float64 `json:"trapBleed"` // bleeding per second as a share of the snap's damage
	TrapBleedSec   float64 `json:"trapBleedSec"`
	TrapHealCut    float64 `json:"trapHealCut"` // share of healing lost while the cut is fresh
	TrapHealCutSec float64 `json:"trapHealCutSec"`
	TrapAssistPct  float64 `json:"trapAssistPct"` // share of the bounty for the trap owner when someone else kills

	// Rewards grow after every boss wave by this share (kills, assists, barricades).
	RewardGrowth  float64              `json:"rewardGrowth"`
	Drinks        map[string]DrinkSpec `json:"drinks"`
	DrunkDecaySec float64              `json:"drunkDecaySec"` // seconds for one mug to wear off
	PassOutMugs   float64              `json:"passOutMugs"`   // this drunk, a mercenary passes out under the table
	PassOutSec    float64              `json:"passOutSec"`    // and ignores his owner's commands this long

	DiceTables   int     `json:"diceTables"`   // games of dice that can run at once
	DiceMinStake int     `json:"diceMinStake"` // smallest stake in gold
	DiceWaitSec  float64 `json:"diceWaitSec"`  // how long a table waits for an opponent
	DiceRollSec  float64 `json:"diceRollSec"`  // suspense before the dice are thrown

	// Souls (progress.go): earned at game over by those who fought.
	SoulsGoldPct     float64 `json:"soulsGoldPct"`   // share of the gold left in the wallet
	SoulsPerDamage   float64 `json:"soulsPerDamage"` // per point of damage dealt this game
	SoulsPerKill     float64 `json:"soulsPerKill"`
	SoulsGameCap     int     `json:"soulsGameCap"`     // most souls one game can give
	SoulsPerMemecoin int     `json:"soulsPerMemecoin"` // price of a memecoin request
	MemeMax          int     `json:"memeMax"`          // most memecoins in one request

	Rewards            []RewardSpec       `json:"rewards"`  // channel point rewards by title
	BitsGold           float64            `json:"bitsGold"` // gold per bit
	SubGold            int                `json:"subGold"`
	GiftRecipientGold  int                `json:"giftRecipientGold"`
	DonationGoldPerRub float64            `json:"donationGoldPerRub"` // 100 RUB fill the wallet by default
	CurrencyRub        map[string]float64 `json:"currencyRub"`        // roubles per unit of currency
	TreasuryGoldPerSec float64            `json:"treasuryGoldPerSec"` // how fast the treasury repairs walls

	// Bots and service accounts whose messages are ignored entirely.
	IgnoreUsers []string `json:"ignoreUsers"`

	BaseHP        float64 `json:"baseHP"`
	HPGrowth      float64 `json:"hpGrowth"`
	ChatterHPCoef float64 `json:"chatterHPCoef"`
	BaseCount     int     `json:"baseCount"`
	CountPerWave  int     `json:"countPerWave"`
	BossHPMult    float64 `json:"bossHPMult"`
	BossEvery     int     `json:"bossEvery"`
	EnemyRegen    float64 `json:"enemyRegen"` // every enemy heals this share of its max HP per second
	// Chance that a boss wave brings ЭссДиЭф instead of the biome's boss.
	SdfChance float64 `json:"sdfChance"`
	// ЭссДиЭф never comes before this wave (`!tdspawn sdf` still works any time).
	PathCurve     float64 `json:"pathCurve"`     // how winding new maps' paths are, 0 (straighter) to 1 (as winding as fits)
	EventChance   float64 `json:"eventChance"`   // chance an ordinary wave brings a guest (tax collector or golden golem)
	EventFromWave int     `json:"eventFromWave"` // not before this wave
	SdfFromWave   int     `json:"sdfFromWave"`

	Enemies map[string]EnemySpec `json:"enemies"`
	Towers  map[string]TowerSpec `json:"towers"`

	// Webcam zone in chess cells, inclusive. Empty From disables it.
	WebcamFrom string `json:"webcamFrom"`
	WebcamTo   string `json:"webcamTo"`
}

type EnemySpec struct {
	Name       string  `json:"name"`
	HPMult     float64 `json:"hpMult"`
	Speed      float64 `json:"speed"` // cells per second
	Armor      float64 `json:"armor"` // 0..1, cut of physical damage
	WallDPS    float64 `json:"wallDps"`
	Bounty     int     `json:"bounty"`
	Heal       float64 `json:"heal"`       // heals allies nearby, share of their max HP per second
	Regen      float64 `json:"regen"`      // heals itself, share of max HP per second
	Ghost      bool    `json:"ghost"`      // floats through barricades and over traps
	SlowImmune bool    `json:"slowImmune"` // traps do not slow it
	Bomb       float64 `json:"bomb"`       // blows up at the first barricade, taking this share of its max HP
	Guard      float64 `json:"guard"`      // shields everyone within this radius: physical damage halved
	Split      string  `json:"split"`      // dies into SplitN of these
	SplitN     int     `json:"splitN"`
	Fly        bool    `json:"fly"`    // flies straight to the tavern over barricades and traps
	Dispel     float64 `json:"dispel"` // washes nets, burns, curses and slows off everyone within this radius
	Boss       bool    `json:"boss"`
}

// TowerRole is how a building acts.
type TowerRole string

const (
	RoleShooter TowerRole = "shooter" // fires projectiles (arrow, magic, fire)
	RoleAura    TowerRole = "aura"    // buffs neighbours (shrine, drums)
	RoleTrap    TowerRole = "trap"    // on the path, hurts and slows passers-by
	RoleWall    TowerRole = "wall"    // on the path, blocks until broken
	RoleCurse   TowerRole = "curse"   // curses enemies so everything hurts them more
)

type TowerSpec struct {
	Name     string    `json:"name"`
	Role     TowerRole `json:"role"`
	Cost     int       `json:"cost"`
	UpgMult  float64   `json:"upgMult"` // main stat multiplier per level
	MaxLevel int       `json:"maxLevel"`

	// Shooters.
	Damage  float64 `json:"damage"`
	Range   float64 `json:"range"`   // cells
	Rate    float64 `json:"rate"`    // shots per second
	ProjSpd float64 `json:"projSpd"` // cells per second
	Magic   bool    `json:"magic"`   // ignores armor
	Splash  float64 `json:"splash"`  // radius of area damage, cells
	BurnDPS float64 `json:"burnDps"` // damage over time after a hit, ignores armor
	BurnSec float64 `json:"burnSec"`
	// Lightning (magic): every StrikeEvery seconds, faster with each level, a
	// bolt hits all enemies within StrikeRadius of one in range for
	// StrikeDmg times the tower's damage.
	StrikeEvery  float64 `json:"strikeEvery"`
	StrikeDmg    float64 `json:"strikeDmg"`
	StrikeRadius float64 `json:"strikeRadius"`
	// Nets (ballista): every NetEvery seconds, more often with each level, a
	// net holds every enemy within NetRadius of one in range for NetSec,
	// without damage.
	NetEvery  float64 `json:"netEvery"`
	NetSec    float64 `json:"netSec"`
	NetRadius float64 `json:"netRadius"`

	// Traps.
	Slow    float64 `json:"slow"` // speed multiplier while slowed
	SlowSec float64 `json:"slowSec"`
	Charges int     `json:"charges"` // triggers per wave, +2 per level

	// Barricades.
	HP float64 `json:"hp"`

	// Auras: bonuses to buildings within AuraRange, +50% per level above 1.
	AuraRange float64 `json:"auraRange"`
	AuraDmg   float64 `json:"auraDmg"`  // damage bonus, 0.15 = +15%
	AuraRate  float64 `json:"auraRate"` // fire rate bonus
	WallHeal  float64 `json:"wallHeal"` // tavern wall repair, HP per second per level

	// Curse (warlock's altar): every CurseEvery seconds, more often with each
	// level, the toughest enemy in Range takes CurseAmp more damage from
	// everything and loses its armor for CurseSec; bosses take half the extra.
	CurseEvery float64 `json:"curseEvery"`
	CurseSec   float64 `json:"curseSec"`
	CurseAmp   float64 `json:"curseAmp"`
	// Chain lightning (shaman's totem): a hit jumps to Chain more enemies, one
	// more every other level, each within ChainRange of the last and taking
	// ChainFalloff of the hit before.
	Chain        int     `json:"chain"`
	ChainRange   float64 `json:"chainRange"`
	ChainFalloff float64 `json:"chainFalloff"`

	// Only: the building fires at this enemy kind alone, and only while it is
	// on the field; BossPct is its damage as a share of the target's max HP.
	Only    string  `json:"only"`
	BossPct float64 `json:"bossPct"`
}

// OnPath reports whether the building stands on the enemy path.
func (s TowerSpec) OnPath() bool { return s.Role == RoleTrap || s.Role == RoleWall }

// TowerKinds lists buildings in the order chat sees them.
var TowerKinds = []string{"arrow", "magic", "fire", "shrine", "trap", "drums", "wall", "curse", "totem", "clown"}

func DefaultConfig() Config {
	return Config{
		TickRate:             20,
		BuildPhaseSec:        35,
		FirstBuildPhaseSec:   60,
		GameOverSec:          18,
		SpawnIntervalSec:     0.9,
		WallsHP:              1000,
		GateDPS:              12,
		StartGold:            120,
		WalletCap:            1000,
		KeepTreasury:         true,
		TowerLimit:           5,
		TowerLimitMax:        10,
		GrantFromWave:        5,
		GrantPct:             0.5,
		SmallTeamSize:        2,
		SmallTeamBonus:       2,
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
		ShrineLimit:          2,
		ShrineLimitCleric:    4,
		ShrineAssistPct:      0.5,
		DrumAssistPct:        0.5,
		ShrineWallRepair:     0.1,
		WallHoldGold:         0.5,
		WallRepairShare:      0.3,
		WallRepairCDSec:      40,
		WallHoldMax:          40,
		SiegeRageSec:         15,
		SiegeRagePerSec:      0.1,
		WallRefundPct:        50,
		WallRefundClass:      75,
		RogueEverySec:        60,
		RogueStealPct:        0.05,
		RogueStealMax:        15,
		DruidEverySec:        45,
		DruidPantherSec:      10,
		DruidPantherSpeed:    4,
		DruidMaul:            0.8,
		WarriorEverySec:      80,
		WarriorSpeed:         6,
		WarriorPush:          2,
		MageEverySec:         90,
		MeteorCount:          6,
		MeteorDmg:            0.25,
		MeteorRadius:         1,
		MeteorBurn:           0.06,
		MeteorBurnSec:        3,
		BardEverySec:         75,
		BardMusicSec:         10,
		NoteDmg:              0.15,
		NoteRate:             0.7,
		NoteRange:            4,
		RogueStepEverySec:    60,
		RogueStepSec:         0.8,
		RogueStepDmg:         1.0,
		RogueFanDmg:          0.25,
		RogueFanRadius:       2,
		ClericEverySec:       60,
		PillarDmg:            0.4,
		PillarStunSec:        2,
		PillarRadius:         0.7,
		PyroEverySec:         75,
		PathFireDmg:          0.25,
		PathFireBurn:         0.04,
		PathFireBurnSec:      4,
		BossNetImmuneSec:     6,
		RangerEverySec:       30,
		WarlockEverySec:      60,
		WarlockCurseSec:      6,
		WarlockCurseAmp:      0.15,
		RangerRangeBonus:     1,
		CritBase:             0.10,
		CritPerBoss:          0.05,
		CritMax:              0.70,
		CritMult:             2,
		ClericShrineDmg:      0.25,
		ClericShrineStun:     0.5,
		PyroGateEverySec:     70,
		PyroGateSec:          15,
		BearEverySec:         60,
		BearSpeed:            4,
		BearFightSec:         10,
		BearReach:            0.9,
		BearDPS:              0.06,
		MageFeastEverySec:    120,
		ShamanWolfEverySec:   50,
		WolfSpeed:            7,
		WolfDmg:              0.3,
		PactEverySec:         90,
		PactWallsPct:         0.08,
		PactBoost:            0.35,
		PactSec:              10,
		PactMinWalls:         0.3,
		ShamanEverySec:       70,
		QuakeStunSec:         1.5,
		CurseShardPct:        0.5,
		CurseSoulChance:      0.05,
		ArrowRainDmg:         0.36,
		ArrowRainRadius:      2,
		TrapBleed:            0.24,
		TrapBleedSec:         4,
		TrapHealCut:          0.5,
		TrapHealCutSec:       3,
		TrapAssistPct:        0.5,
		RewardGrowth:         0.15,
		Drinks: map[string]DrinkSpec{
			"ale":    {Name: "Эль", Price: 10, Mugs: 1, Boost: 1.5, After: 0.5, Sec: 20},
			"mead":   {Name: "Медовуха", Price: 25, Mugs: 2, Boost: 2, After: 0.5, Sec: 30},
			"potion": {Name: "Чёрное зелье", Price: 60, Mugs: 4, Boost: 1.5, Sec: 20},
		},
		DrunkDecaySec:    90,
		PassOutMugs:      8,
		PassOutSec:       90,
		DiceTables:       2,
		DiceMinStake:     10,
		DiceWaitSec:      120,
		DiceRollSec:      3,
		SoulsGoldPct:     0.05,
		SoulsPerDamage:   0.004,
		SoulsPerKill:     0.2,
		SoulsGameCap:     80,
		SoulsPerMemecoin: 10,
		MemeMax:          50,
		Rewards: []RewardSpec{
			{Title: "HTD: Мешок золота", Action: "gold", Amount: 300},
			{Title: "Починить стены", Action: "repair", Amount: 200},
			{Title: "Дождь стрел", Action: "strike", Amount: 50},
			{Title: "Выпивка за мой счёт", Action: "round"},
		},
		BitsGold:           8,
		SubGold:            400,
		GiftRecipientGold:  150,
		DonationGoldPerRub: 10,
		CurrencyRub:        map[string]float64{"RUB": 1, "USD": 90, "EUR": 100, "UAH": 2.2, "KZT": 0.18, "BYN": 28},
		TreasuryGoldPerSec: 40,
		IgnoreUsers:        []string{"nightbot", "streamelements", "streamlabs", "moobot", "fossabot", "wizebot", "sery_bot", "soundalerts", "hasturyan"},
		BaseHP:             30,
		HPGrowth:           1.16,
		ChatterHPCoef:      0.25,
		BaseCount:          6,
		CountPerWave:       2,
		BossHPMult:         10,
		BossEvery:          5,
		EnemyRegen:         0.002,
		SdfChance:          0.2,
		SdfFromWave:        10,
		PathCurve:          0.5,
		EventChance:        0.15,
		EventFromWave:      3,
		Enemies: map[string]EnemySpec{
			"skeleton":      {Name: "Скелет", HPMult: 1, Speed: 1.1, WallDPS: 6, Bounty: 4},
			"ghoul":         {Name: "Гуль", HPMult: 0.6, Speed: 1.9, WallDPS: 4, Bounty: 3},
			"cultist":       {Name: "Культист", HPMult: 0.9, Speed: 1.0, WallDPS: 5, Bounty: 5, Heal: 0.04},
			"spider":        {Name: "Паук", HPMult: 0.5, Speed: 2.1, WallDPS: 4, Bounty: 3},
			"rat":           {Name: "Крыса", HPMult: 0.35, Speed: 2.4, WallDPS: 2, Bounty: 2},
			"troll":         {Name: "Тролль", HPMult: 2.2, Speed: 0.7, Armor: 0.35, WallDPS: 12, Bounty: 9, Regen: 0.01},
			"mummy":         {Name: "Мумия", HPMult: 1.5, Speed: 0.8, Armor: 0.5, WallDPS: 9, Bounty: 6},
			"ghost":         {Name: "Призрак", HPMult: 0.8, Speed: 1.3, Armor: 0.3, WallDPS: 5, Bounty: 5, Ghost: true},
			"necromancer":   {Name: "Некромант", HPMult: 0.9, Speed: 1.0, WallDPS: 5, Bounty: 6, Heal: 0.05},
			"iceskel":       {Name: "Ледяной скелет", HPMult: 1.3, Speed: 1.0, Armor: 0.45, WallDPS: 8, Bounty: 6, SlowImmune: true},
			"bomber":        {Name: "Гоблин-подрывник", HPMult: 0.6, Speed: 1.7, WallDPS: 4, Bounty: 4, Bomb: 0.5},
			"shieldbearer":  {Name: "Щитоносец", HPMult: 1.4, Speed: 0.8, Armor: 0.2, WallDPS: 8, Bounty: 6, Guard: 1.2},
			"slime":         {Name: "Слизень", HPMult: 1.4, Speed: 0.8, WallDPS: 6, Bounty: 4, Split: "slimelet", SplitN: 2},
			"slimelet":      {Name: "Слизнёнок", HPMult: 0.4, Speed: 1.1, WallDPS: 3, Bounty: 1},
			"bat":           {Name: "Летучая мышь", HPMult: 0.5, Speed: 1.5, WallDPS: 3, Bounty: 3, Fly: true},
			"goblin_shaman": {Name: "Гоблин-шаман", HPMult: 0.8, Speed: 1.0, WallDPS: 4, Bounty: 6, Dispel: 1.6},
			"frostspirit":   {Name: "Инеистый дух", HPMult: 0.7, Speed: 1.6, WallDPS: 5, Bounty: 5, Ghost: true, SlowImmune: true},

			"bone_knight":   {Name: "Костяной Рыцарь", HPMult: 1, Speed: 0.6, Armor: 0.3, WallDPS: 60, Bounty: 80, Boss: true},
			"cult_leader":   {Name: "Вожак Культа", HPMult: 1, Speed: 0.6, Armor: 0.1, WallDPS: 60, Bounty: 80, Boss: true},
			"spider_queen":  {Name: "Королева Пауков", HPMult: 1, Speed: 0.7, Armor: 0.2, WallDPS: 60, Bounty: 80, Boss: true},
			"cave_troll":    {Name: "Пещерный Тролль", HPMult: 1, Speed: 0.5, Armor: 0.4, WallDPS: 80, Bounty: 80, Regen: 0.01, Boss: true},
			"lich":          {Name: "Лич", HPMult: 1, Speed: 0.55, Armor: 0.2, WallDPS: 60, Bounty: 80, Boss: true},
			"ghost_king":    {Name: "Призрачный Король", HPMult: 1, Speed: 0.6, Armor: 0.1, WallDPS: 60, Bounty: 80, Ghost: true, Boss: true},
			"ice_golem":     {Name: "Ледяной Голем", HPMult: 1, Speed: 0.5, Armor: 0.5, WallDPS: 80, Bounty: 80, SlowImmune: true, Boss: true},
			"swamp_witch":   {Name: "Болотная Ведьма", HPMult: 1, Speed: 0.6, Armor: 0.1, WallDPS: 60, Bounty: 80, Boss: true},
			"hydra":         {Name: "Гидра", HPMult: 1.1, Speed: 0.5, Armor: 0.25, WallDPS: 70, Bounty: 90, Boss: true},
			"tax_collector": {Name: "Сборщик податей", HPMult: 0.4, Speed: 0.9, Armor: 0.3, Bounty: 20, Boss: true},
			"golden_golem":  {Name: "Золотой голем", HPMult: 0.35, Speed: 1.8, Armor: 0.2, Bounty: 10, Boss: true},
			"frost_demon":   {Name: "Демон Стужи", HPMult: 1, Speed: 0.65, Armor: 0.25, WallDPS: 70, Bounty: 80, SlowImmune: true, Boss: true},
			// A guest boss of any biome, deliberately unfair: see sdf.go.
			"sdf": {Name: "ЭссДиЭф", HPMult: 15, Speed: 0.22, Armor: 0.3, WallDPS: 150, Bounty: 300, SlowImmune: true, Boss: true},
		},
		Towers: map[string]TowerSpec{
			"arrow":  {Name: "Пост лучников", Role: RoleShooter, Cost: 50, Damage: 10, Range: 2.6, Rate: 1.4, ProjSpd: 10, NetEvery: 20, NetSec: 2, NetRadius: 0.7, UpgMult: 1.45, MaxLevel: 5},
			"magic":  {Name: "Тёмный обелиск", Role: RoleShooter, Cost: 80, Damage: 22, Range: 2.2, Rate: 0.6, ProjSpd: 7, Magic: true, StrikeEvery: 8, StrikeDmg: 1.5, StrikeRadius: 1, UpgMult: 1.45, MaxLevel: 5},
			"fire":   {Name: "Жаровня", Role: RoleShooter, Cost: 90, Damage: 14, Range: 2.0, Rate: 0.7, ProjSpd: 6, Splash: 0.9, BurnDPS: 5, BurnSec: 3, UpgMult: 1.45, MaxLevel: 5},
			"shrine": {Name: "Святилище", Role: RoleAura, Cost: 100, AuraRange: 1.6, AuraDmg: 0.15, WallHeal: 1.5, UpgMult: 1.5, MaxLevel: 5},
			"trap":   {Name: "Капканы", Role: RoleTrap, Cost: 40, Damage: 30, Slow: 0.5, SlowSec: 2, Charges: 9, UpgMult: 1.55, MaxLevel: 5},
			"drums":  {Name: "Барабаны войны", Role: RoleAura, Cost: 90, AuraRange: 3, AuraRate: 0.2, UpgMult: 1.5, MaxLevel: 5},
			"wall":   {Name: "Баррикада", Role: RoleWall, Cost: 60, HP: 250, UpgMult: 1.5, MaxLevel: 5},
			// The only real answer to ЭссДиЭф: shoots him anywhere on the map.
			"curse": {Name: "Алтарь проклятий", Role: RoleCurse, Cost: 90, Range: 2.6, CurseEvery: 4, CurseSec: 6, CurseAmp: 0.25, UpgMult: 1.25, MaxLevel: 5},
			"totem": {Name: "Тотем грозы", Role: RoleShooter, Cost: 85, Damage: 9, Range: 2.4, Rate: 0.8, Magic: true, Chain: 3, ChainRange: 1.6, ChainFalloff: 0.75, UpgMult: 1.52, MaxLevel: 5},
			"clown": {Name: "Клоун", Role: RoleShooter, Cost: 100, Range: 99, Rate: 1, ProjSpd: 9, Magic: true, Only: "sdf", BossPct: 0.005, UpgMult: 1.35, MaxLevel: 5},
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
