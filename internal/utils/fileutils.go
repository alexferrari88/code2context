package utils

import (
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	// fileSizeRegex matches numbers followed by optional K, M, G, T and optional B. Case-insensitive.
	fileSizeRegex = regexp.MustCompile(`(?i)^(\d+(?:\.\d+)?)\s*([KMGT])?B?$`)
	// fileSizeRegexOnlyDigits matches if the string is only digits (for plain bytes).
	fileSizeRegexOnlyDigits = regexp.MustCompile(`^(\d+)$`)
)

const (
	_        = iota // ignore first value by assigning to blank identifier
	KB int64 = 1 << (10 * iota)
	MB
	GB
	TB
)

// ParseFileSize converts a human-readable file size string (e.g., "1MB", "500KB", "1024") to bytes.
// Supports K, M, G, T units (case-insensitive), with an optional 'B'.
// Also supports plain numbers for bytes. Allows fractional inputs like "0.5MB".
func ParseFileSize(sizeStr string) (int64, error) {
	sizeStr = strings.TrimSpace(sizeStr)
	if sizeStr == "" {
		return 0, errors.New("file size string is empty")
	}

	// Try matching with units first
	matches := fileSizeRegex.FindStringSubmatch(sizeStr)
	if len(matches) == 3 { // matches[0] is full string, [1] is value, [2] is unit (K,M,G,T) or empty
		valueStr := matches[1]
		unit := strings.ToUpper(matches[2])

		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid numeric value '%s' in size string: %w", valueStr, err)
		}

		var multiplier int64 = 1 // Default for plain bytes if unit is empty (but regex forces K,M,G,T if present)
		switch unit {
		case "K":
			multiplier = KB
		case "M":
			multiplier = MB
		case "G":
			multiplier = GB
		case "T":
			multiplier = TB
		case "": // Matched by (\d+(?:\.\d+)?)B? part, effectively just bytes
			// This case might be hit if e.g. "1024B" and unit part is empty.
			// Or if just "1024".
			// However, the regex structure `([KMGT])?B?` means unit will be K,M,G,T or empty.
			// If unit is empty, it means no K,M,G,T prefix. e.g. "1024" or "1024B"
			multiplier = 1
		default:
			// Should not happen if regex is correct, as ([KMGT])? captures only those or empty.
			return 0, fmt.Errorf("unknown size unit prefix '%s' (from '%s'). Supported: K, M, G, T", unit, sizeStr)
		}
		// Convert float value * multiplier to int64.
		// Be careful with float precision for very large numbers, but for typical file sizes it's fine.
		return int64(value * float64(multiplier)), nil
	}

	// If no unit match, try parsing as plain bytes (digits only)
	digitMatches := fileSizeRegexOnlyDigits.FindStringSubmatch(sizeStr)
	if len(digitMatches) == 2 { // digitMatches[0] is full, [1] is the number
		val, err := strconv.ParseInt(digitMatches[1], 10, 64)
		if err == nil {
			return val, nil
		}
	}

	return 0, fmt.Errorf("invalid file size format: '%s'. Expected format like '1024', '500KB', '0.5MB', '1GB'", sizeStr)
}

// FormatBytes converts bytes to a human-readable string (e.g., 1.5 MiB).
func FormatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	// Use KiB, MiB, GiB for base-2 units
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// DummyDirEntry is a helper for creating fs.DirEntry for testing or specific scenarios
type DummyDirEntry struct {
	name  string
	isDir bool
	typ   fs.FileMode // Only type bits
	info  fs.FileInfo // Full FileInfo
}

// NewDummyDirEntry creates a synthetic fs.DirEntry.
// Mode should be the full os.FileMode.
func NewDummyDirEntry(name string, size int64, mode fs.FileMode, modTime time.Time) fs.DirEntry {
	isDir := mode.IsDir()
	if modTime.IsZero() {
		modTime = time.Now() // Default to now if not specified
	}
	return &DummyDirEntry{
		name:  name,
		isDir: isDir,
		typ:   mode.Type(), // fs.FileMode.Type() extracts type bits (e.g., ModeDir, ModeSymlink)
		info: &dummyFileInfo{
			name:    name,
			size:    size,
			mode:    mode, // Store full mode for FileInfo
			modTime: modTime,
			isDir:   isDir,
		},
	}
}
func (d *DummyDirEntry) Name() string               { return d.name }
func (d *DummyDirEntry) IsDir() bool                { return d.isDir }
func (d *DummyDirEntry) Type() fs.FileMode          { return d.typ }
func (d *DummyDirEntry) Info() (fs.FileInfo, error) { return d.info, nil }

type dummyFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode // Full mode
	modTime time.Time
	isDir   bool
	sys     interface{} // Underlying data source (can be nil)
}

func (fi *dummyFileInfo) Name() string       { return fi.name }
func (fi *dummyFileInfo) Size() int64        { return fi.size }
func (fi *dummyFileInfo) Mode() fs.FileMode  { return fi.mode }
func (fi *dummyFileInfo) ModTime() time.Time { return fi.modTime }
func (fi *dummyFileInfo) IsDir() bool        { return fi.isDir }
func (fi *dummyFileInfo) Sys() interface{}   { return fi.sys }
