package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"ripr/internal/config"
	"ripr/internal/engine"
)

var ansi = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

func plainRows(s string) []string {
	return strings.Split(ansi.ReplaceAllString(s, ""), "\n")
}

func testModel(t *testing.T) *Model {
	t.Helper()
	cfg := config.Default()
	cfg.Motion = "reduced" // no boot, no tear
	m := New(Options{Config: cfg, Tools: engine.Tools{YtDlp: "/x/yt-dlp", Ffmpeg: "/x/ffmpeg", YtDlpVersion: "1", FfmpegVersion: "2"}})
	m.w, m.h = 100, 36
	m.sized = true
	m.now = time.Now()
	return m
}

func TestIdleViewPaintsEveryRegion(t *testing.T) {
	m := testModel(t)
	rows := plainRows(m.View())
	if len(rows) != 36 {
		t.Fatalf("got %d rows, want 36", len(rows))
	}
	if !strings.Contains(rows[0], "ripr") || !strings.Contains(rows[0], "~/Music/ripr") {
		t.Fatalf("header row wrong: %q", rows[0])
	}
	if !strings.HasPrefix(strings.TrimSpace(rows[1]), "┌") || !strings.Contains(rows[2], "paste a link") || !strings.HasPrefix(strings.TrimSpace(rows[3]), "└") {
		t.Fatalf("input box wrong:\n%q\n%q\n%q", rows[1], rows[2], rows[3])
	}
	if !strings.Contains(rows[5], "signal") || !strings.Contains(rows[6], "──") {
		t.Fatalf("signal section wrong: %q / %q", rows[5], rows[6])
	}
	if !strings.Contains(rows[26], "format") || !strings.Contains(rows[26], "mp3") {
		t.Fatalf("format row wrong: %q", rows[26])
	}
	if !strings.Contains(rows[35], "quit") || !strings.Contains(rows[35], "ready") {
		t.Fatalf("footer wrong: %q", rows[35])
	}
	for i, r := range rows {
		if len([]rune(r)) != 100 {
			t.Fatalf("row %d is %d cells wide, want 100: %q", i, len([]rune(r)), r)
		}
	}
}

func TestPrepAndDoneViews(t *testing.T) {
	m := testModel(t)
	m.meta = engine.Meta{ID: "x", Title: "Something About Us", Artist: "Daft Punk", Duration: 232, Extractor: "youtube", Year: "2003", WebpageURL: "https://www.youtube.com/watch?v=x"}
	m.url = m.meta.WebpageURL
	m.st = stPrep
	rows := plainRows(m.View())
	if !strings.Contains(rows[7], "Something About Us") || !strings.Contains(rows[8], "Daft Punk") {
		t.Fatalf("prep title block wrong: %q / %q", rows[7], rows[8])
	}
	if !strings.Contains(rows[30], "rip it") {
		t.Fatalf("prep button row wrong: %q", rows[30])
	}
	m.st = stDone
	m.result = &engine.Result{Path: "/tmp/Daft Punk – Something About Us.mp3", Size: 9_500_000, Duration: 232, Elapsed: 11300 * time.Millisecond, Format: engine.MP3, Bitrate: 320, Amps: make([]float32, 192)}
	m.amps = make([]float32, 192)
	m.revealed = 192
	rows = plainRows(m.View())
	if !strings.Contains(rows[7], "Something About Us.mp3") || !strings.Contains(rows[22], "saved") {
		t.Fatalf("done view wrong: %q / %q", rows[7], rows[22])
	}
}
