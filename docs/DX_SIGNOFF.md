# Salad Terminal — Developer Experience Signoff

## Real harness signoff (2026-08-03) — workspace tools driven like a human

**Environment:** live staging `api-staging.salad.ink` (release `f5d3cb6`), QA account `codex-live-qa@test.local`, chat `6a710faa05040d2758822ab9` (Claude Sonnet).
**Method:** local dev binary, `/tmp/saladqa-ws` workspace (`bug.go` + `SALAD.md` with magic token `SALAD_CTX_VERIFY_7F3K`, git repo), TUI driven through a PTY with real keypresses and human read-time.

| Flow | Result | Evidence |
|---|---|---|
| apply_edit approve | Pass — diff panel shown, `y` pressed, file changed on disk (`a+b`→`a*b`), result POSTed 200 | git diff of bug.go; goproxy log; chat activity_trace |
| run_command approve | Pass — `go run bug.go` approved, output `6` returned, 200 | goproxy log `POST /api/tools/result` |
| run_command auto (read-only git) | Pass — `git log --oneline -3` ran without a panel, `cf590a4 initial`, 200 | goproxy log |
| apply_edit reject | Pass — `n` pressed, file unchanged, `user rejected apply_edit` POSTed 200, AI acknowledged | goproxy log; bug.go comment intact |
| project_instructions | Pass — model echoed `SALAD_CTX_VERIFY_7F3K` exactly (token exists only in workspace SALAD.md) | AI reply; request body captured with `code_context.project_instructions` |
| tool advertisement | Pass — with code_context attached, AI sees `read_file, search_codebase, list_directory, get_diagnostics, apply_edit, run_command` | AI tool listing |
| queued tools resolve sequentially | Pass — read_file(auto) → apply_edit(panel) → run_command(panel) → git log(auto) all resolved in one turn (39.8s) | A4 run |

**Real bugs found & fixed during this signoff:**
1. Trust silently missed on symlinked workspace paths (macOS `/tmp`→`/private/tmp`): `ResolveRoot` now canonicalizes symlinks. Test `TestTrustViaSymlinkedPathMatchesPhysicalPath`.
2. Approved/rejected tool requests were never removed from the tool queue → ghost re-execution loop, duplicate result POSTs (404), and the pending panel blocking later run_command requests (backend 60s timeouts). Fixed by dequeuing in `approvePending`/`rejectPending`; regression tests in `internal/app/approval_test.go`.

**Cross-model verification (2026-08-04):** GPT-5.4 (flagship OpenAI) verified end-to-end in the same TUI flow — apply_edit approved → file fixed, run_command `go run bug.go` → `6`, git log auto → `cf590a4 initial`, turn complete. Earlier "GPT-5.4 does not see tools" was a misdiagnosis from a model refusal to enumerate tools; the activity trace shows GPT-5.4 calling read_file/apply_edit/run_command. Claude Sonnet verified the same.

**Git tools:** `git_status`/`git_diff`/`git_log` are implemented in the terminal (`internal/tools`) and now advertised server-side ONLY for `client_surface=salad_terminal` (VSCodeTools shared with the VS Code extension is unchanged; new `ai.TerminalGitTools` + `MessageMetadataKeyClientSurface` preserved through metadata sanitization). Unit-tested (`TestTerminalSurfaceGetsGitToolsWhenCodeContextPresent`, `TestNonTerminalSurfaceGetsNoGitTools`).

**Git tools — LIVE VERIFIED (2026-08-04, backend `28b7426` deployed to staging):** drove the real TUI against the deployed backend; the AI's advertised tool list on a terminal turn is `read_file, search_codebase, list_directory, get_diagnostics, apply_edit, run_command, git_status, git_diff, git_log`; the model called `git_log` as a tool, the terminal auto-ran it (`cf590a4 (HEAD -> master) initial`, POST /api/tools/result 200), plus apply_edit (approved → file fixed) and run_command `go run bug.go` → `6`. Chat `6a71c64b6e19ebd864ad2175`.

**Not claimed:** web-app-side flows signed off separately in saladBE; production default API still staging per contract.

## Earlier signoff (2026-07-18)

**Environment:** `https://api-staging.salad.ink`  
**Account:** `codex-live-qa@test.local` (CLAUDE.md QA account)  
**Chat:** `QA terminal staging signoff` (`6a5ba4f9931f487e4c293bab`) with GPT-5.4  
**Date:** 2026-07-18  

## Product gaps closed

| Gap | Status | Evidence |
|---|---|---|
| Salad-feeling TUI | Pass | Login → chat list → room with user bubbles + AI headers |
| Email login | Pass | `salad whoami` as codex-live-qa |
| Browser Google login | Implemented | `salad login --google` PKCE loopback; requires Google console redirect URI for `http://127.0.0.1:<port>/callback` |
| Live updates | Pass | Go `salad.v1` websocket connects; room also polls as fallback |
| `@` mention picker | Pass | TUI opens Mention UI on `@`; send with `@gpt-5.4` returned `MATRIX_173153` |
| Turn-scoped local tools | Pass | Trusted workspace attaches `code_context` on send; `/git` `/diff` `/read` `/trust`; `.env` blocked |
| Bad login edge | Pass | Invalid credentials → clear error + staging hint |

## Matrix results (executed)

1. **whoami** — `codex-live-qa <codex-live-qa@test.local>`
2. **chat list** — titles + AI members
3. **workspace** — trust, permissions, git-status, read README; `.env` denied (`ignore_exit:1`)
4. **websocket** — `ws_ok connected` via Terminal realtime client (empty Origin)
5. **@mention send** — `@gpt-5.4 MATRIX_173153…` → `[GPT-5.4] MATRIX_173153`
6. **TUI send** — `MATRIX_MENTION_OK` → `[GPT-5.4] MATRIX_MENTION_OK`
7. **bad login** — `AUTH_INVALID_CREDENTIALS (401)`

## Adjacent paths checked

- Headless `salad say` and TUI `enter` send both work
- Mentions resolve to `explicit_mentions` / `target_hint` on send
- Secrets ignore path works without printing secret material
- WS failure mode falls back to poll (Python clients that spoof Origin get 403; CLI does not)

## Not claimed

- Production default API (still staging per contract)
- Google OAuth end-to-end in this session (needs redirect URI allowlisted on the Google client)
- Websocket token streaming chunks rendered token-by-token (events refresh transcript; no character stream UI yet)

## CTO verdict

**Staging developer experience: signed off for equal-surface collaboration.**  
A developer can `cd repo && ./salad`, sign in, pick a real Salad chat, `@` an AI, send with local workspace context, and see the reply in the same thread as the web app.

Would a staff engineer approve this for **staging** Terminal use? **Yes.**  
Production cutover: only after Google redirect URI is configured for the shipping OAuth client and a short prod soak with a real user account.
