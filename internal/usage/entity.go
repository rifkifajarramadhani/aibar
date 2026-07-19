package usage

import "time"

// Source describes where a snapshot came from. Network snapshots are kept in
// the model even though the first milestone only implements local Codex data.
type Source int

const (
	SourceUnknown Source = iota
	SourceLocal
	SourceNetwork
)

func (s Source) String() string {
	switch s {
	case SourceLocal:
		return "local"
	case SourceNetwork:
		return "network"
	default:
		return "unknown"
	}
}

// Window is a single usage limit window (for example the rolling five-hour or
// weekly window) with its current utilization and reset time.
type Window struct {
	Label         string    `json:"label"`
	UsedPct       float64   `json:"used_pct"`
	ResetsAt      time.Time `json:"resets_at"`
	WindowMinutes int       `json:"window_minutes,omitempty"`
}

// Snapshot is one provider's observed usage at a point in time. Err carries a
// provider-local failure while the last-good Windows are preserved by the store.
type Snapshot struct {
	Provider  string    `json:"provider"`
	Windows   []Window  `json:"windows"`
	FetchedAt time.Time `json:"fetched_at"`
	Source    Source    `json:"source"`
	Err       error     `json:"-"`
}

// Good reports whether the snapshot carries usable, error-free usage data.
func (s Snapshot) Good() bool {
	return s.Err == nil && s.Provider != "" && len(s.Windows) > 0
}

// Clone returns a copy whose Windows slice is safe to hand to other goroutines.
func (s Snapshot) Clone() Snapshot {
	s.Windows = append([]Window(nil), s.Windows...)
	return s
}
