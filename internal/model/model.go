package model

import (
	"context"
	"errors"
	"time"
)

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

type Window struct {
	Label         string    `json:"label"`
	UsedPct       float64   `json:"used_pct"`
	ResetsAt      time.Time `json:"resets_at"`
	WindowMinutes int       `json:"window_minutes,omitempty"`
}

type Snapshot struct {
	Provider  string    `json:"provider"`
	Windows   []Window  `json:"windows"`
	FetchedAt time.Time `json:"fetched_at"`
	Source    Source    `json:"source"`
	Err       error     `json:"-"`
}

var (
	ErrNoLocalSource = errors.New("provider has no local source")
	ErrNoSnapshot    = errors.New("no usable usage snapshot")
)

type Provider interface {
	Name() string
	Fetch(context.Context) (Snapshot, error)
	MinInterval() time.Duration
	Watch(context.Context, chan<- Snapshot) error
}

func (s Snapshot) Good() bool {
	return s.Err == nil && s.Provider != "" && len(s.Windows) > 0
}

func (s Snapshot) Clone() Snapshot {
	s.Windows = append([]Window(nil), s.Windows...)
	return s
}
