package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

func newLogger(verbose bool) *slog.Logger {
	if !verbose {
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	opts := &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey && len(groups) == 0 {
				return slog.Attr{}
			}
			return attr
		},
	}
	return slog.New(slog.NewTextHandler(colorWriter{w: os.Stderr}, opts))
}

type colorWriter struct {
	w io.Writer
}

func (cw colorWriter) Write(p []byte) (int, error) {
	colored := logLevelColors.Replace(string(p))
	_, err := cw.w.Write([]byte(colored))
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

var logLevelColors = strings.NewReplacer(
	"level=DEBUG", "level=\x1b[36mDEBUG\x1b[0m",
	"level=INFO", "level=\x1b[32mINFO\x1b[0m",
	"level=WARN", "level=\x1b[33mWARN\x1b[0m",
	"level=ERROR", "level=\x1b[31mERROR\x1b[0m",
)

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "metago: %v\n", err)
	os.Exit(1)
}
