package usage

import (
	"context"
	"time"
)

// Provider is the contract every usage source implements. It is deliberately
// shaped for both local (Codex) and network-backed (Claude, Cursor) providers:
// Fetch performs a one-shot reconciliation, Watch streams snapshots until the
// context ends, and MinInterval bounds how often a network source may Fetch.
type Provider interface {
	Name() string
	Fetch(context.Context) (Snapshot, error)
	MinInterval() time.Duration
	Watch(context.Context, chan<- Snapshot) error
}

// Refreshable is implemented by providers that can re-emit on demand, driving
// the refresh control command and the SIGUSR1 rescan.
type Refreshable interface {
	Refresh(context.Context, chan<- Snapshot)
}

// SnapshotArchive persists last-good snapshots so a restart never blanks the
// bar. It is a port: the store owns the restore/merge policy while the adapter
// owns the on-disk format and atomic write.
type SnapshotArchive interface {
	Load() ([]Snapshot, error)
	Save(snapshots []Snapshot) error
}
