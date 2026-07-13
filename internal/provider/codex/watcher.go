package codex

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/overhaul/aibar/internal/model"
)

type Watcher struct {
	Root string
	Now  func() time.Time
}

func NewWatcher(root string) *Watcher {
	return &Watcher{Root: root, Now: time.Now}
}

func (w *Watcher) Name() string { return "codex" }

func (w *Watcher) MinInterval() time.Duration { return 0 }

func (w *Watcher) Fetch(context.Context) (model.Snapshot, error) {
	return model.Snapshot{Provider: w.Name(), Source: model.SourceLocal, Err: errors.New("Codex uses its local rollout source")}, errors.New("Codex uses its local rollout source")
}

func (w *Watcher) Watch(ctx context.Context, out chan<- model.Snapshot) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	if err := addDirectoryTree(watcher, w.Root); err != nil {
		return err
	}
	w.emitScan(ctx, out)

	var timer *time.Timer
	var timerC <-chan time.Time
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	scheduleScan := func() {
		if timer == nil {
			timer = time.NewTimer(75 * time.Millisecond)
		} else {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(75 * time.Millisecond)
		}
		timerC = timer.C
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timerC:
			timerC = nil
			w.emitScan(ctx, out)
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&fsnotify.Create != 0 {
				if info, statErr := os.Stat(event.Name); statErr == nil && info.IsDir() {
					_ = addDirectoryTree(watcher, event.Name)
					scheduleScan()
				}
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove|fsnotify.Chmod) != 0 && relevantPath(event.Name, w.Root) {
				scheduleScan()
			}
		case watchErr, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			return watchErr
		}
	}
}

func (w *Watcher) Rescan(ctx context.Context, out chan<- model.Snapshot) {
	w.emitScan(ctx, out)
}

func (w *Watcher) emitScan(ctx context.Context, out chan<- model.Snapshot) {
	now := w.Now()
	snapshot, err := Scan(w.Root, now)
	if err != nil {
		snapshot.Provider = w.Name()
		snapshot.Source = model.SourceLocal
		snapshot.FetchedAt = now
		snapshot.Err = err
	}
	select {
	case out <- snapshot:
	case <-ctx.Done():
	}
}

func addDirectoryTree(watcher *fsnotify.Watcher, root string) error {
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("Codex sessions path is not a directory")
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})
}

func relevantPath(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	// Any event below sessions can indicate a new date directory or a
	// replacement of the newest rollout. The scan is cheap and makes file
	// rotation reliable even when the removed path no longer stats.
	return true
}
