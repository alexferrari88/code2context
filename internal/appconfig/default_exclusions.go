package appconfig

import "strings"

// Default lists are best-effort and can be expanded.

func GetDefaultExcludedDirs() []string {
	return []string{
		// Version control
		".git", ".hg", ".svn",
		// IDE/Editor specific
		".idea", ".vscode", ".vs", ".project", ".settings", ".classpath", ".metals", ".bsp", ".bloop",
		// Build artifacts & Dependencies (Common)
		"node_modules", "vendor", "target", "build", "dist", "out", "bin", "obj",
		// Python
		"__pycache__", ".pytest_cache", ".mypy_cache", ".tox", ".venv", "venv", "ENV", "env",
		// JS frameworks build
		".next", ".nuxt", ".svelte-kit", ".output",
		// Serverless frameworks
		".wrangler", ".serverless",
		// Terraform
		".terraform",
		// OS specific
		".DS_Store", "Thumbs.db",
		// Caching
		".cache",
		// Jupyter
		".ipynb_checkpoints",
		// Elixir / Erlang
		"_build", "deps", "_rel", "ebin",
		// Go (vendor is already listed, but good to be aware)
	}
}

func GetDefaultMediaExtensions() []string { // Ensure all start with a dot
	return []string{
		// Images
		".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".webp", ".svg", ".ico",
		// Audio
		".mp3", ".wav", ".ogg", ".aac", ".flac", ".m4a",
		// Video
		".mp4", ".avi", ".mov", ".wmv", ".mkv", ".flv", ".webm",
		// Documents (often binary or not primary source code)
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
		".odt", ".ods", ".odp",
		// Fonts
		".ttf", ".otf", ".woff", ".woff2", ".eot",
		// Design files
		".psd", ".ai", ".eps", ".sketch", ".fig",
	}
}

func GetDefaultArchiveExtensions() []string { // Ensure all start with a dot
	return []string{
		".zip", ".tar", ".gz", ".bz2", ".xz", ".rar", ".7z",
		".jar", ".war", ".ear", ".apk", ".img", ".iso", ".dmg", ".pkg",
		".deb", ".rpm", ".AppImage", // Moved from exec as they are packages
	}
}

func GetDefaultExecutableExtensions() []string { // Ensure all start with a dot
	// These supplement the POSIX execute bit check
	return []string{
		// Windows
		".exe", ".com", ".bat", ".cmd", ".ps1", ".vbs", ".msi",
		// Compiled/Intermediate
		".pyc", ".pyo", ".class", ".dll", ".so", ".dylib", ".o", ".obj", ".lib", ".a",
		".elf", // Common name for executables on Linux (often no extension)
		// Note: Files with no extension and execute bit are handled separately
	}
}

func GetDefaultLockfilePatterns() []string {
	// These are exact names or glob patterns matched against the base name
	return []string{
		"go.sum", "package-lock.json", "yarn.lock", "composer.lock", "Gemfile.lock",
		"Pipfile.lock", "poetry.lock", "Cargo.lock", "*.gradle.lockfile", "Podfile.lock",
		"pubspec.lock", "mix.lock", "npm-shrinkwrap.json", "pnpm-lock.yaml",
		"requirements.txt", "constraints.txt", // Common for Python deps
		"terraform.lock.hcl",
	}
}

func GetDefaultMiscellaneousFileNames() []string {
	// These are exact names matched against the base name
	return []string{
		"LICENSE", "COPYING", "NOTICE", "AUTHORS", "CHANGELOG", "CONTRIBUTING", "MANIFEST",
	}
}

func GetDefaultMiscellaneousExtensions() []string {
	// These are not code, but not strictly media or archives
	// They are often used for configuration, documentation, or other purposes
	return []string{
		".log", ".tmp", ".bak", ".swp", ".swo", ".orig", ".rej",
		".patch", ".diff", ".sql",
		".gitignore", ".dockerignore", ".npmignore", ".eslintignore", ".prettierignore",
		".editorconfig", ".gitattributes", ".gitmodules",
		".prettierrc", ".stylelintrc", ".eslintrc", ".babelrc",
	}
}

func GetDefaultAuxFileExtensions() []string {
	// Files to skip when --skip-aux-files is used
	// Focused on human-readable config/data/docs that are not primary code
	// Ensure extensions start with a dot, full names are as-is.
	aux := []string{
		// Data/Config formats
		".json", ".jsonc", ".json5", ".xml", ".toml",
		".yml", ".yaml",
		".ini", ".cfg", ".conf", ".properties", ".env",
		".csv", ".tsv",
		// Markup & Docs
		".md", ".markdown", ".rst", ".adoc", ".tex",
		".txt", ".text",
		// Logs
		".log",
		// Tooling configs (often project-specific, not general code)
		".editorconfig", ".gitattributes", ".gitmodules",
		".prettierrc", ".stylelintrc", ".eslintrc", ".babelrc",
		".eslintignore", ".prettierignore", ".dockerignore",
		// Project meta files (full name match, case-insensitive is better for these)
		"LICENSE", "README", "COPYING", "NOTICE", "AUTHORS", "CHANGELOG", "CONTRIBUTING", "MANIFEST",
		// Build/Workflow (can be code-like, but often high-level config)
		// Keep these out of aux by default, user can exclude with patterns if desired.
		// "Makefile", "Dockerfile", "Vagrantfile", "Jenkinsfile", ".gitlab-ci.yml",
		// "*.tf", "*.tfvars" // Terraform files are code.
	}

	normalized := make([]string, 0, len(aux))
	for _, item := range aux {
		if strings.Contains(item, ".") && !strings.HasPrefix(item, "*") { // Likely an extension
			if !strings.HasPrefix(item, ".") {
				normalized = append(normalized, "."+strings.ToLower(item))
			} else {
				normalized = append(normalized, strings.ToLower(item))
			}
		} else { // Full name or pattern
			normalized = append(normalized, item) // Keep case for full names/patterns like "LICENSE"
		}
	}
	return normalized
}
