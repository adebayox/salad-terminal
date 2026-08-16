# Unified Salad Terminal verification

Date: 2026-08-16
Commits: `9c8290a`, `6961877`

## Verified locally

- `go test ./... -count=1`
- `go test -race ./internal/app ./internal/harness ./internal/workspace`
- `go vet ./...`
- `go build ./cmd/salad`
- `git diff --check`
- Repeated ACP approval test: 30 runs passed.
- A real disposable project opened the existing TUI as `Salad Terminal`, showed
  the trusted-workspace gate, and reached `Workspace agent ready`.
- `salad engineer` no longer launches a second terminal; it prints the
  migration to use `salad` from the project directory.

## Architecture boundary verified in code

- One user-facing `salad` command and one Bubble Tea terminal surface.
- Trusted project prompts use the DeepSeek Harness ACP session behind that
  surface.
- Normal Salad Chat continues through the existing API/websocket path and is
  reachable from the same terminal with `/chat`.
- Workspace session messages carry a generation token so late startup/events
  cannot enter a later normal-chat or workspace session.
- ACP uses one reader goroutine, cancellation-aware permission waits, and
  completion-safe child cleanup.

## Remaining external verification

The live model-backed inspect/edit/test journey is not signed off yet. The
local Salad auth session expired; the provider bridge returned HTTP 401. The
correct recovery is `salad login`, not changing providers. Tracked as Luna
work item `work-b84574d78e00`.

The live sign-in trace also found a cross-repository release dependency: the
terminal sends a loopback callback, while the deployed backend currently falls
back to the web login-success page. An isolated backend patch (`a81ef46`) adds
the narrowly scoped loopback callback allowlist; it must be reviewed and
deployed with the terminal release before live auth verification can pass.
