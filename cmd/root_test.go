package cmd

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/alexferrari88/code2context/internal/appconfig"
	"github.com/alexferrari88/code2context/internal/processor"
	"github.com/spf13/cobra"
)

var capturedProcessorConfig processor.Config

// mockProcessorImpl is a mock implementation of the processorInterface.
type mockProcessorImpl struct {
	processError   error  // Error to be returned by Process()
	mockOutputFile string // Value to be returned by GetFinalOutputFile()
}

func (m *mockProcessorImpl) Process() error {
	return m.processError
}

func (m *mockProcessorImpl) GetFinalOutputFile() string {
	return m.mockOutputFile
}

// setupMockProcessorFunc replaces the actual newProcessorFunc with our mock.
// It captures the config and allows specifying errors for New and Process stages.
// - newProcessorError: if non-nil, newProcessorFunc returns this error.
// - processError: if newProcessorError is nil, newProcessorFunc returns a mockProcessorImpl
//   whose Process method will return this error.
func setupMockProcessorFunc(t *testing.T, newProcessorError error, processError error, mockOutput string) {
	originalNewProcessorFunc := newProcessorFunc
	newProcessorFunc = func(cfg processor.Config) (processorInterface, error) {
		capturedProcessorConfig = cfg
		if newProcessorError != nil {
			return nil, newProcessorError
		}
		return &mockProcessorImpl{
			processError:   processError,
			mockOutputFile: mockOutput,
		}, nil
	}
	t.Cleanup(func() {
		newProcessorFunc = originalNewProcessorFunc
		capturedProcessorConfig = processor.Config{} // Reset captured config
	})
}

// Helper function to reset command flags to their defaults and clear args
func resetRootCmdFlags() {
	rootCmd.ResetFlags()
	// Re-initialize flags to their default values as defined in init()
	// This is important because ResetFlags() only removes persistent flags
	// but doesn't reset their values to the defaults specified by AddFlag.
	// We need to manually re-run the flag setup.

	// First, clear existing flags, if any, to avoid "flag redefined" errors
	// This is a bit of a hack; ideally, cobra would provide a more straightforward way
	// to reset to initial defaults.
	// The VisitAll part was problematic and might not be necessary if ResetFlags clears them sufficiently
	// for re-registration. Let's simplify this. If flags are still an issue, specific tests might need
	// to create new Command instances.
	// rootCmd.Flags().VisitAll(func(f *pflag.Flag) {
	// This line was causing issues: rootCmd.Flags().VarPF(f.Value, f.Name, f.Shorthand).DefValue = f.DefValue
	// })

	// Re-register all flags with their default values
	outputFile = ""
	gitRef = ""
	includeTree = true // Default true
	noTree = false
	skipAuxFiles = false
	excludeDirsRaw = ""
	excludeExtsRaw = ""
	excludeGlobsRaw = ""
	maxFileSizeStr = "1MB"
	verbose = false

	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file name")
	rootCmd.Flags().StringVar(&gitRef, "ref", "", "Git reference for remote repositories")
	rootCmd.Flags().BoolVar(&includeTree, "tree", true, "Include tree representation (default true)")
	rootCmd.Flags().BoolVar(&noTree, "no-tree", false, "Disable tree representation")
	rootCmd.Flags().BoolVar(&skipAuxFiles, "skip-aux-files", false, "Skip auxiliary files")
	rootCmd.Flags().StringVar(&excludeDirsRaw, "exclude-dirs", "", "Comma-separated list of directory names to exclude")
	rootCmd.Flags().StringVar(&excludeExtsRaw, "exclude-exts", "", "Comma-separated list of file extensions to exclude")
	rootCmd.Flags().StringVar(&excludeGlobsRaw, "exclude-patterns", "", "Comma-separated list of glob patterns to exclude")
	rootCmd.Flags().StringVar(&maxFileSizeStr, "max-file-size", "1MB", "Maximum file size")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

	rootCmd.SetArgs([]string{}) // Clear any previous arguments
}

// TestRootCmdExists checks if rootCmd is not nil and has the correct Use value.
func TestRootCmdExists(t *testing.T) {
	if rootCmd == nil {
		t.Fatal("rootCmd is nil")
	}
	expectedUse := "c2c <path_or_url>"
	if rootCmd.Use != expectedUse {
		t.Errorf("rootCmd.Use = %q, want %q", rootCmd.Use, expectedUse)
	}
}

