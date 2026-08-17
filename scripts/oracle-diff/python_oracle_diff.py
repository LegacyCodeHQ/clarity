#!/usr/bin/env python3
"""Diff Clarity's Python dependency graph against grimp as an oracle.

grimp (https://github.com/seddonym/grimp, backs import-linter) builds a Python
import graph via static AST analysis -- it does not execute the target code, so
it works without the package's runtime dependencies installed. See README.md
for what this can and cannot tell you.

    python3 scripts/oracle-diff/python_oracle_diff.py /path/to/python/repo

Exits non-zero when unexplained oracle-only edges are found.
"""

import argparse
import os
import re
import subprocess
import sys

CLARITY_EDGE = re.compile(r'"([^"]+)" -> "([^"]+)"')


def run(cmd, cwd=None):
    proc = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True)
    if proc.returncode != 0:
        raise RuntimeError(f"{' '.join(cmd)} failed:\n{proc.stderr[-2000:]}")
    return proc.stdout


def discover_packages(repo, src_root):
    """Top-level package names directly under src_root, plus its absolute path.

    Heuristic, not authoritative: a directory is a top-level package if it
    contains __init__.py directly. This misses packages declared via
    setuptools `package_dir` remapping or namespace packages (PEP 420, no
    __init__.py) -- both are documented limitations, not silent gaps.
    """
    base = os.path.join(repo, src_root) if src_root else repo
    if not os.path.isdir(base):
        return base, []
    packages = []
    for name in sorted(os.listdir(base)):
        if os.path.isfile(os.path.join(base, name, "__init__.py")):
            packages.append(name)
    return base, packages


def module_to_relpath(module_name, base_dir):
    """Resolve a dotted module name to a file, by filesystem convention only.

    No import/execution -- this is what makes the oracle usable without the
    target package's runtime dependencies installed, unlike importlib-based
    resolution.
    """
    rel = module_name.replace(".", os.sep)
    file_candidate = os.path.join(base_dir, rel + ".py")
    if os.path.isfile(file_candidate):
        return os.path.relpath(file_candidate, base_dir)
    pkg_candidate = os.path.join(base_dir, rel, "__init__.py")
    if os.path.isfile(pkg_candidate):
        return os.path.relpath(pkg_candidate, base_dir)
    return None


def oracle_edges(base_dir, packages):
    import grimp

    sys.path.insert(0, base_dir)
    edges = set()
    for package in packages:
        graph = grimp.build_graph(package, include_external_packages=False)
        for module in graph.modules:
            for target in graph.find_modules_directly_imported_by(module):
                src = module_to_relpath(module, base_dir)
                dst = module_to_relpath(target, base_dir)
                if src and dst and src != dst:
                    edges.add((src, dst))
    return edges


def clarity_edges(repo, clarity_bin, src_root):
    args = [clarity_bin, "show"]
    if src_root:
        args.append(src_root)
    args += ["-f", "dot"]
    dot = run(args, cwd=repo)
    prefix = src_root.rstrip(os.sep) + os.sep if src_root else ""
    edges = set()
    for a, b in CLARITY_EDGE.findall(dot):
        # Normalize to the oracle's base -- clarity's paths are repo-relative,
        # the oracle's are relative to base_dir (repo/src_root).
        if prefix:
            a = a[len(prefix):] if a.startswith(prefix) else a
            b = b[len(prefix):] if b.startswith(prefix) else b
        if a != b:
            edges.add((a, b))
    return edges


def preflight(clarity_bin):
    try:
        import grimp  # noqa: F401
    except ImportError:
        sys.exit("error: grimp is not installed. pip install grimp")
    try:
        subprocess.run([clarity_bin, "--version"], capture_output=True, check=True)
    except (OSError, subprocess.CalledProcessError):
        sys.exit(f"error: clarity binary {clarity_bin!r} not runnable. "
                  f"Pass --clarity /path/to/build.")


def main():
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("repo", help="path to the Python repo to check")
    ap.add_argument("--src-root", default="src",
                    help="directory (relative to repo) that top-level packages "
                         "live under; pass '' for a flat/no-src-dir layout")
    ap.add_argument("--clarity", default="clarity",
                    help="clarity binary (point at a fresh build when testing changes)")
    ap.add_argument("--verbose", action="store_true",
                    help="also list the oracle's blind spot")
    args = ap.parse_args()

    repo = os.path.abspath(args.repo)
    preflight(args.clarity)

    base_dir, packages = discover_packages(repo, args.src_root)
    if not packages:
        sys.exit(f"error: no top-level packages (dirs with __init__.py) found "
                  f"under {base_dir!r}. Pass --src-root '' for a flat layout, or "
                  f"a different --src-root.")

    oracle = oracle_edges(base_dir, packages)
    clarity = clarity_edges(repo, args.clarity, args.src_root)

    only_oracle = oracle - clarity
    blind_spot = clarity - oracle

    agree = len(oracle & clarity)
    print(f"packages   {len(packages)}  ({', '.join(packages)})")
    print(f"oracle     {len(oracle)}")
    print(f"clarity    {len(clarity)}")
    print(f"agree      {agree}" + (f"   recall {agree / len(oracle):.0%}" if oracle else ""))
    print()
    print(f"UNEXPLAINED oracle-only edges: {len(only_oracle)}   <-- candidate Clarity misses")
    for a, b in sorted(only_oracle):
        print(f"   - {a} -> {b}")

    print(f"\noracle blind spot (clarity-only): {len(blind_spot)}")
    print("   grimp tracks import statements, not attribute/call expressions.")
    print("   Clarity resolves some of those independently (e.g. wildcard re-export")
    print("   chains); these are NOT necessarily Clarity false positives.")
    if args.verbose:
        for a, b in sorted(blind_spot):
            print(f"   + {a} -> {b}")

    return 1 if only_oracle else 0


if __name__ == "__main__":
    sys.exit(main())
