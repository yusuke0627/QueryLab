# QueryLab: agent instructions

## Project intent

Build a declarative Go SQLite query builder and a MyGo visual editor. Preserve
the roadmap: shared Query Model, SQL and Go generation, SQLite execution,
JOINs and relationships, MyGo GUI, and limited bidirectional editing.

This is primarily a learning project for Go, MyGo, and query-builder algorithms.
Treat explainable implementation and reviewer-facing material as deliverables.
Prefer small, inspectable steps and document trade-offs before introducing abstractions.

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

## 設計原則：Simple Made Easy

Rich Hickeyの *Simple Made Easy* を設計判断の指針とする。単純さはコードの短さではなく、別々に理解・変更・検証できるはずの関心事が、不必要に絡み合っていないことで判断する。

- クエリの意図はデータで表す。画面の状態、クエリモデル、SQL・Goコードの生成、DB実行の責務を分ける。
- 変換は入力と出力を明示し、入力を原則として変更しない。関数内だけで使う一時的な状態は、処理を追いやすくするなら使ってよい。
- 新しい抽象化や依存は、具体的な必要性があるときに導入する。重要な設計判断では「何を独立して変更・検証できるようにしたか」をレビュー資料で説明する。
- この原則だけを理由に依頼範囲外の書き換えを行わない。機能要件と正確性を守り、同じくらい単純なら、学びやすく使いやすい案を選ぶ。

## Development

- Use Go 1.23+ and standard-library tooling. The query model and generators do not import the SQLite driver.
- SQLite integration tests and the execution example require CGO and a C compiler.
- Add focused tests for behavior, errors, argument ordering, and roundtrip semantics.
- Run `gofmt` on changed Go files, `go test ./...`, and `go vet ./...`.
- Run `go test -race ./...` for shared-state or concurrency changes.
- SQL engine tests may skip only when the optional `sqlite3` CLI is unavailable.
- Keep documentation honest about implemented features versus planned work.
- Keep user-facing documentation in Japanese and Go identifiers in English.
- Follow `docs/learning-plan.md` and the review process in `docs/review-guide.md`.
- Every implementation milestone must include reviewer-facing material under `docs/reviews/`.
- Update the relevant review document when behavior, algorithms, or design decisions change.
- Explain learning goals, code reading order, design alternatives, algorithm steps,
  invariants, complexity where relevant, reproduction commands, test evidence, and limitations.
- Distinguish implemented behavior, measured results, analysis, and future work.
- Do not claim that generated code or passing tests demonstrate the learner's understanding;
  include questions or exercises that let reviewers assess that understanding.
- Include documentation updates for later milestones covering Go, MyGo, JOINs, and source import.
- During the current learning phase, proactively add beginner-friendly Japanese code comments.
- Explain unfamiliar Go syntax, data flow, algorithm steps, and reasons for non-obvious choices;
  connect query constructors such as Bool(true) to the SQL and bound arguments they produce.
- Start temporary explanatory comment blocks with `// 学習用:` so they can be found later.
- Keep comments accurate when code changes. Remove or consolidate learning comments only when
  the user requests it; preserve necessary API contracts and design rationale during cleanup.
- Keep reviewer documents alongside code comments; neither replaces the other.
- Do not add generated stubs or dependencies without a concrete need.
- Do not commit, create branches, or publish unless the user asks.

## Git workflow

- Create a topic branch before starting repository changes. Do not edit or commit on `main`.
- Push the topic branch and open a pull request targeting `main` when authorized to publish.
- Do not push directly to `main` or merge the pull request without the user's explicit request.

## Immediate next milestone

M4-1 native preview is prototyped in the separate Go 1.27.3 desktop module.
Confirm the actual window and Japanese input on an unlocked Mac before treating the UI spike as verified.
Then design M4-2 persistence and DB/schema integration, keeping slow I/O off the UI thread.
Root Go 1.23 checks and desktop checks must run separately; see desktop/AGENTS.md.
Keep foreign-key metadata separate from query construction; do not infer joins silently.
