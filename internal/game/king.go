package game

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"unicode"
)

// Patch 0.23: the King in Yellow. The final wave is always his: a scene
// first (a cat in a yellow cloak talks to the chat, then turns into a grim
// cloaked figure), then the king himself instead of the biome's two bosses.
//
// Always: nets, traps, slows, stuns and shoves do not hold him; his torn
// cloak makes every player's damage to him fade with what that player has
// already dealt, so the whole team has to bring him down; and every
// "Хастур" in the chat heals him and speeds him up.
//
// The Yellow Sign, all fight long: every SignEvery seconds he marks a
// building near him, the best shooter by recent damage and the best support
// by level in turn. A marked shooter or trap heals him with a share of its
// damage; a marked support loses a level every few seconds, for good. Two
// players (one when he plays alone) wipe it off with !развеять C4.
//
// By his health: from 70% the faceless retinue (while any is alive he takes
// less damage) and Cassilda's song (mercenaries' skills fail); at 30% the
// mask comes off and auras work the other way for a while.
//
// One more power comes from the biome, against what it is strong in.

const (
	kingKind     = "king"
	facelessKind = "faceless"

	kingSignRadius   = 4.0  // the sign falls on a building this close
	kingSignWall     = 1.0  // and before anything else on a barricade this close
	kingSignHeal     = 0.5  // a marked shooter heals him this share of its damage
	kingSignDrainSec = 5.0  // a marked support loses a level this often
	kingHeatHalfLife = 30.0 // seconds over which a building's recent damage halves

	kingNameHeal  = 0.01 // every "Хастур" heals him this share of his max HP
	kingNameHaste = 0.05 // and speeds him up this much
	kingNameMax   = 0.30 // up to this much
	kingNameSec   = 10.0 // for this long, and one player counts this often

	kingPhase2 = 0.70 // the retinue and the song from this share of his health; the third phase starts with the mask (KingMaskAt)

	retinueEvery = 15.0
	retinueN     = 3
	retinueCut   = 0.30 // he takes this much less damage while his retinue lives

	songEvery = 20.0
	songSec   = 6.0
	songFail  = 0.5 // a mercenary's skill fails this often under the song

	maskSec = 15.0 // auras turn against their owners this long

	petrifyEvery = 25.0 // ruins: stone sleep
	petrifyN     = 2
	petrifySec   = 5.0
	frostSec     = 3.0 // ice: who tries to slow him freezes
	dryFireSec   = 4.0 // swamp: a brazier that hits him goes out
	tearRadius   = 2.0 // river: nets and booms this close tear
)

// Sign is a Yellow Sign on a building.
type Sign struct {
	Tower *Tower
	Left  float64         // seconds until it fades by itself
	By    map[string]bool // players who have wiped at it
	drain float64         // a support's seconds to its next lost level
}

// kingIntro is what the scene before the final says about the game.
type kingIntro struct {
	Players int        `json:"players"`
	Towers  int        `json:"towers"`
	Waves   int        `json:"waves"`
	Drinks  int        `json:"drinks"`
	Spots   []kingSpot `json:"spots,omitempty"`
}

// kingSpot is a player the king singles out, shown beside him with what
// they did: the most damage, the most drinks from Bob, the most dances,
// the most barricades mended.
type kingSpot struct {
	Kind  string `json:"kind"` // damage | drinks | dances | repairs
	Name  string `json:"name"`
	Class string `json:"class"`
	Value int    `json:"value"`
}

// spotMin is how much of a thing makes it worth the king's notice.
var spotMin = map[string]int{"damage": 1, "drinks": 5, "dances": 3, "repairs": 5}

// kingSpots finds the best player at each thing the king talks about.
func (g *Game) kingSpots() []kingSpot {
	var out []kingSpot
	for _, kind := range []string{"damage", "drinks", "dances", "repairs"} {
		best := kingSpot{Kind: kind}
		for _, key := range slices.Sorted(maps.Keys(g.Chatters)) {
			c := g.Chatters[key]
			if key == StreamerName {
				continue
			}
			v := map[string]int{"damage": int(c.GameDamage), "drinks": c.GameDrinks, "dances": c.GameDances, "repairs": c.GameRepairs}[kind]
			if v > best.Value {
				best.Name, best.Class, best.Value = c.Name, c.Class, v
			}
		}
		if best.Value >= spotMin[kind] {
			out = append(out, best)
		}
	}
	return out
}

