# Salad Terminal Contract

Salad Terminal is an equal Salad surface: a CLI/TUI that uses the same user
identity, chats, participants, routing, and transcript semantics as the web app.

## Product shape

Session UX follows the same patterns as Claude Code / Codex CLI:

| Action | Behavior |
|---|---|
| `salad` | Continue the last Salad chat bound to this workspace (else resume picker) |
| `salad --resume` | Explicit picker: ↑↓ + Enter to open, `n` / first row = new chat, `1-9` jump |
| `salad new` / picker `n` | `POST /api/chats` — a real Salad chat that appears on web |
| In-room `/resume`, `esc` | Back to picker |
| In-room `/new` | Create another Salad chat and switch into it |

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

## Local tools (Phase 3)

1. Workspace trust prompt for the cwd (or `--workspace`).
2. Respect `.saladignore` + default deny for secrets/env files.
3. Allowlisted tools first: `read`, `git status`, `git diff`, permissions listing.
4. Later: turn-scoped tool bridge only for turns started from this CLI.

## Non-goals

- Pairing codes / execution sessions / workspace bindings on the server
- MCP servers as the tool transport
- Replacing Ink/canvas `code_execution` or capability receipts
- Staging → production rollout before collaboration matrix passes

## Staging rule

Ship Terminal collaboration experiments against **staging only** until the
equal-surface matrix (login, resume chat, send, participants, local tools on
terminal turns only) passes.
