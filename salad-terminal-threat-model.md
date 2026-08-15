# Salad Terminal threat model

Status: audit draft, 2026-08-15.

## Scope

This covers the engineer-facing Salad Terminal and its local DeepSeek Harness
carrier. Normal Salad Chat is outside the execution boundary and is not being
changed.

## Assets

- source code, uncommitted work, and local project files;
- `.env`, cloud credentials, signing keys, SSH material, and package tokens;
- Salad access/refresh tokens and provider quota;
- the developer's home directory and other workspaces;
- the integrity of files changed by an agent;
- run history, tool arguments, approvals, and provider receipts.

## Trust boundaries

1. The model/provider is untrusted input. A repository can contain prompt
   injection in README files, tests, issue fixtures, or generated output.
2. Salad Terminal starts a local DSH child process. The child must not receive
   Salad access or refresh tokens.
3. The child can ask for filesystem, shell, process, and subagent actions in a
   trusted workspace.
4. The provider bridge sends model requests to SaladBE. Receipts are a separate
   event-only path and must never become normal-chat input.

## Findings

### [P1] DSH can read `.env` and the carrier permits network activity

- Location: the shipped `cordis.yml` mounts DSH filesystem tools without
  Salad's `.saladignore` rules; the DSH sandbox profile is file-write focused.
- Evidence: with a disposable `.env` containing only a sentinel, a fake model
  issued the real DSH `read` tool call for `.env`; the sentinel appeared in the
  next model request. The pinned macOS Seatbelt profile uses `(allow default)`
  and adds write restrictions, rather than a default-deny network/read policy.
- Impact: a malicious project instruction or dependency output could persuade
  the model to read a secret and send it to a remote endpoint. The Salad
  provider bridge protects the Salad token, but it does not protect project
  secrets that DSH can read.
- Required fix: add an enforced secret-read deny layer to the DSH filesystem
  composition, including `.env*`, private keys, credential files, and the
  Salad config directory; make network access default-deny with explicit
  per-session approval/allowlisting. Verify with sentinel read and exfiltration
  negatives on macOS and Linux.
- Release decision: do not call the harness production-ready until fixed.

### [P1] “Resume” is not true session restore

- Location: Salad's ACP adapter starts `initialize`, `session/new`, and one
  `session/prompt` for every invocation. `salad harness resume` carries the old
  prompt into a new session; the local run record does not contain the DSH
  event log.
- Impact: an engineer can resume the files but lose the actual agent history,
  approvals, tool evidence, and goals. This breaks long-running work and makes
  recovery after closing a terminal unreliable.
- Required fix: keep a DSH session alive for the interactive terminal and add
  durable session list/load/resume or a verified event-log replay adapter.

### [P1] The existing terminal chat tool path can outlive its server request

- Location: SaladBE's generic tool executor waits 60 seconds for a local tool
  result. During a real approval-driven terminal test, the CLI received
  `TOOL_RESULT_UNKNOWN_REQUEST` after the request had expired.
- Impact: a developer who is reading a large diff or is away from the keyboard
  can lose the turn and leave the model waiting or confused.
- Required fix: the engineer terminal's runtime must own durable tool requests
  and cancellation, rather than depending on a short-lived request map. This
  should be done in the terminal/harness path and must not alter normal Salad
  Chat behavior without a separately approved migration.

### [P2] The installed carrier is heavy and platform coverage is uneven

- Evidence: v0.2.11 terminal binary is about 15 MB installed; macOS arm64 DSH
  carrier is about 198 MB installed and 53.8 MB compressed. Windows receives no
  DSH carrier.
- Impact: slow first install, large disk footprint, and different capabilities
  by operating system.
- Mitigation: disclose the size, keep a verified lightweight install option,
  and make the harness capability explicit on Windows until a native carrier
  exists.

### [P2] Long-running process lifecycle is not yet a terminal capability

- Evidence: the DSH composition has bash and subprocess groups, but Salad's
  ACP wrapper exposes one prompt per child process and no terminal/session
  dashboard for start, logs, stop, restart, or reconnect.
- Impact: dev servers, watchers, migrations, and test suites can stall or leave
  processes behind.
- Required fix: add a persistent terminal/process seam with clear ownership,
  cancellation, output limits, and cleanup tests.

## Verified controls

- Salad access/refresh tokens are retained by the parent process; the DSH child
  receives only an ephemeral loopback provider token and URL.
- Absolute writes outside the workspace were denied by the shipped carrier.
- Symlink writes to a home-directory path outside the workspace were denied.
- A real project build and a real background-subagent run completed in a
  disposable workspace.
- Release archives and managed carrier installation use SHA-256 verification;
  managed rollback is limited to Salad-owned files.
- Harness lifecycle receipts use a protected event endpoint and do not enter
  normal Salad Chat message processing.

## Residual assumptions to validate before signoff

- macOS and Linux enforce the same secret-read and network policy;
- subagents inherit the exact parent workspace and policy;
- cancellation cleans up every descendant process;
- reconnect/replay cannot duplicate or lose tool results;
- a compromised provider cannot use the bridge to reach arbitrary local
  services;
- the install/update path cannot replace a runtime without checksum and
  rollback evidence.
