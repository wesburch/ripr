package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"ripr/internal/engine"
)

// every size must keep the primary action above the footer rule and keep
// the key hints clear of the status text
func TestSmallSizesKeepButtonVisible(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {80, 27}, {80, 30}, {100, 30}, {120, 40}} {
		m := testModel(t)
		m.w, m.h = sz[0]-1, sz[1]
		m.meta = engine.Meta{ID: "x", Title: "Something About Us", Artist: "Daft Punk", Duration: 232, Extractor: "youtube"}
		m.url = "https://www.youtube.com/watch?v=x"
		m.st = stPrep
		rows := plainRows(m.View())
		if len(rows) != sz[1] {
			t.Fatalf("%v: %d rows", sz, len(rows))
		}
		btn := -1
		for i, r := range rows {
			if strings.Contains(r, "rip it") {
				btn = i
			}
		}
		rule := sz[1] - 3
		if btn < 0 || btn >= rule {
			t.Fatalf("%v: rip button at row %d, footer rule at %d", sz, btn, rule)
		}
		if !strings.HasPrefix(strings.TrimSpace(rows[rule]), "─") {
			t.Fatalf("%v: footer rule missing: %q", sz, rows[rule])
		}
		keys := rows[sz[1]-1]
		if strings.Contains(keys, "backready") || strings.Contains(keys, "nameready") {
			t.Fatalf("%v: hints collide with status: %q", sz, keys)
		}
		// nothing may be painted where the mascot sits (last 12 cells of the last two rows)
		for _, y := range []int{sz[1] - 2, sz[1] - 1} {
			tail := []rune(rows[y])
			if len(tail) > sz[0]-13 {
				for _, ch := range tail[sz[0]-13:] {
					if ch != ' ' && ch != '▄' && ch != '▀' {
						t.Fatalf("%v: text under the mascot on row %d: %q", sz, y, rows[y])
					}
				}
			}
		}
		for i, r := range rows {
			if w := len([]rune(r)); w > sz[0]-1 {
				t.Fatalf("%v: row %d is %d cells", sz, i, w)
			}
		}
	}
}

func TestCancelledRipIsNotAnError(t *testing.T) {
	m := testModel(t)
	m.st = stRip
	m.ripGen = 3
	// a stale batch from an older rip must be ignored
	_, _ = m.Update(evBatchMsg{gen: 2, evs: []engine.Event{{Column: -1, Err: errBoom}}})
	if m.ripErr != "" {
		t.Fatal("stale error applied")
	}
	// esc goes back to prep with a plain notice, not an error
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.st != stPrep || m.notice != "cancelled" || m.ripErr != "" {
		t.Fatalf("after esc: stage=%v notice=%q err=%q", m.st, m.notice, m.ripErr)
	}
	// an event batch from the cancelled rip arriving late is dropped
	_, _ = m.Update(evBatchMsg{gen: 3, evs: []engine.Event{{Column: -1, Err: errBoom}}})
	if m.ripErr != "" || m.notice != "cancelled" {
		t.Fatalf("late batch changed state: err=%q notice=%q", m.ripErr, m.notice)
	}
}

func TestDoneResetsForNewLink(t *testing.T) {
	m := testModel(t)
	m.st = stDone
	m.url = "https://x"
	m.meta = engine.Meta{ID: "x", Title: "t"}
	m.result = &engine.Result{Path: "/tmp/t.mp3"}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.st != stIdle || m.url != "" || m.meta.ID != "" || m.result != nil {
		t.Fatalf("done → enter did not reset: stage=%v url=%q", m.st, m.url)
	}
}

func TestReducedMotionSkipsBootAndAnimation(t *testing.T) {
	m := testModel(t)
	if m.st != stIdle {
		t.Fatal("reduced motion should start at idle")
	}
	m.now = time.Now()
	a := plainRows(m.View())
	m.now = m.now.Add(600 * time.Millisecond)
	b := plainRows(m.View())
	if strings.Join(a, "\n") != strings.Join(b, "\n") {
		t.Fatal("reduced motion frame changed over time")
	}
}

func TestLinkBoxEditableOnPrepAndDone(t *testing.T) {
	for _, st := range []stage{stPrep, stDone} {
		m := testModel(t)
		m.st = st
		m.url = "https://www.youtube.com/watch?v=old"
		m.meta = engine.Meta{ID: "old", Title: "t", Duration: 10}
		m.result = &engine.Result{Path: "/tmp/t.mp3"}
		// "/" opens the box with the current link
		_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
		if !m.linkEdit || m.in.Value() != m.url {
			t.Fatalf("stage %v: '/' did not open the link box with the link", st)
		}
		rows := plainRows(m.View())
		if !strings.Contains(rows[2], "watch?v=old") || !strings.Contains(rows[len(rows)-1], "look it up") {
			t.Fatalf("stage %v: link box not shown as editable: %q / %q", st, rows[2], rows[len(rows)-1])
		}
		// esc backs out without touching the link
		_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		if m.linkEdit || m.url != "https://www.youtube.com/watch?v=old" || m.st != st {
			t.Fatalf("stage %v: esc changed state", st)
		}
		// a paste starts a lookup of the new link
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("https://www.youtube.com/watch?v=new"), Paste: true})
		if !m.probing || cmd == nil {
			t.Fatalf("stage %v: paste did not start a probe", st)
		}
	}
}

type boom struct{}

func (boom) Error() string { return "boom" }

var errBoom error = boom{}
