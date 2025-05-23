package filefilter

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/alexferrari88/code2context/internal/appconfig"
	gitignore "github.com/sabhiram/go-gitignore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockFileInfo implements fs.FileInfo
type mockFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
	sys     interface{}
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return m.size }
func (m mockFileInfo) Mode() fs.FileMode  { return m.mode }
func (m mockFileInfo) ModTime() time.Time { return m.modTime }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() interface{}   { return m.sys }

// mockDirEntry implements fs.DirEntry
type mockDirEntry struct {
	fileInfo mockFileInfo
}

func (m mockDirEntry) Name() string               { return m.fileInfo.name }
func (m mockDirEntry) IsDir() bool                { return m.fileInfo.isDir }
func (m mockDirEntry) Type() fs.FileMode          { return m.fileInfo.mode.Type() }
func (m mockDirEntry) Info() (fs.FileInfo, error) { return m.fileInfo, nil }

func newMockFile(name string, size int64, mode fs.FileMode) mockDirEntry {
	if mode == 0 {
		mode = 0644
	}
	return mockDirEntry{fileInfo: mockFileInfo{name: name, size: size, mode: mode, isDir: false, modTime: time.Now()}}
}

func newMockDir(name string, mode fs.FileMode) mockDirEntry {
	if mode == 0 {
		mode = 0755 | fs.ModeDir
	} else {
		mode = mode | fs.ModeDir
	}
	return mockDirEntry{fileInfo: mockFileInfo{name: name, mode: mode, isDir: true, modTime: time.Now()}}
}

func removeStringFromSlice(slice []string, item string) []string {
	var result []string
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

func defaultFileFilterConfig(t *testing.T, finalOutputPath string) FilterConfig {
	t.Helper()
	absFinalOutputPath, err := filepath.Abs(finalOutputPath)
	require.NoError(t, err, "Failed to get absolute path for finalOutputPath in test helper")

	// Make copies of default slices to allow modification in tests
	defaultMiscExtensions := make([]string, len(appconfig.GetDefaultMiscellaneousExtensions()))
	copy(defaultMiscExtensions, appconfig.GetDefaultMiscellaneousExtensions())
	
	defaultExecExts := make([]string, len(appconfig.GetDefaultExecutableExtensions()))
	copy(defaultExecExts, appconfig.GetDefaultExecutableExtensions())

	return FilterConfig{
		MaxFileSize:                    1 * 1024 * 1024,
		UserExcludeDirs:                nil,
		UserExcludeExts:                nil,
		UserExcludeGlobs:               nil,
		SkipAuxFiles:                   false,
		DefaultExcludeDirs:             appconfig.GetDefaultExcludedDirs(),
		DefaultMediaExts:               appconfig.GetDefaultMediaExtensions(),
		DefaultArchiveExts:             appconfig.GetDefaultArchiveExtensions(),
		DefaultExecExts:                defaultExecExts,
		DefaultLockfilePatterns:        appconfig.GetDefaultLockfilePatterns(),
		DefaultMiscellaneousFileNames:  appconfig.GetDefaultMiscellaneousFileNames(),
		DefaultMiscellaneousExtensions: defaultMiscExtensions,
		DefaultAuxExts:                 appconfig.GetDefaultAuxFileExtensions(),
		FinalOutputFilePath:            absFinalOutputPath,
	}
}

func compileGitIgnoreInDir(t *testing.T, baseDir string, gitIgnoreRelativePath string, content string) *gitignore.GitIgnore {
	t.Helper()
	filePath := filepath.Join(baseDir, gitIgnoreRelativePath)
	err := os.MkdirAll(filepath.Dir(filePath), 0755)
	require.NoError(t, err, "Failed to create dir for .gitignore: %s", filepath.Dir(filePath))
	err = os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err, "Failed to write temporary .gitignore file: %s", filePath)

	matcher, err := gitignore.CompileIgnoreFile(filePath)
	require.NoError(t, err, "Failed to compile .gitignore file: %s", filePath)
	return matcher
}

func TestIsExcluded_OutputFilePathSelfExclusion(t *testing.T) {
	baseDir := t.TempDir()
	absTestOutputPath := filepath.Join(baseDir, "output.txt")
	cfg := defaultFileFilterConfig(t, absTestOutputPath)
	filter, err := NewFileFilter(baseDir, cfg)
	require.NoError(t, err)
	mockEntry := newMockFile("output.txt", 100, 0)
	excluded, err := filter.IsExcluded(absTestOutputPath, mockEntry, nil)
	assert.True(t, excluded)
	assert.NoError(t, err)
}

