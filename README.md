# ∬alad Terminal

Same Salad. In your terminal.

```bash
go build -o salad ./cmd/salad
./salad
```

Sign in → pick a chat → collaborate with the same humans and models as salad.ink.  
Local workspace tools attach only on terminal-initiated turns.

## Commands

| | |
|---|---|
| `salad` | Full Terminal UI |
| `salad login` | Email/password |
| `salad login --google` | Browser Google (PKCE loopback) |
| `salad resume <chat-id>` | Open a chat |
| `salad say …` | Headless send (attaches trusted workspace context) |
| `salad workspace trust\|read\|git-status\|git-diff` | Local tools |

## In the TUI

- `@` — mention picker (Tab/Enter to insert)
- `/git` `/diff` `/read <path>` `/trust` — local tools
- `ctrl+t` — toggle attaching workspace context on send
- `enter` send · `esc` back to chats · `ctrl+c` quit

## Environment

- Default API: staging `https://api-staging.salad.ink`
- Override: `SALAD_API_URL`
- Optional: `SALAD_GOOGLE_CLIENT_ID`

Contract: [docs/TERMINAL_CONTRACT.md](docs/TERMINAL_CONTRACT.md)  
Signoff: [docs/DX_SIGNOFF.md](docs/DX_SIGNOFF.md)
