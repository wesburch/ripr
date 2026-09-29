package tui

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"

	"ripr/internal/crypt"
	"ripr/internal/engine"
	"ripr/internal/tui/draw"
)

func newRenderer(p *Palette) *draw.Renderer { return draw.NewRenderer(p) }

// View implements tea.Model.
func (m *Model) View() string {
	if !m.sized {
		// painting before the terminal reports its size risks a
		// full-width line that scrolls the screen and desyncs the renderer
		return ""
	}
	if m.w < 78 || m.h < 24 {
		f := draw.New(max(m.w, 1), max(m.h, 1))
		f.PutC(0, m.w-1, m.h/2, "ripr needs at least 80×24. make the window bigger.", draw.Dim)
		return m.rend.Render(f)
	}
	var f *draw.Frame
	if m.st == stBoot {
		f = m.viewBoot()
	} else {
		f = m.compose()
	}
	m.last = f
	if m.prev != nil {
		p := m.now.Sub(m.tearT0).Seconds() / 0.32
		if p >= 1 {
			m.prev = nil
		} else {
			if len(m.jitter) != m.h {
				m.jitter = draw.TearJitter(m.h)
			}
			f = draw.Tear(m.prev, f, m.jitter, p)
		}
	}
	return m.rend.Render(f)
}

// grid columns: text at 2, lines at 1..w-2, right text edge at w-3
func (m *Model) cols() (M, LX, LW, RGT int) { return 2, 1, m.w - 2, m.w - 3 }

// natural height of the comfortable layout; taller windows get the column
// centered instead of a footer marooned at the bottom
const naturalH = 38

func (m *Model) compose() *draw.Frame {
	if m.h > naturalH+4 && m.st != stCrypt {
		realH := m.h
		m.h = naturalH
		inner := m.composeAt()
		m.h = realH
		f := draw.New(m.w, realH)
		off := (realH - naturalH) / 2
		for y := 0; y < naturalH; y++ {
			copy(f.Cells[y+off], inner.Cells[y])
		}
		return f
	}
	return m.composeAt()
}

func (m *Model) composeAt() *draw.Frame {
	f := draw.New(m.w, m.h)
	m.chrome(f)
	switch m.st {
	case stIdle:
		m.viewIdle(f)
	case stPrep:
		m.viewPrep(f)
	case stRip:
		m.viewRip(f)
	case stDone:
		m.viewDone(f)
	case stCrypt:
		m.viewCrypt(f)
	case stSettings:
		m.viewSettings(f)
	case stDoctor:
		m.viewDoctor(f)
	}
	m.footer(f)
	return f
}

func (m *Model) chrome(f *draw.Frame) {
	M, LX, LW, RGT := m.cols()
	f.Put(M, 0, "ripr", draw.EmberBold)
	f.Put(M+5, 0, "v"+version, draw.Dim)
	f.PutR(RGT, 0, tildeDir(m.dir), draw.Dim)
	if !m.tools.OK() {
		f.PutR(RGT-draw.Width(tildeDir(m.dir))-3, 0, strings.Join(m.tools.Missing, ", ")+" missing", draw.Gold)
	}
	// the input: the one box on screen
	f.Put(LX, 1, "┌"+strings.Repeat("─", LW-2)+"┐", draw.Line)
	f.Put(LX, 2, "│", draw.Line)
	f.Put(LX+LW-1, 2, "│", draw.Line)
	f.Put(LX, 3, "└"+strings.Repeat("─", LW-2)+"┘", draw.Line)
	f.Put(M+1, 2, "▸", draw.Ember)
	switch {
	case m.linkEdit:
		v := m.in.Value()
		f.Put(M+3, 2, fit(v, LW-10), draw.EmberU)
		f.Put(M+3+min(draw.Width(v), LW-10), 2, " ", m.cursorStyle())
		if m.probeErr != "" {
			f.PutR(RGT-1, 2, m.probeErr, draw.Gold)
		}
	case m.st == stPrep || m.st == stRip || m.st == stDone:
		f.Put(M+3, 2, fit(m.url, LW-8), draw.Plain)
		if m.probing {
			f.PutR(RGT-1, 2, "looking…", draw.Dim)
		}
	case m.st == stIdle && m.in.Value() != "":
		v := m.in.Value()
		f.Put(M+3, 2, fit(v, LW-10), draw.Plain)
		f.Put(M+3+min(draw.Width(v), LW-10), 2, " ", m.cursorStyle())
	default:
		f.Put(M+3, 2, " ", m.cursorStyle())
		f.Put(M+5, 2, "paste a link", draw.Dim)
		if m.probing {
			f.PutR(RGT-1, 2, "looking…", draw.Dim)
		}
	}
}

