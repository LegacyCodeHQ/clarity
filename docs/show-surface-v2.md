# `clarity show` — command surface redesign (v2.1 spec)

> Status: design proposal. Not yet implemented. Supersedes the v2 draft;
> v2.1 corrects the default-anchor semantics under `--commit` and re-frames the
> implementation work around centralizing snapshot selection.

## Intent

Replace the current 15-flag surface — which scrambles *snapshot*, *anchor*,
*traversal*, *rendering*, and *filters* — with a grammar of **four orthogonal
questions**, and unify the three different spellings of "reach" into one.

## Organizing principle

A `show` invocation answers four questions, **at most one answer per group**:

| Group | Question | Default |
|---|---|---|
| **SNAPSHOT** | which tree in history? | working tree |
| **ANCHOR** | what am I looking at? | working-set changes (see below) |
| **LENS** | how do I traverse/shape it? | as-is |
| **RENDER** | how is it drawn? | dot |

## The surface

**SNAPSHOT** — where in history *(default: working tree)*
- `--commit / -c <ref | a..b>` — a commit or range; the anchor is resolved
  **within this tree**

**ANCHOR** — what to look at *(mutually exclusive)*
- `[paths…]` *(positional)* — files/dirs, enumerated · *replaces `-i`*
- `--module / -m <name>` — a declared module · *gives `--module` a short flag*
- `--between <a,b,…>` — the connecting paths · *replaces `-w`*
- `--all` — the whole tree at this snapshot

Default anchor depends on the snapshot:
- working tree → **working-set changes** (uncommitted files)
- `--commit X` → **the files changed in commit/range X** (not the whole tree)
- the whole tree at any snapshot requires `--all` explicitly

**LENS** — how to traverse the anchor *(mutually exclusive; default: as-is)*
- `--reach up | down | both` — walk the graph from the anchor
  - `--depth / -l <n>` — hops (default `1`, `0` = unlimited)
  - `--prune <paths>` — stop the walk at these
- `--collapse` — fold declared modules into single nodes · *replaces `--modules`*

**RENDER**
- `--format / -f`, `--url / -u`, `--orientation`, `--label`, `--no-stats`,
  `--no-phantom`

**FILTERS** (cross-cutting, compose with any anchor)
- `--exclude <paths>`, `--include-ext`, `--exclude-ext`

## The reach unification

One vocabulary replaces three. Defined once, against the import direction:

| Term | Meaning | Replaces |
|---|---|---|
| `down` | what the anchor **depends on** (imports / callees) | file `--scope downstream`, module `out` |
| `up` | what **depends on** the anchor (importers / callers) | module `in`; **new for files** |
| `both` | neighbors on both sides | module `both` |

No `--reach` → anchor shown as-is. This unlocks **upstream file reach**, which
today's dead `--scope` enum (single value `downstream`) was clearly architected
for but never exposed.

## Anchor × Lens validity

| Anchor ↓ \ Lens → | as-is | `--reach` | `--collapse` |
|---|---|---|---|
| `[paths…]` | ✓ region | ✓ cone from paths | ✓ fold modules in region |
| `--module` | ✓ members boxed | ✓ members + up/down neighbors | ✗ (focus vs. fold) |
| `--between` | ✓ connecting paths | ✗ (between *is* a traversal) | ✗ |
| `--all` | ✓ whole tree | ✗ (reach from everything = everything) | ✓ **the architecture overview** |
| *(default: changes)* | ✓ changed files | ✓ reach from changes | ✓ |

`--reach` and `--collapse` are mutually exclusive (both are LENS).
`--commit` composes with every anchor.

## Old → new, by intent

| Intent | Today | Redesigned |
|---|---|---|
| The auth folder | `show -i src/auth` | `show src/auth` |
| What login.go pulls in | `show -p login.go` | `show login.go --reach down` |
| **Who uses login.go** | *(not possible)* | `show login.go --reach up` |
| Module + its deps as context | `show --module auth -d out` | `show -m auth --reach down` |
| Module + everything around it | `show --module auth -d both` | `show -m auth --reach both` |
| Architecture overview | `show --modules` | `show --all --collapse` |
| Paths between two files | `show -w a,b` | `show --between a,b` |
| Auth folder, 2 hops down | *(not possible)* | `show src/auth --reach down --depth 2` |
| Changed files in a commit | `show -c HEAD` | `show -c HEAD` |
| Whole tree as of a commit | *(implicit via `-i .`)* | `show -c HEAD --all` |

