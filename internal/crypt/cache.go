package crypt

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

func sweep(cacheDir string, days int) (int, error) {
	items, err := os.ReadDir(cacheDir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	removed := 0
	for _, it := range items {
		if !it.Type().IsRegular() {
			continue
		}
		info, err := it.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		if os.Remove(filepath.Join(cacheDir, it.Name())) == nil {
			removed++
		}
	}
	return removed, nil
}
