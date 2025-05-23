package processor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexferrari88/code2context/internal/appconfig"
	"github.com/alexferrari88/code2context/internal/gitutils" // Added
	gitignore "github.com/sabhiram/go-gitignore"            // Added
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestDirStructure creates a temporary directory structure for testing.
// structure: map[relativePath]content. If content is empty, it's a directory.
// Returns the root path of the created structure.
func createTestDirStructure(t *testing.T, structure map[string]string) string {
	t.Helper()
	rootDir, err := os.MkdirTemp("", "test_processor_*")
	require.NoError(t, err, "Failed to create temp root dir")

	t.Cleanup(func() {
		err := os.RemoveAll(rootDir)
		if err != nil {
			// Log the error but don't fail the test, as cleanup errors are secondary.
			t.Logf("Warning: failed to remove temp dir %s: %v", rootDir, err)
		}
	})

	for relPath, content := range structure {
		absPath := filepath.Join(rootDir, relPath)
		dir := filepath.Dir(absPath)

		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err, "Failed to create directory %s", dir)

		if content != "" {
			err = os.WriteFile(absPath, []byte(content), 0644)
			require.NoError(t, err, "Failed to write file %s", absPath)
		} else {
			// If content is empty, ensure the directory itself is created if it's a top-level entry
			// os.MkdirAll above handles nested dirs, but this ensures an empty entry like "somedir":"" works
			if _, err := os.Stat(absPath); os.IsNotExist(err) {
				err = os.MkdirAll(absPath, 0755)
				require.NoError(t, err, "Failed to create directory for empty content: %s", absPath)
			}
		}
	}
	return rootDir
}

func getDefaultTestConfig() Config {
	return Config{
		IncludeTree:                    true,
		SkipAuxFiles:                   false,
		DefaultExcludeDirs:             appconfig.GetDefaultExcludedDirs(),
		DefaultMediaExts:               appconfig.GetDefaultMediaExtensions(),
		DefaultArchiveExts:             appconfig.GetDefaultArchiveExtensions(),
		DefaultExecExts:                appconfig.GetDefaultExecutableExtensions(),
		DefaultLockfilePatterns:        appconfig.GetDefaultLockfilePatterns(),
		DefaultMiscellaneousFileNames:  appconfig.GetDefaultMiscellaneousFileNames(),
		DefaultMiscellaneousExtensions: appconfig.GetDefaultMiscellaneousExtensions(),
		DefaultAuxExts:                 appconfig.GetDefaultAuxFileExtensions(),
		MaxFileSize:                    1 * 1024 * 1024, // 1MB
	}
}

