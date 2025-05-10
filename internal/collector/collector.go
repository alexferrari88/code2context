package collector

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

type Options struct {
	Root            string
	IncludeTree     bool
	MaxSize         int64
	ExcludeDirs     []string
	ExcludeExts     []string
	ExcludePatterns []string
	SkipTextual     bool
	Verbose         bool
}

var mediaExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true, ".svg": true,
	".mp4": true, ".mp3": true, ".wav": true, ".ogg": true, ".flac": true,
	".avi": true, ".mkv": true, ".mov": true, ".pdf": true, ".docx": true,
	".pptx": true,
}

var textualButNonCode = map[string]bool{
	".md":   true,
	".txt":  true,
	".json": true,
	".csv":  true,
	".yml":  true,
	".yaml": true,
	".ini":  true,
	".bat":  true,
}

var codeLangByExt = map[string]string{
	".go":    "go",
	".js":    "javascript",
	".ts":    "typescript",
	".py":    "python",
	".java":  "java",
	".rb":    "ruby",
	".rs":    "rust",
	".cpp":   "cpp",
	".c":     "c",
	".h":     "c",
	".cs":    "csharp",
	".php":   "php",
	".swift": "swift",
	".kt":    "kotlin",
	".sql":   "sql",
	".html":  "html",
	".css":   "css",
	".xml":   "xml",
	".sh":    "bash",
	".bat":   "bat",
	".ps1":   "powershell",
}

func Run(ctx context.Context, w io.Writer, opts Options) error {
	root := opts.Root

	igPatterns, err := collectGitIgnorePatterns(root)
	if err != nil {
		return err
	}
	for _, p := range opts.ExcludePatterns {
		igPatterns = append(igPatterns, p)
	}
	ign := ignore.CompileIgnoreLines(igPatterns...)

	include := func(rel string, info os.FileInfo) bool {
		if ign.MatchesPath(rel) {
			return false
		}
		for _, d := range opts.ExcludeDirs {
			if rel == d || strings.HasPrefix(rel, d+string(os.PathSeparator)) {
				return false
			}
		}
		ext := strings.ToLower(filepath.Ext(rel))
		for _, e := range opts.ExcludeExts {
			if ext == "."+strings.ToLower(strings.TrimPrefix(e, ".")) {
				return false
			}
		}
		if mediaExts[ext] {
			return false
		}
		if opts.SkipTextual && textualButNonCode[ext] {
			return false
		}
		if info.Size() > opts.MaxSize {
			return false
		}
		if (info.Mode() & 0111) != 0 {
			return false
		}
		return true
	}

	var paths []string
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			if opts.Verbose {
				log.Printf("warning: %v", walkErr)
			}
			return nil
		}
		if path == root {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if info.IsDir() {
			if !include(rel, info) {
				return filepath.SkipDir
			}
			return nil
		}
		if include(rel, info) {
			paths = append(paths, rel)
		}
		return nil
	})
	if err != nil {
		return err
	}

	bufw := bufio.NewWriter(w)
	if opts.IncludeTree {
		if err := writeTree(bufw, paths); err != nil {
			return err
		}
		fmt.Fprintln(bufw) // blank line after tree
	}

	for _, p := range paths {
		ext := strings.ToLower(filepath.Ext(p))
		lang := codeLangByExt[ext]
		// Header
		fmt.Fprintf(bufw, "```%s %s\n", lang, p)
		// Content
		f, err := os.Open(filepath.Join(root, p))
		if err != nil {
			log.Printf("warning: %s: %v", p, err)
			continue
		}
		if _, err := io.Copy(bufw, f); err != nil {
			log.Printf("warning: copy %s: %v", p, err)
		}
		f.Close()
		fmt.Fprintln(bufw, "```")
	}
	return bufw.Flush()
}

func collectGitIgnorePatterns(root string) ([]string, error) {
	var patterns []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Base(path) == ".gitignore" {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			dir := filepath.Dir(path)
			relDir, _ := filepath.Rel(root, dir)
			lines := strings.Split(string(data), "\n")
			for _, l := range lines {
				l = strings.TrimSpace(l)
				if l == "" || strings.HasPrefix(l, "#") {
					continue
				}
				if relDir != "." {
					l = filepath.Join(relDir, l)
				}
				patterns = append(patterns, l)
			}
		}
		return nil
	})
	return patterns, err
}

func writeTree(w io.Writer, paths []string) error {
	dirs := make(map[string][]string)
	for _, p := range paths {
		dir := filepath.Dir(p)
		dirs[dir] = append(dirs[dir], filepath.Base(p))
	}
	// naive implementation
	for _, p := range paths {
		parts := strings.Split(p, string(os.PathSeparator))
		for i := range parts {
			indent := strings.Repeat("    ", i)
			fmt.Fprintf(w, "%s%s\n", indent, parts[i])
		}
	}
	return nil
}
