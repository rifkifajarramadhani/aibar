// Package statefile persists last-good usage snapshots to a JSON file, so a
// Waybar restart or a crash never blanks the bar. It implements the
// usage.SnapshotArchive port and owns the on-disk format and the atomic write.
package statefile

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/overhaul/aibar/internal/usage"
)

// Archive reads and writes the state file at Path.
type Archive struct {
	Path string
}

var _ usage.SnapshotArchive = (*Archive)(nil)

func New(path string) *Archive {
	return &Archive{Path: path}
}

type diskState struct {
	Providers map[string]usage.Snapshot `json:"providers"`
}

// Load returns the persisted snapshots. A missing file is not an error.
func (a *Archive) Load() ([]usage.Snapshot, error) {
	data, err := os.ReadFile(a.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	var saved diskState
	if err := json.Unmarshal(data, &saved); err != nil {
		return nil, err
	}

	snapshots := make([]usage.Snapshot, 0, len(saved.Providers))

	for provider, snapshot := range saved.Providers {
		if snapshot.Provider == "" {
			snapshot.Provider = provider
		}

		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}

// Save writes the snapshots atomically with restrictive permissions.
func (a *Archive) Save(snapshots []usage.Snapshot) error {
	saved := diskState{Providers: make(map[string]usage.Snapshot, len(snapshots))}
	for _, snapshot := range snapshots {
		saved.Providers[snapshot.Provider] = snapshot
	}

	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(a.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		return err
	}

	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}

	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}

	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, a.Path)
}
