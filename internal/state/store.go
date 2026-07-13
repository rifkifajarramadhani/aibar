package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/overhaul/aibar/internal/model"
)

type Store struct {
	mu      sync.RWMutex
	current map[string]model.Snapshot
	anchors map[string]model.Snapshot
}

func New() *Store {
	return &Store{
		current: make(map[string]model.Snapshot),
		anchors: make(map[string]model.Snapshot),
	}
}

// Apply merges a snapshot while preserving the last good windows when a
// provider reports an error. Network data establishes an anchor that older
// local observations cannot overwrite.
func (s *Store) Apply(snapshot model.Snapshot) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	previous, exists := s.current[snapshot.Provider]
	if snapshot.Err != nil {
		changed := !exists || errorString(previous.Err) != errorString(snapshot.Err)
		if exists {
			previous.Err = snapshot.Err
			s.current[snapshot.Provider] = previous
		} else {
			s.current[snapshot.Provider] = snapshot.Clone()
		}
		return changed
	}

	if snapshot.Source == model.SourceLocal {
		if anchor, ok := s.anchors[snapshot.Provider]; ok && !snapshot.FetchedAt.After(anchor.FetchedAt) {
			return false
		}
	}

	snapshot.Err = nil
	s.current[snapshot.Provider] = snapshot.Clone()
	if snapshot.Source == model.SourceNetwork {
		s.anchors[snapshot.Provider] = snapshot.Clone()
	}
	return !exists || !sameSnapshot(previous, snapshot)
}

func (s *Store) Snapshot(provider string) (model.Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot, ok := s.current[provider]
	return snapshot.Clone(), ok
}

func (s *Store) All() []model.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	providers := make([]model.Snapshot, 0, len(s.current))
	for _, snapshot := range s.current {
		providers = append(providers, snapshot.Clone())
	}
	return providers
}

type diskState struct {
	Providers map[string]model.Snapshot `json:"providers"`
}

func (s *Store) Load(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	var saved diskState
	if err := json.Unmarshal(data, &saved); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for provider, snapshot := range saved.Providers {
		if snapshot.Provider == "" {
			snapshot.Provider = provider
		}
		if snapshot.Source == model.SourceUnknown || len(snapshot.Windows) == 0 {
			continue
		}
		s.current[provider] = snapshot.Clone()
		if snapshot.Source == model.SourceNetwork {
			s.anchors[provider] = snapshot.Clone()
		}
	}
	return nil
}

func (s *Store) Save(path string) error {
	s.mu.RLock()
	saved := diskState{Providers: make(map[string]model.Snapshot, len(s.current))}
	for provider, snapshot := range s.current {
		if snapshot.Err == nil && snapshot.Good() {
			saved.Providers[provider] = snapshot.Clone()
		}
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".state-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func sameSnapshot(a, b model.Snapshot) bool {
	if a.Provider != b.Provider || !a.FetchedAt.Equal(b.FetchedAt) || a.Source != b.Source || errorString(a.Err) != errorString(b.Err) || len(a.Windows) != len(b.Windows) {
		return false
	}
	for i := range a.Windows {
		if a.Windows[i] != b.Windows[i] {
			return false
		}
	}
	return true
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
