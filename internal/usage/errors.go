package usage

import "errors"

// ErrorKind classifies a provider failure so the renderer can surface an
// explicit stale/auth/rate-limit state instead of showing old data as fresh.
type ErrorKind string

const (
	ErrorAuth      ErrorKind = "auth-error"
	ErrorRateLimit ErrorKind = "rate-limit"
	ErrorNetwork   ErrorKind = "network-error"
	ErrorParse     ErrorKind = "parse-error"
)

// ProviderError wraps an underlying error with a classification kind. Providers
// return it so a single provider's failure degrades visibly without blanking
// the whole bar.
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

// ErrorKindOf returns the classification of err, or the empty kind when err is
// not a ProviderError.
func ErrorKindOf(err error) ErrorKind {
	var providerErr *ProviderError
	if errors.As(err, &providerErr) {
		return providerErr.Kind
	}

	return ""
}

var (
	ErrNoLocalSource = errors.New("provider has no local source")
	ErrNoSnapshot    = errors.New("no usable usage snapshot")
)
