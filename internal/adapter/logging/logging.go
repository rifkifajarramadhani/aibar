// Package logging builds the structured logger the daemon writes diagnostics
// to. Logs go to stderr so stdout stays reserved for the Waybar JSON line.
package logging

import (
	"log/slog"
	"os"
)

// New returns a text slog.Logger writing to stderr.
func New() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
}
