package utils

import (
	"log/slog"
	"os"
)

var globalLogger *slog.Logger

// InitLogger initializes or re-initializes the global slog logger.
func InitLogger(verbose bool) {
	var level slog.LevelVar 
	if verbose {
		level.Set(slog.LevelDebug)
	} else {
		level.Set(slog.LevelWarn) // Default to WARN to reduce noise on stderr
	}

	opts := &slog.HandlerOptions{
		Level: level.Level(), 
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{} 
			}
			return a
		},
	}
	handler := slog.NewTextHandler(os.Stderr, opts)
	globalLogger = slog.New(handler)
	slog.SetDefault(globalLogger)
}

// GetLogger returns the configured global logger.
func GetLogger() *slog.Logger {
	if globalLogger == nil {
		InitLogger(false) 
	}
	return globalLogger
}
