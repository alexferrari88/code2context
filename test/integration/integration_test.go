package integration

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var c2cBinaryPath string

func TestMain(m *testing.M) {
	binaryName := "test_c2c_binary"
	cmd := exec.Command("go", "build", "-o", binaryName, "../../main.go")
	buildOutput, err := cmd.CombinedOutput() 
	if err != nil {
		os.Stderr.WriteString("Failed to build c2c binary for integration tests:\n" + string(buildOutput) + "\nError: " + err.Error() + "\n")
		os.Exit(1)
	}

	absPath, err := filepath.Abs(binaryName)
	if err != nil {
		os.Stderr.WriteString("Failed to get absolute path for test binary: " + err.Error() + "\n")
		os.Remove(binaryName) 
		os.Exit(1)
	}
	c2cBinaryPath = absPath

	exitCode := m.Run()

	err = os.Remove(c2cBinaryPath)
	if err != nil {
		os.Stderr.WriteString("Warning: Failed to remove test_c2c_binary: " + err.Error() + "\n")
	}

	os.Exit(exitCode)
}

// runC2C executes the c2c binary with the given arguments.
// workDir: if not empty, sets the command's working directory.
// args: arguments to pass to the c2c binary.
// Returns stdout, stderr, and any execution error.
func runC2C(t *testing.T, workDir string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(c2cBinaryPath, args...)
	if workDir != "" {
		cmd.Dir = workDir
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run() 

	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()

	if err != nil || (!strings.Contains(strings.Join(args, " "), "https://github.com/git-fixtures/basic.git") && stderrStr != "") { 
		// For remote URL tests, git might print to stderr (e.g. progress), so don't log just for any stderr.
		// Log if error, or if it's not a remote test and stderr is present.
		t.Logf("runC2C results (workDir: %q):\nArgs: %v\nError: %v\nStdout: %s\nStderr: %s", workDir, args, err, stdoutStr, stderrStr)
	}


	return stdoutStr, stderrStr, err
}

func createTestProject(t *testing.T, name string, files map[string]string) string {
	t.Helper()
	projectRoot, err := os.MkdirTemp(t.TempDir(), name+"_*")
	require.NoError(t, err, "Failed to create temp project root dir for "+name)

	for relPath, content := range files {
		absPath := filepath.Join(projectRoot, relPath)
		dir := filepath.Dir(absPath)

		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err, "Failed to create directory %s in test project %s", dir, name)

		if content != "" { 
			err = os.WriteFile(absPath, []byte(content), 0644)
			require.NoError(t, err, "Failed to write file %s in test project %s", absPath, name)
		} else { 
			if _, statErr := os.Stat(absPath); os.IsNotExist(statErr) {
				err = os.MkdirAll(absPath, 0755)
				require.NoError(t, err, "Failed to create directory for empty content: %s in test project %s", absPath, name)
			}
		}
	}
	return projectRoot
}

func TestIntegration_BasicLocalPath(t *testing.T) {
	projectName := "simple_project"
	projectFiles := map[string]string{
		"fileA.txt": "Content of fileA",
		"subDir/fileB.go": `package main

import "fmt"

func main(){
	fmt.Println("Hello from fileB")
}`,
		"subDir/emptySubSubDir/": "", 
	}

	projectPath := createTestProject(t, projectName, projectFiles)
	outputDir := t.TempDir()
	outputFilePath := filepath.Join(outputDir, "context_output.txt")

	args := []string{projectPath, "--output", outputFilePath, "--tree"}
	stdout, stderr, err := runC2C(t, "", args...) 

	require.NoError(t, err, "c2c execution failed")
	assert.Empty(t, stderr, "stderr should be empty for successful execution")
	assert.Empty(t, stdout, "stdout should be empty for basic successful execution")

	outputContentBytes, readErr := os.ReadFile(outputFilePath)
	require.NoError(t, readErr, "Failed to read output file")
	outputContent := string(outputContentBytes)

	assert.Contains(t, outputContent, filepath.Base(projectPath), "Tree should contain project root")
	assert.Contains(t, outputContent, "├── fileA.txt", "Tree should list fileA.txt")
	assert.Contains(t, outputContent, "└── subDir", "Tree should list subDir")
	assert.Contains(t, outputContent, "    ├── emptySubSubDir", "Tree should list emptySubSubDir")
	assert.Contains(t, outputContent, "    └── fileB.go", "Tree should list fileB.go")
	
	assert.Contains(t, outputContent, "```fileA.txt\nContent of fileA\n```", "Output should contain content of fileA.txt")
	assert.Contains(t, outputContent, "```subDir/fileB.go\npackage main\n\nimport \"fmt\"\n\nfunc main(){\n\tfmt.Println(\"Hello from fileB\")\n}\n```", "Output should contain content of subDir/fileB.go")
	assert.NotContains(t, outputContent, "\\", "Output should use forward slashes for paths")
}

