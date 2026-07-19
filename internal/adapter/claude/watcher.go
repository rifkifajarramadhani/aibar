package claude

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/overhaul/aibar/internal/usage"
)

type fileUsageState struct {
	Totals   UsageTotals
	HasUsage bool
}

func (p *Provider) Watch(ctx context.Context, out chan<- usage.Snapshot) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	defer func() { _ = watcher.Close() }()

	watchedDirs := make(map[string]bool)
	if err := addDirectoryTree(watcher, watchedDirs, p.config.ProjectsRoot); err != nil {
		return err
	}

	if parent := credentialsParent(p.config.CredentialsPath); parent != "" {
		if err := addDirectoryTree(watcher, watchedDirs, parent); err != nil {
			return err
		}
	}

	usageStates := make(map[string]fileUsageState)
	if err := initializeUsageStates(p.config.ProjectsRoot, usageStates); err != nil {
		return err
	}

	dirtyPaths := make(map[string]struct{})
	var debounceTimer *time.Timer
	var debounceC <-chan time.Time

	var pollTimer *time.Timer
	var pollC <-chan time.Time
	nextAllowed := time.Time{}
	pending := false

	resetDebounce := func() {
		if debounceTimer == nil {
			debounceTimer = time.NewTimer(p.config.Debounce)
		} else {
			if !debounceTimer.Stop() {
				select {
				case <-debounceTimer.C:
				default:
				}
			}

			debounceTimer.Reset(p.config.Debounce)
		}

		debounceC = debounceTimer.C
	}

	resetPoll := func(at time.Time) {
		delay := at.Sub(p.config.Now())
		if delay < 0 {
			delay = 0
		}

		if pollTimer == nil {
			pollTimer = time.NewTimer(delay)
		} else {
			if !pollTimer.Stop() {
				select {
				case <-pollTimer.C:
				default:
				}
			}

			pollTimer.Reset(delay)
		}

		pollC = pollTimer.C
	}

	attemptFetch := func() {
		snapshot, fetchErr := p.Fetch(ctx)
		if errors.Is(fetchErr, ErrNotConfigured) {
			pending = false
			return
		}

		if fetchErr != nil {
			emit(ctx, out, snapshot)
		}

		if fetchErr == nil {
			emit(ctx, out, snapshot)
		}

		now := p.config.Now()
		nextAllowed = now.Add(p.nextDelay(fetchErr))
		resetPoll(nextAllowed)
		pending = false
	}

	if pathExists(p.config.CredentialsPath) {
		attemptFetch()
	}

	defer func() {
		if debounceTimer != nil {
			debounceTimer.Stop()
		}

		if pollTimer != nil {
			pollTimer.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-p.trigger:
			pending = true
			if nextAllowed.IsZero() || !p.config.Now().Before(nextAllowed) {
				attemptFetch()
			}
		case <-pollC:
			if !nextAllowed.IsZero() && p.config.Now().Before(nextAllowed) {
				resetPoll(nextAllowed)
				continue
			}

			attemptFetch()
		case <-debounceC:
			debounceC = nil
			if usageChanged(dirtyPaths, usageStates) {
				pending = true
			}

			dirtyPaths = make(map[string]struct{})
			if pending && (nextAllowed.IsZero() || !p.config.Now().Before(nextAllowed)) {
				attemptFetch()
			}
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			if event.Op&fsnotify.Create != 0 {
				if info, statErr := os.Stat(event.Name); statErr == nil && info.IsDir() {
					if err := addDirectoryTree(watcher, watchedDirs, event.Name); err != nil {
						return err
					}
				}
			}

			if relevantProjectEvent(event, p.config.ProjectsRoot) {
				if event.Op&fsnotify.Remove != 0 || event.Op&fsnotify.Rename != 0 {
					delete(usageStates, event.Name)
				}

				dirtyPaths[event.Name] = struct{}{}
				resetDebounce()
			}

			if relevantCredentialEvent(event, p.config.CredentialsPath) {
				pending = true
				if nextAllowed.IsZero() || !p.config.Now().Before(nextAllowed) {
					attemptFetch()
				}
			}
		case watchErr, ok := <-watcher.Errors:
			if !ok {
				return nil
			}

			return watchErr
		}
	}
}

func initializeUsageStates(root string, states map[string]fileUsageState) error {
	if root == "" {
		return nil
	}

	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		if err != nil {
			return err
		}

		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			return nil
		}

		state, parseErr := readUsageState(path)
		if parseErr == nil {
			states[path] = state
		}

		return nil
	})
}

func usageChanged(paths map[string]struct{}, states map[string]fileUsageState) bool {
	changed := false

	for path := range paths {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			if previous, ok := states[path]; ok && previous.HasUsage {
				changed = true
			}

			delete(states, path)
			continue
		}

		current, err := readUsageState(path)
		if err != nil {
			continue
		}

		previous, ok := states[path]
		if !ok || previous != current {
			if current.HasUsage || previous.HasUsage {
				changed = true
			}
		}

		states[path] = current
	}

	return changed
}

func readUsageState(path string) (fileUsageState, error) {
	file, err := os.Open(path)
	if err != nil {
		return fileUsageState{}, err
	}

	defer func() { _ = file.Close() }()

	totals, err := ParseProjectUsage(file)
	if err != nil {
		return fileUsageState{}, err
	}

	return fileUsageState{Totals: totals, HasUsage: totals.HasUsage()}, nil
}

func addDirectoryTree(watcher *fsnotify.Watcher, watched map[string]bool, root string) error {
	if root == "" || watched[root] {
		return nil
	}

	info, err := os.Stat(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return err
	}

	if !info.IsDir() {
		return errors.New("claude watch path is not a directory")
	}

	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if !entry.IsDir() || watched[path] {
			return nil
		}

		if err := watcher.Add(path); err != nil {
			return err
		}

		watched[path] = true
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func relevantProjectEvent(event fsnotify.Event, root string) bool {
	if root == "" || !strings.HasSuffix(event.Name, ".jsonl") {
		return false
	}

	return withinRoot(event.Name, root)
}

func relevantCredentialEvent(event fsnotify.Event, path string) bool {
	if path == "" {
		return false
	}

	return event.Name == path || filepath.Clean(event.Name) == filepath.Clean(path)
}

func withinRoot(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}

	return true
}
