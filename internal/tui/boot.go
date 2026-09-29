package tui

import (
	"ripr/internal/tui/draw"
)

// viewBoot paints the boot sequence: the warm-up wave settles into the
// wordmark, cooling from ember to bone as each cell lands.
func (m *Model) viewBoot() *draw.Frame {
	f := draw.New(m.w, m.h)
	t := m.now.Sub(m.bootT0).Seconds()
	cw, ch := (m.bootW+1)/2, (m.bootH+3)/4
	x0, y0 := (m.w-cw)/2, (m.h-ch)/2-2
	switch m.cfg.Wordmark {
	case "block":
		sp := draw.WordmarkSprite(m.pal.fg)
		draw.Rasterize(f, x0, y0, 0, sp)
	default:
		b := draw.NewBraille(cw, ch)
		settled := draw.DrawBoot(b, m.bootPts, m.bootW, m.bootH, t)
		for r, row := range b.Rows() {
			f.PutK(x0, y0+r, row, func(i int) draw.Style {
				if settled[r*cw+i] {
					return draw.Plain
				}
				return draw.Ember
			})
		}
	}
	if t > 2.0 {
		f.PutC(0, m.w-1, y0+ch+2, "v"+version, draw.Dim2)
	}
	if t > 2.2 {
		f.PutC(0, m.w-1, m.h-3, "any key to skip", draw.Dim2)
	}
	return f
}

// version is set from main.
var version = "0.1.0"

// SetVersion lets main report the build version in the chrome.
func SetVersion(v string) {
	if v != "" {
		version = v
	}
}
