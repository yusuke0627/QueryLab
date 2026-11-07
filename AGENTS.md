# QueryLab: agent instructions

## Project intent

Build a declarative Go SQLite query builder and a MyGo visual editor. Preserve
the roadmap: shared Query Model, SQL and Go generation, SQLite execution,
JOINs and relationships, MyGo GUI, and limited bidirectional editing.

## Architecture

- Keep the root package as the shared, JSON-serializable Query Model and pure validation.
- Put SQL compilation in `dialect/sqlite` and Go source generation in `gogen`.
- Make both generators consume the same model and reject invalid input.
- Keep database access and MyGo dependencies out of the core model and generators.
- Use declarative structs and expression constructors. Do not introduce mutable fluent chains.
- Never silently discard unsupported syntax during future imports.
- Keep generated SQL deterministic; bind values, quote identifiers, whitelist operators.
- Add JOINs explicitly before introducing relationship conveniences. Never infer joins silently.
- Keep SQL/Go source import restricted to documented supported syntax; never execute imported Go.

## Development

- Use Go 1.23+ and standard-library tooling. The initial core has no external dependencies.
- Add focused tests for behavior, errors, argument ordering, and roundtrip semantics.
- Run `gofmt` on changed Go files, `go test ./...`, and `go vet ./...`.
- Run `go test -race ./...` for shared-state or concurrency changes.
- SQL engine tests may skip only when the optional `sqlite3` CLI is unavailable.
- Keep documentation honest about implemented features versus planned work.
- Keep user-facing documentation in Japanese and Go identifiers in English.
- Do not add inline code comments, generated stubs, or dependencies without a concrete need.
- Do not commit, create branches, or publish unless the user asks.

## Immediate next milestone

See `docs/implementation-plan.md`. Implement the SQLite execution adapter and
schema discovery next, with driver selection recorded in a design decision.
