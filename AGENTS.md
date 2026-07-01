# Agent Instructions

## Committing Changes

- Do **not** commit changes unless the user explicitly asks for a commit.
- This applies even at session completion.

## Session Completion

**When ending a work session**, you MUST complete ALL steps below.

**MANDATORY WORKFLOW:**

1. **Run quality gates** (if code changed) - Tests, linters, builds:
   ```bash
   make lint
   make test
   ```
2. **Hand off** - Provide context for next session

## Commit Guidelines

Follow the established history patterns:

- Use `type: subject` in lowercase with a short, imperative subject line.
- Keep subjects concise (ideally under ~72 characters).
- One logical change per commit.
- Prefer these types (use only when accurate):
  - `feat`: user-visible feature
  - `fix`: bug fix or correctness issue
  - `lint`: lint-only fixes and style-rule compliance
  - `refactor`: behavior-preserving code restructuring
  - `test`: tests or fixtures only
  - `docs`: documentation only
  - `build`: build system or tooling changes
  - `ci`: CI pipeline changes
  - `chore`: non-functional maintenance (e.g., syncs)
  - `skill`: skill/workflow instructions and skill assets
  - `style`: formatting-only changes (non-lint)
  - `human`: human-led or non-mechanical changes (use sparingly)

## Clarity

This project uses `clarity` to visualize design and guide structural refactoring.

### When to Use Clarity

1. **Discussing design** - Use `clarity` to visualize architecture and dependencies for specific files, directories, or commits when discussing design decisions with the developer.
2. **Refactoring verification** - After implementing design changes, run `clarity` to verify the resulting structure aligns with the discussed design.

### How to Use Clarity

**For developer review (visualize):**
- Generate and render graphs for the developer to review
- For CLI agents, default to DOT output (`clarity show` or `clarity show -f dot`)
- For CLI agents, generate a URL with `clarity show -u`, then open that URL in the system browser with the platform command:
  - macOS: `open "<url>"`
  - Linux: `xdg-open "<url>"`
  - Windows (cmd): `start "" "<url>"`
  - Windows (PowerShell): `Start-Process "<url>"`
- Use `clarity show -f mermaid` if your environment supports Mermaid rendering (desktop apps, IDEs)
- Use `clarity show` or `clarity show -f dot` if your environment supports Graphviz rendering or has dot tools installed (supports SVG, PNG, etc.)
- Do not assume `clarity show -u` auto-opens a browser in CLI environments; always open the generated URL explicitly
- Choose the visualization method that works best for your coding environment

**For agent verification (feedback and analysis):**
- Run `clarity show` and read the dot/mermaid output directly
- Parse the graph structure to verify dependencies and relationships
- No visualization needed - the text output contains all structural information
- Use this during refactoring iterations to confirm progress
- Before refactoring a file, inspect its dependents with `clarity show <file> --reach up`

### Quick Reference

```bash
clarity show                                 # Visualize uncommitted changes (most common)
clarity show -c HEAD                         # Visualize changes in last commit
clarity show <files/dirs>                    # Build graph from specific files or directories (space-separated)
clarity show <file> --reach up               # Find what depends on a file before changing it
clarity show <file> --reach both --depth 2   # Bounded blast radius around a file
clarity show --all --collapse                # Whole-tree view collapsed into declared modules
clarity show --between <a,b>                  # Find all paths between two or more files (comma-separated)
clarity show -f mermaid                      # Output in mermaid format (default 'dot' Graphviz format)
clarity show -u                              # Generate visualization URL
```

For full reference, use `clarity show -h`

## Codebase Notes

- Language: Go, plus a web UI build with npm in `cmd/watch/web`
- The web UI is served by the Go web server
- Entry points:
  - Go: `main.go`
  - Frontend: `cmd/watch/web`
- Common commands (from `clarity/Makefile`):
  - `make lint`
  - `make test`
  - `make test-web`
  - `make build-dev`
