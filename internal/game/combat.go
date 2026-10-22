package game

import (
	"fmt"
	"maps"
	"math"
	"slices"
)

type Enemy struct {
	ID     int
	Kind   string
	HP     float64
	MaxHP  float64
	Dist   float64 // distance travelled along the route, in cells
	Pos    vec
	AtGate bool
	Title  string // a boss's name for the health bar

	dead      bool
	summoned  bool    // raised by a boss: worth almost no gold
	blocked   *Tower  // barricade it is hacking at
	siegeSec  float64 // seconds spent at barricades since it last walked
	rage      bool    // held too long: heals no more and hits the barricade harder
	slowLeft  float64 // seconds of slow left
	slowMult  float64
	burnLeft  float64
	burnDPS   float64
	burnOwner string
	traps     map[int]bool // traps already sprung by this enemy
	ability   float64      // boss ability cooldown
	shieldUp  bool

	// Traps (hotfix 2): bleeding, and healing cut while the cut is fresh.
	bleedLeft  float64
	bleedDPS   float64
	bleedOwner string
	healCut    float64 // seconds left of halved healing
	netLeft    float64 // seconds left tangled in a ballista net
	netImmune  float64 // a boss shakes off nets until this runs out
	curseLeft  float64 // seconds left cursed by a warlock's altar
	curseAmp   float64 // extra damage taken while cursed
	curseOwner string
	heads      int             // a hydra's heads
	nextCut    float64         // the share of health at which the hydra grows the next one
	stunLeft   float64         // seconds left stunned by a pillar of light
	helpers    map[string]bool // trap owners who hurt it: they share the kill

	// ЭссДиЭф (sdf.go).
	pause      float64 // stands still for a burp or a fart
	act        int     // 1 burp, 2 fart, while paused
	tricks     int
	offLeft    float64 // seconds left off the path
	offAmt     float64 // 0 on the path .. 1 fully off
	offset     vec
	tavernLeft float64            // seconds left robbing the tavern
	loot       map[string]int     // gold stolen from each chatter
	hurtBy     map[string]float64 // damage dealt to him, by chatter
}

type Tower struct {
	ID       int
	Kind     string
	Cell     Pos
	Owner    string
	Level    int
	Invested int
	Granted  int // the part of Invested paid from a grant: refunds of it are not the owner's
	// Built is false while the tower is a construction site. PendingCost is
	// the paid price of an upgrade a mercenary has not finished yet.
	Built       bool
	PendingCost int
	PendingUp   int     // levels the paid upgrade adds when the mercenary finishes it (!up C4 max: several)
	Progress    float64 // 0..1 of the current construction or upgrade
	HP, MaxHP   float64 // barricades
	Charges     int     // traps
	Disabled    float64 // seconds left webbed, stunned or frozen by a boss
	cooldown    float64
	strikeCD    float64 // magic: seconds to the next lightning strike
	netCD       float64 // ballista: seconds to the next net
	drink       float64 // mead: this building hits drink times as hard
	drinkLeft   float64
	repairing   bool    // a barricade waits for its mercenary to mend it
	repairCD    float64 // seconds until the barricade can be mended again
	holdGold    float64 // gold earned for holding enemies, not paid out yet
	holdPaid    int     // gold paid for holding during this wave
	shrineGold  float64 // a shrine's share of kills, not paid out yet
	curseCD     float64 // seconds until the altar curses again
	hexLeft     float64 // seconds left as a frog (the swamp witch)
	shrineWave  int     // gold a shrine paid its owner this wave, for the toast
}

type Projectile struct {
	ID      int
	Kind    string
	Pos     vec
	Target  int
	Aim     vec // last known target position
	Damage  float64
	Magic   bool
	Speed   float64
	Owner   string
	Splash  float64
	BurnDPS float64
	BurnSec float64
	Assist  []string // owners of the drums that sped this shot up
}