func TestNewProcessor_LocalPath_Success(t *testing.T) {
	structure := map[string]string{
		"testproject/file1.txt": "content1",
		"testproject/subdir/file2.txt": "content2",
	}
	rootDir := createTestDirStructure(t, structure)
	testProjectPath := filepath.Join(rootDir, "testproject")

	absTestProjectPath, err := filepath.Abs(testProjectPath)
	require.NoError(t, err)

	originalCwd, err := os.Getwd()
	require.NoError(t, err)

	t.Run("empty OutputFile", func(t *testing.T) {
		cfg := getDefaultTestConfig()
		cfg.SourcePath = testProjectPath

		p, err := New(cfg)
		require.NoError(t, err)
		// Manually call internal setup methods after New() as per current design
		// This is needed because New() is minimal.
		err = p.setupInitialPaths()
		require.NoError(t, err, "p.setupInitialPaths() failed for empty OutputFile")
		err = p.determineOutputFileAndInitFilter()
		require.NoError(t, err, "p.determineOutputFileAndInitFilter() failed for empty OutputFile")
		
		assert.NotNil(t, p)
		assert.Equal(t, absTestProjectPath, p.basePath, "basePath should be absolute path to testproject")
		assert.Equal(t, "testproject", p.repoName, "repoName should be 'testproject'")
		assert.False(t, p.isTempRepo, "isTempRepo should be false for local paths")
		assert.NotNil(t, p.filter, "filter should be initialized")

		expectedOutputFile := filepath.Join(originalCwd, "testproject.txt")
		absExpectedOutputFile, _ := filepath.Abs(expectedOutputFile)
		assert.Equal(t, absExpectedOutputFile, p.finalOutputFile, "finalOutputFile name mismatch")
		assert.Equal(t, absExpectedOutputFile, p.filter.GetAbsFinalOutputFilePath(), "filter.absFinalOutputFilePath mismatch")
	})

	t.Run("relative OutputFile", func(t *testing.T) {
		cfg := getDefaultTestConfig()
		cfg.SourcePath = testProjectPath
		cfg.OutputFile = "custom.txt"

		p, err := New(cfg)
		require.NoError(t, err)
		err = p.setupInitialPaths()
		require.NoError(t, err, "p.setupInitialPaths() failed for relative OutputFile")
		err = p.determineOutputFileAndInitFilter()
		require.NoError(t, err, "p.determineOutputFileAndInitFilter() failed for relative OutputFile")

		assert.NotNil(t, p)
		expectedOutputFile := filepath.Join(originalCwd, "custom.txt")
		absExpectedOutputFile, _ := filepath.Abs(expectedOutputFile)
		assert.Equal(t, absExpectedOutputFile, p.finalOutputFile, "finalOutputFile name mismatch")
		assert.Equal(t, absExpectedOutputFile, p.filter.GetAbsFinalOutputFilePath(), "filter.absFinalOutputFilePath mismatch")
	})

	t.Run("absolute OutputFile", func(t *testing.T) {
		cfg := getDefaultTestConfig()
		cfg.SourcePath = testProjectPath
		tempOutDir := createTestDirStructure(t, map[string]string{})
		absCustomOutputFilePath := filepath.Join(tempOutDir, "abs_output.txt")
		cfg.OutputFile = absCustomOutputFilePath

		p, err := New(cfg)
		require.NoError(t, err)
		err = p.setupInitialPaths()
		require.NoError(t, err, "p.setupInitialPaths() failed for absolute OutputFile")
		err = p.determineOutputFileAndInitFilter()
		require.NoError(t, err, "p.determineOutputFileAndInitFilter() failed for absolute OutputFile")
		
		assert.NotNil(t, p)
		assert.Equal(t, absCustomOutputFilePath, p.finalOutputFile, "finalOutputFile name mismatch for absolute path")
		assert.Equal(t, absCustomOutputFilePath, p.filter.GetAbsFinalOutputFilePath(), "filter.absFinalOutputFilePath mismatch for absolute path")
	})
}

func TestNewProcessor_LocalPath_SourceIsDirFile(t *testing.T) {
	structure := map[string]string{
		"testproject/somefile.txt": "content",
	}
	rootDir := createTestDirStructure(t, structure)
	filePath := filepath.Join(rootDir, "testproject", "somefile.txt")

	cfg := getDefaultTestConfig()
	cfg.SourcePath = filePath

	p := &Processor{config: cfg}
	err := p.setupInitialPaths() // Error should occur here
	assert.Error(t, err, "Expected an error when SourcePath is a file")
	if err != nil {
		assert.Contains(t, err.Error(), "source path is not a directory", "Error message mismatch")
	}
}

func TestNewProcessor_LocalPath_SourceDoesNotExist(t *testing.T) {
	cfg := getDefaultTestConfig()
	cfg.SourcePath = filepath.Join("non", "existent", "path")

	p := &Processor{config: cfg}
	err := p.setupInitialPaths() // Error should occur here
	assert.Error(t, err, "Expected an error when SourcePath does not exist")
}

func TestNewProcessor_OutputNameFromCurrentDir(t *testing.T) {
	tempTestDir := createTestDirStructure(t, map[string]string{"file.txt": "dummy"})
	originalCwd, err := os.Getwd()
	require.NoError(t, err)

	err = os.Chdir(tempTestDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Chdir(originalCwd)
	})

	currentDirName := filepath.Base(tempTestDir)
	cfg := getDefaultTestConfig()
	cfg.SourcePath = "." 
	cfg.OutputFile = ""  

	p, err := New(cfg)
	require.NoError(t, err)
	err = p.setupInitialPaths()
	require.NoError(t, err)
	err = p.determineOutputFileAndInitFilter()
	require.NoError(t, err)

	assert.NotNil(t, p)
	absTempTestDir, _ := filepath.Abs(tempTestDir) 
	assert.Equal(t, absTempTestDir, p.basePath, "basePath should be the current dir")
	assert.Equal(t, currentDirName, p.repoName, "repoName should be current directory name")
	
	expectedOutputFileInNewCwd := filepath.Join(tempTestDir, fmt.Sprintf("%s.txt", currentDirName))
	absExpectedOutputFile, _ := filepath.Abs(expectedOutputFileInNewCwd)
	assert.Equal(t, absExpectedOutputFile, p.finalOutputFile, "finalOutputFile name mismatch")
}

