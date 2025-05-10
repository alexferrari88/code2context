package processor

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexferrari88/code2context/internal/filefilter"
	"github.com/alexferrari88/code2context/internal/gitutils"
	gitignore "github.com/sabhiram/go-gitignore"
)

type Config struct {
	SourcePath                     string
	GitRef                         string
	OutputFile                     string
	IncludeTree                    bool
	SkipAuxFiles                   bool
	UserExcludeDirs                []string
	UserExcludeExts                []string
	UserExcludeGlobs               []string
	MaxFileSize                    int64
	DefaultExcludeDirs             []string
	DefaultMediaExts               []string
	DefaultArchiveExts             []string
	DefaultExecExts                []string
	DefaultLockfilePatterns        []string
	DefaultMiscellaneousFileNames  []string
	DefaultMiscellaneousExtensions []string
	DefaultAuxExts                 []string
}

type Processor struct {
	config          Config
	filter          *filefilter.FileFilter          // To be initialized after output path is known
	basePath        string                          // Absolute path to the root directory to process
	repoName        string                          // Name of the repo (from URL or local folder name)
	isTempRepo      bool                            // True if basePath is a temporary cloned repository
	tempRepoDir     string                          // The top-level temporary directory created for a clone, to be cleaned up.
	finalOutputFile string                          // Absolute path of the final output file
	gitIgnoreCache  map[string]*gitignore.GitIgnore // Cache for compiled .gitignore files
}

func New(cfg Config) (*Processor, error) {
	p := &Processor{
		config:         cfg,
		gitIgnoreCache: make(map[string]*gitignore.GitIgnore),
	}
	return p, nil
}

func (p *Processor) GetFinalOutputFile() string {
	return p.finalOutputFile
}

// setupInitialPaths determines basePath, repoName, and tempRepoDir if applicable.
// It does NOT initialize the file filter.
func (p *Processor) setupInitialPaths() error {
	if gitutils.IsGitURL(p.config.SourcePath) {
		slog.Info("Input is a Git URL, attempting to clone.", "url", p.config.SourcePath)
		clonedRepoPath, repoName, err := gitutils.CloneRepo(p.config.SourcePath, p.config.GitRef)
		if err != nil {
			return fmt.Errorf("processor: failed to clone repository: %w", err)
		}
		p.basePath = clonedRepoPath                  // This is .../parent_temp_dir/repo_name
		p.tempRepoDir = filepath.Dir(clonedRepoPath) // This is .../parent_temp_dir
		p.repoName = repoName
		p.isTempRepo = true
		slog.Info("Repository cloned", "path", p.basePath)
	} else {
		absPath, err := filepath.Abs(p.config.SourcePath)
		if err != nil {
			return fmt.Errorf("processor: failed to get absolute path for '%s': %w", p.config.SourcePath, err)
		}
		info, err := os.Stat(absPath)
		if err != nil {
			return fmt.Errorf("processor: failed to stat source path '%s': %w", absPath, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("processor: source path '%s' is not a directory", absPath)
		}
		p.basePath = absPath
		p.repoName = filepath.Base(absPath)
		p.isTempRepo = false
		slog.Info("Processing local path", "path", p.basePath)
	}
	return nil
}

// determineOutputFileAndInitFilter determines the final output file path and then initializes the file filter,
// passing the output file path to it for self-exclusion.
func (p *Processor) determineOutputFileAndInitFilter() error {
	var determinedPath string
	if p.config.OutputFile != "" {
		determinedPath = p.config.OutputFile
	} else {
		name := p.repoName
		// Handle cases like "c2c ." where repoName might be "."
		if name == "." || name == "" || name == string(filepath.Separator) {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("processor: failed to get current working directory for default output name: %w", err)
			}
			name = filepath.Base(cwd)
		}
		determinedPath = name + ".txt"
	}

	absOutputFilePath, err := filepath.Abs(determinedPath)
	if err != nil {
		return fmt.Errorf("processor: failed to get absolute path for output file '%s': %w", determinedPath, err)
	}
	p.finalOutputFile = absOutputFilePath // Store the final absolute output path
	slog.Info("Output will be written to", "file", p.finalOutputFile)

	// Now initialize FileFilter with the known output file path
	ffConfig := filefilter.FilterConfig{
		MaxFileSize:                    p.config.MaxFileSize,
		UserExcludeDirs:                p.config.UserExcludeDirs,
		UserExcludeExts:                p.config.UserExcludeExts,
		UserExcludeGlobs:               p.config.UserExcludeGlobs,
		SkipAuxFiles:                   p.config.SkipAuxFiles,
		DefaultExcludeDirs:             p.config.DefaultExcludeDirs,
		DefaultMediaExts:               p.config.DefaultMediaExts,
		DefaultArchiveExts:             p.config.DefaultArchiveExts,
		DefaultExecExts:                p.config.DefaultExecExts,
		DefaultLockfilePatterns:        p.config.DefaultLockfilePatterns,
		DefaultMiscellaneousFileNames:  p.config.DefaultMiscellaneousFileNames,
		DefaultMiscellaneousExtensions: p.config.DefaultMiscellaneousExtensions,
		DefaultAuxExts:                 p.config.DefaultAuxExts,
		FinalOutputFilePath:            p.finalOutputFile, // Crucial: pass the output file path for self-exclusion
	}
	p.filter, err = filefilter.NewFileFilter(p.basePath, ffConfig) // Pass basePath for relative path calculations
	if err != nil {
		return fmt.Errorf("processor: failed to initialize file filter: %w", err)
	}
	return nil
}