func (m *Model) cursorStyle() draw.Style {
	if m.cfg.Motion == "reduced" || (m.now.UnixMilli()/530)%2 == 0 {
		return draw.Cursor
	}
	return draw.Plain
}

func (m *Model) section(f *draw.Frame, y int, label, right string, rk draw.Style) {
	M, LX, LW, RGT := m.cols()
	if label != "" {
		f.Put(M, y, label, draw.Dim)
	}
	if right != "" {
		f.PutR(RGT, y, right, rk)
	}
	f.Rule(LX, y+1, LW, draw.Line)
}

func button(f *draw.Frame, x, y int, label string) int {
	s := " " + label + " "
	f.Put(x, y, s, draw.Button)
	return draw.Width(s)
}

// keyline draws key hints up to limit (exclusive) and drops the ones that
// don't fit, so a narrow window never runs hints into the mascot.
func keyline(f *draw.Frame, x, y, limit int, pairs [][2]string) int {
	for _, p := range pairs {
		w := draw.Width(p[0]) + 1 + draw.Width(p[1])
		if x+w > limit {
			break
		}
		f.Put(x, y, p[0], draw.Ember)
		f.Put(x+draw.Width(p[0])+1, y, p[1], draw.Dim)
		x += w + 2
	}
	return x
}

func (m *Model) footer(f *draw.Frame) {
	M, LX, LW, RGT := m.cols()
	f.Rule(LX, m.h-3, LW, draw.Line)
	var keys [][2]string
	switch {
	case m.linkEdit:
		keys = [][2]string{{"↵", "look it up"}, {"esc", "cancel"}}
	case m.st == stIdle:
		if m.clipURL != "" && m.meta.ID != "" {
			keys = [][2]string{{"↵", "rip clipboard"}, {"tab", "change first"}, {"c", "crypt"}, {",", "settings"}, {"q", "quit"}}
		} else {
			keys = [][2]string{{"↵", "rip"}, {"c", "crypt"}, {",", "settings"}, {"q", "quit"}}
		}
	case m.st == stPrep:
		if m.editing {
			keys = [][2]string{{"↵", "keep"}, {"esc", "cancel"}}
		} else {
			keys = [][2]string{{"tab", "next"}, {"←→", "change"}, {"space", "toggle"}, {"o", "choose folder"}, {"n", "name"}, {"/", "link"}, {"↵", "rip"}, {"esc", "back"}}
		}
	case m.st == stRip:
		keys = [][2]string{{"esc", "cancel"}}
	case m.st == stDone:
		keys = [][2]string{{"p", "play"}, {"r", "reveal"}, {"y", "copy path"}, {"a", "another format"}, {"/", "link"}, {"↵", "new link"}}
	case m.st == stCrypt:
		if m.filtering {
			keys = [][2]string{{"↵", "done"}, {"esc", "clear"}}
		} else {
			keys = [][2]string{{"/", "filter"}, {"p", "play"}, {"r", "reveal"}, {"y", "copy"}, {"a", "re-rip"}, {"x", "forget"}, {"esc", "back"}}
		}
	case m.st == stSettings:
		keys = [][2]string{{"↑↓", "move"}, {"←→", "change"}, {"esc", "save"}}
	case m.st == stDoctor:
		keys = [][2]string{{"↵", "fix it"}, {"r", "check again"}, {"q", "quit"}}
	}
	limit := RGT + 1
	if m.cfg.Mascot {
		limit = RGT - 11
	}
	end := keyline(f, M, m.h-1, limit, keys)
	status := m.statusWord()
	if m.notice != "" && m.now.Sub(m.noticeAt) < 4*time.Second {
		status = m.notice
	}
	// the status yields to the key hints on narrow windows
	room := RGT - 12 - end - 1
	if room > 4 {
		f.PutR(RGT-12, m.h-1, fit(status, room), draw.Dim2)
	}
	if m.cfg.Mascot {
		m.mascot(f, RGT-9, m.h-2)
	}
}