var originalCloneRepoFunc func(repoURL, ref string) (string, string, error)

func setupMockGitClone(t *testing.T, mockRepoPath, mockRepoName string, mockErr error) {
	t.Helper()
	if originalCloneRepoFunc == nil { 
		originalCloneRepoFunc = gitutils.CloneRepoFunc
	}
	gitutils.CloneRepoFunc = func(repoURL, ref string) (string, string, error) {
		return mockRepoPath, mockRepoName, mockErr
	}
	t.Cleanup(func() {
		gitutils.CloneRepoFunc = originalCloneRepoFunc
		originalCloneRepoFunc = nil 
	})
}

func TestNewProcessor_GitURL_Success(t *testing.T) {
	parentTempDirForClone := createTestDirStructure(t, nil) 
	mockRepoName := "clonedtestrepo"
	mockActualClonedPath := filepath.Join(parentTempDirForClone, mockRepoName)
	
	mockClonedStructure := map[string]string{
		"fileA.go": "package main",
		"README.md": "# Test Repo",
	}
	for relPath, content := range mockClonedStructure {
		absPath := filepath.Join(mockActualClonedPath, relPath)
		dir := filepath.Dir(absPath)
		_ = os.MkdirAll(dir, 0755)
		_ = os.WriteFile(absPath, []byte(content), 0644)
	}

	setupMockGitClone(t, mockActualClonedPath, mockRepoName, nil)

	cfg := getDefaultTestConfig()
	cfg.SourcePath = "https://example.com/test/clonedtestrepo.git" 
	cfg.OutputFile = "" 

	p := &Processor{config: cfg, gitIgnoreCache: make(map[string]*gitignore.GitIgnore)} // Corrected map type
	err := p.setupInitialPaths()
	require.NoError(t, err, "p.setupInitialPaths() failed")
	err = p.determineOutputFileAndInitFilter()
	require.NoError(t, err, "p.determineOutputFileAndInitFilter() failed")

	assert.Equal(t, mockActualClonedPath, p.basePath, "basePath should be the mocked cloned path")
	assert.Equal(t, mockRepoName, p.repoName, "repoName should be the mocked repo name")
	assert.True(t, p.isTempRepo, "isTempRepo should be true for Git URLs")
	assert.Equal(t, parentTempDirForClone, p.tempRepoDir, "tempRepoDir should be the parent of mockActualClonedPath")
	assert.NotNil(t, p.filter, "filter should be initialized")

	cwd, _ := os.Getwd()
	expectedOutputFile := filepath.Join(cwd, mockRepoName+".txt")
	absExpectedOutputFile, _ := filepath.Abs(expectedOutputFile)
	assert.Equal(t, absExpectedOutputFile, p.finalOutputFile, "finalOutputFile name mismatch")
}

func TestNewProcessor_GitURL_CloneFails(t *testing.T) {
	expectedCloneError := "mock clone error"
	setupMockGitClone(t, "", "", fmt.Errorf(expectedCloneError))

	cfg := getDefaultTestConfig()
	cfg.SourcePath = "https://example.com/test/failclone.git"

	p := &Processor{config: cfg, gitIgnoreCache: make(map[string]*gitignore.GitIgnore)} // Corrected map type
	err := p.setupInitialPaths() 

	require.Error(t, err, "Expected an error from setupInitialPaths due to clone failure")
	assert.Contains(t, err.Error(), expectedCloneError, "Error message should contain the mock clone error")
	assert.Contains(t, err.Error(), "failed to clone repository", "Error message should indicate clone failure context")
	assert.Empty(t, p.basePath, "basePath should be empty on clone failure")
	assert.Empty(t, p.repoName, "repoName should be empty on clone failure")
	assert.False(t, p.isTempRepo, "isTempRepo should be false or unset on clone failure")
}

