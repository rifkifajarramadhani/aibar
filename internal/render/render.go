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
	visible, usable := snapshotsForRender(snapshots)
	selected, status := selectedSnapshots(visible, usable, view.PinnedProvider)

	if len(selected) == 0 {
		output := Output{Text: Icon, Tooltip: "No usage data available yet.", Class: "stale"}
		if len(visible) > 0 {
			output.Tooltip = tooltip(visible, now)
			output.Class = classForErrors(status)
		}

		return output
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
	if hasAuthError(status) {
		class += " auth-error"
	}

	if hasError(status) {
		class += " stale"
	}

	percentage := math.Round(chosenWindow.UsedPct)

	return Output{
		Text:       Icon,
		Tooltip:    tooltip(visible, now),
		Class:      class,
		Percentage: percentage,
	}
}

func snapshotsForRender(snapshots []model.Snapshot) ([]model.Snapshot, []model.Snapshot) {
	visible := make([]model.Snapshot, 0, len(snapshots))
	usable := make([]model.Snapshot, 0, len(snapshots))

	for _, snapshot := range snapshots {
		if len(snapshot.Windows) > 0 || snapshot.Err != nil {
			visible = append(visible, snapshot)
		}

		if len(snapshot.Windows) > 0 {
			usable = append(usable, snapshot)
		}
	}

	sort.Slice(visible, func(i, j int) bool { return visible[i].Provider < visible[j].Provider })
	sort.Slice(usable, func(i, j int) bool { return usable[i].Provider < usable[j].Provider })

	return visible, usable
}

func selectedSnapshots(visible, usable []model.Snapshot, pinnedProvider string) ([]model.Snapshot, []model.Snapshot) {
	if pinnedProvider == "" {
		return usable, visible
	}

	for _, snapshot := range visible {
		if snapshot.Provider != pinnedProvider {
			continue
		}

		if len(snapshot.Windows) == 0 {
			return nil, []model.Snapshot{snapshot}
		}

		return []model.Snapshot{snapshot}, []model.Snapshot{snapshot}
	}

	return usable, visible
}

// NavigateProvider moves the current view through visible providers. An empty
// PinnedProvider represents aggregate mode and acts as the boundary before
// the first and after the last provider.
func NavigateProvider(snapshots []model.Snapshot, view View, direction int) View {
	if direction == 0 {
		return view
	}

	visible, _ := snapshotsForRender(snapshots)
	providers := make([]string, 0, len(visible))
	for _, snapshot := range visible {
		providers = append(providers, snapshot.Provider)
	}

	view.WindowIndex = 0
	if len(providers) == 0 {
		view.PinnedProvider = ""
		return view
	}

	current := -1
	if view.PinnedProvider != "" {
		for index, provider := range providers {
			if provider == view.PinnedProvider {
				current = index
				break
			}
		}
	}

	if current == -1 {
		if direction > 0 {
			view.PinnedProvider = providers[0]
		} else {
			view.PinnedProvider = providers[len(providers)-1]
		}

		return view
	}

	next := current
	if direction > 0 {
		next++
	} else {
		next--
	}

	if next < 0 || next >= len(providers) {
		view.PinnedProvider = ""
		return view
	}

	view.PinnedProvider = providers[next]
	return view
}

// CycleWindow advances the explicit window selection while preserving the
// automatic most-constrained selection at index zero.
func CycleWindow(view View) View {
	view.WindowIndex++
	return view
}

func classForErrors(snapshots []model.Snapshot) string {
	class := "stale"
	if hasAuthError(snapshots) {
		class += " auth-error"
	}

	return class
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
				if chosenSnapshot.Provider == "" || moreConstrained(snapshot, window, chosenSnapshot, chosen) {
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

func moreConstrained(snapshot model.Snapshot, window model.Window, chosenSnapshot model.Snapshot, chosen model.Window) bool {
	if window.UsedPct != chosen.UsedPct {
		return window.UsedPct > chosen.UsedPct
	}

	if snapshot.Provider != chosenSnapshot.Provider {
		return snapshot.Provider < chosenSnapshot.Provider
	}

	return window.Label < chosen.Label
}

const usageBarWidth = 20

func tooltip(snapshots []model.Snapshot, now time.Time) string {
	lines := make([]string, 0, len(snapshots)*6)

	for index, snapshot := range snapshots {
		if index > 0 {
			lines = append(lines, "")
		}

		lines = append(lines, providerLabel(snapshot.Provider))

		windows := append([]model.Window(nil), snapshot.Windows...)
		sort.Slice(windows, func(i, j int) bool { return windows[i].Label < windows[j].Label })

		for _, window := range windows {
			lines = append(lines,
				"• "+usageLabel(window.Label)+":",
				fmt.Sprintf("  [%s]  %3.0f%%", usageBar(window.UsedPct), wholePercentage(window.UsedPct)),
				"  "+resetText(window.ResetsAt, now),
			)
		}

		if snapshot.Err != nil {
			status := "stale"
			if model.ErrorKindOf(snapshot.Err) == model.ErrorAuth {
				status = "auth-error"
			}

			lines = append(lines, "  Status: "+status+" — "+snapshot.Err.Error())
		}
	}

	return strings.Join(lines, "\n")
}

func usageLabel(label string) string {
	switch label {
	case "5h":
		return "Rolling Usage"
	case "weekly":
		return "Weekly Usage"
	default:
		return label + " Usage"
	}
}

func usageBar(pct float64) string {
	pct = normalizedPercentage(pct)
	filled := int(math.Ceil(pct / 100 * usageBarWidth))

	return strings.Repeat("#", filled) + strings.Repeat("-", usageBarWidth-filled)
}

func wholePercentage(pct float64) float64 {
	return math.Round(normalizedPercentage(pct))
}

func normalizedPercentage(pct float64) float64 {
	if math.IsNaN(pct) || math.IsInf(pct, 0) || pct <= 0 {
		return 0
	}

	if pct >= 100 {
		return 100
	}

	return pct
}

func resetText(reset, now time.Time) string {
	remaining := countdown(reset, now)

	switch remaining {
	case "unknown":
		return "Reset time unavailable"
	case "now":
		return "Resetting now"
	default:
		return "Resets " + remaining
	}
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

func hasAuthError(snapshots []model.Snapshot) bool {
	for _, snapshot := range snapshots {
		if model.ErrorKindOf(snapshot.Err) == model.ErrorAuth {
			return true
		}
	}

	return false
}

func providerLabel(provider string) string {
	switch provider {
	case "codex":
		return "Codex"
	case "claude":
		return "Claude"
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
