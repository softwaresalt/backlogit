package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/softwaresalt/backlogit/internal/cli"
)

func main() {
	initLogger()
	if err := cli.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// initLogger configures the global slog handler from environment variables.
//
// BACKLOGIT_LOG_LEVEL: debug | info | warn | error (default: info)
// BACKLOGIT_LOG_FORMAT: text | json (default: text)
func initLogger() {
	levelStr := strings.ToLower(os.Getenv("BACKLOGIT_LOG_LEVEL"))
	format := strings.ToLower(os.Getenv("BACKLOGIT_LOG_FORMAT"))

	var level slog.Level
	switch levelStr {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(handler))
}
