// Package config reads and writes ~/.config/ripr/config.toml.
package config

import (
	"errors"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// Config is everything the user can set. Field names are the TOML keys.
type Config struct {
	Format    string `toml:"format"`     // mp3 m4a opus flac wav
	Quality   int    `toml:"quality"`    // kbps for lossy formats
	Dir       string `toml:"dir"`        // output folder
	Template  string `toml:"template"`   // filename template
	CacheDays int    `toml:"cache_days"` // keep pulled sources this long
	CoverArt  bool   `toml:"cover_art"`
	Tags      bool   `toml:"tags"`
	Normalize bool   `toml:"normalize"`
	Theme     string `toml:"theme"`    // crypt bone mono
	Wordmark  string `toml:"wordmark"` // dots block plain
	Density   string `toml:"density"`  // comfortable compact
	Mascot    bool   `toml:"mascot"`
	Motion    string `toml:"motion"` // full reduced
}

// Default is what a fresh install uses.
func Default() Config {
	home, _ := os.UserHomeDir()
	return Config{
		Format:    "mp3",
		Quality:   320,
		Dir:       filepath.Join(home, "Music", "ripr"),
		Template:  "{artist} – {title}.{ext}",
		CacheDays: 7,
		CoverArt:  true,
		Tags:      true,
		Normalize: false,
		Theme:     "crypt",
		Wordmark:  "dots",
		Density:   "comfortable",
		Mascot:    true,
		Motion:    "full",
	}
}

// Path is where the config lives: $XDG_CONFIG_HOME/ripr/config.toml or
// ~/.config/ripr/config.toml.
func Path() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "ripr", "config.toml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "ripr", "config.toml")
}

// Load returns the saved config merged over the defaults. A missing file is
// not an error; a malformed one is.
func Load() (Config, error) {
	c := Default()
	b, err := os.ReadFile(Path())
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := toml.Unmarshal(b, &c); err != nil {
		return Default(), err
	}
	return c.normalized(), nil
}

// Save writes the config, creating the directory if needed.
func (c Config) Save() error {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := toml.Marshal(c.normalized())
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (c Config) normalized() Config {
	d := Default()
	if c.Format == "" {
		c.Format = d.Format
	}
	if c.Quality <= 0 {
		c.Quality = d.Quality
	}
	if c.Dir == "" {
		c.Dir = d.Dir
	}
	if c.Template == "" {
		c.Template = d.Template
	}
	if c.CacheDays < 0 {
		c.CacheDays = 0
	}
	switch c.Theme {
	case "crypt", "bone", "mono":
	default:
		c.Theme = d.Theme
	}
	switch c.Wordmark {
	case "dots", "block", "plain":
	default:
		c.Wordmark = d.Wordmark
	}
	switch c.Density {
	case "comfortable", "compact":
	default:
		c.Density = d.Density
	}
	switch c.Motion {
	case "full", "reduced":
	default:
		c.Motion = d.Motion
	}
	return c
}
