# Clarity Usage

A software design tool for AI-native developers and coding agents.

```
clarity <COMMAND> [OPTIONS]
```

**Use cases:**
- Keep a live impact view while coding with `clarity watch`
- Generate focused change snapshots with `clarity show`
- Run repeatable design checks in developer and coding-agent workflows

## Global Flags

Inherited by all subcommands. Extracted from `cmd/root.go`.

| Flag | Short | Default | Description |
|---|---|---|---|
| `--verbose` | `-v` | `false` | Enable verbose/debug output |
| `--version` | `-V` | `false` | Print version information and exit |
## Commands

| Command | Description |
|---|---|
| `languages` | List all supported languages and file extensions |
| `setup` | Add clarity usage instructions to AGENTS.md |
| `show` | Show a scoped file-based dependency graph |
| `watch` | Watch for file changes and serve a live dependency graph |
| `why <from> <to>` | Show direct dependency direction(s) between two files |
| `workspace` | Experimental workspace relationship graph for Go modules and Rust crates |

## `clarity languages`

List all supported programming languages and their mapped file extensions.

Examples:
  clarity languages

```
clarity languages [OPTIONS]
```

| Flag | Short | Type | Default | Description |
|---|---|---|---|---|

---


## `clarity setup`

Initialize AGENTS.md with instructions for AI agents to use clarity.

```
clarity setup [OPTIONS]
```

| Flag | Short | Type | Default | Description |
|---|---|---|---|---|

---


## `clarity show`

Show a scoped file-based dependency graph.

```
clarity show [OPTIONS]
```

| Flag | Short | Type | Default | Description |
|---|---|---|---|---|
| `--format` | `-f` | string | `opts.outputFormat` | fmt.Sprintf("Output format (%s)", formatters.SupportedFormats()) |
| `--repo` | `-r` | string | `""` | Git repository path (default: current directory) |
| `--commit` | `-c` | string | `""` | Git commit or range to analyze (e.g., f0459ec, HEAD~3, f0459ec...be3d11a) |
| `--direction` | `-d` | string | `opts.direction` | fmt.Sprintf("Graph direction (%s)", formatters.SupportedDirections()) |
| `--file` | `-p` | string | `""` | Show dependencies for a specific file |
| `--url` | `-u` | bool | `false` | Generate visualization URL (supported formats: dot, mermaid) |
| `--input` | `-i` | []string | `nil` | Build graph from specific files and/or directories (comma-separated) |
| `--between` | `-w` | []string | `nil` | Find all paths between specified files (comma-separated) |
| `--level` | `-l` | int | `opts.depthLevel` | Depth level for dependencies (used with --file, 0 = unlimited) |
| `--include-ext` | | string | `""` | Include only files with these extensions (comma-separated, e.g. .go,.java) |
| `--exclude-ext` | | string | `""` | Exclude files with these extensions (comma-separated, e.g. .go,.java) |
| `--scope` | | string | `opts.scope` | Dependency scope for --file (downstream only) |
| `--allow-outside-repo` | | bool | `false` | Allow input paths outside the repo root |
| `--label` | | bool | `false` | Add deterministic short labels to edges |
| `--no-stats` | | bool | `false` | Skip file addition/deletion statistics for faster rendering |
| `--exclude` | | []string | `nil` | Exclude specific files and/or directories from graph inputs (comma-separated) |
| `--prune` | | []string | `nil` | Show node but skip its subtree (requires --file; shown with dashed border) |
| `--also` | | []string | `nil` | Include files matching glob patterns that connect to --file graph (requires --file) |

---


## `clarity watch`

Watch a project directory for file changes, rebuild the dependency graph, and serve a live-updating visualization at localhost.

```
clarity watch [OPTIONS]
```

| Flag | Short | Type | Default | Description |
|---|---|---|---|---|
| `--repo` | `-r` | string | `""` | Git repository path (default: current directory) |
| `--direction` | `-d` | string | `opts.direction` | fmt.Sprintf("Graph direction (%s)", formatters.SupportedDirections()) |
| `--input` | `-i` | []string | `nil` | Watch specific files and/or directories (comma-separated) |
| `--port` | `-P` | int | `opts.port` | HTTP server port |
| `--include-ext` | | string | `""` | Include only files with these extensions (comma-separated, e.g. .go,.java) |
| `--exclude-ext` | | string | `""` | Exclude files with these extensions (comma-separated, e.g. .go,.java) |
| `--exclude` | | []string | `nil` | Exclude specific files and/or directories (comma-separated) |

---


## `clarity why <from> <to>`

Show immediate dependency edge(s) between two files, including referenced members when available.

Args: `cobra.ExactArgs(2)`

```
clarity why <from> <to> [OPTIONS]
```

| Flag | Short | Type | Default | Description |
|---|---|---|---|---|
| `--format` | `-f` | string | `opts.outputFormat` | fmt.Sprintf("Output format (%s)", supportedFormats()) |
| `--repo` | `-r` | string | `""` | Git repository path (default: current directory) |
| `--allow-outside-repo` | | bool | `false` | Allow input paths outside the repo root |

---


## `clarity workspace`

Experimental workspace relationship graph for Go modules and Rust crates.

```
clarity workspace [OPTIONS]
```

| Flag | Short | Type | Default | Description |
|---|---|---|---|---|
| `--format` | `-f` | string | `opts.outputFormat` | fmt.Sprintf("Output format (%s)", formatters.SupportedFormats()) |
| `--repo` | `-r` | string | `""` | Repository path (default: current directory) |
| `--direction` | `-d` | string | `opts.direction` | fmt.Sprintf("Graph direction (%s)", formatters.SupportedDirections()) |
| `--url` | `-u` | bool | `false` | Generate visualization URL (supported formats: dot, mermaid) |
| `--language` | | string | `opts.language` | Workspace language filter (auto, go, rust) |

---