// PROCESS METHOD TESTS - LOCAL PATHS
// ==================================
// (Existing Process method tests: TestProcess_LocalPath_Basic, NoTree, WithFilters, SkipAuxFiles, OutputSelfExclusion, Gitignore_Basic, Gitignore_Nested, Gitignore_DirOnlyRule)
// These tests should largely remain the same, ensuring gitIgnoreCache is initialized with *gitignore.GitIgnore
// Example adjustment for one test:
func TestProcess_LocalPath_Basic(t *testing.T) {
	structure := map[string]string{
		"testdata/fileA.txt":      "Content A",
		"testdata/sub/fileB.md":   "Content B",
		"testdata/sub/empty_dir": "", 
	}
	sourceDir := createTestDirStructure(t, structure)
	testDataSourceDir := filepath.Join(sourceDir, "testdata")

	cfg := getDefaultTestConfig()
	cfg.SourcePath = testDataSourceDir
	cfg.IncludeTree = true

	outputTempDir := t.TempDir()
	cfg.OutputFile = filepath.Join(outputTempDir, "output.txt")

	p, err := New(cfg) // New now initializes gitIgnoreCache
	require.NoError(t, err, "New(cfg) failed")
	err = p.setupInitialPaths()
	require.NoError(t, err, "p.setupInitialPaths() failed")
	err = p.determineOutputFileAndInitFilter()
	require.NoError(t, err, "p.determineOutputFileAndInitFilter() failed")

	err = p.Process()
	require.NoError(t, err, "p.Process() failed")

	outputContentBytes, err := os.ReadFile(p.GetFinalOutputFile())
	require.NoError(t, err, "Failed to read output file")
	outputContent := string(outputContentBytes)

	expectedTree := `testdata
├── fileA.txt
└── sub
    ├── empty_dir
    └── fileB.md`
	assert.Contains(t, outputContent, expectedTree, "Output should contain the file tree")
	normalizedFileAContent := strings.ReplaceAll("```testdata/fileA.txt\nContent A\n```", "testdata/", "") 
	assert.Contains(t, outputContent, normalizedFileAContent, "Output should contain content of fileA.txt")
	normalizedFileBContent := strings.ReplaceAll("```testdata/sub/fileB.md\nContent B\n```", "testdata/", "")
	assert.Contains(t, outputContent, normalizedFileBContent, "Output should contain content of fileB.md")
	assert.NotContains(t, outputContent, "\\", "Output should use forward slashes for paths")
}

// ... (Other Process tests would follow a similar pattern of ensuring New initializes the cache) ...
// TestProcess_LocalPath_NoTree, TestProcess_LocalPath_WithFilters, TestProcess_LocalPath_SkipAuxFiles,
// TestProcess_OutputSelfExclusion, TestProcess_LocalPath_Gitignore_Basic,
// TestProcess_LocalPath_Gitignore_Nested, TestProcess_LocalPath_Gitignore_DirOnlyRule
// need to be here. For brevity, I'm showing one and assuming the rest are present from Turn 29/33's context.

func TestProcess_LocalPath_NoTree(t *testing.T) {
	structure := map[string]string{
		"testdata/fileA.txt":    "Content A",
		"testdata/sub/fileB.md": "Content B",
	}
	sourceDir := createTestDirStructure(t, structure)
	testDataSourceDir := filepath.Join(sourceDir, "testdata")

	cfg := getDefaultTestConfig()
	cfg.SourcePath = testDataSourceDir
	cfg.IncludeTree = false 

	outputTempDir := t.TempDir()
	cfg.OutputFile = filepath.Join(outputTempDir, "output_no_tree.txt")

	p, err := New(cfg)
	require.NoError(t, err)
	err = p.setupInitialPaths()
	require.NoError(t, err, "p.setupInitialPaths() failed")
	err = p.determineOutputFileAndInitFilter()
	require.NoError(t, err, "p.determineOutputFileAndInitFilter() failed")

	err = p.Process()
	require.NoError(t, err)

	outputContentBytes, err := os.ReadFile(p.GetFinalOutputFile())
	require.NoError(t, err)
	outputContent := string(outputContentBytes)

	assert.NotContains(t, outputContent, "testdata\n├──", "Output should NOT contain the file tree")
	assert.NotContains(t, outputContent, "└──", "Output should NOT contain the file tree")
	assert.Contains(t, outputContent, "```fileA.txt\nContent A\n```", "Output should contain content of fileA.txt")
	assert.Contains(t, outputContent, "```sub/fileB.md\nContent B\n```", "Output should contain content of fileB.md")
}

