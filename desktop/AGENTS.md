# Desktop UI instructions

- This is a separate Go 1.27.3 module. The repository root remains Go 1.23.
- Pin MyGo v0.10.0. Run commands from this directory with GOWORK=off.
- Keep input-to-model conversion independent of MyGo, window state, and DB access.
- MyGo elements only live during the current view build. Never store them in app state.
- Store draft data in app state and bind controls directly to it; mark stale previews.
- Do not start DB work as a side effect of rendering. Use explicit actions for external work.
- Run CGO_ENABLED=0 go test ./..., CGO_ENABLED=0 go vet ./..., and go tool mygo vet .
- Report headless tests, successful builds, and actual screen verification separately.
- When adding asynchronous DB work later, use MyGo's documented update API and cancel stale jobs.