// levelMult is how much a building's main stat grows with its level.
func (g *Game) levelMult(t *Tower) float64 {
	return math.Pow(g.Cfg.Towers[t.Kind].UpgMult, float64(t.Level-1))
}

// auraMult is how much an aura grows with its level: +50% per level.
func auraMult(t *Tower) float64 { return 1 + 0.5*float64(t.Level-1) }

// readyTower sets per-building state when construction or an upgrade ends.
func (g *Game) readyTower(t *Tower) {
	spec := g.Cfg.Towers[t.Kind]
	bonus := g.classBonus(t.Owner, t.Kind)
	switch spec.Role {
	case RoleWall:
		t.MaxHP = spec.HP * g.levelMult(t) * bonus
		t.HP = t.MaxHP
	case RoleTrap:
		t.Charges = spec.Charges + spec.Charges/3*(t.Level-1) // a third more per level
	}
}

func (g *Game) active(t *Tower) bool { return t.Built && t.Disabled <= 0 }

// StartWave skips whatever is left of the build phase.
func (g *Game) StartWave() {
	if g.Phase != PhaseBuild {
		return
	}
	g.Wave++
	g.Phase = PhaseWave
	for _, t := range g.Towers {
		t.holdPaid = 0
	}
	g.Timer = 0
	sq := math.Sqrt(max(g.difficultyChatters(), 1))
	g.waveHP = g.Cfg.BaseHP * math.Pow(g.Cfg.HPGrowth, float64(g.Wave)) * (1 + g.Cfg.ChatterHPCoef*sq)
	count := g.Cfg.BaseCount + g.Cfg.CountPerWave*g.Wave + int(sq)

	pool := biomeEnemies[g.Map.Biome]
	g.queue = g.queue[:0]
	for i := 0; i < count; i++ {
		kind := pickEnemy(g.rng.Float64(), pool, g.Wave)
		g.queue = append(g.queue, spawn{kind, g.waveHP * g.Cfg.Enemies[kind].HPMult})
	}
	boss := ""
	bossWave := g.Cfg.BossEvery > 0 && g.Wave%g.Cfg.BossEvery == 0
	guest := g.rollGuest(bossWave)
	if bossWave {
		pair := biomeBosses[g.Map.Biome]
		boss = pair[(g.Wave/g.Cfg.BossEvery-1)%len(pair)]
		if g.Wave >= g.Cfg.SdfFromWave && g.rng.Float64() < g.Cfg.SdfChance {
			boss = sdfKind // a guest from nowhere, in any biome
		}
		g.queue = append(g.queue, spawn{boss, g.waveHP * g.Cfg.BossHPMult * g.Cfg.Enemies[boss].HPMult})
	}
	g.spawnTimer = 0
	g.emit(Event{Kind: "waveStart", Amount: g.Wave, Text: boss, User: guest})
}

func (g *Game) endWave() {
	g.Phase = PhaseBuild
	g.Timer = g.Cfg.BuildPhaseSec
	g.packClowns()
	g.shrinesMendWalls()
	g.shrineToasts()
	bonus := int(math.Round(float64(10+2*g.Wave) * g.rewardMult))
	if g.Cfg.BossEvery > 0 && g.Wave%g.Cfg.BossEvery == 0 {
		// A boss wave held: from now on every reward is worth more.
		g.rewardMult *= 1 + g.Cfg.RewardGrowth
		g.emit(Event{Kind: "rewardUp", Amount: int(math.Round((g.rewardMult - 1) * 100))})
		if now := g.limitAfter(g.Wave); now > g.limitAfter(g.Wave-1) {
			g.emit(Event{Kind: "limitUp", Amount: now})
			g.reply("", fmt.Sprintf("босс повержен: лимит построек вырос до %d", now))
		}
	}
	owners := map[string]bool{}
	for _, t := range g.Towers {
		owners[t.Owner] = true
		if t.Built && g.Cfg.Towers[t.Kind].Role == RoleTrap {
			g.readyTower(t) // traps are re-armed between waves
		}
	}
	for name := range owners {
		if c := g.Chatters[name]; c != nil {
			g.addGold(c, bonus)
		}
	}
	g.emit(Event{Kind: "waveCleared", Amount: g.Wave})
}