// TestDefaultFlagValues checks the default values of the flags.
func TestDefaultFlagValues(t *testing.T) {
	resetRootCmdFlags() // Ensure flags are at their defaults

	testCases := []struct {
		name         string
		flagName     string
		expectedVal  string
		actualVal    func() string
		actualValBool func() bool
		isBoolFlag   bool
	}{
		{
			name:        "outputFile default",
			flagName: "output",
			// DefValue is what's used if the flag isn't provided.
			expectedVal: "",
			actualVal:   func() string { return rootCmd.Flag("output").DefValue },
		},
		{
			name:        "gitRef default",
			flagName:    "ref",
			expectedVal: "",
			actualVal:   func() string { return rootCmd.Flag("ref").DefValue },
		},
		{
			name:         "includeTree default",
			flagName:     "tree",
			expectedVal:  "true", // Default value of the flag itself
			actualValBool: func() bool { b, _ := rootCmd.Flags().GetBool("tree"); return b },
			isBoolFlag:    true,
		},
		{
			name:         "noTree default",
			flagName:     "no-tree",
			expectedVal:  "false", // Default value of the flag itself
			actualValBool: func() bool { b, _ := rootCmd.Flags().GetBool("no-tree"); return b },
			isBoolFlag:    true,
		},
		{
			name:         "skipAuxFiles default",
			flagName:     "skip-aux-files",
			expectedVal:  "false", // Default value of the flag itself
			actualValBool: func() bool { b, _ := rootCmd.Flags().GetBool("skip-aux-files"); return b },
			isBoolFlag:    true,
		},
		{
			name:        "excludeDirsRaw default",
			flagName:    "exclude-dirs",
			expectedVal: "",
			actualVal:   func() string { return rootCmd.Flag("exclude-dirs").DefValue },
		},
		{
			name:        "excludeExtsRaw default",
			flagName:    "exclude-exts",
			expectedVal: "",
			actualVal:   func() string { return rootCmd.Flag("exclude-exts").DefValue },
		},
		{
			name:        "excludeGlobsRaw default",
			flagName:    "exclude-patterns",
			expectedVal: "",
			actualVal:   func() string { return rootCmd.Flag("exclude-patterns").DefValue },
		},
		{
			name:        "maxFileSizeStr default",
			flagName:    "max-file-size",
			expectedVal: "1MB",
			actualVal:   func() string { return rootCmd.Flag("max-file-size").DefValue },
		},
		{
			name:         "verbose default",
			flagName:     "verbose",
			expectedVal:  "false", // Default value of the flag itself
			actualValBool: func() bool { b, _ := rootCmd.Flags().GetBool("verbose"); return b },
			isBoolFlag:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// For boolean flags, GetBool correctly returns the actual default value if not set.
			// For string flags, DefValue is the correct field to check for the default.
			if tc.isBoolFlag {
				// We need to ensure the flag is actually registered before trying to get its value
				// The resetRootCmdFlags should handle this by re-adding flags.
				if rootCmd.Flag(tc.flagName) == nil {
					t.Fatalf("Flag %s not found after reset", tc.flagName)
				}
				actualBoolVal, err := rootCmd.Flags().GetBool(tc.flagName)
				if err != nil {
					t.Fatalf("Error getting bool flag %s: %v", tc.flagName, err)
				}
				expectedBoolVal := (tc.expectedVal == "true")
				if actualBoolVal != expectedBoolVal {
					t.Errorf("Flag %s default value = %v, want %v", tc.flagName, actualBoolVal, expectedBoolVal)
				}
			} else {
				if rootCmd.Flag(tc.flagName) == nil {
					t.Fatalf("Flag %s not found after reset", tc.flagName)
				}
				actualStrVal := rootCmd.Flag(tc.flagName).DefValue
				if actualStrVal != tc.expectedVal {
					t.Errorf("Flag %s default value = %q, want %q", tc.flagName, actualStrVal, tc.expectedVal)
				}
			}
		})
	}
}

