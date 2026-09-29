// Package engine drives yt-dlp and ffmpeg. It never reimplements either.
//
// A rip has two phases that map to what the screen shows:
//
//	Pull   yt-dlp downloads the best audio stream into the source cache,
//	       reporting bytes and speed. The cache is what makes re-rips free.
//	Decode ffmpeg reads the cached source and writes the output file while
//	       teeing a low-rate stereo PCM stream back to us. That stream is
//	       folded into waveform columns and stereo levels. Every dot the UI
//	       draws is a measurement from this tap.
//	Tag    metadata and cover art are written as part of the same ffmpeg run
//	       when asked; the phase exists so the UI can show it.
//
// This file is the contract. Implementations live in the sibling files.
package engine

import (
	"context"
	"time"
)

// Tools is what Doctor found on this machine.
type Tools struct {
	YtDlp         string // absolute path, "" if missing
	Ffmpeg        string // absolute path, "" if missing
	YtDlpVersion  string // e.g. "2026.08.19"
	FfmpegVersion string // e.g. "9.0.2"
	Missing       []string
}

// OK reports whether both tools were found.
func (t Tools) OK() bool { return len(t.Missing) == 0 }

// Chapter is one chapter from the source, in seconds.
type Chapter struct {
	Title string
	Start float64
	End   float64
}

// Meta is what Probe learns about a link before anything is downloaded.
type Meta struct {
	ID         string
	Title      string
	Artist     string // yt-dlp "artist", falling back to "uploader"/"channel"
	Uploader   string
	Year       string // 4 digits, "" if unknown
	Duration   float64
	Thumbnail  string // URL of the best thumbnail
	WebpageURL string
	Extractor  string // lowercase short name: "youtube", "soundcloud", ...
	Codec      string // source audio codec, e.g. "opus"
	Bitrate    float64
	SampleRate int
	Channels   int
	Chapters   []Chapter
	IsLive     bool
}

// Format is an output container/codec the user can pick.
type Format string

const (
	MP3  Format = "mp3"
	M4A  Format = "m4a"
	Opus Format = "opus"
	FLAC Format = "flac"
	WAV  Format = "wav"
)

// Formats lists the vessels in display order.
var Formats = []Format{MP3, M4A, Opus, FLAC, WAV}

// Lossy reports whether the format takes a bitrate.
func (f Format) Lossy() bool { return f == MP3 || f == M4A || f == Opus }

// Job is one rip request. Everything the engine needs, nothing it doesn't.
type Job struct {
	URL       string
	Meta      Meta // from Probe; the engine does not re-probe
	Format    Format
	Bitrate   int    // kbps, ignored for lossless formats
	OutDir    string // must exist or be creatable
	Template  string // e.g. "{artist} – {title}.{ext}"
	CoverArt  bool   // embed thumbnail (mp3, m4a, flac only)
	Tags      bool   // write title/artist/date/comment(url)
	Normalize bool   // EBU R128 loudness normalization (loudnorm)
	Trim      *[2]float64
	CacheDir  string // where pulled sources live
	CacheDays int    // sources older than this are swept by the caller
	Columns   int    // waveform columns to fold the tap into; 192 for the TUI
}

// Phase is where a rip is.
type Phase int

const (
	PhasePull Phase = iota
	PhaseDecode
	PhaseTag
	PhaseDone
)

func (p Phase) String() string {
	switch p {
	case PhasePull:
		return "pull"
	case PhaseDecode:
		return "decode"
	case PhaseTag:
		return "tag"
	case PhaseDone:
		return "done"
	}
	return "?"
}

// Event is one message from a running rip. Fields not relevant to the
// phase are zero. Column is -1 when the event carries no waveform column.
type Event struct {
	Phase    Phase
	Progress float64 // 0..1 within the phase
	// Pull
	Bytes, TotalBytes int64
	Speed             float64 // bytes/s
	// Decode
	Time   float64 // seconds of audio processed so far
	SpeedX float64 // multiple of realtime, from ffmpeg's speed=
	// Waveform tap
	Column int     // index of a column whose amplitude is now final
	Amp    float32 // 0..1 peak amplitude for that column
	LevelL float32 // 0..1 current level, updated about 20 times a second
	LevelR float32
	// Diagnostics
	Log string // a raw tool line worth showing in the log tail, or ""
	Err error
	// Only on PhaseDone
	Result *Result
}

// Result describes the file that was written.
type Result struct {
	Path       string
	Size       int64
	Duration   float64
	Elapsed    time.Duration
	SourcePath string    // the cached source used
	Amps       []float32 // final waveform, len == Job.Columns
	Spark      [16]uint8 // 0..3 mini waveform for history rows
	Format     Format
	Bitrate    int
}

// Doctor locates yt-dlp and ffmpeg on PATH and reads their versions.
func Doctor() Tools { return doctor() }

// Probe fetches metadata for url without downloading media.
func Probe(ctx context.Context, tools Tools, url string) (Meta, error) {
	return probe(ctx, tools, url)
}

// Rip runs a job to completion, sending events as it goes. It blocks until
// the job finishes, fails, or ctx is cancelled. The final event on success
// has Phase == PhaseDone and a non-nil Result. Rip never closes events.
func Rip(ctx context.Context, tools Tools, job Job, events chan<- Event) error {
	return rip(ctx, tools, job, events)
}

// Expand renders a filename template. Supported fields: {artist}, {title},
// {uploader}, {year}, {ext}, {id}, {source}. The result is a bare filename
// with path separators and control characters replaced, never a path.
func Expand(template string, meta Meta, format Format) string {
	return expand(template, meta, format)
}

// Estimate returns a rough output size in bytes for the prep screen.
func Estimate(meta Meta, format Format, bitrate int) int64 {
	return estimate(meta, format, bitrate)
}