func (g *Game) tickSpawns(dt float64) {
	if len(g.queue) == 0 {
		return
	}
	g.spawnTimer -= dt
	if g.spawnTimer > 0 {
		return
	}
	s := g.queue[0]
	g.queue = g.queue[1:]
	g.spawnTimer = g.Cfg.SpawnIntervalSec
	if g.Cfg.Enemies[s.kind].Boss {
		e := g.spawnBoss(s.kind, s.hp)
		if isGuest(s.kind) {
			g.emit(Event{Kind: "guest", Text: s.kind, X: e.Pos.X, Y: e.Pos.Y})
		}
	} else {
		g.spawnEnemy(s.kind, s.hp, 0)
	}
}

// spawnBoss brings in a boss under its own name.
func (g *Game) spawnBoss(kind string, hp float64) *Enemy {
	e := g.spawnEnemy(kind, hp, 0)
	e.ability = 3 // let it walk into view before the first trick
	e.Title = g.Cfg.Enemies[kind].Name
	return e
}

func (g *Game) spawnEnemy(kind string, hp, dist float64) *Enemy {
	e := &Enemy{ID: g.id(), Kind: kind, HP: hp, MaxHP: hp, Dist: dist, Pos: g.routePos(dist), slowMult: 1}
	g.Enemies = append(g.Enemies, e)
	return e
}

func (g *Game) speedMult() float64 {
	m := 1.0
	if g.Map.Biome == BiomeIce {
		m = 1.15
	}
	if g.rushLeft > 0 {
		m *= 1.3 // the frost demon's icy path
	}
	return m
}

func (g *Game) tickEnemies(dt float64) {
	g.rushLeft = max(0, g.rushLeft-dt)
	for _, e := range g.Enemies {
		if e.dead {
			continue
		}
		spec := g.Cfg.Enemies[e.Kind]
		if e.burnLeft > 0 {
			e.burnLeft -= dt
			g.damage(e, e.burnDPS*dt, e.burnOwner, true)
			if e.dead {
				continue
			}
		}
		if e.bleedLeft > 0 {
			e.bleedLeft -= dt
			g.damage(e, e.bleedDPS*dt, e.bleedOwner, true)
			if e.dead {
				continue
			}
		}
		e.healCut = max(0, e.healCut-dt)
		e.netLeft = max(0, e.netLeft-dt)
		e.netImmune = max(0, e.netImmune-dt)
		e.curseLeft = max(0, e.curseLeft-dt)
		e.stunLeft = max(0, e.stunLeft-dt)
		// Every enemy slowly closes its wounds; trolls much faster.
		g.heal(e, (g.Cfg.EnemyRegen+spec.Regen)*e.MaxHP*dt)
		if spec.Heal > 0 {
			g.healAround(e, spec.Heal*dt)
		}
		if spec.Boss {
			g.tickBoss(e, dt)
		}
		if spec.Dispel > 0 {
			g.tickDispel(e, spec, dt)
		}
		if e.Kind == "hydra" {
			g.tickHydra(e)
		}
		if e.slowLeft > 0 {
			e.slowLeft -= dt
		} else {
			e.slowMult = 1
		}
		speed, blows := e.fury()
		switch {
		case e.AtGate && isGuest(e.Kind):
			g.guestAtGate(e)
			continue
		case e.AtGate:
			// The tavern door enrages too: a crowd at the gate cannot sit
			// there healing forever.
			g.Walls -= spec.WallDPS * blows * g.siege(e, dt) * dt
			// Guards on the walls shoot back, so a leak costs hit points but
			// is not the end by itself; a boss at the gate still is.
			g.damage(e, g.Cfg.GateDPS*dt, "", true)
		case e.netLeft > 0 || e.stunLeft > 0: // netted or stunned: neither walks nor hacks
		case e.blocked != nil:
			g.hackBarricade(e, spec, blows, dt)
		case e.pause > 0 || e.tavernLeft > 0: // ЭссДиЭф busy with a trick
		case spec.Fly:
			g.fly(e, spec.Speed*speed*g.speedMult()*e.slowMult*dt)
		default:
			g.move(e, spec, spec.Speed*speed*g.speedMult()*e.slowMult*dt)
		}
		if spec.Fly {
			e.Pos = g.flyPos(e) // a shove or a step back moves it along its line
		}
		if e.Kind == sdfKind {
			g.tickSdf(e, dt)
		}
	}
}

