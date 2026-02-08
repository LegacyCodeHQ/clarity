# Agent Instructions

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

## Committing Changes

- Do **not** commit changes unless the user explicitly asks for a commit.
- This applies even at session completion.

## Quick Reference

```bash
bd ready                             # Find available work
bd show <id>                         # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>                        # Complete work
bd sync                              # Sync with git
```

## Session Completion

**When ending a work session**, you MUST complete ALL steps below.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds:
   ```bash
   make lint
   make test
   ```
3. **Update issue status** - Close finished work, update in-progress items
   ```bash
   bd sync
   ```
4. **Hand off** - Provide context for next session

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

## Sanity

This project uses `sanity` to visualize code changes, provide design feedback, and guide refactoring.

### When to Use Sanity

1. **After making changes** - Run `sanity` to visualize your changes, understand impact, and prepare context for developer review.
  - **Always run `sanity graph` when you modify 3 or more files** to ensure the developer can review the full scope of changes
2. **Discussing design** - Use `sanity` to visualize architecture and dependencies for specific files, directories, or commits when discussing design decisions with the developer.
3. **Refactoring verification** - After implementing design changes, run `sanity` to verify the resulting structure aligns with the discussed design.

### How to Use Sanity

**For developer review (visualize):**
- Generate and render graphs for the developer to review
- For CLI agents, default to DOT output (`sanity graph` or `sanity graph -f dot`)
- For CLI agents, generate a URL with `sanity graph -u`, then open that URL in the system browser with the platform command:
  - macOS: `open "<url>"`
  - Linux: `xdg-open "<url>"`
  - Windows (cmd): `start "" "<url>"`
  - Windows (PowerShell): `Start-Process "<url>"`
- Use `sanity graph -f mermaid` if your environment supports Mermaid rendering (desktop apps, IDEs)
- Use `sanity graph` or `sanity graph -f dot` if your environment supports Graphviz rendering or has dot tools installed (supports SVG, PNG, etc.)
- Do not assume `sanity graph -u` auto-opens a browser in CLI environments; always open the generated URL explicitly
- Choose the visualization method that works best for your coding environment

**For agent verification (feedback and analysis):**
- Run `sanity graph` and read the dot/mermaid output directly
- Parse the graph structure to verify dependencies and relationships
- No visualization needed - the text output contains all structural information
- Use this during refactoring iterations to confirm progress

### Quick Reference

```bash
sanity graph                   # Visualize uncommitted changes (most common)
sanity graph -c HEAD           # Visualize changes in last commit
sanity graph -i <files/dirs>   # Build graph from specific files or directories (comma-separated)
sanity graph -w <file1,file2>  # Find all paths between two or more files (comma-separated)
sanity graph -f mermaid        # Generate output in mermaid format (default 'dot' Graphviz format)
sanity graph -u                # Generate visualization URL
```

For full reference, use `sanity graph -h`
