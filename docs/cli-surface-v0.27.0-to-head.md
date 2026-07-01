# Clarity CLI surface comparison: v0.27.0 to HEAD

This comparison was generated for `CLR-7`.

## Inputs

- Baseline: `v0.27.0` (`8e6a0813ea5d3eface04d3bf27c824252eb6659f`)
- Compared revision: `HEAD` (`434c72c4fb1b95d07f96b784a89f705dfa0da500`)
- Generator: `compass view run usage-doc`
- Generated baseline artifact: `/tmp/clarity-usage-v027.md`
- Generated HEAD artifact: `/tmp/clarity-usage-head.md`

`v0.27.0` did not contain a `.compass` directory, so the current `usage-doc`
view definition was copied into the temporary `v0.27.0` worktree before
generation. Both surfaces were therefore extracted with the same query,
schema, and template.

The checked-in `usage-clarity.md` at HEAD matches the generated HEAD artifact.

## Executive summary

HEAD is a breaking CLI surface change relative to `v0.27.0`.

The biggest change is that `show` and `watch` now share the same grammar:
positional path anchors, `--reach`, `--depth`, `--collapse`, `--all`,
`--between`, and module framing. The old `show`-specific file/dependency
flags and the old `watch --input` shape are gone.

Two top-level commands were removed:

- `extensions`
- `why <from> <to>`

The remaining top-level surface at HEAD is:

- `watch [paths...]`
- `show [paths...]`
- `languages`
- `modules`
- `setup`
- `cycles [path...]`
- `workspace`

## Top-level commands

| Command | v0.27.0 | HEAD | Change |
|---|---:|---:|---|
| `cycles [path...]` | yes | yes | unchanged |
| `extensions` | yes | no | removed |
| `languages` | yes | yes | gained `--format` |
| `modules` | yes | yes | unchanged |
| `setup` | yes | yes | unchanged |
| `show` | yes | no | replaced by `show [paths...]` |
| `show [paths...]` | no | yes | new positional path grammar |
| `watch` | yes | no | replaced by `watch [paths...]` |
| `watch [paths...]` | no | yes | new positional path grammar |
| `why <from> <to>` | yes | no | removed |
| `workspace` | yes | yes | description changed; flags unchanged |

## Removed commands

### `clarity extensions`

`extensions` listed file extensions and mapped languages. HEAD removes the
command and moves machine-readable language and extension reporting into:

```sh
clarity languages --format json
```

Breaking impact: scripts or docs that call `clarity extensions` must switch to
`clarity languages --format json` or `clarity languages`.

### `clarity why <from> <to>`

`why` showed immediate dependency directions between two files. HEAD removes
the command. The closest retained surface is:

```sh
clarity show --between a,b
```

Breaking impact: callers using `clarity why` need to migrate to a graph-based
between-path view, or a replacement command must be reintroduced before
release.

## `clarity show`

### Command grammar

| Concern | v0.27.0 | HEAD |
|---|---|---|
| Usage | `clarity show [OPTIONS]` | `clarity show [paths...] [OPTIONS]` |
| Path anchor | `--input, -i <paths>` | positional `[paths...]` |
| Single-file dependency anchor | `--file, -p <file>` | `<file> --reach down` |
| Module collapse | `--modules` | `--collapse` |
| Dependency traversal | `--file` plus `--scope downstream` and `--level` | `--reach up|down|both` plus `--depth` |
| Whole tree | implicit via inputs such as `-i .` | explicit `--all` |

### Removed `show` flags

| Removed flag | v0.27.0 meaning | HEAD replacement |
|---|---|---|
| `--input`, `-i` | comma-separated files/directories | positional paths |
| `--file`, `-p` | show dependencies for one file | positional path plus `--reach down` |
| `--level` | depth for `--file` | `--depth`, `-l` |
| `--scope` | downstream-only file scope | `--reach down` |
| `--modules` | collapse configured modules | `--collapse` |
| `--direction`, `-d` | reserved module boundary direction | `--reach up|down|both` with `--module` |

### Added or changed `show` flags

| HEAD flag | Short | Notes |
|---|---|---|
| `--module` | `-m` | short alias added; boxes a named module inside the current scope |
| `--reach` | none | walks dependencies from the anchor: `up`, `down`, or `both` |
| `--depth` | `-l` | replaces `--level`; depth for `--reach` |
| `--all` | none | explicit whole-tree anchor |
| `--collapse` | none | replaces `--modules` |
| `--prune` | none | now requires `--reach`, not `--file` |
| `--also` | none | now composes with `--reach`, not `--file` |

