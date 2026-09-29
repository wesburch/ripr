# ripr — design concept

A terminal app that pulls the audio out of any video link.

## The idea

Most rippers hide the interesting part: paste a link, a percentage climbs, a file appears.
ripr treats the extraction as the show. The progress indicator is not a bar. It is the
song's own waveform, revealed left to right as the audio is decoded, so you watch the
track come out of the video in real time.

Three rules hold the design together:

1. **The waveform is real.** ffmpeg writes the file and tees a low-rate PCM stream back
   to the TUI. Every dot on screen is a measurement, never a decoration.
2. **One screen, no navigation.** Link bar, stage and crypt are always in the same place.
   The stage changes what it shows. You never lose your bearings.
3. **Room for the app.** Chrome is one row on top (name, version, output folder), one on
   the bottom (keys). Everything else is the stage and history. Dependency status only
   appears in the top bar, in gold, when something is missing. The mascot is a small
   optional corner mark, not a panel, and it never speaks.
4. **Clean, never sterile.** The structure is as plain as Codex or Claude Code: rules,
   labels, one grid. The character lives in the boot sequence, the live waveform, the
   tear wipe, the ember-and-wisp palette and the reaper in the corner. Simplify the
   chrome whenever it gets in the way. Never simplify the character.

## Hierarchy

A filled block means "press this," and there is only ever one on screen: the primary
action. Everything else uses a different signal so nothing else looks like a button.

| role                     | treatment                                      |
|--------------------------|------------------------------------------------|
| logo                     | bold ember text, no block                      |
| section label            | dim text on its own row, full-width rule under |
| selected option in a row | bold + underline; ember when the row has focus |
| focused field            | ember + underline                              |
| list cursor              | ▸ marker in ember, bold text                   |
| primary action           | the one filled ember block                     |
| key hints                | footer only, never inline at a section's edge  |

## Layout (100×36 reference; compact mode fits 80×24)

Decided 2026-09-29: **single column**, like the Codex reference, with ripr's character.
No crypt rail and no recent strip on the main screen. History lives only on the crypt
page (`c`). The waveform gets the full width at double the resolution.

```
  ripr v0.1.0                                                           ~/Music/ripr
 ┌──────────────────────────────────────────────────────────────────────────────────┐
 │ ▸ paste a link                                                                   │
 └──────────────────────────────────────────────────────────────────────────────────┘

  signal                                                                      found
 ──────────────────────────────────────────────────────────────────────────────────
  ▓▓▓▓▓▓  Something About Us
  ▓▓▓▓▓▓  Daft Punk · 3:52 · youtube · 2003
  ▓▓▓▓▓▓  youtube.com/watch?v=… · opus 160k · 48 kHz · stereo · 6 chapters

  ⣿⣿⣿ ... full-width braille waveform, 96 cols × 8 rows ... ⣿⣿⣿
  0:00              0:58              1:56              2:54              3:52

  ripping · decoding and encoding                                       1:52 / 3:52
 ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━──────────────────   ← the rule IS progress
  ✓ pull   ◐ decode   ◐ encode mp3 320k   ○ tag + cover

  format  mp3  m4a  opus  flac  wav                                quality  320 kbps

          [x] cover art  [x] tags  [ ] trim  [ ] chapters  [ ] normalize

   ↵  rip it   mp3 320 kbps → ~/Music/ripr/{artist} – {title}.mp3            ~9 MB


 ──────────────────────────────────────────────────────────────────────────────────

  tab next  ←→ change  o folder  n name  ↵ rip  esc back          ready to rip  [cowl]
```

Rows 26, 28 and 30 change per state, with a blank row between each: prep shows format,
options, rip button; rip shows format (locked), stereo meters with eta, ffmpeg log tail;
done shows format, actions, next-link button. Rows 5–24 are identical in every state.

The column has a natural height of 38 rows. On a taller window it is centered
vertically as one block, footer included, rather than pinning the footer to the bottom
of a mostly empty screen. Below 36 rows the compact map is used; below 80×24 ripr asks
for a bigger window.

