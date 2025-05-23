package filefilter

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/alexferrari88/code2context/internal/utils"
	gitignore "github.com/sabhiram/go-gitignore"
)

type FilterConfig struct {
	MaxFileSize                    int64
	UserExcludeDirs                []string
	UserExcludeExts                []string
	UserExcludeGlobs               []string
	SkipAuxFiles                   bool
	DefaultExcludeDirs             []string
	DefaultMediaExts               []string
	DefaultArchiveExts             []string
	DefaultExecExts                []string
	DefaultLockfilePatterns        []string
	DefaultMiscellaneousFileNames  []string
	DefaultMiscellaneousExtensions []string
	DefaultAuxExts                 []string
	FinalOutputFilePath            string // Absolute path to the final output file
}

type FileFilter struct {
	config                 FilterConfig
	basePath               string // Absolute path to the root of processing
	absFinalOutputFilePath string // Store the absolute output file path
}

func NewFileFilter(basePath string, config FilterConfig) (*FileFilter, error) {
	absBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("NewFileFilter: could not get absolute path for base '%s': %w", basePath, err)
	}

	var absOutputFilePath string
	if config.FinalOutputFilePath != "" {
		absOutputFilePath, err = filepath.Abs(config.FinalOutputFilePath)
		if err != nil {
			return nil, fmt.Errorf("NewFileFilter: could not get absolute path for output file '%s': %w", config.FinalOutputFilePath, err)
		}
	}

	return &FileFilter{
		config:                 config,
		basePath:               absBasePath,
		absFinalOutputFilePath: absOutputFilePath,
	}, nil
}

// GetAbsFinalOutputFilePath returns the absolute path of the final output file.
func (ff *FileFilter) GetAbsFinalOutputFilePath() string {
	return ff.absFinalOutputFilePath
}