### Unchanged `show` flags

- `--format`, `-f`
- `--repo`, `-r`
- `--commit`, `-c`
- `--orientation`, `-o`
- `--url`, `-u`
- `--between`, `-w`
- `--include-ext`
- `--exclude-ext`
- `--allow-outside-repo`
- `--label`
- `--no-stats`
- `--no-phantom`
- `--exclude`

## `clarity watch`

### Command grammar

| Concern | v0.27.0 | HEAD |
|---|---|---|
| Usage | `clarity watch [OPTIONS]` | `clarity watch [paths...] [OPTIONS]` |
| Path anchor | `--input, -i <paths>` | positional `[paths...]` |
| Layout direction | `--direction, -d` | `--orientation, -o` |
| Show parity | limited live graph controls | shares `show` anchors/lenses where live snapshots make sense |

### Removed `watch` flags

| Removed flag | v0.27.0 meaning | HEAD replacement |
|---|---|---|
| `--input`, `-i` | comma-separated watched files/directories | positional paths |
| `--direction`, `-d` | graph direction | `--orientation`, `-o` |

### Added `watch` flags

| HEAD flag | Short | Notes |
|---|---|---|
| `--module` | `-m` | render a named module's files inside a box |
| `--orientation` | `-o` | replaces `--direction` |
| `--between` | `-w` | find paths between files in the live graph |
| `--depth` | `-l` | depth for `--reach` |
| `--reach` | none | walk dependencies from the anchor |
| `--all` | none | render the whole live working tree |
| `--collapse` | none | collapse configured modules |
| `--label` | none | deterministic edge labels |
| `--no-stats` | none | skip addition/deletion statistics |
| `--prune` | none | stop reach traversal at paths |

### Unchanged `watch` flags

- `--repo`, `-r`
- `--format`, `-f`
- `--port`, `-P`
- `--include-ext`
- `--exclude-ext`
- `--no-phantom`
- `--exclude`

## `clarity languages`

`languages` gained machine-readable output:

```sh
clarity languages --format json
```

This is the natural replacement for the removed `extensions` command when a
consumer needs structured language/extension data.

## `clarity workspace`

The command remains present. The generated surface shows no flag changes:

- `--format`, `-f`
- `--repo`, `-r`
- `--direction`, `-d`
- `--url`, `-u`
- `--language`
- `--manifest`

Only the description changed from:

- `Experimental workspace relationship graph for Go modules and Rust crates`

to:

- `Workspace relationship graph for Go modules and Rust crates (experimental)`

## Arguments and aliases

The generated documentation exposed one explicit argument contract in
`v0.27.0`: `why <from> <to>` with `cobra.ExactArgs(2)`. That command is removed
at HEAD.

No command aliases were emitted by the normalized generated docs for either
revision.

## Breaking changes to call out in release notes

- `clarity extensions` was removed; use `clarity languages --format json`.
- `clarity why <from> <to>` was removed; use `clarity show --between a,b` for
  graph-based coupling/path inspection.
- `clarity show -i a,b` was removed; use positional paths:
  `clarity show a b`.
- `clarity show -p file --level N` was removed; use:
  `clarity show file --reach down --depth N`.
- `clarity show --modules` was removed; use:
  `clarity show --all --collapse` or scoped paths with `--collapse`.
- `clarity show --direction in|out|both` was removed; use:
  `clarity show --module name --reach up|down|both`.
- `clarity show --scope` was removed; use `--reach down`.
- `clarity watch -i a,b` was removed; use positional paths:
  `clarity watch a b`.
- `clarity watch --direction` was removed; use `--orientation`.

## Release-readiness notes

This comparison satisfies the surface-diff portion of `CLR-7`. The new surface
is coherent enough to review as a release candidate, but the comparison also
identifies release-note and downstream migration obligations that belong to the
remaining `CLR-2` subtasks:

- `CLR-8`: regenerate and verify `usage-clarity.md`.
- `CLR-9`: reconcile README and setup-generated agent instructions.
- `CLR-10`: update the surface migration doc so it no longer reads as an
  unimplemented proposal.
- `CLR-11`: clean stale legacy flag references in maintained docs, comments,
  test messages, and fixtures.
- `CLR-12`: verify downstream consumers such as Clarity Desktop, root
  instructions, and product copy.
- `CLR-13`: prepare release notes and make the final go/no-go decision.
