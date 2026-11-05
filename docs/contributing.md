# Contributing and testing

See the repository [`CONTRIBUTING.md`](../CONTRIBUTING.md) for workflow and scope rules.

## Test strategy

- Unit tests use temporary directories and controlled fixtures.
- `internal/pathcheck` tests cover duplicates, empty entries, missing paths, relative paths, variable references, candidates, and Linux permission behavior.
- `internal/platform` tests cover Windows system `cmd.exe` validation, batch command construction, unsafe path rejection, exact candidate execution, fake `ComSpec` handling, bounded output, and timeouts.
- `internal/scanner` tests inject lookup and execution functions; they do not require real developer tools.
- `internal/report` and `internal/redact` tests cover schema shape, redaction, report replacement, and failure cleanup.
- `tests/integration` builds the actual binary and runs it against a controlled PATH.

## Platform testing

Windows tests were executed on Windows 11 AMD64. Linux tests are defined for `ubuntu-24.04` CI and have not been executed locally. Cross-compilation and `go vet` are compile checks, not platform test evidence.

## Validation

```sh
go test ./...
go vet ./...
go build ./cmd/kankyo
```

Record the platform, command, exit code, and any skipped or unexecuted tests.