func TestIsExcluded_SymbolicLink(t *testing.T) {
	baseDir := t.TempDir()
	cfg := defaultFileFilterConfig(t, filepath.Join(baseDir, "output.txt"))
	filter, err := NewFileFilter(baseDir, cfg)
	require.NoError(t, err)
	symlinkEntry := newMockFile("symlink.txt", 0, fs.ModeSymlink)
	excluded, err := filter.IsExcluded(filepath.Join(baseDir, "symlink.txt"), symlinkEntry, nil)
	assert.True(t, excluded)
	assert.NoError(t, err)
}

func TestIsExcluded_DirectoryNameExclusion(t *testing.T) {
	baseDir := t.TempDir()
	cfg := defaultFileFilterConfig(t, filepath.Join(baseDir, "output.txt"))
	cfg.UserExcludeDirs = []string{"custom_exclude"}
	filter, err := NewFileFilter(baseDir, cfg)
	require.NoError(t, err)
	testCases := []struct {name string; dirName string; expectedErr error; shouldExclude bool}{
		{"default node_modules", "node_modules", filepath.SkipDir, true},
		{"default .git", ".git", filepath.SkipDir, true},
		{"user custom_exclude", "custom_exclude", filepath.SkipDir, true},
		{"not excluded dir", "src", nil, false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockDir := newMockDir(tc.dirName, 0755)
			absPath := filepath.Join(baseDir, tc.dirName)
			_ = os.MkdirAll(absPath, 0755) 
			excluded, err := filter.IsExcluded(absPath, mockDir, nil)
			assert.Equal(t, tc.shouldExclude, excluded)
			if tc.expectedErr != nil { assert.EqualError(t, err, tc.expectedErr.Error()) } else { assert.NoError(t, err) }
		})
	}
}

func TestIsExcluded_MaxFileSize(t *testing.T) {
	baseDir := t.TempDir()
	cfg := defaultFileFilterConfig(t, filepath.Join(baseDir, "output.txt")); cfg.MaxFileSize = 1024
	filter, _ := NewFileFilter(baseDir, cfg)
	testCases := []struct {name string; file mockDirEntry; path string; expectedErr error; shouldExclude bool}{
		{"large file", newMockFile("large.bin", 2000, 0), filepath.Join(baseDir, "large.bin"), nil, true},
		{"ok file", newMockFile("ok.txt", 500, 0), filepath.Join(baseDir, "ok.txt"), nil, false},
		{"exact size file", newMockFile("exact.dat", 1024, 0), filepath.Join(baseDir, "exact.dat"), nil, false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			excluded, err := filter.IsExcluded(tc.path, tc.file, nil)
			assert.Equal(t, tc.shouldExclude, excluded); assert.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestIsExcluded_UserExcludedExtensions(t *testing.T) {
	baseDir := t.TempDir(); cfg := defaultFileFilterConfig(t, filepath.Join(baseDir, "output.txt")); cfg.UserExcludeExts = []string{".log", ".tmp"}
	filter, _ := NewFileFilter(baseDir, cfg)
	testCases := []struct {name string; file mockDirEntry; path string; shouldExclude bool}{
		{"log file", newMockFile("app.log", 100, 0), filepath.Join(baseDir, "app.log"), true},
		{"tmp file", newMockFile("data.tmp", 100, 0), filepath.Join(baseDir, "data.tmp"), true},
		{"go file", newMockFile("main.go", 100, 0), filepath.Join(baseDir, "main.go"), false},
		{"uppercase log", newMockFile("APP.LOG", 100, 0), filepath.Join(baseDir, "APP.LOG"), true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			excluded, _ := filter.IsExcluded(tc.path, tc.file, nil); assert.Equal(t, tc.shouldExclude, excluded)
		})
	}
}

func TestIsExcluded_UserExcludedGlobs(t *testing.T) {
	baseDir := t.TempDir(); cfg := defaultFileFilterConfig(t, filepath.Join(baseDir, "output.txt")); cfg.UserExcludeGlobs = []string{"*_test.go", "temp/*", "specific_file.txt"}
	filter, _ := NewFileFilter(baseDir, cfg)
	testCases := []struct {name string; file mockDirEntry; path string; shouldExclude bool}{
		{"test go file", newMockFile("utils_test.go", 100, 0), filepath.Join(baseDir, "utils_test.go"), true},
		{"file in temp dir", newMockFile("some.txt", 100, 0), filepath.Join(baseDir, "temp", "some.txt"), true},
		{"specific file name", newMockFile("specific_file.txt", 100, 0), filepath.Join(baseDir, "specific_file.txt"), true},
		{"non-test go file", newMockFile("main.go", 100, 0), filepath.Join(baseDir, "main.go"), false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_ = os.MkdirAll(filepath.Dir(tc.path), 0755) 
			excluded, _ := filter.IsExcluded(tc.path, tc.file, nil); assert.Equal(t, tc.shouldExclude, excluded)
		})
	}
}

func TestIsExcluded_ExecutablePermissions(t *testing.T) {
	if runtime.GOOS == "windows" { t.Skip("Skipping executable permission test on Windows") }
	baseDir := t.TempDir(); cfg := defaultFileFilterConfig(t, filepath.Join(baseDir, "output.txt"))
	cfg.DefaultExecExts = removeStringFromSlice(cfg.DefaultExecExts, ".sh") 
	filter, _ := NewFileFilter(baseDir, cfg)
	testCases := []struct {name string; file mockDirEntry; path string; shouldExclude bool}{
		{"executable sh", newMockFile("run.sh", 100, 0755), filepath.Join(baseDir, "run.sh"), true},
		{"non-executable sh", newMockFile("noexec.sh", 100, 0644), filepath.Join(baseDir, "noexec.sh"), false}, // Corrected
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			excluded, _ := filter.IsExcluded(tc.path, tc.file, nil); assert.Equal(t, tc.shouldExclude, excluded, tc.name)
		})
	}
}

