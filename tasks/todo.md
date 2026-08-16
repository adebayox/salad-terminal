# Workspace tools harness (aligned with Salad CTO plan, 2026-07-20)

## Current upstream evidence

The official DeepSeek Harness repository still labels the project a developer
preview, warns that compatibility-breaking changes are expected, and documents
the MIT license and plugin-based Cordis architecture. That is why Salad keeps a
commit pin, a shape-checked patch, a checksum, and a rollback artifact instead
of tracking upstream `main` directly.

## Unified terminal correction plan — 2026-08-16

The previous preview proved that the DeepSeek Harness can run local work, but
it did not prove an engineer-ready Salad experience. This plan is the gate for
the next implementation pass.

- [x] Keep one user-facing `salad` terminal; do not create a second engineer
  terminal or change the normal Salad Chat transport.
- [x] Put the workspace runtime behind a small adapter boundary so the normal
  chat state machine does not own workspace process, approval, or session code.
- [x] Carry safe work events through the DSH bridge: plan, tool started,
  tool finished, file/change location, command output, approval, cancellation,
  error, and final answer.
- [x] Make the backend provider gateway enforce provider/model entitlement,
  product quota, request and token limits, cancellation, and safe diagnostics.
- [x] Preserve provider tool-round state, including DeepSeek reasoning state,
  and provide real model streaming where the provider supports it.
- [x] Remove stale user-facing `salad engineer` references from the audit
  evidence; `salad` is the only terminal surface and `salad harness` is
  diagnostic/support-only.
- [x] Make carrier release tooling require Node.js 24 and the pinned Koffi
  lockfile version before it mutates/builds the DeepSeek checkout.
- [x] Make workspace provider recovery usable from the one `salad` command:
  `salad --salad-provider <configured-provider>` now selects an explicit
  provider for trusted workspace turns without changing normal Salad Chat.
- [x] Add signed GitHub build-provenance attestation to tagged release archives
  using the existing `SHA256SUMS` manifest; first tagged-release verification
  remains open.
- [x] Add a pull-request model-free Linux x64 build/smoke of the pinned,
  security-patched DeepSeek carrier. The first run exposed and the source now
  fixes a malformed shape-checked persona patch. The next run also exposed
  that `--offline` made clean runners impossible; the builder now prefers the
  cache while allowing a clean pinned dependency fetch. The clean-runner
  carrier build and ACP smoke passed in CI run `31958133727` at commit
  `0f4a10e`.
- [ ] Prove OAuth login, inspect, edit, approve, test, failure, follow-up,
  resume, cancel, and normal Salad Chat journeys in real environments.
- [ ] Ship only a pinned DSH source revision plus the exact Salad patch set,
  checksum, platform capability statement, and rollback path.
- [x] Re-ran the focused terminal verification in pull-request CI with the
  Go 1.25.0 toolchain selected from `go.mod`. Local Go 1.24.4 was not changed
  because the active disk-safety rule prohibits local dependency/toolchain
  installs; the remote matching toolchain is the authoritative verification
  path for this pass.

### Verification record — 2026-08-16

- Added model-free ACP coverage for plan/tool-start/tool-finish events and
  renders them as local workspace activity in the same Salad TUI transcript.
- Removed the full-width room header background that rendered as the long white
  bar in terminals with no reliable light/dark palette.
- Tightened the child environment from prefix-based forwarding to an exact
  allowlist; arbitrary `DSH_*`, `DEEPSEEK_*`, and Salad credential variables no
  longer cross into the harness process.
- Added clearer client recovery messages for provider key/model, quota, and
  request-size failures. The backend handler tests covering request shape,
  tool-round shape, model allowlisting, and explicit unsupported reasoning
  state passed before the latest streaming edits; the latest backend changes
  are not locally compiled during the dependency-download pause.
- Terminal package tests pass for `internal/harness` and `internal/app`; the
  terminal binary builds and its doctor/help/trust paths work.
- A clean pinned DSH worktree accepts the shape-checked Salad patch. The
  installed macOS carrier is checksum-verified and the real free-provider
  flow works, but a newly compiled carrier from the patched source is not
  claimed here because the checkout has no dependencies and local dependency
  downloads are paused.
- Real free Mistral verification completed in a disposable Go project:
  inspect, edit, run a failing test, resume the same session, edit the test,
  and run a passing test. The run list shows both turns sharing one DSH
  session. A separate Groq probe reached the provider and returned a 502,
  which is surfaced as a provider failure rather than a false success.
- Real token-by-token provider streaming and provider-specific reasoning-state
  continuation are now implemented at the Salad gateway boundary; the
  carrier still needs a fresh patched build and the desktop OAuth callback
  still needs a real terminal journey. Normal Salad Chat transport remains
  untouched by this work.
- Static review passed after the streaming/reasoning edits (`gofmt`, shell and
  patch-script syntax checks, diff checks, and provider type-shape inspection).
  The Data volume currently reports about 10 GiB free, but local runtime
  verification remains paused by the active disk-safety instruction. Use
  existing CI or a prepared, matching toolchain for the remaining runtime
  gates.
- Existing CI evidence: release run `31890011530` completed successfully for
  the pinned carrier/job fix at commit `a09460bb117a76ed1b3c4f804296af24063f7ced`.
  It does not cover the current uncommitted provider-streaming edits, so it is
  evidence for the carrier path only, not a substitute for the remaining
  backend runtime verification.
- Terminal hardening and read-only pull-request CI are pushed to PR #21. The
  current head `0f4a10e` passed run `31958133727`: Go formatting, all Go tests,
  terminal build, release-script syntax checks, and the clean pinned DSH
  carrier build/ACP smoke passed. CI provisioned Go 1.25 from `go.mod`; no
  local toolchain or dependency install was required.
- The provider-selection recovery edit and release-script fixes are covered by
  that remote run; no paid provider was used for verification.

No implementation change is considered complete until the relevant item has a
direct test or real-run artifact.

## Follow-up release blocker — account creation keyboard path

- [x] Reproduce the documented `c` action from the initial sign-in screen; it incorrectly entered `c` into the email field.
- [x] Replace the conflicting hidden shortcut with a visible, keyboard-selectable account-creation action.
- [x] Verify email input still accepts `c`, account creation is the fourth focusable action, and Google/email paths remain intact.

## CLI product hardening — 2026-08-12

Scope: make the complete Salad Terminal experience understandable and safe for a first-time engineer, without changing the existing chat/tool architecture unless verification shows a real product defect.