## Implementation: the grammar is necessary but **not sufficient**

Renaming flags fixes nothing on its own. The current correctness bugs live in
the fact that **snapshot selection is decentralized**: three concerns each pick
their tree independently, so under `--commit` they can disagree.

### The current state under `--commit`

| Concern | `-c X -p file` | `-c X -m mod --direction` |
|---|---|---|
| File universe | commit tree (filtered) | **working tree** via `expandPaths(repoPath)` (`show_cmd.go:219`) |
| Module membership | n/a | **filesystem** config + `os.Stat` (`show_cmd.go:1152`) |
| Content reader | **filesystem** — fallback fires for `targetFile` (`show_cmd.go:752`) | commit tree (`GitCommitContentReader`) ✓ |

So the two views fail *differently*:
- `-c X -p file` reads file **content** from the working tree, not commit `X`.
- `-c X -m mod --direction` discovers the neighbor **universe** and **membership**
  from the working tree, while reading content from commit `X` — so a file present
  in the working tree but absent from `X` enters the graph and then content-reads
  to nothing.

Same root, opposite symptom: there is no single source of truth for "the
snapshot."

### The fix

Introduce **one snapshot-scoped resolver** that owns, for the chosen snapshot:
1. tree file listing
2. module membership resolution
3. the content reader

Then apply anchors and lenses over that resolver. With this in place:
- `selectContentReader`'s `targetFile` filesystem fallback is deleted.
- The `--module --direction` whole-repo `expandPaths(repoPath)` reparse is deleted;
  neighbors are discovered over the already-built graph of the chosen
  snapshot+anchor.
- Any future lens inherits correct snapshot behavior for free.

### Also delete / make explicit

- **`--modules` / `--module` silent precedence** (`show_cmd.go:197, 293 vs 309`):
  passing both silently prefers `--module`. Replace with an explicit
  `--collapse` ⊻ `--module` validation error.
- **Vestigial `--scope` enum** (`show_cmd.go:117, 473–479`): single value,
  subsumed by `--reach down`. Remove.

## Migration

Breaking change — golden fixtures, tests, and the desktop app shell out to these
flags. The path taken was **additive → deprecate → remove**, and is now
**complete**:

1. Landed positional `[paths…]`, `--reach`, `--collapse`, `--all`, and the
   snapshot-resolver centralization (the bug fix, valuable on its own).
2. Removed the dead `--scope` and converted `--input`, `--file`, `--level`,
   `--modules`, `--direction` to deprecation warnings.
3. Migrated the internal callers (desktop app, docs, tests) onto the new
   grammar, then removed the deprecated flags entirely.

There are no remaining legacy aliases. (`-w`/`--between` was never legacy — it
is the current anchor flag.) Mapping from the old surface:

- `-i <paths>` → positional `<paths>`
- `-p <file>` → `<file> --reach down`
- `--level` → `--depth` / `-l`
- `--modules` → `--collapse`
- `--direction in|out|both` → `--reach up|down|both` (on a `--module`)
- `--scope` → removed (it was `--reach down`, and was already dead)

## `watch`

`watch` is "`show` over a live working-tree snapshot" — the same grammar,
re-rendered on change. **Implemented** (`feat: add watch show grammar parity`):
`watch` now accepts positional `[paths…]`, `--between`, `--module / -m`,
`--reach`, `--depth / -l`, `--prune`, `--all`, and `--collapse`, alongside the
existing filters, format/orientation, port/repo, and `--no-phantom`. Its legacy
flags (`--input`, `--modules`) have been removed, matching `show`.

The one intentional difference from `show`: `watch` has **no `--commit / -c`**.
A live view is always the working tree, so there is no historical snapshot to
select — the SNAPSHOT axis collapses to "the working tree, right now."

## Deferred

- **Declarative module boundaries** (partition vs. reach) — out of scope here;
  `--module` reads `.clarity/modules.json` as-is. The grammar does not depend on
  resolving that fork.
- **File-count semantics** — "count what's drawn" falls out once the working-tree
  reparse in the module path is gone.
