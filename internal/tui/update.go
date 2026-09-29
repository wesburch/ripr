package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"ripr/internal/config"
	"ripr/internal/crypt"
	"ripr/internal/engine"
)

func homeDir() (string, error) { return os.UserHomeDir() }

func cacheDir() string { return crypt.DefaultCacheDir() }

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// never paint the last column: some emulators wrap a full-width
		// line immediately and scroll the whole frame away
		m.w, m.h = msg.Width-1, msg.Height
		if m.w > 140 {
			m.w = 140
		}
		m.jitter = nil
		if !m.sized {
			m.sized = true
			m.bootT0 = m.now // the boot clock starts when we can actually draw
			m.since = m.now
		}
		return m, tea.ClearScreen

	case tickMsg:
		m.now = time.Time(msg)
		if m.st == stBoot {
			// warm-up wave 0.8 s, the letters land by ~2.3 s, hold, then in
			t := m.now.Sub(m.bootT0).Seconds()
			if m.bootSkip || t > 3.2 {
				m.setStage(stIdle)
				m.prev = nil
			}
		}
		if m.st == stRip && m.result != nil {
			m.setStage(stDone)
		}
		return m, tick(m.tickRate())

	case clipMsg:
		if m.st == stIdle && m.url == "" && !m.probing {
			m.clipURL = msg.url
			return m, m.probe(msg.url, true)
		}
		return m, nil

	case probedMsg:
		if msg.clip && m.st != stIdle {
			// a clipboard lookup that lands after the user moved on is stale
			return m, nil
		}
		m.probing = false
		if msg.err != nil {
			m.probeErr = shortErr(msg.err)
			if msg.clip {
				m.probeErr = ""
				m.clipURL = ""
			}
			return m, nil
		}
		m.meta = msg.meta
		m.url = msg.url
		m.probeErr = ""
		m.thumb = nil
		m.result = nil
		m.fromClipboard = msg.clip
		if !msg.clip {
			m.in.SetValue("")
			m.linkEdit = false
			if m.st == stPrep {
				m.since = m.now // same stage, new link: no tear, just refresh
			} else {
				m.setStage(stPrep)
			}
			m.field = fFormat
		}
		return m, fetchThumb(msg.meta)

	case thumbMsg:
		m.thumb = msg.t
		return m, nil

	case evMsg:
		return m.onEvent(engine.Event(msg))

	case evBatchMsg:
		if msg.gen != m.ripGen || m.st != stRip {
			return m, nil // from a rip that was cancelled or already finished
		}
		var cmd tea.Cmd
		for _, ev := range msg.evs {
			_, cmd = m.onEvent(ev)
			if ev.Err != nil || ev.Phase == engine.PhaseDone {
				return m, nil
			}
		}
		return m, cmd

	case noticeMsg:
		m.say(string(msg))
		return m, nil

	case folderMsg:
		if msg.path == "" {
			m.say("kept " + tildeDir(m.dir))
			return m, nil
		}
		switch msg.target {
		case "cfgdir":
			m.cfg.Dir = msg.path
			m.dir = msg.path
		default:
			m.dir = msg.path
		}
		m.say("saving to " + tildeDir(msg.path))
		return m, nil

	case doctorMsg:
		m.tools = engine.Tools(msg)
		m.installing = false
		if m.tools.OK() && m.st == stDoctor {
			m.setStage(stIdle)
			return m, readClipboard()
		}
		return m, nil

	case installMsg:
		m.installing = false
		if msg.err != nil {
			m.installErr = msg.err.Error()
			return m, nil
		}
		return m, runDoctor()

	case tea.KeyMsg:
		return m.onKey(msg)
	}

	// text inputs get everything else (cursor blink etc.)
	var cmd tea.Cmd
	switch {
	case m.editing:
		m.edit, cmd = m.edit.Update(msg)
	case m.filtering:
		m.filter, cmd = m.filter.Update(msg)
	default:
		m.in, cmd = m.in.Update(msg)
	}
	return m, cmd
}

func (m *Model) tickRate() time.Duration {
	if m.cfg.Motion == "reduced" {
		return slow // static frames; the rip still needs to advance
	}
	if m.st == stBoot || m.st == stRip || m.prev != nil {
		return fast
	}
	return slow
}

func shortErr(err error) string {
	s := err.Error()
	if i := strings.LastIndex(s, ": "); i >= 0 && i < len(s)-2 {
		s = s[i+2:]
	}
	s = strings.TrimPrefix(s, "ERROR: ")
	if len(s) > 80 {
		s = s[:79] + "…"
	}
	return s
}

