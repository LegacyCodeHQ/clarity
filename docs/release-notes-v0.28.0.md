# Clarity v0.28.0

> 💥 **Breaking release.** `clarity show` and `clarity watch` now share one
> grammar, and several v0.27.0 commands and flags were removed. Under 0.x semver
> a minor bump signals breaking changes. Read the **Breaking Changes** section
> before upgrading automated callers.

## Changelog

### ✨ Features

- **Unified `show` / `watch` grammar** – Both commands now take positional
  `[paths…]` anchors and share the same lenses: `--reach`, `--depth`,
  `--collapse`, `--all`, `--between`, and `--module`
  - `watch` reached parity with `show`, gaining `--reach`, `--depth`/`-l`,
    `--between`/`-w`, `--all`, `--collapse`, `--module`/`-m`, `--prune`,
    `--label`, and `--no-stats`
  - `watch` intentionally has no `--commit` — a live view is always the working tree
- **Dependency reach** – `--reach up|down|both` walks the graph from an anchor,
  including *upstream* reach ("who depends on this file?"), which the old surface
  could not express
- **Module boundary direction** – `--module` renders a module's boundary with a
  chosen direction (`none`/`in`/`out`/`both`) and composes as a framing, not an
  exclusive anchor
- **Collapse into modules** – `--all --collapse` folds the whole tree into the
  modules declared in `.clarity/modules.json`
- **HTML language support** added to the dependency graph
- **Machine-readable `languages`** – `languages --format json` emits the language
  and extension data the removed `extensions` command used to print
- **Hugo shortcodes in Markdown** – `relref`/`ref` shortcodes now resolve to real
  links when building the graph
- Experimental commands are now grouped last in `--help`

### 💥 Breaking Changes

#### Removed commands

| Removed | Replacement |
|---|---|
| `clarity extensions` | `clarity languages --format json` (language + extension data) |
| `clarity why <from> <to>` | **Removed; no direct replacement.** `clarity show --between <a,b>` shows connecting *paths* but not the typed direct-edge / callsite detail `why` reported. |

#### Removed flags (`show`)

| Removed | Replacement |
|---|---|
| `--input`, `-i <paths>` | positional `[paths…]` |
| `--file`, `-p <file>` | positional `<file>` with `--reach down` |
| `--level` | `--depth`, `-l` |
| `--scope` | `--reach down` |
| `--modules` | `--collapse` |
| `--direction`, `-d` | `--reach up\|down\|both` (with `--module`) |

#### Removed flags (`watch`)

| Removed | Replacement |
|---|---|
| `--input`, `-i <paths>` | positional `[paths…]` |
| `--direction`, `-d` | `--orientation`, `-o` |

#### Migration at a glance

| Old | New |
|---|---|
| `clarity show -i a b` | `clarity show a b` |
| `clarity show -p file.go` | `clarity show file.go --reach down` |
| `clarity show -p file.go --level 2` | `clarity show file.go --reach down --depth 2` |
| `clarity show --modules` | `clarity show --all --collapse` |
| `clarity show --module auth -d both` | `clarity show --module auth --reach both` |
| `clarity watch -i src` | `clarity watch src` |
| `clarity watch -d LR` | `clarity watch -o LR` |
| `clarity extensions` | `clarity languages --format json` |

### 🐞 Bug Fixes

- **Commit snapshots**: `show` now resolves modules, file anchors, and module
  members from commit snapshots instead of the working tree
- **Sessions**: default to the most recent snapshot when switching sessions
- **Watch worktrees**: recover discovery after stale worktrees and suppress the
  deleted graph for removed worktrees
- **Rust**: phantom prod nodes stay solid
- **Node labels**: graph lifecycle icons are now aligned
- Benign watcher path-removal errors are no longer logged

### 🔒 Security

- Bump dompurify to 3.4.11 (`cmd/watch/web`)
- Bump esbuild, `@sveltejs/vite-plugin-svelte`, `@tailwindcss/vite`, and vite

### ⚙️ Internal

- Legacy `show`/`watch` flags were deprecated first, then removed, so callers got
  a warning window before this release
- `show` snapshot resolution was centralized; watch worktree tabs are driven from
  a single near-free reconcile
- CLI usage doc regenerated; README reframed around structural verification
- `ci`: cache apt packages and the OSXCross toolchain in the release workflow

**Full Changelog**: https://github.com/LegacyCodeHQ/clarity-cli/compare/v0.27.0...v0.28.0
