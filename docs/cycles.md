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

Human output shows the first 20 references per edge and reports how many remain;
JSON retains the complete evidence set. This keeps large, frequently referenced
types readable without weakening automation.

Evidence adapters currently provide symbols and lines for:

- Swift types, symbols, and owner-scoped extension members
- Rust module declarations, imports, re-exports, calls, and type references
- Go imported-package and same-package calls, types, values, and embeddings
- TypeScript/JavaScript runtime imports, type imports, re-exports, calls, and
  inheritance
- Kotlin imported and same-package calls, types, inheritance, and
  companion-style access
- HTML navigation, scripts, stylesheets, images, and embedded resources

Relationships use a stable cross-language taxonomy:

`resolved-dependency`, `module-declaration`, `import`, `type-import`,
`re-export`, `call`, `type-reference`, `symbol-reference`, `inheritance`,
`extension-member`, `companion-member`, `navigation`, `script`, `stylesheet`,
`image`, and `embedded-resource`.

When a language adapter cannot establish a more precise relationship, Clarity
retains a medium-confidence `resolved-dependency` fallback rather than
presenting a guess as fact.

Evidence is structural rather than a compiler proof. Clarity still does not
replace type checking or code review.

## Filtering relationships

Semantic filters are applied to evidence before cycle detection. Clarity then
rebuilds the graph, finds SCCs again, and recomputes verified break sets:

```sh
# Ignore normal Rust module containment.
clarity cycles . --exclude-kind module-declaration

# Show cycles sustained specifically by calls.
clarity cycles . --include-kind call

# Ignore reciprocal website navigation while retaining scripts and assets.
clarity cycles . --exclude-kind navigation
```

Flags accept comma-separated values and may be repeated. Exclusion wins when a
kind appears in both lists. Active filters are included in JSON output.

An edge can have several evidence kinds. Removing `import` evidence does not
remove that edge when a retained call or type reference still sustains the same
file dependency.

## Filtering files

Markdown links are valid dependency relationships and may intentionally point
in both directions. To focus on source code:

```sh
clarity cycles . --code-only
```

`--code-only` excludes `.md` and `.markdown` files before graph construction.
HTML remains included; use `--exclude-kind navigation` when the files matter
but reciprocal page navigation does not.

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
- active include/exclude relationship filters

Paths are relative to the requested scope. `--url` is intentionally incompatible
with JSON output.

## Visualization

```sh
clarity cycles src --url
```

Each URL contains the complete component, not only the representative loop.
Cycle-related output remains experimental and may evolve.