// king is the King in Yellow on the field, or nil.
func (g *Game) king() *Enemy {
	for _, e := range g.Enemies {
		if e.Kind == kingKind && !e.dead {
			return e
		}
	}
	return nil
}

// startKingIntro holds the final wave for the scene and gives the overlay
// what the king will talk about.
func (g *Game) startKingIntro() {
	in := kingIntro{Players: len(g.mapVoters()), Waves: g.Wave - 1, Spots: g.kingSpots()}
	for _, c := range g.Chatters {
		in.Drinks += c.GameDrinks
	}
	for _, t := range g.Towers {
		if t.Owner != StreamerName {
			in.Towers++
		}
	}
	b, _ := json.Marshal(in)
	if !g.muted {
		// The streamer's other sounds go quiet for the King's scene and his
		// theme (a command for the streamer's own bot), back on when it ends.
		g.chatCommand("!mute")
		g.muted = true
	}
	g.introLeft = g.Cfg.KingIntroSec
	g.emit(Event{Kind: "kingIntro", Text: string(b), Amount: int(math.Ceil(g.Cfg.KingIntroSec))})
}

// kingUnmuteSec is how long after the final ends the sounds come back: the
// King's theme fades out over 6 s (king.js).
const kingUnmuteSec = 6.5

// tickMute sends !unmute when the theme has faded after the final.
func (g *Game) tickMute(dt float64) {
	if g.unmuteLeft <= 0 {
		return
	}
	if g.unmuteLeft -= dt; g.unmuteLeft <= 0 {
		g.unmute()
	}
}

func (g *Game) unmute() {
	g.chatCommand("!unmute")
	g.muted, g.unmuteLeft = false, 0
}

// KingSceneDone lets the king in once the overlay has played his scene to
// the end: its length follows the recorded lines, and KingIntroSec only
// caps it for when no overlay is watching.
func (g *Game) KingSceneDone() {
	if g.introLeft <= 0 {
		return
	}
	g.introLeft = 0
	g.emit(Event{Kind: "kingGo"})
}

// skipKingIntro ends the scene now (!tdskip).
func (g *Game) skipKingIntro() {
	if g.introLeft <= 0 {
		return
	}
	g.introLeft = 0
	g.emit(Event{Kind: "kingSkip"})
	g.emit(Event{Kind: "kingGo"})
}

// kingHP is the king's health in the final: KingHPMult bosses of the final's strength.
func (g *Game) kingHP(waveHP float64) float64 {
	return waveHP * g.Cfg.BossHPMult * g.Cfg.FinalBossMult * g.Cfg.KingHPMult * g.Cfg.Enemies[kingKind].HPMult
}

// guarded reports whether the enemy is the king under his whole cloak,
// which nothing holds.
func (e *Enemy) guarded() bool {
	return e.Kind == kingKind && !e.open
}

// unheld keeps the king free of everything that would hold him while his
// cloak is whole; in its open spells nets, slows and stuns take as on any
// boss (shoves never do).
func (g *Game) unheld(e *Enemy) {
	if e.open {
		return
	}
	e.slowMult, e.slowLeft = 1, 0
	e.netLeft, e.stunLeft = 0, 0 // barricades still stop him: he hacks through them
}

// tickCloak lets the king's cloak breathe: KingGuardSec whole, KingOpenSec open.
func (g *Game) tickCloak(e *Enemy, dt float64) {
	if g.Cfg.KingOpenSec <= 0 {
		e.open = false
		return
	}
	if e.cloakCD -= dt; e.cloakCD > 0 {
		return
	}
	e.open = !e.open
	if e.open {
		e.cloakCD = g.Cfg.KingOpenSec
		g.emit(Event{Kind: "kingOpen", X: e.Pos.X, Y: e.Pos.Y, Amount: int(math.Ceil(g.Cfg.KingOpenSec))})
	} else {
		e.cloakCD = g.Cfg.KingGuardSec
		g.emit(Event{Kind: "kingGuard", X: e.Pos.X, Y: e.Pos.Y})
	}
}

