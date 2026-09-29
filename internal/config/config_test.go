package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTripAndDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Format != "mp3" || c.Quality != 320 || c.CacheDays != 7 || c.Theme != "crypt" {
		t.Fatalf("defaults wrong: %+v", c)
	}
	c.Format = "flac"
	c.Theme = "bone"
	c.Mascot = false
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "ripr", "config.toml")); err != nil {
		t.Fatal("config not written where Path() says")
	}
	back, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if back.Format != "flac" || back.Theme != "bone" || back.Mascot {
		t.Fatalf("round trip lost values: %+v", back)
	}
}

func TestUnknownValuesFallBack(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	p := filepath.Join(dir, "ripr", "config.toml")
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte("theme = \"neon\"\nwordmark = \"huge\"\nquality = -5\n"), 0o644)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Theme != "crypt" || c.Wordmark != "dots" || c.Quality != 320 {
		t.Fatalf("bad values not normalized: %+v", c)
	}
}