func (m *Model) statusWord() string {
	switch m.st {
	case stIdle:
		return "ready"
	case stPrep:
		return "ready to rip"
	case stRip:
		return "ripping"
	case stDone:
		return "saved"
	case stCrypt:
		if m.store != nil {
			n, b := m.store.Stats()
			return fmt.Sprintf("%d rips · %s", n, humanBytes(b))
		}
	case stSettings:
		return "~/.config/ripr/config.toml"
	case stDoctor:
		return strings.Join(m.tools.Missing, ", ") + " missing"
	}
	return ""
}

func (m *Model) mascot(f *draw.Frame, x, y int) {
	if m.cfg.Motion == "reduced" {
		draw.Rasterize(f, x, y, 1, draw.Cowl(draw.CowlOpts{}, m.pal.Cowl()))
		return
	}
	s := m.now.UnixMilli()
	period := 3400 + int64(hashf(float64(s/3400))*2200)
	blink := s%period < 160
	o := draw.CowlOpts{Blink: blink}
	if m.st == stRip && m.phase != engine.PhasePull {
		o.Blink = false
		o.Glow = m.now.Before(m.glowUntil)
		o.Dip = m.levelL > 0.6 && (s/150)%2 == 0
	}
	if m.st == stDone && m.now.Sub(m.since) < time.Second {
		o.Glow = true
		o.Blink = false
	}
	py := 1
	if o.Dip {
		py = 2
	}
	draw.Rasterize(f, x, y, py, draw.Cowl(o, m.pal.Cowl()))
}

func hashf(n float64) float64 {
	x := math.Sin(n*12.9898+78.233) * 43758.5453
	return x - math.Floor(x)
}

// ---------- the signal block, shared by idle, prep, rip, done ----------

type signal struct {
	right    string
	rightK   draw.Style
	title    string
	titleK   draw.Style
	meta     string
	url      string
	thumb    bool
	rev      float64 // 0..1 of columns revealed
	wave     draw.Style
	playhead bool
	status   string
	statusK  draw.Style
	time     string
	prog     float64
	progK    draw.Style
	phases   [][2]any // label, state 0/1/2
}

func (m *Model) drawSignal(f *draw.Frame, s signal) {
	M, LX, LW, RGT := m.cols()
	r := m.rows()
	m.section(f, r.sig, "signal", s.right, s.rightK)
	tx := M
	if s.thumb {
		t := m.thumb
		if t == nil {
			t = draw.FauxThumb(12, 3, m.pal.thumb)
		}
		t.Paint(f, M, r.title)
		tx = M + 14
	}
	f.Put(tx, r.title, fit(s.title, RGT-tx), s.titleK)
	if s.meta != "" {
		f.Put(tx, r.title+1, fit(s.meta, RGT-tx), draw.Dim)
	}
	if s.url != "" {
		f.Put(tx, r.title+2, fit(s.url, RGT-tx), draw.Dim2)
	}
	m.drawWave(f, M, r.wave, r.waveH, s.rev, s.wave, s.playhead)
	m.drawRuler(f, M, r.ruler, s.playhead, s.rev)
	f.Put(M, r.status, s.status, s.statusK)
	if s.time != "" {
		f.PutR(RGT, r.status, s.time, draw.Dim)
	}
	n := int(math.Round(s.prog * float64(LW)))
	if n > 0 {
		f.Put(LX, r.prog, strings.Repeat("─", n), s.progK)
	}
	f.Put(LX+n, r.prog, strings.Repeat("─", LW-n), draw.Line)
	x := M
	for _, p := range s.phases {
		label, state := p[0].(string), p[1].(int)
		g, gk, lk := "○", draw.Dim2, draw.Dim2
		switch state {
		case 1:
			g, gk, lk = "◐", draw.Ember, draw.Plain
		case 2:
			g, gk, lk = "✓", draw.Wisp, draw.Wisp
		}
		f.Put(x, r.phases, g, gk)
		f.Put(x+2, r.phases, label, lk)
		x += draw.Width(label) + 5
	}
}

var phasesIdle = [][2]any{{"pull", 0}, {"decode", 0}, {"encode", 0}, {"tag", 0}}