// tickKing runs the king's powers.
func (g *Game) tickKing(e *Enemy, dt float64) {
	if e.phase == 0 {
		e.signCD = g.Cfg.KingSignEvery / 4 // the first sign soon after he comes in
		e.cloakCD = g.Cfg.KingGuardSec     // he comes in with his cloak whole
	}
	g.unheld(e) // what got on him while it was whole does not carry into an open spell
	g.tickCloak(e, dt)
	g.unheld(e)
	e.drawLeft = max(0, e.drawLeft-dt)
	e.nameLeft = max(0, e.nameLeft-dt)
	if e.nameLeft <= 0 {
		e.nameHaste = 0
	}
	g.maskLeft = max(0, g.maskLeft-dt)
	g.songLeft = max(0, g.songLeft-dt)
	// The sign, all fight long.
	if e.signCD -= dt; e.signCD <= 0 {
		e.signCD = g.Cfg.KingSignEvery
		g.placeSign(e)
	}
	// The phases, by his health across all his acts.
	share := g.kingShare(e)
	phase := 1
	if share <= g.Cfg.KingMaskAt {
		phase = 3
	} else if share <= kingPhase2 {
		phase = 2
	}
	for e.phase < phase {
		e.phase++
		e.Title = g.kingTitle(e)
		g.emit(Event{Kind: "kingPhase", Amount: e.phase, X: e.Pos.X, Y: e.Pos.Y})
		if e.phase == 2 {
			e.retinueCD, e.songCD = 0, songEvery/2
			g.reply("", "Король призывает свиту безликих!")
		}
	}
	if e.phase >= 2 {
		// The faceless come only in the second phase; the song plays on to the end.
		if e.retinueCD -= dt; e.phase == 2 && e.retinueCD <= 0 {
			e.retinueCD = retinueEvery
			g.summon(e, facelessKind, max(1, int(math.Round(retinueN*g.teamShare()))))
		}
		if e.songCD -= dt; e.songCD <= 0 {
			e.songCD = songEvery
			g.songLeft = songSec
			g.emit(Event{Kind: "song", X: e.Pos.X, Y: e.Pos.Y})
		}
	}
	if !e.masked && share <= g.Cfg.KingMaskAt {
		e.masked = true
		g.maskLeft = maskSec
		g.emit(Event{Kind: "unmask", X: e.Pos.X, Y: e.Pos.Y})
		g.reply("", "Маска снята! Свита больше не приходит, но святилища и барабаны ослабляют постройки")
	}
	g.kingBiome(e, dt)
}

// kingSpeed is how much faster "Хастур" in the chat has made him.
func (e *Enemy) kingSpeed() float64 { return 1 + e.nameHaste }

// placeSign marks a building near the king: shooters and supports in turn.
func (g *Game) placeSign(king *Enemy) {
	marked := map[*Tower]bool{}
	for _, s := range g.signs {
		marked[s.Tower] = true
	}
	// Of each type he takes first a building still ahead of him, then one
	// of the kind that has hurt him most so far, then as before: the
	// deadliest shooter of late, the support of the highest level.
	fav := map[bool]string{} // support or not -> the kind that hurt him most
	for kind, dmg := range king.hurtByKind {
		sup := supportTower(g.Cfg.Towers[kind])
		if f := fav[sup]; f == "" || dmg > king.hurtByKind[f] || dmg == king.hurtByKind[f] && kind < f {
			fav[sup] = kind
		}
	}
	ahead := func(t *Tower) bool { return g.routeAhead(center(t.Cell), king.Dist) }
	first := func(t, best *Tower) int { // 1: t goes before best, -1: after, 0: a tie
		for _, b := range [][2]bool{{ahead(t), ahead(best)}, {t.Kind == fav[supportTower(g.Cfg.Towers[t.Kind])], best.Kind == fav[supportTower(g.Cfg.Towers[best.Kind])]}} {
			if b[0] != b[1] {
				if b[0] {
					return 1
				}
				return -1
			}
		}
		return 0
	}
	var shooter, support, wall *Tower
	for _, t := range g.Towers {
		if !t.Built || marked[t] || t.Kind == "clown" || center(t.Cell).sub(king.Pos).len() > kingSignRadius {
			continue
		}
		if l := center(t.Cell).sub(king.Pos).len(); g.Cfg.Towers[t.Kind].Role == RoleWall && l <= kingSignWall && (wall == nil || l < center(wall.Cell).sub(king.Pos).len()) {
			wall = t // the barricade in his way
		}
		if supportTower(g.Cfg.Towers[t.Kind]) {
			if support == nil {
				support = t
			} else if c := first(t, support); c > 0 || c == 0 && (t.Level > support.Level || t.Level == support.Level && t.ID < support.ID) {
				support = t
			}
		} else if shooter == nil {
			shooter = t
		} else if c := first(t, shooter); c > 0 || c == 0 && (t.heat > shooter.heat || t.heat == shooter.heat && t.Level > shooter.Level) {
			shooter = t
		}
	}
	at := shooter
	if g.signSupport && support != nil || at == nil {
		at = support
	}
	if wall != nil {
		at = wall // a barricade he has come up to goes first, whoever's turn it is
	}
	if at == nil {
		king.signCD = 2 // nothing near: look again soon
		return
	}
	g.signSupport = !supportTower(g.Cfg.Towers[at.Kind]) // the other kind next time
	g.signs = append(g.signs, &Sign{Tower: at, Left: g.Cfg.KingSignSec, By: map[string]bool{}, drain: kingSignDrainSec})
	king.drawLeft = g.Cfg.KingSignPause // he stops to draw it
	cp := center(at.Cell)
	g.emit(Event{Kind: "sign", X: cp.X, Y: cp.Y, Text: at.Cell.Name()})
	g.reply("", fmt.Sprintf("Жёлтый знак на %s! Развеять: !рз %s", at.Cell.Name(), at.Cell.Name()))
}

