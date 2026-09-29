package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"ripr/internal/engine"
)

// "o" on prep must hand back a command (the picker) or open the text
// editor, never silently do nothing. The command is not run here: it would
// open a real dialog.
func TestFolderKeyOnPrep(t *testing.T) {
	m := testModel(t)
	m.st = stPrep
	m.meta = engine.Meta{ID: "x", Title: "t", Duration: 10}
	m.url = "https://x"
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	if hasFolderPicker() {
		if cmd == nil || m.editing {
			t.Fatal("expected the folder picker command")
		}
	} else if !m.editing || m.editTarget != "dir" {
		t.Fatal("expected the text editor for the folder")
	}
	// a picked folder is applied to this rip only
	_, _ = m.Update(folderMsg{path: "/tmp/somewhere", target: "dir"})
	if m.dir != "/tmp/somewhere" || m.cfg.Dir == "/tmp/somewhere" {
		t.Fatalf("dir=%q cfg.Dir=%q", m.dir, m.cfg.Dir)
	}
	// a cancelled dialog keeps the folder
	_, _ = m.Update(folderMsg{path: "", target: "dir"})
	if m.dir != "/tmp/somewhere" {
		t.Fatal("cancel changed the folder")
	}
	// the default is only changed from settings
	_, _ = m.Update(folderMsg{path: "/tmp/default", target: "cfgdir"})
	if m.cfg.Dir != "/tmp/default" || m.dir != "/tmp/default" {
		t.Fatal("settings pick not applied to config")
	}
}