- [x] First-run launch opens a clear sign-in choice; no hidden key bindings or undocumented next command.
- [x] Email/password and browser sign-in show visible labels, navigation, retry, cancel, timeout, and account-creation recovery.
- [x] `--help` and subcommand help work without starting a prompt; commands have consistent exit codes and human-readable errors.
- [x] Auth/session/chat/resume/say/workspace/update flows have good, bad, offline, empty, stale, and non-interactive coverage.
- [x] Technical diagnostics are opt-in (`SALAD_DEBUG=1`/`salad doctor`) and never replace the user-facing recovery message.
- [x] Local credentials, workspace boundaries, approvals, and update/install behavior are reviewed for security and cross-platform compatibility.
- [x] Non-interactive TUI commands fail before network or prompt work with an actionable terminal requirement; chat IDs and message cursors are validated/escaped before API requests.
- [x] Public install and release artifacts are exercised from a clean macOS environment; Linux and Windows builds/install scripts are statically checked and smoke-tested where available.
- [x] Attempted an independent challenge after implementation; the advisory run returned no report, so release signoff relies on the direct verification matrix recorded below.

Evidence/research: official Claude Code, Codex CLI, OpenCode, GitHub CLI, npm, Stripe CLI, Vercel CLI, CLI Guidelines, and Diátaxis references reviewed; fresh temporary Salad installs exercised. No implementation began until this plan was recorded.

## Release re-audit — 2026-08-12

- [x] Removed the legacy `.salad-trust` bypass; an in-repository marker no longer grants local tool access.
- [x] Bound every realtime `tool_request` to the active room and the current workspace opaque ID; mismatches fail closed before execution.
- [x] Moved CLI WebSocket authentication to the `Authorization` header; tokens are no longer sent in the URL or subprotocol.
- [x] Added visible WebSocket disconnect state and bounded reconnect attempts while polling remains the data fallback.
- [x] Rejected staging/production credential mixing when `SALAD_API_URL` conflicts with the saved session environment.
- [x] Added atomic local metadata writes and rollback for partial keyring/plaintext migration failures.
- [x] Sanitized remote titles, participants, and messages before printing in headless commands so ANSI/OSC control sequences cannot control the terminal.
- [x] Surfaced local continue-state persistence failures and partial AI-member additions instead of silently presenting success.
- [x] Added strict workspace subcommand argument validation and a browser password-recovery command.
- [x] Confirmed Excalidraw is an active whiteboard dependency (`SaladCanvas.jsx`), retained it, and removed the current high audit finding with a root `brace-expansion` override; `npm audit` now reports zero vulnerabilities.
- [x] Fixed packaged Electron API resolution: the private `salad-app://` origin now defaults to production instead of inheriting the local `.env` API, and desktop release builds explicitly use production API/WebSocket/PostHog settings.

Remaining work: final CLI test/build/PTY matrix, staging verification, then superseding public release and developer-page/browser signoff.

Implementation evidence: visible first-run PTY flow, clean-binary command matrix, live staging QA login/whoami/chat/resume/participants/workspace checks, unknown-chat recovery, room open/exit, production API doctor probe, local Go tests including race detection, vet, shell syntax check, and cross-target build attempts. Remaining release work is final review, release artifact verification, and independent challenge.

Shared brain / separate hands. Full plan lives in `saladBE/tasks/todo.md`.

## DeepSeek Harness sidecar preview — 2026-08-13

Scope: add an opt-in local DeepSeek Harness preview without changing ordinary
Salad chat, chat routing, SaladBE APIs, `salad.v1`, billing, or the default
`salad`/`--continue`/`--resume` flows.

Plan before code:

- [x] Re-read `AGENTS.md`/`CLAUDE.md` operating rules and inspect the clean
  Terminal checkout plus the already-dirty backend checkout.
- [x] Research DeepSeek Harness source/docs, Codex App Server, Claude/Gemini
  sandboxing, OpenCode, and current practitioner reports.
- [x] Confirm the safety boundary: trusted local workspace, separate child
  process, explicit preview command, no Salad credential forwarding, and no
  normal-chat code path changes.
- [x] Add a small stdio JSON-RPC client for the DSH SDK protocol: initialize,
  prompt, session events/status, shutdown, bounded lines, and process cleanup.
- [x] Add `salad harness <prompt>` as an opt-in preview command. Require a
  trusted workspace and make the DSH executable/config explicit through flags
  or environment; do not make it the default engine.
- [x] Add fake-runtime tests for successful events, malformed/oversized input,
  cancellation/child shutdown, and no credential leakage.
- [x] Run gofmt, unit tests, race tests, vet, build, and a CLI help/negative
  command matrix. Confirm the existing normal-chat entry points are unchanged.
- [x] Record what this preview proves and what remains before mapping DSH runs
  into Salad chats or SaladBE policy/quota.

Implementation status:

- [x] Added `internal/harness` with the DSH stdio JSON-RPC boundary, trusted
  workspace caller, scrubbed child environment, event rendering, session ID,
  graceful EOF shutdown, and bounded kill fallback.
- [x] Added `salad harness [options] <prompt>`. It is opt-in and never called
  by the existing `salad`, chat, resume, send, or workspace commands.
- [x] Added fake-runtime tests for the real `assistant/message` event shape,
  malformed/oversized frames, cancellation cleanup, and credential scrubbing.
- [x] Full Go tests, targeted race tests, vet, build, CLI help, trust failure,
  and normal `salad say` negative-path smoke pass.
- [x] Build and review DeepSeek's source-defined single-file runtime carrier
  from the pinned source checkout (`47f943859bef60e4160492346772ded9b24f765a`)
  and pass the real initialize/shutdown handshake.
- [x] Exercise a real prompt against DeepSeek's deterministic mock provider:
  session events, model-issued filesystem write, tool result, turn completion,
  shutdown, and a model-issued `go test ./...` command all completed in a
  disposable project.
- [x] Replace the first JSON-RPC bridge with DeepSeek's ACP automation bridge
  for the interactive CLI path. ACP is the documented DSH interface that has
  cancellation and one-shot permission decisions; JSON-RPC does not expose
  either capability.
- [x] Run the ACP path through a disposable project with a rejected operation
  and a successful completion; add `salad harness doctor` and environment-based
  runtime/config discovery so a normal developer does not need to know DSH
  internals.
- [x] Close ACP edge cases found during verification: numeric JSON-RPC IDs,
  valid approval responses, Ctrl-C cancellation, approval-session binding, and
  persistent input buffering across multiple approval prompts.
- [ ] Keep the preview local until Salad adds provider/quota policy, linked run
  identity, replay-safe reconnect, explicit cancellation, and a reviewed DSH
  distribution/upgrade path.

Safety gates:

