# ∬alad Terminal

Same Salad. In your terminal. Same account, same chats, same AI collaborators.

```bash
go build -o salad ./cmd/salad
./salad
```

You’ll get the Salad surface: sign in → pick a chat → talk with the same humans and models as the web app. Local workspace tools stay on your machine and only apply to terminal-initiated work.

## Commands

| | |
|---|---|
| `salad` | Full Terminal UI |
| `salad resume <chat-id>` | Open a chat directly |
| `salad login` / `logout` / `whoami` | Account |
| `salad say …` | Headless send |
| `salad workspace trust\|read\|git-status\|git-diff` | Local tools |

## Environment

- `SALAD_API_URL` — defaults to staging (`https://api-staging.salad.ink`) until Terminal is signed off
- `SALAD_CONFIG_DIR` — credentials directory override

See [docs/TERMINAL_CONTRACT.md](docs/TERMINAL_CONTRACT.md).
