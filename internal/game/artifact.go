package game

import (
	"fmt"
	"math"
)

// Artifact buildings (0.16): a class's signature building can be raised one
// level past the others, to level 6 (a red dot), and gains a power of its own:
//   - Капканы (rogue): no limit on charges, and poison: up to 3 stacks, each
//     eating a share of what HP the enemy has left every second;
//   - Пост лучников (ranger): arrows of poison, fire or blast, switched with
//     !стрелы C4 огонь|яд|взрыв|обычные at any time;
//   - Барабаны (bard): more fire rate, and now and then a war beat: the
//     buildings in their reach hit harder for a while;
//   - Баррикада (warrior): spikes that hurt whoever hacks at it, and now and
//     then it shoves every enemy at it a cell back;
//   - Тотем (shaman): the chain loses only 5% a jump, and now and then the
//     totem lays a charge of lightning on the nearest path cell that slows
//     whoever walks through it, for 20 s;
//   - Обелиск (mage): a chance to hit for x1.5 and slow the target;
//   - Жаровня (pyromancer): a burn that stacks up to 3 times;
//   - Святилище (cleric): a shield over the tavern walls, 10% of their
//     strength each, refilled only in the build phase;
//   - Алтарь (warlock): every two of one owner's artifact altars are joined
//     by a soul link; an enemy crossing it is hurt and slowed.

const (
	poisonMax    = 3   // poison stacks
	burnStackMax = 3   // a brazier's stacked burns
	arrowBurnSec = 3.0 // fire arrows burn this long
)

// Ammo of an artifact ballista.
const (
	ammoNormal = ""
	ammoPoison = "poison"
	ammoFire   = "fire"
	ammoBlast  = "blast"
)

var ammoAliases = map[string]string{
	"яд": ammoPoison, "ядовитые": ammoPoison, "poison": ammoPoison,
	"огонь": ammoFire, "огненные": ammoFire, "fire": ammoFire,
	"взрыв": ammoBlast, "взрывные": ammoBlast, "blast": ammoBlast, "bomb": ammoBlast,
	"обычные": ammoNormal, "обычный": ammoNormal, "normal": ammoNormal, "нет": ammoNormal,
}

var ammoNames = map[string]string{ammoNormal: "обычные", ammoPoison: "ядовитые", ammoFire: "огненные", ammoBlast: "взрывные"}

// maxLevel is how far t can be raised: a level more for its owner's class
// building, the artifact level.
func (g *Game) maxLevel(t *Tower) int {
	spec := g.Cfg.Towers[t.Kind]
	if t.Kind != clownKind && g.classBonus(t.Owner, t.Kind) > 1 {
		return spec.MaxLevel + 1
	}
	return spec.MaxLevel
}

// artifact reports whether t is at the artifact level and its powers work:
// only while it is its owner's class building, so no class change, a
// restart or the panel lends a second class's artifacts.
func (g *Game) artifact(t *Tower) bool {
	return t.Level > g.Cfg.Towers[t.Kind].MaxLevel && g.classBonus(t.Owner, t.Kind) > 1
}

// hasArtifact reports whether key owns a building at the artifact level.
func (g *Game) hasArtifact(key string) bool {
	for _, t := range g.Towers {
		if t.Owner == key && t.Level > g.Cfg.Towers[t.Kind].MaxLevel {
			return true
		}
	}
	return false
}

// poison adds a stack of poison to e.
func (g *Game) poison(e *Enemy, owner string) {
	if e.poisonLeft <= 0 {
		e.poisonN = 0
	}
	e.poisonN = min(poisonMax, e.poisonN+1)
	e.poisonLeft, e.poisonOwner = g.Cfg.PoisonSec, owner
}

// tickAfflictions runs poison and stacked burns. It reports whether e died.
func (g *Game) tickAfflictions(e *Enemy, spec EnemySpec, dt float64) bool {
	if e.poisonLeft > 0 {
		e.poisonLeft -= dt
		pct := g.Cfg.PoisonPct * float64(e.poisonN)
		if spec.Boss {
			pct *= g.Cfg.PoisonBossMult
		}
		// A share of what is left: the weaker the enemy, the less it eats.
		g.damage(e, e.HP*pct*dt, e.poisonOwner, true)
		if e.dead {
			return true
		}
	}
	if e.sburnLeft > 0 {
		e.sburnLeft -= dt
		g.damage(e, e.sburnDPS*float64(e.sburnN)*dt, e.sburnOwner, true)
		if e.dead {
			return true
		}
	}
	return false
}

// burning reports whether any fire is on e: it halves healing and seals a
// hydra's wounds.
func (e *Enemy) burning() bool { return e.burnLeft > 0 || e.sburnLeft > 0 }

// stackBurn adds a stack of an artifact brazier's burn.
func (g *Game) stackBurn(e *Enemy, p *Projectile) {
	if e.sburnLeft <= 0 {
		e.sburnN, e.sburnDPS = 0, 0
	}
	e.sburnN = min(burnStackMax, e.sburnN+1)
	e.sburnDPS = math.Max(e.sburnDPS, p.BurnDPS)
	e.sburnLeft, e.sburnOwner = p.BurnSec, p.Owner
}

