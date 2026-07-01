## Clarity

This project uses `clarity` to visualize code changes, provide design feedback, and guide refactoring.

### When to Use Clarity

1. **After making changes** - Run `clarity` to visualize your changes, understand impact, and prepare context for developer review.
    - **Always run `clarity show` when you modify 3 or more files** to ensure the developer can review the full scope of changes
2. **Discussing design** - Use `clarity` to visualize architecture and dependencies for specific files, directories, or commits when discussing design decisions with the developer.
3. **Refactoring verification** - After implementing design changes, run `clarity` to verify the resulting structure aligns with the discussed design.

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
