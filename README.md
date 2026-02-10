# Clarity (formerly Sanity)

[![Built with Clarity](https://raw.githubusercontent.com/LegacyCodeHQ/clarity/main/badges/built-with-clarity-sunrise.svg)](https://raw.githubusercontent.com/LegacyCodeHQ/clarity/main/badges/built-with-clarity-sunrise.svg)
[![License](https://img.shields.io/github/license/LegacyCodeHQ/clarity)](LICENSE)
[![Release](https://img.shields.io/github/v/release/LegacyCodeHQ/clarity)](https://github.com/LegacyCodeHQ/clarity/releases)
[![npm version](https://img.shields.io/npm/v/@legacycodehq/clarity)](https://www.npmjs.com/package/@legacycodehq/clarity)
[![Go Report Card](https://goreportcard.com/badge/github.com/LegacyCodeHQ/clarity)](https://goreportcard.com/report/github.com/LegacyCodeHQ/clarity)

Clarity is a software design tool for developers and coding agents.

> **Renamed from Sanity:** If you previously used `sanity`, this is the same project under a new name.

## Quick Start

**Step 1:** Install with npm (cross-platform):

```bash
npm install -g @legacycodehq/clarity
```

Or install on macOS/Linux using Homebrew:

```bash
brew install LegacyCodeHQ/tap/clarity
```

**Step 2:** Inside your project:

```bash
clarity setup  # Add usage instructions to AGENTS.md for your coding agent
```

More install options: [Installation Guide](docs/usage/installation.md).

## Supported Languages

- C
- C++
- C#
- Dart
- Go
- JavaScript
- Java
- Kotlin
- Python
- Ruby
- Rust
- Swift
- TypeScript

## Use Cases

- Build maintainable software
- Understand codebases
- [Audit AI-generated code](https://youtu.be/EqOwJnZSiQs)
- Stabilize and reclaim apps built with AI

## Problems

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

## How Clarity Helps

Clarity uses a file-based dependency graph to visualize the impact of AI-generated changes, showing you:

- The files changed and the relationships between them
- The order to review files (**short answer:** review from right-to-left)
- Color-coded files by extension to help you quickly categorize and group them for review
- Test files at a glance
- An accurate mental model of the system as you evolve it

## Clarity in Action

Clarity works with Desktop and IDE coding agents. If you are using a CLI coding agent, the agent can open diagrams in your browser for review.

<p align="center">
  <img src="docs/images/clarity+codex-app.png" alt="Clarity graph in Codex app">
  <small>Clarity shows impacted files and highlights tests in green.</small>
</p>

---

## License

This project is licensed under the [GNU Affero General Public License v3.0](LICENSE).
