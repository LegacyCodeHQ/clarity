# Sanity (experimental)

[![License](https://img.shields.io/github/license/LegacyCodeHQ/sanity)](LICENSE)
[![Release](https://img.shields.io/github/v/release/LegacyCodeHQ/sanity)](https://github.com/LegacyCodeHQ/sanity/releases)

Sanity helps you audit AI-generated code, stabilize vibe-coded apps, and build a solid understanding of unfamiliar codebases.

## The Problem

Every time a coding agent makes changes to your codebase, you have the following questions:

- Which files should I review first and in what order?
- Where should I spend most of my review effort?
- What is the blast radius of this change?
- Which parts of the change are too risky?
- How does this solution fit into the existing system?
- Are there adequate tests for these changes?

These concerns worsen when there are:

- Too many files to review
- You have an outdated mental model of your codebase

## How Sanity Helps

Sanity uses a file-based dependency graph to visualize the impact of AI-generated changes, showing you:

- The files changed and the relationships between them
- The order to review files (simple answer: review from right-to-left)
- Color-coded files by extensions to quickly categorize and group them for review
- Identify test files at a glance
- Help you build an accurate mental model of the system as you evolve it

## Visuals

<p align="center">
  <img src="docs/images/go-example.png" alt="Go">
  <small>Fig. 1: Relationships between impacted files are shown and tests are highlighted in green.</small>
</p>

<p align="center">
  <img src="docs/images/kotlin-example.png" alt="Kotlin">
  <small>Fig. 2: New files are marked with a 🪴 emoji.</small>
</p>

## Supported Languages

- Dart
- Go
- JavaScript
- Java
- Kotlin
- Python
- TypeScript

## Quick Start

### Installation

Install on Linux and Mac using Homebrew:

```bash
brew install LegacyCodeHQ/tap/sanity
```

For other installation methods (pre-built binaries, build from source, Go install), see
the [Installation Guide](docs/usage/installation.md).

## Usage

### Commands

#### `sanity graph`

Generate dependency graphs for Dart, Go, JavaScript, Java, Kotlin, Python, and TypeScript files.

**Flags**:

| Flag             | Description                  | Notes          |
|------------------|------------------------------|----------------|
| `--repo`, `-r`   | Git repository path          | Default: "."   |
| `--commit`, `-c` | Git commit to analyze        |                |
| `--format`, `-f` | Output format (dot, mermaid) | Default: "dot" |

**Examples**:

```bash
# Analyze uncommitted files in current repository (most common use case)
sanity graph

# Output dependency graph in JSON format
sanity graph --format=json

# Analyze files changed in a specific commit
sanity graph --commit 8d4f78

# Analyze uncommitted files in a different repository
sanity graph --repo /path/to/repo --commit HEAD~1

# Analyze specific files directly
sanity graph --input file1.dart,file2.dart,file3.dart
```

#### `sanity languages`

List supported languages and mapped file extensions for your installed Sanity version.

**Example**:

```bash
sanity languages
```

#### Help

- **List all commands**: `sanity --help`
- **Command-specific help**: `sanity <command> --help`
- **Help command alias**: `sanity help <command>`
