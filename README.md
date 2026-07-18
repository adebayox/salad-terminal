# ∬alad Terminal

Same Salad chats, in your terminal.

## Install (once)

```bash
curl -fsSL https://raw.githubusercontent.com/adebayox/salad-terminal/main/install.sh | bash
```

Needs Go + git. Puts `salad` on your PATH.

## Updates (automatic)

Like Claude Code: **`salad` auto-updates on launch** when GitHub `main` is newer, then restarts itself.

- Opt out: `export SALAD_DISABLE_AUTOUPDATER=1`
- Force now: `salad update`
- Check: `salad version`

## Use

```bash
salad login
salad
```

| Command | What it does |
|---|---|
| `salad` | Continue last chat for this folder (or open picker) |
| `salad --resume` | Pick a chat |
| `salad new` | New chat → pick AIs → create (shows on web) |
| `salad update` | Reinstall latest from GitHub |

Resume picker: `↑↓` · `enter` · `n` new · `1–9` · `q`  
AI picker: `space` toggle · `enter` create · `a` defaults · `A` all chat AIs  

Default API: staging (`https://api-staging.salad.ink`)
