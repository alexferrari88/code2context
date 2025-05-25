# Code2Context (c2c)

[![Go Report Card](https://goreportcard.com/badge/github.com/alexferrari88/code2context)](https://goreportcard.com/report/github.com/alexferrari88/code2context)
[![Go Version](https://img.shields.io/github/go-mod/go-version/alexferrari88/code2context)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

`code2context` is a command-line interface (CLI) tool written in Go that helps you consolidate an entire codebase (or parts of it) into a single, LLM-friendly text file. This is incredibly useful for providing Large Language Models with the necessary context for coding projects without tedious manual copying and pasting of files.

The tool intelligently processes a local folder or a public GitHub repository, includes a `tree`-like representation of the codebase structure, and wraps each file's content in GitHub-style fenced code blocks with the file path as an info string. It smartly ignores irrelevant files and directories like `.git`, `node_modules`, media files, binaries, and respects nested `.gitignore` rules.

## Features

- **Single Text File Output:** Concatenates all relevant code files into one `.txt` file.
- **GitHub & Local Repo Support:** Point to a local directory or a public GitHub repository URL.
- **Smart Filtering:**
  - Ignores common VCS folders (`.git`, etc.).
  - Skips typically irrelevant directories (`node_modules`, `vendor`, build outputs, etc.).
  - Respects all nested `.gitignore` files.
  - Excludes media files (images, videos, audio).
  - Excludes binary/executable files (based on extension and POSIX permissions).
  - Skips files larger than a configurable size (default 1MB).
  - Excludes symbolic links.
  - Skips common lock files (`package-lock.json`, `go.sum`, etc.).
- **Codebase Tree View:** Optionally prepends a `tree`-like structure of the included files and folders to the output (enabled by default).
- **Formatted Output:** Each file's content is wrapped like:
  ````
  ```path/to/your/file.go
  // content of file.go
  ```
  ````

- **Customizable Exclusions:**
  - Exclude specific directories by name.
  - Exclude files by extension.
  - Exclude files/directories by glob patterns.
  - Option to skip non-code, human-readable auxiliary files (e.g., `.json`, `.csv`, `.md`, `.txt`).
- **Git Reference Support:** For GitHub repositories, specify a branch, tag, or commit hash using the `--ref` flag.
- **Configurable File Size:** Set a maximum file size to include using `--max-file-size`.
- **Verbose Logging:** Use `-v` or `--verbose` for detailed processing logs.
- **Self-Exclusion:** The generated output file is automatically excluded from its own content if generated within the source directory.

## Installation

### Prerequisites

- Go (version 1.24 or later recommended).
- Git (for cloning repositories).

### From Source

1. Clone the repository:

   ```bash
   git clone https://github.com/alexferrari88/code2context.git
   cd code2context
   ```

2. Build the executable:

   ```bash
   go build -o c2c ./main.go
   ```

3. (Optional) Move the `c2c` executable to a directory in your system's PATH, for example:

   ```bash
   sudo mv c2c /usr/local/bin/
   # or for a local user path
   # mkdir -p ~/bin && mv c2c ~/bin
   # (ensure ~/bin is in your PATH)
   ```

### Using `go install` (Recommended for Go users)

```bash
go install github.com/alexferrari88/code2context@latest
```

This will download, compile, and install the `c2c` binary into your `$GOPATH/bin` or `$GOBIN` directory. Ensure this directory is in your system's PATH.

### Prebuilt Binaries

Prebuilt executables for **macOS (x64/arm64)** and **Linux (x64)** are available on the [Releases page](https://github.com/alexferrari88/code2context/releases).

> **Note:** Windows and Linux on ARM64 binaries are not yet provided. If you’re proficient with GitHub Actions and would like to help add these targets, please propose changes to the [release workflow](https://github.com/alexferrari88/code2context/blob/main/.github/workflows/release.yml) and open a pull request.

### Adding to PATH

Once you've downloaded the appropriate binary for your platform, add it to your system `PATH` so you can run `c2c` from any terminal.

#### Linux and macOS

1. Make the binary executable:

   ```bash
   chmod +x c2c
   ```

2. Move it to a directory in your PATH (for example `/usr/local/bin`):

   ```bash
   sudo mv c2c /usr/local/bin/
   ```

   Alternatively, for a user-local install:

   ```bash
   mkdir -p ~/bin
   mv c2c ~/bin
   # Ensure ~/bin is in your PATH (e.g., add `export PATH="$HOME/bin:$PATH"` to your shell profile)
   ```

#### Windows

1. Rename the downloaded executable to `c2c.exe` (if necessary).

2. Move it to a permanent directory, for example `C:\tools\c2c`, and then add that directory to your system `PATH`:

   * Open **System Properties** > **Advanced** > **Environment Variables**.
   * Under **User variables** or **System variables**, select `Path` and click **Edit**.
   * Click **New**, then enter `C:\tools\c2c` (or your chosen folder).
   * Click **OK** to apply.

   Or, in PowerShell (as administrator):

   ```powershell
   setx PATH $env:PATH + ";C:\tools\c2c"
   ```

After that, you can run `c2c` from any terminal.

## Usage

The executable is named `c2c`.

```bash
c2c <path_or_url> [flags]
```

**Arguments:**

- `<path_or_url>`: (Required) Path to a local directory or a public GitHub repository URL.

**Flags:**

```
  -o, --output string           Output file name (default: <folder_name>.txt or <repo_name>.txt)
      --ref string              Git reference (branch, tag, commit) for remote repositories
      --tree                    Include a tree representation of the codebase (enabled by default) (default true)
      --no-tree                 Disable the tree representation of the codebase (overrides --tree if set)
      --skip-aux-files          Skip non-code, human-readable auxiliary files (json, csv, yml, md, txt, etc.)
      --exclude-dirs string     Comma-separated list of directory names to exclude (e.g., "docs,tests")
      --exclude-exts string     Comma-separated list of file extensions to exclude (e.g., ".log,.tmp,json")
      --exclude-patterns string Comma-separated list of glob patterns to exclude (e.g., "*_test.go,vendor/*")
      --max-file-size string    Maximum file size to include (e.g., "500KB", "2MB", "1024") (default "1MB")
  -v, --verbose                 Enable verbose logging
  -h, --help                    help for c2c
```

### Examples

1.  **Process the current directory and save to `myproject_context.txt`:**

    ```bash
    c2c . -o myproject_context.txt
    ```

2.  **Process a specific local project folder and use default output name (`myproject.txt`):**

    ```bash
    c2c ../myproject
    ```

3.  **Process a public GitHub repository (default branch):**

    ```bash
    c2c https://github.com/spf13/cobra
    ```

4.  **Process a specific branch (`v1.7.0`) of a GitHub repository and skip auxiliary files:**

    ```bash
    c2c https://github.com/spf13/cobra --ref v1.7.0 --skip-aux-files
    ```

5.  **Process with custom exclusions and no tree view:**

    ```bash
    c2c . --exclude-dirs "examples,temp" --exclude-exts ".tmp,.bak" --exclude-patterns "**/testdata/*" --no-tree
    ```

6.  **Process and set a maximum file size of 500KB:**

    ```bash
    c2c . --max-file-size 500KB
    ```

7.  **Process with verbose logging:**
    ```bash
    c2c . -v
    ```

## How it Works

1.  **Input:** Takes a local path or a GitHub URL. If a URL is provided, the repository is cloned into a temporary directory.
2.  **File Traversal:** Walks through the codebase directory structure.
3.  **Filtering:** For each file and directory, a series of exclusion rules are applied:
    - The tool's own output file is always excluded.
    - Symbolic links are skipped.
    - Default directory exclusions (e.g., `.git`, `node_modules`).
    - User-defined directory exclusions (`--exclude-dirs`).
    - `.gitignore` rules: The tool respects `.gitignore` files at all levels of the repository. Rules in deeper `.gitignore` files can override or supplement those in parent directories for their specific scope.
    - If a directory is excluded, its contents are not processed further.
    - For files:
      - Max file size (`--max-file-size`).
      - User-defined extension exclusions (`--exclude-exts`).
      - User-defined glob pattern exclusions (`--exclude-patterns`).
      - Default executable file exclusions (by extension and POSIX execute bit).
      - Default media and archive file exclusions (by extension).
      - Default lock file exclusions (by name/pattern).
      - Optional auxiliary file exclusion (`--skip-aux-files`).
4.  **Tree Generation:** If enabled (`--tree`, default), a `tree`-like representation of all _included_ files and directories is generated.
5.  **Content Aggregation:** The content of each _included_ file is read.
6.  **Output Formatting:** The tree (if included) and the content of each file are written to the output `.txt` file. Each file's content is enclosed in GitHub-style fenced code blocks, with its relative path as the info string.
7.  **Cleanup:** If a repository was cloned, the temporary directory is removed.

## Contributing

Contributions are welcome! If you have suggestions for improvements or encounter any bugs, please [open an issue](https://github.com/alexferrari88/code2context/issues) or submit a pull request.

## Acknowledgments

- Gemini 2.5 Pro for code generation

## License

This project is licensed under the [MIT License](LICENSE).