// move advances an enemy, stopping at barricades and springing traps.
func (g *Game) move(e *Enemy, spec EnemySpec, step float64) {
	e.siegeSec, e.rage = 0, false // walking again cools it down
	from, to := e.Dist, e.Dist+step
	for i, p := range g.Map.Path {
		t := g.Towers[p]
		if t == nil || !t.Built || spec.Ghost {
			continue
		}
		c := g.cellDist[i]
		switch g.Cfg.Towers[t.Kind].Role {
		case RoleWall:
			if edge := c - 0.5; from < edge && to >= edge {
				to = edge - 0.01
				e.blocked = t
			}
		case RoleTrap:
			if from < c && to >= c && t.Disabled <= 0 && t.Charges > 0 && !e.traps[t.ID] {
				g.springTrap(e, t, spec)
			}
		}
		if e.blocked != nil {
			break
		}
	}
	e.Dist = min(to, g.total)
	if e.Dist >= g.total {
		e.AtGate = true
	}
	if e.blocked != nil && spec.Bomb > 0 {
		g.blowUp(e, spec)
	}
	e.Pos = g.routePos(e.Dist)
}

func (g *Game) springTrap(e *Enemy, t *Tower, spec EnemySpec) {
	ts := g.Cfg.Towers[t.Kind]
	if e.traps == nil {
		e.traps = map[int]bool{}
	}
	e.traps[t.ID] = true
	t.Charges--
	// A trap slows everyone, even those immune to frost: each charge is one
	// snap with its full damage.
	e.slowMult = ts.Slow
	e.slowLeft = ts.SlowSec
	dmg := ts.Damage * g.levelMult(t) * g.classBonus(t.Owner, t.Kind) * g.drinkMult(t)
	// The cut keeps bleeding, harder from a better trap, and halves healing
	// for a while; whoever set the trap shares the kill.
	e.bleedLeft, e.bleedDPS, e.bleedOwner = g.Cfg.TrapBleedSec, dmg*g.Cfg.TrapBleed, t.Owner
	e.healCut = g.Cfg.TrapHealCutSec
	if e.helpers == nil {
		e.helpers = map[string]bool{}
	}
	e.helpers[t.Owner] = true
	g.damage(e, dmg, t.Owner, false)
	cp := center(t.Cell)
	g.emit(Event{Kind: "trap", X: cp.X, Y: cp.Y, Amount: t.Charges})
}

func (g *Game) hackBarricade(e *Enemy, spec EnemySpec, blows, dt float64) {
	t := e.blocked
	if g.Towers[t.Cell] != t {
		e.blocked = nil // sold or already broken
		return
	}
	t.HP -= spec.WallDPS * blows * g.siege(e, dt) * dt
	g.payHold(t, dt)
	if t.HP <= 0 {
		g.breakWall(t)
	}
}

// siege counts the time an enemy spends hacking at a barricade or at the
// tavern door and returns how much harder it hits. Held too long, it flies
// into a rage: it stops healing (see heal) and hits harder every second, so
// neither barricades nor the door can stall a wave for good.
func (g *Game) siege(e *Enemy, dt float64) float64 {
	e.siegeSec += dt
	over := e.siegeSec - g.Cfg.SiegeRageSec
	if g.Cfg.SiegeRageSec <= 0 || over < 0 {
		return 1
	}
	if !e.rage {
		e.rage = true
		g.emit(Event{Kind: "rage", X: e.Pos.X, Y: e.Pos.Y})
	}
	return 1 + g.Cfg.SiegeRagePerSec*over
}

