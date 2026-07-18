# ∬alad Terminal

Same Salad chats, in your terminal.

## Install (once)

```bash
curl -fsSL https://raw.githubusercontent.com/adebayox/salad-terminal/main/install.sh | bash
```

## Session entry (Claude Code model)

| Command | What happens |
|---|---|
| `salad` | **New chat** → pick AIs → create |
| `salad --continue` | Resume last chat for this folder |
| `salad --resume` | Pick a chat |
| `salad new` | Same as `salad` |

## Updates

Auto-updates on launch when GitHub `main` is newer. Opt out: `SALAD_DISABLE_AUTOUPDATER=1`. Force: `salad update`.

## Use

```bash
salad login
salad
```

In a chat: `@` mention · `/add` more AIs · `/resume` · `esc` · `q`  
AI picker: `space` toggle · `enter` · `m` more models  

Default API: staging (`https://api-staging.salad.ink`)
