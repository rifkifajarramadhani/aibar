package render

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/overhaul/aibar/internal/model"
)

type Output struct {
	Text       string  `json:"text"`
	Tooltip    string  `json:"tooltip"`
	Class      string  `json:"class"`
	Percentage float64 `json:"percentage"`
}

type View struct {
	PinnedProvider string
	WindowIndex    int
}

const Icon = "󰚩"

func JSON(snapshots []model.Snapshot, view View, now time.Time) ([]byte, error) {
	output := Build(snapshots, view, now)
	return json.Marshal(output)
}

func Build(snapshots []model.Snapshot, view View, now time.Time) Output {
	usable := make([]model.Snapshot, 0, len(snapshots))

	for _, snapshot := range snapshots {
		if len(snapshot.Windows) > 0 {
			usable = append(usable, snapshot)
		}
	}

	sort.Slice(usable, func(i, j int) bool { return usable[i].Provider < usable[j].Provider })

	if len(usable) == 0 {
		return Output{
			Text:    Icon,
			Tooltip: "No usage data available yet.",
			Class:   "stale",
		}
	}

	selected := usable
	if view.PinnedProvider != "" {
		selected = selected[:0]

		for _, snapshot := range usable {
			if snapshot.Provider == view.PinnedProvider {
				selected = append(selected, snapshot)
			}
		}

		if len(selected) == 0 {
			selected = usable
		}
	}

	chosenSnapshot, chosenWindow := chooseWindow(selected, view.WindowIndex)
	if chosenSnapshot.Provider == "" {
		return Output{Text: Icon, Tooltip: "No usage data available yet.", Class: "stale"}
	}

	maxPct := 0.0

	for _, snapshot := range selected {
		for _, window := range snapshot.Windows {
			maxPct = math.Max(maxPct, window.UsedPct)
		}
	}

	class := classFor(maxPct)
	if hasError(selected) {
		class += " stale"
	}

	percentage := math.Round(chosenWindow.UsedPct)

	return Output{
		Text:       Icon,
		Tooltip:    tooltip(selected, chosenSnapshot, chosenWindow, now),
		Class:      class,
		Percentage: percentage,
	}
}

func chooseWindow(snapshots []model.Snapshot, index int) (model.Snapshot, model.Window) {
	if len(snapshots) == 0 {
		return model.Snapshot{}, model.Window{}
	}
	// The default is the most-constrained window. Cycling selects from the
	// pinned provider's available windows in stable label order.
	if index <= 0 {
		var chosen model.Window

		var chosenSnapshot model.Snapshot

		for _, snapshot := range snapshots {
			for _, window := range snapshot.Windows {
				if chosenSnapshot.Provider == "" || window.UsedPct > chosen.UsedPct {
					chosenSnapshot, chosen = snapshot, window
				}
			}
		}

		return chosenSnapshot, chosen
	}

	all := make([]struct {
		snapshot model.Snapshot
		window   model.Window
	}, 0)

	for _, snapshot := range snapshots {
		windows := append([]model.Window(nil), snapshot.Windows...)
		sort.Slice(windows, func(i, j int) bool { return windows[i].Label < windows[j].Label })

		for _, window := range windows {
			all = append(all, struct {
				snapshot model.Snapshot
				window   model.Window
			}{snapshot: snapshot, window: window})
		}
	}

	if len(all) == 0 {
		return model.Snapshot{}, model.Window{}
	}

	choice := all[(index-1)%len(all)]

	return choice.snapshot, choice.window
}

func tooltip(snapshots []model.Snapshot, chosen model.Snapshot, chosenWindow model.Window, now time.Time) string {
	lines := []string{"aibar", ""}

	for _, snapshot := range snapshots {
		windows := append([]model.Window(nil), snapshot.Windows...)
		sort.Slice(windows, func(i, j int) bool { return windows[i].Label < windows[j].Label })

		lines = append(lines, providerLabel(snapshot.Provider)+":")

		for _, window := range windows {
			marker := " "
			if snapshot.Provider == chosen.Provider && window.Label == chosenWindow.Label {
				marker = "›"
			}

			lines = append(lines, fmt.Sprintf("%s %s  %5.1f%%  resets %s  updated %s", marker, window.Label, window.UsedPct, countdown(window.ResetsAt, now), age(snapshot.FetchedAt, now)))
		}

		if snapshot.Err != nil {
			lines = append(lines, "  status: stale ("+snapshot.Err.Error()+")")
		}
	}

	return strings.Join(lines, "\n")
}

func classFor(pct float64) string {
	switch {
	case pct >= 90:
		return "critical"
	case pct >= 75:
		return "warning"
	default:
		return "ok"
	}
}

func hasError(snapshots []model.Snapshot) bool {
	for _, snapshot := range snapshots {
		if snapshot.Err != nil {
			return true
		}
	}

	return false
}

func providerLabel(provider string) string {
	if provider == "codex" {
		return "Codex"
	}

	return provider
}

func countdown(reset, now time.Time) string {
	if reset.IsZero() {
		return "unknown"
	}

	d := reset.Sub(now)
	if d <= 0 {
		return "now"
	}

	minutes := int(math.Ceil(d.Minutes()))
	if minutes < 60 {
		return fmt.Sprintf("in %dm", minutes)
	}

	hours := minutes / 60
	remainingMinutes := minutes % 60

	if hours < 24 {
		return fmt.Sprintf("in %dh %02dm", hours, remainingMinutes)
	}

	return fmt.Sprintf("in %dd %02dh", hours/24, hours%24)
}

func age(fetched, now time.Time) string {
	if fetched.IsZero() {
		return "unknown"
	}

	minutes := int(now.Sub(fetched).Minutes())
	if minutes < 0 {
		minutes = 0
	}

	return fmt.Sprintf("%dm ago", minutes)
}
