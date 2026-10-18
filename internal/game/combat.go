package game

import (
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
	Title  string // bosses carry a chatter's name

	dead      bool
	summoned  bool    // raised by a boss: worth almost no gold
	blocked   *Tower  // barricade it is hacking at
	slowLeft  float64 // seconds of slow left
	slowMult  float64
	burnLeft  float64
	burnDPS   float64
	burnOwner string
	traps     map[int]bool // traps already sprung by this enemy
	ability   float64      // boss ability cooldown
	shieldUp  bool

	// ЭссДиЭф (sdf.go).
	pause      float64 // stands still for a burp or a fart
	act        int     // 1 burp, 2 fart, while paused
	tricks     int
	offLeft    float64 // seconds left off the path
	offAmt     float64 // 0 on the path .. 1 fully off
	offset     vec
	tavernLeft float64        // seconds left robbing the tavern
	loot       map[string]int // gold stolen from each chatter
}

type Tower struct {
	ID       int
	Kind     string
	Cell     Pos
	Owner    string
	Level    int
	Invested int
	// Built is false while the tower is a construction site. PendingCost is
	// the paid price of an upgrade a mercenary has not finished yet.
	Built       bool
	PendingCost int
	Progress    float64 // 0..1 of the current construction or upgrade
	HP, MaxHP   float64 // barricades
	Charges     int     // traps
	Disabled    float64 // seconds left webbed, stunned or frozen by a boss
	cooldown    float64
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
		t.Charges = spec.Charges + 2*(t.Level-1)
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
	g.Timer = 0
	n := max(g.ActiveChatters(), 1)
	sq := math.Sqrt(float64(n))
	g.waveHP = g.Cfg.BaseHP * math.Pow(g.Cfg.HPGrowth, float64(g.Wave)) * (1 + g.Cfg.ChatterHPCoef*sq)
	count := g.Cfg.BaseCount + g.Cfg.CountPerWave*g.Wave + int(sq)

	pool := biomeEnemies[g.Map.Biome]
	g.queue = g.queue[:0]
	for i := 0; i < count; i++ {
		kind := pickEnemy(g.rng.Float64(), pool, g.Wave)
		g.queue = append(g.queue, spawn{kind, g.waveHP * g.Cfg.Enemies[kind].HPMult})
	}
	boss := ""
	if g.Cfg.BossEvery > 0 && g.Wave%g.Cfg.BossEvery == 0 {
		pair := biomeBosses[g.Map.Biome]
		boss = pair[(g.Wave/g.Cfg.BossEvery-1)%len(pair)]
		if g.rng.Float64() < g.Cfg.SdfChance {
			boss = sdfKind // a guest from nowhere, in any biome
		}
		g.queue = append(g.queue, spawn{boss, g.waveHP * g.Cfg.BossHPMult * g.Cfg.Enemies[boss].HPMult})
	}
	g.spawnTimer = 0
	g.emit(Event{Kind: "waveStart", Amount: g.Wave, Text: boss})
}

func (g *Game) endWave() {
	g.Phase = PhaseBuild
	g.Timer = g.Cfg.BuildPhaseSec
	g.packClowns()
	bonus := 10 + 2*g.Wave
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
		g.spawnBoss(s.kind, s.hp)
	} else {
		g.spawnEnemy(s.kind, s.hp, 0)
	}
}

// spawnBoss brings in a boss named after a random chatter in the tavern.
func (g *Game) spawnBoss(kind string, hp float64) *Enemy {
	e := g.spawnEnemy(kind, hp, 0)
	e.ability = 3 // let it walk into view before the first trick
	e.Title = g.Cfg.Enemies[kind].Name
	if name := g.randomActiveName(); name != "" {
		e.Title += " " + name
	}
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
		if spec.Regen > 0 {
			e.HP = min(e.MaxHP, e.HP+spec.Regen*e.MaxHP*dt)
		}
		if spec.Heal > 0 {
			g.healAround(e, spec.Heal*dt)
		}
		if spec.Boss {
			g.tickBoss(e, dt)
		}
		if e.slowLeft > 0 {
			e.slowLeft -= dt
		} else {
			e.slowMult = 1
		}
		switch {
		case e.AtGate:
			g.Walls -= spec.WallDPS * dt
			// Guards on the walls shoot back, so a leak costs hit points but
			// is not the end by itself; a boss at the gate still is.
			g.damage(e, g.Cfg.GateDPS*dt, "", true)
		case e.blocked != nil:
			g.hackBarricade(e, spec, dt)
		case e.pause > 0 || e.tavernLeft > 0: // ЭссДиЭф busy with a trick
		default:
			g.move(e, spec, spec.Speed*g.speedMult()*e.slowMult*dt)
		}
		if e.Kind == sdfKind {
			g.tickSdf(e, dt)
		}
	}
}