Mockup-only rendering notes, so nobody chases them in the app: a browser leaves a sliver
of cell background around half-block glyphs, so a sprite draws its brighter pixel as the
glyph and the darker as background; glyphs missing from the page font (✓ ◐ ▸ ▰ ↵ …) are
pinned to one cell width. A real terminal has neither problem.

### Previous layout, superseded

One grid. Two-column margins on both sides, shared by the header, the input and every
section. No boxes and no corner glyphs anywhere: a box-drawing corner draws its line
down the middle of a cell while text starts at the cell edge, so any corner beside text
looks notched and half a column off. A section is a dim label on its own row with a
full-width rule under it, so no rule ever starts inward of another. The input is a line
with a rule under it. Stage and rail are separated by one vertical rule. Every section
gets a blank row above and below.

```
  ripr v0.1.0                                                           ~/Music/ripr

  ▸ paste a link
  ──────────────────────────────────────────────────────────────────────────────────

  found                                                idle  │ crypt      47 · 412 MB
  ───────────────────────────────────────────────────────── │ ────────────────────────
   ... stage: idle / prep / rip / done / settings / doctor   │ title
                                                             │ ⣀⣤⣶ fmt · size
  vessel                                                     │ title
  ───────────────────────────────────────────────────────── │ ⣀⣤⣶ fmt · size
   mp3   m4a   opus   flac   wav        (selected: bold+u)  │ ... 10 rows ...
   ↵  rip it   mp3 320 kbps → ~/Music/ripr/                  │
  ──────────────────────────────────────────────────────────────────────────────────

   keys for the current stage                                                 [cowl]
```

Primary actions are a filled ember block with the plan beside it in dim text, so the
button never sits alone.

Pressing `c` expands the crypt across the full width (title, waveform, format, size,
date, source, filter) in place of stage and rail. Esc collapses it back. There is never a
second copy of the list on screen. Below 80×24 the rail hides, history is reached with
`c`, and the mascot is dropped.

## Moments

- **Boot.** A single line draws across the screen like an oscilloscope warming up. Then
  every dot flies to its place in the wordmark, left to right, cooling from ember to bone
  as it lands. Just over 3 s in all, every launch (a shortened repeat-launch version was
  tried and dropped: it went by too fast to see). Any key skips it, and a pasted or typed
  link during the boot is kept. Wordmark style is a setting (dots, block, plain).
- **Tear wipe.** Every stage change is a rip: a jagged edge sweeps across in 300 ms with
  the new content underneath. One motif everywhere so the app feels like one object.
- **Clipboard first.** Open ripr with a supported link on the clipboard and the stage
  already shows what it is, with one key to rip it. Zero typing is the common case.
- **The reveal.** Download is a thin pull bar. Then the overview waveform fills from the
  left as decoding runs, with a playhead, a live stereo meter and the real ffmpeg log
  tail. On finish the whole waveform turns wisp green.
- **The crypt, always visible.** The whole right rail is history, eleven entries deep,
  each with a braille sparkline of its waveform. Play, reveal in Finder, copy path, or
  re-rip into another format from the cached source without downloading again.
  Playlists group under one header.
- **The reaper, peeking.** Only the top of a rounded cowl, 10×3 pixels (half-block
  characters, two pixels per cell), rising from the bottom edge in the right corner,
  below the footer rule and never touching it.
  Two ember eyes in the dark under the hood are all you see of him. No dialogue. Blinks
  every 3 to 6 seconds when idle. While ripping the eyes glow on loud sections and he
  bobs one pixel on the beat, driven by the same level data as the meters.
  `mascot = off` and the corner is empty.

## Stages

| stage    | shows                                                                                  | keys                                            |
|----------|----------------------------------------------------------------------------------------|-------------------------------------------------|
| idle     | Clipboard card if a link is there, otherwise one hint line.                            | paste, ↵ rip clipboard, c crypt, , settings     |
| prep     | Metadata card with the real thumbnail. Format row, quality, toggles (cover art, tags, normalize), save folder, filename template. | tab, ←→, space, o folder, n name, / link, ↵ rip, esc |

