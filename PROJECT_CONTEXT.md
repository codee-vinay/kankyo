# Project context

## Overview

Kankyō (環境) is an independent MIT-licensed Go CLI for Windows and Linux. It helps developers understand their local development environment, inspect the process PATH, check eight supported tools, and produce terminal or JSON diagnostics.

Executable: `kankyo`
Module: `github.com/codee-vinay/kankyo`
Target: v0.1.0

## Current stage

Phase 3 implementation is complete. Phase 4 validation is partially complete: Windows tests/build/vet passed, Linux cross-build/vet passed, but Linux tests and CI have not executed. Phase 5 final review is not complete.

## Architecture

- `cmd/kankyo` — entry point.
- `internal/cli` — commands, flags, exit codes.
- `internal/pathcheck` — PATH parsing, entry checks, candidates, PATH findings.
- `internal/scanner` — tool resolution, bounded execution, version parsing, tool findings.
- `internal/diagnostics` — finding model and registry.
- `internal/report` — JSON schema and safe writing.
- `internal/redact` — home/username redaction.
- `internal/render` — terminal report.
- `internal/platform` — build-tagged Windows/Linux implementation.
- `internal/tools` — allowlist and parsers.
- `internal/version` — version variables.

## Implemented

- `kankyo`, `scan`, `path`, `tools`, `report`, `version`, `help`.
- Eight tool checks with version parsing and multiple-candidate detection.
- PATH duplicate, empty, missing, inaccessible, relative, not-directory, variable-reference, and multiple-candidate findings.
- Stable diagnostic IDs and severities.
- Terminal output with status labels, `NO_COLOR`, redirected-output handling, width wrapping, and verbose evidence.
- Versioned JSON report, `--redact`, and safe temporary-file report replacement.
- Windows trusted system `cmd.exe` validation, explicit batch command lines, `ComSpec` mismatch warning, unsafe path rejection, and exact candidate preservation.
- Bounded subprocess output, per-command and global timeouts, Linux process-group termination, Windows direct-child-only termination.
- Unit and integration tests; GitHub Actions Windows/Ubuntu matrix.

## Pending

- Run the Ubuntu CI job on a real Linux runner (not yet executed; no push authorized).
- Final Phase 4 validation and Phase 5 review.
- Optional future work explicitly deferred: shell inference, persistent PATH inspection, Windows Job Objects, ACL precision, symlink/junction/8.3 canonicalization, per-candidate version checks, `--no-color`, `--debug`, `--no-exec`.

## Approved decisions

See `docs/decisions.md`. Key points:

- Go 1.27.3 build toolchain; `go 1.26.0` directive.
- `x/term v0.47.0` and `x/sys v0.49.0` only.
- Standard-library CLI.
- Process PATH only.
- Missing tools INFO; multiple installations WARNING; genuine execution/operational failures ERROR.
- Exit codes 0/1/2/3.
- No shell inference.
- Report temp-and-rename workflow without universal atomicity claims.

## Known limitations

- Windows timeout termination is direct-child-only; descendants of batch wrappers may survive.
- Windows ACL/searchability is not evaluated; `searchable` is null.
- Linux searchability uses mode bits, not full ACL semantics.
- Symlink/junction/8.3 aliases are not canonicalized for duplicate detection.
- Persistent user/system PATH is not inspected.
- External version-check programs are not sandboxed and may have side effects.
- Report replacement is not universally atomic or crash-consistent.
- Shell aliases/functions and full shell command resolution are not inspected.

## Verification record

Environment: Windows 11 AMD64; Go 1.27.3 installed at `C:\Program Files\Go\bin\go.exe` but **not on PATH** in the development shell; commands were run with the absolute Go path.

Last successful build:

```text
go build ./cmd/kankyo
exit 0
```

Last successful tests:

```text
go test -count=1 ./...
exit 0
```

Windows packages tested: cli, diagnostics, pathcheck, platform, redact, render, report, scanner, tools, tests/integration.
Windows `go vet ./...`: exit 0.
Windows `go build ./cmd/kankyo`: exit 0.
Linux `go build` and `go vet ./...` cross-checks: exit 0; **Linux tests not executed**.

`go test -race ./...` was attempted and did not build because the local MinGW `cc1.exe` reports `unimplemented: 64-bit mode not compiled in`. This is an environment/toolchain limitation, not verified race coverage.

Actual Windows scan observed: 128 PATH entries, 5 tools detected, 42 WARNING findings, 0 ERROR findings. Go was not on PATH and was reported as not found; Git, Node.js, npm, and Java were detected; Python had multiple candidates. These values are environment-specific, not product guarantees.

CI workflow: `.github/workflows/ci.yml`; not executed because no push was authorized.

## Next task

Run the GitHub Actions matrix on an actual Ubuntu runner and Windows runner, then perform the final Phase 4/5 validation review. Do not claim cross-platform verification until those runs succeed.