func (m *Model) drawWave(f *draw.Frame, x, y, rows int, rev float64, cls draw.Style, playhead bool) {
	cells := m.waveCells()
	b := draw.NewBraille(cells, rows)
	mid := rows * 2
	amp := float64(rows*2 - 1)
	ncol := cells * 2
	revCol := int(rev * float64(ncol))
	for c := 0; c < ncol; c++ {
		if c < revCol {
			a := m.ampAt(c, ncol)
			h := int(float64(a) * amp)
			b.VLine(c, mid-h, mid+h)
		} else if c%2 == 0 {
			b.Set(c, mid)
		}
	}
	pc := revCol / 2
	for r, row := range b.Rows() {
		f.PutK(x, y+r, row, func(i int) draw.Style {
			if i < pc {
				return cls
			}
			return draw.WaveDim
		})
	}
	if playhead && rev > 0 && rev < 1 {
		for r := 0; r < rows; r++ {
			f.Set(x+pc, y+r, draw.Cell{R: '▏', S: draw.Plain})
		}
	}
}

// ampAt maps a canvas column to the engine's columns, which may differ
// if the window was resized mid-rip.
func (m *Model) ampAt(c, ncol int) float32 {
	if len(m.amps) == 0 {
		return 0
	}
	i := c * len(m.amps) / ncol
	if i >= len(m.amps) {
		i = len(m.amps) - 1
	}
	a := m.amps[i] * 1.25
	if a > 1 {
		a = 1
	}
	return a
}

func (m *Model) drawRuler(f *draw.Frame, x, y int, playhead bool, rev float64) {
	w := m.waveCells()
	dur := m.meta.Duration
	f.Put(x, y, "0:00", draw.Dim)
	if dur > 0 {
		f.PutR(x+w-1, y, fmtTime(dur), draw.Dim)
	} else {
		f.PutR(x+w-1, y, "—:——", draw.Dim2)
	}
	px := -99
	if playhead && rev > 0 && rev < 1 {
		px = x + int(rev*float64(w-1))
	}
	for i := 1; i < 4; i++ {
		xx := x + int(math.Round(float64(w)*float64(i)/4))
		f.Put(xx, y, "┼", draw.Dim2)
		if dur > 0 && abs(xx-px) > 6 {
			f.PutC(xx-3, xx+3, y+1, fmtTime(dur*float64(i)/4), draw.Dim2)
		}
	}
	if px > 0 {
		f.Put(px, y, "▲", draw.Ember)
		f.PutC(px-3, px+3, y+1, fmtTime(rev*dur), draw.Ember)
	}
}

func (m *Model) drawFormatRow(f *draw.Frame, y int, active bool) {
	M, _, _, RGT := m.cols()
	f.Put(M, y, "format", draw.Dim)
	x := M + 8
	for i, fm := range engine.Formats {
		n := string(fm)
		st := draw.Dim2
		if i == m.fmtIdx {
			st = draw.Plain
			if active {
				st = draw.BoldU
				if m.field == fFormat {
					st = draw.EmberBoldU
				}
			}
		}
		f.Put(x, y, n, st)
		x += draw.Width(n) + 3
	}
	q := strconv.Itoa(m.quality()) + " kbps"
	qs := draw.Plain
	if !m.format().Lossy() {
		q = "lossless"
		qs = draw.Dim
	} else if active {
		qs = draw.BoldU
		if m.field == fQuality {
			qs = draw.EmberBoldU
		}
	}
	f.PutR(RGT, y, q, qs)
	f.PutR(RGT-draw.Width(q)-2, y, "quality", draw.Dim)
}

// ---------- stages ----------

