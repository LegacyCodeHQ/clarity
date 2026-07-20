#!/usr/bin/env python3
"""Diff Clarity's Rust dependency graph against cargo-modules as an oracle.

See README.md for what this can and cannot tell you. In short: the two tools
measure related but different relations, so the two directions of the diff mean
different things and are reported separately.

    python3 scripts/oracle-diff/oracle_diff.py /path/to/rust/repo

Exits non-zero when unexplained oracle-only edges are found.
"""

import argparse
import json
import os
import re
import subprocess
import sys

ORACLE_EDGE = re.compile(r'"([\w:]+)" -> "([\w:]+)" \[label="uses"')
CLARITY_EDGE = re.compile(r'"([^"]+)" -> "([^"]+)"')


def run(cmd, cwd=None):
    proc = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True)
    if proc.returncode != 0:
        raise RuntimeError(f"{' '.join(cmd)} failed:\n{proc.stderr[-2000:]}")
    return proc.stdout


def crate_targets(repo):
    """Every lib/bin target, with its root module name and source dir.

    The root module is the TARGET name, not the package name. taskd-cli's bin
    target is `taskd`; deriving it from the package silently drops the whole
    crate, and the resulting diff still looks plausible.
    """
    meta = json.loads(run(["cargo", "metadata", "--no-deps", "--format-version", "1"], cwd=repo))
    targets = []
    for pkg in meta["packages"]:
        for tgt in pkg["targets"]:
            if tgt["kind"][0] in ("lib", "bin"):
                targets.append({
                    "package": pkg["name"],
                    "kind": tgt["kind"][0],
                    # cargo takes the target name verbatim; Rust module paths
                    # use the underscored form. They are not interchangeable.
                    "name": tgt["name"],
                    "module": tgt["name"].replace("-", "_"),
                    "src_dir": os.path.dirname(tgt["src_path"]),
                })
    return targets


def module_file_map(repo, targets):
    """module path tuple -> repo-relative file."""
    mapping = {}
    for tgt in targets:
        for dirpath, _, names in os.walk(tgt["src_dir"]):
            for name in names:
                if not name.endswith(".rs"):
                    continue
                full = os.path.join(dirpath, name)
                parts = os.path.relpath(full, tgt["src_dir"])[: -len(".rs")].split(os.sep)
                if parts[-1] in ("mod", "main", "lib"):
                    parts = parts[:-1]
                mapping.setdefault(tuple([tgt["module"]] + parts), os.path.relpath(full, repo))
    return mapping


def to_file(mod_path, mapping, unmapped):
    """Resolve an oracle module path to a file.

    Inline modules (`mod tests {}`) own no file, so peel trailing segments
    until one matches. That collapses them onto their parent file, which is
    how Clarity attributes them.
    """
    parts = tuple(mod_path.split("::"))
    for cut in range(len(parts), 0, -1):
        if parts[:cut] in mapping:
            return mapping[parts[:cut]]
    unmapped.add(mod_path)
    return None


def oracle_edges(repo, targets, mapping):
    unmapped, edges = set(), set()
    for tgt in targets:
        flag = ["--lib"] if tgt["kind"] == "lib" else ["--bin", tgt["name"]]
        dot = run(["cargo", "modules", "dependencies", "-p", tgt["package"], *flag,
                   "--no-externs", "--no-sysroot", "--no-owns", "--cfg-test"], cwd=repo)
        for a, b in ORACLE_EDGE.findall(dot):
            fa, fb = to_file(a, mapping, unmapped), to_file(b, mapping, unmapped)
            if fa and fb and fa != fb:
                edges.add((fa, fb))
    return edges, unmapped


def clarity_edges(repo, clarity_bin):
    dot = run([clarity_bin, "show", ".", "-f", "dot"], cwd=repo)
    edges = set()
    for a, b in CLARITY_EDGE.findall(dot):
        # Collapse inline-module nodes (`foo.rs::tests`) onto their file.
        a, b = a.split("::")[0], b.split("::")[0]
        if a != b:
            edges.add((a, b))
    return edges


def owned_dir(path):
    """The directory whose child modules belong to this file."""
    if os.path.basename(path) in ("mod.rs", "lib.rs", "main.rs"):
        return os.path.dirname(path)
    return path[: -len(".rs")]


