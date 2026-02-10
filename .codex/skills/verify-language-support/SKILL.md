---
name: verify-language-support
description: End-to-end workflow for validating language support changes in the clarity dependency graph analyzer
---

Use this workflow to validate language support end-to-end in `clarity`.

## Prepare

1. Identify the target language module under `depgraph/<language>/`.
2. Identify affected command output in `cmd/languages/` when maturity level changes.
3. Confirm current local changes with `git status --short`.

## Verify Parser/Resolver Behavior

1. Run targeted tests first:
   - `go test ./depgraph/<language>`
2. If behavior changed, add or update regression tests near changed logic:
   - Parser tests in `depgraph/<language>/parser_*_test.go`
   - Resolver tests in `depgraph/<language>/*resolver*_test.go` (or add one)
3. Prefer symbol-usage-based assertions over package-level assumptions.

## Validate on a Real Repository (Always Interactive)

1. Pick a representative repo for the language.
2. Clone into `/tmp`.
3. Build a review queue before rendering graphs:
   - Use non-merge commits only.
   - Use commits with `5-30` changed files.
   - Prioritize commits that are mostly about the target language (based on file extensions/paths).
   - Each selected commit should include at least a few files in the target language (minimum 3 unless unavailable).
   - Default queue size is 10 commits unless the user requests a different count.
4. Show the queue to the user before starting graph renders.
5. Render exactly one commit at a time in the IDE:
   - Show commit context immediately before the diagram:
     - `git -C /tmp/<repo> show -s --format='%h %s' <sha>`
   - In chat, print the commit message line before the Mermaid block so the user knows what change they are reviewing.
   - `go run . graph --repo /tmp/<repo> -c <sha> -f mermaid`
   - Always include the rendered diagram directly in chat (Mermaid fenced block). If Mermaid is not visible, provide DOT/text fallback in chat.
   - If diagrams still are not visible to the user, generate and open the graph URL in the browser (for example `clarity graph -u` or equivalent URL output).
6. Pause after each commit and wait for explicit user confirmation (for example, `next`) before continuing.
7. Continue this request/response loop until the queue is complete or the user stops.
8. After completing the default 10-commit queue, ask:
   - "Do you want me to proceed with updating maturity level and committing the changes?"
   - If the user says yes, do it immediately without extra confirmation:
     - Update module maturity as justified by validation results.
     - Update `cmd/languages` golden output if changed.
     - Run quality gates (`make lint`, `make test`).
     - Commit the changes with a focused `type: subject` message.
9. Keep validation focused on edge quality for each commit:
   - Production file incorrectly depends on test file
   - Fan-out caused by same-name declarations across source sets/targets
   - Missing edges where symbols are clearly referenced
10. If the user asks a side question during review (for example what `.in` means), answer briefly and then resume the queue when prompted.

## Fixing Issues

1. Update parser extraction to collect real referenced symbols.
2. Update resolver logic to map imports/references to concrete declarations.
3. Avoid package-wide linking for imports.
4. For ambiguous symbol definitions (multiple files define same symbol), skip linking unless disambiguation is available.
5. Add regression tests covering the exact failing pattern from real commits.

## Quality Gates

1. Run lint and full tests:
   - `make lint`
   - `make test`
2. If `cmd/languages` output changes (for example, maturity icon changed), update golden:
   - `go test ./cmd/languages -run TestLanguagesCommand_PrintsSupportedLanguagesAndExtensions -update`
   - Re-run `make test`

## Review Graph of Local Changes

1. If 3 or more files changed, run:
   - `./clarity graph -f mermaid`
2. Ensure test-to-code directionality and dependency density look plausible.

## Handoff Checklist

1. Summarize what changed and why.
2. Include exact files touched.
3. Include command results for `make lint` and `make test`.
4. If commit is requested, use one focused commit message in `type: subject` format.