// IsExcluded checks if a file or directory should be excluded.
// `activeGitIgnores` is a slice of gitignore.GitIgnore objects, ordered from root to most specific.
// The path provided to this function should be absolute.
func (ff *FileFilter) IsExcluded(absPath string, d fs.DirEntry, activeGitIgnores []*gitignore.GitIgnore) (bool, error) {
	// 0. Highest Priority: Never include the output file itself.
	if ff.absFinalOutputFilePath != "" && absPath == ff.absFinalOutputFilePath {
		slog.Debug("Filter: Skipping the output file itself", "path", absPath)
		return true, nil // Or SkipDir if it's a directory, though unlikely for the output file.
	}

	info, err := d.Info()
	if err != nil {
		// Handle broken symlinks gracefully
		if d.Type()&fs.ModeSymlink != 0 && os.IsNotExist(err) {
			slog.Debug("Filter: Skipping broken symbolic link", "path", absPath)
			return true, nil
		}
		slog.Warn("Filter: Failed to get file info", "path", absPath, "error", err)
		return false, fmt.Errorf("filter: could not get file info for '%s': %w", absPath, err)
	}

	relPath, err := filepath.Rel(ff.basePath, absPath)
	if err != nil {
		slog.Warn("Filter: Could not get relative path", "base", ff.basePath, "target", absPath, "error", err)
		relPath = filepath.Base(absPath)
	}
	relPath = filepath.ToSlash(relPath)
	baseName := filepath.Base(absPath)

	// 0b. Symbolic links (always excluded, moved after output file check)
	if info.Mode()&os.ModeSymlink != 0 {
		slog.Debug("Filter: Skipping symbolic link", "path", relPath)
		return true, nil
	}

	// 1. Default and User-defined Directory Name Exclusions
	if info.IsDir() {
		allExcludeDirs := append(ff.config.DefaultExcludeDirs, ff.config.UserExcludeDirs...)
		for _, excludedDirName := range allExcludeDirs {
			if baseName == excludedDirName {
				slog.Debug("Filter: Skipping directory by name", "path", relPath, "rule", excludedDirName)
				return true, filepath.SkipDir
			}
		}
	}

	// 2. Gitignore check
	for i := len(activeGitIgnores) - 1; i >= 0; i-- {
		matcher := activeGitIgnores[i]
		if matcher != nil && matcher.MatchesPath(absPath) {
			slog.Debug("Filter: Path ignored by .gitignore", "path", relPath, "gitignore_at_level", i)
			if info.IsDir() {
				return true, filepath.SkipDir
			}
			return true, nil
		}
	}

	if info.IsDir() {
		return false, nil
	}

	// 3. Max file size
	if ff.config.MaxFileSize > 0 && info.Size() > ff.config.MaxFileSize {
		slog.Info("Filter: Skipping large file",
			"path", relPath,
			"size", utils.FormatBytes(uint64(info.Size())),
			"limit", utils.FormatBytes(uint64(ff.config.MaxFileSize)))
		return true, nil
	}

	fileExt := strings.ToLower(filepath.Ext(absPath))

	// 4. User-defined excluded extensions
	for _, excludedExt := range ff.config.UserExcludeExts {
		if excludedExt != "" && fileExt == excludedExt {
			slog.Debug("Filter: Skipping by user-excluded extension", "path", relPath, "ext", fileExt)
			return true, nil
		}
	}

	// 5. User-defined excluded glob patterns
	for _, pattern := range ff.config.UserExcludeGlobs {
		if pattern == "" {
			continue
		}
		matchedRel, _ := filepath.Match(pattern, relPath)
		if matchedRel {
			slog.Debug("Filter: Skipping by user glob pattern (relative path)", "path", relPath, "pattern", pattern)
			return true, nil
		}
		matchedBase, _ := filepath.Match(pattern, baseName)
		if matchedBase {
			slog.Debug("Filter: Skipping by user glob pattern (basename)", "path", relPath, "pattern", pattern)
			return true, nil
		}
	}

	// 6. Executable check
	if runtime.GOOS != "windows" && (info.Mode()&0111 != 0) {
		slog.Debug("Filter: Skipping executable by POSIX permission", "path", relPath)
		return true, nil
	}
	for _, execExt := range ff.config.DefaultExecExts {
		if fileExt == execExt {
			slog.Debug("Filter: Skipping executable by extension", "path", relPath, "ext", fileExt)
			return true, nil
		}
	}
	if fileExt == "" && runtime.GOOS != "windows" && (info.Mode()&0111 != 0) {
		slog.Debug("Filter: Skipping executable (no extension, POSIX permission)", "path", relPath)
		return true, nil
	}

	// 7. Media file extensions
	for _, mediaExt := range ff.config.DefaultMediaExts {
		if fileExt == mediaExt {
			slog.Debug("Filter: Skipping media file by extension", "path", relPath, "ext", fileExt)
			return true, nil
		}
	}

	// 8. Archive file extensions
	for _, archiveExt := range ff.config.DefaultArchiveExts {
		if fileExt == archiveExt {
			slog.Debug("Filter: Skipping archive file by extension", "path", relPath, "ext", archiveExt)
			return true, nil
		}
	}

	// 9. Lock file patterns
	for _, lockPattern := range ff.config.DefaultLockfilePatterns {
		matched, _ := filepath.Match(lockPattern, baseName)
		if matched {
			slog.Debug("Filter: Skipping lock file", "path", relPath, "pattern", lockPattern)
			return true, nil
		}
	}

	// 9b. Miscellaneous extensions
	for _, miscExt := range ff.config.DefaultMiscellaneousExtensions {
		if fileExt == miscExt {
			slog.Debug("Filter: Skipping miscellaneous file by extension", "path", relPath, "ext", miscExt)
			return true, nil
		}
	}

	// 9c. Miscellaneous file names
	for _, miscName := range ff.config.DefaultMiscellaneousFileNames {
		if baseName == miscName {
			slog.Debug("Filter: Skipping miscellaneous file by name", "path", relPath, "name", miscName)
			return true, nil
		}
	}

	// 10. Skip auxiliary files
	if ff.config.SkipAuxFiles {
		isAux := false
		lowerBaseName := strings.ToLower(baseName)
		for _, auxPattern := range ff.config.DefaultAuxExts {
			if strings.HasPrefix(auxPattern, ".") {
				if fileExt == auxPattern {
					isAux = true
					break
				}
			} else if strings.Contains(auxPattern, "*") || strings.Contains(auxPattern, "?") {
				matched, _ := filepath.Match(auxPattern, baseName)
				if matched {
					isAux = true
					break
				}
			} else {
				if lowerBaseName == strings.ToLower(auxPattern) {
					isAux = true
					break
				}
				if strings.HasPrefix(lowerBaseName, strings.ToLower(auxPattern)) && (auxPattern == "README" || auxPattern == "LICENSE" || auxPattern == "COPYING" || auxPattern == "NOTICE" || auxPattern == "AUTHORS" || auxPattern == "CHANGELOG" || auxPattern == "CONTRIBUTING" || auxPattern == "MANIFEST") {
					isAux = true
					break
				}
			}
		}
		if isAux {
			slog.Debug("Filter: Skipping auxiliary file", "path", relPath, "rule_type", "aux-skip")
			return true, nil
		}
	}

	return false, nil
}
