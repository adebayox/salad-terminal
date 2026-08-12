# ∬alad Terminal

Same Salad chats, in your terminal.

## Install (no Go required)

```bash
curl -fsSL https://raw.githubusercontent.com/adebayox/salad-terminal/main/install.sh | bash
```

Downloads a prebuilt binary for your Mac/Linux (same idea as Claude Code’s native installer).  
Contributors building from source: `SALAD_FORCE_SOURCE=1` plus Go.

The installer resolves the latest versioned GitHub Release and refuses an
archive without a matching `SHA256SUMS` entry. `SALAD_ALLOW_UNVERIFIED_INSTALL=1`
exists only for recovery from an old development release and is not a shipping
configuration.

## Session entry (Claude Code model)

| Command | What happens |
|---|---|
| `salad` | **New chat** (AI picker). Previous conversations listed on the same screen. |
| `salad --continue` / `-c` | Resume last chat for this folder |
| `salad --resume` / `-r` | Full previous-conversations picker |
| `salad new` | Same as `salad` |

On the new-chat entry: `1-3` open a recent chat · `c` continue this folder · `r` see all · `enter` start new.

## Updates

Auto-updates on launch when a newer release binary is published. Opt out: `SALAD_DISABLE_AUTOUPDATER=1`. Force: `salad update`.

## Use

```bash
salad login
salad
```

In a chat: `@` mention · `/add` · `/resume` · `/new` · `esc` · `q`

Default API: staging (`https://api-staging.salad.ink`). Production is a separate
release gate: signed artifacts, the Google OAuth redirect allowlist, and a
production soak must be verified before changing this default.
