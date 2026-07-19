package statefile

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/overhaul/aibar/internal/usage"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	archive := New(path)
	snapshot := usage.Snapshot{Provider: "codex", Source: usage.SourceLocal, FetchedAt: time.Now().UTC(), Windows: []usage.Window{{Label: "weekly", UsedPct: 12.5}}}

	if err := archive.Save([]usage.Snapshot{snapshot}); err != nil {
		t.Fatal(err)
	}

	loaded, err := archive.Load()
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded) != 1 || loaded[0].Windows[0] != snapshot.Windows[0] {
		t.Fatalf("round trip mismatch: %#v", loaded)
	}
}

func TestSaveUsesRestrictivePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	archive := New(path)

	if err := archive.Save([]usage.Snapshot{{Provider: "codex", Source: usage.SourceLocal, Windows: []usage.Window{{Label: "5h", UsedPct: 1}}}}); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("state file mode = %o, want 600", perm)
	}
}

func TestLoadMissingFileIsNotAnError(t *testing.T) {
	archive := New(filepath.Join(t.TempDir(), "absent.json"))

	loaded, err := archive.Load()
	if err != nil {
		t.Fatal(err)
	}

	if loaded != nil {
		t.Fatalf("expected nil snapshots, got %#v", loaded)
	}
}
