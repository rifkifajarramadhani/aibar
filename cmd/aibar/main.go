package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/overhaul/aibar/internal/adapter/control"
	"github.com/overhaul/aibar/internal/adapter/logging"
	"github.com/overhaul/aibar/internal/bootstrap"
	"github.com/overhaul/aibar/internal/config"
	"github.com/overhaul/aibar/internal/daemon"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		fatal(err)
	}

	switch os.Args[1] {
	case "daemon":
		if err := runDaemon(cfg); err != nil {
			fatal(err)
		}
	case daemon.Refresh, daemon.NextProvider, daemon.PrevProvider, daemon.CycleWindow:
		if err := control.Send(cfg.CacheDir, os.Args[1]); err != nil {
			fatal(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func runDaemon(cfg config.Config) error {
	fs := flag.NewFlagSet("daemon", flag.ExitOnError)
	codexRoot := fs.String("codex-root", cfg.CodexRoot, "Codex sessions directory")
	claudeCredentials := fs.String("claude-credentials", cfg.ClaudeCredentials, "Claude Code credentials file")
	claudeProjects := fs.String("claude-projects", cfg.ClaudeProjects, "Claude Code projects directory")
	statePath := fs.String("state", cfg.StatePath, "last-good state file")
	cacheDir := fs.String("cache-dir", cfg.CacheDir, "aibar runtime directory")
	_ = fs.Parse(os.Args[2:])

	cfg.CodexRoot = *codexRoot
	cfg.ClaudeCredentials = *claudeCredentials
	cfg.ClaudeProjects = *claudeProjects
	cfg.StatePath = *statePath
	cfg.CacheDir = *cacheDir

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := logging.New()

	app, err := bootstrap.WireDaemon(cfg, os.Stdout, logger, time.Now)
	if err != nil {
		return err
	}

	return app.Run(ctx)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: aibar {daemon|"+strings.Join(daemon.Commands(), "|")+"}")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "aibar:", err)
	os.Exit(1)
}