func TestProcess_LocalPath_WithFilters(t *testing.T) {
	structure := map[string]string{
		"testproject/main.go":        "package main",
		"testproject/data.json":      `{"key": "value"}`,
		"testproject/docs/manual.txt": "manual",
	}
	sourceDir := createTestDirStructure(t, structure)
	testProjectSourceDir := filepath.Join(sourceDir, "testproject")

	cfg := getDefaultTestConfig()
	cfg.SourcePath = testProjectSourceDir
	cfg.IncludeTree = true
	cfg.UserExcludeExts = []string{".json"} 
	cfg.UserExcludeDirs = []string{"docs"}    

	outputTempDir := t.TempDir()
	cfg.OutputFile = filepath.Join(outputTempDir, "output_filtered.txt")

	p, err := New(cfg)
	require.NoError(t, err)
	err = p.setupInitialPaths(); require.NoError(t, err)
	err = p.determineOutputFileAndInitFilter(); require.NoError(t, err)
	err = p.Process(); require.NoError(t, err)
	outputContentBytes, err := os.ReadFile(p.GetFinalOutputFile()); require.NoError(t, err)
	outputContent := string(outputContentBytes)

	expectedTree := `testproject
└── main.go` 
	assert.Contains(t, outputContent, expectedTree)
	assert.NotContains(t, outputContent, "data.json")
	assert.NotContains(t, outputContent, "manual.txt")
	assert.Contains(t, outputContent, "```main.go\npackage main\n```")
	assert.NotContains(t, outputContent, "```data.json")
}


func TestProcess_LocalPath_SkipAuxFiles(t *testing.T) {
	structure := map[string]string{
		"testproject/main.go":        "package main",
		"testproject/README.md":      "readme content", 
	}
	sourceDir := createTestDirStructure(t, structure)
	testProjectSourceDir := filepath.Join(sourceDir, "testproject")

	cfg := getDefaultTestConfig()
	cfg.SourcePath = testProjectSourceDir
	cfg.IncludeTree = true
	cfg.SkipAuxFiles = true 

	outputTempDir := t.TempDir()
	cfg.OutputFile = filepath.Join(outputTempDir, "output_skip_aux.txt")

	p, err := New(cfg)
	require.NoError(t, err)
	err = p.setupInitialPaths(); require.NoError(t, err)
	err = p.determineOutputFileAndInitFilter(); require.NoError(t, err)
	err = p.Process(); require.NoError(t, err)
	outputContentBytes, err := os.ReadFile(p.GetFinalOutputFile()); require.NoError(t, err)
	outputContent := string(outputContentBytes)
	
	expectedTree := `testproject
└── main.go`
	assert.Contains(t, outputContent, expectedTree)
	assert.NotContains(t, outputContent, "README.md")
	assert.Contains(t, outputContent, "```main.go\npackage main\n```")
	assert.NotContains(t, outputContent, "```README.md")
}


func TestProcess_OutputSelfExclusion(t *testing.T) {
	tempCwd := t.TempDir()
	originalCwd, _ := os.Getwd()
	_ = os.Chdir(tempCwd)
	t.Cleanup(func() { _ = os.Chdir(originalCwd) })

	_ = os.WriteFile(filepath.Join(tempCwd, "somefile.txt"), []byte("this is some file"), 0644)
	
	cfg := getDefaultTestConfig()
	cfg.SourcePath = "." 
	cfg.OutputFile = "output.txt" 
	cfg.IncludeTree = true

	p, err := New(cfg)
	require.NoError(t, err)
	err = p.setupInitialPaths(); require.NoError(t, err)
	err = p.determineOutputFileAndInitFilter(); require.NoError(t, err)
	err = p.Process(); require.NoError(t, err)
	outputContentBytes, err := os.ReadFile(p.GetFinalOutputFile()); require.NoError(t, err)
	outputContent := string(outputContentBytes)

	currentDirName := filepath.Base(tempCwd)
	assert.NotContains(t, outputContent, "output.txt")
	assert.Contains(t, outputContent, "```somefile.txt\nthis is some file\n```")
	expectedTreePrefix := fmt.Sprintf("%s\n└── somefile.txt", currentDirName)
	assert.Contains(t, outputContent, expectedTreePrefix)
}

