# ∬alad Terminal

Same Salad chats, in your terminal.

## Install (once)

```bash
curl -fsSL https://raw.githubusercontent.com/adebayox/salad-terminal/main/install.sh | bash
```

Needs Go + git. Puts `salad` on your PATH.

## Get updates

Install does **not** auto-update. After we ship changes:

```bash
salad update
```

That pulls latest `main` from GitHub and reinstalls. Check with `salad version`.

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
