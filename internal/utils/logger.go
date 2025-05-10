package utils

import (
	"log/slog"
	"os"
)

var globalLogger *slog.Logger

// InitLogger initializes or re-initializes the global slog logger.
func InitLogger(verbose bool) {
	var level slog.LevelVar // Use LevelVar for dynamic level setting if needed later
	if verbose {
		level.Set(slog.LevelDebug)
	} else {
		level.Set(slog.LevelInfo)
	}

	opts := &slog.HandlerOptions{
		Level: level.Level(), // Get current level from LevelVar
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				// More concise time format or remove if too noisy for CLI
				// return slog.String(slog.TimeKey, a.Value.Time().Format(time.StampMilli))
				return slog.Attr{} // Remove time for cleaner CLI output
			}
			// Example: Optionally shorten source file paths if they are logged
			// if a.Key == slog.SourceKey {
			// 	source, _ := a.Value.Any().(*slog.Source)
			// 	if source != nil {
			// 		source.File = filepath.Base(source.File)
			// 	}
			// }
			return a
		},
		// AddSource: verbose, // Optionally add source file and line number if verbose
	}

	// Using os.Stderr for all logs is common for CLI tools.
	// Info and Debug could go to Stdout, Warn/Error to Stderr if desired,
	// but that requires a more complex handler setup.
	handler := slog.NewTextHandler(os.Stderr, opts)
	globalLogger = slog.New(handler)
	slog.SetDefault(globalLogger)
}

// GetLogger returns the configured global logger.
// Should generally not be needed if slog.Default() is used throughout.
func GetLogger() *slog.Logger {
	if globalLogger == nil {
		// Fallback if InitLogger was somehow not called.
		// This ensures slog.Default() is always set, but Init should be preferred.
		InitLogger(false) // Default to non-verbose
	}
	return globalLogger
}