// breakWall takes down a barricade whose hit points ran out.
func (g *Game) breakWall(t *Tower) {
	g.removeTower(t)
	cp := center(t.Cell)
	g.emit(Event{Kind: "wallBroken", X: cp.X, Y: cp.Y, Amount: g.wallRefund(t)})
	for _, o := range g.Enemies {
		if o.blocked == t {
			o.blocked = nil
		}
	}
}

func (g *Game) healAround(healer *Enemy, share float64) {
	for _, o := range g.Enemies {
		if o != healer && !o.dead && o.Pos.sub(healer.Pos).len() <= 1.5 {
			g.heal(o, share*o.MaxHP)
		}
	}
}

// auras returns the damage and fire rate multipliers neighbouring shrines and
// drums give to t.
func (g *Game) auras(t *Tower) (dmg, rate float64) {
	dmg, rate = 1, 1
	tc := center(t.Cell)
	for _, a := range g.Towers {
		spec := g.Cfg.Towers[a.Kind]
		if a == t || spec.Role != RoleAura || !g.active(a) || center(a.Cell).sub(tc).len() > g.auraRange(a, spec) {
			continue
		}
		m := auraMult(a) * g.classBonus(a.Owner, a.Kind)
		dmg += spec.AuraDmg * m
		rate += spec.AuraRate * m
	}
	return dmg, rate
}

func (g *Game) tickTowers(dt float64) {
	for _, t := range g.Towers {
		t.repairCD = max(0, t.repairCD-dt)
		t.hexLeft = max(0, t.hexLeft-dt)
		if t.Disabled > 0 {
			t.Disabled -= dt
			continue
		}
		if !t.Built {
			continue
		}
		spec := g.Cfg.Towers[t.Kind]
		if spec.WallHeal > 0 {
			heal := spec.WallHeal * float64(t.Level) * g.classBonus(t.Owner, t.Kind) * dt
			g.Walls = min(g.Walls+heal, g.Cfg.WallsHP)
		}
		if spec.AuraRate > 0 {
			g.tickNotes(t, dt) // a bard's drums shoot notes while he plays
		}
		if spec.Role == RoleCurse {
			g.tickCurse(t, spec, dt)
			continue
		}
		if spec.Role != RoleShooter {
			continue
		}
		if spec.NetEvery > 0 {
			g.tickNet(t, spec, dt)
		}
		if spec.StrikeEvery > 0 {
			g.tickStrike(t, spec, dt)
		}
		t.cooldown -= dt
		if t.cooldown > 0 {
			continue
		}
		tc := center(t.Cell)
		var target *Enemy
		for _, e := range g.Enemies {
			if spec.Only != "" && e.Kind != spec.Only {
				continue // clowns see nobody but ЭссДиЭф
			}
			if !e.dead && e.Pos.sub(tc).len() <= spec.Range && (target == nil || e.Dist > target.Dist) {
				target = e
			}
		}
		if target == nil {
			continue
		}
		dmgAura, rateAura := g.auras(t)
		t.cooldown = 1 / (spec.Rate * rateAura)
		mult := g.levelMult(t) * g.classBonus(t.Owner, t.Kind) * dmgAura * g.drinkMult(t)
		damage := spec.Damage * mult
		if spec.BossPct > 0 {
			damage = spec.BossPct * target.MaxHP * mult
		}
		if spec.Chain > 0 {
			g.chainZap(t, spec, target, damage) // lightning needs no flight time
			continue
		}
		g.Projectiles = append(g.Projectiles, &Projectile{
			ID: g.id(), Kind: t.Kind, Pos: tc, Target: target.ID, Aim: target.Pos,
			Damage: damage, Magic: spec.Magic, Speed: spec.ProjSpd, Owner: t.Owner,
			Splash: spec.Splash, BurnDPS: spec.BurnDPS * mult, BurnSec: spec.BurnSec,
			Assist: g.drummers(t),
		})
	}
}

