package engine

import (
	"os/exec"
	"strings"
)

func doctor() Tools {
	var t Tools
	if p, err := exec.LookPath("yt-dlp"); err == nil {
		t.YtDlp = p
		if out, err := exec.Command(p, "--version").Output(); err == nil {
			t.YtDlpVersion = strings.TrimSpace(string(out))
		}
	} else {
		t.Missing = append(t.Missing, "yt-dlp")
	}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		t.Ffmpeg = p
		if out, err := exec.Command(p, "-version").Output(); err == nil {
			t.FfmpegVersion = parseFfmpegVersion(string(out))
		}
	} else {
		t.Missing = append(t.Missing, "ffmpeg")
	}
	return t
}

// parseFfmpegVersion reads the token after "ffmpeg version" on the first line.
func parseFfmpegVersion(out string) string {
	line, _, _ := strings.Cut(out, "\n")
	f := strings.Fields(line)
	for i, w := range f {
		if w == "version" && i+1 < len(f) {
			return f[i+1]
		}
	}
	return ""
}
