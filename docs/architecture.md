# Architecture

## Overview

Kankyō is a read-only diagnostic CLI. It reads the process environment, resolves a fixed set of supported tools, inspects PATH entries, produces findings, and renders either terminal output or a versioned JSON report.

```text
cmd/kankyo
  -> internal/cli
       -> internal/pathcheck -> internal/platform, internal/tools
       -> internal/scanner   -> internal/platform, internal/pathcheck
       -> internal/report    -> internal/pathcheck, internal/scanner
       -> internal/render    -> internal/report
       -> internal/redact
```

## Packages

- `internal/cli` — command dispatch, flags, exit codes, wiring.
- `internal/pathcheck` — PATH parsing, entry inspection, duplicate detection, candidate discovery, PATH findings.
- `internal/scanner` — allowlisted tool resolution, version execution, version parsing, tool findings.
- `internal/diagnostics` — finding types, stable ID registry, severity, sorting and deduplication.
- `internal/report` — JSON schema, summary counts, redaction application, safe report writing.
- `internal/redact` — home-prefix and username-segment redaction.
- `internal/render` — terminal sections, status labels, color, width handling, recommendation list.
- `internal/platform` — build-tagged Windows/Linux path rules, executable discovery, process execution, terminal capability, trusted `cmd.exe` handling.
- `internal/tools` — the fixed tool allowlist and version parsers.
- `internal/version` — build version variables.

`internal/platform` uses build-tagged files in one package instead of separate `windows` and `linux` subpackages. The platform surface is small; this avoids trivial wrapper packages and interface boilerplate while preserving compile-time platform isolation.

## Execution model

- PATH is read from the current process environment only.
- Supported tools are resolved through PATH and `exec.LookPath`; current-directory-relative resolutions (`exec.ErrDot`) are not executed.
- Version checks use fixed commands and arguments.
- Per-command timeout: 3 seconds. Global scan timeout: 8 seconds.
- stdout and stderr each retain at most 32 KiB; writers continue draining after the cap.
- Linux terminates the process group with a direct-child fallback.
- Windows terminates the direct child only. Descendants of batch wrappers may survive; this is a documented limitation.
- Windows batch files are executed only through the validated system `cmd.exe` obtained through `GetSystemDirectory`, using an explicit `SysProcAttr.CmdLine` with `/d /s /v:off /c`.
- `ComSpec` is inspected for a diagnostic mismatch but never used to choose the interpreter.

## Reporting

The JSON schema is versioned as `1.0`. Reports contain schema version, Kankyō version, UTC timestamp, platform OS/arch, tool results, PATH entries and analysis counts, findings, and summary counts.

Report writing:

1. Inspect the destination with `Lstat`; reject symlinks and directories.
2. Create a same-directory temporary file with restrictive permissions.
3. Write and sync the complete JSON document.
4. Close the temporary file.
5. Attempt replacement with `os.Rename`.
6. Remove the temporary file on failure.

The destination is never opened for in-place truncation. Kankyō does not promise universal atomicity, crash consistency, directory-entry durability, or universal original-file preservation on every filesystem.

## Privacy and security

- No network communication, telemetry, analytics, cloud service, or background service.
- No automatic repair, installation, or uninstallation.
- No modification of PATH, environment variables, registry, shell profiles, permissions, or system settings.
- External version-check programs are outside Kankyō's control and may have their own side effects.
- `--redact` replaces the known home prefix with `<HOME>` and exact username path segments with `<USER>`.

## Testing

Unit tests use temporary directories and fake tool fixtures. Windows-specific tests cover the system interpreter, batch command-line construction, unsafe path rejection, exact batch candidate use, bounded output, and timeout behavior. Linux-specific tests cover execute bits, searchability, process-group timeouts, and bounded output. Integration tests build the binary and run it against a controlled PATH.
