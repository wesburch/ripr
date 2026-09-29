package draw

// Sprite is pixel art two pixels tall per cell. Rows are strings of color
// keys; '.' is transparent. Colors maps a key to a hex color and Bright
// ranks keys so the brighter pixel of a pair is always drawn as the glyph
// and the darker as the background. That keeps any rendering sliver dark.
type Sprite struct {
	Rows   []string
	Colors map[byte]string
	Bright map[byte]int
}

// Rasterize paints the sprite with its top-left pixel at cell (x, y),
// pixel row offset py (0 or 1) within the first cell row.
func Rasterize(f *Frame, x, y, py int, sp Sprite) {
	if len(sp.Rows) == 0 {
		return
	}
	w := 0
	for _, r := range sp.Rows {
		if len(r) > w {
			w = len(r)
		}
	}
	hpx := len(sp.Rows) + py
	if hpx%2 == 1 {
		hpx++
	}
	grid := make([][]byte, hpx)
	for i := range grid {
		grid[i] = make([]byte, w)
		for j := range grid[i] {
			grid[i][j] = '.'
		}
	}
	for r, row := range sp.Rows {
		for c := 0; c < len(row); c++ {
			if row[c] != '.' {
				grid[r+py][c] = row[c]
			}
		}
	}
	for cr := 0; cr < hpx/2; cr++ {
		for c := 0; c < w; c++ {
			t, b := grid[cr*2][c], grid[cr*2+1][c]
			if t == '.' && b == '.' {
				continue
			}
			var cell Cell
			switch {
			case t != '.' && b != '.' && t == b:
				cell = Cell{R: ' ', BG: sp.Colors[t]}
			case t != '.' && b != '.':
				if sp.Bright[t] >= sp.Bright[b] {
					cell = Cell{R: '▀', FG: sp.Colors[t], BG: sp.Colors[b]}
				} else {
					cell = Cell{R: '▄', FG: sp.Colors[b], BG: sp.Colors[t]}
				}
			case t != '.':
				cell = Cell{R: '▀', FG: sp.Colors[t]}
			default:
				cell = Cell{R: '▄', FG: sp.Colors[b]}
			}
			f.Set(x+c, y+cr, cell)
		}
	}
}

// CowlColors are the mascot's colors, supplied by the theme.
type CowlColors struct {
	Hood string // H
	Void string // V, the dark under the hood
	Eye  string // E
	Glow string // G, eyes on loud sections
}

// CowlOpts are the mascot's states.
type CowlOpts struct {
	Blink bool
	Glow  bool
	Dip   bool // bob one pixel on the beat
}

// the 10×3 rounded cowl peeking up from the bottom edge
var cowlRows = []string{
	"...HHHH...",
	"..HVVVVH..",
	".HVEVVEVH.",
}

// Cowl returns the mascot sprite for the given state. Rasterize it with
// py = 1 (or 2 when Dip) so it rises from the bottom of a two-row area.
func Cowl(o CowlOpts, c CowlColors) Sprite {
	rows := append([]string(nil), cowlRows...)
	eyes := []byte(rows[2])
	switch {
	case o.Blink:
		eyes[3], eyes[6] = 'V', 'V'
	case o.Glow:
		eyes[3], eyes[6] = 'G', 'G'
	}
	rows[2] = string(eyes)
	return Sprite{
		Rows:   rows,
		Colors: map[byte]string{'H': c.Hood, 'V': c.Void, 'E': c.Eye, 'G': c.Glow},
		Bright: map[byte]int{'E': 3, 'G': 3, 'H': 1, 'V': 0},
	}
}