func (g *Game) enemy(id int) *Enemy {
	for _, e := range g.Enemies {
		if e.ID == id && !e.dead {
			return e
		}
	}
	return nil
}

func (g *Game) tickProjectiles(dt float64) {
	alive := g.Projectiles[:0]
	for _, p := range g.Projectiles {
		target := g.enemy(p.Target)
		if target != nil {
			p.Aim = target.Pos
		}
		d := p.Aim.sub(p.Pos)
		step := p.Speed * dt
		if d.len() > step {
			p.Pos = vec{p.Pos.X + d.X/d.len()*step, p.Pos.Y + d.Y/d.len()*step}
			alive = append(alive, p)
			continue
		}
		g.assist = p.Assist // drummers share the kills of this shot
		switch {
		case p.Splash > 0: // fire hits everything around where it lands
			for _, e := range g.Enemies {
				if !e.dead && e.Pos.sub(p.Aim).len() <= p.Splash {
					g.burn(e, p)
					g.damage(e, p.Damage, p.Owner, p.Magic)
				}
			}
			g.emit(Event{Kind: "blast", X: p.Aim.X, Y: p.Aim.Y})
		case target != nil:
			g.damage(target, p.Damage, p.Owner, p.Magic)
		}
		g.assist = nil
	}
	g.Projectiles = alive
}

func (g *Game) burn(e *Enemy, p *Projectile) {
	if p.BurnDPS <= 0 {
		return
	}
	e.burnLeft, e.burnDPS, e.burnOwner = p.BurnSec, p.BurnDPS, p.Owner
}

// damage hurts an enemy, credits the owner and handles the kill. Dead
// enemies are swept at the end of the tick so loops over Enemies stay valid.
func (g *Game) damage(e *Enemy, dmg float64, owner string, ignoreArmor bool) {
	if e.dead || dmg <= 0 {
		return
	}
	physical := !ignoreArmor
	if e.curseLeft > 0 {
		dmg *= 1 + e.curseAmp // a cursed enemy takes more from everything
	} else if physical {
		dmg *= 1 - g.Cfg.Enemies[e.Kind].Armor // and has no armor left
	}
	if physical && g.guarded(e) {
		dmg *= 1 - guardCut
	}
	if e.shieldUp {
		dmg *= 0.2
	}
	c := g.Chatters[owner]
	if c != nil {
		c.GameDamage += min(dmg, e.HP) // overkill does not count
		if tracksDamage(e.Kind) {
			if e.hurtBy == nil {
				e.hurtBy = map[string]float64{}
			}
			e.hurtBy[owner] += min(dmg, e.HP)
		}
	}
	e.HP -= dmg
	if e.HP > 0 {
		return
	}
	e.dead = true
	if s := g.Cfg.Enemies[e.Kind]; s.Split != "" && s.SplitN > 0 {
		g.split(e, s)
	}
	if e.loot != nil {
		g.returnLoot(e)
	}
	if e.hurtBy != nil && e.Kind == sdfKind {
		g.crownClown(e)
	}
	if isGuest(e.Kind) {
		g.guestKilled(e)
	}
	bounty := int(math.Round(float64(g.Cfg.Enemies[e.Kind].Bounty) * g.rewardMult))
	if e.summoned {
		bounty = 1
	}
	if c == nil {
		bounty = 0 // killed by the guards or a stray shot: nobody is paid
	}
	if c != nil {
		c.Kills++
		c.GameKills++
		g.onKill(owner, c, e)
		bounty = g.addGold(c, bounty)
	}
	g.payAssists(owner, e)
	g.payCurse(e)
	g.emit(Event{Kind: "kill", User: owner, X: e.Pos.X, Y: e.Pos.Y, Amount: bounty, Text: e.Kind})
}

func (g *Game) sweepDead() {
	g.Enemies = slices.DeleteFunc(g.Enemies, func(e *Enemy) bool { return e.dead })
}

