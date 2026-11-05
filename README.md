# Kankyō (環境)

**Understand your environment.**

Kankyō is an independent, MIT-licensed command-line utility for Windows and Linux. It helps developers understand their local development environment, inspect PATH configuration, check supported tools, and receive actionable recommendations.

## What it does

- Checks Go, Git, Node.js, npm, Python, Rust, Cargo, and Java.
- Reports resolved executable paths, versions, execution success, and multiple PATH candidates.
- Inspects the current process PATH for duplicates, empty entries, missing or inaccessible directories, relative entries, and unexpanded variable references.
- Produces a structured terminal report and a versioned JSON report.
- Supports `--redact` for home directories and usernames in paths.

## What it does not do

- It does not modify PATH, environment variables, the registry, shell profiles, permissions, or system settings.
- It does not install, uninstall, repair, or automatically fix anything.
- It does not use telemetry, network communication, cloud services, databases, or background services.
- It does not execute arbitrary user-provided commands.
- It does not claim complete shell command-resolution accuracy.

## Requirements

- Windows 10/11 or Linux.
- Go 1.26 or newer to build.
- The release build is produced with Go 1.27.3.

## Build

```sh
go build -o kankyo ./cmd/kankyo
```

On Windows:

```powershell
go build -o kankyo.exe ./cmd/kankyo
```

The project declares `go 1.26.0` in `go.mod`. Build and test with Go 1.26.x or 1.27.x.

## Commands

| Command | Purpose |
|---|---|
| `kankyo` | Welcome header, version, commands, and scan suggestion |
| `kankyo scan` | Complete environment scan |
| `kankyo path` | PATH diagnostics only |
| `kankyo tools` | Supported developer-tool checks only |
| `kankyo report --output report.json` | Versioned JSON report |
| `kankyo version` | Version and build information |
| `kankyo help [command]` | General or command-specific help |

Common flags:

- `--verbose` — show evidence and per-entry PATH details (`scan`, `path`, `tools`).
- `--redact` — redact the home directory and username in paths (`scan`, `path`, `tools`, `report`).
- `--output <path|->` — required for `report`; `-` writes JSON to stdout.

Examples:

```sh
kankyo
kankyo scan
kankyo path --verbose
kankyo tools
kankyo report --output report.json --redact
kankyo report --output - | jq .
```

Exit codes:

- `0` — successful operation, no ERROR findings.
- `1` — completed with ERROR findings.
- `2` — usage error.
- `3` — operational failure, such as report writing failure.

## Limitations

- **Process PATH only.** Persistent user/system PATH is not inspected.
- **Windows timeout limitation.** Kankyō terminates the direct child process. If a batch wrapper such as `cmd.exe` has started descendants, those descendants may survive a timeout.
- **Approximate PATH checks.** Windows ACL searchability is not evaluated. Linux searchability is based on mode bits. Symlink, junction, and 8.3 alias canonicalization is not performed.
- **No shell inference.** Kankyō does not guess the current shell from inherited environment variables.
- **External tools are outside Kankyō's control.** Version checks run allowlisted external programs with fixed arguments. Those programs may have their own side effects. Kankyō does not sandbox them.
- **Report replacement is not universally atomic.** The destination is never opened for in-place truncation, and the temporary file is written, synced, and closed before replacement is attempted. Kankyō does not guarantee universal atomicity, crash consistency, directory-entry durability, or universal original-file preservation on every filesystem.

See [`docs/diagnostics.md`](docs/diagnostics.md), [`docs/architecture.md`](docs/architecture.md), and [`SECURITY.md`](SECURITY.md) for details.

## Testing

```sh
go test ./...
go vet ./...
go build ./cmd/kankyo
```

Tests use controlled fixtures and temporary directories; they do not rely on the developer's installed tools. Windows tests were executed locally on Windows 11 AMD64. Linux tests are defined for the Ubuntu CI matrix and must be verified by an actual Linux run; cross-compilation is not treated as platform testing.

## Privacy

Reports are local. Kankyō does not transmit environment information, collect telemetry, or log secrets. `--redact` replaces the known home prefix with `<HOME>` and exact username path segments with `<USER>`. Redaction is a privacy aid, not anonymization.

## License

MIT. See [`LICENSE`](LICENSE).