// arm loads an artifact shooter's shot with its power: the ballista's
// ammo, the obelisk's chance of an empowered slowing hit, the brazier's
// stacking burn.
func (g *Game) arm(t *Tower, p *Projectile) {
	if !g.artifact(t) {
		return
	}
	switch t.Kind {
	case "arrow":
		switch t.Ammo {
		case ammoPoison:
			p.Poison = true
		case ammoFire:
			p.BurnDPS, p.BurnSec = p.Damage*g.Cfg.ArrowFireBurn, arrowBurnSec
		case ammoBlast:
			p.Splash, p.SplashPct = g.Cfg.ArrowBlastRadius, g.Cfg.ArrowBlastPct
		}
	case "magic":
		if g.rng.Float64() < g.Cfg.MageArtChance {
			p.Damage *= g.Cfg.MageArtMult
			p.SlowHit, p.Arcane = true, true
		}
	case "fire":
		p.BurnStack = true
	}
}

// cmdAmmo switches an artifact ballista's arrows: !стрелы C4 огонь.
func (g *Game) cmdAmmo(c *Chatter, key string, args []string) {
	t := g.ownTower(c, key, args)
	if t == nil {
		return
	}
	if t.Kind != "arrow" || !g.artifact(t) {
		g.reply(c.Name, "менять стрелы можно только у артефактного поста лучников (6 ур., класс следопыт)")
		return
	}
	if len(args) < 2 {
		g.reply(c.Name, fmt.Sprintf("%s: стрелы %s. Сменить: !стрелы %s огонь|яд|взрыв|обычные", t.Cell.Name(), ammoNames[t.Ammo], t.Cell.Name()))
		return
	}
	ammo, ok := ammoAliases[args[1]]
	if !ok {
		g.reply(c.Name, "стрелы бывают: огонь, яд, взрыв, обычные")
		return
	}
	t.Ammo = ammo
	cp := center(t.Cell)
	g.emit(Event{Kind: "ammo", User: c.Name, Text: ammo, X: cp.X, Y: cp.Y})
	g.reply(c.Name, fmt.Sprintf("%s: теперь %s стрелы", t.Cell.Name(), ammoNames[ammo]))
}

// tickArtifact runs the timed powers of an artifact building: the drums'
// war beat, the barricade's shove, the totem's lightning charge.
func (g *Game) tickArtifact(t *Tower, spec TowerSpec, dt float64) {
	t.warLeft = max(0, t.warLeft-dt)
	if !g.artifact(t) || !t.Built || t.Disabled > 0 {
		return
	}
	t.artCD -= dt
	switch t.Kind {
	case "drums":
		if t.artCD > 0 || g.Phase != PhaseWave {
			return
		}
		t.artCD = g.Cfg.DrumWarEvery
		tc := center(t.Cell)
		reach := g.auraRange(t, spec)
		for _, o := range g.Towers {
			if o != t && o.Built && center(o.Cell).sub(tc).len() <= reach {
				o.warLeft = g.Cfg.DrumWarSec
			}
		}
		g.emit(Event{Kind: "warDrum", X: tc.X, Y: tc.Y})
	case "wall":
		if t.artCD > 0 {
			return
		}
		shoved := false
		for _, e := range g.Enemies {
			if e.dead || e.blocked != t {
				continue
			}
			e.Dist = max(0, e.Dist-1)
			e.blocked = nil
			e.siegeSec, e.rage = 0, false
			e.Pos = g.routePos(e.Dist)
			shoved = true
			g.emit(Event{Kind: "shove", X: e.Pos.X, Y: e.Pos.Y})
		}
		if shoved {
			t.artCD = g.Cfg.WallShoveEvery
		}
	}
}

// Storm is a charge of lightning an artifact totem's bolt left behind.
type Storm struct {
	Pos  vec
	left float64
}

// tickStorms slows whoever walks through a lightning charge.
func (g *Game) tickStorms(dt float64) {
	alive := g.storms[:0]
	for _, s := range g.storms {
		s.left -= dt
		if s.left <= 0 {
			continue
		}
		for _, e := range g.Enemies {
			if e.dead || g.Cfg.Enemies[e.Kind].SlowImmune || e.Pos.sub(s.Pos).len() > g.Cfg.StormRadius {
				continue
			}
			e.slowMult = math.Min(e.slowMult, g.Cfg.StormSlow)
			e.slowLeft = max(e.slowLeft, 0.25)
		}
		alive = append(alive, s)
	}
	g.storms = alive
}

// wallShieldMax is the shield artifact shrines lay over the tavern walls.
func (g *Game) wallShieldMax() float64 {
	n := 0
	for _, t := range g.Towers {
		if t.Kind == "shrine" && t.Built && g.artifact(t) {
			n++
		}
	}
	return float64(n) * g.Cfg.ShrineShieldPct * g.Cfg.WallsHP
}

