# Salad Terminal

Use the same Salad chats from your terminal.

Salad Terminal is a native command-line app. You do not need npm or Go to
install the published release.

## Install

### macOS and Linux

```bash
curl -fsSL https://raw.githubusercontent.com/adebayox/salad-terminal/main/install.sh | bash
```

The installer downloads the latest published release for your operating
system and CPU. It checks the SHA-256 checksum before it installs the binary.

To install a specific release:

```bash
curl -fsSL https://raw.githubusercontent.com/adebayox/salad-terminal/main/install.sh | SALAD_TERMINAL_RELEASE=v0.2.1 bash
```

### Windows

Run PowerShell as your normal user:

```powershell
Invoke-WebRequest https://raw.githubusercontent.com/adebayox/salad-terminal/main/install.ps1 -OutFile install.ps1
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

The installer checks the SHA-256 checksum and puts `salad.exe` in
`%LOCALAPPDATA%\Salad\bin`.

## Start

```bash
salad login
salad
```

Run `salad` from the repository that you want to work on. The first command
opens sign-in. The second command opens a new Salad chat.

If you do not have a Salad account, [create one in the Salad staging web app](https://staging.salad.ink/?auth=signup) first. Verify your email if Salad asks you to, then run `salad login` again. The CLI does not create accounts. An unknown email or wrong password returns an invalid-credentials error; repeated failed attempts can trigger the normal login rate limit. The current release uses staging until the production API rollout is complete.

You can also use Google sign-in:

```bash
salad login --google
```

## Common commands

| Command | Use it to |
|---|---|
| `salad` | Start a new chat. |
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

The app checks for updates when it starts. To disable this for one command:

```bash
SALAD_DISABLE_AUTOUPDATER=1 salad
```

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

The default API is the Salad staging API while the terminal release completes
its production rollout.
