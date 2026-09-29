package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var skipExt = map[string]bool{
	"jpg": true, "jpeg": true, "png": true, "webp": true,
	"part": true, "ytdl": true, "temp": true,
}

// findSource returns a cached source for id, or "".
func findSource(cacheDir, id string) string {
	if id == "" {
		return ""
	}
	items, err := os.ReadDir(cacheDir)
	if err != nil {
		return ""
	}
	for _, it := range items {
		name := it.Name()
		if !it.Type().IsRegular() || !strings.HasPrefix(name, id+".") {
			continue
		}
		ext := name[len(id)+1:]
		if ext == "" || strings.Contains(ext, ".") || skipExt[strings.ToLower(ext)] {
			continue
		}
		return filepath.Join(cacheDir, name)
	}
	return ""
}

func coverPath(cacheDir, id string) string {
	p := filepath.Join(cacheDir, id+".jpg")
	if st, err := os.Stat(p); err == nil && st.Size() > 0 {
		return p
	}
	return ""
}

func pull(ctx context.Context, tools Tools, job Job, events chan<- Event) (src, cover string, err error) {
	id := job.Meta.ID
	if id == "" {
		return "", "", errors.New("pull: job has no media id")
	}
	if tools.YtDlp == "" {
		return "", "", errors.New("pull: yt-dlp not found")
	}
	if err := os.MkdirAll(job.CacheDir, 0o755); err != nil {
		return "", "", err
	}
	if src = findSource(job.CacheDir, id); src != "" {
		ev := newEvent(PhasePull)
		ev.Progress, ev.Log = 1, "source from cache"
		if !send(ctx, events, ev) {
			return "", "", ctx.Err()
		}
		if job.CoverArt && coverPath(job.CacheDir, id) == "" {
			fetchThumb(ctx, tools, job) // best effort
		}
		return src, coverPath(job.CacheDir, id), nil
	}

	args := []string{
		"-f", "bestaudio/best", "--no-playlist", "--no-warnings", "--newline",
		"--progress-template", "download:RIPR %(progress.downloaded_bytes)s %(progress.total_bytes)s %(progress.total_bytes_estimate)s %(progress.speed)s",
		"-o", filepath.Join(job.CacheDir, "%(id)s.%(ext)s"),
	}
	if job.CoverArt {
		args = append(args, "--write-thumbnail", "--convert-thumbnails", "jpg",
			"-o", "thumbnail:"+filepath.Join(job.CacheDir, "%(id)s.%(ext)s"))
	}
	args = append(args, "--", job.URL)

	cmd := exec.CommandContext(ctx, tools.YtDlp, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", "", err
	}
	if err := cmd.Start(); err != nil {
		return "", "", err
	}

	var (
		mu      sync.Mutex
		lastLog time.Time
		lastErr string
		wg      sync.WaitGroup
	)
	handle := func(line string) {
		line = strings.TrimSpace(line)
		if line == "" {
			return
		}
		if ev, ok := parseRiprLine(line); ok {
			send(ctx, events, ev)
			return
		}
		mu.Lock()
		if strings.HasPrefix(line, "ERROR") {
			lastErr = line
		}
		emit := strings.HasPrefix(line, "[") && time.Since(lastLog) >= 100*time.Millisecond
		if emit {
			lastLog = time.Now()
		}
		mu.Unlock()
		if emit {
			ev := newEvent(PhasePull)
			ev.Log = line
			send(ctx, events, ev)
		}
	}
	for _, r := range []interface{ Read([]byte) (int, error) }{stdout, stderr} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			scanLines(r, handle)
		}()
	}
	wg.Wait()
	werr := cmd.Wait()
	if ctx.Err() != nil {
		return "", "", ctx.Err()
	}
	if werr != nil {
		if lastErr == "" {
			lastErr = werr.Error()
		}
		return "", "", fmt.Errorf("pull: yt-dlp failed: %s", lastErr)
	}
	src = findSource(job.CacheDir, id)
	if src == "" {
		return "", "", errors.New("pull: yt-dlp finished but no source file was found")
	}
	done := newEvent(PhasePull)
	done.Progress = 1
	if !send(ctx, events, done) {
		return "", "", ctx.Err()
	}
	return src, coverPath(job.CacheDir, id), nil
}

// fetchThumb grabs only the cover for an already-cached source.
func fetchThumb(ctx context.Context, tools Tools, job Job) {
	cmd := exec.CommandContext(ctx, tools.YtDlp, "--skip-download", "--no-playlist", "--no-warnings",
		"--write-thumbnail", "--convert-thumbnails", "jpg",
		"-o", "thumbnail:"+filepath.Join(job.CacheDir, "%(id)s.%(ext)s"), "--", job.URL)
	_ = cmd.Run()
}
