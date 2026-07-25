# Clarity

[![Built with Clarity](https://img.shields.io/endpoint?url=https%3A%2F%2Fraw.githubusercontent.com%2FLegacyCodeHQ%2Fclarity-cli%2Frefs%2Fheads%2Fmain%2Fbadges%2Fshields.io.json)](https://github.com/LegacyCodeHQ/clarity-cli)
[![License](https://img.shields.io/github/license/LegacyCodeHQ/clarity-cli)](LICENSE)
[![Release](https://img.shields.io/github/v/release/LegacyCodeHQ/clarity-cli)](https://github.com/LegacyCodeHQ/clarity-cli/releases)
[![npm version](https://img.shields.io/npm/v/@legacycodehq/clarity)](https://www.npmjs.com/package/@legacycodehq/clarity)

See the structure of a code change before you commit it.

Clarity builds dependency impact graphs from source code. It shows how files,
modules, tests, and documentation connect so developers and coding agents can
reason about design changes with evidence instead of intuition.

Use it when you want to know:

- What does this change touch?
- Who depends on this file?
- Why are these two areas connected?
- Did this refactor cross a boundary?
- Did we introduce a cycle?
- What does this branch look like structurally?

Clarity works at file granularity. It recovers coupling shape, not runtime
behavior or full API contracts.

## Quick Start

Install with npm:

```sh
npm install -g @legacycodehq/clarity
```

Or on macOS/Linux with Homebrew:

```sh
brew install LegacyCodeHQ/tap/clarity
```

Keep a live graph open while you code:

```sh
clarity watch
```

Show the structural impact of your uncommitted work:

```sh
clarity show
```

Find what depends on a file before changing it:

```sh
clarity show path/to/file.go --reach up
```

Review the structural footprint of a branch:

```sh
clarity show -c main...HEAD
```

## The Model

Every graph answers four questions.

| Question | Meaning | Examples |
|---|---|---|
| Which snapshot? | working tree, commit, or range | `clarity show`, `-c HEAD`, `-c main...HEAD` |
| What anchor? | changed files, paths, modules, whole tree, paths between files | `src/auth`, `--module auth`, `--all`, `--between a,b` |
| What lens? | how much context or abstraction to apply | `--reach up`, `--reach down`, `--depth 2`, `--collapse` |
| What rendering? | how to output the graph | DOT, Mermaid, browser URL, live UI |

This makes Clarity useful for both quick local checks and repeatable agent
workflows.

## Core Use Cases

### Review a Change Before Commit

```sh
clarity show
```

Shows the dependency graph around your uncommitted work.

Use it to check:

- whether the change stayed inside the intended area
- which neighboring files are affected
- whether tests are connected to the changed code
- whether new coupling appeared

### Review a Commit, Branch, or PR

```sh
clarity show -c HEAD
clarity show -c main...HEAD
```

Use commit and range snapshots to review structural impact after the fact. This
is useful for pull requests, regression windows, release reviews, and
agent-generated changes.

### Refactor Safely

```sh
clarity show path/to/file.go --reach up
clarity show path/to/file.go --reach both --depth 2
```

Use upstream reach to answer "who imports this?" before changing a file. Use
bounded reach to estimate blast radius.

### Understand a Codebase

```sh
clarity show src/auth
clarity show src/auth --reach both
clarity show --all --collapse
```

Use scoped graphs to explore unfamiliar code. Collapse configured modules when
the file-level graph is too noisy.

### Trace Unexpected Coupling

```sh
clarity show --between ui/login.ts,server/session.go
```

Shows the dependency paths connecting two files. Use this when you need to
explain why two areas are coupled or decide where to cut a dependency.

### Audit Module Boundaries

```sh
clarity modules
clarity show --module auth --reach both
clarity show --all --collapse
```

Declare modules in `.clarity/modules.json`, inspect a module in context, or
collapse modules into architecture-level nodes.

### Find Circular Dependencies

```sh
clarity cycles src
clarity cycles src --explain
clarity cycles src --code-only
clarity cycles src --exclude-kind module-declaration,navigation
clarity cycles src --include-kind call,type-reference
clarity cycles src --format json
clarity cycles src --url
```

Lists complete cyclic dependency components, shows a representative loop, and
recommends verified dependency-edge break sets (exact minima for bounded
components and labelled heuristics for larger ones). Explain mode adds the exact
symbols and source lines behind each edge; code-only mode excludes Markdown
navigation loops. Semantic filters rebuild the graph and recompute break sets
for selected relationship kinds. See
[actionable cycle analysis](docs/cycles.md).

### Keep Feedback Live While Coding

```sh
clarity watch
clarity watch src/auth --reach both
```

Runs a local live graph that updates as files change. Use it during refactors or
large agent edits to keep structural feedback visible.

### Support Coding Agents

```sh
clarity setup
```

Adds repository instructions so coding agents can use Clarity as part of their
normal loop.

Good agent patterns:

- inspect dependents before refactoring with `--reach up`
- run `clarity show -f mermaid` after edits
- generate `clarity show -u` for a shareable review artifact
- use the graph to catch unexpected coupling before commit

## Output

```sh
clarity show -f dot
clarity show -f mermaid
clarity show -u
```

DOT is the default. Mermaid works well in docs, IDEs, and agent UIs. `-u`
creates a browser-friendly visualization URL.

## Language Support

Clarity supports dependency extraction for:

- C, C++, C#
- Dart
- Go
- Java, Kotlin, Scala
- JavaScript, TypeScript, Svelte
- Markdown
- Python, Ruby
- Rust
- Swift
- Zig

Support quality varies by language. Run:

```sh
clarity languages
```

to see the current maturity and extension list.

## Experimental Surfaces

```sh
clarity cycles
clarity workspace
```

`cycles` reports complete cyclic components, source evidence, and verified
cycle-break recommendations. See [actionable cycle analysis](docs/cycles.md).

`workspace` builds Go module and Rust crate relationship graphs.

These surfaces are useful, but remain experimental. For `cycles`, the command
name, flags, relationship taxonomy, Go evidence types, human output, and JSON
schema may change without compatibility notice before stabilization.

## What Clarity Is Not

Clarity is not a replacement for tests, type checks, linters, or code review.

It does not provide:

- full symbol-level call graphs
- runtime behavior analysis
- API contract verification
- semantic correctness guarantees

It is a structural verification tool: it shows coupling, impact, boundaries,
cycles, and change shape.

## License

This project is licensed under the [Apache License, Version 2.0](LICENSE).

Copyright (c) 2026-present, Legacy Code Headquarters (OPC) Private Limited. All
rights reserved.