func TestIsExcluded_DefaultExecExtensions(t *testing.T) {
	baseDir := t.TempDir(); cfg := defaultFileFilterConfig(t, filepath.Join(baseDir, "output.txt"))
	filter, _ := NewFileFilter(baseDir, cfg)
	testCases := []struct {name string; file mockDirEntry; path string; shouldExclude bool}{
		{"sh file (non-exec perm)", newMockFile("script.sh", 100, 0644), filepath.Join(baseDir, "script.sh"), true},
		{"exe file", newMockFile("myprog.exe", 100, 0644), filepath.Join(baseDir, "myprog.exe"), true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			excluded, _ := filter.IsExcluded(tc.path, tc.file, nil); assert.Equal(t, tc.shouldExclude, excluded, tc.name)
		})
	}
}

func TestIsExcluded_MediaExtensions(t *testing.T) {
	baseDir := t.TempDir(); cfg := defaultFileFilterConfig(t, filepath.Join(baseDir, "output.txt")); filter, _ := NewFileFilter(baseDir, cfg)
	require.True(t, len(cfg.DefaultMediaExts) > 0); filePath := filepath.Join(baseDir, "image"+cfg.DefaultMediaExts[0])
	excluded, _ := filter.IsExcluded(filePath, newMockFile("image"+cfg.DefaultMediaExts[0], 100, 0), nil); assert.True(t, excluded)
}
func TestIsExcluded_ArchiveExtensions(t *testing.T) {
	baseDir := t.TempDir(); cfg := defaultFileFilterConfig(t, filepath.Join(baseDir, "output.txt")); filter, _ := NewFileFilter(baseDir, cfg)
	require.True(t, len(cfg.DefaultArchiveExts) > 0); filePath := filepath.Join(baseDir, "archive"+cfg.DefaultArchiveExts[0])
	excluded, _ := filter.IsExcluded(filePath, newMockFile("archive"+cfg.DefaultArchiveExts[0], 100, 0), nil); assert.True(t, excluded)
}
func TestIsExcluded_LockfilePatterns(t *testing.T) {
	baseDir := t.TempDir(); cfg := defaultFileFilterConfig(t, filepath.Join(baseDir, "output.txt")); filter, _ := NewFileFilter(baseDir, cfg)
	require.True(t, len(cfg.DefaultLockfilePatterns) > 0); filePath := filepath.Join(baseDir, cfg.DefaultLockfilePatterns[0])
	excluded, _ := filter.IsExcluded(filePath, newMockFile(cfg.DefaultLockfilePatterns[0], 100, 0), nil); assert.True(t, excluded)
}
func TestIsExcluded_MiscellaneousExtensionsAndNames(t *testing.T) {
	baseDir := t.TempDir(); cfg := defaultFileFilterConfig(t, filepath.Join(baseDir, "output.txt")); filter, _ := NewFileFilter(baseDir, cfg)
	if len(cfg.DefaultMiscellaneousExtensions) > 0 {
		miscExt := cfg.DefaultMiscellaneousExtensions[0]
		if miscExt == ".log" && len(cfg.DefaultMiscellaneousExtensions) > 1 { miscExt = cfg.DefaultMiscellaneousExtensions[1]}
		if miscExt != ".log" {
			filePath := filepath.Join(baseDir, "file"+miscExt)
			excluded, _ := filter.IsExcluded(filePath, newMockFile("file"+miscExt, 100, 0), nil); assert.True(t, excluded)
		}
	}
	if len(cfg.DefaultMiscellaneousFileNames) > 0 {
		filePath := filepath.Join(baseDir, cfg.DefaultMiscellaneousFileNames[0])
		excluded, _ := filter.IsExcluded(filePath, newMockFile(cfg.DefaultMiscellaneousFileNames[0], 100, 0), nil); assert.True(t, excluded)
	}
}
func TestIsExcluded_SkipAuxFiles(t *testing.T) {
	baseDir := t.TempDir(); cfg := defaultFileFilterConfig(t, filepath.Join(baseDir, "output.txt")); cfg.SkipAuxFiles = true; filter, _ := NewFileFilter(baseDir, cfg)
	testCases := []struct {name string; path string; shouldExclude bool}{
		{"markdown", filepath.Join(baseDir, "README.md"), true}, {"go", filepath.Join(baseDir, "code.go"), false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			excluded, _ := filter.IsExcluded(tc.path, newMockFile(filepath.Base(tc.path), 100, 0), nil); assert.Equal(t, tc.shouldExclude, excluded)
		})
	}
}