func TestIntegration_NoTree(t *testing.T) {
	projectName := "no_tree_project"
	projectFiles := map[string]string{
		"main.py": "print('Hello')",
		"util.py": "def helper(): pass",
	}
	projectPath := createTestProject(t, projectName, projectFiles)
	outputDir := t.TempDir()
	outputFilePath := filepath.Join(outputDir, "no_tree_output.txt")

	args := []string{projectPath, "--output", outputFilePath, "--no-tree"}
	stdout, stderr, err := runC2C(t, "", args...)

	require.NoError(t, err, "c2c execution with --no-tree failed")
	assert.Empty(t, stderr, "stderr should be empty for --no-tree")
	assert.Empty(t, stdout, "stdout should be empty for --no-tree")

	outputContentBytes, readErr := os.ReadFile(outputFilePath)
	require.NoError(t, readErr, "Failed to read output file for --no-tree test")
	outputContent := string(outputContentBytes)

	assert.NotContains(t, outputContent, filepath.Base(projectPath)+"\n", "Output should not start with project name as a tree root")
	assert.NotContains(t, outputContent, "├──", "Output should not contain tree drawing characters")
	assert.NotContains(t, outputContent, "└──", "Output should not contain tree drawing characters")

	assert.Contains(t, outputContent, "```main.py\nprint('Hello')\n```", "Output should contain main.py content")
	assert.Contains(t, outputContent, "```util.py\ndef helper(): pass\n```", "Output should contain util.py content")
	assert.NotContains(t, outputContent, "\\", "Output should use forward slashes for paths")
}

func TestIntegration_Exclusions(t *testing.T) {
	projectName := "exclusions_project"
	projectFiles := map[string]string{
		"fileA.txt":                 "Content A",
		"main.go":                   "package main",
		"docs/guide.md":             "Guide content",
		"temp_data.log":             "Log data",
		"archive/old_temp_data.zip": "zip content", 
		"another_temp.json":         "json data",   
	}
	projectPath := createTestProject(t, projectName, projectFiles)
	outputDir := t.TempDir()
	outputFilePath := filepath.Join(outputDir, "exclusions_output.txt")

	args := []string{
		projectPath,
		"--output", outputFilePath,
		"--tree",
		"--exclude-dirs", "docs,archive",
		"--exclude-exts", ".log", 
		"--exclude-patterns", "*_temp.*", 
	}
	stdout, stderr, err := runC2C(t, "", args...)

	require.NoError(t, err, "c2c execution with exclusions failed")
	assert.Empty(t, stderr, "stderr should be empty for exclusions test")
	assert.Empty(t, stdout, "stdout should be empty for exclusions test")

	outputContentBytes, readErr := os.ReadFile(outputFilePath)
	require.NoError(t, readErr, "Failed to read output file for exclusions test")
	outputContent := string(outputContentBytes)
	
	assert.Contains(t, outputContent, filepath.Base(projectPath), "Tree should contain project root")
	assert.Contains(t, outputContent, "├── fileA.txt", "Tree should list fileA.txt")
	assert.Contains(t, outputContent, "└── main.go", "Tree should list main.go")
	
	assert.NotContains(t, outputContent, "docs", "Tree should not list excluded dir 'docs'")
	assert.NotContains(t, outputContent, "archive", "Tree should not list excluded dir 'archive'")
	assert.NotContains(t, outputContent, "guide.md", "guide.md should be excluded by dir exclusion")
	assert.NotContains(t, outputContent, "old_temp_data.zip", "old_temp_data.zip should be excluded by dir exclusion")
	assert.NotContains(t, outputContent, "temp_data.log", "temp_data.log should be excluded by ext or pattern")
	assert.NotContains(t, outputContent, "another_temp.json", "another_temp.json should be excluded by pattern")

	assert.Contains(t, outputContent, "```fileA.txt\nContent A\n```")
	assert.Contains(t, outputContent, "```main.go\npackage main\n```")
	assert.NotContains(t, outputContent, "\\", "Output should use forward slashes for paths")
}

