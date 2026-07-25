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
| `cycles [path...]` | Analyze cyclic components and break sets (experimental) |
| `languages` | List all supported languages and file extensions |
| `modules` | List the modules declared for this project |
| `setup` | Add clarity usage instructions to AGENTS.md |
| `show [paths...]` | Show a scoped file-based dependency graph |
| `watch [paths...]` | Watch for file changes and serve a live dependency graph |
| `workspace` | Workspace relationship graph for Go modules and Rust crates (experimental) |

---


## `clarity cycles [path...]`

List cyclic dependency components between files within a scope.

Scopes to the directories (or files) you pass, defaulting to the current
directory, and reports every strongly connected group found within that scope.
Each component includes one representative loop and a verified set of dependency
edges that can be removed to make the component acyclic. Bounded components
receive exact minimum sets; larger ones receive a labelled heuristic.

With --url, each complete component is rendered as its own focused diagram and
the command emits a shareable visualization URL beneath it.

This command is experimental; its output may change.

```
clarity cycles [path...] [OPTIONS]
```

| Flag | Short | Type | Default | Description |
|---|---|---|---|---|

---


## `clarity languages`

List all supported programming languages and their mapped file extensions.

Examples:
  clarity languages
  clarity languages --format json

```
clarity languages [OPTIONS]
```

| Flag | Short | Type | Default | Description |
|---|---|---|---|---|
| `--format` | | string | `opts.format` | Output format (text, json) |

---


## `clarity modules`

List the modules declared in .clarity/modules.json.

Each module reports the files it resolves to after expanding globs, split into
test and non-test files (mistyped patterns surface as a count of 0). Pass a
listed name to "clarity show --module <name>" to render that module.

Examples:
  clarity modules
  clarity modules --sort-by size
  clarity modules --repo path/to/repo

```
clarity modules [OPTIONS]
```

| Flag | Short | Type | Default | Description |
|---|---|---|---|---|
| `--repo` | `-r` | string | `""` | Git repository path (default: current directory) |
| `--sort-by` | | string | `opts.sortBy` | Order modules by: name (A→Z) or size (largest first) |

---


## `clarity setup`

Initialize AGENTS.md with instructions for AI agents to use clarity.

```
clarity setup [OPTIONS]
```

| Flag | Short | Type | Default | Description |
|---|---|---|---|---|

---


## `clarity show [paths...]`

Show a scoped file-based dependency graph.

```
clarity show [paths...] [OPTIONS]
```

| Flag | Short | Type | Default | Description |
|---|---|---|---|---|
| `--format` | `-f` | string | `opts.outputFormat` | fmt.Sprintf("Output format (%s)", formatters.SupportedFormats()) |
| `--repo` | `-r` | string | `""` | Git repository path (default: current directory) |
| `--commit` | `-c` | string | `""` | Git commit or range to analyze (e.g., f0459ec, HEAD~3, f0459ec...be3d11a) |
| `--orientation` | `-o` | string | `opts.orientation` | fmt.Sprintf("Graph layout orientation (%s)", formatters.SupportedDirections()) |
| `--module` | `-m` | string | `""` | Render the named module's files inside a box, alongside any files already in scope such as working-set changes (quote names with spaces) |
| `--url` | `-u` | bool | `false` | Generate visualization URL (supported formats: dot, mermaid) |
| `--between` | `-w` | []string | `nil` | Find all paths between specified files (comma-separated) |
| `--depth` | `-l` | int | `opts.depthLevel` | Depth for --reach (0 = unlimited) |
| `--include-ext` | | string | `""` | Include only files with these extensions (comma-separated, e.g. .go,.java) |
| `--exclude-ext` | | string | `""` | Exclude files with these extensions (comma-separated, e.g. .go,.java) |
| `--reach` | | string | `""` | Walk dependencies from the anchor: up, down, both |
| `--allow-outside-repo` | | bool | `false` | Allow input paths outside the repo root |
| `--all` | | bool | `false` | Render the whole tree at this snapshot |
| `--collapse` | | bool | `false` | Collapse files into the modules declared in .clarity/modules.json |
| `--label` | | bool | `false` | Add deterministic short labels to edges |
| `--no-stats` | | bool | `false` | Skip file addition/deletion statistics for faster rendering |
| `--no-phantom` | | bool | `false` | Suppress phantom test nodes (Rust files with #[cfg(test)] regions are rendered as a single node) |
| `--exclude` | | []string | `nil` | Exclude specific files and/or directories from graph inputs (comma-separated) |
| `--prune` | | []string | `nil` | Show node but skip its subtree (requires --reach; shown with dashed border) |
| `--also` | | []string | `nil` | Include files matching glob patterns that connect to the --reach graph (requires --reach) |

---


## `clarity watch [paths...]`

Watch a project directory for file changes, rebuild the dependency graph, and serve a live-updating visualization at localhost.

```
clarity watch [paths...] [OPTIONS]
```

| Flag | Short | Type | Default | Description |
|---|---|---|---|---|
| `--repo` | `-r` | string | `""` | Git repository path (default: current directory) |
| `--module` | `-m` | string | `""` | Render the named module's files inside a box |
| `--orientation` | `-o` | string | `opts.direction` | fmt.Sprintf("Graph layout orientation (%s)", formatters.SupportedDirections()) |
| `--format` | `-f` | string | `opts.format` | fmt.Sprintf("Output format (%s)", formatters.SupportedFormats()) |
| `--between` | `-w` | []string | `nil` | Find all paths between specified files (comma-separated) |
| `--port` | `-P` | int | `opts.port` | HTTP server port |
| `--depth` | `-l` | int | `opts.depthLevel` | Depth for --reach (0 = unlimited) |
| `--include-ext` | | string | `""` | Include only files with these extensions (comma-separated, e.g. .go,.java) |
| `--exclude-ext` | | string | `""` | Exclude files with these extensions (comma-separated, e.g. .go,.java) |
| `--reach` | | string | `""` | Walk dependencies from the anchor: up, down, both |
| `--all` | | bool | `false` | Render the whole live working tree |
| `--collapse` | | bool | `false` | Collapse files into the modules declared in .clarity/modules.json |
| `--label` | | bool | `false` | Add deterministic short labels to edges |
| `--no-stats` | | bool | `false` | Skip file addition/deletion statistics for faster rendering |
| `--no-phantom` | | bool | `false` | Suppress phantom test nodes (Rust files with #[cfg(test)] regions are rendered as a single node) |
| `--exclude` | | []string | `nil` | Exclude specific files and/or directories (comma-separated) |
| `--prune` | | []string | `nil` | Show node but skip its subtree (requires --reach) |

---


## `clarity workspace`

Workspace relationship graph for Go modules and Rust crates (experimental).

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
| `--manifest` | | string | `""` | Prune workspace graph to the dependency subgraph rooted at this manifest path |

---
