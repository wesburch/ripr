// Package draw is the character grid ripr paints into, plus the primitives
// that give it character: a braille canvas, half-block sprites, the tear
// wipe and the wordmark. It knows nothing about audio or Bubble Tea.
package draw

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// Style is a role, not a color. The theme resolves it.
type Style uint8

const (
	Plain      Style = iota // bone text
	Dim                     // ash
	Dim2                    // darker ash: hints, unrevealed waveform
	Line                    // rules and the input box
	Bold                    // bone, bold
	BoldU                   // bone, bold, underlined: a selected option
	Ember                   // live, in progress
	EmberBold               // logo, focused option label
	EmberBoldU              // focused selected option
	EmberU                  // focused text field
	Wisp                    // finished, alive
	WispBold                // finished filename
	Gold                    // warnings
	Button                  // the one filled block: ember ground, dark text, bold
	Cursor                  // the input cursor block
	Wave                    // revealed waveform (ember)
	WaveWisp                // finished waveform
	WaveDim                 // unrevealed baseline
	TearEdge                // the tear wipe edge
)

// Cell is one character with its role. FG/BG are hex overrides used by
// sprites and thumbnails; when set they win over the role's colors.
type Cell struct {
	R  rune
	S  Style
	FG string
	BG string
}

// Frame is a fixed grid of cells.
type Frame struct {
	W, H  int
	Cells [][]Cell
}

// New returns a frame full of spaces.
func New(w, h int) *Frame {
	f := &Frame{W: w, H: h, Cells: make([][]Cell, h)}
	for y := range f.Cells {
		row := make([]Cell, w)
		for x := range row {
			row[x] = Cell{R: ' '}
		}
		f.Cells[y] = row
	}
	return f
}

// Set writes one cell, ignoring out-of-range coordinates.
func (f *Frame) Set(x, y int, c Cell) {
	if x < 0 || y < 0 || x >= f.W || y >= f.H {
		return
	}
	f.Cells[y][x] = c
}

// Put writes s starting at x on row y.
func (f *Frame) Put(x, y int, s string, st Style) {
	if y < 0 || y >= f.H {
		return
	}
	i := 0
	for _, r := range s {
		f.Set(x+i, y, Cell{R: r, S: st})
		i++
	}
}

// PutR writes s so that it ends at column x (inclusive).
func (f *Frame) PutR(x, y int, s string, st Style) {
	f.Put(x-Width(s)+1, y, s, st)
}

// PutC centers s between x0 and x1 inclusive.
func (f *Frame) PutC(x0, x1, y int, s string, st Style) {
	f.Put(x0+(x1-x0+1-Width(s))/2, y, s, st)
}

// PutK writes s with a style chosen per character index.
func (f *Frame) PutK(x, y int, s string, fn func(i int) Style) {
	if y < 0 || y >= f.H {
		return
	}
	i := 0
	for _, r := range s {
		f.Set(x+i, y, Cell{R: r, S: fn(i)})
		i++
	}
}

// Rule draws a horizontal line of w cells.
func (f *Frame) Rule(x, y, w int, st Style) {
	f.Put(x, y, strings.Repeat("─", max(0, w)), st)
}

// Fill fills a rectangle with one cell.
func (f *Frame) Fill(x, y, w, h int, c Cell) {
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			f.Set(x+i, y+j, c)
		}
	}
}

// Clone copies the frame.
func (f *Frame) Clone() *Frame {
	g := &Frame{W: f.W, H: f.H, Cells: make([][]Cell, f.H)}
	for y := range f.Cells {
		g.Cells[y] = append([]Cell(nil), f.Cells[y]...)
	}
	return g
}

// Width counts terminal cells: wide glyphs (CJK, emoji) take two.
// Braille and box-drawing glyphs are one cell in every terminal font.
func Width(s string) int {
	return runewidth.StringWidth(s)
}

// Palette resolves roles to colors. Ground is painted behind every cell
// when non-empty (the light theme needs it; the dark theme leaves the
// terminal's own background alone).
type Palette interface {
	Resolve(s Style) (fg, bg string, bold, underline bool)
	Ground() string
}

type runKey struct {
	s      Style
	fg, bg string
}

// Renderer turns frames into ANSI strings, caching lipgloss styles.
type Renderer struct {
	P     Palette
	cache map[runKey]lipgloss.Style
}

// NewRenderer returns a renderer for a palette.
func NewRenderer(p Palette) *Renderer {
	return &Renderer{P: p, cache: map[runKey]lipgloss.Style{}}
}

func (r *Renderer) style(k runKey) lipgloss.Style {
	if s, ok := r.cache[k]; ok {
		return s
	}
	fg, bg, bold, ul := r.P.Resolve(k.s)
	if k.fg != "" {
		fg = k.fg
	}
	if k.bg != "" {
		bg = k.bg
	}
	if bg == "" {
		bg = r.P.Ground()
	}
	s := lipgloss.NewStyle()
	if fg != "" {
		s = s.Foreground(lipgloss.Color(fg))
	}
	if bg != "" {
		s = s.Background(lipgloss.Color(bg))
	}
	if bold {
		s = s.Bold(true)
	}
	if ul {
		s = s.Underline(true)
	}
	r.cache[k] = s
	return s
}

// Render produces the full screen as one string with no trailing newline.
func (r *Renderer) Render(f *Frame) string {
	var out strings.Builder
	out.Grow(f.W * f.H * 4)
	for y, row := range f.Cells {
		var run strings.Builder
		var cur runKey
		first := true
		flush := func() {
			if run.Len() == 0 {
				return
			}
			out.WriteString(r.style(cur).Render(run.String()))
			run.Reset()
		}
		for _, c := range row {
			k := runKey{c.S, c.FG, c.BG}
			if first || k != cur {
				flush()
				cur = k
				first = false
			}
			run.WriteRune(c.R)
		}
		flush()
		if y < f.H-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}