What each vessel can carry, and what the prep screen says about it:

| format | cover art | tags | note |
|--------|-----------|------|------|
| mp3    | yes (ID3v2 picture) | yes | plays anywhere |
| m4a    | yes | yes | Apple-native |
| flac   | yes | yes | lossless |
| opus   | no  | yes (Vorbis comments on the stream) | the Ogg muxer has no attached-picture support |
| wav    | no  | yes (RIFF INFO chunk) | Finder and Music ignore INFO tags; ffprobe sees them |

When cover art is impossible the toggle shows `[–] cover art · not in wav` and cannot be
switched on; the done receipt only lists what was actually written.

Choosing a folder: `o` on prep (or ↵ on the save-to field, or ↵ on "save to" in
settings) opens the platform's folder dialog. On macOS that is Finder's own "Choose a
folder" sheet via osascript; on Linux it is zenity when installed. Where no dialog
exists the field becomes a text editor. A folder picked on prep applies to that rip;
one picked in settings becomes the default.

The link box is live on prep and done as well as idle: pasting a link anywhere starts a
new lookup, `/` puts the cursor in the box with the current link for editing, ↵ looks it
up, esc backs out. During a rip the box is read-only.
| rip      | Phase chips, pull bar, live overview waveform, ruler + playhead, stereo meter, speed/ETA, ffmpeg log tail. | esc cancel                        |
| done     | Receipt: file, folder, size, bitrate, duration, embedded extras. Full waveform in wisp. | p play, r reveal, y copy, a another format, ↵ new |
| crypt    | The rail expanded to full width: title, waveform, format, size, date, source, filter. Replaces stage and rail while open. | / filter, ↵ actions, x forget, esc collapse |
| settings | Defaults: format, quality, folder, template, source cache, cover art, loudness, theme, wordmark, hint text, mascot. | ↑↓ ←→, esc                 |
| doctor   | Replaces idle when yt-dlp or ffmpeg is missing or stale; the top bar shows the missing tool in gold. Offers to run the brew command. | ↵ fix it                       |

## Palette

| token | hex     | use                           |
|-------|---------|-------------------------------|
| crypt | #0b0a0f | background                    |
| bone  | #e9e3d5 | text                          |
| ash   | #6b667a | dim text                      |
| ember | #ff5d3a | live, selected, in progress   |
| wisp  | #6fe3c1 | finished, alive               |
| gold  | #f2c14e | warnings                      |
| hood  | #4d4562 | mascot, chrome                |

Two accents only, with fixed meaning: ember is anything happening now, wisp is anything
finished. Truecolor first, 256-color table, 16-color fallback. Braille and block glyphs
need UTF-8 and a modern monospace font; fall back to ASCII bars otherwise.

## Fonts and theming

**It is all one font, at one size.** A terminal draws every character in the one font
set in the terminal app, at the size set there. No terminal program can choose a font or
a font size per element. What reads as a "different font" is the same face drawn dim or
bold. Font size is changed in the terminal (cmd + / cmd −). The in-app equivalent is the
`density` setting: compact drops blank rows and shortens labels.

What ripr does control, themeable in `~/.config/ripr/themes/*.toml`:

- **Colors.** The seven palette tokens. Ships with `crypt` (dark, default), `bone`
  (light) and `mono` (no color).
- **Style per role.** title, value, label, hint, log, key: each regular, bold, dim,
  italic or underlined. `hint = regular` if dim text bothers you.
- **Wordmark style.** `dots` (braille), `block` (solid half-block letters) or `plain`
  (no logo). The logo is drawn by ripr, not by a font, so this is a real choice.
- **Density.** `comfortable` or `compact`.

For the font itself: any monospace with braille and block glyphs. JetBrains Mono,
Berkeley Mono, Iosevka, Commit Mono, SF Mono. Set it in the terminal app and ripr
inherits it.

## Stack

**Recommended: Go + Bubble Tea.** Elm-style model/update/view. Lip Gloss for styling,
Bubbles for text input, list and file picker, Harmonica for spring physics on the boot.
One binary via Homebrew.