- DSH is never launched by bare `salad` or existing chat/resume commands.
- The child receives a scrubbed environment; Salad access/refresh tokens are
  never forwarded.
- The child starts only inside an explicitly trusted workspace.
- The preview does not post DSH output into Salad chats.
- A missing DSH binary, malformed event, timeout, or disconnect fails locally
  and leaves normal Salad chat usable.
- JSON-RPC is retained only as a low-level compatibility path until ACP has
  replaced it for interactive use; it must not be presented as approval-safe.

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

- [ ] Allowlist loopback redirect URI on Google OAuth client for staging; production browser entry is available, but staging requires provider-console configuration before end-to-end Google QA.
- [ ] Token-stream UI for `stream_chunk` events

## CLI public release (2026-08-12)

- [x] Local release gates: gofmt, go vet, unit tests, and six cross-compiled targets.
- [x] Published `v0.2.3` with macOS, Linux, and Windows amd64/arm64 archives, `VERSION`, `SHA256SUMS`, and `release-manifest.json`.
- [x] Verified all six public archive checksums and archive contents; Unix installer exercised against the public release.
- [x] Fixed release workflow checkout and manifest generation; added manifest validation so empty names or malformed hashes fail the publish job.
- [x] Re-uploaded and re-verified corrected `release-manifest.json` for the already-published `v0.2.3` release.
- [x] Fixed installer cleanup under `set -u`; public-release macOS arm64 install now exits successfully and reports `salad 0.2.3`.
- [x] Published `v0.2.4` after the product-hardening commit passed CI and clean-install verification.

## DeepSeek Harness execution follow-up (2026-08-13)

- [x] Build and exercise a pinned DeepSeek ACP carrier from source revision
  `47f943859bef60e4160492346772ded9b24f765a`, including the plugin closure
  required by the example Cordis profile.
- [x] Add managed carrier installation under the private Salad config
  directory, with atomic copies, an install record, SHA-256 verification, and
  `salad harness doctor` diagnostics.
- [x] Add local run records and an explicit `salad harness resume` command.
  ACP starts a fresh session, so the command records that continuation
  honestly instead of pretending DSH supports server-side resume.
- [x] Exercise the actual pinned carrier through the installed CLI in a clean
  disposable Go project: model streaming completed, a real `bash` tool call
  created a proof file, and a second real tool call changed the failing test;
  independent `go test ./...` and `go build ./cmd/app` then passed. The first
  installer proof used a placeholder binary and was discarded as invalid.
- [x] Add additive authenticated Salad lifecycle receipts through
  `/api/harness/events`. Receipts are event-only and contain no prompt, file
  contents, tool arguments, or secrets; normal chat messages and routing are
  untouched.
- [x] Verify ordinary `salad say` still follows its existing no-active-chat
  path and never launches the harness.
- [x] Wire the pinned ACP carrier into the normal macOS/Linux release flow:
  release CI builds four platform carriers from the immutable DeepSeek
  revision, publishes them beside the six Salad archives, and `install.sh`
  verifies and installs the matching carrier automatically. `SALAD_SKIP_HARNESS=1`
  is an explicit opt-out; Windows remains normal Salad Terminal only until a
  native carrier exists.
- [x] Add managed upgrade backup and `salad harness rollback`; rollback is
  limited to Salad-owned files and never touches a project workspace.
- [x] Exercise the remote-release installer with a local release fixture;
  fixed checksum lookup so temporary download paths are compared by archive
  filename, then verified `salad harness doctor` sees the managed carrier.
- [x] Add the Salad-authenticated provider bridge so a normal Salad account can
  use the managed harness without manually exporting a DeepSeek provider key.
  The Salad token stays in the parent process; the carrier sees only an
  ephemeral loopback token and URL. Direct `DEEPSEEK_API_KEY` use remains an
  explicit escape hatch.
- [x] Deploy the authenticated lifecycle/provider routes to staging through the
  protected workflow. The first post-merge deployment exposed a host preflight
  mismatch; a controlled rerun completed successfully and `/health/ready`
  reported release `bca40aa`.
- [x] Verify the live Salad-backed provider path with the installed pinned
  carrier: text returned through the authenticated bridge, then a real project
  edit/test workflow completed. The stream-format defect found in verification
  was fixed in backend PR #98 and redeployed.
- [x] Run the neighboring normal-chat check after harness execution: the
  existing `salad say` path returned `NORMAL_CHAT_OK`; harness runs did not
  create normal chat messages or invoke the normal chat command.
- [ ] Complete browser receipt verification and make the terminal release
  publicly installable. The frontend receipt PR remains open because its CI
  quality job has unrelated pre-existing ChatArea test drift; lifecycle replay
  after reconnect is not implemented yet.

## Engineer-terminal standard audit (2026-08-15)

- [x] Confirm the product boundary: normal Salad Chat remains unchanged; DSH
  is an internal runtime for the one engineer-facing Salad Terminal.
- [x] Drive a real DSH project workflow with the public v0.2.11 binary and
  managed carrier: create files, run `npm run build`, and verify built output.
- [x] Drive the installed normal terminal workspace-tool path separately in a
  disposable project. File-edit approval rendered and applied correctly; the
  run-command request later returned `TOOL_RESULT_UNKNOWN_REQUEST` during the
  interactive test, so this path is not signed off as a reliable engineer loop.
- [x] Measure packaging: terminal binary is about 15 MB installed; macOS
  arm64 DSH carrier is about 198 MB installed and 53.8 MB compressed. The
  first install downloads both unless `SALAD_SKIP_HARNESS=1` is set.
- [x] Run DSH security negatives with the shipped carrier: absolute writes
  outside the workspace were blocked; symlink writes to a home-directory path
  outside the workspace were blocked. Platform temp roots are intentionally
  writable and must be described as such.
- [x] Exercise the configured DSH background-subagent path through the shipped
  carrier in the disposable workspace. The subagent completed and the parent
  agent created and verified a file. The first attempt exposed a test-launch
  cwd mistake and created a temporary home README; that file was removed, and
  an explicit-cd rerun recorded the correct disposable workspace.
- [x] Run a harmless sentinel security test against the real DSH filesystem
  tool. It can read a project `.env` and return its content to the model; DSH's
  current file policy also leaves network access available. This is a P1
  release blocker for prompt-injection and secret-exfiltration risk.
- [x] Read DeepSeek's source architecture. It has durable event-sourced
  sessions, subagents, background jobs, workflows, sandbox policy, and replay
  concepts. Salad's ACP adapter currently exposes fresh sessions only.
- [x] Record the single-terminal architecture and release gate in
  `docs/ENGINEER_TERMINAL_STANDARD.md`.