func TestIntegration_Gitignore(t *testing.T) {
	projectName := "gitignore_project"
	projectFiles := map[string]string{
		"fileA.txt":         "Content A",
		"secret.key":        "Secret stuff",
		".gitignore":        "*.key\nlogs/\nsub/fileB.txt", 
		"logs/app.log":      "Log content",
		"sub/fileB.txt":     "Content B", 
		"sub/.gitignore":    "*.txt\n!important.txt", 
		"sub/important.txt": "This is important",
		"sub/another.md":    "Another markdown in sub",
	}
	projectPath := createTestProject(t, projectName, projectFiles)
	outputDir := t.TempDir()
	outputFilePath := filepath.Join(outputDir, "gitignore_output.txt")

	args := []string{projectPath, "--output", outputFilePath, "--tree"}
	stdout, stderr, err := runC2C(t, "", args...)

	require.NoError(t, err, "c2c execution with .gitignore failed")
	assert.Empty(t, stderr, "stderr should be empty for .gitignore test")
	assert.Empty(t, stdout, "stdout should be empty for .gitignore test")

	outputContentBytes, readErr := os.ReadFile(outputFilePath)
	require.NoError(t, readErr, "Failed to read output file for .gitignore test")
	outputContent := string(outputContentBytes)

	assert.Contains(t, outputContent, filepath.Base(projectPath), "Tree should contain project root")
	assert.Contains(t, outputContent, "├── .gitignore", "Tree should list root .gitignore")
	assert.Contains(t, outputContent, "├── fileA.txt", "Tree should list fileA.txt")
	assert.Contains(t, outputContent, "└── sub", "Tree should list sub directory")
	assert.Contains(t, outputContent, "    ├── .gitignore", "Tree should list sub .gitignore")
	assert.Contains(t, outputContent, "    ├── another.md", "Tree should list sub/another.md")
	assert.Contains(t, outputContent, "    └── important.txt", "Tree should list sub/important.txt")

	assert.NotContains(t, outputContent, "secret.key", "secret.key should be excluded by .gitignore")
	assert.NotContains(t, outputContent, "logs", "logs directory should be excluded by .gitignore")
	assert.NotContains(t, outputContent, "app.log", "app.log should be excluded by .gitignore")
	assert.NotContains(t, outputContent, "sub/fileB.txt", "sub/fileB.txt should be excluded by root .gitignore")
	
	assert.Contains(t, outputContent, "```fileA.txt\nContent A\n```")
	assert.Contains(t, outputContent, "```.gitignore\n*.key\nlogs/\nsub/fileB.txt\n```")
	assert.Contains(t, outputContent, "```sub/.gitignore\n*.txt\n!important.txt\n```")
	assert.Contains(t, outputContent, "```sub/important.txt\nThis is important\n```")
	assert.Contains(t, outputContent, "```sub/another.md\nAnother markdown in sub\n```")
	assert.NotContains(t, outputContent, "\\", "Output should use forward slashes for paths")
}

