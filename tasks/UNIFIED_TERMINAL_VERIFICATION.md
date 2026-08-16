# Unified Salad Terminal verification

Date: 2026-08-16
Terminal release branch: `5fb65d6` (`codex/unified-terminal-harness`)
Backend release dependency: `54589ae` / PR `adebayox/saladBE#107`

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
work item `work-c4c9e7b6915b`.

The live sign-in trace also found a cross-repository release dependency: the
terminal sends a loopback callback, while the deployed backend currently falls
back to the web login-success page. Backend PR `adebayox/saladBE#107` adds the
state-preserving desktop callback and strict loopback validation; it must be
merged and deployed with the terminal release before live auth verification
can pass.