- [ ] Replace the internal dual execution paths with one engineer-terminal
  experience: one event model, one approval policy, one run record, and DSH as
  the replaceable local runtime. Do not change normal Salad Chat.
- [ ] Add true session restore, long-running process management, background
  task controls, and reconnect/replay before calling the engineer terminal
  production-ready.
- [ ] Add an enforced secret-read deny layer and default-deny/allowlisted
  network policy to the DSH composition, with sentinel and exfiltration tests.
- [ ] Decide whether the DSH carrier is installed lazily or remains opt-out;
  disclose the size and make the lightweight install path obvious.

### Engineer-terminal safety implementation slice (2026-08-15)

- Plan: keep the normal Salad Chat product and runtime outside this change; patch the pinned DSH carrier only so model-controlled filesystem reads and shell commands have an explicit sensitive-file and network boundary.
- Scope: `tools/build-dsh-acp-carrier.sh`, the DSH source patch applied by that builder, and harness environment defaults/tests. No SaladBE or normal terminal-chat tool path changes.
- Safety rule: filesystem tool reads of credential-shaped files fail closed; confined shell commands default to no network and deny the same sensitive file family at the OS runner where the platform supports it. Provider traffic remains in the DSH parent/bridge path and is not changed by this slice.
- Verification required: build the carrier from the pinned source, run a real sentinel `.env` read negative, run a shell network negative, rerun the real project build, and verify direct-key/provider-bridge startup still works. If any platform cannot prove the boundary, it remains a release blocker.
- Remaining after this slice: true cross-command session restore, persistent process/dev-server control, reconnect/replay, and full platform parity.

- Completed in this slice: added a shape-checked DSH source patch, default
  `DSH_NETWORK_MODE=deny` for both ACP and JSON-RPC child launches, a protected
  credential-file family, and real macOS carrier smoke tests for secret read,
  shell network, provider bridge, and ordinary workspace write. The Seatbelt
  rule is now global rather than limited to the workspace; the rebuilt carrier
  includes it.
- Still open: Linux carrier proof, Windows capability decision, and a visible
  allowlisted network approval flow instead of the preview escape hatch.
- Follow-on completed: JSON-RPC run records now retain the DSH session ID and
  use a private, stable per-workspace session directory, so JSON-RPC resume
  reuses persisted DSH history instead of copying the old prompt. ACP remains
  fresh-session by protocol design.
- Verification completed: two separate invocations through a packaged
  JSON-RPC carrier built from the pinned DSH source restored the same session;
  the second provider request saw the first marker and the durable JSONL log
  contains both turns. The JSON-RPC example composition used for this smoke is
  not the release security profile, so JSON-RPC remains compatibility-only.

### Engineer-terminal runtime verification follow-up (2026-08-15)

- [x] Correct the outside-write negative test: `/private/tmp` is an allowed OS
  temporary area under DSH `workspace-write`, so the test now targets a path
  outside both the trusted workspace and temporary roots. The real installed
  carrier denied it and the file was absent.
- [x] Run the real installed carrier through a disposable project workflow after
  the safety changes. The model-created project recovered from a failed build,
  reran the build, and independent `npm run build` plus output checks passed.
- [x] Run the real ACP carrier cancellation path with a 60-second child
  process. The first run exposed an orphaned `sleep` process; the parent now
  launches the carrier in its own process group and kills that group on the
  bounded cancellation fallback. The rerun exited as cancelled with no child
  process remaining.
- [x] Build a release-shaped macOS candidate with the rebuilt carrier, serve
  it locally with its checksum manifest, and run the real installer into an
  isolated prefix. The installed binary reported its candidate version and
  `salad harness doctor` verified the managed carrier checksum and config.
- [ ] Run a model-controlled home-directory credential read against the
  globally patched macOS carrier; project-local secret denial is verified, but
  this absolute-path negative is not yet signed off.
- [x] Validate the exact global Seatbelt expression independently against a
  home-directory `.env` sentinel; the OS denied the read. This is static
  policy evidence, not a substitute for the model-controlled carrier smoke.
- [x] Build and run the packaged JSON-RPC carrier twice to prove durable
  session restore. ACP remains the verified interactive path; JSON-RPC remains
  a compatibility path because it has no per-prompt cancellation and is not a
  distributed default carrier.
- [x] Make the ACP adapter capability-aware: persist the actual returned ACP
  session ID, use `session/load` or `session/resume` when the carrier advertises
  one, and keep an explicit fresh-continuation fallback for the pinned DSH
  preview that advertises neither. Fake-runtime tests cover both branches.
- [x] Make the Unix installer transactional across the Salad binary and the
  managed carrier: carrier download/extraction is preflighted before the
  binary swap, and a carrier-install failure restores the previous binary.
  Verified both a failed carrier archive (binary unchanged) and a failed
  managed install after the binary swap (previous binary restored) with local
  release fixtures.
- [x] Close the carrier-side half of that transaction: managed install failure
  now restores or removes runtime/config/manifest files instead of leaving
  partial Salad-owned state, forced upgrades refresh all backup components,
  and `salad harness rollback` removes newly added config when the prior
  install had none. Unit tests cover partial first-install cleanup and stale
  config rollback. The Unix installer also rolls back the binary on INT/TERM
  during the update window.
- [x] Replace the invisible `DSH_NETWORK_MODE=allow` parent-shell escape hatch
  with `salad harness --network allow` plus a visible confirmation prompt;
  parent-shell allow is rejected and the default remains deny.
- [ ] Prove Linux and Windows carrier safety on native runners, and replace the
  visible per-run network approval with a domain allowlist.
- [x] Make `salad engineer` the single user-facing local-agent entry point:
  one ACP process accepts multiple follow-up prompts, handles approvals in the
  same session, and shuts down cleanly on Ctrl-D. Keep `salad harness` only as
  the compatibility path for installation, diagnostics, JSON-RPC, and older
  one-shot scripts. Normal Salad Chat remains outside this path.
- [ ] Make the interactive engineer session resume across processes with a
  real packaged carrier capability, then add reconnect/replay and long-lived
  process controls before calling the engineer mode production-ready.
- [ ] Rebuild and smoke the carrier after the source-side ACP patch: the
  pinned DSH ACP plugin already has durable persistence and `ctx.agents.resume`,
  so `tools/patch-dsh-security.py` now prepares capability-aware
  `session/resume` and `session/close` handlers plus a private ACP persistence
  root. This is not shipped or signed off until a compiled carrier survives
  two separate processes and rejects a mismatched workspace cwd.

### Latest engineer-terminal verification (2026-08-15)

