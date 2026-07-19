package codex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/overhaul/aibar/internal/usage"
)

func TestWatcherEmitsInitialFixtureSnapshot(t *testing.T) {
	root := t.TempDir()

	day := filepath.Join(root, "2026", "07", "13")
	if err := os.MkdirAll(day, 0o700); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile("../../../testdata/codex/rollout-fixture.jsonl")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(day, "rollout-fixture.jsonl"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	watcher := NewWatcher(root)
	watcher.Now = func() time.Time { return time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC) }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan usage.Snapshot, 1)
	errCh := make(chan error, 1)

	go func() { errCh <- watcher.Watch(ctx, out) }()

	select {
	case snapshot := <-out:
		if snapshot.Err != nil || snapshot.Provider != "codex" || len(snapshot.Windows) != 2 {
			t.Fatalf("unexpected snapshot: %#v", snapshot)
		}
	case err := <-errCh:
		t.Fatalf("watcher exited before snapshot: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for fixture snapshot")
	}
	cancel()
}

func TestWatcherResolvesNewerRolloutAfterRotation(t *testing.T) {
	root := t.TempDir()

	day := filepath.Join(root, "2026", "07", "13")
	if err := os.MkdirAll(day, 0o700); err != nil {
		t.Fatal(err)
	}

	writeRollout := func(path string, used float64) {
		t.Helper()

		line := fmt.Sprintf(`{"type":"event_msg","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":%.1f,"window_minutes":10080}}}}`, used) + "\n"
		if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	old := filepath.Join(day, "rollout-old.jsonl")
	writeRollout(old, 10)

	oldTime := time.Now().Add(-time.Second)
	if err := os.Chtimes(old, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	watcher := NewWatcher(root)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan usage.Snapshot, 8)
	errCh := make(chan error, 1)

	go func() { errCh <- watcher.Watch(ctx, out) }()
	select {
	case <-out:
	case err := <-errCh:
		t.Fatalf("watcher exited before initial snapshot: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for initial snapshot")
	}

	newer := filepath.Join(day, "rollout-new.jsonl")
	writeRollout(newer, 88)
	select {
	case snapshot := <-out:
		if len(snapshot.Windows) == 0 || snapshot.Windows[0].UsedPct != 88 {
			t.Fatalf("got rotated snapshot %#v", snapshot)
		}
	case err := <-errCh:
		t.Fatalf("watcher exited after rotation: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for rotated snapshot")
	}
}
