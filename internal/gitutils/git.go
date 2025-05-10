package gitutils

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CloneRepo clones a Git repository to a temporary directory.
// Returns the path to the cloned repo (inside a unique temp dir) and the repo name.
func CloneRepo(repoURL, ref string) (string, string, error) {
	// Create a unique parent temporary directory first
	parentTempDir, err := os.MkdirTemp("", "c2c_clone_parent_*")
	if err != nil {
		return "", "", fmt.Errorf("gitutils: failed to create parent temporary directory: %w", err)
	}

	repoName := getRepoNameFromURL(repoURL)
	// Clone into a subdirectory named after the repo within the unique parent temp dir
	// This makes the tempDir path returned more predictable (parentTempDir/repoName)
	// and ensures the target directory for clone does not exist.
	clonePath := filepath.Join(parentTempDir, repoName)

	slog.Info("Cloning repository...", "url", repoURL, "ref", ref, "target_path", clonePath)

	cmdArgs := []string{"clone", "--no-tags", "--no-recurse-submodules"} // Start with leaner clone options
	if ref != "" {
		cmdArgs = append(cmdArgs, "--branch", ref, "--single-branch") // Clone specific branch, also implies depth 1 often
		// For commits/tags that are not branch heads, --depth 1 with --branch might not work.
		// Git intelligently handles this; if ref is a tag/commit, it checks it out.
		// However, --depth 1 implies getting only the tip of that branch.
		// If ref is a specific commit, we might not need --depth 1, or ensure it's a shallow clone of that commit.
		// Modern Git is quite good; --branch <tag_or_commit> usually works and creates a detached HEAD.
		// Let's stick to this; if specific commit depth is needed, it's an advanced scenario.
		// We can add --depth 1 unconditionally, git usually figures it out or makes it a shallow clone of the ref.
		cmdArgs = append(cmdArgs, "--depth", "1")
	} else {
		cmdArgs = append(cmdArgs, "--depth", "1") // Shallow clone default branch
	}
	cmdArgs = append(cmdArgs, repoURL, clonePath)

	cmd := exec.Command("git", cmdArgs...)

	// Capture output for better error reporting if verbose is not on
	var outBuilder, errBuilder strings.Builder
	cmd.Stdout = &outBuilder
	cmd.Stderr = &errBuilder

	slog.Debug("Executing git command", "args", strings.Join(cmd.Args, " "))

	if err := cmd.Run(); err != nil {
		os.RemoveAll(parentTempDir) // Clean up on failure
		slog.Error("Git clone command output", "stdout", outBuilder.String(), "stderr", errBuilder.String())
		return "", "", fmt.Errorf("gitutils: failed to clone repository '%s' (ref: '%s'): %w. Stderr: %s", repoURL, ref, err, errBuilder.String())
	}

	slog.Info("Repository cloned successfully", "path", clonePath)
	// The path to the actual repo content is clonePath.
	// The parentTempDir is what needs to be cleaned up eventually.
	// We return clonePath as the basePath for processing, and parentTempDir for cleanup.
	// Let processor handle cleanup of parentTempDir. For simplicity, we'll make cloneRepo return just the clonePath
	// and assume the caller of CloneRepo (or its wrapper) will know how to clean it (e.g., if parentTempDir was its idea).
	// For now, CloneRepo creates parentTempDir AND clonePath, so it should return parentTempDir for cleanup.
	// The processor will use clonePath (which is parentTempDir/repoName).

	return clonePath, repoName, nil // Caller cleans up parentTempDir which contains clonePath
}

func getRepoNameFromURL(repoURL string) string {
	parsedURL := repoURL
	// Remove common prefixes
	if strings.HasPrefix(parsedURL, "https://") {
		parsedURL = strings.TrimPrefix(parsedURL, "https://")
	} else if strings.HasPrefix(parsedURL, "http://") {
		parsedURL = strings.TrimPrefix(parsedURL, "http://")
	} else if strings.HasPrefix(parsedURL, "git@") {
		parsedURL = strings.TrimPrefix(parsedURL, "git@")
		parsedURL = strings.Replace(parsedURL, ":", "/", 1) // Replace git@host:path to git@host/path form
	}

	// Remove .git suffix
	parsedURL = strings.TrimSuffix(parsedURL, ".git")

	// Get the last path component
	parts := strings.Split(parsedURL, "/")
	if len(parts) > 0 {
		repoName := parts[len(parts)-1]
		if repoName != "" {
			return repoName
		}
	}
	// Fallback if parsing is difficult
	return "repository"
}

// IsGitURL checks if the input string looks like a git URL or SCP-like path.
func IsGitURL(path string) bool {
	return strings.HasPrefix(path, "http://") ||
		strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "git@") || // Covers git@github.com:user/repo.git
		strings.HasPrefix(path, "ssh://") ||
		strings.HasSuffix(path, ".git") // Covers file:///path/to/repo.git or local clones identified by .git
}
