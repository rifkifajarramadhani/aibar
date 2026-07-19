package codex

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseClassifiesWindowsByDuration(t *testing.T) {
	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	input := `{"type":"event_msg","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":42.5,"window_minutes":10080,"resets_at":1784534584},"secondary":{"used_percent":18,"window_minutes":300,"resets_in_seconds":600}}}}`

	snapshot, err := Parse(strings.NewReader(input), now)
	if err != nil {
		t.Fatal(err)
	}

	if len(snapshot.Windows) != 2 {
		t.Fatalf("got %d windows, want 2", len(snapshot.Windows))
	}

	if snapshot.Windows[0].Label != "weekly" || snapshot.Windows[0].UsedPct != 42.5 {
		t.Fatalf("unexpected weekly window: %#v", snapshot.Windows[0])
	}

	if snapshot.Windows[1].Label != "5h" || snapshot.Windows[1].UsedPct != 18 {
		t.Fatalf("unexpected 5h window: %#v", snapshot.Windows[1])
	}

	if !snapshot.Windows[1].ResetsAt.Equal(now.Add(10 * time.Minute)) {
		t.Fatalf("unexpected reset: %s", snapshot.Windows[1].ResetsAt)
	}
}

func TestParseUsesLatestCompleteTokenCountEvent(t *testing.T) {
	now := time.Now()
	input := strings.Join([]string{
		`{"type":"event_msg","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":10,"window_minutes":10080}}}}`,
		`{"type":"event_msg","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":20,"window_minutes":10080}}}}`,
		`{"type":"event_msg","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":30,"window_minutes":10080}}`,
	}, "\n")

	snapshot, err := Parse(strings.NewReader(input), now)
	if err != nil {
		t.Fatal(err)
	}

	if got := snapshot.Windows[0].UsedPct; got != 20 {
		t.Fatalf("got %.1f, want 20", got)
	}
}

func TestParseIgnoresUnknownAndMissingWindows(t *testing.T) {
	now := time.Now()
	input := `{"type":"event_msg","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":30,"window_minutes":60},"secondary":null}}}`

	if _, err := Parse(strings.NewReader(input), now); !errors.Is(err, ErrNoWindows) {
		t.Fatalf("got %v, want ErrNoWindows", err)
	}
}

func TestFindNewest(t *testing.T) {
	root := t.TempDir()
	old := root + "/2026/07/old/rollout-old.jsonl"
	newest := root + "/2026/07/new/rollout-new.jsonl"

	for _, path := range []string{old, newest} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	oldTime := time.Now().Add(-time.Minute)
	if err := os.Chtimes(old, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	got, err := FindNewest(root)
	if err != nil {
		t.Fatal(err)
	}

	if got != newest {
		t.Fatalf("got %s, want %s", got, newest)
	}
}
