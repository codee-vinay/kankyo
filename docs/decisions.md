# Engineering decisions

| ID | Decision | Reason | Alternatives considered | Verification |
|---|---|---|---|---|
| D001 | Implement only the approved v0.1.0 CLI scope | Prevent scope expansion and keep the utility small | GUI, plugins, repair, package management | Scope documented in README and code |
| D002 | Build with Go 1.27.3; declare `go 1.26.0` | Latest stable toolchain, while remaining buildable by the previous supported line; matches `x/term` v0.47.0 | Pin only 1.27; support older Go | `go version` and build/tests executed with 1.27.3; CI matrix declares 1.26.x and 1.27.x |
| D003 | Depend only on `golang.org/x/term` and `golang.org/x/sys` | Reliable terminal detection and Windows console-mode inspection without a CLI framework or color library | Standard-library-only with weaker Windows terminal support; third-party CLI/color libraries | Modules resolved; Windows tests executed |
| D004 | Use standard-library `flag` and explicit subcommand dispatch | Predictable, no framework dependency, sufficient for seven commands | Cobra, pflag, urfave/cli | CLI tests executed on Windows |
| D005 | Inspect the process PATH only | Read-only and avoids claiming persistent PATH inspection | Registry/user/system PATH inspection | Path tests and real scan executed; persistent PATH intentionally not inspected |
| D006 | Keep platform code in one build-tagged `internal/platform` package | Small platform surface; avoids trivial wrapper packages and interfaces | Separate `platform/windows` and `platform/linux` packages | Windows and Linux compile/vet checked |
| D007 | Central diagnostic registry with stable IDs | Consistent text, severity, and recommendations | Per-package ad hoc findings | Diagnostics tests executed |
| D008 | Missing optional tools are INFO; multiple installations are WARNING; genuine execution failures are ERROR | Avoid treating optional absence as a failure while surfacing real breakage | Missing tools as ERROR | Scanner and CLI tests executed |
| D009 | Exit codes 0/1/2/3 | Script-friendly distinction between success, ERROR findings, usage errors, and operational failure | Single non-zero code | CLI tests executed |
| D010 | Use only the validated system `cmd.exe` from `GetSystemDirectory`; never execute `ComSpec` | Prevent a substitute interpreter from an arbitrary directory | Trust any `cmd.exe` basename; use `ComSpec` | Windows tests executed; spike verified actual system directory and fake `ComSpec` ignored |
| D011 | Preserve the exact validated batch candidate path and build the command line once | Avoid re-resolution or working-directory substitution between validation and execution | Re-run `LookPath` in the runner | Windows exact-candidate test executed |
| D012 | Use `/d /s /v:off /c`, reject unsafe batch metacharacters, never use `call` or `cmd /c` for non-batch commands | Disable AutoRun/delayed expansion, deterministically quote, and constrain shell parsing | Generic shell escaping; implicit `CreateProcess` batch handling | Windows command-line and unsafe-path tests executed |
| D013 | Bound subprocess output with draining writers: 32 KiB stdout + 32 KiB stderr | Bound memory without blocking a child that writes more than the cap | `CombinedOutput` then truncate; `io.ReadAll` | Writer and real-subprocess cap tests executed on Windows |
| D014 | 3-second per-command timeout; 8-second global timeout; 500 ms `WaitDelay` | Bound hangs while allowing normal Java/npm startup | 1-second per command; no timeout | Windows timeout tests executed |
| D015 | Linux process-group termination; Windows direct-child termination only | Remove wrapper descendants on Linux; Windows Job Objects were deferred | Job Objects on Windows; no tree handling | Windows descendant-survival limitation observed; Linux tests declared for CI |
| D016 | Omit shell inference from v0.1.0 | Inherited environment variables do not reliably identify the current shell; omission beats a false claim | `SHELL`/`PSModulePath` heuristics; parent-process inspection | No shell field in schema or output |
| D017 | Write reports through a same-directory temporary file, sync, close, then rename; reject symlink/directory destinations | Avoid in-place truncation and partial writes from Kankyō's own write path | `os.WriteFile` directly; in-place truncation | Report success/failure tests executed; no universal atomicity claimed |
| D018 | Redact the known home prefix and exact username path segments | Predictable, path-aware privacy aid without global substring corruption | Global username replacement | Redaction tests and CLI report test executed |
| D019 | CI matrix on Windows and Ubuntu with Go 1.26.x/1.27.x | Actual platform testing, not cross-compilation | Windows-only CI; cross-build only | Workflow written; not executed because no push was authorized |

## Verification status

- Windows unit/integration tests executed successfully with Go 1.27.3.
- Windows `go vet` executed successfully.
- Linux `go build` and `go vet` cross-checks executed successfully.
- Linux tests have **not** been executed locally; they require the Ubuntu CI job or another real Linux machine.
- CI workflow has not run because pushing was not authorized.
