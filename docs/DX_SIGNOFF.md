# Salad Terminal — Developer Experience Signoff

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