func TestIsExcluded_Gitignore_NoActiveIgnores(t *testing.T) {
	baseDir := t.TempDir(); cfg := defaultFileFilterConfig(t, filepath.Join(baseDir, "output.txt")); cfg.SkipAuxFiles = false
	cfg.DefaultMiscellaneousExtensions = removeStringFromSlice(cfg.DefaultMiscellaneousExtensions, ".log")
	filter, _ := NewFileFilter(baseDir, cfg)
	excluded, _ := filter.IsExcluded(filepath.Join(baseDir, "some.log"), newMockFile("some.log", 100, 0), nil)
	assert.False(t, excluded)
}

func TestIsExcluded_Gitignore_SingleActiveIgnore_FileMatch(t *testing.T) {
	baseDir := t.TempDir(); cfg := defaultFileFilterConfig(t, filepath.Join(baseDir, "output.txt"))
	cfg.DefaultMiscellaneousExtensions = removeStringFromSlice(cfg.DefaultMiscellaneousExtensions, ".log")
	filter, _ := NewFileFilter(baseDir, cfg)
	rootIgnore := compileGitIgnoreInDir(t, baseDir, ".gitignore", "*.log\n!keep.log"); activeIgnores := []*gitignore.GitIgnore{rootIgnore}
	excluded, _ := filter.IsExcluded(filepath.Join(baseDir, "app.log"), newMockFile("app.log", 100, 0), activeIgnores); assert.True(t, excluded)
	excluded, _ = filter.IsExcluded(filepath.Join(baseDir, "keep.log"), newMockFile("keep.log", 100, 0), activeIgnores); assert.False(t, excluded)
}

func TestIsExcluded_Gitignore_SingleActiveIgnore_DirMatch(t *testing.T) {
	baseDir := t.TempDir(); cfg := defaultFileFilterConfig(t, filepath.Join(baseDir, "output.txt")); filter, _ := NewFileFilter(baseDir, cfg)
	rootIgnore := compileGitIgnoreInDir(t, baseDir, ".gitignore", "logs/\nbuild/"); activeIgnores := []*gitignore.GitIgnore{rootIgnore}
	
	logsPath := filepath.Join(baseDir, "logs"); _ = os.MkdirAll(logsPath, 0755)
	excluded, err := filter.IsExcluded(logsPath, newMockDir("logs", 0), activeIgnores)
	assert.True(t, excluded, "logs dir should be excluded"); assert.Equal(t, filepath.SkipDir, err, "err for logs dir")

	buildPath := filepath.Join(baseDir, "build"); _ = os.MkdirAll(buildPath, 0755)
	excluded, err = filter.IsExcluded(buildPath, newMockDir("build", 0), activeIgnores)
	assert.True(t, excluded, "build dir should be excluded"); assert.Equal(t, filepath.SkipDir, err, "err for build dir")
}