func TestProcess_LocalPath_Gitignore_Basic(t *testing.T) {
	structure := map[string]string{
		"testproject_gitignore1/.gitignore": "*.log\nignored_dir/",
		"testproject_gitignore1/fileA.txt": "Content A",
		"testproject_gitignore1/fileB.log": "Log B",
		"testproject_gitignore1/ignored_dir/fileD.txt": "Content D",
	}
	sourceRoot := createTestDirStructure(t, structure)
	testDataSourceDir := filepath.Join(sourceRoot, "testproject_gitignore1")

	cfg := getDefaultTestConfig()
	cfg.SourcePath = testDataSourceDir
	outputTempDir := t.TempDir()
	cfg.OutputFile = filepath.Join(outputTempDir, "output_gitignore_basic.txt")

	p, err := New(cfg); require.NoError(t, err)
	err = p.setupInitialPaths(); require.NoError(t, err)
	err = p.determineOutputFileAndInitFilter(); require.NoError(t, err)
	err = p.Process(); require.NoError(t, err)
	outputContentBytes, err := os.ReadFile(p.GetFinalOutputFile()); require.NoError(t, err)
	outputContent := string(outputContentBytes)

	expectedTree := `testproject_gitignore1
├── .gitignore
└── fileA.txt`
	assert.Contains(t, outputContent, expectedTree)
	assert.NotContains(t, outputContent, "fileB.log")
	assert.NotContains(t, outputContent, "ignored_dir")
	assert.Contains(t, outputContent, "```.gitignore\n*.log\nignored_dir/\n```")
	assert.Contains(t, outputContent, "```fileA.txt\nContent A\n```")
	assert.NotContains(t, outputContent, "```fileB.log")
}

func TestProcess_LocalPath_Gitignore_Nested(t *testing.T) {
	structure := map[string]string{
		"p/.gitignore":          "*.log",
		"p/fileA.txt":           "A",
		"p/sub/.gitignore":      "!important.log\n*.txt",
		"p/sub/important.log":   "IL",
		"p/sub/fileC.md":        "C",
		"p/sub/other.txt":		 "OT",
	}
	sourceRoot := createTestDirStructure(t, structure)
	testDataSourceDir := filepath.Join(sourceRoot, "p")
	cfg := getDefaultTestConfig(); cfg.SourcePath = testDataSourceDir
	cfg.OutputFile = filepath.Join(t.TempDir(), "out.txt")

	p, err := New(cfg); require.NoError(t, err)
	err = p.setupInitialPaths(); require.NoError(t, err)
	err = p.determineOutputFileAndInitFilter(); require.NoError(t, err)
	err = p.Process(); require.NoError(t, err)
	outputContent, _ := os.ReadFile(p.GetFinalOutputFile())
	
	assert.Contains(t, string(outputContent), "p\n├── .gitignore\n├── fileA.txt\n└── sub\n    ├── .gitignore\n    ├── fileC.md\n    └── important.log")
	assert.NotContains(t, string(outputContent), "other.txt")
}

func TestProcess_LocalPath_Gitignore_DirOnlyRule(t *testing.T) {
	structure := map[string]string{
		"p/.gitignore": "cache/\nfile.ignore",
		"p/fileA.txt":  "A",
		"p/cache/a.txt": "in cache",
		"p/file.ignore": "ignored file",
	}
	sourceRoot := createTestDirStructure(t, structure)
	testDataSourceDir := filepath.Join(sourceRoot, "p")
	cfg := getDefaultTestConfig(); cfg.SourcePath = testDataSourceDir
	cfg.OutputFile = filepath.Join(t.TempDir(), "out.txt")

	p, err := New(cfg); require.NoError(t, err)
	err = p.setupInitialPaths(); require.NoError(t, err)
	err = p.determineOutputFileAndInitFilter(); require.NoError(t, err)
	err = p.Process(); require.NoError(t, err)
	outputContent, _ := os.ReadFile(p.GetFinalOutputFile())

	assert.Contains(t, string(outputContent), "p\n├── .gitignore\n└── fileA.txt")
	assert.NotContains(t, string(outputContent), "cache")
	assert.NotContains(t, string(outputContent), "file.ignore")
}


