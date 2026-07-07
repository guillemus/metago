package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
)

func newLogger(verbose bool) *slog.Logger {
	if !verbose {
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	opts := &tint.Options{
		Level:     slog.LevelDebug,
		AddSource: true,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey && len(groups) == 0 {
				return slog.Attr{}
			}
			return attr
		},
		TimeFormat: "",
		NoColor:    false,
	}
	return slog.New(tint.NewHandler(os.Stderr, opts))
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "metago: %v\n", err)
	os.Exit(1)
}
