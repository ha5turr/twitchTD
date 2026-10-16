package game

import (
	"fmt"
	"strings"
	"unicode"
)

// The field is GridW columns (A..P) by GridH rows. Internally y=0 is the top
// row; in chess notation row 1 is the bottom one.
const (
	GridW = 16
	GridH = 10
)

type CellKind uint8

const (
	CellBuild    CellKind = iota // free, towers may go here
	CellPath                     // enemy path
	CellObstacle                 // decor, nothing may be built
	CellWebcam                   // hidden under the streamer's webcam
)

type Pos struct{ X, Y int }

func (p Pos) In() bool { return p.X >= 0 && p.X < GridW && p.Y >= 0 && p.Y < GridH }

// Name returns the chess-style name, e.g. "C4".
func (p Pos) Name() string {
	return fmt.Sprintf("%c%d", 'A'+p.X, GridH-p.Y)
}

// ParseCell accepts "c4", "C 4", "4c" and Cyrillic look-alikes of A..P letters
// that people type by accident on a Russian layout.
func ParseCell(s string) (Pos, bool) {
	s = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
	var letter rune
	num := 0
	digits := 0
	for _, r := range s {
		switch {
		case unicode.IsDigit(r):
			num = num*10 + int(r-'0')
			digits++
		case letter == 0:
			letter = latinize(r)
		default:
			return Pos{}, false
		}
	}
	if letter < 'A' || letter >= 'A'+GridW || digits == 0 || digits > 2 {
		return Pos{}, false
	}
	p := Pos{X: int(letter - 'A'), Y: GridH - num}
	return p, p.In()
}

var cyrToLat = map[rune]rune{
	'А': 'A', 'В': 'B', 'С': 'C', 'Е': 'E', 'Н': 'H', 'К': 'K', 'М': 'M', 'О': 'O', 'Р': 'P',
}

func latinize(r rune) rune {
	if l, ok := cyrToLat[r]; ok {
		return l
	}
	return r
}

type Grid struct {
	Cells [GridH][GridW]CellKind
}

func (g *Grid) At(p Pos) CellKind { return g.Cells[p.Y][p.X] }
func (g *Grid) Set(p Pos, k CellKind) {
	g.Cells[p.Y][p.X] = k
}
