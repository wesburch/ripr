package draw

// Braille is a dot canvas: each cell holds 2×4 pixels.
type Braille struct {
	W, H   int // cells
	PW, PH int // pixels
	px     []bool
}

var brailleBits = [2][4]rune{{0x01, 0x02, 0x04, 0x40}, {0x08, 0x10, 0x20, 0x80}}

// NewBraille makes a canvas of w×h cells.
func NewBraille(w, h int) *Braille {
	return &Braille{W: w, H: h, PW: w * 2, PH: h * 4, px: make([]bool, w*2*h*4)}
}

// Clear turns every pixel off.
func (b *Braille) Clear() {
	for i := range b.px {
		b.px[i] = false
	}
}

// Set turns a pixel on. Out-of-range pixels are ignored.
func (b *Braille) Set(x, y int) {
	if x < 0 || y < 0 || x >= b.PW || y >= b.PH {
		return
	}
	b.px[y*b.PW+x] = true
}

// VLine draws a vertical run of pixels from y0 to y1 inclusive.
func (b *Braille) VLine(x, y0, y1 int) {
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	for y := y0; y <= y1; y++ {
		b.Set(x, y)
	}
}

// Rows renders the canvas as one string of braille characters per cell row.
func (b *Braille) Rows() []string {
	out := make([]string, b.H)
	for cy := 0; cy < b.H; cy++ {
		row := make([]rune, b.W)
		for cx := 0; cx < b.W; cx++ {
			var m rune
			for dx := 0; dx < 2; dx++ {
				for dy := 0; dy < 4; dy++ {
					if b.px[(cy*4+dy)*b.PW+cx*2+dx] {
						m |= brailleBits[dx][dy]
					}
				}
			}
			row[cx] = 0x2800 + m
		}
		out[cy] = string(row)
	}
	return out
}

// Spark renders 16 amplitude buckets (0..3) as a one-row, 8-cell sparkline.
func Spark(v [16]uint8) string {
	b := NewBraille(8, 1)
	for x := 0; x < 16; x++ {
		h := int(v[x])
		if h > 3 {
			h = 3
		}
		if h > 0 {
			b.VLine(x, 3-h, 3)
		} else {
			b.Set(x, 3)
		}
	}
	return b.Rows()[0]
}
