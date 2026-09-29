// Package tui is ripr's screen: one Bubble Tea model, one column, one grid.
package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"ripr/internal/config"
	"ripr/internal/crypt"
	"ripr/internal/engine"
	"ripr/internal/tui/draw"
)

type stage int

const (
	stBoot stage = iota
	stIdle
	stPrep
	stRip
	stDone
	stCrypt
	stSettings
	stDoctor
)

// prep fields, in tab order. trim and chapter splitting are not built yet,
// so they are not offered.
const (
	fFormat = iota
	fQuality
	fCover
	fTags
	fNormalize
	fDir
	fName
	fCount
)

var qualities = []int{128, 192, 256, 320}

// Model is the whole app state.
type Model struct {
	cfg   config.Config
	tools engine.Tools
	store *crypt.Store
	pal   *Palette
	rend  *draw.Renderer

	w, h  int
	sized bool
	st    stage
	since time.Time
	now   time.Time

	// transitions
	prev   *draw.Frame
	tearT0 time.Time
	jitter []float64
	last   *draw.Frame

	// boot
	bootPts  []draw.BootPoint
	bootW    int
	bootH    int
	bootT0   time.Time
	bootSkip bool

	// link
	in            textinput.Model
	linkEdit      bool // the link box has focus on prep or done
	url           string
	meta          engine.Meta
	probing       bool
	probeErr      string
	thumb         *draw.Thumb
	fromClipboard bool
	clipURL       string

	// prep
	fmtIdx, qIdx, field int
	cover, tags         bool
	normalize           bool
	dir, name           string
	edit                textinput.Model
	editing             bool
	editTarget          string // "dir", "name", "cfgdir", "cfgname"

	// rip
	cancel   context.CancelFunc
	events   chan engine.Event
	ripGen   int           // events from an older rip are dropped
	ripDone  chan struct{} // closed when the engine goroutine returns
	amps     []float32
	revealed int
	phase    engine.Phase
	pullP    float64
	bytes    int64
	total    int64
	speed    float64
	timeS    float64
	speedX   float64
	levelL   float32
	levelR   float32
	logLine  string
	ripStart time.Time
	ripErr   string
	result   *engine.Result

	// crypt
	entries   []crypt.Entry
	sel       int
	filter    textinput.Model
	filtering bool

	// settings
	setSel int

	// doctor
	installing bool
	installErr string

	// mascot
	glowUntil time.Time
	notice    string
	noticeAt  time.Time
}

// Options configure a new model.
type Options struct {
	Config config.Config
	Tools  engine.Tools
	Store  *crypt.Store
	URL    string // open straight into prep for this link
	// one-run overrides from flags; they never reach the saved config
	Format  string
	Quality int
	Dir     string
}

// New builds the model.
func New(o Options) *Model {
	pal := Theme(o.Config.Theme)
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = ""
	in.CharLimit = 2048
	in.Width = 200
	edit := textinput.New()
	edit.Prompt = ""
	edit.CharLimit = 512
	edit.Width = 200
	filter := textinput.New()
	filter.Prompt = ""
	filter.CharLimit = 80
	filter.Width = 60

	m := &Model{
		cfg: o.Config, tools: o.Tools, store: o.Store, pal: pal, rend: draw.NewRenderer(pal),
		w: 100, h: 36, in: in, edit: edit, filter: filter,
		cover: o.Config.CoverArt, tags: o.Config.Tags, normalize: o.Config.Normalize,
		dir: o.Config.Dir, name: o.Config.Template,
	}
	fmtName := o.Config.Format
	if o.Format != "" {
		fmtName = o.Format
	}
	m.fmtIdx = indexOf(formatNames(), fmtName)
	quality := o.Config.Quality
	if o.Quality > 0 {
		quality = o.Quality
	}
	m.qIdx = 3
	for i, q := range qualities {
		if q == quality {
			m.qIdx = i
		}
	}
	if o.Dir != "" {
		m.dir = o.Dir
	}
	m.now = time.Now()
	m.since = m.now
	m.url = o.URL
	if !o.Tools.OK() {
		m.st = stDoctor
	} else if o.Config.Motion == "reduced" || o.Config.Wordmark == "plain" {
		m.st = stIdle
	} else {
		m.st = stBoot
		m.bootT0 = m.now
		m.bootPts, m.bootW, m.bootH = draw.BootPoints(2, float64(m.now.UnixNano()%1000))
	}
	if m.store != nil {
		m.entries = m.store.List()
	}
	if m.st == stIdle {
		m.in.Focus() // no boot to pass through, so focus the box here
	}
	return m
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tick(fast), textinput.Blink}
	switch {
	case m.st == stDoctor:
		// nothing to probe until the tools exist
	case m.url != "":
		m.st = stIdle
		m.probing = true
		cmds = append(cmds, m.probe(m.url, false))
	default:
		cmds = append(cmds, readClipboard())
	}
	return tea.Batch(cmds...)
}

func formatNames() []string {
	out := make([]string, len(engine.Formats))
	for i, f := range engine.Formats {
		out[i] = string(f)
	}
	return out
}

func indexOf(xs []string, s string) int {
	for i, x := range xs {
		if x == s {
			return i
		}
	}
	return 0
}

func (m *Model) format() engine.Format { return engine.Formats[m.fmtIdx] }
func (m *Model) quality() int          { return qualities[m.qIdx] }

// setStage switches stage with a tear wipe.
func (m *Model) setStage(s stage) {
	if s == m.st {
		return
	}
	if m.last != nil && m.cfg.Motion != "reduced" && m.st != stBoot {
		m.prev = m.last
		m.tearT0 = m.now
		if len(m.jitter) != m.h {
			m.jitter = draw.TearJitter(m.h)
		}
	}
	m.st = s
	m.since = m.now
	m.notice = ""
	if s == stIdle {
		m.in.Focus()
	} else {
		m.in.Blur()
	}
	if s == stCrypt && m.store != nil {
		m.entries = m.store.List()
		if m.sel >= len(m.entries) {
			m.sel = 0
		}
	}
}

func (m *Model) say(s string) {
	m.notice = s
	m.noticeAt = m.now
}

// Layout rows for the current height and density. The footer rule sits at
// h-3, so the last content row must be at most h-5.
type rows struct {
	sig, title, wave, waveH, ruler, status, prog, phases, fmt, opts, btn int
	compact                                                              bool
}

func (m *Model) rows() rows {
	if m.cfg.Density == "compact" || m.h < 36 {
		// fits 80×24: button at row 19, footer rule at row 21
		return rows{sig: 4, title: 6, wave: 8, waveH: 4, ruler: 12, status: 14, prog: 15, phases: 16, fmt: 17, opts: 18, btn: 19, compact: true}
	}
	return rows{sig: 5, title: 7, wave: 11, waveH: 8, ruler: 19, status: 22, prog: 23, phases: 24, fmt: 26, opts: 28, btn: 30}
}

// Columns for the current width: two waveform columns per cell.
func (m *Model) waveCells() int { return m.w - 4 }
