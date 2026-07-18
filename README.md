# ∬alad Terminal

Same Salad chats, in your terminal.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/adebayox/salad-terminal/main/install.sh | bash
```

Needs Go + git once. Puts `salad` on your PATH.

Then:

```bash
salad login
salad
```

## Use

| Command | What it does |
|---|---|
| `salad` | Continue last chat for this folder (or open picker) |
| `salad --resume` | Pick a chat |
| `salad new` | New Salad chat (shows on web too) |

Picker: `↑↓` move · `enter` open · `n` new · `1–9` jump · `q` quit  

In a chat: `@` mention · `/new` · `/resume` · `esc` picker · `q` quit  
Local tools off by default — `/git` `/read` or `ctrl+t` to attach.

Default API: staging (`https://api-staging.salad.ink`)
