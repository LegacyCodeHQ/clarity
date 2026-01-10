# Sanity

[![License](https://img.shields.io/github/license/LegacyCodeHQ/sanity)](LICENSE)
[![Release](https://img.shields.io/github/v/release/LegacyCodeHQ/sanity)](https://github.com/LegacyCodeHQ/sanity/releases)

Sanity is a CLI tool for analyzing and visualizing dependency graphs in your codebase, with support for Dart and Go
files.

## Usage

### Commands

#### `sanity graph`

Generate dependency graphs for Dart and Go files.

**Flags**:

| Flag           | Description                                      | Notes             |
|----------------|--------------------------------------------------|-------------------|
| `--repo, -r`   | Git repository path to analyze uncommitted files |                   |
| `--commit, -c` | Git commit to analyze                            | Requires `--repo` |
| `--format, -f` | Output format (dot, json)                        | Default: "dot"    |

**Examples**:

```bash
# Analyze uncommitted files in current repository (most common use case)
sanity graph --repo .

# Output dependency graph in JSON format
sanity graph --repo . --format=json

# Analyze files changed in a specific commit
sanity graph --repo . --commit 8d4f78

# Analyze uncommitted files in a different repository
sanity graph --repo /path/to/repo

# Analyze specific files directly
sanity graph file1.dart file2.dart file3.dart
```

#### Help

- **List all commands**: `sanity --help`
- **Command-specific help**: `sanity <command> --help`
- **Help command alias**: `sanity help <command>`

---

## Development

### Testing and Code Coverage

This project uses Go's built-in testing framework with code coverage support.

**Available make targets**:

| Command              | Description                                   |
|----------------------|-----------------------------------------------|
| `make test`          | Run all tests                                 |
| `make test-coverage` | Run tests and show coverage percentage        |
| `make coverage`      | Generate coverage profile (coverage.out)      |
| `make coverage-html` | Generate HTML coverage report (coverage.html) |
| `make clean`         | Remove coverage files and binary              |
