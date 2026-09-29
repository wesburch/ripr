package draw

import "math"

// The wordmark is drawn by ripr, not by a font, so it can be a setting:
// dots (braille), block (half-block letters) or plain (no logo).
// Glyphs are 1× bitmaps with a 3-pixel stroke; x-height rows 4–13,
// the i's dot in rows 0–2, the p's descender in rows 14–17.

var glyphR = []string{
	"...........",
	"...........",
	"...........",
	"...........",
	"###..####..",
	"###.######.",
	"######...##",
	"#####......",
	"###........",
	"###........",
	"###........",
	"###........",
	"###........",
	"###........",
	"...........",
	"...........",
	"...........",
	"...........",
}

var glyphI = []string{
	"###",
	"###",
	"###",
	"...",
	"###",
	"###",
	"###",
	"###",
	"###",
	"###",
	"###",
	"###",
	"###",
	"###",
	"...",
	"...",
	"...",
	"...",
}

var glyphP = []string{
	"...........",
	"...........",
	"...........",
	"...........",
	"###.#####..",
	"###.#######",
	"######...##",
	"###......##",
	"###......##",
	"###......##",
	"###......##",
	"######...##",
	"###.#######",
	"###.#####..",
	"###........",
	"###........",
	"###........",
	"###........",
}

// WordmarkPixels returns the set pixels of "ripr" at the given integer
// scale, plus the pixel width and height of the whole mark.
func WordmarkPixels(scale int) (pts [][2]int, w, h int) {
	if scale < 1 {
		scale = 1
	}
	glyphs := [][]string{glyphR, glyphI, glyphP, glyphR}
	gap := 3
	x := 0
	for _, g := range glyphs {
		gw := len(g[0])
		for r, row := range g {
			for c := 0; c < len(row); c++ {
				if row[c] != '#' {
					continue
				}
				for dy := 0; dy < scale; dy++ {
					for dx := 0; dx < scale; dx++ {
						pts = append(pts, [2]int{(x+c)*scale + dx, r*scale + dy})
					}
				}
			}
		}
		x += gw + gap
	}
	w = (x - gap) * scale
	h = len(glyphR) * scale
	return pts, w, h
}

// BootPoint is one wordmark pixel with the position it flies in from.
type BootPoint struct {
	TX, TY float64 // target
	SX, SY float64 // start, on the warm-up wave
	Delay  float64 // seconds, left to right
}

// BootPoints prepares the boot animation for a mark of the given scale.
func BootPoints(scale int, seed float64) (pts []BootPoint, w, h int) {
	raw, w, h := WordmarkPixels(scale)
	pts = make([]BootPoint, len(raw))
	amp := float64(h) / 3
	for i, p := range raw {
		sx := hashf(float64(i)*3.1+seed) * float64(w)
		pts[i] = BootPoint{
			TX: float64(p[0]), TY: float64(p[1]),
			SX: sx, SY: float64(h)/2 + amp*math.Sin(sx*0.11+1.3),
			Delay: float64(p[0])/float64(w)*0.55 + hashf(float64(i)+seed)*0.1,
		}
	}
	return pts, w, h
}

// DrawBoot paints the boot at time t (seconds) into a braille canvas of
// the mark's size and returns, per cell, whether every point in it has
// settled. Phase 1 (t < 0.8) is the warm-up wave; after that the points fly.
func DrawBoot(b *Braille, pts []BootPoint, w, h int, t float64) (settled []bool) {
	b.Clear()
	settled = make([]bool, b.W*b.H)
	for i := range settled {
		settled[i] = true
	}
	if t < 0.8 {
		a := t / 0.8
		for x := 0; x < int(float64(w)*a); x++ {
			y := float64(h)/2 + float64(h)/3*a*math.Sin(float64(x)*0.11+1.3+t*4)
			b.Set(x, int(y))
		}
		for i := range settled {
			settled[i] = false
		}
		return settled
	}
	for _, p := range pts {
		u := (t - 0.8 - p.Delay) / 0.9
		if u < 0 {
			u = 0
		}
		if u > 1 {
			u = 1
		}
		e := 1 - math.Pow(1-u, 3)
		x := p.SX + (p.TX-p.SX)*e
		y := p.SY + (p.TY-p.SY)*e
		b.Set(int(x), int(y))
		if u < 1 {
			cx, cy := int(x)/2, int(y)/4
			if cx >= 0 && cy >= 0 && cx < b.W && cy < b.H {
				settled[cy*b.W+cx] = false
			}
		}
	}
	return settled
}

// BootDone reports whether every point has landed at time t.
func BootDone(pts []BootPoint, t float64) bool {
	for _, p := range pts {
		if t-0.8-p.Delay < 0.9 {
			return false
		}
	}
	return true
}

// WordmarkSprite returns the mark as a half-block sprite (the "block" style).
func WordmarkSprite(color string) Sprite {
	raw, w, h := WordmarkPixels(1)
	rows := make([][]byte, h)
	for i := range rows {
		rows[i] = make([]byte, w)
		for j := range rows[i] {
			rows[i][j] = '.'
		}
	}
	for _, p := range raw {
		rows[p[1]][p[0]] = 'W'
	}
	s := make([]string, h)
	for i := range rows {
		s[i] = string(rows[i])
	}
	return Sprite{Rows: s, Colors: map[byte]string{'W': color}, Bright: map[byte]int{'W': 1}}
}

func hashf(n float64) float64 {
	x := math.Sin(n*12.9898+78.233) * 43758.5453
	return x - math.Floor(x)
}