func TestIntegration_SkipAuxFiles(t *testing.T) {
	projectName := "skip_aux_project"
	projectFiles := map[string]string{
		"main.go":        "package main",
		"README.md":      "Readme content",
		"data.json":      "{\"key\": \"value\"}",
		"notes.txt":      "My notes",
		"LICENSE":        "License text",
		"script.py":      "print('hello')", 
	}
	projectPath := createTestProject(t, projectName, projectFiles)
	outputDir := t.TempDir()
	outputFilePath := filepath.Join(outputDir, "skip_aux_output.txt")

	args := []string{projectPath, "--output", outputFilePath, "--tree", "--skip-aux-files"}
	stdout, stderr, err := runC2C(t, "", args...)

	require.NoError(t, err, "c2c execution with --skip-aux-files failed")
	assert.Empty(t, stderr, "stderr should be empty for --skip-aux-files test")
	assert.Empty(t, stdout, "stdout should be empty for --skip-aux-files test")

	outputContentBytes, readErr := os.ReadFile(outputFilePath)
	require.NoError(t, readErr, "Failed to read output file for --skip-aux-files test")
	outputContent := string(outputContentBytes)
	
	assert.Contains(t, outputContent, filepath.Base(projectPath), "Tree should contain project root")
	assert.Contains(t, outputContent, "├── main.go", "Tree should list main.go")
	assert.Contains(t, outputContent, "└── script.py", "Tree should list script.py")

	assert.NotContains(t, outputContent, "README.md", "README.md should be skipped")
	assert.NotContains(t, outputContent, "data.json", "data.json should be skipped")
	assert.NotContains(t, outputContent, "notes.txt", "notes.txt should be skipped")
	assert.NotContains(t, outputContent, "LICENSE", "LICENSE file should be skipped")

	assert.Contains(t, outputContent, "```main.go\npackage main\n```")
	assert.Contains(t, outputContent, "```script.py\nprint('hello')\n```")
	assert.NotContains(t, outputContent, "\\", "Output should use forward slashes for paths")
}

func TestIntegration_MaxFileSize(t *testing.T) {
	projectName := "max_filesize_project"
	projectFiles := map[string]string{
		"small.txt": "This is a small file.",         
		"large.txt": strings.Repeat("A", 1500), 
	}
	projectPath := createTestProject(t, projectName, projectFiles)
	outputDir := t.TempDir()
	outputFilePath := filepath.Join(outputDir, "max_filesize_output.txt")

	args := []string{projectPath, "--output", outputFilePath, "--tree", "--max-file-size=1KB"}
	stdout, stderr, err := runC2C(t, "", args...)

	require.NoError(t, err, "c2c execution with --max-file-size failed")
	assert.Empty(t, stderr, "stderr should be empty for --max-file-size test")
	assert.Empty(t, stdout, "stdout should be empty unless verbose logging of skipping is on by default")

	outputContentBytes, readErr := os.ReadFile(outputFilePath)
	require.NoError(t, readErr, "Failed to read output file for --max-file-size test")
	outputContent := string(outputContentBytes)

	assert.Contains(t, outputContent, filepath.Base(projectPath), "Tree should contain project root")
	assert.Contains(t, outputContent, "└── small.txt", "Tree should list small.txt")
	assert.NotContains(t, outputContent, "large.txt", "Tree should NOT list large.txt")

	assert.Contains(t, outputContent, "```small.txt\nThis is a small file.\n```")
	assert.NotContains(t, outputContent, "```large.txt", "Output should NOT contain content of large.txt")
	assert.NotContains(t, outputContent, "\\", "Output should use forward slashes for paths")
}

func TestIntegration_OutputNaming(t *testing.T) {
	t.Run("Specified Output File", func(t *testing.T) {
		projectName := "output_specified_project"
		projectFiles := map[string]string{"main.c": "int main() { return 0; }"}
		projectPath := createTestProject(t, projectName, projectFiles)
		
		outputDir := t.TempDir()
		specifiedOutputFile := filepath.Join(outputDir, "custom_out.txt")

		args := []string{projectPath, "-o", specifiedOutputFile}
		stdout, stderr, err := runC2C(t, "", args...)

		require.NoError(t, err, "c2c execution with specified output file failed")
		assert.Empty(t, stderr, "stderr should be empty")
		assert.Empty(t, stdout, "stdout should be empty")

		_, statErr := os.Stat(specifiedOutputFile)
		assert.NoError(t, statErr, "Specified output file should exist")

		outputContentBytes, readErr := os.ReadFile(specifiedOutputFile)
		require.NoError(t, readErr)
		outputContent := string(outputContentBytes)
		assert.Contains(t, outputContent, "```main.c\nint main() { return 0; }\n```")
	})

	t.Run("Default Output File", func(t *testing.T) {
		projectName := "output_default_project" 
		projectFiles := map[string]string{"app.js": "console.log('hello');"}
		projectPath := createTestProject(t, projectName, projectFiles) 
		
		outputDir := t.TempDir() 

		args := []string{projectPath} 
		stdout, stderr, err := runC2C(t, outputDir, args...) 

		require.NoError(t, err, "c2c execution for default output file failed")
		assert.Empty(t, stderr, "stderr should be empty")
		assert.Empty(t, stdout, "stdout should be empty")

		expectedDefaultOutputFile := filepath.Join(outputDir, filepath.Base(projectPath)+".txt")
		_, statErr := os.Stat(expectedDefaultOutputFile)
		assert.NoError(t, statErr, "Default output file %s should exist in the execution directory", expectedDefaultOutputFile)

		outputContentBytes, readErr := os.ReadFile(expectedDefaultOutputFile)
		require.NoError(t, readErr)
		outputContent := string(outputContentBytes)
		assert.Contains(t, outputContent, "```app.js\nconsole.log('hello');\n```")
	})
}

