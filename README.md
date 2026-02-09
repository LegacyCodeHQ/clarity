# Sanity

[![Built with Sanity](https://raw.githubusercontent.com/LegacyCodeHQ/sanity/main/badges/built-with-sanity-sunrise.svg)](https://raw.githubusercontent.com/LegacyCodeHQ/sanity/main/badges/built-with-sanity-sunrise.svg)
[![License](https://img.shields.io/github/license/LegacyCodeHQ/sanity)](LICENSE)
[![Release](https://img.shields.io/github/v/release/LegacyCodeHQ/sanity)](https://github.com/LegacyCodeHQ/sanity/releases)
[![npm version](https://img.shields.io/npm/v/@legacycodehq/sanity)](https://www.npmjs.com/package/@legacycodehq/sanity)
[![Go Report Card](https://goreportcard.com/badge/github.com/LegacyCodeHQ/sanity)](https://goreportcard.com/report/github.com/LegacyCodeHQ/sanity)

Sanity is a software design tool for developers and coding agents.

## Quick Start

**Step 1:** Install with npm (cross-platform):

```bash
npm install -g @legacycodehq/sanity
```

Or install on macOS/Linux using Homebrew:

```bash
brew install sanity
```

**Step 2:** Inside your project:

```bash
sanity setup  # Add usage instructions to AGENTS.md for your coding agent
```

More install options: [Installation Guide](docs/usage/installation.md).

## Supported Languages

C • C++ • C# • Dart • Go • JavaScript • Java • Kotlin • Python • Ruby • Rust • Swift • TypeScript

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

## How Sanity Helps

Sanity uses a file-based dependency graph to visualize the impact of AI-generated changes, showing you:

- The files changed and the relationships between them
- The order to review files (**short answer:** review from right-to-left)
- Color-coded files by extension to help you quickly categorize and group them for review
- Test files at a glance
- An accurate mental model of the system as you evolve it

## Sanity in Action

Sanity works with Desktop and IDE coding agents. If you are using a CLI coding agent, the agent can open diagrams in your browser for review.

<p align="center">
  <img src="docs/images/sanity+codex-app.png" alt="Sanity graph in Codex app">
  <small>Sanity shows impacted files and highlights tests in green.</small>
</p>

---

## License

This project is licensed under the [GNU Affero General Public License v3.0](LICENSE).
