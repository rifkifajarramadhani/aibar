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

type ErrorKind string

const (
	ErrorAuth      ErrorKind = "auth-error"
	ErrorRateLimit ErrorKind = "rate-limit"
	ErrorNetwork   ErrorKind = "network-error"
	ErrorParse     ErrorKind = "parse-error"
)

type ProviderError struct {
	Kind ErrorKind
	Err  error
}

func (e *ProviderError) Error() string {
	if e == nil || e.Err == nil {
		return string(e.Kind)
	}

	return e.Err.Error()
}

func (e *ProviderError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

func NewProviderError(kind ErrorKind, err error) error {
	return &ProviderError{Kind: kind, Err: err}
}

func ErrorKindOf(err error) ErrorKind {
	var providerErr *ProviderError
	if errors.As(err, &providerErr) {
		return providerErr.Kind
	}

	return ""
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

type Refreshable interface {
	Refresh(context.Context, chan<- Snapshot)
}

func (s Snapshot) Good() bool {
	return s.Err == nil && s.Provider != "" && len(s.Windows) > 0
}

func (s Snapshot) Clone() Snapshot {
	s.Windows = append([]Window(nil), s.Windows...)
	return s
}
