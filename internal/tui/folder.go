package tui

import (
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// folderMsg carries the folder the user picked; empty means cancelled.
type folderMsg struct {
	path   string
	target string // "dir" (this rip) or "cfgdir" (the default)
}

// chooseFolder opens the platform's folder picker. On macOS that is the
// real Finder "Choose a folder" sheet via osascript. On Linux it is zenity
// when installed. Where neither exists it returns nothing and the caller
// falls back to typing the path.
func chooseFolder(start, target string) tea.Cmd {
	return func() tea.Msg {
		var out []byte
		var err error
		switch runtime.GOOS {
		case "darwin":
			script := `POSIX path of (choose folder with prompt "Save ripped audio to")`
			if start != "" {
				script = `POSIX path of (choose folder with prompt "Save ripped audio to" default location POSIX file "` + strings.ReplaceAll(start, `"`, `\"`) + `")`
			}
			out, err = exec.Command("osascript", "-e", script).Output()
		default:
			if _, e := exec.LookPath("zenity"); e != nil {
				return folderMsg{target: target}
			}
			args := []string{"--file-selection", "--directory", "--title=Save ripped audio to"}
			if start != "" {
				args = append(args, "--filename="+strings.TrimSuffix(start, "/")+"/")
			}
			out, err = exec.Command("zenity", args...).Output()
		}
		if err != nil {
			return folderMsg{target: target} // cancelled or unavailable
		}
		return folderMsg{path: strings.TrimSuffix(strings.TrimSpace(string(out)), "/"), target: target}
	}
}

// hasFolderPicker reports whether chooseFolder can show a dialog here.
func hasFolderPicker() bool {
	switch runtime.GOOS {
	case "darwin":
		_, err := exec.LookPath("osascript")
		return err == nil
	default:
		_, err := exec.LookPath("zenity")
		return err == nil
	}
}
