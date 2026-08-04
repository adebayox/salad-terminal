# Salad Terminal Contract

Salad Terminal is an equal Salad surface: a CLI/TUI that uses the same user
identity, chats, participants, routing, and transcript semantics as the web app.

## Product shape

Session UX follows Claude Code’s CLI model:

| Action | Behavior |
|---|---|
| `salad` | New session — AI picker → `POST /api/chats` (appears on web) |
| `salad --continue` | Resume last Salad chat bound to this workspace |
| `salad --resume` | Explicit picker: ↑↓ + Enter, `n` new, `1-9` jump |
| `salad new` | Same as bare `salad` |
| In-room `/resume`, `esc` | Back to picker |
| In-room `/add` | Add more AIs to the current chat |
| In-room `/new` | Create another Salad chat |

- `cd <repo> && salad` authenticates as a normal Salad user (email/password or
  browser OAuth via the same mobile auth endpoints).
- Chats are the same persisted Salad chats as the web app (not a parallel
  terminal-only transcript). Workspace → chat binding is local only
  (`workspace_chats.json`); the server stays chat-centric.
- Local workspace tools (read, git, diff, permissions) run only on
  **terminal-initiated** turns. Other surfaces must never execute commands on
  the developer's machine.
- No device pairing, no web “approve this machine” step, no MCP, no execution
  sessions, no device JWTs.

## Auth & APIs (MVP)

Reuse existing SaladBE surfaces — do not invent a parallel identity:

| Concern | Endpoint family |
|---|---|
| Login / refresh / logout / me | `POST/GET /api/mobile/auth/*` |
| Chat list / bootstrap | `GET /api/mobile/bootstrap`, `GET /api/mobile/chats/:id/bootstrap` |
| Create chat | `POST /api/chats` (same as web; `ai_product_slugs`) |
| Send / list messages | `/api/chats/:id/messages` |
| Members / routing context | Existing chat membership APIs |

Credentials are stored locally under the user's config directory
(`~/.config/salad/credentials.json` or platform equivalent). Tokens are normal
user access + refresh tokens — never device credentials.

## Realtime / ACK (explicit non-dependency)

Terminal MVP does **not** depend on finishing the realtime_v2 / consumer-scoped
ACK migration.

- `salad.v1` user-scoped ACKs remain the production chat path today.
- Consumer-scoped ACK / realtime_v2 foundations in SaladBE are **platform-owned**
  normal-chat work. They must not be deleted for Terminal cleanup, and Terminal
  must not block on completing that migration.
- Track consumer ACK completion as a separate PR series (see `tasks/todo.md`
  “ACK platform track”).

## Local tools (harness — Phase 1 read-only, Phase 2 edit, Phase 3 commands)

1. Workspace trust prompt for the cwd (or `--workspace`). Trust is stored in the
   config dir (`trusted_workspaces.json`); legacy in-repo `.salad-trust` is still
   honored. Workspace roots are symlink-canonicalized so a trust recorded through
   a symlinked path matches the physical path the process resolves.
2. Respect `.saladignore` + default deny for secrets/env files. Symlinks are
   resolved and re-checked against the workspace root before any read/write.
3. When `attach_tools` is on and the workspace is trusted, terminal turns send
   `client_surface=salad_terminal` metadata + compact `code_context` (opaque
   workspace id, `surface="salad_terminal"`, git summary, optional focused files,
   and `project_instructions` loaded from `SALAD.md`/`CLAUDE.md` at the workspace
   root). The Salad member personality and chat history remain intact.
4. Backend advertises the workspace tool set when `code_context` is present:
   read-only `list_directory`, `read_file`, `search_codebase`, `get_diagnostics`,
   and edit/command `apply_edit`, `run_command`. When `code_context.surface` is
   `salad_terminal` the backend additionally advertises `git_status`, `git_diff`,
   `git_log`. The surface signal rides the transient code context (never stored
   on messages), so the durable message schema and web client are untouched; the
   VS Code extension keeps the smaller shared set.
5. Terminal listens for `tool_request` on salad.v1 and resolves one at a time:
   read-only tools auto-run; `apply_edit` and non-read-only `run_command` open an
   in-TUI approval panel (`y` approve / `a` approve-for-session / `n` reject);
   read-only git commands (`git status/diff/log/show/...`) auto-run. Results are
   posted to `POST /api/tools/result` (success or rejection error). If the
   terminal is offline, the backend fails closed instead of waiting a full minute.
6. `run_command` is bounded: no shell (`argv` only, metacharacters rejected),
   workspace-bounded cwd, 60s timeout, 32KB output cap, secret-ish env vars
   stripped. `apply_edit` is full-file replace with an atomic write, size caps,
   and escape/ignore checks.
7. Later phases: network categories, richer approval summaries.

## Non-goals

- Pairing codes / execution sessions / workspace bindings on the server
- MCP servers as the tool transport
- Replacing Ink/canvas `code_execution` or capability receipts
- Staging → production rollout before collaboration matrix passes

## Staging rule

Ship Terminal collaboration experiments against **staging only** until the
equal-surface matrix (login, resume chat, send, participants, local tools on
terminal turns only) passes.