- [x] Built a real 201 MB macOS x64 ACP carrier from pinned DSH source
  `47f943859bef60e4160492346772ded9b24f765a`; checksum verification passed and
  the managed install/doctor path reported the installed digest.
- [x] Fixed a real fresh-session bug found by the first live CLI attempt: a
  generated local session ID was being mistaken for a resume request. New ACP
  runs now call `session/new`; only an explicitly supplied saved ID calls
  `session/resume` or `session/load`.
- [x] Ran the installed carrier through a disposable project with actual ACP
  reads, edits, and `npm test` tool calls, then independently verified the
  resulting files and test pass. Also verified the same-process interactive
  two-turn flow and clean Ctrl-D shutdown.
- [x] Restored a saved ACP session in a second process and recovered the
  `SALAD-RESTORE-42` marker. A direct resume with a different workspace was
  rejected with `-32602 Invalid params: session cwd does not match`.
- [x] Confirmed normal chat remains outside the code path: only terminal
  harness files changed in this checkout; no `internal/chat`, realtime, TUI,
  SaladBE, or normal `salad say` files were modified.
- [x] The authenticated live provider route is deployed at
  `https://api.salad.ink/api/harness/provider/v1/chat/completions`; no-auth
  requests return 401 instead of 404, and a real production-backed engineer
  turn completed through the installed carrier. This is separate from normal
  Salad Chat.
- [x] Published `v0.2.12` with the patched carrier, all six terminal archives,
  four macOS/Linux carrier archives, SHA256SUMS, and release-manifest.json.
  The public installer was run in an isolated prefix; it installed version
  0.2.12, signed the 198 MB arm64 carrier, and `salad harness doctor` passed.
- [ ] Remaining preview gates are explicit: native Linux/Windows safety proof,
  domain allowlist instead of per-run network approval, long-lived process UX,
  reconnect/replay, and receipt-browser verification.

### Live provider promotion follow-up (2026-08-15)

- [x] Identified the 404 as a deployment-source problem: current production
  `main` did not contain the staging harness work, while staging already served
  the protected provider route.
- [x] Promoted only the additive provider gateway and one authenticated,
  rate-limited route to production. Lifecycle receipt/realtime files and normal
  chat routing were intentionally excluded.
- [x] SaladBE PR #106 passed the full backend test, vet, build, lint, and
  document-compiler checks and was merged to `main` as `b71fe3c`.
- [x] Verified the production deployment and ran the installed CLI against the
  live provider route. Readiness reported release `b71fe3c`; no-auth returned
  401; the clean-installed `v0.2.12` binary/carrier returned the expected
  project answer through production.
- [x] Ran a real human-style engineer workflow against staging with the
  installed candidate carrier: read `AGENTS.md`, explain the failing test,
  wait for approval, edit only `main.go`, run `gofmt`, `go test ./...`,
  `go build ./...`, run the program, review the diff, close with Ctrl-D, and
  resume from a second process. The workflow produced `Hello, Salad!`, changed
  only `main.go`, and restored the final answer from the saved run.
- [x] Fixed the misleading fresh-run message found during that live workflow:
  the adapter now reports a restore fallback only when the user actually
  requested a saved-session resume.
- [x] Reproduced and resolved the apparent macOS signing failure during the
  clean install: the host disk was full from task-owned 198 MB carrier test
  copies. After removing those temporary artifacts, the same public installer
  and carrier passed signing and doctor verification.

### Final engineer-terminal audit (2026-08-15)

- [x] Re-ran a real long-lived-process workflow with the public `v0.2.12`
  install: started a Python server, verified it over HTTP from outside the
  agent, followed up in the same session, resumed it in a second process, and
  confirmed Ctrl-D cleaned up the child process group.
- [x] Re-ran a model-controlled absolute read of `/Users/davidnifemi/.env`
  with the public carrier; the carrier denied it. The test also exposed that
  non-conventional names such as `.salad-home-read-sentinel.env` are not
  covered by the current filename policy and must not be described as
  protected merely because they end in `.env`.
- [x] Add durable run status and immediate session-ID persistence so a crash
  does not leave a run looking resumable without the actual ACP session.
- [x] Make `salad engineer resume <run-id>` and saved-run inspection visible in
  help and the normal engineer workflow.
- [x] Integrate DeepSeek's official persistent PTY and background-job plugins
  into the pinned carrier build. The first rebuilt carrier exposed an
  abstract-job-registry composition error; commit `a09460b` now loads the
  concrete `@deepseek-ai/dsh-jobs-local` implementation. CI run `31890011530`
  rebuilt all four carrier targets, and a corrected macOS candidate booted and
  passed a live Salad-backed PTY/job smoke.
- [x] Ran a real collaboration prompt against the public carrier. The model
  could delegate a subagent, but the v0.2.12 config gave the engineer no
  observable job-list/output/kill control; the follow-up could not collect the
  child result before cancellation. This is evidence for the PTY/job carrier
  change, not a pass for the old package.
- [x] Decide and document the process-control and replay boundary: model
  session history resumes, while live OS processes and terminal output do not
  survive carrier shutdown; child process groups are cleaned up.
- [ ] Merge PR #10 and publish a new carrier release. The public `v0.2.12`
  package remains the older one-shot carrier.
- [ ] Re-run the remaining release matrix on native Linux/Windows, including
  sandbox enforcement, reconnect/replay, cancellation, and package install.
  Normal Salad Chat remains outside this work and must stay a no-change
  regression check.

### v0.2.14 exact-release audit (2026-08-15)

- [x] Found and fixed the public macOS packaging blocker: DeepSeek's required
  `-spawn-helper` sidecar was missing from v0.2.13, so persistent PTYs failed
  with `posix_spawnp failed`. The fix covers build output, archives, checksums,
  installer validation, managed install, doctor integrity, and rollback.
- [x] Merged PR #11 and published v0.2.14. Release workflow `31892583809`
  passed Go tests/vet, all six terminal builds, all four carrier builds, and
  publish. The exact arm64 harness archive checksum matched and contained the
  carrier, config, and executable spawn helper.
- [x] Installed the exact v0.2.14 bytes through the installer. Doctor passed;
  the managed carrier and helper are present under Salad's private config.
- [x] Ran the real disposable Node project: read instructions, one read-only
  collaborator, minimal source edit, project test, project build, persistent
  server, external HTTP request, same-process follow-up, and explicit terminal
  close with external port verification.
- [x] Found and fixed a resume safety gap: concurrent resume of an active run
  can make two carriers write one session log and corrupt it. Resume now
  refuses `starting`/`running` records and documents that live PTYs/dev servers
  belong to the current carrier process.