// TestPathArgumentHandling tests how rootCmd handles path arguments.
func TestPathArgumentHandling(t *testing.T) {
	// Store original RunE and defer restoration
	originalRunE := rootCmd.RunE
	t.Cleanup(func() { rootCmd.RunE = originalRunE })

	// For these tests, we only care about argument parsing, not the actual execution.
	// So, we set a dummy RunE.
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return nil // Indicate success for arg parsing
	}

	tests := []struct {
		name      string
		args      []string
		expectErr bool
		errSubstr string // Substring to look for in the error message
	}{
		{
			name:      "valid single argument",
			args:      []string{"."},
			expectErr: false,
		},
		{
			name:      "no arguments",
			args:      []string{},
			expectErr: true,
			errSubstr: "accepts 1 arg(s), received 0",
		},
		{
			name:      "too many arguments",
			args:      []string{".", "another"},
			expectErr: true,
			errSubstr: "accepts 1 arg(s), received 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected an error for args: %v, but got nil", tt.args)
				} else if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("Expected error for args: %v to contain %q, but got: %v", tt.args, tt.errSubstr, err)
				}
			} else {
				if err != nil {
					// For the valid case, if RunE is reached, Execute() might still return an error
					// if the dummy RunE itself returns an error. Here we expect no error.
					// We are primarily checking that "accepts X arg(s)" error does NOT occur.
					if strings.Contains(err.Error(), "accepts 1 arg(s)") {
						t.Errorf("Did not expect an argument count error for args: %v, but got: %v", tt.args, err)
					}
				}
			}
		})
	}
}

func TestFlagParsingOutput(t *testing.T) {
	resetRootCmdFlags()
	setupMockProcessorFunc(t, nil, nil, "mock_output.txt") // No errors, default mock output

	expectedOutput := "test_output.txt"
	rootCmd.SetArgs([]string{".", "--output", expectedOutput})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	if capturedProcessorConfig.OutputFile != expectedOutput {
		t.Errorf("OutputFile = %q, want %q", capturedProcessorConfig.OutputFile, expectedOutput)
	}
}

func TestFlagParsingGitRef(t *testing.T) {
	resetRootCmdFlags()
	setupMockProcessorFunc(t, nil, nil, "mock_output.txt")

	expectedGitRef := "mybranch"
	rootCmd.SetArgs([]string{"https://github.com/some/repo", "--ref", expectedGitRef})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	if capturedProcessorConfig.GitRef != expectedGitRef {
		t.Errorf("GitRef = %q, want %q", capturedProcessorConfig.GitRef, expectedGitRef)
	}
}

func TestFlagParsingTreeLogic(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedTree   bool
		checkNoTreeVal bool // if true, check the 'noTree' variable directly
		expectedNoTree bool // expected value for 'noTree' if checkNoTreeVal is true
	}{
		{"no tree flags", []string{"."}, true, false, false},
		{"--tree", []string{".", "--tree"}, true, false, false},
		{"--tree=true", []string{".", "--tree=true"}, true, false, false},
		{"--tree=false", []string{".", "--tree=false"}, false, false, false},
		{"--no-tree", []string{".", "--no-tree"}, false, true, true},
		{"--no-tree=true", []string{".", "--no-tree=true"}, false, true, true},
		{"--no-tree=false", []string{".", "--no-tree=false"}, true, true, false}, // --no-tree=false means "do not 'no-tree'", so tree is true
		{"--tree --no-tree", []string{".", "--tree", "--no-tree"}, false, true, true},
		{"--tree=true --no-tree=true", []string{".", "--tree=true", "--no-tree=true"}, false, true, true},
		{"--tree=false --no-tree", []string{".", "--tree=false", "--no-tree"}, false, true, true}, // --no-tree wins
		{"--tree --no-tree=false", []string{".", "--tree", "--no-tree=false"}, true, true, false}, // --no-tree=false means tree is true
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetRootCmdFlags()
			setupMockProcessorFunc(t, nil, nil, "mock_output.txt")

			// Reset the global vars that store flag values, Cobra doesn't always reset them fully
			// for bool flags if they are not explicitly provided in args.
			includeTree = true // Default for --tree
			noTree = false     // Default for --no-tree

			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()
			if err != nil {
				t.Fatalf("Execute() failed: %v", err)
			}

			if capturedProcessorConfig.IncludeTree != tt.expectedTree {
				t.Errorf("IncludeTree = %v, want %v for args: %v", capturedProcessorConfig.IncludeTree, tt.expectedTree, tt.args)
			}

			if tt.checkNoTreeVal {
				// This checks the global `noTree` variable value after parsing.
				// Need to inspect the flag value directly from cobra if the global var isn't perfectly synced
				// For this test, we assume the global 'noTree' var correctly reflects the parsed flag value.
				// This is a bit of a white-box test for the flag variable itself.
				actualNoTreeVal, _ := rootCmd.Flags().GetBool("no-tree")
				if actualNoTreeVal != tt.expectedNoTree {
					t.Errorf("global noTree var = %v, want %v for args: %v", actualNoTreeVal, tt.expectedNoTree, tt.args)
				}
			}
		})
	}
}

