# Security policy

## Supported versions

Security fixes are applied to the current v0.1.x line while the project is in MVP development.

## Read-only boundary

Kankyō itself does not:

- modify PATH, environment variables, registry, shell profiles, permissions, or system settings;
- install, uninstall, repair, or delete software or user files;
- execute arbitrary user-provided commands;
- use telemetry, analytics, cloud transmission, or background services.

The only write performed by Kankyō is the explicitly requested report file.

## External programs are a separate trust boundary

Kankyō launches a small, fixed allowlist of external version checks with fixed arguments. Those programs are outside Kankyō's control. They may read or write configuration or caches, load plugins, start processes, or use the network. Kankyō does not sandbox them and does not claim they are side-effect-free.

Mitigations:

- fixed allowlist and version arguments;
- absolute resolved executable paths;
- `exec.ErrDot` resolutions are refused;
- Windows app-execution-alias paths are skipped;
- 3-second per-command timeout and 8-second global timeout;
- 32 KiB stdout and 32 KiB stderr retention with continued draining;
- no arbitrary user-controlled arguments;
- no `--exec` mode.

## Windows batch execution

Kankyō does not use `ComSpec` to select an interpreter. It obtains the system directory through `GetSystemDirectory`, constructs and validates `cmd.exe`, rejects symlinks/reparse points and unexpected directories, and invokes it with `/d /s /v:off /c` through an explicit `SysProcAttr.CmdLine`.

Batch paths are validated before execution. The exact validated absolute path is preserved. The configured unsafe metacharacter class is rejected, `call` is never used, and `cmd /c` is never used for non-batch checks.

A `ComSpec` mismatch is reported as a warning but does not influence interpreter selection.

## Windows process limitation

Kankyō terminates only the direct child on Windows. A batch wrapper can start descendants that survive a timeout. This is a known limitation of v0.1.0. Kankyō does not claim full process-tree termination on Windows.

## Reports and redaction

Reports are local files unless the user explicitly moves or exports them. Kankyō does not include raw environment-variable values or raw subprocess output.

`--redact`:

- replaces the known home directory prefix with `<HOME>`;
- replaces exact username path segments with `<USER>`;
- applies to terminal paths, tool paths, PATH entries, and evidence.

Redaction is a privacy aid, not anonymization. Paths outside the known home directory and paths that do not contain the username may remain. Secrets should not be placed in PATH entries.

Report writing never opens the destination for in-place truncation. It writes and syncs a same-directory temporary file, closes it, then attempts replacement. Kankyō does not guarantee universal atomicity, crash consistency, directory-entry durability, or universal original-file preservation on every filesystem.

## Reporting a vulnerability

Open a private security advisory on the repository rather than a public issue. Include the Kankyō version, platform, reproduction steps, and whether external tools or report files are involved.