- [x] Published v0.2.15 with the resume guard and reran the exact-release
  resume/cancellation matrix from a clean workspace. The exact release passed
  safe resume after process exit and refused active-run resume. Native
  Linux/Windows runtime proof, domain-level network allowlisting,
  reconnect/replay, and browser receipt verification remain open; normal Salad
  Chat remains untouched.

### Post-release challenge adjudication (2026-08-15)

- [x] Ran independent read-only security/package and engineer-flow reviews
  after the DeepSeek reviewer bridge timed out. The reviews confirmed that
  v0.2.15 is a usable macOS-focused preview, not a generally shippable
  cross-platform engineer product.
- [x] Converted the review findings into durable work items for native
  Linux/Windows sandbox proof, domain-level network allowlisting, package
  provenance and Windows rollback, and release-level collaboration/
  cancellation/replay evidence.
- [x] Added staged atomic replacement and previous-binary preservation to the
  Windows installer. This is source-complete but not released or native-tested
  yet.
- [ ] Do not call the engineer terminal production-ready until the four
  durable work items above have evidence. Normal Salad Chat remains outside
  the harness path and has no changed files.

### Loopback developer-flow correction (2026-08-15)

- [x] Reproduced the real v0.2.15 developer flow from the exact installed
  release. Collaboration, edit/test/build, persistent PTY, and cleanup work;
  default-deny networking incorrectly blocked even a localhost dev server.
- [x] Added explicit `--network loopback` mode for macOS. It prompts visibly,
  allows model-controlled processes to bind/connect to localhost, and keeps
  external network access denied. Linux now rejects that mode explicitly after
  native bubblewrap proved it cannot create a usable loopback interface;
  `--network allow` remains the broader, separately approved mode.
- [x] Rebuilt the pinned macOS carrier from the audited DeepSeek source and
  verified the candidate against the real Node project: localhost HTTP passed,
  an external HTTPS probe failed under the candidate Seatbelt policy, and
  closing the PTY closed the port. Restored the exact v0.2.15 managed carrier
  after the test.
- [ ] Publish the loopback correction only after release CI and a clean exact
  release install reproduce this matrix; native Linux loopback, Windows
  runtime, domain allowlisting, package provenance, reconnect/replay, and
  browser receipt work remain open.

### v0.2.16 exact-release verification (2026-08-15)

- [x] Published v0.2.16 after release CI passed the terminal and carrier
  matrix; the exact release artifacts matched the public SHA256SUMS manifest.
- [x] Ran the exact public installer against the verified v0.2.16 assets;
  terminal version, managed carrier, spawn helper, and `harness doctor` all
  passed. The installer preserved the previous managed runtime for rollback.
- [x] Ran the exact v0.2.16 release in a separate real Node workspace:
  delegated one read-only collaborator, edited a source file, passed four
  project tests, passed the build command, started a persistent server, called
  it from outside the harness, followed up in the same session, and closed
  the process with the port gone afterward.
- [x] Verified loopback-only behavior in the exact release: localhost worked
  while an external HTTPS probe was denied. Explicit resume with a concrete
  new request restored the prior ACP conversation and returned the stored
  marker.
- [x] Native Linux x64/ARM64 release smoke proved external network denial and
  also proved the current unprivileged bubblewrap namespace cannot support
  localhost (`Failed RTM_NEWADDR: Operation not permitted`). The CLI now
  rejects Linux `--network loopback` rather than promising a broken dev-server
  flow.
- [x] Removed the empty synthetic prompt from no-argument interactive resume;
  a local build now restores directly to `[salad engineer] >`, and an explicit
  follow-up returned `RESUME_NO_EMPTY`.
- [ ] Native Windows carrier and sandbox proof, domain-level network
  allowlisting, package provenance, cancellation matrix, and browser receipt
  verification remain release gates. Native Linux packaged session resume and
  provider-error recovery are now verified. Normal Salad Chat remains outside
  this work and has no changed files.

### Fresh engineer workflow recheck (2026-08-15)

- [x] Repeated the exact installed v0.2.16 flow against a fresh Node project
  instead of relying on the earlier audit: project instructions were read,
  `/health` was improved, the test passed, and the loopback approval path was
  exercised.
- [x] Rebuilt the pinned macOS carrier from DeepSeek commit
  `47f943859bef60e4160492346772ded9b24f765a` after adding explicit guidance
  for persistent terminal/job use and evidence-based reporting. The carrier
  built successfully, a collaborator returned a concrete review, and a real
  persistent terminal served port 4321, was checked externally, and was closed
  with no listener remaining.
- [x] Found and corrected the long-running command UX gap: the candidate now
  gives an explicit `env PORT=...` example and verifies the actual listener.
  A real Node server started on port 4324, returned `/health` over curl, and
  both terminal sessions closed cleanly.
- [x] Ran live cancellation against the rebuilt macOS candidate: a Python
  server was externally confirmed on port 4322, Ctrl-C cancelled the engineer
  run, and the child port closed without manual cleanup.
- [ ] Publish the carrier guidance change and repeat the clean exact-release
  workflow, including environment-based app startup, cancellation, and
  cross-platform release checks.

### Native Linux release smoke (2026-08-15)

- [x] Rebased the engineer follow-up branch onto the published v0.2.16 main
  commit; PR #14 is clean again.
- [x] Ran the release matrix on native GitHub Linux x64/ARM runners. The six
  terminal archives and four macOS/Linux carrier archives built successfully;
  Windows terminal archives also built successfully.
- [x] Added a release-gated Linux carrier smoke: boot the packaged carrier,
  initialize ACP, create a session, complete a prompt through a local mock
  DeepSeek endpoint, and prove bubblewrap denies external network access.
- [x] Run the new smoke on the follow-up branch; release workflow
  `31898110098` passed on native Linux x64 and ARM64. It booted the packaged
  carrier through ACP against a local mock provider and passed the network
  deny check on both architectures.
- [x] Re-ran the corrected release workflow `31898839394` after making the
  Linux loopback limitation explicit; terminal archives, carrier builds, and
  both native Linux carrier smoke jobs passed.
- [x] Native Windows runner evidence passed in release workflow
  `31899990278`: the Windows terminal archive executed, engineer help stated
  that normal Salad Chat is a separate path, and `harness doctor` reported the
  missing DSH carrier explicitly. This validates the Windows terminal
  boundary; it does not claim Windows engineer runtime support.
- [x] Native Linux x64/ARM64 release workflow `31900302285` passed a packaged
  cross-process session test: ACP initialized, the first prompt completed, the
  session closed, a fresh carrier process resumed the same session, and the
  second prompt completed with exactly two provider requests on each
  architecture.
