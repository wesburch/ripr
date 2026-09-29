// ripr rips the audio out of any video link.
//
//	ripr                              open the TUI
//	ripr <url>                        open the TUI straight into prep for that link
//	ripr <url> --headless [-f mp3] [-q 320] [-o dir]   plain progress, no animation
//	ripr crypt                        print history
//	ripr doctor                       check yt-dlp and ffmpeg
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"

	"ripr/internal/config"
	"ripr/internal/crypt"
	"ripr/internal/engine"
	"ripr/internal/tui"
)

var version = "0.1.0"

func main() {
	tui.SetVersion(version)
	fs := flag.NewFlagSet("ripr", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	format := fs.String("f", "", "format: mp3 m4a opus flac wav")
	quality := fs.Int("q", 0, "bitrate in kbps for lossy formats")
	outDir := fs.String("o", "", "output folder")
	headless := fs.Bool("headless", false, "no TUI: plain progress lines")
	noCover := fs.Bool("no-cover", false, "skip cover art")
	noTags := fs.Bool("no-tags", false, "skip tags")
	normalize := fs.Bool("normalize", false, "EBU R128 loudness normalization")
	showVersion := fs.Bool("version", false, "print version")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: ripr [url] [-f mp3|m4a|opus|flac|wav] [-q kbps] [-o dir] [--headless]")
		fmt.Fprintln(os.Stderr, "       ripr crypt | ripr doctor")
		fs.PrintDefaults()
	}

	// allow "ripr <url> -f flac" as well as "ripr -f flac <url>"
	args := os.Args[1:]
	var positional []string
	var flagsOnly []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			flagsOnly = append(flagsOnly, a)
			if !strings.Contains(a, "=") && i+1 < len(args) && needsValue(a) {
				flagsOnly = append(flagsOnly, args[i+1])
				i++
			}
			continue
		}
		positional = append(positional, a)
	}
	if err := fs.Parse(flagsOnly); err != nil {
		os.Exit(2)
	}
	if *showVersion {
		fmt.Println("ripr", version)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err, "(using defaults)")
	}
	// flags override this run only; the saved config is never changed by them
	run := cfg
	if *format != "" {
		run.Format = *format
	}
	if *quality > 0 {
		run.Quality = *quality
	}
	if *outDir != "" {
		run.Dir = *outDir
	}
	if *noCover {
		run.CoverArt = false
	}
	if *noTags {
		run.Tags = false
	}
	if *normalize {
		run.Normalize = true
	}

	tools := engine.Doctor()
	store, serr := crypt.Open(crypt.DefaultStorePath())
	if serr != nil {
		fmt.Fprintln(os.Stderr, "crypt:", serr)
	}
	if n, _ := crypt.Sweep(crypt.DefaultCacheDir(), run.CacheDays); n > 0 && *headless {
		fmt.Fprintf(os.Stderr, "swept %d old sources from the cache\n", n)
	}

	sub := ""
	if len(positional) > 0 {
		sub = positional[0]
	}
	switch sub {
	case "doctor":
		printDoctor(tools)
		return
	case "crypt":
		printCrypt(store)
		return
	case "version":
		fmt.Println("ripr", version)
		return
	}

	url := sub
	interactive := isatty.IsTerminal(os.Stdout.Fd()) && !*headless
	if !interactive {
		if url == "" {
			fs.Usage()
			os.Exit(2)
		}
		os.Exit(runHeadless(run, tools, store, url))
	}

	cfg.CoverArt, cfg.Tags, cfg.Normalize = run.CoverArt, run.Tags, run.Normalize
	p := tea.NewProgram(tui.New(tui.Options{Config: cfg, Tools: tools, Store: store, URL: url, Format: *format, Quality: *quality, Dir: *outDir}), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func needsValue(flag string) bool {
	switch strings.TrimLeft(flag, "-") {
	case "f", "q", "o":
		return true
	}
	return false
}

func printDoctor(t engine.Tools) {
	mark := func(ok bool) string {
		if ok {
			return "✓"
		}
		return "✗"
	}
	fmt.Printf("%s yt-dlp %s\n", mark(t.YtDlp != ""), or(t.YtDlpVersion, "not found"))
	fmt.Printf("%s ffmpeg %s\n", mark(t.Ffmpeg != ""), or(t.FfmpegVersion, "not found"))
	if !t.OK() {
		fmt.Printf("\nfix: brew install %s\n", strings.Join(t.Missing, " "))
		os.Exit(1)
	}
}

func printCrypt(s *crypt.Store) {
	if s == nil {
		return
	}
	entries := s.List()
	if len(entries) == 0 {
		fmt.Println("nothing here yet. rip something.")
		return
	}
	for _, e := range entries {
		fmt.Printf("%s  %-5s %8s  %s\n", e.When.Format("2006-01-02 15:04"), e.Format, human(e.Size), e.Path)
	}
	n, b := s.Stats()
	fmt.Printf("\n%d rips · %s\n", n, human(b))
}

func runHeadless(cfg config.Config, tools engine.Tools, store *crypt.Store, url string) int {
	if !tools.OK() {
		fmt.Fprintf(os.Stderr, "missing: %s\nfix: brew install %s\n", strings.Join(tools.Missing, ", "), strings.Join(tools.Missing, " "))
		return 1
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Fprintf(os.Stderr, "looking up %s\n", url)
	meta, err := engine.Probe(ctx, tools, url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "probe:", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "%s – %s (%s)\n", meta.Artist, meta.Title, fmtTime(meta.Duration))

	job := engine.Job{
		URL: url, Meta: meta, Format: engine.Format(cfg.Format), Bitrate: cfg.Quality,
		OutDir: cfg.Dir, Template: cfg.Template, CoverArt: cfg.CoverArt, Tags: cfg.Tags, Normalize: cfg.Normalize,
		CacheDir: crypt.DefaultCacheDir(), CacheDays: cfg.CacheDays, Columns: 96,
	}
	events := make(chan engine.Event, 256)
	done := make(chan error, 1)
	go func() { done <- engine.Rip(ctx, tools, job, events) }()

	last := time.Time{}
	var result *engine.Result
	for {
		select {
		case ev := <-events:
			if ev.Err != nil {
				fmt.Fprintln(os.Stderr, "error:", ev.Err)
				continue
			}
			if ev.Phase == engine.PhaseDone && ev.Result != nil {
				result = ev.Result
				continue
			}
			if ev.Log != "" && !strings.HasPrefix(ev.Log, "[download]") {
				fmt.Fprintln(os.Stderr, "  ", ev.Log)
			}
			if time.Since(last) < 200*time.Millisecond {
				continue
			}
			last = time.Now()
			switch ev.Phase {
			case engine.PhasePull:
				fmt.Fprintf(os.Stderr, "\rpull    %3.0f%%  %s/s      ", ev.Progress*100, human(int64(ev.Speed)))
			case engine.PhaseDecode:
				fmt.Fprintf(os.Stderr, "\rdecode  %3.0f%%  %s  %.1fx    ", ev.Progress*100, fmtTime(ev.Time), ev.SpeedX)
			case engine.PhaseTag:
				fmt.Fprintf(os.Stderr, "\rtag             ")
			}
		case err := <-done:
			// the final event may still be queued behind the completion
			for drained := false; !drained; {
				select {
				case ev := <-events:
					if ev.Phase == engine.PhaseDone && ev.Result != nil {
						result = ev.Result
					}
				default:
					drained = true
				}
			}
			fmt.Fprintln(os.Stderr)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				return 1
			}
			if result != nil {
				fmt.Println(result.Path)
				fmt.Fprintf(os.Stderr, "saved %s · %s · %s\n", human(result.Size), fmtTime(result.Duration), result.Elapsed.Round(100*time.Millisecond))
				if store != nil {
					_ = store.Add(crypt.Entry{
						ID: fmt.Sprintf("%s-%s-%d", meta.ID, result.Format, time.Now().UnixNano()), Title: meta.Title, Artist: meta.Artist, URL: url, Extractor: meta.Extractor,
						Path: result.Path, Format: string(result.Format), Bitrate: result.Bitrate, Size: result.Size, Duration: result.Duration,
						When: time.Now(), Spark: result.Spark, Source: result.SourcePath,
					})
				}
			}
			return 0
		}
	}
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func human(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(b)/float64(1<<10))
	}
	return fmt.Sprintf("%d B", b)
}

func fmtTime(s float64) string {
	t := int(s)
	if t >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", t/3600, (t%3600)/60, t%60)
	}
	return fmt.Sprintf("%d:%02d", t/60, t%60)
}

var _ = filepath.Join
