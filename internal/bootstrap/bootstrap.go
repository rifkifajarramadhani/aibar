// Package bootstrap assembles concrete adapters into the daemon application
// service. It owns no process resources: the command layer creates the logger
// and cancellation context and runs the daemon that bootstrap returns.
package bootstrap

import (
	"io"
	"log/slog"
	"time"

	"github.com/overhaul/aibar/internal/adapter/claude"
	"github.com/overhaul/aibar/internal/adapter/codex"
	"github.com/overhaul/aibar/internal/adapter/control"
	"github.com/overhaul/aibar/internal/adapter/statefile"
	"github.com/overhaul/aibar/internal/adapter/waybar"
	"github.com/overhaul/aibar/internal/config"
	"github.com/overhaul/aibar/internal/daemon"
	"github.com/overhaul/aibar/internal/usage"
)

// WireDaemon builds the daemon from configuration, binding the control socket
// as a side effect. A live socket returns the "already running" error.
func WireDaemon(cfg config.Config, out io.Writer, logger *slog.Logger, now func() time.Time) (*daemon.Daemon, error) {
	if now == nil {
		now = time.Now
	}

	archive := statefile.New(cfg.StatePath)
	store := usage.NewStore(archive)

	codexWatcher := codex.NewWatcher(cfg.CodexRoot)
	claudeWatcher := claude.New(claude.Config{
		CredentialsPath: cfg.ClaudeCredentials,
		ProjectsRoot:    cfg.ClaudeProjects,
		Now:             now,
	})

	providers := []usage.Provider{codexWatcher, claudeWatcher}
	refreshables := []usage.Refreshable{codexWatcher, claudeWatcher}

	actions := make(chan string, 8)

	server, err := control.Listen(cfg.CacheDir, actions)
	if err != nil {
		return nil, err
	}

	return daemon.New(daemon.Deps{
		Store:        store,
		Providers:    providers,
		Refreshables: refreshables,
		Renderer:     waybar.New(),
		Control:      server,
		Actions:      actions,
		Output:       out,
		Now:          now,
		Logger:       logger,
	}), nil
}
