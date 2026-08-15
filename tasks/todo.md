# Workspace tools harness (aligned with Salad CTO plan, 2026-07-20)

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
- [ ] Native Linux/Windows carrier and sandbox proof, domain-level network
  allowlisting, package provenance, reconnect/replay, cancellation matrix,
  and browser receipt verification remain release gates. Normal Salad Chat
  remains outside this work and has no changed files.

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
- [ ] Add native Windows
  installer/runtime evidence. DeepSeek's pinned carrier builder currently
  supports Linux/macOS only, so Windows engineer mode still needs an explicit
  product/runtime decision rather than a misleading partial install.