func TestFlagParsingSkipAuxFiles(t *testing.T) {
	testCases := []struct {
		name                string
		args                []string
		expectedSkipAuxFiles bool
	}{
		{
			name:                "with --skip-aux-files",
			args:                []string{".", "--skip-aux-files"},
			expectedSkipAuxFiles: true,
		},
		{
			name:                "without --skip-aux-files",
			args:                []string{"."},
			expectedSkipAuxFiles: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resetRootCmdFlags()
			setupMockProcessorFunc(t, nil, nil, "mock_output.txt")
			skipAuxFiles = false // Reset before parsing

			rootCmd.SetArgs(tc.args)
			err := rootCmd.Execute()
			if err != nil {
				t.Fatalf("Execute() failed: %v", err)
			}

			if capturedProcessorConfig.SkipAuxFiles != tc.expectedSkipAuxFiles {
				t.Errorf("SkipAuxFiles = %v, want %v", capturedProcessorConfig.SkipAuxFiles, tc.expectedSkipAuxFiles)
			}
		})
	}
}

func TestFlagParsingExcludeDirs(t *testing.T) {
	tests := []struct {
		name              string
		args              []string
		expectedExcludeDirs []string
	}{
		{"no exclude-dirs", []string{"."}, nil}, // Expect nil or empty slice
		{"empty exclude-dirs", []string{".", "--exclude-dirs", ""}, nil},
		{"single dir", []string{".", "--exclude-dirs", "dir1"}, []string{"dir1"}},
		{"multiple dirs", []string{".", "--exclude-dirs", "dir1, dir2,dir3 "}, []string{"dir1", "dir2", "dir3"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetRootCmdFlags()
			setupMockProcessorFunc(t, nil, nil, "mock_output.txt")
			excludeDirsRaw = "" // reset

			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()
			if err != nil {
				t.Fatalf("Execute() failed: %v", err)
			}
			if len(capturedProcessorConfig.UserExcludeDirs) == 0 && len(tt.expectedExcludeDirs) == 0 {
				// Both are nil/empty, which is fine.
			} else if !reflect.DeepEqual(capturedProcessorConfig.UserExcludeDirs, tt.expectedExcludeDirs) {
				t.Errorf("UserExcludeDirs = %v, want %v", capturedProcessorConfig.UserExcludeDirs, tt.expectedExcludeDirs)
			}
		})
	}
}

func TestFlagParsingExcludeExts(t *testing.T) {
	tests := []struct {
		name              string
		args              []string
		expectedExcludeExts []string
	}{
		{"no exclude-exts", []string{"."}, nil},
		{"empty exclude-exts", []string{".", "--exclude-exts", ""}, nil},
		{"single ext", []string{".", "--exclude-exts", ".log"}, []string{".log"}},
		{"multiple exts", []string{".", "--exclude-exts", ".log, .tmp,json "}, []string{".log", ".tmp", ".json"}},
		{"ext without dot", []string{".", "--exclude-exts", "txt"}, []string{".txt"}},
		{"mixed exts", []string{".", "--exclude-exts", "log,.md,toml"}, []string{".log", ".md", ".toml"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetRootCmdFlags()
			setupMockProcessorFunc(t, nil, nil, "mock_output.txt")
			excludeExtsRaw = "" // reset

			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()
			if err != nil {
				t.Fatalf("Execute() failed: %v", err)
			}
			if len(capturedProcessorConfig.UserExcludeExts) == 0 && len(tt.expectedExcludeExts) == 0 {
				// Both are nil/empty, which is fine.
			} else if !reflect.DeepEqual(capturedProcessorConfig.UserExcludeExts, tt.expectedExcludeExts) {
				t.Errorf("UserExcludeExts = %#v, want %#v", capturedProcessorConfig.UserExcludeExts, tt.expectedExcludeExts)
			}
		})
	}
}