// drummers lists the other chatters whose drums speed t up: they get a
// share of every kill it makes. Drums help anyone's buildings.
func (g *Game) drummers(t *Tower) []string {
	var out []string
	tc := center(t.Cell)
	for _, a := range g.Towers {
		spec := g.Cfg.Towers[a.Kind]
		if spec.AuraRate <= 0 || !g.active(a) || a.Owner == t.Owner || center(a.Cell).sub(tc).len() > g.auraRange(a, spec) {
			continue
		}
		if !slices.Contains(out, a.Owner) {
			out = append(out, a.Owner)
		}
	}
	return out
}

// payAssists gives each drummer behind the killing shot a share of the
// bounty on top of what the killer got.
func (g *Game) payAssists(killer string, e *Enemy) {
	if e.summoned {
		return
	}
	g.payShrines(killer, e)
	// Drummers behind the killing shot and trap owners who cut it share it.
	pct := map[string]float64{}
	for _, key := range g.assist {
		pct[key] = g.Cfg.DrumAssistPct
	}
	for key := range e.helpers {
		pct[key] = max(pct[key], g.Cfg.TrapAssistPct)
	}
	bounty := float64(g.Cfg.Enemies[e.Kind].Bounty) * g.rewardMult
	for _, key := range slices.Sorted(maps.Keys(pct)) {
		share := int(math.Round(bounty * pct[key]))
		if c := g.Chatters[key]; c != nil && key != killer && key != StreamerName && share > 0 {
			got := g.addGold(c, share)
			g.emit(Event{Kind: "assist", User: c.Name, X: e.Pos.X, Y: e.Pos.Y, Amount: got})
		}
	}
}

// payShrines gives every shrine a part of a kill's bounty in proportion to
// how much of the killing damage its buff made: a +15% shrine under a total
// multiplier of 1.3 made 15/130 of it. Shrines buff the whole map, so they
// share in every kill but the ones their own owner makes. Fractions add up
// and are paid out whole.
func (g *Game) payShrines(killer string, e *Enemy) {
	if g.Cfg.ShrineAssistPct <= 0 || g.Chatters[killer] == nil {
		return
	}
	type buff struct {
		t *Tower
		v float64
	}
	var shrines []buff
	total := 1.0
	for _, t := range g.Towers {
		spec := g.Cfg.Towers[t.Kind]
		if spec.AuraDmg > 0 && g.active(t) {
			v := spec.AuraDmg * auraMult(t) * g.classBonus(t.Owner, t.Kind)
			shrines = append(shrines, buff{t, v})
			total += v
		}
	}
	bounty := float64(g.Cfg.Enemies[e.Kind].Bounty) * g.rewardMult
	for _, s := range shrines {
		c := g.Chatters[s.t.Owner]
		if c == nil || s.t.Owner == killer || s.t.Owner == StreamerName {
			continue
		}
		s.t.shrineGold += bounty * g.Cfg.ShrineAssistPct * s.v / total
		if n := int(s.t.shrineGold); n > 0 {
			s.t.shrineGold -= float64(n)
			s.t.shrineWave += g.addGold(c, n)
		}
	}
}

// shrineToasts tells, after a wave, what each owner's shrines earned.
func (g *Game) shrineToasts() {
	earned := map[string]int{}
	for _, t := range g.Towers {
		if t.shrineWave > 0 {
			earned[t.Owner] += t.shrineWave
			t.shrineWave = 0
		}
	}
	for _, key := range slices.Sorted(maps.Keys(earned)) {
		if c := g.Chatters[key]; c != nil {
			g.emit(Event{Kind: "shrineGold", User: c.Name, Amount: earned[key]})
		}
	}
}

// heal restores hit points, halved while a trap's cut is fresh.
func (g *Game) heal(e *Enemy, hp float64) {
	if e.rage {
		return // too busy smashing a barricade to mend
	}
	if e.healCut > 0 || e.burnLeft > 0 { // fresh cuts and every burn halve healing
		hp *= 1 - g.Cfg.TrapHealCut
	}
	e.HP = min(e.MaxHP, e.HP+hp)
}