func (m *Model) onEvent(ev engine.Event) (tea.Model, tea.Cmd) {
	if ev.Err != nil {
		m.ripErr = shortErr(ev.Err)
		m.cancel = nil
		m.say("hm. that one's cursed.")
		return m, nil
	}
	m.phase = ev.Phase
	switch ev.Phase {
	case engine.PhasePull:
		m.pullP = ev.Progress
		m.bytes, m.total, m.speed = ev.Bytes, ev.TotalBytes, ev.Speed
	case engine.PhaseDecode, engine.PhaseTag:
		if ev.Time > 0 {
			m.timeS = ev.Time
		}
		if ev.SpeedX > 0 {
			m.speedX = ev.SpeedX
		}
		if ev.Column >= 0 && ev.Column < len(m.amps) {
			m.amps[ev.Column] = ev.Amp
			if ev.Column+1 > m.revealed {
				m.revealed = ev.Column + 1
			}
		}
		if ev.LevelL > 0 || ev.LevelR > 0 {
			m.levelL, m.levelR = ev.LevelL, ev.LevelR
			if ev.LevelL > 0.75 || ev.LevelR > 0.75 {
				m.glowUntil = m.now.Add(120 * time.Millisecond)
			}
		}
	case engine.PhaseDone:
		m.result = ev.Result
		m.cancel = nil
		m.revealed = len(m.amps)
		if ev.Result != nil {
			copy(m.amps, ev.Result.Amps)
			m.remember(ev.Result)
		}
		return m, nil
	}
	if ev.Log != "" && !strings.HasPrefix(ev.Log, "[info]") && !strings.HasPrefix(ev.Log, "[debug]") {
		m.logLine = ev.Log
	}
	return m, waitEvent(m.events, m.ripGen)
}

func (m *Model) remember(r *engine.Result) {
	if m.store == nil || r == nil {
		return
	}
	e := crypt.Entry{
		ID: fmt.Sprintf("%s-%s-%d", m.meta.ID, r.Format, time.Now().UnixNano()), Title: m.meta.Title, Artist: m.meta.Artist, URL: m.url, Extractor: m.meta.Extractor,
		Path: r.Path, Format: string(r.Format), Bitrate: r.Bitrate, Size: r.Size, Duration: r.Duration,
		When: time.Now(), Spark: r.Spark, Source: r.SourcePath,
	}
	_ = m.store.Add(e)
	m.entries = m.store.List()
}

// keyLog appends every key message to $RIPR_KEYLOG when set. Debug aid.
var keyLog = os.Getenv("RIPR_KEYLOG")

func (m *Model) onKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	// a line feed is enter: terminals hand one over before raw mode is set
	if k.Type == tea.KeyCtrlJ {
		k = tea.KeyMsg{Type: tea.KeyEnter}
	}
	key := k.String()
	if keyLog != "" {
		if f, err := os.OpenFile(keyLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			fmt.Fprintf(f, "stage=%d type=%d paste=%v runes=%q str=%q\n", m.st, k.Type, k.Paste, string(k.Runes), key)
			f.Close()
		}
	}
	if key == "ctrl+c" {
		return m, m.quit()
	}
	if m.st == stBoot {
		// any key skips the boot; a pasted link or typed text is kept
		m.bootSkip = true
		m.setStage(stIdle)
		m.prev = nil
		// keep typed or pasted text; a raw paste can arrive one byte at a
		// time, so even a single character is kept unless it is a command
		if k.Paste || (k.Type == tea.KeyRunes && !strings.Contains("qcd,", string(k.Runes))) {
			return m.onIdleKey(k)
		}
		return m, nil
	}
	if m.editing {
		return m.onEditKey(k)
	}
	if m.filtering {
		return m.onFilterKey(k)
	}
	if m.linkEdit {
		return m.onLinkEditKey(k)
	}
	// a pasted link on prep or done starts over with it; "/" opens the box
	if (m.st == stPrep || m.st == stDone) && (isPaste(k) || key == "/") {
		m.beginLinkEdit(isPaste(k))
		if isPaste(k) {
			return m.onLinkEditKey(k)
		}
		return m, nil
	}
	switch m.st {
	case stIdle:
		return m.onIdleKey(k)
	case stPrep:
		return m.onPrepKey(k)
	case stRip:
		if key == "esc" {
			m.stopRip()
			m.setStage(stPrep) // clears the notice, so say after
			m.say("cancelled")
		}
		return m, nil
	case stDone:
		return m.onDoneKey(k)
	case stCrypt:
		return m.onCryptKey(k)
	case stSettings:
		return m.onSettingsKey(k)
	case stDoctor:
		return m.onDoctorKey(k)
	}
	return m, nil
}

