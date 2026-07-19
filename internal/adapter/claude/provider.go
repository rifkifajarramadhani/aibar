package claude

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/overhaul/aibar/internal/usage"
)

const (
	DefaultEndpoint    = "https://api.anthropic.com/api/oauth/usage"
	MinNetworkInterval = 5 * time.Minute
	requestTimeout     = 10 * time.Second
	maxBackoff         = time.Hour
)

type Config struct {
	CredentialsPath string
	ProjectsRoot    string
	Endpoint        string
	HTTPClient      *http.Client
	Now             func() time.Time
	Debounce        time.Duration
}

type Provider struct {
	config  Config
	trigger chan struct{}

	mu      sync.Mutex
	backoff time.Duration
}

var (
	_ usage.Provider    = (*Provider)(nil)
	_ usage.Refreshable = (*Provider)(nil)
)

func New(config Config) *Provider {
	if config.Endpoint == "" {
		config.Endpoint = DefaultEndpoint
	}

	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: requestTimeout}
	}

	if config.Now == nil {
		config.Now = time.Now
	}

	if config.Debounce <= 0 {
		config.Debounce = 250 * time.Millisecond
	}

	return &Provider{config: config, trigger: make(chan struct{}, 1)}
}

func (p *Provider) Name() string { return "claude" }

func (p *Provider) MinInterval() time.Duration { return MinNetworkInterval }

func (p *Provider) Fetch(ctx context.Context) (usage.Snapshot, error) {
	now := p.config.Now()
	credentials, err := LoadCredentials(p.config.CredentialsPath, now)
	if err != nil {
		return usage.Snapshot{Provider: p.Name(), Source: usage.SourceNetwork, FetchedAt: now, Err: err}, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, p.config.Endpoint, nil)
	if err != nil {
		err = usage.NewProviderError(usage.ErrorNetwork, errors.New("create claude usage request"))
		return usage.Snapshot{Provider: p.Name(), Source: usage.SourceNetwork, FetchedAt: now, Err: err}, err
	}

	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+credentials.AccessToken)
	request.Header.Set("anthropic-beta", "oauth-2025-04-20")

	response, err := p.config.HTTPClient.Do(request)
	if err != nil {
		message := "claude usage request failed"
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			message = "claude usage request timed out"
		}

		classified := usage.NewProviderError(usage.ErrorNetwork, &httpFailure{err: errors.New(message), cause: err})
		return usage.Snapshot{Provider: p.Name(), Source: usage.SourceNetwork, FetchedAt: now, Err: classified}, classified
	}

	defer func() { _ = response.Body.Close() }()

	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		err = usage.NewProviderError(usage.ErrorAuth, &httpFailure{status: response.StatusCode, err: errors.New("claude usage request was unauthorized")})
		return usage.Snapshot{Provider: p.Name(), Source: usage.SourceNetwork, FetchedAt: now, Err: err}, err
	}

	if response.StatusCode == http.StatusTooManyRequests {
		retryAfter := parseRetryAfter(response.Header.Get("Retry-After"), now)
		err = usage.NewProviderError(usage.ErrorRateLimit, &httpFailure{status: response.StatusCode, retryAfter: retryAfter, err: errors.New("claude usage request was rate limited")})
		return usage.Snapshot{Provider: p.Name(), Source: usage.SourceNetwork, FetchedAt: now, Err: err}, err
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		kind := usage.ErrorNetwork
		if response.StatusCode >= http.StatusBadRequest && response.StatusCode < http.StatusInternalServerError {
			kind = usage.ErrorParse
		}

		err = usage.NewProviderError(kind, &httpFailure{status: response.StatusCode, err: fmt.Errorf("claude usage request returned HTTP %d", response.StatusCode)})
		return usage.Snapshot{Provider: p.Name(), Source: usage.SourceNetwork, FetchedAt: now, Err: err}, err
	}

	snapshot, err := ParseUsage(response.Body, now)
	if err != nil {
		err = usage.NewProviderError(usage.ErrorParse, err)
		return usage.Snapshot{Provider: p.Name(), Source: usage.SourceNetwork, FetchedAt: now, Err: err}, err
	}

	snapshot.Provider = p.Name()
	snapshot.Source = usage.SourceNetwork

	return snapshot, nil
}

func (p *Provider) Refresh(context.Context, chan<- usage.Snapshot) {
	select {
	case p.trigger <- struct{}{}:
	default:
	}
}

func (p *Provider) nextDelay(err error) time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err == nil {
		p.backoff = 0
		return p.MinInterval()
	}

	var failure *httpFailure
	if errors.As(err, &failure) && failure.retryAfter > 0 {
		if failure.retryAfter < p.MinInterval() {
			return p.MinInterval()
		}

		return failure.retryAfter
	}

	if !isTransient(err) {
		return p.MinInterval()
	}

	if p.backoff == 0 {
		p.backoff = p.MinInterval()
	} else {
		p.backoff *= 2
		if p.backoff > maxBackoff {
			p.backoff = maxBackoff
		}
	}

	return p.backoff
}

func isTransient(err error) bool {
	var failure *httpFailure
	if errors.As(err, &failure) {
		return failure.status == 0 || failure.status >= http.StatusInternalServerError
	}

	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, net.ErrClosed)
}

type httpFailure struct {
	status     int
	retryAfter time.Duration
	err        error
	cause      error
}

func (e *httpFailure) Error() string {
	if e == nil || e.err == nil {
		return "claude usage request failed"
	}

	return e.err.Error()
}

func (e *httpFailure) Unwrap() error {
	if e == nil {
		return nil
	}

	if e.cause != nil {
		return e.cause
	}

	return e.err
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}

	if timestamp, err := http.ParseTime(value); err == nil {
		return timestamp.Sub(now)
	}

	return 0
}

func credentialsParent(path string) string {
	if path == "" {
		return ""
	}

	return filepath.Dir(path)
}

func pathExists(path string) bool {
	if path == "" {
		return false
	}

	_, err := os.Stat(path)
	return err == nil
}

func emit(ctx context.Context, out chan<- usage.Snapshot, snapshot usage.Snapshot) {
	select {
	case out <- snapshot:
	case <-ctx.Done():
	}
}
