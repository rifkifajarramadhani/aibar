package daemon

import (
	"context"
	"time"

	"github.com/overhaul/aibar/internal/usage"
)

// Control-plane commands the daemon acts on. They double as the CLI subcommand
// names and the wire protocol the control adapter validates.
const (
	Refresh      = "refresh"
	NextProvider = "next-provider"
	PrevProvider = "prev-provider"
	CycleWindow  = "cycle-window"
)

// Commands lists every valid control command in CLI order.
func Commands() []string {
	return []string{Refresh, NextProvider, PrevProvider, CycleWindow}
}

// ValidCommand reports whether command is a recognized control command.
func ValidCommand(command string) bool {
	switch command {
	case Refresh, NextProvider, PrevProvider, CycleWindow:
		return true
	default:
		return false
	}
}

// Renderer turns the merged usage state into a Waybar line and advances the
// view. It is a port so the daemon never imports the presentation adapter.
type Renderer interface {
	Render(snapshots []usage.Snapshot, view usage.View, now time.Time) ([]byte, error)
	NavigateProvider(snapshots []usage.Snapshot, view usage.View, direction int) usage.View
	CycleWindow(view usage.View) usage.View
}

// ControlServer is the inbound control plane. The daemon runs it and closes it;
// the adapter owns the Unix socket, single-instance guard, and runtime files.
type ControlServer interface {
	Run(ctx context.Context) error
	Close() error
}
