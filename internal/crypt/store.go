package crypt

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(data, &s.entries); err != nil {
		aside := fmt.Sprintf("%s.corrupt-%d", path, time.Now().Unix())
		if rerr := os.Rename(path, aside); rerr != nil {
			return nil, rerr
		}
		s.entries = nil
	}
	return s, nil
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".crypt-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, s.path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

func (s *Store) add(e Entry) error {
	kept := make([]Entry, 0, len(s.entries)+1)
	kept = append(kept, e)
	for _, old := range s.entries {
		if old.Path != e.Path {
			kept = append(kept, old)
		}
	}
	s.entries = kept
	return s.save()
}

func (s *Store) remove(id string) error {
	kept := make([]Entry, 0, len(s.entries))
	for _, old := range s.entries {
		if old.ID != id {
			kept = append(kept, old)
		}
	}
	s.entries = kept
	return s.save()
}

func (s *Store) list() []Entry {
	out := make([]Entry, len(s.entries))
	copy(out, s.entries)
	return out
}

func (s *Store) stats() (int, int64) {
	var n int64
	for _, e := range s.entries {
		n += e.Size
	}
	return len(s.entries), n
}
