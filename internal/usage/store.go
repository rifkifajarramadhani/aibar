package usage

import "sync"

// Store holds the merged, last-good usage for every provider. It owns the merge
// policy and the restore policy; on-disk persistence is delegated to the
// injected SnapshotArchive port.
type Store struct {
	mu      sync.RWMutex
	current map[string]Snapshot
	anchors map[string]Snapshot
	archive SnapshotArchive
}

func NewStore(archive SnapshotArchive) *Store {
	return &Store{
		current: make(map[string]Snapshot),
		anchors: make(map[string]Snapshot),
		archive: archive,
	}
}

// Apply merges a snapshot while preserving the last good windows when a
// provider reports an error. Network data establishes an anchor that older
// local observations cannot overwrite.
func (s *Store) Apply(snapshot Snapshot) bool {
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

	if snapshot.Source == SourceLocal {
		if anchor, ok := s.anchors[snapshot.Provider]; ok && !snapshot.FetchedAt.After(anchor.FetchedAt) {
			return false
		}
	}

	snapshot.Err = nil
	s.current[snapshot.Provider] = snapshot.Clone()

	if snapshot.Source == SourceNetwork {
		s.anchors[snapshot.Provider] = snapshot.Clone()
	}

	return !exists || !sameSnapshot(previous, snapshot)
}

func (s *Store) Snapshot(provider string) (Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot, ok := s.current[provider]

	return snapshot.Clone(), ok
}

func (s *Store) All() []Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	providers := make([]Snapshot, 0, len(s.current))
	for _, snapshot := range s.current {
		providers = append(providers, snapshot.Clone())
	}

	return providers
}

// Load restores last-good snapshots through the archive. Snapshots without a
// source or windows are ignored, and network snapshots re-establish anchors.
func (s *Store) Load() error {
	if s.archive == nil {
		return nil
	}

	saved, err := s.archive.Load()
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, snapshot := range saved {
		if snapshot.Provider == "" || snapshot.Source == SourceUnknown || len(snapshot.Windows) == 0 {
			continue
		}

		s.current[snapshot.Provider] = snapshot.Clone()
		if snapshot.Source == SourceNetwork {
			s.anchors[snapshot.Provider] = snapshot.Clone()
		}
	}

	return nil
}

// Save writes the current good snapshots through the archive.
func (s *Store) Save() error {
	if s.archive == nil {
		return nil
	}

	s.mu.RLock()
	good := make([]Snapshot, 0, len(s.current))

	for _, snapshot := range s.current {
		if snapshot.Err == nil && snapshot.Good() {
			good = append(good, snapshot.Clone())
		}
	}
	s.mu.RUnlock()

	return s.archive.Save(good)
}

func sameSnapshot(a, b Snapshot) bool {
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