func TestIsExcluded_Gitignore_NestedIgnores_Override(t *testing.T) {
	baseDir := t.TempDir(); cfg := defaultFileFilterConfig(t, filepath.Join(baseDir, "output.txt"))
	cfg.DefaultMiscellaneousExtensions = removeStringFromSlice(cfg.DefaultMiscellaneousExtensions, ".log")
	filter, _ := NewFileFilter(baseDir, cfg)
	
	rootIgnore := compileGitIgnoreInDir(t, baseDir, ".gitignore", "*.log")
	subIgnore := compileGitIgnoreInDir(t, baseDir, "sub/.gitignore", "!special.log\n*.txt")
	_ = os.MkdirAll(filepath.Join(baseDir, "sub"), 0755)

	activeRoot := []*gitignore.GitIgnore{rootIgnore}; activeSub := []*gitignore.GitIgnore{rootIgnore, subIgnore}

	excluded, _ := filter.IsExcluded(filepath.Join(baseDir, "regular.log"), newMockFile("regular.log", 100, 0), activeRoot); assert.True(t, excluded, "regular.log")
	excluded, _ = filter.IsExcluded(filepath.Join(baseDir, "sub", "special.log"), newMockFile("special.log", 100, 0), activeSub); assert.False(t, excluded, "sub/special.log")
	excluded, _ = filter.IsExcluded(filepath.Join(baseDir, "sub", "another.log"), newMockFile("another.log", 100, 0), activeSub); assert.True(t, excluded, "sub/another.log")
	excluded, _ = filter.IsExcluded(filepath.Join(baseDir, "sub", "data.txt"), newMockFile("data.txt", 100, 0), activeSub); assert.True(t, excluded, "sub/data.txt")
}

func TestIsExcluded_Gitignore_PathMatching(t *testing.T) {
	baseDir := t.TempDir(); cfg := defaultFileFilterConfig(t, filepath.Join(baseDir, "output.txt")); filter, _ := NewFileFilter(baseDir, cfg)
	rootIgnore := compileGitIgnoreInDir(t, baseDir, ".gitignore", "specific_dir/file.txt\n/root_level_file.txt\nsub_dir/*.md")
	activeIgnores := []*gitignore.GitIgnore{rootIgnore}
	
	_ = os.MkdirAll(filepath.Join(baseDir, "specific_dir"), 0755)
	_ = os.MkdirAll(filepath.Join(baseDir, "other_dir", "specific_dir"), 0755)
	_ = os.MkdirAll(filepath.Join(baseDir, "sub_dir"), 0755)

	excluded, _ := filter.IsExcluded(filepath.Join(baseDir, "specific_dir", "file.txt"), newMockFile("file.txt", 100, 0), activeIgnores); assert.True(t, excluded, "specific_dir/file.txt")
	excluded, _ = filter.IsExcluded(filepath.Join(baseDir, "other_dir", "specific_dir", "file.txt"), newMockFile("file.txt", 100, 0), activeIgnores); assert.False(t, excluded, "other_dir/specific_dir/file.txt")
	excluded, _ = filter.IsExcluded(filepath.Join(baseDir, "root_level_file.txt"), newMockFile("root_level_file.txt", 100, 0), activeIgnores); assert.True(t, excluded, "/root_level_file.txt")
	excluded, _ = filter.IsExcluded(filepath.Join(baseDir, "sub_dir", "root_level_file.txt"), newMockFile("root_level_file.txt", 100, 0), activeIgnores); assert.False(t, excluded, "sub_dir/root_level_file.txt")
	excluded, _ = filter.IsExcluded(filepath.Join(baseDir, "sub_dir", "doc.md"), newMockFile("doc.md", 100, 0), activeIgnores); assert.True(t, excluded, "sub_dir/doc.md")
	excluded, _ = filter.IsExcluded(filepath.Join(baseDir, "doc.md"), newMockFile("doc.md", 100, 0), activeIgnores); assert.False(t, excluded, "doc.md at root")
}