// tickStrike lets a magic tower call lightning on the thickest crowd in its
// range. The bolt comes more often and hits harder with every level.
func (g *Game) tickStrike(t *Tower, spec TowerSpec, dt float64) {
	t.strikeCD -= dt
	if t.strikeCD > 0 {
		return
	}
	tc := center(t.Cell)
	var at *Enemy
	best := 0
	for _, e := range g.Enemies {
		if e.dead || e.Pos.sub(tc).len() > spec.Range {
			continue
		}
		n := 0
		for _, o := range g.Enemies {
			if !o.dead && o.Pos.sub(e.Pos).len() <= spec.StrikeRadius {
				n++
			}
		}
		if n > best {
			at, best = e, n
		}
	}
	if at == nil {
		return // waits charged until someone comes
	}
	t.strikeCD = spec.StrikeEvery / (1 + 0.25*float64(t.Level-1))
	dmgAura, _ := g.auras(t)
	dmg := spec.Damage * spec.StrikeDmg * g.levelMult(t) * g.classBonus(t.Owner, t.Kind) * dmgAura * g.drinkMult(t)
	p := at.Pos
	g.assist = g.drummers(t)
	for _, e := range g.Enemies {
		if !e.dead && e.Pos.sub(p).len() <= spec.StrikeRadius {
			g.damage(e, dmg, t.Owner, true)
		}
	}
	g.assist = nil
	g.emit(Event{Kind: "lightning", X: p.X, Y: p.Y, Text: t.Owner})
}

// tickNet lets a ballista throw a net at the enemy furthest along in range:
// everyone within NetRadius of it stops for NetSec, unhurt. Better
// ballistas throw more often.
func (g *Game) tickNet(t *Tower, spec TowerSpec, dt float64) {
	t.netCD -= dt
	if t.netCD > 0 {
		return
	}
	tc := center(t.Cell)
	var at *Enemy
	for _, e := range g.Enemies {
		if !e.dead && e.netLeft <= 0 && e.netImmune <= 0 && e.Pos.sub(tc).len() <= spec.Range && (at == nil || e.Dist > at.Dist) {
			at = e
		}
	}
	if at == nil {
		return // keeps the net ready until someone comes
	}
	t.netCD = spec.NetEvery / (1 + 0.25*float64(t.Level-1))
	p := at.Pos
	for _, e := range g.Enemies {
		if !e.dead && e.netImmune <= 0 && e.Pos.sub(p).len() <= spec.NetRadius {
			e.netLeft = spec.NetSec
			// A boss tears free and stays wary for a while, so a row of
			// ballistas cannot hold him for good.
			if g.Cfg.Enemies[e.Kind].Boss {
				e.netImmune = spec.NetSec + g.Cfg.BossNetImmuneSec
			}
		}
	}
	g.emit(Event{Kind: "net", X: p.X, Y: p.Y, Text: t.Owner})
}

// TowerLimit is how many buildings a player may own right now. Lowering it
// never takes buildings away: it only stops new ones.
func (g *Game) TowerLimit() int {
	held := g.Wave
	if g.Phase == PhaseWave {
		held-- // the wave in progress is not held yet
	}
	n := g.limitAfter(held)
	if a := g.ActiveChatters(); a > 0 && a <= g.Cfg.SmallTeamSize {
		n += g.Cfg.SmallTeamBonus
	}
	return n
}

// limitAfter is the base limit once this many waves are held: one more
// after every boss wave, up to TowerLimitMax.
func (g *Game) limitAfter(held int) int {
	n := g.Cfg.TowerLimit
	if g.Cfg.BossEvery > 0 && held > 0 {
		n += held / g.Cfg.BossEvery
	}
	return min(n, max(g.Cfg.TowerLimitMax, g.Cfg.TowerLimit))
}
