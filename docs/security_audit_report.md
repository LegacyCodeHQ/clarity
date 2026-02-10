# Security Audit Report

## Executive Summary
This is a Go CLI tool (no HTTP server in the repo). The primary security-relevant surfaces are git subprocess calls and file-system access based on CLI inputs. I found 3 issues: one Medium risk hardening gap around untrusted commit/path arguments passed to `git`, one Medium risk process gap (no `govulncheck` in CI), and one Low risk path access concern that matters if the CLI is ever exposed to untrusted input (e.g., a service wrapper). No critical issues were found.

## Medium Severity

### SBP-001 — Git subprocess arguments accept untrusted refs/paths without option termination or allowlist
- Rule ID: GO-INJECT-002
- Severity: Medium
- Location:
  - `/Users/ragunath/GolandProjects/clarity/vcs/git/git.go` `validateCommit` lines 60-72
  - `/Users/ragunath/GolandProjects/clarity/vcs/git/git_diff.go` `getCommitFiles` lines 132-148
  - `/Users/ragunath/GolandProjects/clarity/vcs/git/git_diff.go` `GetFileContentFromCommit` lines 164-175
  - `/Users/ragunath/GolandProjects/clarity/vcs/git/git_diff.go` `GetCommitRangeFiles` lines 188-229
  - `/Users/ragunath/GolandProjects/clarity/vcs/git/git_tree.go` `GetCommitTreeFiles` lines 37-45
- Evidence:
  - `cmd := exec.Command("git", "rev-parse", "--verify", commitID+"^{commit}")`
  - `cmd := exec.Command("git", "diff-tree", "--no-commit-id", "--name-only", "-r", "--root", "--diff-filter=d", commitID)`
  - `cmd := exec.Command("git", "show", ref)` with `ref := fmt.Sprintf("%s:%s", commitID, filePath)`
- Impact: If `commitID` or `filePath` are attacker-controlled (e.g., CLI invoked by a service), a crafted value starting with `-` or containing unexpected ref syntax can be interpreted as git options or alternate revs, potentially reading unintended objects or altering command behavior. This is not shell injection (you correctly avoid `sh -c`), but it is still command argument injection into git’s option/revision parsing.
- Fix: Add allowlist validation for commit IDs and paths (e.g., reject values starting with `-`, containing whitespace/NUL, or path traversal segments), and terminate option parsing where supported (e.g., `git rev-parse --verify -- <rev>`). For `git show`, consider `git show -- <rev>` only if you also use `--` for pathspec separation and ensure you are not passing paths as separate args; otherwise validate the `commitID` and `filePath` separately before constructing `commit:path`.
- Mitigation: If you intentionally accept arbitrary git revs from trusted local users only, document this assumption. If invoked by automation, sanitize inputs before calling these functions.
- False positive notes: If all `commitID`/`filePath` values are derived exclusively from trusted git output and not from user input, risk is reduced. Verify how CLI flags are exposed in any automation or service wrapper.

### SBP-002 — `govulncheck` not run in CI
- Rule ID: GO-DEPLOY-001
- Severity: Medium
- Location: `/Users/ragunath/GolandProjects/clarity/.github/workflows/test.yml` (entire workflow; no `govulncheck` step present)
- Evidence:
  - CI runs `go mod verify` and tests, but no `govulncheck` invocation appears in the workflow.
- Impact: Known vulnerabilities in dependencies (and in stdlib if you ever pin an older patch) can slip into releases unnoticed.
- Fix: Add a `govulncheck` step (source scan or binary scan) to CI, and fail builds on findings above an agreed threshold.
- Mitigation: If `govulncheck` runs elsewhere (e.g., a separate security pipeline), document and link it in the repo.
- False positive notes: If this repo is intentionally excluded from vulnerability scanning due to its usage context, document the exception.

## Low Severity

### SBP-003 — CLI path resolution allows reading arbitrary absolute paths
- Rule ID: GO-PATH-001 (contextual)
- Severity: Low
- Location:
  - `/Users/ragunath/GolandProjects/clarity/cmd/graph/path_resolver.go` `Resolve` lines 36-46
  - `/Users/ragunath/GolandProjects/clarity/vcs/content_reader.go` `FilesystemContentReader` lines 9-12
- Evidence:
  - `if filepath.IsAbs(pathStr) { return AbsolutePath(filepath.Clean(pathStr)), nil }`
  - `return os.ReadFile(absPath)`
- Impact: If `clarity graph -i` is ever exposed to untrusted input (for example, via a web service wrapper), an attacker could request arbitrary file reads outside the repo. As a local CLI for trusted users, this may be acceptable.
- Fix: When `--repo` is provided, enforce that resolved paths stay under the repo root by default. If you want to preserve current behavior, add an explicit `--allow-outside-repo` flag and gate the behavior behind it.
- Mitigation: Document that `-i` paths are trusted local inputs and should not be wired to untrusted sources without validation.
- False positive notes: If this is strictly a local CLI used by trusted developers, you may accept this risk.

## Notes on Non-Issues
- No HTTP server code or cookie handling appears in this repo; HTTP-specific hardening guidance (timeouts, CORS, CSRF, headers) is not applicable here.
- No direct evidence of secret leakage or insecure crypto usage was found in production code.