// routeAhead reports whether p borders the stretch of the route the king
// at distance d has yet to walk, within the sign's reach: a building there
// still stands in his way, whatever it borders behind him.
func (g *Game) routeAhead(p vec, d float64) bool {
	for x := d + 0.5; x <= min(g.total, d+kingSignRadius+1); x += 0.2 {
		if g.routePos(x).sub(p).len() <= 1.6 {
			return true
		}
	}
	return false
}

// supportTower reports whether a building helps others rather than hurts:
// a sign drains its levels instead of turning its damage into healing.
func supportTower(spec TowerSpec) bool {
	return spec.Role == RoleAura || spec.Role == RoleCurse || spec.Role == RoleWall
}

// signOn returns the sign on a building, or nil.
func (g *Game) signOn(t *Tower) *Sign {
	for _, s := range g.signs {
		if s.Tower == t {
			return s
		}
	}
	return nil
}

// tickSigns fades signs, drains the supports under them and drops those
// whose building is gone.
func (g *Game) tickSigns(dt float64) {
	if len(g.demoDispel) > 0 {
		g.tickDemo()
	}
	g.signs = slices.DeleteFunc(g.signs, func(s *Sign) bool {
		if g.Towers[s.Tower.Cell] != s.Tower {
			return true
		}
		s.Left -= dt
		if s.Left <= 0 {
			cp := center(s.Tower.Cell)
			g.emit(Event{Kind: "signFade", X: cp.X, Y: cp.Y})
			return true
		}
		if supportTower(g.Cfg.Towers[s.Tower.Kind]) {
			if s.drain -= dt; s.drain <= 0 {
				s.drain = kingSignDrainSec
				g.drainLevel(s.Tower)
			}
		}
		return false
	})
}

// drainLevel takes a level off a building for good, down to the first: the
// gold put into it is gone, and a sale counts from the level it is left at.
func (g *Game) drainLevel(t *Tower) {
	if t.Level <= 1 {
		return
	}
	hp := 1.0
	if t.MaxHP > 0 {
		hp = t.HP / t.MaxHP
	}
	t.Level--
	g.readyTower(t)
	if t.MaxHP > 0 {
		t.HP = t.MaxHP * hp // a barricade keeps the share it had
	}
	worth := 0
	for l := 1; l <= t.Level; l++ {
		worth += g.Cfg.TowerCost(t.Kind, l)
	}
	t.Invested = min(t.Invested, worth)
	t.Granted = min(t.Granted, t.Invested)
	cp := center(t.Cell)
	g.emit(Event{Kind: "drain", X: cp.X, Y: cp.Y, Amount: t.Level})
}

// signHeal heals the king with a share of the damage a marked building deals.
func (g *Game) signHeal(t *Tower, dmg float64) {
	if g.signOn(t) == nil {
		return
	}
	if k := g.king(); k != nil {
		k.HP = min(k.MaxHP, k.HP+dmg*kingSignHeal)
	}
}

// warm counts damage a building deals, for the sign to find the deadliest;
// what it deals the king counts for its kind too.
func (t *Tower) warm(dmg float64, e *Enemy) {
	t.heat += dmg
	if e != nil && e.Kind == kingKind {
		t.kingDmg += dmg
		if e.hurtByKind == nil {
			e.hurtByKind = map[string]float64{}
		}
		e.hurtByKind[t.Kind] += dmg
	}
}

