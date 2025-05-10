package processor

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alexferrari88/code2context/internal/filefilter"
	gitignore "github.com/sabhiram/go-gitignore"
)

const (
	treePrefixEntry    = "├── "
	treePrefixLast     = "└── "
	treePrefixContinue = "│   "
	treePrefixEmpty    = "    "
)

type TreeBuilder struct {
	basePath             string
	filter               *filefilter.FileFilter
	gitIgnoreCache       map[string]*gitignore.GitIgnore                    // Shared cache from Processor
	compileGitIgnoreFunc func(dirPath string) (*gitignore.GitIgnore, error) // Function to compile/get from cache
}

func NewTreeBuilder(
	basePath string,
	filter *filefilter.FileFilter,
	cache map[string]*gitignore.GitIgnore,
	compileFunc func(dirPath string) (*gitignore.GitIgnore, error),
) *TreeBuilder {
	return &TreeBuilder{
		basePath:             basePath,
		filter:               filter,
		gitIgnoreCache:       cache, // Use the shared cache
		compileGitIgnoreFunc: compileFunc,
	}
}

type treeNode struct {
	name     string
	isDir    bool
	children []*treeNode
}

func (tb *TreeBuilder) BuildTreeString() (string, error) {
	absBasePath, err := filepath.Abs(tb.basePath)
	if err != nil {
		return "", fmt.Errorf("treebuilder: failed to get absolute base path: %w", err)
	}

	rootNodeName := filepath.Base(absBasePath)
	rootNode := &treeNode{name: rootNodeName, isDir: true}

	var initialGitIgnores []*gitignore.GitIgnore
	rootGitIgnore, _ := tb.compileGitIgnoreFunc(absBasePath) // Use the passed function
	if rootGitIgnore != nil {
		initialGitIgnores = append(initialGitIgnores, rootGitIgnore)
	}

	err = tb.buildNodeRecursive(absBasePath, rootNode, initialGitIgnores)
	if err != nil {
		return "", err // Error already contextualized
	}

	var builder strings.Builder
	builder.WriteString(rootNode.name + "\n")
	tb.writeNodeRecursive(&builder, rootNode.children, "") // Start with children of root
	return builder.String(), nil
}

func (tb *TreeBuilder) buildNodeRecursive(currentDirPath string, parentNode *treeNode, parentActiveIgnores []*gitignore.GitIgnore) error {
	entries, err := os.ReadDir(currentDirPath)
	if err != nil {
		// Don't fail the whole tree for one unreadable dir, just log and skip its children.
		slog.Warn("TreeBuilder: Failed to read directory (skipping its children in tree)", "path", currentDirPath, "error", err)
		return nil // Continue building other parts of the tree
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir() // Dirs first
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name()) // Then alphanumeric
	})

	currentDirGitIgnore, _ := tb.compileGitIgnoreFunc(currentDirPath)
	currentActiveIgnores := parentActiveIgnores
	if currentDirGitIgnore != nil {
		// Avoid appending duplicate if currentDirPath's ignore is already the last one in parentActiveIgnores
		// (e.g. if parentActiveIgnores was passed down from this same level due to root call)
		isNewMatcher := true
		if len(parentActiveIgnores) > 0 && parentActiveIgnores[len(parentActiveIgnores)-1] == currentDirGitIgnore {
			isNewMatcher = false
		}
		if isNewMatcher {
			// Create a new slice to avoid modifying the parent's slice
			currentActiveIgnores = make([]*gitignore.GitIgnore, len(parentActiveIgnores))
			copy(currentActiveIgnores, parentActiveIgnores)
			currentActiveIgnores = append(currentActiveIgnores, currentDirGitIgnore)
		}
	}

	for _, entry := range entries {
		entryAbsPath := filepath.Join(currentDirPath, entry.Name())

		// Use the filter to decide if this entry (file or dir) should be in the tree
		// The filter itself will log why something is skipped if verbose.
		excluded, filterErr := tb.filter.IsExcluded(entryAbsPath, entry, currentActiveIgnores)

		if filterErr != nil {
			if errors.Is(filterErr, filepath.SkipDir) {
				// Filter explicitly said to skip this directory and its contents
				slog.Debug("TreeBuilder: Directory skipped by filter decision in tree", "path", entryAbsPath)
				continue
			}
			// Some other error during filtering this entry
			slog.Warn("TreeBuilder: Error filtering entry for tree (entry skipped)", "path", entryAbsPath, "error", filterErr)
			continue
		}

		if excluded {
			// If it's a directory and filter said "excluded=true" (but not SkipDir error),
			// it means the directory itself matches an exclusion, so don't recurse.
			// If it's a file and excluded, just don't add it.
			// Logging is handled by the filter.
			continue
		}

		node := &treeNode{name: entry.Name(), isDir: entry.IsDir()}
		parentNode.children = append(parentNode.children, node)

		if entry.IsDir() {
			// Recursively build for subdirectories
			// Pass down the currentActiveIgnores, which now includes this directory's .gitignore if present
			err := tb.buildNodeRecursive(entryAbsPath, node, currentActiveIgnores)
			if err != nil {
				// Log or handle, but typically continue building other branches
				slog.Debug("TreeBuilder: Error processing sub-directory for tree", "path", entryAbsPath, "error", err)
			}
		}
	}
	return nil
}

func (tb *TreeBuilder) writeNodeRecursive(builder *strings.Builder, children []*treeNode, prefix string) {
	for i, child := range children {
		connector := treePrefixEntry
		nextPrefixElement := treePrefixContinue
		if i == len(children)-1 {
			connector = treePrefixLast
			nextPrefixElement = treePrefixEmpty
		}

		builder.WriteString(prefix)
		builder.WriteString(connector)
		builder.WriteString(child.name)
		builder.WriteString("\n")

		if child.isDir && len(child.children) > 0 {
			tb.writeNodeRecursive(builder, child.children, prefix+nextPrefixElement)
		}
	}
}
