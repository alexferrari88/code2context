package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/alexferrari88/code2context/internal/appconfig"
	"github.com/alexferrari88/code2context/internal/processor"
	"github.com/alexferrari88/code2context/internal/utils"
	"github.com/spf13/cobra"
)

// processorInterface defines the methods we expect from a processor.
type processorInterface interface {
	Process() error
	GetFinalOutputFile() string
}

// newProcessorFunc is a variable that holds the function to create a new processor.
// It's initialized with a wrapper around processor.New, but can be replaced by a mock in tests.
var newProcessorFunc func(cfg processor.Config) (processorInterface, error) = func(cfg processor.Config) (processorInterface, error) {
	p, err := processor.New(cfg)
	if err != nil {
		return nil, err
	}
	return p, nil // *processor.Processor implicitly satisfies processorInterface
}

var (
	outputFile      string
	gitRef          string
	includeTree     bool // Default true
	noTree          bool // explicit --no-tree
	skipAuxFiles    bool
	excludeDirsRaw  string
	excludeExtsRaw  string
	excludeGlobsRaw string
	maxFileSizeStr  string
	verbose         bool
)

var rootCmd = &cobra.Command{
	Use:   "c2c <path_or_url>",
	Short: "c2c (code2context) aggregates codebase files into a single text file for LLM context.",
	Long: `c2c is a CLI tool that processes a local codebase or a public GitHub repository.
It concatenates the content of selected files into a single .txt output.
The tool intelligently skips common non-code files, respects .gitignore (including nested ones),
and allows for custom exclusion rules. An optional file tree can be included at the top.`,
	Example: `  c2c . -o my_project_context.txt
  c2c ./my_module --no-tree
  c2c https://github.com/spf13/cobra --ref v1.7.0
  c2c . --exclude-dirs "docs,examples" --exclude-exts ".log,.tmp"
  c2c . --skip-aux-files --max-file-size 500KB --exclude-patterns "internal/*_test.go"`,
	Args: cobra.ExactArgs(1),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		utils.InitLogger(verbose)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]

		maxFileSize, err := utils.ParseFileSize(maxFileSizeStr)
		if err != nil {
			return fmt.Errorf("invalid max file size: %w", err)
		}

		var excludeDirs []string
		if excludeDirsRaw != "" {
			excludeDirs = strings.Split(excludeDirsRaw, ",")
			for i, dir := range excludeDirs {
				excludeDirs[i] = strings.TrimSpace(dir)
			}
		}

		var excludeExts []string
		if excludeExtsRaw != "" {
			excludeExts = strings.Split(excludeExtsRaw, ",")
			for i, ext := range excludeExts {
				trimmedExt := strings.TrimSpace(ext)
				if !strings.HasPrefix(trimmedExt, ".") && trimmedExt != "" {
					excludeExts[i] = "." + trimmedExt
				} else {
					excludeExts[i] = trimmedExt
				}
			}
		}

		var excludeGlobs []string
		if excludeGlobsRaw != "" {
			excludeGlobs = strings.Split(excludeGlobsRaw, ",")
			for i, glob := range excludeGlobs {
				excludeGlobs[i] = strings.TrimSpace(glob)
			}
		}

		// Determine final includeTree value
		finalIncludeTree := includeTree     // Default to true via flag default
		if cmd.Flags().Changed("no-tree") { // If --no-tree was explicitly used
			finalIncludeTree = !noTree
		} else if cmd.Flags().Changed("tree") { // If --tree was explicitly used
			finalIncludeTree = includeTree
		}

		cfg := processor.Config{
			SourcePath:                     source,
			GitRef:                         gitRef,
			OutputFile:                     outputFile,
			IncludeTree:                    finalIncludeTree,
			SkipAuxFiles:                   skipAuxFiles,
			UserExcludeDirs:                excludeDirs,
			UserExcludeExts:                excludeExts,
			UserExcludeGlobs:               excludeGlobs,
			MaxFileSize:                    maxFileSize,
			DefaultExcludeDirs:             appconfig.GetDefaultExcludedDirs(),
			DefaultMediaExts:               appconfig.GetDefaultMediaExtensions(),
			DefaultArchiveExts:             appconfig.GetDefaultArchiveExtensions(),
			DefaultExecExts:                appconfig.GetDefaultExecutableExtensions(),
			DefaultLockfilePatterns:        appconfig.GetDefaultLockfilePatterns(),
			DefaultMiscellaneousFileNames:  appconfig.GetDefaultMiscellaneousFileNames(),
			DefaultMiscellaneousExtensions: appconfig.GetDefaultMiscellaneousExtensions(),
			DefaultAuxExts:                 appconfig.GetDefaultAuxFileExtensions(),
		}

		proc, err := newProcessorFunc(cfg)
		if err != nil {
			return fmt.Errorf("failed to initialize processor: %w", err)
		}

		slog.Info("Starting processing...", "source", source)
		err = proc.Process()
		if err != nil {
			// Error should be logged by the processor if it's a processing error.
			// This return will be handled by Cobra (printed to stderr).
			return err
		}
		slog.Info("Processing complete.", "output_file", proc.GetFinalOutputFile())
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// Cobra already prints the error using the RunE pattern
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file name (default: <folder_name>.txt or <repo_name>.txt)")
	rootCmd.Flags().StringVar(&gitRef, "ref", "", "Git reference (branch, tag, commit) for remote repositories")

	// --tree is true by default. --no-tree can explicitly disable it.
	rootCmd.Flags().BoolVar(&includeTree, "tree", true, "Include a tree representation of the codebase (enabled by default)")
	rootCmd.Flags().BoolVar(&noTree, "no-tree", false, "Disable the tree representation of the codebase (overrides --tree if set)")
	// If both --tree=false and --no-tree are set, --no-tree (which means don't include tree) wins.
	// If --tree=true and --no-tree is set, --no-tree wins.
	// This logic is handled in RunE.

	rootCmd.Flags().BoolVar(&skipAuxFiles, "skip-aux-files", false, "Skip non-code, human-readable auxiliary files (json, csv, yml, md, txt, etc.)")
	rootCmd.Flags().StringVar(&excludeDirsRaw, "exclude-dirs", "", "Comma-separated list of directory names to exclude (e.g., \"docs,tests\")")
	rootCmd.Flags().StringVar(&excludeExtsRaw, "exclude-exts", "", "Comma-separated list of file extensions to exclude (e.g., \".log,.tmp,json\")")
	rootCmd.Flags().StringVar(&excludeGlobsRaw, "exclude-patterns", "", "Comma-separated list of glob patterns to exclude (e.g., \"*_test.go,vendor/*\")")
	rootCmd.Flags().StringVar(&maxFileSizeStr, "max-file-size", "1MB", "Maximum file size to include (e.g., \"500KB\", \"2MB\", \"1024\")")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

	// Set executable name for usage printout
	rootCmd.Use = "c2c <path_or_url>"
	cobra.EnableCommandSorting = false
}
