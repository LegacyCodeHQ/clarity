# Clarity

[![Built with Clarity](https://raw.githubusercontent.com/LegacyCodeHQ/clarity/main/badges/built-with-clarity-sunrise.svg)](https://raw.githubusercontent.com/LegacyCodeHQ/clarity/main/badges/built-with-clarity-sunrise.svg)
[![License](https://img.shields.io/github/license/LegacyCodeHQ/clarity)](LICENSE)
[![Release](https://img.shields.io/github/v/release/LegacyCodeHQ/clarity)](https://github.com/LegacyCodeHQ/clarity/releases)
[![npm version](https://img.shields.io/npm/v/@legacycodehq/clarity)](https://www.npmjs.com/package/@legacycodehq/clarity)
[![Go Report Card](https://goreportcard.com/badge/github.com/LegacyCodeHQ/clarity)](https://goreportcard.com/report/github.com/LegacyCodeHQ/clarity)

Clarity is a software design tool for AI-native developers and coding agents.

**Note:** Clarity supports [**14 languages**](#supported-languages) (parsing quality may vary by language).

## What You Get

Clarity generates impact graphs from your code so you can review design effects before commit.

- Keep a live impact view while coding with `clarity watch`.
- Generate focused snapshots for uncommitted changes, commits, commit ranges, and file-to-file paths with `clarity show`.
- Run repeatable design checks in developer and coding-agent workflows, with shareable visualization output when needed.

## Quick Start

**Step 1:** Install with npm (cross-platform):

```bash
npm install -g @legacycodehq/clarity
```

Or install on macOS/Linux using Homebrew:

```bash
brew install LegacyCodeHQ/tap/clarity
```

For more install options see the [installation guide](docs/usage/installation.md).

**Step 2:** Inside your project:

```bash
clarity setup # Configures AGENTS.md for your coding agent to use Clarity
```

**Step 3:** Start with a live impact view while you code:

```bash
clarity watch
```

## Developers & Agents

Clarity helps teams using coding agents make safer, faster design changes.

**For developers**, the value is practical:

- Understand what a change will affect before commit.
- Review architecture and design impact quickly during feature work.
- Give coding agents concrete feedback grounded in actual low-level design.

**For agents**, Clarity provides a deterministic and repeatable way to verify and validate their design changes.

### Developer Workflows

#### 1) During development: keep impact visible while making changes

```bash
clarity watch
```

Use `clarity watch` during active development to keep design impact visible as the codebase evolves.

#### 2) Before committing: generate focused change context

If your coding agent is already configured with Clarity via `clarity setup` (one-time), the agent can run these commands and render
a diagram for you.

If not, run them manually as part of your development flow.

```bash
clarity show                      # Visualize uncommitted changes
clarity show -c HEAD              # Visualize the latest commit
clarity show -c HEAD~3...HEAD     # Visualize a commit range
```

Use this output to answer: "What did we actually touch?", "What does the solution look like?" and "Which parts of the system are now coupled?"

#### 3) Explore the codebase and debug design decisions: trace specific relationships

```bash
clarity show -i src,tests         # Build graph from specific files/directories
clarity show -w a.go,b.go         # Show all paths between files
```

**Note:** Use the `-u` flag, as in `clarity show -u` to generate a shareable visualization URL.

> **💡 Tip:** During design discussions, use actual graphs to explain or challenge design decisions with evidence instead of intuition.

#### When to Use `watch` vs `show`

If your coding agent is configured using `clarity setup`, running `clarity show` manually is optional.

| Use case                                                     | Command                 | Why                                                                            |
|--------------------------------------------------------------|-------------------------|--------------------------------------------------------------------------------|
| You are actively coding and want continuous feedback         | `clarity watch`         | Keeps a live view updated as files change so you can catch design drift early. |
| You want a point-in-time view of current uncommitted work    | `clarity show`          | Produces a snapshot of what your current changes impact.                       |
| You are reviewing committed history (single commit or range) | `clarity show -c <rev>` | Focuses analysis on specific commits for review or debugging.                  |
| You want a shareable/browser-friendly view                   | `clarity show -u`       | Generates a visualization URL you can open or share.                           |

### Agent Workflow

If you use a coding agent, set up Clarity once so the agent can include design checks in its normal loop.

1. Run `clarity setup` in your repository.
2. Confirm `AGENTS.md` includes Clarity instructions.
3. Ask your agent to run `clarity show` after meaningful changes and use the output in its review.

<p align="center">
  <img src="docs/images/clarity+codex-app.png" alt="Clarity graph in Codex app">
  <small>Clarity highlights impacted files and related tests so you can review design impact before commit.</small>
</p>

Clarity works across desktop and CLI-based agent workflows.
In desktop products, agents can render Mermaid diagrams inline.
In CLI workflows, agents are configured to open a generated visualization URL in a new browser tab when showing design output.

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
- Svelte
- Swift
- TypeScript

---

## License

This project is licensed under the [GNU Affero General Public License v3.0](LICENSE).
