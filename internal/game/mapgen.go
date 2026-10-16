package game

import (
	"math/rand/v2"
)

type Biome string

const (
	BiomeRuins   Biome = "ruins"
	BiomeDungeon Biome = "dungeon"
	BiomeCrypt   Biome = "crypt"
	BiomeIce     Biome = "ice"
)

var AllBiomes = []Biome{BiomeRuins, BiomeDungeon, BiomeCrypt, BiomeIce}

// biomeProfile drives the generator: path shape and obstacle style.
type biomeProfile struct {
	minLen, maxLen int
	minTurns       int
	straight       float64 // weight multiplier for keeping the current direction
	obstacles      func(r *rand.Rand, g *Grid)
}

var profiles = map[Biome]biomeProfile{
	BiomeRuins:   {minLen: 34, maxLen: 50, minTurns: 8, straight: 0.8, obstacles: scatterObstacles(0.08)},
	BiomeDungeon: {minLen: 28, maxLen: 42, minTurns: 6, straight: 1.6, obstacles: wallObstacles(0.20)},
	BiomeCrypt:   {minLen: 26, maxLen: 38, minTurns: 4, straight: 3.0, obstacles: rowObstacles(0.15)},
	BiomeIce:     {minLen: 22, maxLen: 32, minTurns: 4, straight: 1.4, obstacles: blobObstacles(0.18)},
}

type MapData struct {
	Seed    uint64
	Biome   Biome
	Grid    Grid
	Path    []Pos // from the right edge to column A
	Torches []Pos
}

// Rect is an inclusive cell rectangle.
type Rect struct{ Min, Max Pos }

func (r Rect) Has(p Pos) bool {
	return p.X >= r.Min.X && p.X <= r.Max.X && p.Y >= r.Min.Y && p.Y <= r.Max.Y
}

// WebcamRect converts the configured chess cells into a rectangle. ok is false
// when the zone is disabled or invalid.
func (c Config) WebcamRect() (Rect, bool) {
	a, ok1 := ParseCell(c.WebcamFrom)
	b, ok2 := ParseCell(c.WebcamTo)
	if !ok1 || !ok2 {
		return Rect{}, false
	}
	return Rect{
		Min: Pos{min(a.X, b.X), min(a.Y, b.Y)},
		Max: Pos{max(a.X, b.X), max(a.Y, b.Y)},
	}, true
}

// GenerateMap builds a map for the biome. The same seed, biome and webcam zone
// always give the same map.
func GenerateMap(seed uint64, biome Biome, webcam Rect, hasWebcam bool) *MapData {
	prof := profiles[biome]
	for attempt := uint64(0); ; attempt++ {
		r := rand.New(rand.NewPCG(seed, attempt))
		m := &MapData{Seed: seed, Biome: biome}
		if hasWebcam {
			for y := webcam.Min.Y; y <= webcam.Max.Y; y++ {
				for x := webcam.Min.X; x <= webcam.Max.X; x++ {
					m.Grid.Set(Pos{x, y}, CellWebcam)
				}
			}
		}
		path := carvePath(r, &m.Grid, prof)
		if path == nil {
			if attempt > 400 {
				// Practically unreachable; relax the profile rather than hang.
				prof.minLen, prof.minTurns = 16, 2
			}
			continue
		}
		m.Path = path
		for _, p := range path {
			m.Grid.Set(p, CellPath)
		}
		prof.obstacles(r, &m.Grid)
		m.Torches = placeTorches(r, &m.Grid, path)
		return m
	}
}

var dirs = [4]Pos{{-1, 0}, {0, -1}, {0, 1}, {1, 0}} // left, up, down, right
var dirWeight = [4]float64{3, 1.6, 1.6, 0.6}

type carver struct {
	r       *rand.Rand
	g       *Grid
	prof    biomeProfile
	visited [GridH][GridW]bool
	path    []Pos
	budget  int
}

func carvePath(r *rand.Rand, g *Grid, prof biomeProfile) []Pos {
	var starts []int
	for y := 1; y < GridH-1; y++ {
		if g.At(Pos{GridW - 1, y}) == CellBuild {
			starts = append(starts, y)
		}
	}
	if len(starts) == 0 {
		return nil
	}
	c := &carver{r: r, g: g, prof: prof, budget: 30000}
	start := Pos{GridW - 1, starts[r.IntN(len(starts))]}
	c.visited[start.Y][start.X] = true
	c.path = append(c.path, start)
	if c.walk(start, 0, 0) {
		return c.path
	}
	return nil
}

// walk extends the path depth-first with backtracking. dir is the index of
// the last move in dirs.
func (c *carver) walk(cur Pos, dir, turns int) bool {
	c.budget--
	if c.budget <= 0 || len(c.path) > c.prof.maxLen {
		return false
	}
	if cur.X == 0 && len(c.path) >= c.prof.minLen && turns >= c.prof.minTurns {
		return true
	}
	var prev Pos
	if len(c.path) >= 2 {
		prev = c.path[len(c.path)-2]
	} else {
		prev = Pos{GridW, cur.Y} // virtual entry point beyond the right edge
	}
	for _, d := range c.order(dir) {
		n := Pos{cur.X + dirs[d].X, cur.Y + dirs[d].Y}
		if !c.free(n, cur, prev) {
			continue
		}
		t := turns
		if d != dir {
			t++
		}
		c.visited[n.Y][n.X] = true
		c.path = append(c.path, n)
		if c.walk(n, d, t) {
			return true
		}
		c.path = c.path[:len(c.path)-1]
		c.visited[n.Y][n.X] = false
	}
	return false
}

