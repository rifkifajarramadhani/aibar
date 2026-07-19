package waybar

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/overhaul/aibar/internal/usage"
)

func TestBuildChoosesMostConstrainedWindow(t *testing.T) {
	now := time.Now()

	output := Build([]usage.Snapshot{{Provider: "codex", FetchedAt: now, Windows: []usage.Window{
		{Label: "weekly", UsedPct: 40, ResetsAt: now.Add(time.Hour)},
		{Label: "5h", UsedPct: 88, ResetsAt: now.Add(time.Hour)},
	}}}, usage.View{}, now)
	if output.Text != Icon || output.Class != "warning" || output.Percentage != 88 {
		t.Fatalf("unexpected output: %#v", output)
	}

	if !strings.Contains(output.Tooltip, "• Rolling Usage:") || !strings.Contains(output.Tooltip, "Resets in 1h 00m") {
		t.Fatalf("tooltip missing countdown: %s", output.Tooltip)
	}

	if strings.Contains(output.Tooltip, "updated") || strings.Contains(output.Tooltip, "pace:") {
		t.Fatalf("tooltip contains removed diagnostic fields: %s", output.Tooltip)
	}
}

func TestBuildShowsStaleWithLastGoodData(t *testing.T) {
	now := time.Now()

	output := Build([]usage.Snapshot{{Provider: "codex", FetchedAt: now.Add(-time.Hour), Err: errFixture, Windows: []usage.Window{{Label: "weekly", UsedPct: 12}}}}, usage.View{}, now)
	if output.Class != "ok stale" {
		t.Fatalf("got class %q", output.Class)
	}

	if !strings.Contains(output.Tooltip, "Status: stale — fixture watcher error") {
		t.Fatalf("tooltip missing stale status: %s", output.Tooltip)
	}
}

func TestBuildKeepsHealthyProviderVisibleWhenAnotherProviderNeedsAuth(t *testing.T) {
	now := time.Now()
	output := Build([]usage.Snapshot{
		{Provider: "claude", Err: usage.NewProviderError(usage.ErrorAuth, errors.New("claude access token is expired"))},
		{Provider: "codex", FetchedAt: now, Windows: []usage.Window{{Label: "weekly", UsedPct: 12}}},
	}, usage.View{}, now)

	if output.Class != "ok auth-error stale" {
		t.Fatalf("got class %q", output.Class)
	}

	if !strings.Contains(output.Tooltip, "Claude") || !strings.Contains(output.Tooltip, "Status: auth-error") || output.Percentage != 12 {
		t.Fatalf("unexpected output: %#v", output)
	}
}

func TestBuildAggregatesProvidersInStableOrder(t *testing.T) {
	now := time.Now()
	output := Build([]usage.Snapshot{
		{Provider: "codex", FetchedAt: now, Windows: []usage.Window{{Label: "weekly", UsedPct: 42}}},
		{Provider: "claude", FetchedAt: now, Windows: []usage.Window{{Label: "5h", UsedPct: 86}}},
	}, usage.View{}, now)

	if output.Percentage != 86 || output.Class != "warning" {
		t.Fatalf("unexpected aggregate output: %#v", output)
	}

	claudeIndex := strings.Index(output.Tooltip, "Claude")
	codexIndex := strings.Index(output.Tooltip, "Codex")
	if claudeIndex == -1 || codexIndex == -1 || claudeIndex > codexIndex {
		t.Fatalf("providers are not stably ordered: %s", output.Tooltip)
	}
}