// cool lets a building's recent damage fade.
func (t *Tower) cool(dt float64) { t.heat *= math.Pow(0.5, dt/kingHeatHalfLife) }

// cmdDispel wipes at a sign: two players take it off, or one who plays alone.
func (g *Game) cmdDispel(c *Chatter, key string, args []string) {
	if len(args) == 0 {
		g.reply(c.Name, "развеять знак: !рз C4")
		return
	}
	p, ok := ParseCell(args[0])
	if !ok {
		return
	}
	var s *Sign
	for _, x := range g.signs {
		if x.Tower.Cell == p {
			s = x
		}
	}
	if s == nil {
		g.reply(c.Name, fmt.Sprintf("на %s нет Жёлтого знака", p.Name()))
		return
	}
	if s.By[key] {
		g.reply(c.Name, "нужен ещё один игрок")
		return
	}
	s.By[key] = true
	need := min(2, max(1, len(g.mapVoters())))
	cp := center(p)
	if len(s.By) < need {
		g.emit(Event{Kind: "dispelling", User: c.Name, X: cp.X, Y: cp.Y})
		g.reply("", fmt.Sprintf("%s развеивает знак на %s: нужен ещё один игрок", c.Name, p.Name()))
		return
	}
	g.signs = slices.DeleteFunc(g.signs, func(x *Sign) bool { return x == s })
	g.emit(Event{Kind: "signOff", User: c.Name, X: cp.X, Y: cp.Y})
	g.reply("", fmt.Sprintf("Жёлтый знак на %s развеян", p.Name()))
}

// clearKing drops what the king left behind once the final is over.
func (g *Game) clearKing() {
	g.signs = nil
	g.maskLeft, g.songLeft, g.introLeft = 0, 0, 0
	g.signSupport = false
	g.brokenLinks = nil
	g.kingRest, g.restLeft, g.burned, g.strikeKey = nil, 0, nil, 0
}

// kingCut is what the king's torn cloak and his retinue leave of a hit.
func (g *Game) kingCut(e *Enemy, dmg float64, owner string) float64 {
	if owner != "" && g.Cfg.KingCloak > 0 {
		// The cloak stretches over the team: a smaller one has fewer to share the work.
		dmg /= 1 + e.hurtBy[owner]/(g.Cfg.KingCloak/g.teamShare()*e.MaxHP)
	}
	for _, o := range g.Enemies {
		if o.Kind == facelessKind && !o.dead {
			return dmg * (1 - retinueCut)
		}
	}
	return dmg
}

// nameWords are "Хастур" in the forms the chat would write it, not the
// channel's name.
var nameWords = map[string]bool{
	"хастур": true, "хастура": true, "хастуру": true, "хастуром": true, "хастуре": true, "hastur": true,
}

// sayName heals and speeds up the king for every "Хастур" in the chat while
// he walks: once in a while from each player, and the bot warns once.
func (g *Game) sayName(key, text string) {
	k := g.king()
	if k == nil {
		return
	}
	named := false
	for _, w := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) }) {
		if nameWords[w] {
			named = true
			break
		}
	}
	if !named {
		return
	}
	if !g.nameWarned {
		g.nameWarned = true
		g.reply("", "Не произносите его имя: каждое делает Короля сильнее")
	}
	if k.namedBy == nil {
		k.namedBy = map[string]float64{}
	}
	now := g.matchElapsed
	if last, ok := k.namedBy[key]; ok && now-last < kingNameSec {
		return
	}
	k.namedBy[key] = now
	k.HP = min(k.MaxHP, k.HP+kingNameHeal*k.MaxHP)
	k.nameHaste = min(kingNameMax, k.nameHaste+kingNameHaste)
	k.nameLeft = kingNameSec
	g.emit(Event{Kind: "named", X: k.Pos.X, Y: k.Pos.Y})
}

// songFails reports whether Cassilda's song breaks a mercenary's skill now.
func (g *Game) songFails() bool {
	return g.songLeft > 0 && g.rng.Float64() < songFail
}