- [x] Native Linux x64/ARM64 release workflow `31901155291` passed the
  provider-recovery extension: a simulated 503 surfaced as an ACP error, the
  session closed cleanly, a fresh carrier process resumed it, and a later
  prompt succeeded. The carrier retried the failed provider call; both
  architectures completed with six mock requests.
- [ ] Add a native Windows DSH carrier and engineer runtime. DeepSeek's pinned
  carrier builder currently supports Linux/macOS only, so Windows engineer
  mode still needs an explicit product/runtime decision rather than a
  misleading partial install.
### v0.2.17 public-release engineer proof (2026-08-15)

- [x] Merged the engineer-only carrier safeguards in PR #14 and published
  `v0.2.17` from the merged main commit. Release workflow `31903646186`
  passed terminal tests, six terminal archives, four macOS/Linux carrier
  archives, native Linux x64/ARM64 sandbox smoke, the Windows terminal
  boundary smoke, and immutable release publication.
- [x] Installed `v0.2.17` through the public release installer with
  `SALAD_FORCE_REMOTE=1`; the installed binary reported `salad 0.2.17`, the
  managed carrier passed `harness doctor`, and the installed carrier hash was
  `71aaeac3eafa88696768fa40a489fafb8052e3a5bbb03f4284f3eb7963c9a941`.
- [x] Ran the published bytes in the disposable Node workspace: the model
  read `AGENTS.md`, made a small test improvement, ran `npm test` with one
  passing test, started `env PORT=4325 npm run start` in a persistent
  terminal, verified `/health` with an external curl, surfaced a read-only
  reviewer result, and left no listener on port 4325.
- [x] Verified public-release cancellation with a Python server on port 4326:
  external `lsof` and curl observed it while the run was active; interrupting
  Salad made the saved run `cancelled` and the port closed without manual
  process cleanup.
- [x] Verified public-release resume creates a new run and restores the prior
  conversation's test/reviewer/server-cleanup context. It intentionally does
  not claim that an old OS process or PTY survives carrier shutdown.
- [ ] Remaining stronger-release gates are unchanged: native Windows DSH
  carrier/runtime, domain-scoped network allowlisting, package provenance and
  rollback hardening, and fuller replay/reconnect semantics. `v0.2.17` is an
  engineer preview for macOS/Linux, not universal Windows engineer support.

### Empty-workspace reality check (2026-08-15 continuation)

- [x] Used the public `v0.2.17` CLI in a new empty trusted Git workspace. It
  created a dependency-free task-board app, produced a `dist/` build artifact,
  ran tests, and served the page on a persistent terminal. Independent curl
  and lsof checks confirmed `/health`, the page marker, and process cleanup.
- [x] Found and corrected real generated-project defects instead of accepting
  the model's summary: the first `start` script only echoed text; the first
  test cleanup used the wrong server object; the first accessibility repair
  duplicated HTML blocks; and the first test only checked substring presence.
  Exact independent counts and the final build/test then passed.
- [x] Found a second real safety defect in the generated app: the default
  server bound wildcard IPv6 and encoded traversal returned HTTP 500. The
  engineer session entered a repeated repair loop and corrupted the disposable
  `server.js`/`test.js` files with NUL bytes and duplicated blocks before it was
  cancelled. This is a harness/model-workflow failure, not a release pass.
- [x] Added a bounded ACP prompt timeout (default 10 minutes, override with
  `SALAD_ENGINEER_PROMPT_TIMEOUT`) and a regression test proving a stalled
  carrier turn is cancelled and reaped. Added explicit persona guidance to
  report reviewer/tool failure and stop repeated repair attempts.
- [ ] Do not claim the collaboration lane is signed off yet. The requested
  public and rebuilt-candidate reviewer prompts hit provider HTTP 502s, and
  the model previously claimed reviewer approval without a returned finding.
  Re-test the bounded reviewer lane when the provider route is healthy, then
  publish the fix only after an actual returned read-only review is observed.

### v0.2.19 exact-release continuation (2026-08-15)

- [x] Published v0.2.19 after the release matrix passed Go test/vet, all six
  terminal archives, all four macOS/Linux carrier builds, native Linux x64/
  ARM64 sandbox smoke, Windows terminal-boundary smoke, and immutable release
  publication. The exact public installer installed v0.2.19 and `harness
  doctor` reported the managed carrier hash.
- [x] Re-ran the plain provider health turn from a trusted Git workspace with
  the exact public v0.2.19 CLI. It still returned provider HTTP 502 after the
  terminal's one bounded transient retry, so a real collaboration turn is not
  signed off; this is currently a Salad provider-route blocker, not a normal
  Salad Chat path failure.
- [x] Removed the implicit active-chat lifecycle-receipt fallback. Engineer
  receipts must be explicit opt-in so starting a local agent can never mutate
  the user's normal active chat by accident; added a unit test for the
  explicit `--chat`/`SALAD_HARNESS_CHAT_ID` selection rule.

### v0.2.20 public engineer reality check (2026-08-15)

- [x] Exercised the exact installed public `v0.2.20` CLI in a fresh trusted
  workspace through a local provider fixture: it created files, ran `npm test`,
  ran the build, opened a persistent terminal, started a real Node server,
  verified `/health` with an independent curl, and closed the terminal with no
  listener left behind. Interactive mode then exited cleanly with Ctrl-D.
- [x] Exercised the collaborator protocol through the exact public CLI: the
  parent requested `subagent_fork`, received the child result, and returned it
  to the developer. This is protocol evidence only; it is not a production
  provider signoff while Salad's provider route is unhealthy.
- [x] Confirmed the public install boundary: `salad 0.2.20`, managed carrier
  hash `925f20a9461be01fcd9ef5d259e79caa912c05703a5a3e22a9ed785ac9fb3971`,
  active carrier about 198 MB, compressed release about 51 MB, and about 399 MB
  on disk when one rollback carrier is retained.
- [x] Re-ran a real authenticated `salad engineer` provider probe. The
  dedicated route still returns `HTTP 502` after the terminal's bounded retry;
  this is a Salad provider/origin incident, not a harness-install or normal
  Salad Chat failure.
- [x] Confirmed the default OpenAI provider failure is provider-specific, not
  a missing harness installation or a route-wide failure: explicit xAI returns
  real model responses through the same authenticated dedicated route.

### Real Salad-backed engineer workflow (2026-08-15 continuation)

- [x] Tested the dedicated provider choice instead of treating the default
  OpenAI 502 as a route-wide blocker. `--salad-provider xai` returned a real
  model response through the public `v0.2.20` CLI; OpenAI still returned 502
  and Anthropic closed the carrier.
