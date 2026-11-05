# Diagnostics

Every finding has a stable ID, severity (`INFO`, `WARNING`, `ERROR`), title, description, evidence, impact, and recommendation.

## PATH detection confidence

### Reliable

- Splitting on the platform PATH separator, including Windows quote handling.
- Empty entries.
- Explicit relative entries.
- Missing entries through `os.Stat` not-exist results.
- Entry is a file rather than a directory.
- Lexically duplicate entries after documented normalization.
- Literal environment-variable references.
- Supported-tool candidate discovery.

### Approximate or platform-limited

- Inaccessible directory: only when a syscall returns a permission error.
- Linux searchability: mode-bit approximation; ACLs and root behavior are not fully modeled.
- Windows searchability: not evaluated; `searchable` is `null`.
- Multiple candidates and shadowing: only for the eight allowlisted commands, not every executable.
- Symlink, junction, and 8.3 alias canonicalization: not performed.

### Not attempted

- Persistent user/system PATH from the registry.
- Full shell resolution, aliases, functions, or hashed command tables.
- Windows ACL evaluation.
- Exhaustive filesystem search for all installations.

## IDs

| ID | Severity | Trigger |
|---|---|---|
| `PATH_UNAVAILABLE` | ERROR | PATH is unset or empty |
| `PATH_DUPLICATE_ENTRY` | WARNING | Same normalized directory appears more than once |
| `PATH_EMPTY_ENTRY` | WARNING (INFO on Windows) | Empty PATH entry |
| `PATH_DIRECTORY_NOT_FOUND` | WARNING | PATH directory does not exist |
| `PATH_DIRECTORY_INACCESSIBLE` | WARNING | Permission error while inspecting a directory |
| `PATH_DIRECTORY_NOT_SEARCHABLE` | WARNING | Linux directory lacks execute/search bits |
| `PATH_ENTRY_NOT_DIRECTORY` | WARNING | PATH entry is a file |
| `PATH_RELATIVE_ENTRY` | WARNING | Non-empty relative PATH entry |
| `PATH_ENV_VAR_NOT_EXPANDED` | WARNING | Literal `%VAR%` or `$VAR` in PATH |
| `PATH_MULTIPLE_CANDIDATES` | WARNING | More than one executable candidate for a supported command |
| `TOOL_NOT_FOUND` | INFO | No candidate for a supported tool |
| `TOOL_NOT_EXECUTABLE` | ERROR | Candidate cannot execute |
| `TOOL_PERMISSION_DENIED` | ERROR | OS denies execution |
| `TOOL_EXECUTION_FAILED` | ERROR | Version command exits non-zero |
| `TOOL_TIMEOUT` | ERROR | Version command exceeds its timeout |
| `TOOL_VERSION_UNPARSED` | WARNING | Output did not match the expected version format |
| `TOOL_CWD_RESOLUTION` | WARNING | Resolution relative to the current directory; execution refused |
| `TOOL_EXECUTION_SKIPPED` | INFO | Execution intentionally skipped |
| `TOOL_EXECUTION_UNSUPPORTED` | ERROR | Safe execution mode could not be constructed |
| `PLATFORM_COMSPEC_MISMATCH` | WARNING | `ComSpec` differs from the validated system `cmd.exe` |

## Severity policy

- **ERROR** — likely prevents normal work or a found tool could not be executed.
- **WARNING** — likely misconfiguration, risk, or ambiguous resolution.
- **INFO** — observation, absence, or intentional skip.

Missing optional tools are INFO. Multiple installations are WARNING. Genuine execution and operational failures are ERROR.

## Stable IDs

IDs describe conditions, not implementation details. Do not rename an existing ID for a cosmetic reason. Add a new ID for a materially different condition. The JSON report and documentation must remain consistent.
