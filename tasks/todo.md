# Workspace tools harness (aligned with Salad CTO plan, 2026-07-20)

Shared brain / separate hands. Full plan lives in `saladBE/tasks/todo.md`.

- [x] Phase 0 (with BE): stop treating code_context as machine access; no advertised tools until bridge exists; preserve Salad identity on terminal turns
- [x] Phase 1: handle `tool_request` → local read tools → `POST /tools/result`; migrate trust off in-repo `.salad-trust`; symlink-safe reads
- [x] Phase 2: patch apply + approval UX
  - [x] `workspace.ApplyEdit` / `PreviewEdit` (full-file replace, safe unified diff, atomic write, ignored/escape checks)
  - [x] `apply_edit` tool_request → diff-preview approval panel (`y`/`n`/`a` remember/`esc`), reject posts an error result
  - [x] Approvals are queued so parallel tool calls resolve one at a time (no interleaving)
- [x] Phase 3: bounded verification commands
  - [x] `tools.ParseCommand` (no shell, POSIX-ish tokenizer, rejects `; | & < > backtick $(` and newlines outside quotes)
  - [x] `tools.Classify`: read-only git/ls/version → auto-run; test/build/unknown → approval; denied → fail closed
  - [x] `workspace.RunCommand`: argv exec (no `sh -c`), workspace-bounded cwd, 60s timeout, output cap, per-command session auto-approve
- [x] Project memory: `SALAD.md`/`CLAUDE.md` at workspace root loaded into `code_context.project_instructions` each turn
- [x] Backend (saladBE): `CodeContext.ProjectInstructions` field + rendered in `formatCodeContext`
- [x] Terminal `get_diagnostics` returns a helpful redirect (use run_command to verify) instead of a hard error

## REAL HUMAN-STYLE SIGN-OFF (2026-08-03) — staged, driving the actual TUI against live staging

- Method: built the local binary, logged in as QA (`codex-live-qa@test.local`) on `api-staging.salad.ink`, trusted a scratch workspace (`/tmp/saladqa-ws`: bug.go + SALAD.md with magic token `SALAD_CTX_VERIFY_7F3K`, git repo), and drove the real bubbletea TUI through a PTY like a human (typed the prompt, read the diff, pressed `y`/`n`). Chat: `6a710faa05040d2758822ab9` (Claude Sonnet).
- Scenario A (approve): prompt to fix bug.go with apply_edit + run_command + git log.
  - apply_edit panel rendered with real unified diff → pressed `y` → file changed on disk (`a+b` → `a*b`, verified via git diff) → result POST /api/tools/result 200.
  - run_command panel (`go run bug.go`) → pressed `y` → output `6` returned → 200.
  - `git log --oneline -3` auto-ran (read-only git) without a panel → `cf590a4 initial` → 200.
  - Turn completed; AI reported all three outputs. Evidence: `internal/workspace` git diff, goproxy request log, chat activity_trace, PTY transcripts in `/tmp/saladqa-driver/`.
- Scenario B (reject): apply_edit panel → pressed `n` → file comment unchanged on disk, error `user rejected apply_edit` POSTed (200), AI acknowledged the rejection.
- Scenario C (project_instructions): asked the model for the project-instructions magic token → replied exactly `SALAD_CTX_VERIFY_7F3K` (token only exists in the workspace SALAD.md, delivered via `code_context.project_instructions`; request body captured through the logging proxy).
- Tool advertisement verified: with code_context attached, the AI sees `read_file, search_codebase, list_directory, get_diagnostics, apply_edit, run_command` (claude-sonnet).
- REAL BUG #1 FOUND+FIXED: trust silently missed on symlinked workspace paths (macOS `/tmp`→`/private/tmp`) because `os.Getwd()` returns the physical path while `Trust` stored the logical path. Fix: `ResolveRoot` now canonicalizes symlinks (`internal/workspace/workspace.go`). Regression test `TestTrustViaSymlinkedPathMatchesPhysicalPath`.
- REAL BUG #2 FOUND+FIXED: `approvePending`/`rejectPending` never removed the approved/rejected request from `m.toolQueue`, so the same request was re-processed in a ghost loop — repeated `(no changes)` re-execution, duplicate `POST /api/tools/result` with an already-consumed request_id (404), and a stuck pendingTool that blocked the queue so subsequent run_command requests were never shown → backend 60s timeouts. Fix: dequeue in `approvePending` and `rejectPending` (`internal/app/app.go`). Regression tests in `internal/app/approval_test.go`.
- Observations (not blockers): GPT-5.4 does not see the workspace tools on staging (model-specific; claude-sonnet works); `git_status`/`git_diff`/`git_log` terminal handlers exist but the backend's advertised `VSCodeTools` set does not include them (git works via bounded run_command).

## Follow-up (2026-08-04): git tools + cross-model
- GPT-5.4 FULLY VERIFIED live in the TUI (apply_edit → file fixed, run_command `go run bug.go` → `6`, git log auto → `cf590a4 initial`). Earlier "gpt-5.4 can't see tools" was a model-refusal misdiagnosis; activity_trace proves it calls the tools.
- `git_status`/`git_diff`/`git_log` advertised server-side only when `code_context.surface=salad_terminal` (new `ai.TerminalGitTools`). Signal rides the transient code context — never persisted on messages, so the durable message schema and web client are untouched. VS Code extension tool set unchanged. Backend tests cover terminal vs non-terminal surfaces.
- DONE LIVE (2026-08-04): backend commits `28b7426` (initial) + `bbe4a91` (architectural correction: gate on `code_context.surface`, not durable metadata). Deployed to api-staging.salad.ink (`DEPLOY_OK bbe4a91`). Live re-verify: git tools advertised, user message metadata has NO `client_surface` key (only `ai_turn`), apply_edit + run_command + git_log all 200. Chat `6a71e3c3f32316f1a82ef16c`. GPT-5.4 + Claude Sonnet + Gemini Pro full TUI loops also verified.

## CROSS-MODEL MATRIX (2026-08-04) — anti-tunnel-vision pass
Verified the workspace-tool flow across every tool-capable model family on live staging (`66149ee`). Full real-TUI loops (approve edit → file changed, run_command, git): claude-sonnet, gpt-5.4, gemini-pro. Headless probes (backend advertises tools + model calls read_file/run_command; tool_request broadcast works): claude-opus, gemini-flash, grok-4, mistral-medium. `groq-compound-mini` is NOT tool-capable (backend `function_calling`=partial) — tools correctly not sent; it returns empty streams on tool-demand prompts (pre-existing, non-deterministic groq/provider flakiness; plain turns + control work). Not a harness regression. Evidence chats on saladbe_staging; driver/probe transcripts in `/tmp/saladqa-driver/`. See docs/DX_SIGNOFF.md.

## Signed off (staging)

## Signed off (staging)



- [x] Equal-surface TUI (Salad chrome, chat list, room)
- [x] Email login + browser Google PKCE (`salad login --google`)
- [x] salad.v1 websocket live + poll fallback
- [x] @ mention picker + explicit_mentions/target_hint on send
- [x] Turn-scoped local tools (`code_context`, `/git` `/read` `/diff` `/trust`)
- [x] DX matrix on staging QA account — see `docs/DX_SIGNOFF.md`

## Follow-ups

- [ ] Allowlist loopback redirect URI on Google OAuth client; live-verify `--google`
- [ ] Token-stream UI for `stream_chunk` events
- [ ] Production API default after soak