// free reports whether n can join the path without touching earlier path
// cells, diagonals included, so corridors never merge visually.
func (c *carver) free(n, cur, prev Pos) bool {
	if !n.In() || c.visited[n.Y][n.X] || c.g.At(n) != CellBuild {
		return false
	}
	// Edge columns/rows are allowed only for the start and finish columns.
	if n.Y == 0 || n.Y == GridH-1 {
		return false
	}
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			q := Pos{n.X + dx, n.Y + dy}
			if q == n || q == cur || q == prev || !q.In() {
				continue
			}
			if c.visited[q.Y][q.X] {
				return false
			}
		}
	}
	return true
}

// order returns direction indices in a weighted random order.
func (c *carver) order(cur int) []int {
	w := dirWeight
	w[cur] *= c.prof.straight
	idx := []int{0, 1, 2, 3}
	out := make([]int, 0, 4)
	for len(idx) > 0 {
		total := 0.0
		for _, i := range idx {
			total += w[i]
		}
		pick := c.r.Float64() * total
		k := 0
		for j, i := range idx {
			pick -= w[i]
			if pick <= 0 {
				k = j
				break
			}
		}
		out = append(out, idx[k])
		idx = append(idx[:k], idx[k+1:]...)
	}
	return out
}

func buildCells(g *Grid) []Pos {
	var out []Pos
	for y := 0; y < GridH; y++ {
		for x := 0; x < GridW; x++ {
			if g.Cells[y][x] == CellBuild {
				out = append(out, Pos{x, y})
			}
		}
	}
	return out
}

func targetCount(g *Grid, density float64) int {
	return int(float64(len(buildCells(g))) * density)
}

func scatterObstacles(density float64) func(*rand.Rand, *Grid) {
	return func(r *rand.Rand, g *Grid) {
		cells := buildCells(g)
		r.Shuffle(len(cells), func(i, j int) { cells[i], cells[j] = cells[j], cells[i] })
		n := int(float64(len(cells)) * density)
		for _, p := range cells[:n] {
			g.Set(p, CellObstacle)
		}
	}
}

func wallObstacles(density float64) func(*rand.Rand, *Grid) {
	return func(r *rand.Rand, g *Grid) {
		want := targetCount(g, density)
		for placed, tries := 0, 0; placed < want && tries < 500; tries++ {
			p := Pos{r.IntN(GridW), r.IntN(GridH)}
			d := dirs[r.IntN(4)]
			for i := 0; i < 2+r.IntN(3) && placed < want; i++ {
				q := Pos{p.X + d.X*i, p.Y + d.Y*i}
				if !q.In() || g.At(q) != CellBuild {
					break
				}
				g.Set(q, CellObstacle)
				placed++
			}
		}
	}
}

func rowObstacles(density float64) func(*rand.Rand, *Grid) {
	return func(r *rand.Rand, g *Grid) {
		want := targetCount(g, density)
		ox, oy := r.IntN(3), r.IntN(2)
		var spots []Pos
		for y := oy; y < GridH; y += 2 {
			for x := ox; x+1 < GridW; x += 3 {
				spots = append(spots, Pos{x, y})
			}
		}
		r.Shuffle(len(spots), func(i, j int) { spots[i], spots[j] = spots[j], spots[i] })
		placed := 0
		for _, p := range spots {
			q := Pos{p.X + 1, p.Y}
			if placed >= want {
				break
			}
			if g.At(p) == CellBuild && g.At(q) == CellBuild {
				g.Set(p, CellObstacle)
				g.Set(q, CellObstacle)
				placed += 2
			}
		}
	}
}

func blobObstacles(density float64) func(*rand.Rand, *Grid) {
	return func(r *rand.Rand, g *Grid) {
		want := targetCount(g, density)
		for placed, tries := 0, 0; placed < want && tries < 200; tries++ {
			c := Pos{r.IntN(GridW), r.IntN(GridH)}
			rad := 1 + r.IntN(2)
			for dy := -rad; dy <= rad; dy++ {
				for dx := -rad; dx <= rad; dx++ {
					q := Pos{c.X + dx, c.Y + dy}
					if dx*dx+dy*dy > rad*rad || !q.In() || g.At(q) != CellBuild || placed >= want {
						continue
					}
					if r.Float64() < 0.75 {
						g.Set(q, CellObstacle)
						placed++
					}
				}
			}
		}
	}
}

// placeTorches puts a light source by roughly every fifth path cell: on a
// neighbouring obstacle when there is one, otherwise on the path cell itself
// (drawn as a sconce at its edge, it never blocks building).
func placeTorches(r *rand.Rand, g *Grid, path []Pos) []Pos {
	var out []Pos
	for i := 2; i < len(path); i += 4 + r.IntN(3) {
		p := path[i]
		torch := p
		for _, d := range dirs {
			q := Pos{p.X + d.X, p.Y + d.Y}
			if q.In() && g.At(q) == CellObstacle {
				torch = q
				break
			}
		}
		out = append(out, torch)
	}
	return out
}
