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

The two tools do not measure the same relation, so the two directions of the
diff mean different things and are reported separately. Do not collapse them
into a single "differences" number.

**UNEXPLAINED oracle-only** — the actionable bucket. Edges cargo-modules found
that Clarity did not, after removing known-intended differences. Treat each as a
candidate Clarity bug and reproduce it in a minimal fixture before filing; the
mapping layer here is not authoritative.

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
