package claude

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseUsageSupportsClaudeWindowsAndResetFormats(t *testing.T) {
	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	input := `{"five_hour":{"utilization":42.5,"resets_at":"2026-07-13T09:00:00Z"},"seven_day":{"used_percentage":18,"resets_at":1784534584},"seven_day_opus":{"utilization":99}}`

	snapshot, err := ParseUsage(strings.NewReader(input), now)
	if err != nil {
		t.Fatal(err)
	}

	if len(snapshot.Windows) != 2 {
		t.Fatalf("got %d windows, want 2", len(snapshot.Windows))
	}

	if snapshot.Windows[0].Label != "5h" || snapshot.Windows[0].UsedPct != 42.5 {
		t.Fatalf("unexpected 5h window: %#v", snapshot.Windows[0])
	}

	if snapshot.Windows[1].Label != "weekly" || snapshot.Windows[1].UsedPct != 18 {
		t.Fatalf("unexpected weekly window: %#v", snapshot.Windows[1])
	}

	if !snapshot.Windows[0].ResetsAt.Equal(time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected 5h reset: %s", snapshot.Windows[0].ResetsAt)
	}
}

func TestParseUsageSupportsNestedUsageAndClampsPercentages(t *testing.T) {
	input := `{"usage":{"five_hour":{"used_percentage":-2},"weekly":{"utilization":120}}}`

	snapshot, err := ParseUsage(strings.NewReader(input), time.Now())
	if err != nil {
		t.Fatal(err)
	}

	if snapshot.Windows[0].UsedPct != 0 || snapshot.Windows[1].UsedPct != 100 {
		t.Fatalf("unexpected clamped windows: %#v", snapshot.Windows)
	}
}

func TestParseUsageRejectsEmptyAndMalformedPayloads(t *testing.T) {
	for _, input := range []string{"{}", `{"five_hour":{}}`, "{"} {
		if _, err := ParseUsage(strings.NewReader(input), time.Now()); err == nil {
			t.Fatalf("input %q unexpectedly parsed", input)
		}
	}
}

func TestParseProjectUsageIgnoresMalformedTrailingLine(t *testing.T) {
	file, err := os.Open("../../../testdata/claude/project-fixture.jsonl")
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = file.Close() }()

	totals, err := ParseProjectUsage(file)
	if err != nil {
		t.Fatal(err)
	}

	want := UsageTotals{InputTokens: 12, OutputTokens: 25, CacheCreationInputTokens: 3, CacheReadInputTokens: 4}
	if totals != want || !totals.HasUsage() {
		t.Fatalf("got %#v, want %#v", totals, want)
	}
}
