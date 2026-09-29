package engine

import (
	"bufio"
	"context"
	"io"
	"strconv"
	"strings"
)

// send delivers ev unless ctx is cancelled first.
func send(ctx context.Context, ch chan<- Event, ev Event) bool {
	select {
	case ch <- ev:
		return true
	case <-ctx.Done():
		return false
	}
}

func scanLines(r io.Reader, fn func(string)) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		fn(sc.Text())
	}
	// Drain so the child never blocks on a full pipe after a scanner error.
	io.Copy(io.Discard, r)
}

func atoiNA(s string) int64 {
	if s == "" || s == "NA" || s == "None" {
		return 0
	}
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int64(f)
	}
	return 0
}

func atofNA(s string) float64 {
	if s == "" || s == "NA" || s == "None" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// parseRiprLine parses "RIPR <downloaded> <total> <estimate> <speed>".
func parseRiprLine(line string) (ev Event, ok bool) {
	f := strings.Fields(line)
	if len(f) != 5 || f[0] != "RIPR" {
		return Event{}, false
	}
	ev = newEvent(PhasePull)
	ev.Bytes = atoiNA(f[1])
	ev.TotalBytes = atoiNA(f[2])
	if ev.TotalBytes == 0 {
		ev.TotalBytes = atoiNA(f[3])
	}
	ev.Speed = atofNA(f[4])
	if ev.TotalBytes > 0 {
		ev.Progress = float64(ev.Bytes) / float64(ev.TotalBytes)
		if ev.Progress > 1 {
			ev.Progress = 1
		}
	}
	return ev, true
}

// parseKV splits an ffmpeg -progress line. ok is false for anything that is
// not key=value (those are ffmpeg diagnostics).
func parseKV(line string) (key, val string, ok bool) {
	k, v, found := strings.Cut(line, "=")
	if !found || k == "" {
		return "", "", false
	}
	for _, r := range k {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == ':') {
			return "", "", false
		}
	}
	return k, strings.TrimSpace(v), true
}

// progressState accumulates one ffmpeg -progress block.
type progressState struct {
	Time   float64
	SpeedX float64
	End    bool
}

// feed consumes one kv pair and reports true when a block just completed.
func (p *progressState) feed(k, v string) bool {
	switch k {
	case "out_time_us", "out_time_ms":
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
			p.Time = float64(n) / 1e6
		}
	case "speed":
		p.SpeedX = atofNA(strings.TrimSuffix(v, "x"))
	case "progress":
		p.End = v == "end"
		return true
	}
	return false
}
