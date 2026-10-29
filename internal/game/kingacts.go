package game

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// Patch 0.25: the King in Yellow's acts. A killbox in one turn of the path
// brought him down in a minute and a half, so he no longer dies at once:
//
//   - His health is KingActs bars. When a bar runs out he falls and leaves
//     the field for an interlude (KingAntractSec); then he rises a few
//     cells further along the route with a full bar, and his phases follow
//     the health of all his bars together.
//   - Where he fell, the ground burns: the cells within KingBurnRadius of
//     where the buildings hurt him most this act. Buildings on burnt ground
//     do nothing until the next act ends.
//   - A building can be carried (!перенести C4 H7) while he is on the field
//     or in the interlude: the mercenary takes it apart, carries it and puts
//     it up again, KingMoveSec of work at each end; meanwhile it does
//     nothing. The treasury pays KingMoveCost, or the owner when it is empty.
//     Out of burnt ground a building works again.
//   - The rain of arrows for channel points falls once a wave, in the final
//     once an act.

const kingStride = 4.0 // cells further along the route he rises after a fall

// kingActs is how many bars of health the king has.
func (g *Game) kingActs() int { return max(1, g.Cfg.KingActs) }

// kingShare is the share of his health left across all his bars: his
// phases and the mask follow it.
func (g *Game) kingShare(e *Enemy) float64 {
	acts := float64(g.kingActs())
	return (e.HP/e.MaxHP + acts - 1 - float64(e.kingAct)) / acts
}

// kingFalls takes a blow that would kill the king before his last act: he
// falls instead, the ground where he was hurt most burns, and the interlude
// begins. It reports whether he fell.
func (g *Game) kingFalls(e *Enemy, dmg float64) bool {
	if e.Kind != kingKind || e.HP-dmg > 0 || e.kingAct >= g.kingActs()-1 {
		return false
	}
	e.kingAct++
	e.HP = e.MaxHP
	e.hurtBy = nil // his torn cloak is whole again for the new act
	e.dead = true  // off the field till he rises (kingRest keeps him)
	e.blocked, e.linked, e.traps = nil, nil, nil
	g.kingRest, g.restLeft = e, g.Cfg.KingAntractSec
	g.burnGround()
	e.Dist = max(e.Dist, min(e.Dist+kingStride, g.total-3)) // never back, never at the gate
	e.Pos = g.routePos(e.Dist)
	g.emit(Event{Kind: "kingFall", X: e.Pos.X, Y: e.Pos.Y, Amount: e.kingAct + 1})
	g.reply("", fmt.Sprintf("Король пал… но это ещё не конец. Через %d с начнётся акт %d. Земля, где его ранили, выжжена: постройки на ней молчат. Унести постройку: !перенести C4 H7",
		int(math.Ceil(g.Cfg.KingAntractSec)), e.kingAct+1))
	return true
}

// burnGround burns the cells around where the buildings hurt the king most
// this act, and starts the count for the next one.
func (g *Game) burnGround() {
	var at vec
	sum := 0.0
	for _, t := range g.Towers {
		if t.kingDmg > 0 {
			c := center(t.Cell)
			at = vec{at.X + c.X*t.kingDmg, at.Y + c.Y*t.kingDmg}
			sum += t.kingDmg
		}
		t.kingDmg = 0
	}
	g.burned = map[Pos]bool{}
	if sum <= 0 || g.Cfg.KingBurnRadius <= 0 {
		return
	}
	at = vec{at.X / sum, at.Y / sum}
	for y := 0; y < GridH; y++ {
		for x := 0; x < GridW; x++ {
			if p := (Pos{x, y}); center(p).sub(at).len() <= g.Cfg.KingBurnRadius {
				g.burned[p] = true
			}
		}
	}
	g.emit(Event{Kind: "burn", X: at.X, Y: at.Y, Amount: len(g.burned)})
}

