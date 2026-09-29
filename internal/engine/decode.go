package engine

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"sync"
)

func coverFormat(f Format) bool { return f == MP3 || f == M4A || f == FLAC }

func trimLen(job Job) float64 {
	if job.Trim != nil {
		if d := job.Trim[1] - job.Trim[0]; d > 0 {
			return d
		}
	}
	return job.Meta.Duration
}

func ffmpegArgs(job Job, src, cover, out string) []string {
	a := []string{"-hide_banner", "-nostdin", "-y", "-loglevel", "error", "-nostats", "-progress", "pipe:2"}
	if job.Trim != nil {
		a = append(a, "-ss", strconv.FormatFloat(job.Trim[0], 'f', -1, 64), "-to", strconv.FormatFloat(job.Trim[1], 'f', -1, 64))
	}
	a = append(a, "-i", src)
	withCover := cover != "" && job.CoverArt && coverFormat(job.Format)
	if withCover {
		a = append(a, "-i", cover)
	}
	a = append(a, "-map", "0:a")
	if withCover {
		a = append(a, "-map", "1:v", "-c:v", "mjpeg", "-disposition:v", "attached_pic")
	}
	br := strconv.Itoa(job.Bitrate) + "k"
	switch job.Format {
	case MP3:
		a = append(a, "-c:a", "libmp3lame", "-b:a", br, "-id3v2_version", "3", "-f", "mp3")
	case M4A:
		a = append(a, "-c:a", "aac", "-b:a", br, "-movflags", "+faststart", "-f", "ipod")
	case Opus:
		a = append(a, "-c:a", "libopus", "-b:a", br, "-f", "opus")
	case FLAC:
		a = append(a, "-c:a", "flac", "-f", "flac")
	case WAV:
		a = append(a, "-c:a", "pcm_s16le", "-f", "wav")
	}
	if job.Normalize {
		a = append(a, "-af", "loudnorm")
	}
	if job.Tags {
		a = append(a,
			"-metadata", "title="+job.Meta.Title,
			"-metadata", "artist="+job.Meta.Artist,
			"-metadata", "date="+job.Meta.Year,
			"-metadata", "comment="+job.Meta.WebpageURL)
	} else {
		// tags off means no tags at all, including whatever the source carried
		a = append(a, "-map_metadata", "-1")
	}
	a = append(a, out,
		"-map", "0:a", "-ac", "2", "-ar", "4000", "-f", "s16le", "pipe:1")
	return a
}

func decode(ctx context.Context, tools Tools, job Job, src, cover, out string, events chan<- Event) ([]float32, error) {
	if tools.Ffmpeg == "" {
		return nil, errors.New("decode: ffmpeg not found")
	}
	switch job.Format {
	case MP3, M4A, Opus, FLAC, WAV:
	default:
		return nil, fmt.Errorf("decode: unknown format %q", job.Format)
	}
	dur := trimLen(job)
	cmd := exec.CommandContext(ctx, tools.Ffmpeg, ffmpegArgs(job, src, cover, out)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var (
		wg      sync.WaitGroup
		amps    []float32
		foldErr error
		lastErr string
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		amps, foldErr = fold(stdout, dur, job.Columns, func(ev Event) bool { return send(ctx, events, ev) })
		if foldErr != nil {
			// Keep the pipe drained so ffmpeg can exit (or be killed by ctx).
			scanLines(stdout, func(string) {})
		}
	}()
	go func() {
		defer wg.Done()
		var st progressState
		scanLines(stderr, func(line string) {
			k, v, ok := parseKV(line)
			if !ok {
				if line != "" {
					lastErr = line
					ev := newEvent(PhaseDecode)
					ev.Log = line
					send(ctx, events, ev)
				}
				return
			}
			if st.feed(k, v) {
				ev := newEvent(PhaseDecode)
				ev.Time, ev.SpeedX = st.Time, st.SpeedX
				if dur > 0 {
					ev.Progress = min(st.Time/dur, 1)
				}
				send(ctx, events, ev)
			}
		})
	}()
	wg.Wait()
	werr := cmd.Wait()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if werr != nil {
		if lastErr == "" {
			lastErr = werr.Error()
		}
		return nil, fmt.Errorf("decode: ffmpeg failed: %s", lastErr)
	}
	if foldErr != nil {
		return nil, fmt.Errorf("decode: reading tap: %w", foldErr)
	}
	return amps, nil
}
