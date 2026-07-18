# ∬alad Terminal

Same Salad chats, in your terminal.

## Install (once)

```bash
curl -fsSL https://raw.githubusercontent.com/adebayox/salad-terminal/main/install.sh | bash
```

## Session entry (Claude Code model)

| Command | What happens |
|---|---|
| `salad` | **New chat** (AI picker). Previous conversations listed on the same screen. |
| `salad --continue` / `-c` | Resume last chat for this folder |
| `salad --resume` / `-r` | Full previous-conversations picker |
| `salad new` | Same as `salad` |

On the new-chat entry: `1-3` open a recent chat · `c` continue this folder · `r` see all · `enter` start new.

## Updates

Auto-updates on launch when GitHub `main` is newer. Opt out: `SALAD_DISABLE_AUTOUPDATER=1`. Force: `salad update`.

## Use

```bash
salad login
salad
```

In a chat: `@` mention · `/add` · `/resume` · `/new` · `esc` · `q`

Default API: staging (`https://api-staging.salad.ink`)
