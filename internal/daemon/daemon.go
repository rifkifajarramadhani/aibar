package daemon

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/overhaul/aibar/internal/control"
	"github.com/overhaul/aibar/internal/model"
	"github.com/overhaul/aibar/internal/provider/codex"
	"github.com/overhaul/aibar/internal/render"
	"github.com/overhaul/aibar/internal/state"
)

type Config struct {
	CodexRoot string
	StatePath string
	CacheDir  string
	Output    io.Writer
	Now       func() time.Time
}

func Run(ctx context.Context, cfg Config) error {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}

	store := state.New()
	if err := store.Load(cfg.StatePath); err != nil {
		fmt.Fprintf(os.Stderr, "aibar: load state: %v\n", err)
	}
	view := render.View{}

	snapshots := make(chan model.Snapshot, 16)
	actions := make(chan string, 8)
	watcher := codex.NewWatcher(cfg.CodexRoot)

	server, err := control.Listen(cfg.CacheDir, actions)
	if err != nil {
		return err
	}
	if err := control.WritePID(control.PIDPath(cfg.CacheDir)); err != nil {
		server.Close()
		return err
	}
	defer control.RemoveRuntimeFiles(cfg.CacheDir)

	watchCtx, stopWatch := context.WithCancel(ctx)
	defer stopWatch()
	go runControlServer(watchCtx, server)
	go runWatcher(watchCtx, watcher, snapshots)

	usr1 := make(chan os.Signal, 1)
	signal.Notify(usr1, syscall.SIGUSR1)
	defer signal.Stop(usr1)

	lastOutput := ""
	emit := func() {
		data, renderErr := render.JSON(store.All(), view, cfg.Now())
		if renderErr != nil {
			return
		}
		line := string(data)
		if line == lastOutput {
			return
		}
		lastOutput = line
		_, _ = fmt.Fprintln(cfg.Output, line)
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
			watcher.Rescan(ctx, snapshots)
		case action := <-actions:
			switch action {
			case control.Refresh:
				watcher.Rescan(ctx, snapshots)
			case control.CycleWindow:
				view.WindowIndex++
				emit()
			case control.NextProvider, control.PrevProvider:
				// Provider pinning becomes meaningful when Claude and Cursor
				// are added. The command remains safe and intentionally no-ops
				// for this Codex-only milestone.
				emit()
			}
		case snapshot := <-snapshots:
			if store.Apply(snapshot) && snapshot.Err == nil && snapshot.Good() {
				if err := store.Save(cfg.StatePath); err != nil {
					fmt.Fprintf(os.Stderr, "aibar: save state: %v\n", err)
				}
			}
			emit()
		}
	}
}

func runControlServer(ctx context.Context, server *control.Server) {
	if err := server.Run(ctx); err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "aibar: control server: %v\n", err)
	}
}

func runWatcher(ctx context.Context, watcher *codex.Watcher, out chan<- model.Snapshot) {
	for ctx.Err() == nil {
		if err := watcher.Watch(ctx, out); err != nil && ctx.Err() == nil {
			now := time.Now()
			select {
			case out <- model.Snapshot{Provider: watcher.Name(), Source: model.SourceLocal, FetchedAt: now, Err: err}:
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