func TestIntegration_ErrorHandling_InvalidArgs(t *testing.T) {
	t.Run("No Arguments", func(t *testing.T) {
		stdout, stderr, err := runC2C(t, "", []string{}...) 
		
		assert.Error(t, err, "c2c should error with no arguments")
		assert.Contains(t, stderr, "Error: accepts 1 arg(s), received 0", "stderr should contain specific error for no args")
		assert.Contains(t, stderr, "Usage:", "stderr should contain Usage information")
		assert.Empty(t, stdout, "stdout should be empty on error")
	})

	t.Run("Non-Existent Path", func(t *testing.T) {
		nonExistentPath := filepath.Join(t.TempDir(), "this_path_does_not_exist")
		stdout, stderr, err := runC2C(t, "", nonExistentPath)
		
		assert.Error(t, err, "c2c should error with non-existent path")
		assert.Contains(t, stderr, "Error: processor: failed to stat source path", "stderr should indicate stat failure")
		assert.Contains(t, stderr, "no such file or directory", "stderr should indicate no such file or directory")
		assert.Empty(t, stdout, "stdout should be empty on error")
	})

	t.Run("Invalid Max File Size", func(t *testing.T) {
		projectPath := createTestProject(t, "dummy_project_maxfilesize_err", map[string]string{"f.txt":"content"})
		stdout, stderr, err := runC2C(t, "", projectPath, "--max-file-size=invalid")
		
		assert.Error(t, err, "c2c should error with invalid max-file-size")
		assert.Contains(t, stderr, "Error: invalid max file size: invalid file size format: 'invalid'", "stderr should indicate invalid max file size format")
		assert.Empty(t, stdout, "stdout should be empty on error")
	})
}

func TestIntegration_OutputSelfExclusionInPlace(t *testing.T) {
	projectName := "self_exclude_inplace"
	projectFiles := map[string]string{"fileA.txt": "Content A"}
	projectPath := createTestProject(t, projectName, projectFiles)

	outputFilePath := filepath.Join(projectPath, "output_in_src.txt") 

	args := []string{projectPath, "-o", outputFilePath, "--tree"}
	stdout, stderr, err := runC2C(t, "", args...)

	require.NoError(t, err, "c2c execution for self-exclusion test failed")
	assert.Empty(t, stderr, "stderr should be empty")
	assert.Empty(t, stdout, "stdout should be empty")

	outputContentBytes, readErr := os.ReadFile(outputFilePath)
	require.NoError(t, readErr, "Failed to read output file for self-exclusion test")
	outputContent := string(outputContentBytes)

	assert.Contains(t, outputContent, filepath.Base(projectPath), "Tree should contain project root")
	assert.Contains(t, outputContent, "└── fileA.txt", "Tree should list fileA.txt")
	assert.NotContains(t, outputContent, "output_in_src.txt", "Tree should NOT list the output file itself")
	
	assert.Contains(t, outputContent, "```fileA.txt\nContent A\n```")
	assert.NotContains(t, outputContent, "```output_in_src.txt", "Output should NOT contain its own content section")
	assert.NotContains(t, outputContent, "\\", "Output should use forward slashes for paths")
}

