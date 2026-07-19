package usage

import (
	"errors"
	"testing"
	"time"
)

func TestLocalSnapshotCannotOverwriteNewerNetworkAnchor(t *testing.T) {
	store := NewStore(nil)
	anchorTime := time.Now()
	network := Snapshot{Provider: "claude", Source: SourceNetwork, FetchedAt: anchorTime, Windows: []Window{{Label: "5h", UsedPct: 50}}}
	local := Snapshot{Provider: "claude", Source: SourceLocal, FetchedAt: anchorTime.Add(-time.Second), Windows: []Window{{Label: "5h", UsedPct: 55}}}

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
	store := NewStore(nil)
	good := Snapshot{Provider: "codex", Source: SourceLocal, FetchedAt: time.Now(), Windows: []Window{{Label: "weekly", UsedPct: 4}}}

	if !store.Apply(good) {
		t.Fatal("good snapshot was not applied")
	}

	store.Apply(Snapshot{Provider: "codex", Source: SourceLocal, Err: errors.New("watch failed")})

	got, _ := store.Snapshot("codex")
	if got.Windows[0].UsedPct != 4 || got.Err == nil {
		t.Fatalf("last good data was not retained: %#v", got)
	}
}
