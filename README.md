# ripr

A terminal app that pulls the audio out of any video link. Paste a link, pick a
format, watch the track's own waveform appear as it is decoded, and the file lands
in `~/Music/ripr`.

```
ripr                               open the TUI
ripr <url>                         open the TUI straight into prep for that link
ripr <url> --headless -f flac      plain progress, no animation, for scripts
ripr crypt                         print history
ripr doctor                        check yt-dlp and ffmpeg
```

## Install

```
brew install wesburch/tap/ripr
```

That pulls in yt-dlp and ffmpeg too. Or build from source:

```
go build -o ripr ./cmd/ripr
```

## Requirements

- macOS or Linux, a UTF-8 terminal with a monospace font that has braille and
  block glyphs (JetBrains Mono, SF Mono, Menlo, Iosevka all do), at least 80×24.
- `yt-dlp` and `ffmpeg` on PATH. `ripr doctor` tells you what's missing and the
  doctor screen inside the app offers to run `brew install` for you.

## Releasing

Releases are cut by goreleaser on a version tag and the Homebrew formula is
written to `wesburch/homebrew-tap` automatically. The workflow needs a
`HOMEBREW_TAP_TOKEN` repository secret: a fine-grained personal access token
with contents read/write on the tap repo.

```
git tag v0.2.0 && git push origin v0.2.0
```

Go 1.27. No cgo. Dependencies are Bubble Tea, Lip Gloss, Bubbles, go-toml and a
clipboard shim.

## How it works

Two phases, both real:

1. **Pull.** yt-dlp downloads the best audio stream into a source cache
   (`~/Library/Caches/ripr` on macOS). Sources are kept 7 days so re-ripping the
   same link into another format never downloads again.
2. **Decode.** One ffmpeg run reads the cached source, writes the output file
   with tags and cover art, and tees a 4 kHz stereo PCM stream back to ripr.
   That stream is folded into the waveform and the level meters on screen. Every
   dot is a measurement.

History lives in `~/.local/share/ripr/crypt.json`. Settings live in
`~/.config/ripr/config.toml`; the settings screen edits it.

## Layout of the code

```
cmd/ripr/            flags, headless mode, launches the TUI
internal/engine/     yt-dlp and ffmpeg wrappers, progress parsing, the PCM tap
internal/crypt/      history store and the source cache
internal/config/     config file
internal/tui/        the Bubble Tea model, one column, one grid
internal/tui/draw/   braille canvas, half-block sprites, tear wipe, wordmark
```

`DESIGN.md` is the design spec the app is built to. `prototype/` holds an earlier
browser-based design study and is not part of the build.

## Verification

```
go test ./...                                          unit tests
RIPR_SMOKE=1 go test ./internal/engine -run Smoke -v   real rip of a 19 s public video
```