// isPaste treats a bracketed paste and a burst of typed characters the
// same way: terminals without bracketed paste send the latter.
func isPaste(k tea.KeyMsg) bool {
	return k.Paste || (k.Type == tea.KeyRunes && len(k.Runes) > 1)
}

// beginLinkEdit puts the cursor in the link box. A paste replaces the
// link; "/" keeps the current one for editing.
func (m *Model) beginLinkEdit(replace bool) {
	m.linkEdit = true
	m.probeErr = ""
	if replace {
		m.in.SetValue("")
	} else {
		m.in.SetValue(m.url)
	}
	m.in.CursorEnd()
	m.in.Focus()
}

func (m *Model) onLinkEditKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc":
		m.linkEdit = false
		m.in.SetValue("")
		m.in.Blur()
		return m, nil
	case "enter":
		typed := strings.TrimSpace(m.in.Value())
		if looksLikeURL(typed) && !m.probing {
			m.probing = true
			m.linkEdit = false
			m.in.SetValue("")
			m.in.Blur()
			return m, m.probe(typed, false)
		}
		if typed != "" {
			m.probeErr = "that doesn't look like a link"
		}
		return m, nil
	}
	if isPaste(k) {
		m.in.SetValue("") // a paste replaces what was there
	}
	var cmd tea.Cmd
	m.in, cmd = m.in.Update(k)
	if isPaste(k) && looksLikeURL(strings.TrimSpace(m.in.Value())) && !m.probing {
		m.probing = true
		m.linkEdit = false
		u := strings.TrimSpace(m.in.Value())
		m.in.SetValue("")
		m.in.Blur()
		return m, tea.Batch(cmd, m.probe(u, false))
	}
	return m, cmd
}

func (m *Model) stopRip() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
}

// quit cancels a running rip and gives the engine a moment to kill its
// children and remove the partial file before the process exits.
func (m *Model) quit() tea.Cmd {
	m.stopRip()
	done := m.ripDone
	return func() tea.Msg {
		if done != nil {
			select {
			case <-done:
			case <-time.After(2 * time.Second):
			}
		}
		return tea.Quit()
	}
}

func (m *Model) onIdleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	typed := strings.TrimSpace(m.in.Value())
	// commands only while nothing is typed; once you type, keys are yours
	if typed == "" && !k.Paste {
		switch key {
		case "q":
			return m, m.quit()
		case "c":
			m.setStage(stCrypt)
			return m, nil
		case ",":
			m.setStage(stSettings)
			return m, nil
		case "enter":
			if m.clipURL != "" && m.meta.ID != "" {
				m.url = m.clipURL
				m.setStage(stPrep)
				m.field = fFormat
			}
			return m, nil
		case "tab":
			if m.clipURL != "" && m.meta.ID != "" {
				m.url = m.clipURL
				m.setStage(stPrep)
				m.field = fFormat
			}
			return m, nil
		case "d":
			m.setStage(stDoctor)
			return m, nil
		}
	}
	switch key {
	case "esc":
		m.in.SetValue("")
		m.probeErr = ""
		return m, nil
	case "enter":
		if looksLikeURL(typed) && !m.probing {
			m.probing = true
			m.probeErr = ""
			return m, m.probe(typed, false)
		}
		if typed != "" {
			m.probeErr = "that doesn't look like a link"
		}
		return m, nil
	}
	if isPaste(k) {
		m.in.SetValue("") // a paste replaces what was there
	}
	var cmd tea.Cmd
	m.in, cmd = m.in.Update(k)
	// a pasted link goes straight to probe
	if isPaste(k) && looksLikeURL(strings.TrimSpace(m.in.Value())) && !m.probing {
		m.probing = true
		return m, tea.Batch(cmd, m.probe(strings.TrimSpace(m.in.Value()), false))
	}
	return m, cmd
}

func (m *Model) onPrepKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc":
		m.setStage(stIdle)
		return m, nil
	case "tab":
		m.field = (m.field + 1) % fCount
	case "shift+tab":
		m.field = (m.field + fCount - 1) % fCount
	case "left", "h":
		m.adjust(-1)
	case "right", "l":
		m.adjust(1)
	case " ":
		m.toggle()
	case "o":
		return m, m.pickFolder()
	case "n":
		m.beginEdit("name")
	case "c":
		m.setStage(stCrypt)
	case ",":
		m.setStage(stSettings)
	case "enter":
		// enter on prep always rips; the folder and name have their own keys
		m.setStage(stRip)
		return m, m.startRip()
	}
	return m, nil
}

