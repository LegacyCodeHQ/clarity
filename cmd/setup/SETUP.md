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