// move advances an enemy, stopping at barricades and springing traps.
func (g *Game) move(e *Enemy, spec EnemySpec, step float64) {
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
	e.Pos = g.routePos(e.Dist)
}

func (g *Game) springTrap(e *Enemy, t *Tower, spec EnemySpec) {
	ts := g.Cfg.Towers[t.Kind]
	if e.traps == nil {
		e.traps = map[int]bool{}
	}
	e.traps[t.ID] = true
	t.Charges--
	if !spec.SlowImmune {
		e.slowMult = ts.Slow
		e.slowLeft = ts.SlowSec
	}
	g.damage(e, ts.Damage*g.levelMult(t)*g.classBonus(t.Owner, t.Kind), t.Owner, false)
	cp := center(t.Cell)
	g.emit(Event{Kind: "trap", X: cp.X, Y: cp.Y, Amount: t.Charges})
}

func (g *Game) hackBarricade(e *Enemy, spec EnemySpec, dt float64) {
	t := e.blocked
	if g.Towers[t.Cell] != t {
		e.blocked = nil // sold or already broken
		return
	}
	t.HP -= spec.WallDPS * dt
	if t.HP <= 0 {
		g.removeTower(t)
		cp := center(t.Cell)
		g.emit(Event{Kind: "wallBroken", X: cp.X, Y: cp.Y})
		for _, o := range g.Enemies {
			if o.blocked == t {
				o.blocked = nil
			}
		}
	}
}

func (g *Game) healAround(healer *Enemy, share float64) {
	for _, o := range g.Enemies {
		if o != healer && !o.dead && o.Pos.sub(healer.Pos).len() <= 1.5 {
			o.HP = min(o.MaxHP, o.HP+share*o.MaxHP)
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
		if a == t || spec.Role != RoleAura || !g.active(a) || center(a.Cell).sub(tc).len() > spec.AuraRange {
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
		if spec.Role != RoleShooter {
			continue
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
		mult := g.levelMult(t) * g.classBonus(t.Owner, t.Kind) * dmgAura
		damage := spec.Damage * mult
		if spec.BossPct > 0 {
			damage = spec.BossPct * target.MaxHP * mult
		}
		g.Projectiles = append(g.Projectiles, &Projectile{
			ID: g.id(), Kind: t.Kind, Pos: tc, Target: target.ID, Aim: target.Pos,
			Damage: damage, Magic: spec.Magic, Speed: spec.ProjSpd, Owner: t.Owner,
			Splash: spec.Splash, BurnDPS: spec.BurnDPS * mult, BurnSec: spec.BurnSec,
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
	if !ignoreArmor {
		dmg *= 1 - g.Cfg.Enemies[e.Kind].Armor
	}
	if e.shieldUp {
		dmg *= 0.2
	}
	c := g.Chatters[owner]
	if c != nil {
		c.GameDamage += min(dmg, e.HP) // overkill does not count
	}
	e.HP -= dmg
	if e.HP > 0 {
		return
	}
	e.dead = true
	if e.loot != nil {
		g.returnLoot(e)
	}
	bounty := g.Cfg.Enemies[e.Kind].Bounty
	if e.summoned {
		bounty = 1
	}
	if c == nil {
		bounty = 0 // killed by the guards or a stray shot: nobody is paid
	}
	if c != nil {
		c.Kills++
		c.GameKills++
		bounty = g.addGold(c, bounty)
	}
	g.emit(Event{Kind: "kill", User: owner, X: e.Pos.X, Y: e.Pos.Y, Amount: bounty, Text: e.Kind})
}

func (g *Game) sweepDead() {
	g.Enemies = slices.DeleteFunc(g.Enemies, func(e *Enemy) bool { return e.dead })
}