func (m *Model) viewIdle(f *draw.Frame) {
	M, _, _, RGT := m.cols()
	r := m.rows()
	if m.clipURL != "" && m.meta.ID != "" {
		m.drawSignal(f, signal{right: "on your clipboard", rightK: draw.Dim, thumb: true, title: m.meta.Title, titleK: draw.Bold, meta: metaLine(m.meta), url: shortURL(m.meta.WebpageURL),
			wave: draw.Wave, status: "ready", statusK: draw.Dim, time: "— / " + fmtTime(m.meta.Duration), phases: phasesIdle})
		m.drawFormatRow(f, r.fmt, false)
		w := button(f, M, r.btn, "↵  rip it")
		f.Put(M+w+2, r.btn, fmt.Sprintf("%s %s → %s", m.format(), qualityWord(m.format(), m.quality()), tildeDir(m.dir)), draw.Dim)
		f.PutR(RGT, r.btn, "~"+humanBytes(engine.Estimate(m.meta, m.format(), m.quality())), draw.Dim2)
		return
	}
	title, tk, meta := "nothing to rip yet", draw.Dim, "paste a link above, or copy one and come back."
	if m.probing {
		title, tk, meta = "looking…", draw.Dim, "asking yt-dlp what this is"
	}
	if m.probeErr != "" {
		title, tk, meta = "that one's cursed", draw.Gold, m.probeErr
	}
	m.drawSignal(f, signal{right: "idle", rightK: draw.Dim, title: title, titleK: tk, meta: meta,
		wave: draw.Wave, status: "ready", statusK: draw.Dim, time: "— / —", phases: phasesIdle})
	m.drawFormatRow(f, r.fmt, false)
}

func (m *Model) viewPrep(f *draw.Frame) {
	M, _, _, RGT := m.cols()
	r := m.rows()
	m.drawSignal(f, signal{right: "found", rightK: draw.Dim, thumb: true, title: m.meta.Title, titleK: draw.Bold, meta: metaLine(m.meta), url: sourceLine(m.meta),
		wave: draw.Wave, status: "ready to rip", statusK: draw.Plain, time: "— / " + fmtTime(m.meta.Duration), phases: phasesIdle})
	if m.ripErr != "" {
		f.Put(M, r.status, "that one's cursed · "+m.ripErr, draw.Gold)
	}
	m.drawFormatRow(f, r.fmt, true)
	tg := []struct {
		on    bool
		label string
		field int
	}{{m.cover, "cover art", fCover}, {m.tags, "tags", fTags}, {m.normalize, "normalize", fNormalize}}
	x := M + 8
	for _, t := range tg {
		// on = bone, off = dim; ember only marks focus. wisp is for finished things.
		box, bk, lk := "[ ]", draw.Dim2, draw.Dim
		label := t.label
		if t.on {
			box, bk, lk = "[x]", draw.Plain, draw.Plain
		}
		if t.field == fCover && !coverArtPossible(m.format()) {
			// the container has no place for a picture; say so instead of pretending
			box, bk, lk = "[–]", draw.Dim2, draw.Dim2
			label = "cover art · not in " + string(m.format())
		}
		if m.field == t.field {
			bk, lk = draw.EmberBold, draw.EmberBold
		}
		f.Put(x, r.opts, box, bk)
		f.Put(x+4, r.opts, label, lk)
		x += draw.Width(label) + 7
	}
	if m.editing {
		label := "save to"
		if m.editTarget == "name" {
			label = "name"
		}
		f.Put(M, r.btn, label, draw.Dim)
		v := m.edit.Value()
		f.Put(M+9, r.btn, fit(v, RGT-M-12), draw.EmberU)
		f.Put(M+9+min(draw.Width(v), RGT-M-12), r.btn, " ", m.cursorStyle())
		return
	}
	w := button(f, M, r.btn, "↵  rip it")
	plan := fmt.Sprintf("%s %s → %s", m.format(), qualityWord(m.format(), m.quality()), filepath.Join(tildeDir(m.dir), engine.Expand(m.name, m.meta, m.format())))
	pk := draw.Dim
	if m.field == fDir || m.field == fName {
		pk = draw.EmberU
	}
	f.Put(M+w+2, r.btn, fit(plan, RGT-M-w-12), pk)
	f.PutR(RGT, r.btn, "~"+humanBytes(engine.Estimate(m.meta, m.format(), m.quality())), draw.Dim2)
}

