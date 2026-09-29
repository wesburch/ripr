package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type probeJSON struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Track       string  `json:"track"`
	Artist      string  `json:"artist"`
	Creator     string  `json:"creator"`
	Uploader    string  `json:"uploader"`
	Channel     string  `json:"channel"`
	ReleaseYear float64 `json:"release_year"`
	UploadDate  string  `json:"upload_date"`
	Duration    float64 `json:"duration"`
	Thumbnail   string  `json:"thumbnail"`
	WebpageURL  string  `json:"webpage_url"`
	ExtractorK  string  `json:"extractor_key"`
	Acodec      string  `json:"acodec"`
	Abr         float64 `json:"abr"`
	Asr         float64 `json:"asr"`
	Channels    float64 `json:"audio_channels"`
	IsLive      bool    `json:"is_live"`
	Chapters    []struct {
		Title string  `json:"title"`
		Start float64 `json:"start_time"`
		End   float64 `json:"end_time"`
	} `json:"chapters"`
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

func normalizeExtractor(key string) string {
	k := strings.ToLower(key)
	if i := strings.Index(k, ":"); i >= 0 {
		k = k[:i]
	}
	for _, suf := range []string{"tab", "user"} {
		if strings.HasSuffix(k, suf) && len(k) > len(suf) {
			k = strings.TrimSuffix(k, suf)
			break
		}
	}
	return k
}

func parseProbe(data []byte) (Meta, error) {
	var p probeJSON
	if err := json.Unmarshal(data, &p); err != nil {
		return Meta{}, fmt.Errorf("probe: parse yt-dlp output: %w", err)
	}
	m := Meta{
		ID:         p.ID,
		Title:      firstNonEmpty(p.Track, p.Title),
		Artist:     firstNonEmpty(p.Artist, p.Creator, p.Uploader, p.Channel),
		Uploader:   firstNonEmpty(p.Uploader, p.Channel),
		Duration:   p.Duration,
		Thumbnail:  p.Thumbnail,
		WebpageURL: p.WebpageURL,
		Extractor:  normalizeExtractor(p.ExtractorK),
		Codec:      p.Acodec,
		Bitrate:    p.Abr,
		SampleRate: int(p.Asr),
		Channels:   int(p.Channels),
		IsLive:     p.IsLive,
	}
	if p.ReleaseYear > 0 {
		m.Year = fmt.Sprintf("%d", int(p.ReleaseYear))
	} else if len(p.UploadDate) >= 4 {
		m.Year = p.UploadDate[:4]
	}
	for _, c := range p.Chapters {
		m.Chapters = append(m.Chapters, Chapter{Title: c.Title, Start: c.Start, End: c.End})
	}
	return m, nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func probe(ctx context.Context, tools Tools, url string) (Meta, error) {
	bin := tools.YtDlp
	if bin == "" {
		return Meta{}, errors.New("probe: yt-dlp not found")
	}
	cmd := exec.CommandContext(ctx, bin, "-J", "--no-playlist", "--no-warnings", "--skip-download", "--", url)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return Meta{}, ctx.Err()
		}
		msg := lastLine(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return Meta{}, fmt.Errorf("probe: yt-dlp failed: %s", msg)
	}
	return parseProbe(stdout.Bytes())
}
