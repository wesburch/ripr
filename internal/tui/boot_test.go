package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"ripr/internal/config"
	"ripr/internal/engine"
)

// a link pasted while the boot is still playing must skip the boot, land in
// the box, and be looked up on enter
func TestPasteDuringBootIsKept(t *testing.T) {
	cfg := config.Default()
	m := New(Options{Config: cfg, Tools: engine.Tools{YtDlp: "/x/yt-dlp", Ffmpeg: "/x/ffmpeg"}})
	m.now = time.Now()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 36})
	if m.st != stBoot {
		t.Fatalf("expected boot, got %v", m.st)
	}
	url := "https://www.youtube.com/watch?v=jNQXAC9IVRw"
	// a burst of characters is a paste: it lands in the box and is looked up at once
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(url)})
	if m.st != stIdle {
		t.Fatalf("paste did not skip boot: %v", m.st)
	}
	if got := m.in.Value(); got != url {
		t.Fatalf("box has %q", got)
	}
	if !m.probing || cmd == nil {
		t.Fatalf("paste did not start a probe (probing=%v cmd=%v)", m.probing, cmd != nil)
	}
	// typing one character at a time during boot keeps the characters too
	m2 := New(Options{Config: cfg, Tools: engine.Tools{YtDlp: "/x/yt-dlp", Ffmpeg: "/x/ffmpeg"}})
	m2.now = time.Now()
	_, _ = m2.Update(tea.WindowSizeMsg{Width: 100, Height: 36})
	for _, r := range "https://a.b/c" {
		_, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if got := m2.in.Value(); got != "https://a.b/c" {
		t.Fatalf("typed text lost during boot: %q", got)
	}
	_, cmd = m2.Update(tea.KeyMsg{Type: tea.KeyCtrlJ}) // a line feed is enter
	if !m2.probing || cmd == nil {
		t.Fatal("line-feed enter did not start a probe")
	}
}

func TestPasteReplacesBox(t *testing.T) {
	m := testModel(t)
	m.in.SetValue("https://old.example/x")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("https://new.example/y")})
	if got := m.in.Value(); got != "https://new.example/y" {
		t.Fatalf("paste appended instead of replacing: %q", got)
	}
	if !m.probing || cmd == nil {
		t.Fatal("paste did not start a probe")
	}
}