func (m *Model) viewRip(f *draw.Frame) {
	M, _, _, RGT := m.cols()
	r := m.rows()
	dur := m.meta.Duration
	rev := 0.0
	if len(m.amps) > 0 {
		rev = float64(m.revealed) / float64(len(m.amps))
	}
	st := func(p engine.Phase) int {
		switch {
		case m.phase > p:
			return 2
		case m.phase == p:
			return 1
		}
		return 0
	}
	enc := "encode " + string(m.format())
	if m.format().Lossy() {
		enc += " " + strconv.Itoa(m.quality()) + "k"
	}
	phases := [][2]any{{"pull", st(engine.PhasePull)}, {"decode", st(engine.PhaseDecode)}, {enc, st(engine.PhaseDecode)}, {"tag + cover", st(engine.PhaseTag)}}
	var status, right string
	prog := 0.0
	switch m.phase {
	case engine.PhasePull:
		status = fmt.Sprintf("pulling · %d%% of %s at %s/s", int(m.pullP*100), humanBytes(m.total), humanBytes(int64(m.speed)))
		if m.total == 0 {
			status = fmt.Sprintf("pulling · %s at %s/s", humanBytes(m.bytes), humanBytes(int64(m.speed)))
		}
		right = "downloading"
		prog = m.pullP * 0.28
	case engine.PhaseDecode:
		status = "ripping · decoding and encoding"
		right = fmt.Sprintf("%.1f× realtime", m.speedX)
		prog = 0.28 + rev*0.66
	case engine.PhaseTag:
		status = "tagging · metadata and cover art"
		right = "almost"
		prog = 0.95
	}
	if m.ripErr != "" {
		status = "that one's cursed · " + m.ripErr
	}
	m.drawSignal(f, signal{right: right, rightK: draw.Dim, thumb: true, title: m.meta.Title, titleK: draw.Bold, meta: metaLine(m.meta), url: shortURL(m.meta.WebpageURL),
		rev: rev, wave: draw.Wave, playhead: true, status: status, statusK: draw.Ember, time: fmtTime(m.timeS) + " / " + fmtTime(dur), prog: prog, progK: draw.Ember, phases: phases})
	if m.ripErr != "" {
		f.Put(M, r.status, "that one's cursed · "+m.ripErr, draw.Gold)
	}
	m.drawFormatRow(f, r.fmt, false)
	meter := func(x int, label string, v float32) {
		f.Put(x, r.opts, label, draw.Dim)
		n := int(math.Round(float64(v) * 10))
		f.PutK(x+2, r.opts, strings.Repeat("▰", n)+strings.Repeat("▱", 10-n), func(i int) draw.Style {
			switch {
			case i >= n:
				return draw.Dim2
			case i >= 8:
				return draw.Gold
			}
			return draw.Ember
		})
	}
	meter(M, "L", m.levelL)
	meter(M+14, "R", m.levelR)
	if m.phase != engine.PhasePull && m.speedX > 0 && dur > 0 {
		eta := (dur - m.timeS) / m.speedX
		f.PutR(RGT, r.opts, fmt.Sprintf("eta %s · %s", fmtTime(eta), humanBytes(int64(float64(engine.Estimate(m.meta, m.format(), m.quality()))*rev))), draw.Dim)
	}
	if m.logLine != "" {
		f.Put(M, r.btn, fit(m.logLine, RGT-M), draw.Dim2)
	}
}

func (m *Model) viewDone(f *draw.Frame) {
	M, _, _, RGT := m.cols()
	r := m.rows()
	res := m.result
	if res == nil {
		return
	}
	extras := []string{}
	if m.tags {
		extras = append(extras, "tagged")
	}
	if m.cover && coverArtPossible(res.Format) {
		extras = append(extras, "cover art")
	}
	meta := fmt.Sprintf("%s · %s · %s %s · %s", tildeDir(filepath.Dir(res.Path)), humanBytes(res.Size), res.Format, qualityWord(res.Format, res.Bitrate), fmtTime(res.Duration))
	if len(extras) > 0 {
		meta += " · " + strings.Join(extras, " · ")
	}
	m.drawSignal(f, signal{right: "✓ " + fmtElapsed(res.Elapsed), rightK: draw.Wisp, title: filepath.Base(res.Path), titleK: draw.WispBold, meta: meta,
		rev: 1, wave: draw.WaveWisp, status: "saved", statusK: draw.Wisp, time: fmtTime(res.Duration) + " / " + fmtTime(res.Duration), prog: 1, progK: draw.Wisp,
		phases: [][2]any{{"pull", 2}, {"decode", 2}, {"encode " + string(res.Format), 2}, {"tag + cover", 2}}})
	m.drawFormatRow(f, r.fmt, false)
	keyline(f, M, r.opts, RGT+1, [][2]string{{"p", "play"}, {"r", "reveal in finder"}, {"y", "copy path"}, {"a", "another format"}})
	button(f, M, r.btn, "↵  next link")
}