// PROCESS METHOD TESTS - GIT URLS
// ===============================
func TestProcess_GitURL_Success(t *testing.T) {
	parentTempDir := t.TempDir() 
	mockRepoName := "myClonedRepo"
	mockClonedPath := filepath.Join(parentTempDir, mockRepoName) 
	structure := map[string]string{
		"main.go":   "package main",
		"README.md": "# Readme",
	}
	for relPath, content := range structure {
		absPath := filepath.Join(mockClonedPath, relPath)
		_ = os.MkdirAll(filepath.Dir(absPath), 0755)
		_ = os.WriteFile(absPath, []byte(content), 0644)
	}
	setupMockGitClone(t, mockClonedPath, mockRepoName, nil)
	cfg := getDefaultTestConfig()
	cfg.SourcePath = "https://example.com/u/" + mockRepoName + ".git"
	cfg.OutputFile = filepath.Join(t.TempDir(), "out_git.txt")

	p, err := New(cfg); require.NoError(t, err)
	err = p.setupInitialPaths(); require.NoError(t, err)
	err = p.determineOutputFileAndInitFilter(); require.NoError(t, err)
	err = p.Process(); require.NoError(t, err)
	outputContentBytes, _ := os.ReadFile(p.GetFinalOutputFile())
	outputContent := string(outputContentBytes)

	expectedTree := mockRepoName + "\n├── README.md\n└── main.go"
	assert.Contains(t, outputContent, expectedTree)
	assert.Contains(t, outputContent, "```README.md\n# Readme\n```")
	assert.Contains(t, outputContent, "```main.go\npackage main\n```")
	_, errClonedPathStat := os.Stat(mockClonedPath)
	assert.True(t, os.IsNotExist(errClonedPathStat))
	_, errParentDirStat := os.Stat(parentTempDir) 
	assert.True(t, os.IsNotExist(errParentDirStat))
}

// PROCESS METHOD TESTS - ERROR CONDITIONS
// =====================================
func TestProcess_Error_CreateTempOutputFileFails(t *testing.T) {
	sourceDir := createTestDirStructure(t, map[string]string{"file.txt": "content"})
	outputParentDir := t.TempDir()
	nonWritableDir := filepath.Join(outputParentDir, "target_output_dir")
	_ = os.Mkdir(nonWritableDir, 0755)
	cfg := getDefaultTestConfig()
	cfg.SourcePath = sourceDir
	cfg.OutputFile = filepath.Join(nonWritableDir, "output.txt") 

	p, err := New(cfg); require.NoError(t, err)
	err = p.setupInitialPaths(); require.NoError(t, err)
	err = p.determineOutputFileAndInitFilter(); require.NoError(t, err)
	
	_ = os.Chmod(nonWritableDir, 0400) 
	t.Cleanup(func() { _ = os.Chmod(nonWritableDir, 0755) })
	err = p.Process()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create temporary output file")
}

func TestProcess_Error_WalkDirFails_AccessDeniedToSubDir(t *testing.T) {
	sourceStructure := map[string]string{
		"tp/fileA.txt": "A",
		"tp/inaccessible/fileB.txt": "B",
		"tp/another.md": "C",
	}
	sourceRoot := createTestDirStructure(t, sourceStructure)
	testDataSourceDir := filepath.Join(sourceRoot, "tp")
	inaccessiblePath := filepath.Join(testDataSourceDir, "inaccessible")

	cfg := getDefaultTestConfig()
	cfg.SourcePath = testDataSourceDir
	cfg.OutputFile = filepath.Join(t.TempDir(), "out_walk_err.txt")

	p, err := New(cfg); require.NoError(t, err)
	err = p.setupInitialPaths(); require.NoError(t, err)
	err = p.determineOutputFileAndInitFilter(); require.NoError(t, err)

	originalPerms, errStat := os.Stat(inaccessiblePath)
	if errStat == nil { 
		_ = os.Chmod(inaccessiblePath, 0000)
		t.Cleanup(func() { _ = os.Chmod(inaccessiblePath, originalPerms.Mode().Perm()) })
	} else {
		t.Logf("Skipping Chmod on inaccessiblePath: %v", errStat)
	}
	err = p.Process()
	require.NoError(t, err) 
	outputContentBytes, _ := os.ReadFile(p.GetFinalOutputFile())
	outputContent := string(outputContentBytes)

	assert.Contains(t, outputContent, "```another.md\nC\n```")
	assert.Contains(t, outputContent, "```fileA.txt\nA\n```")
	assert.NotContains(t, outputContent, "inaccessible")
	assert.NotContains(t, outputContent, "Content B")
	expectedTree := `tp
├── another.md
└── fileA.txt`
	assert.Contains(t, outputContent, expectedTree)
}

[end of internal/processor/processor_test.go]
