package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/overhaul/aibar/internal/control"
	"github.com/overhaul/aibar/internal/daemon"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fatal(err)
	}
	defaults := defaultsFor(home)

	switch os.Args[1] {
	case "daemon":
		fs := flag.NewFlagSet("daemon", flag.ExitOnError)
		codexRoot := fs.String("codex-root", defaults.codexRoot, "Codex sessions directory")
		statePath := fs.String("state", defaults.statePath, "last-good state file")
		cacheDir := fs.String("cache-dir", defaults.cacheDir, "aibar runtime directory")
		_ = fs.Parse(os.Args[2:])
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := daemon.Run(ctx, daemon.Config{CodexRoot: *codexRoot, StatePath: *statePath, CacheDir: *cacheDir, Output: os.Stdout}); err != nil {
			fatal(err)
		}
	case "refresh", "next-provider", "prev-provider", "cycle-window":
		if err := control.Send(defaults.cacheDir, os.Args[1]); err != nil {
			fatal(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

type defaultPaths struct {
	codexRoot string
	statePath string
	cacheDir  string
}

func defaultsFor(home string) defaultPaths {
	cacheDir := os.Getenv("XDG_CACHE_HOME")
	if cacheDir == "" {
		cacheDir = filepath.Join(home, ".cache")
	}
	cacheDir = filepath.Join(cacheDir, "aibar")
	return defaultPaths{
		codexRoot: filepath.Join(home, ".codex", "sessions"),
		statePath: filepath.Join(cacheDir, "state.json"),
		cacheDir:  cacheDir,
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: aibar {daemon|refresh|next-provider|prev-provider|cycle-window}")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "aibar:", err)
	os.Exit(1)
}