// pickFolder opens the folder dialog, or the text editor where there is none.
// It never moves the focus, so the next enter still rips.
func (m *Model) pickFolder() tea.Cmd {
	if hasFolderPicker() {
		m.say("choose a folder in the dialog")
		return chooseFolder(expandHome(m.dir), "dir")
	}
	m.beginEdit("dir")
	return nil
}

func (m *Model) adjust(d int) {
	switch m.field {
	case fFormat:
		n := len(engine.Formats)
		m.fmtIdx = (m.fmtIdx + d + n) % n
	case fQuality:
		n := len(qualities)
		m.qIdx = (m.qIdx + d + n) % n
	default:
		m.toggle()
	}
}

func (m *Model) toggle() {
	switch m.field {
	case fCover:
		if !coverArtPossible(m.format()) {
			m.say("no place for a picture in " + string(m.format()))
			return
		}
		m.cover = !m.cover
	case fTags:
		m.tags = !m.tags
	case fNormalize:
		m.normalize = !m.normalize
	}
}

// beginEdit opens the text editor on one of: dir, name (prep) or
// cfgdir, cfgname (settings).
func (m *Model) beginEdit(target string) {
	m.editing = true
	m.editTarget = target
	switch target {
	case "dir":
		m.edit.SetValue(m.dir)
	case "name":
		m.edit.SetValue(m.name)
	case "cfgdir":
		m.edit.SetValue(m.cfg.Dir)
	case "cfgname":
		m.edit.SetValue(m.cfg.Template)
	}
	m.edit.CursorEnd()
	m.edit.Focus()
}

func (m *Model) onEditKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc":
		m.editing = false
		m.edit.Blur()
		return m, nil
	case "enter":
		v := strings.TrimSpace(m.edit.Value())
		if v != "" {
			switch m.editTarget {
			case "dir":
				m.dir = v
			case "name":
				m.name = v
			case "cfgdir":
				m.cfg.Dir = v
				m.dir = v
			case "cfgname":
				m.cfg.Template = v
				m.name = v
			}
		}
		m.editing = false
		m.edit.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.edit, cmd = m.edit.Update(k)
	return m, cmd
}

func (m *Model) onDoneKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	r := m.result
	switch k.String() {
	case "p":
		if r != nil {
			return m, openPath(r.Path)
		}
	case "r":
		if r != nil {
			return m, revealPath(r.Path)
		}
	case "y":
		if r != nil {
			return m, copyText(r.Path)
		}
	case "a":
		m.setStage(stPrep)
		m.field = fFormat
	case "c":
		m.setStage(stCrypt)
	case ",":
		m.setStage(stSettings)
	case "enter", "esc":
		m.url = ""
		m.clipURL = ""
		m.meta = engine.Meta{}
		m.thumb = nil
		m.result = nil
		m.setStage(stIdle)
		return m, readClipboard()
	case "q":
		return m, m.quit()
	}
	return m, nil
}

func (m *Model) visibleEntries() []crypt.Entry {
	q := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	if q == "" {
		return m.entries
	}
	var out []crypt.Entry
	for _, e := range m.entries {
		if strings.Contains(strings.ToLower(e.Title+" "+e.Artist+" "+e.Format), q) {
			out = append(out, e)
		}
	}
	return out
}

func (m *Model) onCryptKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.visibleEntries()
	var cur *crypt.Entry
	if m.sel >= 0 && m.sel < len(items) {
		cur = &items[m.sel]
	}
	switch k.String() {
	case "esc", "c":
		m.setStage(stIdle)
	case "q":
		return m, m.quit()
	case "/":
		m.filtering = true
		m.filter.Focus()
	case "down", "j":
		if m.sel < len(items)-1 {
			m.sel++
		}
	case "up", "k":
		if m.sel > 0 {
			m.sel--
		}
	case "p", "enter":
		if cur != nil {
			return m, openPath(cur.Path)
		}
	case "r":
		if cur != nil {
			return m, revealPath(cur.Path)
		}
	case "y":
		if cur != nil {
			return m, copyText(cur.Path)
		}
	case "a":
		if cur != nil {
			m.url = cur.URL
			m.probing = true
			m.setStage(stIdle)
			return m, m.probe(cur.URL, false)
		}
	case "x":
		if cur != nil && m.store != nil {
			_ = m.store.Remove(cur.ID)
			m.entries = m.store.List()
			if m.sel >= len(m.visibleEntries()) && m.sel > 0 {
				m.sel--
			}
			m.say("forgotten. the file is still there.")
		}
	}
	return m, nil
}

