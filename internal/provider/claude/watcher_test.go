package claude

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/overhaul/aibar/internal/model"
)

func TestWatcherFetchesInitiallyAndDebouncesLocalUsageWithinInterval(t *testing.T) {
	projects := t.TempDir()
	project := filepath.Join(projects, "project")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}

	fixture, err := os.ReadFile("../../../testdata/claude/project-fixture.jsonl")
	if err != nil {
		t.Fatal(err)
	}

	rollout := filepath.Join(project, "session.jsonl")
	if err := os.WriteFile(rollout, fixture, 0o600); err != nil {
		t.Fatal(err)
	}

	credentials := writeCredentials(t, `{"claudeAiOauth":{"accessToken":"test-token"}}`)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		_, _ = writer.Write([]byte(`{"five_hour":{"utilization":12}}`))
	}))
	defer server.Close()

	provider := New(Config{CredentialsPath: credentials, ProjectsRoot: projects, Endpoint: server.URL, HTTPClient: server.Client(), Debounce: 25 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan model.Snapshot, 2)
	errCh := make(chan error, 1)
	go func() { errCh <- provider.Watch(ctx, out) }()

	select {
	case snapshot := <-out:
		if snapshot.Err != nil || snapshot.Provider != "claude" || len(snapshot.Windows) != 1 {
			t.Fatalf("unexpected initial snapshot: %#v", snapshot)
		}
	case err := <-errCh:
		t.Fatalf("watcher exited before initial snapshot: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for initial snapshot")
	}

	appendUsage := "\n{\"type\":\"assistant\",\"message\":{\"role\":\"assistant\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n"
	file, err := os.OpenFile(rollout, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := file.WriteString(appendUsage); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	_ = file.Close()

	time.Sleep(150 * time.Millisecond)
	if got := requests.Load(); got != 1 {
		t.Fatalf("got %d requests after local change, want 1 due to interval gate", got)
	}

	newProject := filepath.Join(projects, "new-project")
	if err := os.MkdirAll(newProject, 0o700); err != nil {
		t.Fatal(err)
	}

	newSession := filepath.Join(newProject, "new-session.jsonl")
	if err := os.WriteFile(newSession, []byte("{\"type\":\"assistant\",\"message\":{\"role\":\"assistant\",\"usage\":{\"input_tokens\":3}}}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	time.Sleep(150 * time.Millisecond)
	if got := requests.Load(); got != 1 {
		t.Fatalf("got %d requests after new project, want 1 due to interval gate", got)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("watcher returned error on cancellation: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop")
	}
}

func TestUsageChangedDetectsTruncationAndRemoval(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	usage := []byte("{\"type\":\"assistant\",\"message\":{\"role\":\"assistant\",\"usage\":{\"input_tokens\":3}}}\n")
	if err := os.WriteFile(path, usage, 0o600); err != nil {
		t.Fatal(err)
	}

	state, err := readUsageState(path)
	if err != nil {
		t.Fatal(err)
	}

	states := map[string]fileUsageState{path: state}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	if !usageChanged(map[string]struct{}{path: {}}, states) {
		t.Fatal("truncation did not trigger a usage change")
	}

	states[path] = state

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	if !usageChanged(map[string]struct{}{path: {}}, states) {
		t.Fatal("removal did not trigger a usage change")
	}
}

func TestWatcherRefreshesWhenCredentialsAppear(t *testing.T) {
	projects := t.TempDir()
	credentials := filepath.Join(t.TempDir(), ".credentials.json")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		_, _ = writer.Write([]byte(`{"five_hour":{"utilization":12}}`))
	}))
	defer server.Close()

	provider := New(Config{CredentialsPath: credentials, ProjectsRoot: projects, Endpoint: server.URL, HTTPClient: server.Client(), Debounce: 25 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan model.Snapshot, 1)
	errCh := make(chan error, 1)
	go func() { errCh <- provider.Watch(ctx, out) }()

	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(credentials, []byte(`{"claudeAiOauth":{"accessToken":"test-token"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	select {
	case snapshot := <-out:
		if snapshot.Err != nil || snapshot.Provider != "claude" {
			t.Fatalf("unexpected snapshot: %#v", snapshot)
		}
	case err := <-errCh:
		t.Fatalf("watcher exited before credential snapshot: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for credential-triggered fetch")
	}

	if requests.Load() != 1 {
		t.Fatalf("got %d requests, want 1", requests.Load())
	}

	cancel()
}
