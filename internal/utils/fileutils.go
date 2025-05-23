package utils

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	fileSizeRegex         = regexp.MustCompile(`(?i)^(\d+(?:\.\d+)?)\s*([KMGT])?B?$`)
	fileSizeRegexOnlyDigits = regexp.MustCompile(`^(\d+)$`)
)

const (
	_        = iota
	KB int64 = 1 << (10 * iota)
	MB
	GB
	TB
)

func ParseFileSize(sizeStr string) (int64, error) {
	sizeStr = strings.TrimSpace(sizeStr)
	if sizeStr == "" {
		return 0, errors.New("file size string is empty")
	}

	// Priority 1: Try to parse as a plain integer (bytes)
	// This handles large integers precisely and catches their overflows correctly via strconv.ParseInt.
	if digitMatches := fileSizeRegexOnlyDigits.FindStringSubmatch(sizeStr); len(digitMatches) == 2 {
		val, err := strconv.ParseInt(digitMatches[1], 10, 64)
		if err != nil { // err includes strconv.ErrRange for overflow
			return 0, fmt.Errorf("invalid plain byte size '%s': %w", digitMatches[1], err)
		}
		return val, nil
	}

	// Priority 2: Try to parse with units (K, M, G, T) and optional B.
	// This regex also allows for float values like "1.5MB" or "1024.0B".
	matches := fileSizeRegex.FindStringSubmatch(sizeStr)
	if len(matches) == 3 { // matches[0] is full string, [1] is valueStr, [2] is unit char or empty
		valueStr := matches[1]
		unitChar := strings.ToUpper(matches[2]) // K, M, G, T, or empty

		valueFloat, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			// This catches cases like "1.2.3KB" or "abcKB" if the regex somehow passed them to here.
			return 0, fmt.Errorf("invalid numeric value '%s' in size string: %w", valueStr, err)
		}

		if unitChar == "" { // Handles cases like "1024.0B" or "123.45" (if not caught by plain digits)
			// Ensure it's not negative, and does not overflow int64 when converted.
			if valueFloat < 0 {
				return 0, fmt.Errorf("file size cannot be negative: %s", sizeStr)
			}
			if valueFloat >= float64(math.MaxInt64)+0.5 { // Check if float value itself is too large (+0.5 for rounding)
				return 0, fmt.Errorf("file size '%s' (value %f bytes) overflows int64", sizeStr, valueFloat)
			}
			return int64(valueFloat), nil
		}

		var multiplier int64
		switch unitChar {
		case "K":
			multiplier = KB
		case "M":
			multiplier = MB
		case "G":
			multiplier = GB
		case "T":
			multiplier = TB
		default:
			// This should not be reached if the regex is correct, as ([KMGT])? means K,M,G,T or empty.
			// If unitChar was not empty and not K,M,G,T, the regex should not have matched.
			return 0, fmt.Errorf("internal error: unknown size unit prefix '%s' from regex. Input: '%s'", unitChar, sizeStr)
		}
		
		if valueFloat < 0 {
             return 0, fmt.Errorf("numeric part of file size cannot be negative: %s", sizeStr)
        }

		// Check for potential overflow before multiplication for positive values.
		// If valueFloat is already greater than (MaxInt64 / multiplier), it will surely overflow.
		if valueFloat > 0 && float64(multiplier) > 0 && valueFloat > (float64(math.MaxInt64) / float64(multiplier)) {
			return 0, fmt.Errorf("file size '%s' (value %f for unit %s) would overflow int64 due to large numeric part for the unit", sizeStr, valueFloat, unitChar)
		}

		calculatedBytesFloat := valueFloat * float64(multiplier)

		// Final overflow check on the calculated float value.
		// Add 0.5 to handle potential floating point inaccuracies when comparing with MaxInt64.
		// E.g. if calculatedBytesFloat is math.MaxInt64 due to rounding of a slightly larger actual value.
		if calculatedBytesFloat >= float64(math.MaxInt64)+0.5 {
			return 0, fmt.Errorf("file size '%s' results in byte value %f that overflows int64", sizeStr, calculatedBytesFloat)
		}
		
		return int64(calculatedBytesFloat), nil
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
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// DummyDirEntry, NewDummyDirEntry, and dummyFileInfo remain unchanged...
// (Code for DummyDirEntry and dummyFileInfo as provided before)
type DummyDirEntry struct {
	name  string
	isDir bool
	typ   fs.FileMode
	info  fs.FileInfo
}
func NewDummyDirEntry(name string, size int64, mode fs.FileMode, modTime time.Time) fs.DirEntry {
	isDir := mode.IsDir()
	if modTime.IsZero() { modTime = time.Now() }
	return &DummyDirEntry{
		name: name, isDir: isDir, typ: mode.Type(),
		info: &dummyFileInfo{
			name: name, size: size, mode: mode, modTime: modTime, isDir: isDir,
		},
	}
}
func (d *DummyDirEntry) Name() string               { return d.name }
func (d *DummyDirEntry) IsDir() bool                { return d.isDir }
func (d *DummyDirEntry) Type() fs.FileMode          { return d.typ }
func (d *DummyDirEntry) Info() (fs.FileInfo, error) { return d.info, nil }
type dummyFileInfo struct {
	name    string; size    int64; mode    fs.FileMode
	modTime time.Time; isDir   bool; sys     interface{}
}
func (fi *dummyFileInfo) Name() string       { return fi.name }
func (fi *dummyFileInfo) Size() int64        { return fi.size }
func (fi *dummyFileInfo) Mode() fs.FileMode  { return fi.mode }
func (fi *dummyFileInfo) ModTime() time.Time { return fi.modTime }
func (fi *dummyFileInfo) IsDir() bool        { return fi.isDir }
func (fi *dummyFileInfo) Sys() interface{}   { return fi.sys }
