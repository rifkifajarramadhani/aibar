// Package daemon is the application service: it owns the central select loop
// that merges provider snapshots, renders the Waybar line, and handles control
// commands. It depends only on core ports (Provider, Renderer, ControlServer),
// which the bootstrap layer satisfies with concrete adapters.
package daemon

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/overhaul/aibar/internal/usage"
)

// Deps are the collaborators the bootstrap layer wires into the daemon.
type Deps struct {
	Store        *usage.Store
	Providers    []usage.Provider
	Refreshables []usage.Refreshable
	Renderer     Renderer
	Control      ControlServer
	Actions      <-chan string
	Output       io.Writer
	Now          func() time.Time
	Logger       *slog.Logger
}

// Daemon coordinates the watchers, store, renderer, and control plane.
type Daemon struct {
	store        *usage.Store
	providers    []usage.Provider
	refreshables []usage.Refreshable
	renderer     Renderer
	control      ControlServer
	actions      <-chan string
	output       io.Writer
	now          func() time.Time
	logger       *slog.Logger
}

func New(deps Deps) *Daemon {
	if deps.Now == nil {
		deps.Now = time.Now
	}

	if deps.Output == nil {
		deps.Output = os.Stdout
	}

	if deps.Logger == nil {
		deps.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &Daemon{
		store:        deps.Store,
		providers:    deps.Providers,
		refreshables: deps.Refreshables,
		renderer:     deps.Renderer,
		control:      deps.Control,
		actions:      deps.Actions,
		output:       deps.Output,
		now:          deps.Now,
		logger:       deps.Logger,
	}
}

// Run drives the daemon until ctx is cancelled.
func (d *Daemon) Run(ctx context.Context) error {
	if err := d.store.Load(); err != nil {
		d.logger.Error("load state", "error", err)
	}

	view := usage.View{}

	snapshots := make(chan usage.Snapshot, 16)

	defer func() { _ = d.control.Close() }()

	watchCtx, stopWatch := context.WithCancel(ctx)
	defer stopWatch()

	go d.runControlServer(watchCtx)

	for _, provider := range d.providers {
		go d.runWatcher(watchCtx, provider, snapshots)
	}

	usr1 := make(chan os.Signal, 1)

	signal.Notify(usr1, syscall.SIGUSR1)
	defer signal.Stop(usr1)

	lastOutput := ""
	emit := func() {
		data, renderErr := d.renderer.Render(d.store.All(), view, d.now())
		if renderErr != nil {
			return
		}

		line := string(data)
		if line == lastOutput {
			return
		}

		lastOutput = line
		_, _ = d.write(line)
	}
	emit()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			emit()
		case <-usr1:
			d.refreshSources(ctx, snapshots)
		case action := <-d.actions:
			switch action {
			case Refresh:
				d.refreshSources(ctx, snapshots)
			case CycleWindow:
				view = d.renderer.CycleWindow(view)

				emit()
			case NextProvider:
				view = d.renderer.NavigateProvider(d.store.All(), view, 1)

				emit()
			case PrevProvider:
				view = d.renderer.NavigateProvider(d.store.All(), view, -1)

				emit()
			}
		case snapshot := <-snapshots:
			if d.store.Apply(snapshot) && snapshot.Err == nil && snapshot.Good() {
				if err := d.store.Save(); err != nil {
					d.logger.Error("save state", "error", err)
				}
			}

			emit()
		}
	}
}

func (d *Daemon) write(line string) (int, error) {
	return io.WriteString(d.output, line+"\n")
}

func (d *Daemon) runControlServer(ctx context.Context) {
	if err := d.control.Run(ctx); err != nil && ctx.Err() == nil {
		d.logger.Error("control server", "error", err)
	}
}

func (d *Daemon) runWatcher(ctx context.Context, watcher usage.Provider, out chan<- usage.Snapshot) {
	for ctx.Err() == nil {
		if err := watcher.Watch(ctx, out); err != nil && ctx.Err() == nil {
			fetchedAt := d.now()
			select {
			case out <- usage.Snapshot{Provider: watcher.Name(), Source: usage.SourceLocal, FetchedAt: fetchedAt, Err: err}:
			case <-ctx.Done():
				return
			}

			timer := time.NewTimer(time.Second)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return
			}

			continue
		}

		return
	}
}

func (d *Daemon) refreshSources(ctx context.Context, out chan<- usage.Snapshot) {
	for _, source := range d.refreshables {
		source.Refresh(ctx, out)
	}
}