// compileAndCacheGitIgnore compiles a .gitignore file if it exists at the given dirPath (absolute)
// and caches the compiled matcher (or nil if no file/error).
func (p *Processor) compileAndCacheGitIgnore(dirPath string) (*gitignore.GitIgnore, error) {
	gitIgnorePath := filepath.Join(dirPath, ".gitignore")

	// Check cache first
	if matcher, RIsCached := p.gitIgnoreCache[gitIgnorePath]; RIsCached {
		return matcher, nil // Return cached matcher (could be nil)
	}

	// Check if .gitignore file exists
	if _, statErr := os.Stat(gitIgnorePath); statErr == nil {
		// File exists, try to compile it
		matcher, compileErr := gitignore.CompileIgnoreFile(gitIgnorePath)
		if compileErr != nil {
			slog.Warn("Processor: Failed to compile .gitignore, it will be ineffective", "path", gitIgnorePath, "error", compileErr)
			p.gitIgnoreCache[gitIgnorePath] = nil // Cache nil to prevent re-attempts and indicate failure
			return nil, nil                       // Not a fatal error for the whole process, just this .gitignore is skipped
		}
		slog.Debug("Processor: Loaded and compiled .gitignore", "path", gitIgnorePath)
		p.gitIgnoreCache[gitIgnorePath] = matcher // Cache the successful matcher
		return matcher, nil
	} else if !os.IsNotExist(statErr) {
		// Some other error stating the file (e.g., permission denied)
		slog.Warn("Processor: Error trying to stat .gitignore file", "path", gitIgnorePath, "error", statErr)
	}
	// File does not exist or unstat-able for non-existence reasons, cache nil
	p.gitIgnoreCache[gitIgnorePath] = nil
	return nil, nil
}