func (m *Model) onFilterKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc":
		m.filter.SetValue("")
		m.filtering = false
		m.filter.Blur()
		m.sel = 0
		return m, nil
	case "enter":
		m.filtering = false
		m.filter.Blur()
		m.sel = 0
		return m, nil
	}
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(k)
	m.sel = 0
	return m, cmd
}

var settingRows = []string{"default vessel", "default quality", "save to", "filename", "keep source for re-rips", "cover art", "loudness normalize", "theme", "wordmark", "density", "mascot", "motion"}

func (m *Model) settingValue(i int) string {
	c := m.cfg
	switch i {
	case 0:
		return c.Format
	case 1:
		return itoa(c.Quality) + " kbps"
	case 2:
		return c.Dir
	case 3:
		return c.Template
	case 4:
		return itoa(c.CacheDays) + " days"
	case 5:
		return onoff(c.CoverArt, "embed", "off")
	case 6:
		return onoff(c.Normalize, "on", "off")
	case 7:
		return c.Theme
	case 8:
		return c.Wordmark
	case 9:
		return c.Density
	case 10:
		return onoff(c.Mascot, "on", "off")
	case 11:
		return c.Motion
	}
	return ""
}

func (m *Model) cycleSetting(i, d int) {
	c := &m.cfg
	cyc := func(opts []string, cur string) string {
		idx := indexOf(opts, cur)
		return opts[(idx+d+len(opts))%len(opts)]
	}
	switch i {
	case 0:
		c.Format = cyc(formatNames(), c.Format)
		m.fmtIdx = indexOf(formatNames(), c.Format)
	case 1:
		idx := 3
		for j, q := range qualities {
			if q == c.Quality {
				idx = j
			}
		}
		idx = (idx + d + len(qualities)) % len(qualities)
		c.Quality = qualities[idx]
		m.qIdx = idx
	case 4:
		c.CacheDays += d
		if c.CacheDays < 0 {
			c.CacheDays = 0
		}
		if c.CacheDays > 90 {
			c.CacheDays = 90
		}
	case 5:
		c.CoverArt = !c.CoverArt
		m.cover = c.CoverArt
	case 6:
		c.Normalize = !c.Normalize
		m.normalize = c.Normalize
	case 7:
		c.Theme = cyc([]string{"crypt", "bone", "mono"}, c.Theme)
		m.pal = Theme(c.Theme)
		m.rend = newRenderer(m.pal)
	case 8:
		c.Wordmark = cyc([]string{"dots", "block", "plain"}, c.Wordmark)
	case 9:
		c.Density = cyc([]string{"comfortable", "compact"}, c.Density)
	case 10:
		c.Mascot = !c.Mascot
	case 11:
		c.Motion = cyc([]string{"full", "reduced"}, c.Motion)
	}
}

func (m *Model) onSettingsKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc", ",":
		if err := m.cfg.Save(); err != nil {
			m.say("couldn't save: " + err.Error())
		} else {
			m.say("saved")
		}
		m.setStage(stIdle)
	case "q":
		_ = m.cfg.Save()
		return m, m.quit()
	case "down", "j":
		if m.setSel < len(settingRows)-1 {
			m.setSel++
		}
	case "up", "k":
		if m.setSel > 0 {
			m.setSel--
		}
	case "left", "h":
		m.cycleSetting(m.setSel, -1)
	case "right", "l", "enter", " ":
		switch m.setSel {
		case 2:
			if hasFolderPicker() && k.String() == "enter" {
				m.say("choose a folder in the dialog")
				return m, chooseFolder(expandHome(m.cfg.Dir), "cfgdir")
			}
			m.beginEdit("cfgdir")
		case 3:
			m.beginEdit("cfgname")
		default:
			m.cycleSetting(m.setSel, 1)
		}
	}
	return m, nil
}

func (m *Model) onDoctorKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "q", "esc":
		return m, m.quit()
	case "enter":
		if !m.installing {
			m.installing = true
			m.installErr = ""
			return m, install(m.tools.Missing)
		}
	case "r":
		return m, runDoctor()
	}
	return m, nil
}

func onoff(b bool, on, off string) string {
	if b {
		return on
	}
	return off
}

func itoa(i int) string { return strconv.Itoa(i) }

func configPath() string { return config.Path() }
