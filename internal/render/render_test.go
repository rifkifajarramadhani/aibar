package render

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/overhaul/aibar/internal/model"
)

func TestBuildChoosesMostConstrainedWindow(t *testing.T) {
	now := time.Now()

	output := Build([]model.Snapshot{{Provider: "codex", FetchedAt: now, Windows: []model.Window{
		{Label: "weekly", UsedPct: 40, ResetsAt: now.Add(time.Hour)},
		{Label: "5h", UsedPct: 88, ResetsAt: now.Add(time.Hour)},
	}}}, View{}, now)
	if output.Text != Icon || output.Class != "warning" || output.Percentage != 88 {
		t.Fatalf("unexpected output: %#v", output)
	}

	if !strings.Contains(output.Tooltip, "resets in 1h 00m") {
		t.Fatalf("tooltip missing countdown: %s", output.Tooltip)
	}
}

func TestBuildShowsStaleWithLastGoodData(t *testing.T) {
	now := time.Now()

	output := Build([]model.Snapshot{{Provider: "codex", FetchedAt: now.Add(-time.Hour), Err: errFixture, Windows: []model.Window{{Label: "weekly", UsedPct: 12}}}}, View{}, now)
	if output.Class != "ok stale" {
		t.Fatalf("got class %q", output.Class)
	}

	if !strings.Contains(output.Tooltip, "status: stale") {
		t.Fatalf("tooltip missing stale status: %s", output.Tooltip)
	}
}

func TestBuildKeepsHealthyProviderVisibleWhenAnotherProviderNeedsAuth(t *testing.T) {
	now := time.Now()
	output := Build([]model.Snapshot{
		{Provider: "claude", Err: model.NewProviderError(model.ErrorAuth, errors.New("claude access token is expired"))},
		{Provider: "codex", FetchedAt: now, Windows: []model.Window{{Label: "weekly", UsedPct: 12}}},
	}, View{}, now)

	if output.Class != "ok auth-error stale" {
		t.Fatalf("got class %q", output.Class)
	}

	if !strings.Contains(output.Tooltip, "Claude:") || !strings.Contains(output.Tooltip, "status: auth-error") || output.Percentage != 12 {
		t.Fatalf("unexpected output: %#v", output)
	}
}

var errFixture = fixtureError("fixture watcher error")

type fixtureError string

func (e fixtureError) Error() string { return string(e) }