func TestNavigateProviderUsesAggregateBoundaries(t *testing.T) {
	now := time.Now()
	snapshots := []usage.Snapshot{
		{Provider: "codex", FetchedAt: now, Windows: []usage.Window{{Label: "weekly", UsedPct: 42}}},
		{Provider: "claude", Err: usage.NewProviderError(usage.ErrorAuth, errors.New("expired"))},
	}

	view := usage.View{WindowIndex: 4}
	view = NavigateProvider(snapshots, view, 1)
	if view.PinnedProvider != "claude" || view.WindowIndex != 0 {
		t.Fatalf("next from aggregate got %#v", view)
	}

	view = NavigateProvider(snapshots, view, 1)
	if view.PinnedProvider != "codex" {
		t.Fatalf("next provider got %#v", view)
	}

	view = NavigateProvider(snapshots, view, 1)
	if view.PinnedProvider != "" {
		t.Fatalf("next at upper boundary got %#v", view)
	}

	view = NavigateProvider(snapshots, view, -1)
	if view.PinnedProvider != "codex" {
		t.Fatalf("prev from aggregate got %#v", view)
	}

	view = NavigateProvider(snapshots, view, -1)
	if view.PinnedProvider != "claude" {
		t.Fatalf("prev provider got %#v", view)
	}

	view = NavigateProvider(snapshots, view, -1)
	if view.PinnedProvider != "" {
		t.Fatalf("prev at lower boundary got %#v", view)
	}
}

func TestNavigateProviderTogglesSingleProvider(t *testing.T) {
	snapshots := []usage.Snapshot{{Provider: "codex", Windows: []usage.Window{{Label: "weekly", UsedPct: 42}}}}

	view := NavigateProvider(snapshots, usage.View{}, 1)
	if view.PinnedProvider != "codex" {
		t.Fatalf("provider was not pinned: %#v", view)
	}

	view = NavigateProvider(snapshots, view, 1)
	if view.PinnedProvider != "" {
		t.Fatalf("provider was not unpinned at boundary: %#v", view)
	}
}

func TestCycleWindowPreservesAutomaticIndexAndWraps(t *testing.T) {
	view := CycleWindow(usage.View{})
	if view.WindowIndex != 1 {
		t.Fatalf("first cycle got index %d", view.WindowIndex)
	}

	view = CycleWindow(view)
	view = CycleWindow(view)
	if view.WindowIndex != 3 {
		t.Fatalf("cycles did not advance: %d", view.WindowIndex)
	}

	now := time.Now()
	snapshots := []usage.Snapshot{{Provider: "codex", Windows: []usage.Window{
		{Label: "weekly", UsedPct: 20},
		{Label: "5h", UsedPct: 80},
	}}}
	output := Build(snapshots, usage.View{WindowIndex: view.WindowIndex}, now)
	if output.Percentage != 80 {
		t.Fatalf("window cycle did not wrap: %#v", output)
	}
}

func TestBuildPinnedProviderIsolatesUnrelatedErrors(t *testing.T) {
	now := time.Now()
	output := Build([]usage.Snapshot{
		{Provider: "claude", FetchedAt: now, Windows: []usage.Window{{Label: "5h", UsedPct: 86}}},
		{Provider: "codex", Err: usage.NewProviderError(usage.ErrorAuth, errors.New("expired"))},
	}, usage.View{PinnedProvider: "claude"}, now)

	if output.Class != "warning" || output.Percentage != 86 {
		t.Fatalf("pinned output was affected by unrelated error: %#v", output)
	}

	if !strings.Contains(output.Tooltip, "Status: auth-error") {
		t.Fatalf("pinned tooltip omitted unrelated provider status: %s", output.Tooltip)
	}
}

func TestBuildPinnedErrorOnlyProviderShowsStatus(t *testing.T) {
	now := time.Now()
	output := Build([]usage.Snapshot{
		{Provider: "claude", Err: usage.NewProviderError(usage.ErrorAuth, errors.New("expired"))},
		{Provider: "codex", FetchedAt: now, Windows: []usage.Window{{Label: "weekly", UsedPct: 12}}},
	}, usage.View{PinnedProvider: "claude"}, now)

	if output.Class != "stale auth-error" || output.Percentage != 0 {
		t.Fatalf("unexpected error-only pinned output: %#v", output)
	}

	if !strings.Contains(output.Tooltip, "Claude") || !strings.Contains(output.Tooltip, "Status: auth-error") {
		t.Fatalf("error-only pinned tooltip missing status: %s", output.Tooltip)
	}
}

