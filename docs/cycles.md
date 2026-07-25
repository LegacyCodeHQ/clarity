# Actionable cycle analysis

`clarity cycles [path...]` finds cyclic strongly connected components (SCCs) in
the scoped file-dependency graph. An SCC is a maximal group in which every file
can reach every other file. It can contain one loop, many overlapping loops, or
a self-loop.

The command reports components rather than claiming that one representative
path is the complete cycle:

```text
Found 1 cyclic component in src:

  1. 3 files, 4 internal dependencies
     Representative loop: a.ts → b.ts → a.ts
     Smallest break set (exact, 2 edges):
       - a.ts → b.ts
       - a.ts → c.ts
```

The representative loop is a deterministic orientation aid. The file and edge
counts describe the complete component.

## Finding a break

A break set is a set of directed dependency edges whose removal makes the
component acyclic. Clarity verifies every emitted set by removing its edges and
running cycle detection again.

- Components with at most 16 internal edges use exact minimum-cardinality
  search. Up to eight equivalent minimum alternatives are retained.
- Larger components use a deterministic greedy heuristic. The result is still
  verified, but it is labelled `heuristic` because a smaller set may exist.
- Equivalent exact sets are ranked by confidence first and then by the number
  of supporting source references. This makes uncertain inferred edges visible
  before recommending a larger source refactor.

Minimum edge count does not necessarily mean minimum engineering cost. One edge
may represent dozens of call sites, while a two-edge alternative may be a small
interface extraction. Use `--explain` to inspect the evidence before changing
code.

## Explaining an edge

```sh
clarity cycles src --explain
```

For every internal dependency, explain mode prints:

- the symbol or link that created the edge
- the reference file and line
- the declaration file and line
- the relationship kind
- confidence (`high`, `medium`, or `low`)

Swift evidence distinguishes owner-scoped extension members from unrelated
bare declarations. Qualified value members such as `logger.error`, leading-dot
enum cases such as `.error`, implicit `catch` error values, and
`private`/`fileprivate` declarations do not create unrelated cross-file edges.

Evidence is structural rather than a compiler proof. Clarity still does not
replace type checking or code review.

## Filtering documentation

Markdown links are valid dependency relationships and may intentionally point
in both directions. To focus on source code:

```sh
clarity cycles . --code-only
```

`--code-only` excludes `.md` and `.markdown` files before graph construction.

## JSON output

```sh
clarity cycles src --format json
```

JSON includes:

- complete component nodes and internal edges
- evidence for each internal edge
- the representative loop
- `exact` or `heuristic` break analysis
- every retained break-set alternative

Paths are relative to the requested scope. `--url` is intentionally incompatible
with JSON output.

## Visualization

```sh
clarity cycles src --url
```

Each URL contains the complete component, not only the representative loop.
Cycle-related output remains experimental and may evolve.