func TestFlagParsingExcludePatterns(t *testing.T) {
	tests := []struct {
		name                string
		args                []string
		expectedExcludeGlobs []string
	}{
		{"no exclude-patterns", []string{"."}, nil},
		{"empty exclude-patterns", []string{".", "--exclude-patterns", ""}, nil},
		{"single pattern", []string{".", "--exclude-patterns", "pattern1"}, []string{"pattern1"}},
		{"multiple patterns", []string{".", "--exclude-patterns", "pattern1, pattern2 "}, []string{"pattern1", "pattern2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetRootCmdFlags()
			setupMockProcessorFunc(t, nil, nil, "mock_output.txt")
			excludeGlobsRaw = "" // reset

			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()
			if err != nil {
				t.Fatalf("Execute() failed: %v", err)
			}
			if len(capturedProcessorConfig.UserExcludeGlobs) == 0 && len(tt.expectedExcludeGlobs) == 0 {
				// Both are nil/empty, which is fine.
			} else if !reflect.DeepEqual(capturedProcessorConfig.UserExcludeGlobs, tt.expectedExcludeGlobs) {
				t.Errorf("UserExcludeGlobs = %v, want %v", capturedProcessorConfig.UserExcludeGlobs, tt.expectedExcludeGlobs)
			}
		})
	}
}

func TestFlagParsingMaxFileSize(t *testing.T) {
	tests := []struct {
		name              string
		args              []string
		expectedMaxFileSize int64
		expectError       bool
		errorContains     string
	}{
		{"500KB", []string{".", "--max-file-size", "500KB"}, 500 * 1024, false, ""},
		{"2MB", []string{".", "--max-file-size", "2MB"}, 2 * 1024 * 1024, false, ""},
		{"1024 (bytes)", []string{".", "--max-file-size", "1024"}, 1024, false, ""},
		{"0 (no limit)", []string{".", "--max-file-size", "0"}, 0, false, ""}, // Assuming 0 means no limit or specific handling
		{"1gb", []string{".", "--max-file-size", "1gb"}, 1 * 1024 * 1024 * 1024, false, ""},
		{"invalid value abc", []string{".", "--max-file-size", "abc"}, 0, true, "invalid max file size"},
		// Test overflow with a value that correctly causes int64 overflow
		{"overflowing unit", []string{".", "--max-file-size", "8388608TB"}, 0, true, "overflows int64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetRootCmdFlags()
			// No error expected from mock processor itself for successful parsing cases
			// For error cases, newProcessorError will be nil, allowing parsing to proceed.
			setupMockProcessorFunc(t, nil, nil, "mock_output.txt")
			maxFileSizeStr = "1MB" // reset

			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error, but got nil")
				}
				if tt.errorContains != "" && !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errorContains)) {
					t.Errorf("Error message %q does not contain %q", err.Error(), tt.errorContains)
				}
			} else {
				if err != nil {
					t.Fatalf("Execute() failed: %v", err)
				}
				if capturedProcessorConfig.MaxFileSize != tt.expectedMaxFileSize {
					t.Errorf("MaxFileSize = %d, want %d", capturedProcessorConfig.MaxFileSize, tt.expectedMaxFileSize)
				}
			}
		})
	}
}

func TestFlagParsingVerbose(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		expectedVerbose bool
	}{
		{"no verbose flag", []string{"."}, false},
		{"-v flag", []string{".", "-v"}, true},
		{"--verbose flag", []string{".", "--verbose"}, true},
	}

	// This test primarily checks the `verbose` global variable in the `cmd` package.
	// Testing the logger initialization directly is harder without specific logger mocks.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetRootCmdFlags()
			setupMockProcessorFunc(t, nil, nil, "mock_output.txt") // Mock for consistent execution path
			verbose = false               // Reset global verbose flag

			// Store original RunE and Run, then restore
			originalRunE := rootCmd.RunE
			originalRun := rootCmd.Run // Just in case Run is used (though RunE is typical)
			t.Cleanup(func() {
				rootCmd.RunE = originalRunE
				rootCmd.Run = originalRun
				verbose = false // Clean up global state
			})

			// We need to execute the command to have PersistentPreRun populate 'verbose'
			// and then have RunE use it (though here we mostly care about PersistentPreRun's effect)
			// For this test, the actual execution of RunE is not critical beyond flag parsing.
			// We mock newProcessor, so the core logic of RunE won't run.
			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute() // This will trigger PersistentPreRun

			if err != nil {
				// We don't expect errors related to verbose flag parsing itself.
				// Errors might come from arg validation if not handled, or mocked processor.
				// For this test, assume args are valid (e.g. ".")
				t.Logf("Execute() error (may be benign if from arg validation or mock): %v", err)
			}

			// The `verbose` variable in the `cmd` package should be updated by PersistentPreRun.
			if verbose != tt.expectedVerbose {
				t.Errorf("Global verbose variable = %v, want %v for args: %v", verbose, tt.expectedVerbose, tt.args)
			}

			// Additionally, if verbose were passed to config (it's not directly, logger is just initialized)
			// we could check capturedProcessorConfig.Verbose or similar.
			// For now, testing the global `verbose` flag is the primary check.
		})
	}
}

