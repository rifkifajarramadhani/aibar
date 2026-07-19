// Package config resolves the filesystem paths the daemon runs against. aibar
// makes no network calls in this milestone, so configuration is just a handful
// of local paths derived from the home and XDG cache directories, with daemon
// flags overriding individual fields.
package config

import (
	"os"
	"path/filepath"
)

// Config holds the resolved runtime paths.
type Config struct {
	CodexRoot         string
	ClaudeCredentials string
	ClaudeProjects    string
	StatePath         string
	CacheDir          string
}

// Load returns the default configuration derived from the current user's home
// and cache directories.
func Load() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}

	return Defaults(home), nil
}

// Defaults builds the configuration rooted at the given home directory. It is
// separated from Load so tests can pin the home directory.
func Defaults(home string) Config {
	cacheDir := os.Getenv("XDG_CACHE_HOME")
	if cacheDir == "" {
		cacheDir = filepath.Join(home, ".cache")
	}

	cacheDir = filepath.Join(cacheDir, "aibar")

	return Config{
		CodexRoot:         filepath.Join(home, ".codex", "sessions"),
		ClaudeCredentials: filepath.Join(home, ".claude", ".credentials.json"),
		ClaudeProjects:    filepath.Join(home, ".claude", "projects"),
		StatePath:         filepath.Join(cacheDir, "state.json"),
		CacheDir:          cacheDir,
	}
}