def is_declared_child(src, dst):
    """True when dst is simply a submodule that src declares.

    Decision A (CLR-23): a `mod X;` declaration is containment, not a
    dependency. cargo-modules reports these; Clarity does not, by design. If
    consumers reach the child directly, Clarity already draws that edge, and a
    parent edge would only restate it.
    """
    return os.path.dirname(dst) == owned_dir(src)


def crate_of(path):
    parts = path.split(os.sep)
    return parts[1] if parts[0] == "crates" else parts[0]


def preflight(clarity_bin):
    """Fail early with something actionable rather than a traceback."""
    for cmd, hint in (
        (["cargo", "modules", "--version"],
         "cargo-modules is missing or too old. It needs rustc >= 1.91, which is "
         "newer than the superepo default: cargo +1.95.0 install cargo-modules"),
        ([clarity_bin, "--version"],
         f"clarity binary {clarity_bin!r} not runnable. Pass --clarity /path/to/build."),
    ):
        try:
            subprocess.run(cmd, capture_output=True, check=True)
        except (OSError, subprocess.CalledProcessError):
            sys.exit(f"error: {hint}")


def main():
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("repo", help="path to the Rust repo to check")
    ap.add_argument("--clarity", default="clarity",
                    help="clarity binary (point at a fresh build when testing changes)")
    ap.add_argument("--verbose", action="store_true",
                    help="also list expected divergences and the oracle's blind spot")
    args = ap.parse_args()

    repo = os.path.abspath(args.repo)
    preflight(args.clarity)
    targets = crate_targets(repo)
    mapping = module_file_map(repo, targets)
    oracle, unmapped = oracle_edges(repo, targets, mapping)
    clarity = clarity_edges(repo, args.clarity)

    if unmapped:
        print(f"!! {len(unmapped)} module paths did not map to a file — the diff below is "
              f"incomplete. First few: {sorted(unmapped)[:5]}\n")

    cross = {e for e in clarity if crate_of(e[0]) != crate_of(e[1])}
    only_oracle = oracle - clarity
    expected = {e for e in only_oracle if is_declared_child(*e)}
    unexplained = only_oracle - expected
    blind_spot = clarity - oracle - cross

    # Recall is measured against IN-SCOPE oracle edges: the raw oracle set
    # minus the containment edges Clarity does not model by design. Dividing by
    # the raw set penalises a repo for using a legitimate idiom -- lever-cli
    # scored 89% purely because issue.rs declares ten submodules without
    # re-exporting them, while having no actual misses.
    #
    # Note `expected` is a subset of oracle-only, so parent->child edges that
    # Clarity *does* draw (via a `pub use`) stay counted on both sides.
    in_scope = oracle - expected
    agree = len(oracle & clarity)

    print(f"targets    {len(targets)}  ({', '.join(t['module'] for t in targets)})")
    print(f"oracle     {len(oracle)}   ({len(in_scope)} in scope, "
          f"{len(expected)} excluded by design)")
    print(f"clarity    {len(clarity)}")
    print(f"agree      {agree}" + (f"   recall {agree / len(in_scope):.0%} of in-scope edges"
                                   if in_scope else ""))
    print()
    print(f"UNEXPLAINED oracle-only edges: {len(unexplained)}   <-- candidate Clarity misses")
    for a, b in sorted(unexplained):
        print(f"   - {a} -> {b}")

    print(f"\nexpected divergences (mod-decl containment, decision A): {len(expected)}")
    if args.verbose:
        for a, b in sorted(expected):
            print(f"   = {a} -> {b}")

    print(f"oracle blind spot (clarity-only, same-crate): {len(blind_spot)}")
    print("   these are mostly qualified-path expressions cargo-modules cannot see.")
    print("   they are NOT Clarity false positives — do not read this as noise.")
    if args.verbose:
        for a, b in sorted(blind_spot):
            print(f"   + {a} -> {b}")
    print(f"cross-crate clarity edges (oracle runs per-crate, structurally blind): {len(cross)}")

    return 1 if unexplained else 0


if __name__ == "__main__":
    sys.exit(main())