func TestIntegration_RemoteURL(t *testing.T) {
	repoURL := "https://github.com/git-fixtures/basic.git"
	outputDir := t.TempDir()

	// Test default branch
	t.Run("Default Branch", func(t *testing.T) {
		outputFilePathDefault := filepath.Join(outputDir, "output_git_default.txt")
		args := []string{repoURL, "--output", outputFilePathDefault, "--tree"}
		stdout, stderr, err := runC2C(t, "", args...)

		require.NoError(t, err, "c2c execution for remote URL (default branch) failed")
		// Git clone might print to stderr (e.g. "Cloning into 'basic'...")
		// Assert that if stderr is not empty, it contains typical git messages, not c2c errors.
		if stderr != "" {
			assert.Contains(t, stderr, "Cloning into", "stderr contains git clone messages")
		}
		assert.Empty(t, stdout, "stdout should be empty")

		outputContentBytes, readErr := os.ReadFile(outputFilePathDefault)
		require.NoError(t, readErr, "Failed to read output file for remote URL (default branch)")
		outputContent := string(outputContentBytes)

		// Verify tree (repo name is "basic")
		expectedTree := "basic\n" +
			"├── .gitattributes\n" +
			"├── .gitignore\n" +
			"├── CHANGELOG\n" +
			"└── go\n" +
			"    └── gofixture.go"
		assert.Contains(t, outputContent, expectedTree, "Tree structure mismatch for default branch")

		// Verify content of key files
		assert.Contains(t, outputContent, "```.gitattributes\n* text=auto\n```", "Expected .gitattributes content")
		assert.Contains(t, outputContent, "```.gitignore\n*.mode.*\n```", "Expected .gitignore content")
		assert.Contains(t, outputContent, "```CHANGELOG\nInitial changelog\n```", "Expected CHANGELOG content")
		expectedGoFixture := "```go/gofixture.go\n" +
			"package gofixture\n\n" +
			"import \"fmt\"\n\n" +
			"func Print() {\n" +
			"\tfmt.Println(\"This is a go fixture\")\n" +
			"}\n```"
		assert.Contains(t, outputContent, expectedGoFixture, "Expected go/gofixture.go content")
		assert.NotContains(t, outputContent, "\\", "Output should use forward slashes for paths")
	})

	// Test specific ref (tag v1.0.0)
	t.Run("Tag v1.0.0", func(t *testing.T) {
		outputFilePathTag := filepath.Join(outputDir, "output_git_tag.txt")
		args := []string{repoURL, "--ref", "v1.0.0", "--output", outputFilePathTag, "--tree"}
		stdout, stderr, err := runC2C(t, "", args...)

		require.NoError(t, err, "c2c execution for remote URL (tag v1.0.0) failed")
		if stderr != "" {
			assert.Contains(t, stderr, "Cloning into", "stderr contains git clone messages")
		}
		assert.Empty(t, stdout, "stdout should be empty")

		outputContentBytes, readErr := os.ReadFile(outputFilePathTag)
		require.NoError(t, readErr, "Failed to read output file for remote URL (tag v1.0.0)")
		outputContent := string(outputContentBytes)

		// Verify tree (repo name is "basic") - structure is same for v1.0.0
		expectedTree := "basic\n" +
			"├── .gitattributes\n" +
			"├── .gitignore\n" +
			"├── CHANGELOG\n" +
			"└── go\n" +
			"    └── gofixture.go"
		assert.Contains(t, outputContent, expectedTree, "Tree structure mismatch for tag v1.0.0")
		
		assert.Contains(t, outputContent, "```.gitattributes\n* text=auto\n```", "Expected .gitattributes content for tag v1.0.0")
		assert.Contains(t, outputContent, "```.gitignore\n*.mode.*\n```", "Expected .gitignore content for tag v1.0.0")
		assert.Contains(t, outputContent, "```CHANGELOG\nInitial changelog\n```", "Expected CHANGELOG content for tag v1.0.0")
		expectedGoFixture := "```go/gofixture.go\n" +
			"package gofixture\n\n" +
			"import \"fmt\"\n\n" +
			"func Print() {\n" +
			"\tfmt.Println(\"This is a go fixture\")\n" +
			"}\n```"
		assert.Contains(t, outputContent, expectedGoFixture, "Expected go/gofixture.go content for tag v1.0.0")
		assert.NotContains(t, outputContent, "\\", "Output should use forward slashes for paths")
	})
}