func (m *Model) viewCrypt(f *draw.Frame) {
	M, _, _, RGT := m.cols()
	n, b := 0, int64(0)
	if m.store != nil {
		n, b = m.store.Stats()
	}
	m.section(f, 5, "crypt", fmt.Sprintf("%d rips · %s", n, humanBytes(b)), draw.Dim)
	f.Put(M, 7, "/", draw.Ember)
	if m.filtering || m.filter.Value() != "" {
		v := m.filter.Value()
		f.Put(M+2, 7, v, draw.EmberU)
		if m.filtering {
			f.Put(M+2+draw.Width(v), 7, " ", m.cursorStyle())
		}
	} else {
		f.Put(M+2, 7, "filter…", draw.Dim2)
	}
	wave := 50
	cFmt, cSize, cWhen, cSrc := wave+12, wave+22, wave+32, wave+42
	if m.w < 100 {
		wave = m.w - 50
		cFmt, cSize, cWhen, cSrc = wave+10, wave+17, wave+27, wave+38
	}
	f.Put(M+2, 9, "title", draw.Dim2)
	f.Put(wave, 9, "waveform", draw.Dim2)
	f.Put(cFmt, 9, "format", draw.Dim2)
	f.Put(cSize, 9, "size", draw.Dim2)
	f.Put(cWhen, 9, "when", draw.Dim2)
	f.Put(cSrc, 9, "source", draw.Dim2)
	items := m.visibleEntries()
	maxRows := (m.h - 3 - 11) / 2
	if maxRows < 1 {
		maxRows = 1
	}
	start := 0
	if m.sel >= maxRows {
		start = m.sel - maxRows + 1
	}
	if len(items) == 0 {
		f.Put(M+2, 11, "nothing here yet. rip something.", draw.Dim)
	}
	for i := start; i < len(items) && i-start < maxRows; i++ {
		e := items[i]
		y := 11 + (i-start)*2
		on := i == m.sel
		k, kd, ws := draw.Plain, draw.Dim, draw.WaveDim
		if on {
			k, kd, ws = draw.Bold, draw.Plain, draw.Wave
			f.Put(M, y, "▸", draw.Ember)
		}
		f.Put(M+2, y, fit(entryTitle(e), wave-M-4), k)
		f.Put(wave, y, draw.Spark(e.Spark), ws)
		f.Put(cFmt, y, e.Format, k)
		f.Put(cSize, y, humanBytes(e.Size), kd)
		f.Put(cWhen, y, ago(e.When, m.now), kd)
		f.Put(cSrc, y, fit(e.Extractor, RGT-cSrc+1), kd)
	}
}

func (m *Model) viewSettings(f *draw.Frame) {
	M, _, _, RGT := m.cols()
	m.section(f, 5, "settings", tildeDir(configPath()), draw.Dim2)
	for i, label := range settingRows {
		y := 7 + i*2
		if y >= m.h-4 {
			break
		}
		on := i == m.setSel
		if on {
			f.Put(M, y, "▸", draw.Ember)
		}
		lk, vk := draw.Dim, draw.Plain
		if on {
			lk, vk = draw.Bold, draw.EmberBoldU
		}
		f.Put(M+2, y, label, lk)
		if on && m.editing {
			v := m.edit.Value()
			f.Put(M+30, y, fit(v, RGT-M-32), draw.EmberU)
			f.Put(M+30+min(draw.Width(v), RGT-M-32), y, " ", m.cursorStyle())
			continue
		}
		f.Put(M+30, y, fit(m.settingValue(i), RGT-M-30), vk)
	}
}

