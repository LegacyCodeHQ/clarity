# Sanity (experimental)

[![License](https://img.shields.io/github/license/LegacyCodeHQ/sanity)](LICENSE)
[![Release](https://img.shields.io/github/v/release/LegacyCodeHQ/sanity)](https://github.com/LegacyCodeHQ/sanity/releases)

Sanity is a file-level dependency graph generator that helps you assess risk and confidently review code you didn't write—from AI agents or teammates.

## Supported Languages

- Dart
- Go

## Quick Start

### Installation

Install on Linux and Mac using Homebrew:

```bash
brew install LegacyCodeHQ/tap/sanity
```

For other installation methods (pre-built binaries, build from source, Go install), see
the [Installation Guide](docs/installation.md).

## Usage

### Commands

#### `sanity graph`

Generate dependency graphs for Dart and Go files.

**Flags**:

| Flag             | Description               | Notes          |
|------------------|---------------------------|----------------|
| `--repo`, `-r`   | Git repository path       | Default: "."   |
| `--commit`, `-c` | Git commit to analyze     |                |
| `--format`, `-f` | Output format (dot, json) | Default: "dot" |

**Examples**:

```bash
# Analyze uncommitted files in current repository (most common use case)
sanity graph

# Output dependency graph in JSON format
sanity graph --format=json

# Analyze files changed in a specific commit
sanity graph --commit 8d4f78

# Analyze uncommitted files in a different repository
sanity graph --repo /path/to/repo

# Analyze specific files directly
sanity graph file1.dart file2.dart file3.dart
```

#### Help

- **List all commands**: `sanity --help`
- **Command-specific help**: `sanity <command> --help`
- **Help command alias**: `sanity help <command>`
