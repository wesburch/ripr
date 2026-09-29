package tui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"

	"ripr/internal/engine"
	"ripr/internal/tui/draw"
)

// message types
type tickMsg time.Time
type probedMsg struct {
	url  string
	meta engine.Meta
	err  error
	clip bool
}
type thumbMsg struct{ t *draw.Thumb }
type clipMsg struct{ url string }
type evMsg engine.Event
type doctorMsg engine.Tools
type installMsg struct{ err error }
type noticeMsg string

const (
	fast = 33 * time.Millisecond
	slow = 100 * time.Millisecond
)

func tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// looksLikeURL is the bar for accepting a pasted or clipboard string.
func looksLikeURL(s string) bool {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return false
	}
	u, err := url.Parse(s)
	return err == nil && u.Host != "" && !strings.ContainsAny(s, " \n\t")
}

func readClipboard() tea.Cmd {
	return func() tea.Msg {
		s, err := clipboard.ReadAll()
		if err != nil || !looksLikeURL(s) {
			return nil
		}
		return clipMsg{url: strings.TrimSpace(s)}
	}
}

func (m *Model) probe(u string, clip bool) tea.Cmd {
	tools := m.tools
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		meta, err := engine.Probe(ctx, tools, u)
		return probedMsg{url: u, meta: meta, err: err, clip: clip}
	}
}

// fetchThumb downloads the thumbnail and reduces it to a 12×3 block.
// YouTube's default thumbnail is webp, which the standard library cannot
// decode, so the jpg variant is requested instead.
func fetchThumb(meta engine.Meta) tea.Cmd {
	u := meta.Thumbnail
	if u == "" {
		return nil
	}
	if meta.Extractor == "youtube" && meta.ID != "" {
		u = "https://i.ytimg.com/vi/" + meta.ID + "/hqdefault.jpg"
	}
	return func() tea.Msg {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(u)
		if err != nil {
			return nil
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil
		}
		t, err := draw.DecodeThumb(resp.Body, 12, 3)
		if err != nil {
			return nil
		}
		return thumbMsg{t: t}
	}
}

// startRip launches the engine and returns the command that waits for its
// first event.
func (m *Model) startRip() tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	ch := make(chan engine.Event, 256)
	m.events = ch
	m.ripGen++
	gen := m.ripGen
	done := make(chan struct{})
	m.ripDone = done
	cols := m.waveCells() * 2
	m.amps = make([]float32, cols)
	m.revealed = 0
	m.phase = engine.PhasePull
	m.pullP, m.timeS, m.speedX = 0, 0, 0
	m.levelL, m.levelR = 0, 0
	m.logLine = ""
	m.ripErr = ""
	m.result = nil
	m.ripStart = m.now
	job := engine.Job{
		URL: m.url, Meta: m.meta, Format: m.format(), Bitrate: m.quality(),
		OutDir: expandHome(m.dir), Template: m.name,
		CoverArt: m.cover, Tags: m.tags, Normalize: m.normalize,
		CacheDir: cacheDir(), CacheDays: m.cfg.CacheDays, Columns: cols,
	}
	tools := m.tools
	go func() {
		defer close(done)
		err := engine.Rip(ctx, tools, job, ch)
		// a cancel is the user's choice, not an error
		if err != nil && !errors.Is(err, context.Canceled) {
			select {
			case ch <- engine.Event{Column: -1, Err: err}:
			default:
			}
		}
	}()
	return waitEvent(ch, gen)
}

// waitEvent blocks for one event, then drains whatever else is already
// queued so a fast engine never outruns a 30 fps screen. The batch carries
// the rip generation so events from a cancelled rip are dropped.
func waitEvent(ch <-chan engine.Event, gen int) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		batch := []engine.Event{ev}
		for len(batch) < 64 {
			select {
			case more, ok := <-ch:
				if !ok {
					return evBatchMsg{gen: gen, evs: batch}
				}
				batch = append(batch, more)
			default:
				return evBatchMsg{gen: gen, evs: batch}
			}
		}
		return evBatchMsg{gen: gen, evs: batch}
	}
}

type evBatchMsg struct {
	gen int
	evs []engine.Event
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := homeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

func openPath(p string) tea.Cmd {
	return func() tea.Msg {
		var c *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			c = exec.Command("open", p)
		default:
			c = exec.Command("xdg-open", p)
		}
		if err := c.Start(); err != nil {
			return noticeMsg("couldn't open: " + err.Error())
		}
		return noticeMsg("playing")
	}
}

func revealPath(p string) tea.Cmd {
	return func() tea.Msg {
		var c *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			c = exec.Command("open", "-R", p)
		default:
			c = exec.Command("xdg-open", filepath.Dir(p))
		}
		if err := c.Start(); err != nil {
			return noticeMsg("couldn't reveal: " + err.Error())
		}
		return noticeMsg("revealed in finder")
	}
}

func copyText(s string) tea.Cmd {
	return func() tea.Msg {
		if err := clipboard.WriteAll(s); err != nil {
			return noticeMsg("couldn't copy: " + err.Error())
		}
		return noticeMsg("path copied")
	}
}

func runDoctor() tea.Cmd {
	return func() tea.Msg { return doctorMsg(engine.Doctor()) }
}

// install runs brew for the missing tools. It is the one place ripr
// changes the machine, and only on an explicit ↵ from the doctor screen.
func install(missing []string) tea.Cmd {
	return func() tea.Msg {
		if len(missing) == 0 {
			return installMsg{}
		}
		brew, err := exec.LookPath("brew")
		if err != nil {
			return installMsg{err: errors.New("homebrew not found; install ffmpeg and yt-dlp yourself")}
		}
		args := append([]string{"install"}, missing...)
		out, err := exec.Command(brew, args...).CombinedOutput()
		if err != nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			return installMsg{err: fmt.Errorf("brew: %s", lines[len(lines)-1])}
		}
		return installMsg{}
	}
}