func (m *Model) viewDoctor(f *draw.Frame) {
	M, _, _, _ := m.cols()
	m.section(f, 5, "can’t rip yet", "", draw.Dim)
	y := 7
	for _, name := range []string{"yt-dlp", "ffmpeg"} {
		missing := false
		for _, mm := range m.tools.Missing {
			if mm == name {
				missing = true
			}
		}
		if missing {
			f.Put(M, y, "✗", draw.Gold)
			f.Put(M+2, y, name+" not found", draw.Bold)
			if name == "ffmpeg" {
				f.Put(M+2, y+1, "ripr needs it to decode and encode audio.", draw.Dim)
			} else {
				f.Put(M+2, y+1, "ripr needs it to talk to youtube and 1,800 other sites.", draw.Dim)
			}
		} else {
			v := m.tools.YtDlpVersion
			if name == "ffmpeg" {
				v = m.tools.FfmpegVersion
			}
			f.Put(M, y, "✓", draw.Wisp)
			f.Put(M+2, y, name+" "+v, draw.Plain)
		}
		y += 3
	}
	if len(m.tools.Missing) > 0 {
		f.Put(M, y, "fix", draw.Dim)
		f.Put(M+5, y, "brew install "+strings.Join(m.tools.Missing, " "), draw.Plain)
		y += 2
		if m.installing {
			f.Put(M, y, "installing… this takes a minute.", draw.Ember)
		} else {
			w := button(f, M, y, "↵  run it for me")
			f.Put(M+w+2, y, "or run it yourself and press r", draw.Dim)
		}
		if m.installErr != "" {
			f.Put(M, y+2, m.installErr, draw.Gold)
		}
	}
}

// ---------- helpers ----------

// coverArtPossible mirrors the engine: only these containers carry a picture.
func coverArtPossible(f engine.Format) bool {
	return f == engine.MP3 || f == engine.M4A || f == engine.FLAC
}

func metaLine(meta engine.Meta) string {
	parts := []string{}
	if meta.Artist != "" {
		parts = append(parts, meta.Artist)
	}
	if meta.Duration > 0 {
		parts = append(parts, fmtTime(meta.Duration))
	}
	if meta.Extractor != "" {
		parts = append(parts, meta.Extractor)
	}
	if meta.Year != "" {
		parts = append(parts, meta.Year)
	}
	return strings.Join(parts, " · ")
}

func sourceLine(meta engine.Meta) string {
	parts := []string{shortURL(meta.WebpageURL)}
	if meta.Codec != "" {
		c := meta.Codec
		if meta.Bitrate > 0 {
			c += fmt.Sprintf(" %.0fk", meta.Bitrate)
		}
		parts = append(parts, c)
	}
	if meta.SampleRate > 0 {
		parts = append(parts, fmt.Sprintf("%d kHz", meta.SampleRate/1000))
	}
	switch meta.Channels {
	case 1:
		parts = append(parts, "mono")
	case 2:
		parts = append(parts, "stereo")
	}
	if len(meta.Chapters) > 0 {
		parts = append(parts, fmt.Sprintf("%d chapters", len(meta.Chapters)))
	}
	return strings.Join(parts, " · ")
}

func shortURL(u string) string {
	u = strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
	return strings.TrimPrefix(u, "www.")
}

func entryTitle(e crypt.Entry) string {
	if e.Artist != "" && !strings.Contains(e.Title, e.Artist) {
		return e.Artist + " – " + e.Title
	}
	return e.Title
}

func qualityWord(f engine.Format, kbps int) string {
	if f.Lossy() {
		return strconv.Itoa(kbps) + " kbps"
	}
	return "lossless"
}

func fmtTime(s float64) string {
	if s < 0 || math.IsNaN(s) || math.IsInf(s, 0) {
		s = 0
	}
	t := int(s)
	if t >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", t/3600, (t%3600)/60, t%60)
	}
	return fmt.Sprintf("%d:%02d", t/60, t%60)
}

func fmtElapsed(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1f s", d.Seconds())
	}
	return fmt.Sprintf("%d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}

func humanBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(b)/float64(1<<10))
	}
	return fmt.Sprintf("%d B", b)
}

func ago(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return strings.ToLower(t.Format("Mon"))
	}
	return strings.ToLower(t.Format("Jan 2"))
}

func tildeDir(p string) string {
	home, err := homeDir()
	if err == nil && strings.HasPrefix(p, home) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}

// fit truncates s to n terminal cells, counting wide glyphs as two.
func fit(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if draw.Width(s) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return runewidth.Truncate(s, n, "…")
}

func abs(i int) int {
	if i < 0 {
		return -i
	}
	return i
}
