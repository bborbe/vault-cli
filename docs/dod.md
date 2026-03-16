# Definition of Done

## Code Quality

- Exported types, functions, and interfaces have doc comments
- Error handling uses `github.com/bborbe/errors` with context wrapping
- No debug output (print statements, fmt.Printf) — use structured logging
- Factory functions are pure composition — no conditionals, no I/O, no `context.Background()`
- Follow Interface → Constructor → Struct → Method pattern

## Testing

- New code has good test coverage (target >= 80%)
- Changes to existing code have tests covering at least the changed behavior
- Tests use Ginkgo v2 / Gomega with Counterfeiter mocks
- `make precommit` passes (lint + format + generate + test + checks)

## Documentation

- CHANGELOG.md has an entry under `## Unreleased`