// kingBiome runs the king's power from the biome.
func (g *Game) kingBiome(e *Enemy, dt float64) {
	switch g.Map.Biome {
	case BiomeRuins: // stone sleep: the best buildings near him turn to stone
		if e.biomeCD -= dt; e.biomeCD <= 0 {
			e.biomeCD = petrifyEvery
			var near []*Tower
			for _, t := range g.Towers {
				if t.Built && center(t.Cell).sub(e.Pos).len() <= kingSignRadius {
					near = append(near, t)
				}
			}
			slices.SortFunc(near, func(a, b *Tower) int {
				if a.Level != b.Level {
					return b.Level - a.Level
				}
				return a.ID - b.ID
			})
			for _, t := range near[:min(petrifyN, len(near))] {
				t.Disabled = max(t.Disabled, petrifySec)
				cp := center(t.Cell)
				g.emit(Event{Kind: "petrify", X: cp.X, Y: cp.Y})
			}
		}
	case BiomeRiver: // a tide without end: nets and booms near him tear
		for _, t := range g.Towers {
			role := g.Cfg.Towers[t.Kind].Role
			if !t.Built || !g.wet(t.Cell) || (role != RoleTrap && role != RoleWall) || e.torn[t.ID] || center(t.Cell).sub(e.Pos).len() > tearRadius {
				continue
			}
			if e.torn == nil {
				e.torn = map[int]bool{}
			}
			e.torn[t.ID] = true
			if role == RoleTrap {
				t.Charges = 0
				if g.artifact(t) {
					t.Disabled = max(t.Disabled, 10) // an artifact net is not spent, only torn for a while
				}
			} else {
				t.HP /= 2
			}
			cp := center(t.Cell)
			g.emit(Event{Kind: "tear", X: cp.X, Y: cp.Y})
		}
	}
}

// kingFreezes (ice) freezes a building that tried to slow the king.
func (g *Game) kingFreezes(e *Enemy, t *Tower) {
	if e.Kind != kingKind || t == nil || g.Map.Biome != BiomeIce || e.open {
		return
	}
	t.Disabled = max(t.Disabled, frostSec)
	cp := center(t.Cell)
	g.emit(Event{Kind: "frostback", X: cp.X, Y: cp.Y})
}

// kingDousesFire (swamp) puts out a brazier whose shot hit the king.
func (g *Game) kingDousesFire(e *Enemy, t *Tower) {
	if e.Kind != kingKind || t == nil || t.Kind != "fire" || g.Map.Biome != BiomeSwamp {
		return
	}
	t.Disabled = max(t.Disabled, dryFireSec)
	cp := center(t.Cell)
	g.emit(Event{Kind: "doused", X: cp.X, Y: cp.Y})
}

// fireproof (swamp) reports whether burns do not stick on the enemy.
func (g *Game) fireproof(e *Enemy) bool {
	return e.Kind == kingKind && g.Map.Biome == BiomeSwamp
}

// tearsLink (crypt) reports whether the enemy breaks a soul link instead of
// being hurt by it, and breaks it.
func (g *Game) tearsLink(e *Enemy, id int) bool {
	if e.Kind != kingKind || g.Map.Biome != BiomeCrypt {
		return false
	}
	if g.brokenLinks == nil {
		g.brokenLinks = map[int]bool{}
	}
	g.brokenLinks[id] = true
	g.emit(Event{Kind: "linkBroken", X: e.Pos.X, Y: e.Pos.Y, Amount: id})
	return true
}

// cramped (dungeon) is how much harder the king and his retinue hit barricades.
func (g *Game) cramped(e *Enemy) float64 {
	if g.Map.Biome == BiomeDungeon && (e.Kind == kingKind || e.Kind == facelessKind) {
		return 2
	}
	return 1
}

// towerByID finds a building by its id.
func (g *Game) towerByID(id int) *Tower {
	for _, t := range g.Towers {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// kingHit lets the king answer a shot that hit him: on the ice he freezes
// the obelisk that tried to slow him, in the swamp he douses the brazier.
func (g *Game) kingHit(e *Enemy, p *Projectile) {
	if e.Kind != kingKind || p.From == 0 {
		return
	}
	t := g.towerByID(p.From)
	if p.SlowHit {
		g.kingFreezes(e, t)
	}
	g.kingDousesFire(e, t)
}

// SignCells lists the cells under a Yellow Sign (the balance simulator's
// bots wipe them off as the chat would).
func (g *Game) SignCells() []Pos {
	out := make([]Pos, 0, len(g.signs))
	for _, s := range g.signs {
		out = append(out, s.Tower.Cell)
	}
	return out
}
