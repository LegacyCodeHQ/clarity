This directory holds per-language oracle-diff harnesses: scripts that
cross-check Clarity's dependency graph for one language against an
independent, non-Clarity tool. See CLR-23 for why this exists and CLR-64 for
the Python pass. Jump to [Python oracle diff](#python-oracle-diff) below.

# Rust oracle diff

Cross-checks Clarity's Rust dependency graph against
[cargo-modules](https://github.com/regexident/cargo-modules), used as an
independent oracle. Run it on demand after changing the Rust extractor.

It found CLR-24 through CLR-28.

```sh
python3 scripts/oracle-diff/oracle_diff.py /path/to/rust/repo
python3 scripts/oracle-diff/oracle_diff.py ../../taskd --clarity ./clarity --verbose
```

Exits non-zero when there are unexplained oracle-only edges. Use `--clarity` to
point at a fresh build — the default picks up whatever `clarity` is on `PATH`,
which is rarely the build you are testing.

## Requirements

- `cargo-modules`, which needs **rustc >= 1.91**. The superepo default toolchain
  is older, so install it with a newer one:
  `cargo +1.95.0 install cargo-modules`
- The target repo must **compile**. See Limitations.

## Reading the output

`recall` is measured against **in-scope** oracle edges — the raw oracle set minus
the containment edges Clarity does not model by design. Dividing by the raw set
penalises a repo for using a legitimate idiom: lever-cli scored 89% purely
because `issue.rs` declares ten submodules without re-exporting them, while
having no actual misses. The header prints both figures so the exclusion is
visible rather than assumed.

The two tools do not measure the same relation, so the two directions of the
diff mean different things and are reported separately. Do not collapse them
into a single "differences" number.

**UNEXPLAINED oracle-only** — the actionable bucket. Edges cargo-modules found
that Clarity did not, after removing known-intended differences. Treat each as a
candidate Clarity bug and reproduce it in a minimal fixture before filing; the
mapping layer here is not authoritative.

Some of these are **oracle artifacts**, not Clarity bugs. On ripgrep the oracle
reports `core/search.rs -> core/flags/hiargs.rs`, but search.rs contains no
occurrence of "flags" at all and imports only `std` and external crates — there
is nothing for Clarity to have missed. Always check the source before believing
the oracle. "The oracle says so" is not evidence on its own.

**Expected divergences** — `mod X;` declarations. cargo-modules reports these;
Clarity does not draw them, by design. A `mod` declaration is containment, not
dependency: if consumers reach the child directly, Clarity already draws that
edge, and a parent edge would only restate it, turning every `mod.rs` into a hub.
(CLR-23, decision A.)

**Oracle blind spot** — Clarity-only edges. cargo-modules tracks `use`
*declarations* and cannot see fully-qualified call expressions
(`crate::db::open(...)` with no `use`). Clarity recovers those. **These are not
Clarity false positives.** Reading them as noise is the single most likely way
to misuse this tool.

**Cross-crate** — cargo-modules runs per crate, so cross-crate edges are
structurally invisible to it. Excluded rather than reported as misses.

## Limitations

- **Green, committed states only.** The oracle needs the crate to build. Clarity
  does not — it works on a dirty tree, an arbitrary commit, a partial checkout.
  Those are Clarity's most common uses and this harness cannot cover them.
- **Integration test targets** (`tests/*.rs`) are separate crates and are not
  walked; their edges surface as Clarity-only noise.
- **`#[path]` attributes** are not handled by the module-to-file mapping.
- The mapping collapses inline modules (`mod tests {}`) onto their parent file,
  matching how Clarity attributes them. Without this, `::tests` nodes dominate
  the diff.

## Notes for whoever extends this

Crate roots come from `cargo metadata` **target** names, not package names —
taskd-cli's binary target is `taskd`. Deriving them from package names silently
drops an entire crate into "unmapped" while still producing a plausible-looking
diff. The script warns loudly when anything fails to map; that warning means the
numbers below it are incomplete, not that a few edges were skipped.

# Python oracle diff

Cross-checks Clarity's Python dependency graph against
[grimp](https://github.com/seddonym/grimp) (which backs
[import-linter](https://github.com/seddonym/import-linter)), used as an
independent oracle. grimp builds its import graph via static AST analysis — it
does not execute the target package, so it works without that package's
runtime dependencies installed. Run it on demand after changing the Python
extractor.

It found CLR-65.

```sh
pip install grimp
python3 scripts/oracle-diff/python_oracle_diff.py /path/to/python/repo
python3 scripts/oracle-diff/python_oracle_diff.py ../../some-repo --src-root '' --clarity ./clarity --verbose
```

Exits non-zero when there are unexplained oracle-only edges. Use `--clarity` to
point at a fresh build — the default picks up whatever `clarity` is on `PATH`.
Use `--src-root ''` for a flat/no-`src`-directory layout; the default `src`
matches the common `src/<package>/` convention.

## Requirements

- `pip install grimp` (tested against grimp on Python 3.9+; no other runtime
  dependency of the target package needs to be installed).

## Reading the output

Unlike the Rust harness, there is no "expected divergence" bucket here: Python
packages have no equivalent of a `mod X;` declaration that is containment
rather than dependency, so every module-level import the oracle sees is a real
edge Clarity should draw too.

**UNEXPLAINED oracle-only** — the actionable bucket. Imports grimp found that
Clarity did not. Treat each as a candidate Clarity bug and check the source
before filing — grimp only sees import statements, so an oracle-only edge is
almost always a real parser or resolver gap, not an artifact.

**Oracle blind spot** — Clarity-only edges. grimp tracks import statements; it
cannot see attribute or call expressions, and Clarity's resolver recovers a
few shapes independently (e.g. some re-export chains). **These are not
automatically Clarity false positives** — check the source before assuming
noise, the same discipline the Rust harness's blind-spot bucket needs.

## Limitations

- **Package discovery is a filesystem heuristic**, not read from
  `pyproject.toml`/`setup.cfg`. A directory under `--src-root` counts as a
  top-level package only if it contains `__init__.py` directly. This misses
  `package_dir` remapping and PEP 420 namespace packages (no `__init__.py`) —
  both are gaps in what this harness can validate, not silent gaps in what it
  reports: it fails loudly with "no top-level packages found" rather than
  reporting an empty, misleadingly-clean diff.
- **Module-to-file resolution is filesystem-convention-only** (dotted name →
  path, `.py` or `/__init__.py`), deliberately not `importlib`-based, so the
  harness does not need the target package's dependencies installed. This
  matches how Clarity itself resolves imports, but means neither tool can see
  a runtime `sys.path` hack.
- Validated so far against two single-package `src`-layout repos (psf/requests,
  pallets/click). A multi-package layout and a flat (no `src/`) layout have not
  been run yet — see CLR-64.

## Notes for whoever extends this

The single-name and dotted-name relative-import forms look identical at a
glance but are structurally different in the parse tree: `from .models import
X` puts the dots and the module name in one `relative_import` node, while
`from . import X` puts only the dots there and leaves `X` as a sibling node
after the `import` keyword. CLR-65 was exactly this distinction not being
handled. If you add another shape to the oracle diff, check the actual parse
tree (`go run` a throwaway program against
`github.com/smacker/go-tree-sitter/python`) before assuming the two forms
share a code path.
