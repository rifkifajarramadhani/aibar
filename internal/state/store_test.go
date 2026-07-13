package state

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/overhaul/aibar/internal/model"
)

func TestLocalSnapshotCannotOverwriteNewerNetworkAnchor(t *testing.T) {
	store := New()
	anchorTime := time.Now()
	network := model.Snapshot{Provider: "claude", Source: model.SourceNetwork, FetchedAt: anchorTime, Windows: []model.Window{{Label: "5h", UsedPct: 50}}}
	local := model.Snapshot{Provider: "claude", Source: model.SourceLocal, FetchedAt: anchorTime.Add(-time.Second), Windows: []model.Window{{Label: "5h", UsedPct: 55}}}
	if !store.Apply(network) {
		t.Fatal("network snapshot was not applied")
	}
	if store.Apply(local) {
		t.Fatal("older local snapshot overwrote network anchor")
	}
	got, _ := store.Snapshot("claude")
	if got.Windows[0].UsedPct != 50 {
		t.Fatalf("got %.1f, want 50", got.Windows[0].UsedPct)
	}
}

func TestErrorKeepsLastGoodSnapshot(t *testing.T) {
	store := New()
	good := model.Snapshot{Provider: "codex", Source: model.SourceLocal, FetchedAt: time.Now(), Windows: []model.Window{{Label: "weekly", UsedPct: 4}}}
	if !store.Apply(good) {
		t.Fatal("good snapshot was not applied")
	}
	store.Apply(model.Snapshot{Provider: "codex", Source: model.SourceLocal, Err: errors.New("watch failed")})
	got, _ := store.Snapshot("codex")
	if got.Windows[0].UsedPct != 4 || got.Err == nil {
		t.Fatalf("last good data was not retained: %#v", got)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	store := New()
	snapshot := model.Snapshot{Provider: "codex", Source: model.SourceLocal, FetchedAt: time.Now().UTC(), Windows: []model.Window{{Label: "weekly", UsedPct: 12.5}}}
	store.Apply(snapshot)
	if err := store.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded := New()
	if err := loaded.Load(path); err != nil {
		t.Fatal(err)
	}
	got, ok := loaded.Snapshot("codex")
	if !ok || got.Windows[0] != snapshot.Windows[0] {
		t.Fatalf("round trip mismatch: %#v", got)
	}
}
