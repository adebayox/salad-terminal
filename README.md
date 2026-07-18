# Salad Terminal

CLI equal surface for Salad: normal user login, same chats, local tools only on terminal-initiated turns.

See docs/TERMINAL_CONTRACT.md.

## Build

```bash
go build -o salad ./cmd/salad
```

## Quick start (staging)

```bash
export SALAD_API_URL=https://<staging-api>
./salad login
./salad chat
./salad resume <chat-id>
./salad workspace trust
```

No device pairing. No MCP. Realtime v2 / consumer ACK is platform-owned and not required for this MVP.