// TestDefaultValuesInProcessorConfig verifies that when no specific flags
// are set, the processor.Config gets the correct default values from appconfig.
func TestDefaultValuesInProcessorConfig(t *testing.T) {
	resetRootCmdFlags()
	setupMockProcessorFunc(t, nil, nil, "mock_output.txt")

	rootCmd.SetArgs([]string{"."}) // Minimal valid arguments
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	// Check a few key default fields that are sourced from appconfig
	if !reflect.DeepEqual(capturedProcessorConfig.DefaultExcludeDirs, appconfig.GetDefaultExcludedDirs()) {
		t.Errorf("DefaultExcludeDirs mismatch. Got %v, want %v", capturedProcessorConfig.DefaultExcludeDirs, appconfig.GetDefaultExcludedDirs())
	}
	if !reflect.DeepEqual(capturedProcessorConfig.DefaultMediaExts, appconfig.GetDefaultMediaExtensions()) {
		t.Errorf("DefaultMediaExts mismatch. Got %v, want %v", capturedProcessorConfig.DefaultMediaExts, appconfig.GetDefaultMediaExtensions())
	}
	if !reflect.DeepEqual(capturedProcessorConfig.DefaultArchiveExts, appconfig.GetDefaultArchiveExtensions()) {
		t.Errorf("DefaultArchiveExts mismatch. Got %v, want %v", capturedProcessorConfig.DefaultArchiveExts, appconfig.GetDefaultArchiveExtensions())
	}
	if !reflect.DeepEqual(capturedProcessorConfig.DefaultExecExts, appconfig.GetDefaultExecutableExtensions()) {
		t.Errorf("DefaultExecExts mismatch. Got %v, want %v", capturedProcessorConfig.DefaultExecExts, appconfig.GetDefaultExecutableExtensions())
	}
	if !reflect.DeepEqual(capturedProcessorConfig.DefaultLockfilePatterns, appconfig.GetDefaultLockfilePatterns()) {
		t.Errorf("DefaultLockfilePatterns mismatch. Got %v, want %v", capturedProcessorConfig.DefaultLockfilePatterns, appconfig.GetDefaultLockfilePatterns())
	}
}

func TestProcessorNewError(t *testing.T) {
	resetRootCmdFlags()
	expectedErrStr := "mock processor new error"
	// Setup mock to return an error when newProcessorFunc is called
	setupMockProcessorFunc(t, fmt.Errorf("%s", expectedErrStr), nil, "")

	rootCmd.SetArgs([]string{"."}) // Valid args
	err := rootCmd.Execute()

	if err == nil {
		t.Fatal("Expected an error from rootCmd.Execute(), but got nil")
	}

	// The error from RunE is wrapped, so we check if it contains the expected message.
	if !strings.Contains(err.Error(), expectedErrStr) {
		t.Errorf("Error message %q does not contain expected string %q", err.Error(), expectedErrStr)
	}
}

func TestProcessorProcessError(t *testing.T) {
	resetRootCmdFlags()
	expectedErrStr := "mock processor process error"
	// Setup mock: newProcessorFunc succeeds, but the returned mockProcessor's Process() method returns an error.
	setupMockProcessorFunc(t, nil, fmt.Errorf("%s", expectedErrStr), "mock_output.txt")

	rootCmd.SetArgs([]string{"."}) // Valid args
	err := rootCmd.Execute()

	if err == nil {
		t.Fatal("Expected an error from rootCmd.Execute(), but got nil")
	}

	// In this case, the error comes directly from proc.Process() and is returned by RunE.
	// Cobra might wrap it, or it might come through as is.
	// For now, let's assume it's not double-wrapped by RunE's "failed to initialize"
	if !strings.Contains(err.Error(), expectedErrStr) {
		t.Errorf("Error message %q does not contain expected string %q", err.Error(), expectedErrStr)
	}
}
