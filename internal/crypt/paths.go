package crypt

import (
	"os"
	"path/filepath"
	"runtime"
)

func home() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return os.Getenv("HOME")
}

func defaultStorePath() string {
	return filepath.Join(home(), ".local", "share", "ripr", "crypt.json")
}

func defaultCacheDir() string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(home(), "Library", "Caches", "ripr")
	}
	if x := os.Getenv("XDG_CACHE_HOME"); x != "" {
		return filepath.Join(x, "ripr")
	}
	return filepath.Join(home(), ".cache", "ripr")
}