- [x] Ran a real xAI-backed engineer session in a brand-new workspace. The
  model created a Node project, started a persistent server on port 4327,
  verified `/health` externally, and returned a concrete read-only collaborator
  review.
- [x] Independent verification caught that the model had ignored the requested
  build step. `salad engineer resume` added `npm run build`; independent
  `npm run build` produced `dist/server.js` and `dist/build.json`, and a fresh
  independent `npm test` passed.
- [x] Ran current public-release cancellation with the real xAI provider. A
  server on port 4330 was externally reachable, Ctrl-C changed the saved run to
  `cancelled`, and the port closed without manual process cleanup.
- [x] Made provider recovery explicit in `salad engineer` help and failure
  output, including `--salad-provider` and `SALAD_HARNESS_PROVIDER`, without
  silent cross-provider fallback.
- [ ] Post-release follow-up: add a read-only provider-health check so ordinary
  users can see which Salad provider is available before starting a long run.

### Shipping decision (2026-08-15)

- [x] Ship `v0.2.20` as the Salad Terminal engineer preview for macOS and
  Linux. The exact public release is installed, the real xAI-backed workflow
  has been independently verified, and normal Salad Chat remains a separate,
  untouched path.
- [x] Ship with explicit boundaries: Windows has terminal-only support until a
  native DSH carrier exists; Linux loopback mode is rejected; external network
  access requires an explicit per-run choice; lifecycle receipts remain
  opt-in; closing the carrier does not preserve live OS processes.
- [ ] Post-release follow-up: make the default provider's health and recovery
  path clearer. OpenAI's current default route returned 502 in this audit,
  while explicit xAI succeeded. Do not silently fail over between providers or
  route engineer work through normal Salad Chat.

### VS Code terminal UX reality check (2026-08-16)

- [x] Reproduced the user's exact installed v0.2.21 command in the actual
  /Users/davidnifemi/code/saladBE workspace. salad engineer without an
  explicit provider still fails with DeepSeek API error (HTTP 502);
  salad engineer --salad-provider xai completes a real read-only workspace
  inspection and exits cleanly.
- [x] Identified the screenshot's long white line as the normal terminal chat
  header painting its light background across a dark VS Code terminal. The
  terminal theme now uses adaptive light/dark colors, the header explicitly
  says Salad chat, and the footer points codebase work to salad engineer.
- [x] Added a regression test for the normal-chat header/footer mode boundary.
  Normal web Salad Chat and the SaladBE source repository were not modified.
- [ ] Required release follow-up: publish this terminal-only UX fix and
  resolve the server-selected engineer provider for accounts where the
  default OpenAI route still returns 502. Keep provider selection explicit;
  do not silently switch providers.
## Unified Salad Terminal architecture correction (2026-08-16)

The previous preview exposed `salad engineer` as a separate user-facing path.
That is not the intended product. Salad Terminal must remain one terminal
experience; DeepSeek Harness is an internal execution engine for workspace
turns, not a second command developers must learn.

Plan before implementation:

- [x] Keep one primary user entry point: `salad` from a repository.
- [x] Keep normal Salad Chat transport and web-shared conversations intact;
  no harness prompt, tool call, or receipt may enter the normal message/router
  path.
- [x] Add a shared terminal session adapter so the existing TUI owns prompt
  entry, transcript, approvals, cancellation, and exit while the DSH ACP
  process owns the model/tool loop for trusted workspace turns.
- [x] Make the workspace decision visible and safe: an untrusted project shows
  a trust gate; `/chat` explicitly opens normal Salad Chat, while trusted
  repository prompts use the harness behind the same TUI.
- [x] Keep carrier installation, doctor, rollback, and low-level compatibility
  commands internal/diagnostic; remove `salad engineer` from the normal help
  and documentation.
- [x] Preserve an explicit, clearly labeled way to open a normal Salad chat
  from the same terminal without creating a second terminal product.
- [ ] Verify one real TUI journey: trust workspace -> inspect -> edit approval
  -> run test approval -> follow-up -> clean exit, plus a separate normal chat
  control proving no normal-chat transport changes.

Implementation evidence so far:

- [x] Added `internal/harness.Session`, a client-owned ACP session with prompt,
  streamed assistant events, permission responses, cancellation, and cleanup.
- [x] Added the workspace adapter behind the existing Bubble Tea model; the
  TUI owns the composer, transcript, approval screen, `/chat` escape hatch,
  and shutdown while DSH owns the local model/tool loop.
- [x] Bare `salad` detects project folders, shows a trust gate, and starts the
  integrated workspace session after `/trust`; untrusted/non-project flows
  remain available as normal Salad Chat.
- [x] Removed `salad engineer` from normal help and made the old command fail
  with a migration message instead of starting a second runtime.
- [x] Added fake-ACP session and UI boundary tests; `go test ./...`, `go vet
  ./...`, and `git diff --check` pass.
- [x] Added repeated approval-race and close-while-permission tests; 30
  repetitions pass, and the session now cancels safely instead of dropping a
  fast approval or hanging on shutdown.
- [x] Final verification passes: `go test ./... -count=1`, targeted race tests,
  `go vet ./...`, `go build ./cmd/salad`, help/migration checks, and
  `git diff --check`.
- [x] Real TUI smoke reached `Salad Terminal · salad-unified-smoke` and
  `Workspace agent ready` in a trusted disposable Go project.
- [ ] Real authenticated model turn through the unified TUI is pending because
  the local Salad session expired; the observed provider request returned HTTP
  401. Re-authentication is required for final live inspect/edit/test proof.

Open verification work item: `work-c4c9e7b6915b`.

Architecture decision: one binary, one TUI, one user-facing session surface;
two internal adapters (Salad Chat and DSH workspace execution) behind the
surface. They share presentation and safety policy but do not share transport.

### Free-provider live QA and carrier follow-up (2026-08-16)

- [x] Published and installed terminal v0.2.24; the trusted repository path
  enters the integrated DeepSeek Harness workspace mode.
- [x] Reproduced the first free-Mistral failure against staging and traced it
  to DSH's observed `max_tokens=256000` exceeding the gateway's request limit;
  the gateway now accepts that bounded carrier envelope and caps the effective
  provider request at 32768 tokens.
- [x] Verified the same free-Mistral prompt returns `OK` through staging and
  verified an isolated project write/read when the model omits invalid
  escalation fields.
- [ ] Publish a new terminal release containing the carrier persona guidance
  that prevents malformed write/edit escalation arguments by default, then
  reinstall and repeat the unprompted file-operation smoke.
