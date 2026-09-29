package crypt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "crypt.json")
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	s.Add(Entry{ID: "a", Path: "/x/a.mp3", Size: 10})
	s.Add(Entry{ID: "b", Path: "/x/b.mp3", Size: 20})
	s.Add(Entry{ID: "a2", Path: "/x/a.mp3", Size: 5}) // replaces a
	s2, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	l := s2.List()
	if len(l) != 2 || l[0].ID != "a2" || l[1].ID != "b" {
		t.Fatalf("list = %+v", l)
	}
	if n, b := s2.Stats(); n != 2 || b != 25 {
		t.Fatalf("stats %d %d", n, b)
	}
	l[0].ID = "mut"
	if s2.List()[0].ID != "a2" {
		t.Fatal("list not a copy")
	}
	if err := s2.Remove("b"); err != nil {
		t.Fatal(err)
	}
	if n, _ := s2.Stats(); n != 1 {
		t.Fatal("remove failed")
	}
}

func TestCorrupt(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "crypt.json")
	os.WriteFile(p, []byte("{nope"), 0o644)
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.List()) != 0 {
		t.Fatal("expected empty")
	}
	m, _ := filepath.Glob(filepath.Join(dir, "crypt.json.corrupt-*"))
	if len(m) != 1 || !strings.Contains(m[0], "corrupt-") {
		t.Fatalf("aside = %v", m)
	}
}

func TestSweep(t *testing.T) {
	if n, err := Sweep(filepath.Join(t.TempDir(), "none"), 1); n != 0 || err != nil {
		t.Fatal(n, err)
	}
	dir := t.TempDir()
	old, fresh := filepath.Join(dir, "old.webm"), filepath.Join(dir, "new.webm")
	os.WriteFile(old, []byte("x"), 0o644)
	os.WriteFile(fresh, []byte("x"), 0o644)
	os.Mkdir(filepath.Join(dir, "d"), 0o755)
	past := time.Now().Add(-10 * 24 * time.Hour)
	os.Chtimes(old, past, past)
	n, err := Sweep(dir, 7)
	if err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatal("fresh removed")
	}
	if _, err := os.Stat(old); err == nil {
		t.Fatal("old kept")
	}
}
