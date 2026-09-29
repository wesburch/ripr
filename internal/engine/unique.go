package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// uniquePath returns p, or p with " (2)", " (3)", … inserted before the
// extension when a file already exists there. ripr never overwrites.
func uniquePath(p string) string {
	if _, err := os.Stat(p); err != nil {
		return p
	}
	ext := filepath.Ext(p)
	base := strings.TrimSuffix(p, ext)
	for n := 2; n < 1000; n++ {
		q := fmt.Sprintf("%s (%d)%s", base, n, ext)
		if _, err := os.Stat(q); err != nil {
			return q
		}
	}
	return p
}