**Alternative: Rust + Ratatui.** Braille canvas widget built in, very fast, slower to
iterate. Pick it for Rust practice.

**Engine: yt-dlp + ffmpeg.** Both already installed on this Mac. ripr never
reimplements either.

### Pipeline

```
yt-dlp --dump-json URL                      → metadata for prep (title, duration, thumbnail, chapters)
yt-dlp -f bestaudio -o - URL                → stdout (progress parsed with --newline)
  | ffmpeg -i pipe:0
      -map 0:a -c:a libmp3lame -b:a 320k -id3v2_version 3 out.mp3    (the file)
      -map 0:a -ac 1 -ar 4000 -f s16le pipe:3                         (the waveform tap)
ffmpeg -i out.mp3 -i cover.jpg -map 0 -map 1 -c copy -disposition:v attached_pic   (cover art)
```

The tap is 8 KB/s. Each chunk folds into the overview column it belongs to and into the
meter. The overview needs duration up front, which the metadata call provides. Live
streams with no duration scroll instead of filling.

### Code layout

```
cmd/ripr/main.go          flags, headless mode, launches the TUI
internal/engine/          yt-dlp and ffmpeg wrappers, progress parsing, the PCM tap
internal/tui/             the Bubble Tea model: one root, one message loop
internal/tui/stage/       idle, prep, rip, done, crypt, settings, doctor
internal/tui/rail/        crypt list
internal/tui/draw/        braille canvas, block art, sprite rasterizer, tear wipe, wordmark
internal/tui/mascot/      the 14×4 peeking hood and its states (blink, glow, bob)
internal/crypt/           history store (SQLite via modernc, no cgo) and source cache
internal/config/          ~/.config/ripr/config.toml and themes/
```

### CLI surface

```
ripr                               open the TUI
ripr <url>                         open the TUI straight into prep for that link
ripr <url> -f flac -o ~/Desktop    headless: plain progress, no animation, for scripts
ripr crypt                         print history
ripr doctor                        check yt-dlp and ffmpeg, offer to install or update
```

## Motion rules

- 30 fps cap. Redraw on ticks or messages only.
- Boot just over 3 s, every launch, any key skips.
- Tear wipe 300 ms. Nothing else moves during it.
- Idle animation is the corner mascot only. The stage does not fidget.
- `motion = reduced`, `NO_COLOR`, or non-TTY drops to static frames and plain text.

## Build order

1. **Engine, headless.** Paste a link, get an mp3, PCM tap producing numbers. No TUI.
2. **Shell.** Persistent layout, link bar, prep, rip with a plain bar, done. Ugly, complete.
3. **The show.** Braille canvas, live waveform, meters, tear wipe, boot.
4. **Memory.** Crypt store, source cache, re-rip, playlists and chapters, settings, themes.
5. **Ship.** Doctor, small and dumb terminal fallbacks, Homebrew tap. Mascot last, if kept.

## Build status (2026-09-29)

Built and verified: boot, tear wipe, single-column layout, live waveform from the PCM
tap, meters, cover art and tags, the 7-day cache and cache re-rips, crypt page, settings
page, doctor, headless mode, compact layout down to 80×24, reduced motion.

Not built yet, and therefore not shown in the UI: trim, chapter splitting, playlists.
The options row offers only cover art, tags and normalize until those exist.

## Decisions (all 2026-09-29)

- **Go + Bubble Tea.** Chosen over Rust + Ratatui for iteration speed, the Charm
  component set, and subprocess ergonomics. Rust's only real edge (Ratatui's built-in
  braille canvas, raw speed) doesn't matter at 30 fps on a 100-column grid.
- **Source cache for re-rips: 7 days.**
- **The peeking reaper stays**, 10×3, no dialogue, switchable off.
- **The light "bone" theme ships in v1.** Same accents, light ground; part of build
  step 4 alongside the theme file format.
- **Layout is single column**, no rail, no recent strip. History only on the crypt page.
