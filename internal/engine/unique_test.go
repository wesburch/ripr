package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUniquePath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "song.mp3")
	if got := uniquePath(p); got != p {
		t.Fatalf("fresh path changed: %q", got)
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "song (2).mp3")
	if got := uniquePath(p); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if err := os.WriteFile(want, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := uniquePath(p); got != filepath.Join(dir, "song (3).mp3") {
		t.Fatalf("got %q", got)
	}
}
