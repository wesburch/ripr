# RIPR / Signal Cut

Second design concept. This document applies only to this directory; the shared root design remains untouched.

## Direction

A dense terminal instrument. A brief particle field forms the RIPR wordmark with no subtitle, then gives way to a stable working screen. The wordmark condenses from a column-aligned audio signal, rather than a scattered particle cloud. A large character-cell waveform reveals itself during extraction. No mascot in this concept.

## User constraints

Preserve the CLI, pixel-like look and expressive opening motion from the supplied recording. Remove persistent dependency badges and boot subtitle. Improve typography and spacing. Output directory stays discoverable; dependencies live in Doctor. Any future mascot must be small and secondary.

## Visual system

Graphite background #101318, off-white #e0e6ed, secondary text #9aa6b6, structural rules #35404d. Amber #ffc078 means active; mint #8edbc5 means complete. Menlo / SFMono-Regular / Consolas typography, 13px at 1.5 line height. Pixel artwork is built from discrete square cells, not scaled bitmap lettering. Controls are inline text, with a rectangular selected text background and visible keyboard focus. No gradients, glows or card shadows.

## Structure

Single header, full-width URL entry, full-width signal stage, inline encoding command strip, and a horizontal recent-history shelf above the compact key bar. At narrow sizes, the history shelf reduces to two entries. Output folder expands inline from the header. Doctor replaces the signal view and returns to it without changing settings.

## Motion

Boot: 2.6 seconds, skippable, waveform columns condense into a pixel wordmark. Rip: one 450ms diagonal cut, then the waveform fills left to right. Completion: waveform turns mint and holds still. No continuous idle animation. OS reduced-motion and the design motion toggle replace sequences with still states.

## Prototype boundaries

All sources, waveforms, progress, recents and diagnostics are explicitly demonstrations. No network requests, extraction, directory access, real dependency checks or audio playback occur. Actual terminal rendering and engine integration remain separate work.
