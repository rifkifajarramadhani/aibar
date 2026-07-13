package claude

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/overhaul/aibar/internal/model"
)

func TestFetchParsesUsageAndSendsOAuthHeaders(t *testing.T) {
	path := writeCredentials(t, `{"claudeAiOauth":{"accessToken":"test-token"}}`)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("authorization header = %q", got)
		}

		if got := request.Header.Get("anthropic-beta"); got != "oauth-2025-04-20" {
			t.Errorf("anthropic-beta header = %q", got)
		}

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"five_hour":{"utilization":25,"resets_at":"2026-07-13T09:00:00Z"}}`))
	}))
	defer server.Close()

	provider := New(Config{CredentialsPath: path, Endpoint: server.URL, HTTPClient: server.Client()})
	snapshot, err := provider.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if snapshot.Provider != "claude" || snapshot.Source != model.SourceNetwork || len(snapshot.Windows) != 1 || snapshot.Windows[0].UsedPct != 25 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
}

func TestFetchClassifiesAuthAndRateLimitErrorsWithoutResponseBody(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		retryAfter string
		kind       model.ErrorKind
	}{
		{name: "auth", status: http.StatusUnauthorized, kind: model.ErrorAuth},
		{name: "forbidden", status: http.StatusForbidden, kind: model.ErrorAuth},
		{name: "rate limit", status: http.StatusTooManyRequests, retryAfter: "600", kind: model.ErrorRateLimit},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeCredentials(t, `{"claudeAiOauth":{"accessToken":"test-token"}}`)
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if test.retryAfter != "" {
					writer.Header().Set("Retry-After", test.retryAfter)
				}

				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte("secret response body that must not leak"))
			}))
			defer server.Close()

			provider := New(Config{CredentialsPath: path, Endpoint: server.URL, HTTPClient: server.Client()})
			snapshot, err := provider.Fetch(context.Background())
			if model.ErrorKindOf(err) != test.kind || model.ErrorKindOf(snapshot.Err) != test.kind {
				t.Fatalf("err=%v snapshot=%v, want %s", err, snapshot.Err, test.kind)
			}

			if strings.Contains(err.Error(), "secret response body") {
				t.Fatal("response body leaked into error")
			}
		})
	}
}

func TestFetchClassifiesServerAndParseErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		kind   model.ErrorKind
	}{
		{name: "server", status: http.StatusBadGateway, body: "bad gateway", kind: model.ErrorNetwork},
		{name: "malformed", status: http.StatusOK, body: "{", kind: model.ErrorParse},
		{name: "missing windows", status: http.StatusOK, body: `{}`, kind: model.ErrorParse},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeCredentials(t, `{"claudeAiOauth":{"accessToken":"test-token"}}`)
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()

			provider := New(Config{CredentialsPath: path, Endpoint: server.URL, HTTPClient: server.Client()})
			snapshot, err := provider.Fetch(context.Background())
			if model.ErrorKindOf(err) != test.kind || model.ErrorKindOf(snapshot.Err) != test.kind {
				t.Fatalf("err=%v snapshot=%v, want %s", err, snapshot.Err, test.kind)
			}
		})
	}
}

func TestNextDelayEnforcesMinimumAndBacksOffTransientFailures(t *testing.T) {
	provider := New(Config{})

	if got := provider.nextDelay(nil); got != MinNetworkInterval {
		t.Fatalf("success delay = %s, want %s", got, MinNetworkInterval)
	}

	transient := model.NewProviderError(model.ErrorNetwork, &httpFailure{status: http.StatusBadGateway, err: errors.New("temporary failure")})
	if got := provider.nextDelay(transient); got != MinNetworkInterval {
		t.Fatalf("first transient delay = %s, want %s", got, MinNetworkInterval)
	}

	if got := provider.nextDelay(transient); got != 2*MinNetworkInterval {
		t.Fatalf("second transient delay = %s, want %s", got, 2*MinNetworkInterval)
	}

	rateLimited := model.NewProviderError(model.ErrorRateLimit, &httpFailure{retryAfter: 10 * time.Minute, err: errors.New("rate limited")})
	if got := provider.nextDelay(rateLimited); got != 10*time.Minute {
		t.Fatalf("rate-limit delay = %s, want 10m", got)
	}
}

func TestFetchHonorsCanceledContext(t *testing.T) {
	path := writeCredentials(t, `{"claudeAiOauth":{"accessToken":"test-token"}}`)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		<-request.Context().Done()
	}))
	defer server.Close()

	provider := New(Config{CredentialsPath: path, Endpoint: server.URL, HTTPClient: server.Client()})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := provider.Fetch(ctx)
	if model.ErrorKindOf(err) != model.ErrorNetwork || requests.Load() != 0 {
		t.Fatalf("got err=%v requests=%d", err, requests.Load())
	}
}

func TestFetchHonorsHTTPTimeout(t *testing.T) {
	path := writeCredentials(t, `{"claudeAiOauth":{"accessToken":"test-token"}}`)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		select {
		case <-time.After(200 * time.Millisecond):
		case <-request.Context().Done():
		}
	}))
	defer server.Close()

	provider := New(Config{
		CredentialsPath: path,
		Endpoint:        server.URL,
		HTTPClient:      &http.Client{Timeout: 20 * time.Millisecond},
	})

	_, err := provider.Fetch(context.Background())
	if model.ErrorKindOf(err) != model.ErrorNetwork {
		t.Fatalf("got %v, want network error", err)
	}
}

func TestDefaultEndpointAndCredentialParent(t *testing.T) {
	provider := New(Config{})
	if provider.config.Endpoint != DefaultEndpoint || provider.MinInterval() != 5*time.Minute {
		t.Fatalf("unexpected defaults: %#v", provider.config)
	}

	if got := credentialsParent(filepath.Join("/tmp", ".claude", ".credentials.json")); got != filepath.Join("/tmp", ".claude") {
		t.Fatalf("credential parent = %s", got)
	}
}