// tickKingActs runs the interlude and keeps burnt and carried buildings
// silent.
func (g *Game) tickKingActs(dt float64) {
	for _, t := range g.Towers {
		if g.burned[t.Cell] || t.moveTo != nil || t.packed {
			t.Disabled = max(t.Disabled, dt+1e-9) // silent this tick, whatever else holds it
		}
	}
	e := g.kingRest
	if e == nil || g.Phase != PhaseWave {
		return
	}
	if g.restLeft -= dt; g.restLeft > 0 {
		return
	}
	g.kingRest = nil
	e.dead = false
	g.Enemies = append(g.Enemies, e)
	e.Title = g.kingTitle(e)
	g.emit(Event{Kind: "kingRise", X: e.Pos.X, Y: e.Pos.Y, Amount: e.kingAct + 1})
	g.reply("", fmt.Sprintf("Король встаёт: акт %d", e.kingAct+1))
}

// kingTitle is the king's name on his health bar: his act, if he has more
// than one, and his phase.
func (g *Game) kingTitle(e *Enemy) string {
	name := g.Cfg.Enemies[kingKind].Name
	roman := []string{"", "I", "II", "III"}[max(e.phase, 1)]
	if g.kingActs() > 1 {
		return fmt.Sprintf("%s · акт %d · %s", name, e.kingAct+1, roman)
	}
	return fmt.Sprintf("%s · %s", name, roman)
}

// kingFight reports whether the king is on the field or in his interlude.
func (g *Game) kingFight() bool { return g.king() != nil || g.kingRest != nil }

// cmdMove sends the mercenary to carry a building elsewhere (!перенести C4 H7).
func (g *Game) cmdMove(c *Chatter, key string, args []string) {
	if g.busyAtDice(c, key) {
		return
	}
	if !g.kingFight() {
		g.reply(c.Name, "переносить постройки можно только в бою с Королём")
		return
	}
	if len(args) < 2 {
		g.reply(c.Name, "перенести постройку: !перенести C4 H7")
		return
	}
	t := g.ownTower(c, key, args[:1])
	if t == nil {
		return
	}
	to, ok := ParseCell(args[1])
	if !ok {
		g.reply(c.Name, "куда переносить? пример: !перенести C4 H7")
		return
	}
	spec := g.Cfg.Towers[t.Kind]
	switch {
	case spec.OnPath():
		g.reply(c.Name, strings.ToLower(spec.Name)+" не переносится")
		return
	case !t.Built || t.PendingCost > 0 || t.repairing || t.moveTo != nil || t.packed:
		g.reply(c.Name, t.Cell.Name()+": наёмник ещё работает над ней")
		return
	case g.Map.Grid.At(to) != CellBuild:
		g.reply(c.Name, to.Name()+": здесь строить нельзя")
		return
	case g.Towers[to] != nil || g.carriedTo(to) != nil:
		g.reply(c.Name, to.Name()+": клетка занята")
		return
	case g.burned[to]:
		g.reply(c.Name, to.Name()+": земля выжжена")
		return
	case !g.isStreamer(key) && !g.canQueueTwo(key):
		g.reply(c.Name, fmt.Sprintf("у наёмника уже %d приказа, подожди", g.Cfg.MaxOrders))
		return
	}
	cost := g.Cfg.KingMoveCost
	paid := "из казны"
	switch {
	case g.isStreamer(key) || cost <= 0:
		paid = ""
	case g.Treasury >= cost:
		g.Treasury -= cost
	case c.Gold >= cost:
		c.Gold -= cost
		paid = "из кошелька: казна пуста"
	default:
		g.reply(c.Name, fmt.Sprintf("перенос стоит %d золота, в казне %d, у тебя %d", cost, g.Treasury, c.Gold))
		return
	}
	t.moveTo = &to
	g.order(key, c.Name, t.Cell)
	g.order(key, c.Name, to)
	cp := center(t.Cell)
	g.emit(Event{Kind: "carry", User: c.Name, X: cp.X, Y: cp.Y, Text: to.Name()})
	msg := fmt.Sprintf("наёмник переносит %s на %s", t.Cell.Name(), to.Name())
	if paid != "" {
		msg += fmt.Sprintf(" (%d золота %s)", cost, paid)
	}
	g.reply(c.Name, msg)
}

// carriedTo is the building on its way to cell p, or nil.
func (g *Game) carriedTo(p Pos) *Tower {
	for _, t := range g.Towers {
		if t.moveTo != nil && *t.moveTo == p {
			return t
		}
	}
	return nil
}

