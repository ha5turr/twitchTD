package game

import "slices"

// Worker is a chatter's mercenary while it is out of the tavern. It walks
// from the tavern gate to each ordered cell, builds or upgrades the tower
// there, and walks back. An enemy that reaches it stuns it: the mercenary
// flees home, rests, and then resumes its orders.
type Worker struct {
	Owner  string
	Name   string
	Pos    vec
	State  WorkerState
	Orders []Pos // cells to work on, in order
	work   float64
	rest   float64
	Veiled bool // a rogue's veil: enemies do not see him this trip
}

type WorkerState string

const (
	WorkerWalk   WorkerState = "walk"
	WorkerBuild  WorkerState = "build"
	WorkerReturn WorkerState = "return"
	WorkerFlee   WorkerState = "flee"
	WorkerRest   WorkerState = "rest" // stunned, recovering at the gate
)

// gate is where mercenaries leave and enter the tavern.
func (g *Game) gate() vec {
	return vec{-0.4, g.route[len(g.route)-1].Y}
}

// canQueue reports whether the owner's mercenary may take another order.
func (g *Game) canQueue(owner string) bool {
	w := g.Workers[owner]
	return w == nil || len(w.Orders) < g.Cfg.MaxOrders
}

// canQueueTwo reports whether the owner's mercenary may take two more orders.
func (g *Game) canQueueTwo(owner string) bool {
	w := g.Workers[owner]
	return w == nil && g.Cfg.MaxOrders >= 2 || w != nil && len(w.Orders) < g.Cfg.MaxOrders-1
}

func (g *Game) order(owner, name string, cell Pos) {
	w := g.Workers[owner]
	if w == nil {
		w = &Worker{Owner: owner, Name: name, Pos: g.gate(), State: WorkerWalk}
		g.Workers[owner] = w
	}
	w.Orders = append(w.Orders, cell)
	if w.State == WorkerReturn {
		w.State = WorkerWalk
	}
}

// removeTower deletes a tower and any orders that pointed at it.
func (g *Game) removeTower(t *Tower) {
	delete(g.Towers, t.Cell)
	if w := g.Workers[t.Owner]; w != nil {
		at := len(w.Orders) > 0 && w.Orders[0] == t.Cell
		w.Orders = slices.DeleteFunc(w.Orders, func(p Pos) bool { return p == t.Cell })
		if at && w.State == WorkerBuild {
			w.State = WorkerWalk
		}
	}
}

// needsWork reports whether the tower at cell still has unfinished work.
func (g *Game) needsWork(cell Pos) *Tower {
	t := g.Towers[cell]
	if t == nil || (t.Built && t.PendingCost == 0 && !t.repairing) {
		return nil
	}
	return t
}

// step moves w toward target and reports whether it arrived.
func (g *Game) step(w *Worker, target vec, dt float64) bool {
	d := target.sub(w.Pos)
	s := g.Cfg.WorkerSpeed * dt
	if d.len() <= s {
		w.Pos = target
		return true
	}
	w.Pos = vec{w.Pos.X + d.X/d.len()*s, w.Pos.Y + d.Y/d.len()*s}
	return false
}

func (g *Game) tickWorkers(dt float64) {
	for owner, w := range g.Workers {
		g.veil(owner, w)
		// The Воитель is never stunned by enemies, whatever runs into him, nor
		// a veiled rogue, whom they do not see.
		if w.State != WorkerFlee && w.State != WorkerRest && !w.Veiled && g.Chatters[owner].Class != "warrior" && g.enemyNear(w.Pos) {
			w.State = WorkerFlee
			g.emit(Event{Kind: "stun", User: w.Name, X: w.Pos.X, Y: w.Pos.Y})
		}
		// Drop orders whose tower was sold or finished meanwhile.
		for len(w.Orders) > 0 && g.needsWork(w.Orders[0]) == nil {
			w.Orders = w.Orders[1:]
			if w.State == WorkerBuild {
				w.State = WorkerWalk
			}
		}
		switch w.State {
		case WorkerWalk:
			if len(w.Orders) == 0 {
				w.State = WorkerReturn
				continue
			}
			if g.step(w, center(w.Orders[0]), dt) {
				w.State = WorkerBuild
				w.work = 0
			}
		case WorkerBuild:
			if len(w.Orders) == 0 {
				w.State = WorkerReturn
				continue
			}
			t := g.Towers[w.Orders[0]]
			total := g.Cfg.BuildTimeSec
			if t.Built {
				total = g.Cfg.UpgradeTimeSec * float64(max(t.PendingUp, 1)) // each level takes its time
			}
			w.work += dt
			t.Progress = min(w.work/total, 1)
			if w.work >= total {
				g.finish(t, w)
				w.Orders = w.Orders[1:]
				w.State = WorkerWalk
			}
		case WorkerReturn, WorkerFlee:
			if g.step(w, g.gate(), dt) {
				if w.State == WorkerFlee {
					w.State = WorkerRest
					w.rest = g.Cfg.StunRestSec
				} else if len(w.Orders) == 0 {
					delete(g.Workers, owner)
				} else {
					w.State = WorkerWalk
				}
			}
		case WorkerRest:
			// While enemies besiege the gate the mercenary waits inside
			// instead of stepping out into them over and over.
			if !g.enemyNear(w.Pos) {
				w.rest -= dt
			}
			if w.rest <= 0 {
				w.State = WorkerWalk
			}
		}
	}
}

func (g *Game) finish(t *Tower, w *Worker) {
	cp := center(t.Cell)
	t.Progress = 0
	if t.repairing {
		t.repairing = false
		t.HP = t.MaxHP
		t.repairCD = g.Cfg.WallRepairCDSec
		g.emit(Event{Kind: "mended", User: w.Name, X: cp.X, Y: cp.Y})
		return
	}
	if !t.Built {
		t.Built = true
		g.readyTower(t)
		g.emit(Event{Kind: "built", User: w.Name, X: cp.X, Y: cp.Y, Text: t.Kind})
		g.checkMonogamist(t.Owner)
		if c := g.Chatters[t.Owner]; c != nil && c.count("builds", 1) >= buildsNeeded {
			g.unlock(t.Owner, "architect")
		}
		return
	}
	t.Level += max(t.PendingUp, 1)
	t.PendingCost, t.PendingUp = 0, 0
	g.readyTower(t) // a stronger barricade is also a repaired one
	g.emit(Event{Kind: "upgraded", User: w.Name, X: cp.X, Y: cp.Y, Amount: t.Level})
	if t.Level >= g.Cfg.Towers[t.Kind].MaxLevel {
		g.unlock(t.Owner, "masterpiece")
	}
	g.announceArtifact(t, w.Name)
}

// veil cloaks a rogue's mercenary the first time in a wave he is out
// building or upgrading: enemies do not see him until he is home again.
// Once a wave; a shadow-step is a skill of its own and never uses it.
func (g *Game) veil(owner string, w *Worker) {
	c := g.Chatters[owner]
	if w.Veiled || g.Phase != PhaseWave || w.State == WorkerRest || c == nil || c.Class != "rogue" || c.veilWave == g.Wave {
		return
	}
	w.Veiled, c.veilWave = true, g.Wave
	g.emit(Event{Kind: "veil", User: w.Name, X: w.Pos.X, Y: w.Pos.Y})
}

func (g *Game) enemyNear(p vec) bool {
	for _, e := range g.Enemies {
		if e.Pos.sub(p).len() <= g.Cfg.StunRadius {
			return true
		}
	}
	return false
}