// tickWallShield refills the shield in the build phase only, and trims it
// if a shrine is gone.
func (g *Game) tickWallShield() {
	most := g.wallShieldMax()
	if g.Phase == PhaseBuild {
		g.wallShield = most
	}
	g.wallShield = math.Min(g.wallShield, most)
}

// hurtWalls takes damage off the shield first, then off the walls.
func (g *Game) hurtWalls(dmg float64) {
	absorbed := math.Min(g.wallShield, dmg)
	g.wallShield -= absorbed
	g.Walls -= dmg - absorbed
}

// SoulLink joins two of one warlock's artifact altars.
type SoulLink struct {
	ID     int
	Owner  string
	A, B   vec
	Damage float64
}

// soulLinks lists the links between every two artifact altars of one owner
// that stand within SoulLinkRange cells of each other.
func (g *Game) soulLinks() []SoulLink {
	byOwner := map[string][]*Tower{}
	for _, t := range g.Towers {
		if t.Kind == "curse" && t.Built && g.artifact(t) {
			byOwner[t.Owner] = append(byOwner[t.Owner], t)
		}
	}
	var out []SoulLink
	for owner, ts := range byOwner {
		sortTowers(ts)
		for i := 0; i < len(ts); i++ {
			for j := i + 1; j < len(ts); j++ {
				if center(ts[i].Cell).sub(center(ts[j].Cell)).len() > g.Cfg.SoulLinkRange+1e-9 {
					continue
				}
				out = append(out, SoulLink{ID: ts[i].ID*100000 + ts[j].ID, Owner: owner, A: center(ts[i].Cell), B: center(ts[j].Cell)})
			}
		}
	}
	return out
}

// tickSoulLinks hurts and slows every enemy that crosses a link, once per
// link.
func (g *Game) tickSoulLinks() {
	links := g.soulLinks()
	if len(links) == 0 {
		return
	}
	for _, e := range g.Enemies {
		if e.dead {
			continue
		}
		for _, l := range links {
			if e.linked[l.ID] || segDist(e.Pos, l.A, l.B) > g.Cfg.SoulLinkWidth {
				continue
			}
			if e.linked == nil {
				e.linked = map[int]bool{}
			}
			e.linked[l.ID] = true
			g.damage(e, g.skillHP()*g.Cfg.SoulLinkDmg, l.Owner, true)
			if !g.Cfg.Enemies[e.Kind].SlowImmune {
				e.slowMult = math.Min(e.slowMult, g.Cfg.SoulLinkSlow)
				e.slowLeft = max(e.slowLeft, g.Cfg.SoulLinkSlowSec)
			}
			g.emit(Event{Kind: "soulLink", X: e.Pos.X, Y: e.Pos.Y, Amount: l.ID}) // the overlay flashes that link
		}
	}
}

// segDist is the distance from p to the segment ab.
func segDist(p, a, b vec) float64 {
	ab := b.sub(a)
	l2 := ab.X*ab.X + ab.Y*ab.Y
	if l2 == 0 {
		return p.sub(a).len()
	}
	k := math.Max(0, math.Min(1, ((p.X-a.X)*ab.X+(p.Y-a.Y)*ab.Y)/l2))
	return p.sub(vec{a.X + ab.X*k, a.Y + ab.Y*k}).len()
}

// announceArtifact tells the chat a building just reached the artifact level.
func (g *Game) announceArtifact(t *Tower, by string) {
	if !g.artifact(t) {
		return
	}
	cp := center(t.Cell)
	g.emit(Event{Kind: "artifact", User: by, Text: t.Kind, X: cp.X, Y: cp.Y})
	g.unlock(t.Owner, "artifact")
	hint := ""
	if t.Kind == "arrow" {
		hint = " Стрелы: !стрелы " + t.Cell.Name() + " огонь|яд|взрыв"
	}
	g.reply(by, fmt.Sprintf("%s: %s стал артефактом!%s", t.Cell.Name(), g.Cfg.Towers[t.Kind].Name, hint))
}

// TowerMaxLevel is how far a player can raise t (the balance simulator asks).
func (g *Game) TowerMaxLevel(t *Tower) int { return g.maxLevel(t) }

// IsArtifact reports whether t is at the artifact level.
func (g *Game) IsArtifact(t *Tower) bool { return g.artifact(t) }

// checkMonogamist gives "Однолюб" to a player whose class buildings fill
// everything they may build: the tower limit, or for a cleric every shrine
// he may own. Other buildings beside them do not count.
func (g *Game) checkMonogamist(owner string) {
	c := g.Chatters[owner]
	if c == nil {
		return
	}
	cl, ok := classByKey(c.Class)
	if !ok || cl.Tower == "" {
		return // a druid has no class building
	}
	need := g.TowerLimit()
	if cl.Tower == "shrine" && g.Cfg.ShrineLimitCleric > 0 {
		need = min(need, g.Cfg.ShrineLimitCleric)
	}
	n := 0
	for _, t := range g.Towers {
		if t.Owner == owner && t.Kind == cl.Tower && t.Built {
			n++
		}
	}
	if n >= need {
		g.unlock(owner, "monogamist")
	}
}
