What this command does:
  - Adds a minimal sanity snippet to AGENTS.md
  - (Optional) Adds the same snippet to .github/copilot-instructions.md

How it relates to other commands:
  - sanity onboard  -> shows the minimal snippet without changing files
  - sanity prime    -> prints the full workflow context for this tool

Quick start:
  - Run 'sanity prime' after a new session or context reset
  - Run 'sanity graph -u' to visualize dependencies in a browser
  - Run 'sanity graph -f mermaid' to render in IDEs / applications that support mermaid
  - Run 'sanity graph' to output Graphviz (dot) for tools that can render it

Re-running init:
  - Use --force to overwrite files instead of appending
