package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const smokeURL = "https://www.youtube.com/watch?v=jNQXAC9IVRw"

func runRip(t *testing.T, tools Tools, job Job) ([]Event, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	ch := make(chan Event, 64)
	var evs []Event
	done := make(chan struct{})
	go func() {
		for e := range ch {
			evs = append(evs, e)
		}
		close(done)
	}()
	err := Rip(ctx, tools, job, ch)
	close(ch)
	<-done
	return evs, err
}

func TestSmoke(t *testing.T) {
	if os.Getenv("RIPR_SMOKE") != "1" {
		t.Skip("set RIPR_SMOKE=1 to run")
	}
	tools := Doctor()
	if !tools.OK() {
		t.Fatalf("missing tools: %v", tools.Missing)
	}
	meta, err := Probe(context.Background(), tools, smokeURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("meta: %+v", meta)
	out, cache := t.TempDir(), t.TempDir()
	job := Job{URL: smokeURL, Meta: meta, Format: MP3, Bitrate: 192, OutDir: out, CacheDir: cache,
		CoverArt: true, Tags: true, Columns: 192, Template: "{artist} – {title}.{ext}"}
	evs, err := runRip(t, tools, job)
	if err != nil {
		t.Fatal(err)
	}
	last := evs[len(evs)-1]
	if last.Phase != PhaseDone || last.Result == nil {
		t.Fatalf("last event %+v", last)
	}
	r := last.Result
	if r.Size <= 50_000 || r.Duration < 17 || r.Duration > 21 {
		t.Fatalf("size %d duration %v", r.Size, r.Duration)
	}
	loud := 0
	for _, a := range r.Amps {
		if a > 0.05 {
			loud++
		}
	}
	if float64(loud)/float64(len(r.Amps)) < 0.4 {
		t.Fatalf("only %d/%d columns loud", loud, len(r.Amps))
	}
	nz := false
	for _, s := range r.Spark {
		nz = nz || s != 0
	}
	if !nz {
		t.Fatal("spark all zero")
	}
	if !strings.HasSuffix(r.Path, ".mp3") || filepath.Dir(r.Path) != out {
		t.Fatalf("path %q", r.Path)
	}
	// phase order
	want := []Phase{PhasePull, PhaseDecode, PhaseTag, PhaseDone}
	wi, colSeen := 0, false
	for _, e := range evs {
		if e.Phase == PhaseDecode && e.Column >= 0 {
			colSeen = true
		}
		if wi < len(want) && e.Phase == want[wi] && (want[wi] != PhaseDecode || colSeen) {
			wi++
		}
	}
	if wi != len(want) {
		t.Fatalf("phase order incomplete, matched %d of %d", wi, len(want))
	}
	t.Logf("size=%d duration=%.1f elapsed=%s loud=%d/%d spark=%v", r.Size, r.Duration, r.Elapsed, loud, len(r.Amps), r.Spark)

	// Second rip: flac, must come from cache.
	before, err := os.Stat(r.SourcePath)
	if err != nil {
		t.Fatal(err)
	}
	job.Format = FLAC
	evs, err = runRip(t, tools, job)
	if err != nil {
		t.Fatal(err)
	}
	cached := false
	for _, e := range evs {
		if strings.Contains(e.Log, "cache") {
			cached = true
		}
	}
	if !cached {
		t.Fatal("no cache log event")
	}
	after, err := os.Stat(r.SourcePath)
	if err != nil || !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("source modified: %v", err)
	}
	t.Logf("cache hit observed; flac size=%d", evs[len(evs)-1].Result.Size)
}