// carryWork is the work a mercenary has at cell p for a carried building:
// taking it apart where it stands or putting it up where it goes.
func (g *Game) carryWork(p Pos) *Tower {
	if t := g.Towers[p]; t != nil && (t.moveTo != nil || t.packed) {
		return t
	}
	return nil
}

// carryDone finishes one end of a carry: taken apart, the building goes to
// its new cell in pieces; put up, it works again.
func (g *Game) carryDone(t *Tower, w *Worker) {
	t.Progress = 0
	if t.packed {
		t.packed = false
		cp := center(t.Cell)
		g.emit(Event{Kind: "carried", User: w.Name, X: cp.X, Y: cp.Y, Text: t.Kind})
		return
	}
	to := *t.moveTo
	delete(g.Towers, t.Cell)
	t.Cell, t.moveTo, t.packed = to, nil, true
	g.Towers[to] = t
	cp := center(to)
	g.emit(Event{Kind: "packed", User: w.Name, X: cp.X, Y: cp.Y, Text: t.Kind})
}

// strikeNow names the wave, and in the final the act, a rain of arrows falls
// in: it falls once in each (strikeKey keeps the last, plus one).
func (g *Game) strikeNow() int {
	key := g.Wave * 10
	if e := g.kingNow(); e != nil {
		key += e.kingAct
	}
	return key + 1
}

// kingNow is the king on the field or resting between acts, or nil.
func (g *Game) kingNow() *Enemy {
	if g.kingRest != nil {
		return g.kingRest
	}
	return g.king()
}

// burnedCells lists burnt ground for the overlay.
func (g *Game) burnedCells() [][2]int {
	out := make([][2]int, 0, len(g.burned))
	for p := range g.burned {
		out = append(out, [2]int{p.X, p.Y})
	}
	slices.SortFunc(out, func(a, b [2]int) int {
		if a[1] != b[1] {
			return a[1] - b[1]
		}
		return a[0] - b[0]
	})
	return out
}

// kingHacks reports whether the King in Yellow is hacking at barricade t:
// then it cannot be mended (KingWallSec).
func (g *Game) kingHacks(t *Tower) bool {
	k := g.king()
	return k != nil && k.blocked == t && g.Cfg.KingWallSec > 0
}

// Patch 0.26: his powers grow against a killbox. The Yellow Sign falls on
// KingSignPerAct more buildings with every act; Cassilda's song turns
// artifacts into plain level-5 buildings while it plays; his faceless have
// a share of his own health (KingRetinueHP) instead of a wave's; and in his
// last act the mask comes off again and again (KingMaskSec every
// KingMaskEvery seconds).

// songMutes reports whether Cassilda's song holds the artifacts now.
func (g *Game) songMutes() bool { return g.songLeft > 0 && g.Cfg.KingSongArt }

// summonRetinue calls n faceless, each with a share of the king's health.
func (g *Game) summonRetinue(king *Enemy, n int) {
	if g.Cfg.KingRetinueHP <= 0 {
		g.summon(king, facelessKind, n)
		return
	}
	for i := 0; i < n; i++ {
		e := g.spawnEnemy(facelessKind, king.MaxHP*g.Cfg.KingRetinueHP, max(0, king.Dist-0.3-0.25*float64(i)))
		e.summoned = true
	}
}

// tickMaskAgain takes the mask off again and again in his last act.
func (g *Game) tickMaskAgain(e *Enemy, dt float64) {
	if !e.masked || e.kingAct < g.kingActs()-1 || g.Cfg.KingMaskEvery <= 0 {
		return
	}
	if e.maskCD == 0 {
		e.maskCD = g.Cfg.KingMaskEvery // counted from the first time it came off
	}
	if e.maskCD -= dt; e.maskCD > 0 {
		return
	}
	e.maskCD = g.Cfg.KingMaskEvery
	g.maskLeft = max(g.maskLeft, g.Cfg.KingMaskSec)
	g.emit(Event{Kind: "maskAgain", X: e.Pos.X, Y: e.Pos.Y, Amount: int(math.Ceil(g.Cfg.KingMaskSec))})
}
