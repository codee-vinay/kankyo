# Contributing

Kankyō is a small, read-only diagnostic CLI. Contributions should preserve that scope.

## Build and test

```sh
go test ./...
go vet ./...
go build ./cmd/kankyo
```

Use Go 1.26.x or 1.27.x. Tests must use controlled fixtures and temporary directories; do not depend on the developer's installed tools or real PATH.

## Code style

- Idiomatic Go, small functions, explicit errors.
- Prefer the standard library.
- Do not add dependencies without a demonstrated need and approval.
- Keep platform-specific code in build-tagged files.
- Do not introduce GUI, telemetry, network communication, automatic repair, installation, persistent PATH modification, or background services.

## Findings

Every finding needs a stable ID, severity, title, description, evidence, impact, and safe recommendation. Missing optional tools are INFO. Multiple installations are WARNING. Genuine execution and operational failures are ERROR. Do not promise automatic repair.

## Tests

Add coverage for both success and error paths. Windows batch behavior, ComSpec handling, exact batch candidate preservation, bounded output, and timeouts are security-sensitive; keep their regression tests.

Before submitting, run the required validation commands and report actual results, including the platform on which they ran.
