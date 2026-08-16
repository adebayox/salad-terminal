# Salad Terminal

Use the same Salad chats from your terminal.

Salad Terminal is a native command-line app.

## Install

### macOS and Linux

```bash
curl -fsSL https://raw.githubusercontent.com/adebayox/salad-terminal/main/install.sh | bash
```

The installer downloads the latest published release for your operating
system and CPU. It checks the SHA-256 checksum before it installs the binary.

To install a specific release:

```bash
curl -fsSL https://raw.githubusercontent.com/adebayox/salad-terminal/main/install.sh | SALAD_TERMINAL_RELEASE=v0.2.11 bash
```

### Windows

Run PowerShell as your normal user:

```powershell
Invoke-WebRequest https://raw.githubusercontent.com/adebayox/salad-terminal/main/install.ps1 -OutFile install.ps1
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

The installer checks the SHA-256 checksum and puts `salad.exe` in
`%LOCALAPPDATA%\Salad\bin`.

## One terminal for chat and code

There is one Salad Terminal user experience. Start it from the project:

```bash
cd your-repository
salad
```

When Salad detects a project, it opens the existing terminal UI and asks you
to trust the folder if needed. After `/trust`, DeepSeek Harness runs behind
that same UI. The prompt box, transcript, approvals, cancellation, and exit
stay in Salad Terminal; the harness supplies the model/tool loop underneath.

```text
open project → salad → /trust → inspect → edit approval → test → follow up
```

Use `/chat` inside the same terminal when you want an ordinary Salad
conversation. Those messages continue through the existing Salad Chat API and
remain available in Salad web. Workspace prompts and local tool activity never
enter that normal chat/router path.

The old `salad engineer` command is no longer a separate product path. Run
`salad` from the repository instead. `salad harness` remains an internal
diagnostic and carrier-management command for support and development.

On macOS and Linux, the installer also downloads the harness carrier: about
54 MB compressed and about 198 MB installed in the current preview. Use
`SALAD_SKIP_HARNESS=1` when you only want the lightweight chat terminal. The
Windows installer currently installs the normal terminal only; `salad harness
doctor` reports that the carrier is unavailable instead of silently failing.

## Start

```bash
salad
```

Run `salad` from the repository that you want to work on. On first launch,
choose Google sign-in or enter your email and password. If you do not have an
account, run `salad signup`; the browser opens the account creation flow.
Verify your email if asked, then run `salad login`.

If you forget your password, run `salad recover`. The browser opens the reset
flow. Finish it there, then run `salad login`.

You can also start sign-in directly:

```bash
salad login --google
salad login
salad signup
salad recover
```

Set `SALAD_API_URL=https://api-staging.salad.ink` only when testing staging.

## Common commands

| Command | Use it to |
|---|---|
| `salad` | Start the one terminal surface; in a project it opens workspace work. |
| `/chat` | Return to an ordinary Salad Chat inside the terminal. |
| `salad new` | Start a new chat. |
| `salad --continue` | Continue the chat linked to the current folder. |
| `salad --resume` | Choose a previous chat. |
| `salad resume <chat-id>` | Open one chat by ID. |
| `salad chat` | List your chats. |
| `salad chat participants <chat-id>` | See the people and AI members in a chat. |
| `salad say "message"` | Send one message to the active chat. |
| `salad whoami` | Show the signed-in account. |
| `salad logout` | Sign out on this computer. |
| `salad version` | Show the installed version. |
| `salad update` | Check for and install an update now. |
| `salad help` | Show the command list. |
| `salad doctor` | Check the install, sign-in, API, and workspace. |

The app checks for updates when it starts. To disable this for one command:

```bash
SALAD_DISABLE_AUTOUPDATER=1 salad
```

If you need technical details for a support report, add `SALAD_DEBUG=1` to
the command. Normal errors stay short and tell you what to do next.

## Work with a repository

Trust a repository before Salad can use local workspace context:

```bash
cd path/to/your-repository
salad workspace trust
```

Then you can inspect the workspace with:

```bash
salad workspace permissions
salad workspace git-status
salad workspace git-diff
salad workspace read README.md
```

Salad keeps this access inside the trusted repository. Tool actions that can
change files or run commands ask for approval in the chat.

## Collaborate

Salad Terminal uses the same chats as the Salad app. You can work with people
and several AI models in one conversation.

Inside a chat:

- Type `@` to mention a person or AI member.
- Use `/add` to add a member.
- Use `/new` to start another chat.
- Use `/resume` to choose an earlier chat.
- Press `Esc` to leave the current view.
- Press `q` to quit.

Use `salad --continue` when you want the chat for the current repository. Use
`salad --resume` when you want to choose any previous chat.

## Build from source

Contributors need Go:

```bash
go run ./cmd/salad
```

To build the binary:

```bash
go build -o salad ./cmd/salad
```

The default API is the Salad production API. Set `SALAD_API_URL` to the staging
API when testing staging.