func (p *Processor) Process() error {
	// Step 1: Setup base paths (local or cloned repo)
	if err := p.setupInitialPaths(); err != nil {
		return err // Error already contextualized by setupInitialPaths
	}

	// Step 2: Determine the final output file path and initialize the file filter.
	// The filter needs to know the output file path to exclude it.
	if err := p.determineOutputFileAndInitFilter(); err != nil {
		return err // Error already contextualized
	}

	// Defer cleanup if a temporary repository was cloned
	if p.isTempRepo && p.tempRepoDir != "" {
		defer func() {
			slog.Info("Cleaning up temporary repository parent directory...", "path", p.tempRepoDir)
			if err := os.RemoveAll(p.tempRepoDir); err != nil {
				slog.Error("Processor: Failed to remove temporary directory", "path", p.tempRepoDir, "error", err)
			} else {
				slog.Debug("Processor: Temporary repository parent directory removed successfully.")
			}
		}()
	}

	// The explicit error check for "output file path is inside the processed source directory"
	// is no longer needed here, as the FileFilter will now handle excluding the output file.

	// Write to a temporary file first to prevent data loss on error and to handle outputting to source dir
	tempOutFile, err := os.CreateTemp(filepath.Dir(p.finalOutputFile), "c2c_out_*.tmp")
	if err != nil {
		return fmt.Errorf("processor: failed to create temporary output file: %w", err)
	}
	tempFileName := tempOutFile.Name()
	successfulWrite := false // Flag to control cleanup of temp file

	defer func() {
		// tempOutFile.Close() might have already been called, but calling again on a closed file is safe.
		_ = tempOutFile.Close()
		if !successfulWrite {
			slog.Debug("Processor: Cleaning up temporary output file due to error or incomplete processing", "path", tempFileName)
			if removeErr := os.Remove(tempFileName); removeErr != nil {
				slog.Warn("Processor: Failed to remove incomplete temporary output file", "path", tempFileName, "error", removeErr)
			}
		}
	}()

	writer := bufio.NewWriter(tempOutFile)

	// 1. Generate and write tree if enabled
	if p.config.IncludeTree {
		slog.Info("Generating file tree...")
		// TreeBuilder uses the same filter instance, so it will also exclude the output file.
		// It also uses the shared gitignore cache and compilation function.
		treeBuilder := NewTreeBuilder(p.basePath, p.filter, p.gitIgnoreCache, p.compileAndCacheGitIgnore)
		treeStr, treeErr := treeBuilder.BuildTreeString()
		if treeErr != nil {
			slog.Error("Processor: Failed to generate file tree. Skipping tree output.", "error", treeErr)
			// Continue without tree if it fails
		} else {
			if _, writeErr := writer.WriteString(treeStr + "\n\n"); writeErr != nil {
				return fmt.Errorf("processor: failed to write tree to output: %w", writeErr)
			}
			slog.Debug("Processor: File tree written to output.")
		}
	}

	// 2. Process and write file contents
	slog.Info("Walking directory and processing files...", "path", p.basePath)

	// activeGitIgnores stores compiled .gitignore objects from root down to current path for the WalkDir callback.
	// Initialize with the root .gitignore if it exists.
	var rootGitIgnoreMatchers []*gitignore.GitIgnore
	if matcher, _ := p.compileAndCacheGitIgnore(p.basePath); matcher != nil {
		rootGitIgnoreMatchers = append(rootGitIgnoreMatchers, matcher)
	}

	walkErr := filepath.WalkDir(p.basePath, func(currentPath string, d fs.DirEntry, walkPathErr error) error {
		if walkPathErr != nil {
			slog.Warn("Processor: Error accessing path during walk (entry skipped)", "path", currentPath, "error", walkPathErr)
			if d != nil && d.IsDir() && errors.Is(walkPathErr, fs.ErrPermission) {
				return fs.SkipDir // Skip directories we can't read.
			}
			return nil // Skip this entry but continue walk for other recoverable errors.
		}

		absCurrentPath := currentPath // filepath.WalkDir provides absolute paths if the root is absolute.
		// Ensure basePath was made absolute earlier.

		// Build the stack of active .gitignore matchers for the current path.
		// The stack goes from root-most .gitignore to the deepest one applicable.
		var currentActiveIgnores []*gitignore.GitIgnore
		currentDir := absCurrentPath
		if !d.IsDir() {
			currentDir = filepath.Dir(absCurrentPath)
		}

		// Collect matchers from currentDir up to basePath
		var pathStack []*gitignore.GitIgnore // Deepest first in this temp stack
		for strings.HasPrefix(currentDir, p.basePath) && currentDir != "" {
			matcher, _ := p.compileAndCacheGitIgnore(currentDir)
			if matcher != nil {
				pathStack = append(pathStack, matcher)
			}
			if currentDir == p.basePath {
				break // Stop once we've processed the basePath's .gitignore
			}
			parentDir := filepath.Dir(currentDir)
			if parentDir == currentDir { // Safety break for filesystem root
				break
			}
			currentDir = parentDir
		}
		// Reverse pathStack to get [root, sub, subsub] order
		for i := len(pathStack) - 1; i >= 0; i-- {
			currentActiveIgnores = append(currentActiveIgnores, pathStack[i])
		}

		// Now, call the filter
		excluded, filterErr := p.filter.IsExcluded(absCurrentPath, d, currentActiveIgnores)
		if filterErr != nil {
			// Check if it's a SkipDir signal from the filter itself
			if errors.Is(filterErr, filepath.SkipDir) {
				slog.Debug("Processor: Directory skipped by filter's SkipDir directive", "path", currentPath)
				return filepath.SkipDir
			}
			// For other errors from filter (e.g., stat failure for a file), log and skip entry
			slog.Warn("Processor: Error during filtering process, skipping entry", "path", currentPath, "error", filterErr)
			return nil // Skip this entry but continue walk
		}

		if excluded {
			if d.IsDir() { // If filter excluded a directory (not via SkipDir error but bool return)
				slog.Debug("Processor: Directory excluded by filter, skipping its contents", "path", currentPath)
				return filepath.SkipDir
			}
			// If it's an excluded file, filter might have logged it if verbose.
			return nil
		}

		// If it's a directory and not excluded, WalkDir will traverse into it. Nothing to do here for dirs.
		if d.IsDir() {
			return nil
		}

		// --- File processing: If we reach here, it's a file to include ---
		relPath, relErr := filepath.Rel(p.basePath, absCurrentPath)
		if relErr != nil {
			slog.Warn("Processor: Could not get relative path for included file (skipping)", "path", absCurrentPath, "error", relErr)
			return nil // Skip this file
		}
		slog.Info("Processor: Including file", "path", relPath)

		// Write file path header (use forward slashes for consistency in output)
		header := fmt.Sprintf("```%s\n", filepath.ToSlash(relPath))
		if _, writeErr := writer.WriteString(header); writeErr != nil {
			// This is a more critical error, likely relates to disk space or permissions for the temp output file.
			return fmt.Errorf("processor: failed to write file header for '%s' to temporary output: %w", relPath, writeErr)
		}

		// Write file content
		file, openErr := os.Open(absCurrentPath)
		if openErr != nil {
			slog.Warn("Processor: Failed to open file for reading (content skipped)", "path", relPath, "error", openErr)
			// Write a note into the output file about the failure
			if _, noteErr := fmt.Fprintf(writer, "// Error reading file '%s': %v\n", relPath, openErr); noteErr != nil {
				return fmt.Errorf("processor: failed to write error note for '%s' to temporary output: %w", relPath, noteErr)
			}
		} else {
			// Using a scanner is good for line-by-line processing.
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				if _, writeErr := writer.WriteString(scanner.Text() + "\n"); writeErr != nil {
					_ = file.Close()
					return fmt.Errorf("processor: failed to write file content for '%s' to temporary output: %w", relPath, writeErr)
				}
			}
			if scanErr := scanner.Err(); scanErr != nil {
				slog.Warn("Processor: Error scanning file content", "path", relPath, "error", scanErr)
				if _, noteErr := fmt.Fprintf(writer, "// Error scanning file '%s': %v\n", relPath, scanErr); noteErr != nil {
					_ = file.Close()
					return fmt.Errorf("processor: failed to write scan error note for '%s' to temporary output: %w", relPath, noteErr)
				}
			}
			_ = file.Close()
		}

		// Write file path footer
		if _, writeErr := writer.WriteString("```\n\n"); writeErr != nil {
			return fmt.Errorf("processor: failed to write file footer for '%s' to temporary output: %w", relPath, writeErr)
		}
		return nil
	})

	if walkErr != nil {
		// This error is from the WalkDir function itself or propagated from a critical error in the callback.
		return fmt.Errorf("processor: error during file walk: %w", walkErr)
	}

	// All content successfully written to tempOutFile's buffer
	if flushErr := writer.Flush(); flushErr != nil {
		return fmt.Errorf("processor: failed to flush writer for temporary output file: %w", flushErr)
	}
	if closeErr := tempOutFile.Close(); closeErr != nil { // Ensure temp file is closed before rename
		return fmt.Errorf("processor: failed to close temporary output file '%s': %w", tempFileName, closeErr)
	}

	// Rename temporary file to final output file
	slog.Debug("Processor: Attempting to rename temporary output file", "from", tempFileName, "to", p.finalOutputFile)
	if renameErr := os.Rename(tempFileName, p.finalOutputFile); renameErr != nil {
		slog.Warn("Processor: Rename failed, attempting copy fallback", "from", tempFileName, "to", p.finalOutputFile, "error", renameErr)
		// Fallback to copy if rename fails (e.g., across different devices/filesystems)
		in, readErr := os.Open(tempFileName)
		if readErr != nil {
			// Original temp file might still be there, don't remove if open failed.
			return fmt.Errorf("processor: failed to open temp file '%s' for copying: %w (original rename error: %v)", tempFileName, readErr, renameErr)
		}
		// defer in.Close() // Not needed here as 'in' is local to this block

		out, createErr := os.Create(p.finalOutputFile)
		if createErr != nil {
			_ = in.Close()
			return fmt.Errorf("processor: failed to create final output file '%s' for copying: %w (original rename error: %v)", p.finalOutputFile, createErr, renameErr)
		}
		// defer out.Close() // Not needed here

		_, copyErr := io.Copy(out, in)
		_ = in.Close()  // Close input file after copy attempt
		_ = out.Close() // Close output file after copy attempt

		if copyErr != nil {
			return fmt.Errorf("processor: failed to copy temp file to final output file: %w (original rename error: %v)", copyErr, renameErr)
		}
		// If copy succeeds, remove the original temporary file
		if removeErr := os.Remove(tempFileName); removeErr != nil {
			slog.Warn("Processor: Failed to remove temporary output file after successful copy", "path", tempFileName, "error", removeErr)
		}
	}
	successfulWrite = true // Mark as successful so defer doesn't remove the (now renamed or copied) temp file.
	slog.Info("Successfully wrote output to", "file", p.finalOutputFile)
	return nil
}
