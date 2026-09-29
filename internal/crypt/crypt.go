// Package crypt is where finished rips rest: a small JSON history store and
// the source cache that makes re-rips free.
//
// This file is the contract. Implementations live in the sibling files.
package crypt

import "time"

// Entry is one finished rip.
type Entry struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Artist    string    `json:"artist"`
	URL       string    `json:"url"`
	Extractor string    `json:"extractor"` // "youtube", "soundcloud", ...
	Path      string    `json:"path"`      // output file
	Format    string    `json:"format"`
	Bitrate   int       `json:"bitrate"`
	Size      int64     `json:"size"`
	Duration  float64   `json:"duration"`
	When      time.Time `json:"when"`
	Spark     [16]uint8 `json:"spark"`  // 0..3 mini waveform
	Source    string    `json:"source"` // cached source path, may no longer exist
}

// Store is the history file. Safe for use from one process at a time.
type Store struct {
	path    string
	entries []Entry
}

// DefaultStorePath is ~/.local/share/ripr/crypt.json on every platform
// (macOS included, on purpose: one place, easy to find).
func DefaultStorePath() string { return defaultStorePath() }

// DefaultCacheDir is where pulled sources live:
// ~/Library/Caches/ripr on macOS, $XDG_CACHE_HOME/ripr elsewhere.
func DefaultCacheDir() string { return defaultCacheDir() }

// Open loads the store, creating parent directories and an empty file when
// none exists. A corrupt file is renamed aside, not silently overwritten.
func Open(path string) (*Store, error) { return open(path) }

// Add prepends an entry and saves. Entries with the same Path are replaced.
func (s *Store) Add(e Entry) error { return s.add(e) }

// Remove deletes the entry with the given ID and saves.
func (s *Store) Remove(id string) error { return s.remove(id) }

// List returns entries newest first. The slice is a copy.
func (s *Store) List() []Entry { return s.list() }

// Stats returns the number of rips and the total bytes written.
func (s *Store) Stats() (count int, bytes int64) { return s.stats() }

// Sweep deletes files in cacheDir older than days. It returns how many were
// removed. It never removes directories or files it did not create the
// naming convention for (see engine's cache layout: <id>.<ext> plus <id>.jpg).
func Sweep(cacheDir string, days int) (removed int, err error) { return sweep(cacheDir, days) }
