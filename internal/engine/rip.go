package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"
)

func rip(ctx context.Context, tools Tools, job Job, events chan<- Event) error {
	start := time.Now()
	if job.Columns <= 0 {
		job.Columns = 192
	}
	if job.CacheDir == "" {
		return errors.New("rip: job has no cache dir")
	}
	if job.OutDir == "" {
		return errors.New("rip: job has no output dir")
	}
	outDir, err := filepath.Abs(job.OutDir)
	if err != nil {
		return err
	}
	cacheDir, err := filepath.Abs(job.CacheDir)
	if err != nil {
		return err
	}
	job.CacheDir = cacheDir
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	fail := func(err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}

	src, cover, err := pull(ctx, tools, job, events)
	if err != nil {
		return fail(err)
	}
	src, _ = filepath.Abs(src)

	out := uniquePath(filepath.Join(outDir, expand(job.Template, job.Meta, job.Format)))
	amps, err := decode(ctx, tools, job, src, cover, out, events)
	if err != nil {
		os.Remove(out)
		return fail(err)
	}

	fin := newEvent(PhaseDecode)
	fin.Progress = 1
	if !send(ctx, events, fin) {
		os.Remove(out)
		return ctx.Err()
	}
	if job.Tags || job.CoverArt {
		tag := newEvent(PhaseTag)
		tag.Progress = 1
		if !send(ctx, events, tag) {
			os.Remove(out)
			return ctx.Err()
		}
	}

	st, err := os.Stat(out)
	if err != nil {
		return fail(err)
	}
	done := newEvent(PhaseDone)
	done.Progress = 1
	done.Result = &Result{
		Path:       out,
		Size:       st.Size(),
		Duration:   trimLen(job),
		Elapsed:    time.Since(start),
		SourcePath: src,
		Amps:       amps,
		Spark:      spark(amps),
		Format:     job.Format,
		Bitrate:    job.Bitrate,
	}
	if !send(ctx, events, done) {
		return ctx.Err()
	}
	return nil
}
