package game

import "maps"

// KingProbe is what the balance simulator reads off the King in Yellow
// between ticks. It only looks: nothing here changes the game.
type KingProbe struct {
	HP, MaxHP float64
	Dist      float64 // along the route, in cells
	Speed     float64 // cells a second he walks when nothing holds him
	Phase     int     // 1..3
	Open      bool    // his cloak is open
	Masked    bool
	Blocked   bool // hacking at a barricade
	Drawing   bool // standing to draw a Yellow Sign
	AtGate    bool
	Faceless  int                // of his retinue alive
	HurtBy    map[string]float64 // damage he took by player, after his cloak
	HurtByKin map[string]float64 // what buildings dealt him by kind, before it
}

// KingNow reports the king on the field, if he is there.
func (g *Game) KingNow() (KingProbe, bool) {
	e := g.king()
	if e == nil {
		return KingProbe{}, false
	}
	speed, _ := e.fury()
	p := KingProbe{
		HP: e.HP, MaxHP: e.MaxHP, Dist: e.Dist, Phase: e.phase, Open: e.open, Masked: e.masked,
		Speed:   g.Cfg.Enemies[kingKind].Speed * speed * e.kingSpeed() * g.speedMult(),
		Blocked: e.blocked != nil, Drawing: e.drawLeft > 0, AtGate: e.AtGate,
		HurtBy: maps.Clone(e.hurtBy), HurtByKin: maps.Clone(e.hurtByKind),
	}
	for _, o := range g.Enemies {
		if o.Kind == facelessKind && !o.dead {
			p.Faceless++
		}
	}
	return p, true
}

// SetAmmo loads every artifact ballista of key with this ammo (the balance
// simulator, for a demo team whose ballistas were never given arrows).
func (g *Game) SetAmmo(key, ammo string) {
	for _, t := range g.Towers {
		if t.Owner == key && t.Kind == "arrow" && g.artifact(t) {
			t.Ammo = ammoAliases[ammo]
		}
	}
}
