package codex

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/overhaul/aibar/internal/model"
)

const (
	fiveHourMinutes = 5 * 60
	weeklyMinutes   = 7 * 24 * 60
)

var (
	ErrNoRollout = errors.New("no Codex rollout file found")
	ErrNoWindows = errors.New("Codex rollout has no recognized rate-limit windows")
)

type envelope struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

type tokenCountPayload struct {
	Type       string     `json:"type"`
	RateLimits rateLimits `json:"rate_limits"`
}

type rateLimits struct {
	Primary   *rateLimit `json:"primary"`
	Secondary *rateLimit `json:"secondary"`
}

type rateLimit struct {
	UsedPercent     float64 `json:"used_percent"`
	WindowMinutes   int     `json:"window_minutes"`
	ResetsAt        int64   `json:"resets_at"`
	ResetsInSeconds int64   `json:"resets_in_seconds"`
}

func ParseFile(path string, now time.Time) (model.Snapshot, error) {
	file, err := os.Open(path)
	if err != nil {
		return model.Snapshot{}, err
	}
	defer file.Close()

	snapshot, err := Parse(file, now)
	if err != nil {
		return model.Snapshot{}, fmt.Errorf("parse %s: %w", path, err)
	}
	snapshot.Provider = "codex"
	snapshot.Source = model.SourceLocal
	snapshot.FetchedAt = now
	return snapshot, nil
}

func Parse(reader io.Reader, now time.Time) (model.Snapshot, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	var latest *tokenCountPayload
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var event envelope
		if err := json.Unmarshal(line, &event); err != nil {
			// Rollout files can contain partially written lines while Codex is
			// appending. Ignore those lines and keep the last complete event.
			continue
		}
		if event.Type != "event_msg" {
			continue
		}
		var payload tokenCountPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil || payload.Type != "token_count" {
			continue
		}
		if len(windowsFrom(payload.RateLimits, now)) > 0 {
			copy := payload
			latest = &copy
		}
	}
	if err := scanner.Err(); err != nil {
		return model.Snapshot{}, err
	}
	if latest == nil {
		return model.Snapshot{}, ErrNoWindows
	}
	return model.Snapshot{Windows: windowsFrom(latest.RateLimits, now)}, nil
}

func FindNewest(root string) (string, error) {
	var newestPath string
	var newestMod time.Time
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return filepath.SkipDir
			}
			return err
		}
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "rollout-") || !strings.HasSuffix(entry.Name(), ".jsonl") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if newestPath == "" || info.ModTime().After(newestMod) {
			newestPath = path
			newestMod = info.ModTime()
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if newestPath == "" {
		return "", ErrNoRollout
	}
	return newestPath, nil
}

func Scan(root string, now time.Time) (model.Snapshot, error) {
	path, err := FindNewest(root)
	if err != nil {
		return model.Snapshot{Provider: "codex", Source: model.SourceLocal, FetchedAt: now, Err: err}, err
	}
	snapshot, err := ParseFile(path, now)
	if err != nil {
		return model.Snapshot{Provider: "codex", Source: model.SourceLocal, FetchedAt: now, Err: err}, err
	}
	return snapshot, nil
}

func windowsFrom(limits rateLimits, now time.Time) []model.Window {
	windows := make([]model.Window, 0, 2)
	for _, limit := range []*rateLimit{limits.Primary, limits.Secondary} {
		if limit == nil {
			continue
		}
		label, ok := labelForMinutes(limit.WindowMinutes)
		if !ok {
			continue
		}
		resetsAt := time.Time{}
		if limit.ResetsAt > 0 {
			resetsAt = time.Unix(limit.ResetsAt, 0)
		} else if limit.ResetsInSeconds > 0 {
			resetsAt = now.Add(time.Duration(limit.ResetsInSeconds) * time.Second)
		}
		used := limit.UsedPercent
		if used < 0 {
			used = 0
		}
		if used > 100 {
			used = 100
		}
		windows = append(windows, model.Window{Label: label, UsedPct: used, ResetsAt: resetsAt, WindowMinutes: limit.WindowMinutes})
	}
	return dedupeWindows(windows)
}

func labelForMinutes(minutes int) (string, bool) {
	switch minutes {
	case fiveHourMinutes:
		return "5h", true
	case weeklyMinutes:
		return "weekly", true
	default:
		return "", false
	}
}

func dedupeWindows(windows []model.Window) []model.Window {
	seen := make(map[string]bool, len(windows))
	result := windows[:0]
	for _, window := range windows {
		if seen[window.Label] {
			continue
		}
		seen[window.Label] = true
		result = append(result, window)
	}
	return result
}