func TestTooltipRendersAllProvidersAndWindows(t *testing.T) {
	now := time.Date(2026, time.July, 13, 8, 0, 0, 0, time.UTC)
	output := Build([]usage.Snapshot{
		{Provider: "codex", Windows: []usage.Window{
			{Label: "weekly", UsedPct: 53, ResetsAt: now.Add(3*24*time.Hour + 16*time.Hour)},
			{Label: "5h", UsedPct: 3, ResetsAt: now.Add(4*time.Hour + 25*time.Minute)},
		}},
		{Provider: "claude", Windows: []usage.Window{
			{Label: "weekly", UsedPct: 24, ResetsAt: now.Add(3*24*time.Hour + 16*time.Hour)},
			{Label: "5h", UsedPct: 2, ResetsAt: now.Add(4*time.Hour + 55*time.Minute)},
		}},
	}, usage.View{}, now)

	want := strings.Join([]string{
		"Claude",
		"• Rolling Usage:",
		"  [#-------------------]    2%",
		"  Resets in 4h 55m",
		"• Weekly Usage:",
		"  [#####---------------]   24%",
		"  Resets in 3d 16h",
		"",
		"Codex",
		"• Rolling Usage:",
		"  [#-------------------]    3%",
		"  Resets in 4h 25m",
		"• Weekly Usage:",
		"  [###########---------]   53%",
		"  Resets in 3d 16h",
	}, "\n")

	if output.Tooltip != want {
		t.Fatalf("unexpected tooltip:\n%s\nwant:\n%s", output.Tooltip, want)
	}
}

func TestUsageBarAndPercentageNormalizeValues(t *testing.T) {
	tests := []struct {
		name       string
		percentage float64
		bar        string
		whole      float64
	}{
		{name: "zero", percentage: 0, bar: "--------------------", whole: 0},
		{name: "low", percentage: 2, bar: "#-------------------", whole: 2},
		{name: "fraction", percentage: 24.4, bar: "#####---------------", whole: 24},
		{name: "rounded", percentage: 24.5, bar: "#####---------------", whole: 25},
		{name: "high", percentage: 53, bar: "###########---------", whole: 53},
		{name: "clamped", percentage: 125, bar: "####################", whole: 100},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := usageBar(test.percentage); got != test.bar {
				t.Fatalf("bar = %q, want %q", got, test.bar)
			}

			if got := wholePercentage(test.percentage); got != test.whole {
				t.Fatalf("whole percentage = %v, want %v", got, test.whole)
			}
		})
	}
}

func TestTooltipUsesResetFallbacks(t *testing.T) {
	now := time.Date(2026, time.July, 13, 8, 0, 0, 0, time.UTC)
	output := Build([]usage.Snapshot{{Provider: "codex", Windows: []usage.Window{
		{Label: "5h", UsedPct: 12, ResetsAt: now.Add(-time.Second)},
		{Label: "weekly", UsedPct: 8},
	}}}, usage.View{}, now)

	if !strings.Contains(output.Tooltip, "Resetting now") {
		t.Fatalf("elapsed reset fallback missing: %s", output.Tooltip)
	}

	if !strings.Contains(output.Tooltip, "Reset time unavailable") {
		t.Fatalf("unknown reset fallback missing: %s", output.Tooltip)
	}
}

func TestJSONMaintainsWaybarContract(t *testing.T) {
	now := time.Now()
	data, err := JSON([]usage.Snapshot{{Provider: "codex", FetchedAt: now, Windows: []usage.Window{{Label: "weekly", UsedPct: 12}}}}, usage.View{}, now)
	if err != nil {
		t.Fatal(err)
	}

	var output Output
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}

	if output.Text != Icon || output.Tooltip == "" || output.Class != "ok" || output.Percentage != 12 {
		t.Fatalf("unexpected Waybar output: %#v", output)
	}
}

var errFixture = fixtureError("fixture watcher error")

type fixtureError string

func (e fixtureError) Error() string { return string(e) }
