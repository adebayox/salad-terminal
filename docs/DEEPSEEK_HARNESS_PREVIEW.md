# DeepSeek Harness preview

Salad Terminal now has an opt-in bridge to DeepSeek Harness. This is a local
developer preview; it is not the normal Salad chat engine. The interactive
bridge uses DeepSeek's ACP interface because it supports approval decisions
and cancellation.

## What is unchanged

These commands still use the existing Salad chat path:

```text
salad
salad new
salad --continue
salad --resume
salad resume <chat-id>
salad say "..."
```

The preview does not create messages or enter Salad's normal AI router. If a
chat is active, it may publish a small lifecycle receipt through
the authenticated `/api/harness/events` endpoint. That receipt contains a
run ID, opaque workspace ID, status, and short summary; it does not contain
prompts, file contents, tool arguments, or secrets. Normal chat messages,
routing, billing, quota, and message storage are unchanged.

## Run it

On a supported macOS or Linux release, the normal installer installs the
matching pinned ACP carrier automatically. Then trust the repository
deliberately:

```bash
cd your-repository
salad workspace trust
```

Sign in to Salad before the first run. The installed carrier uses a
loopback-only bridge to Salad's authenticated provider path, so the Salad
access token never enters the child process. Choose the active provider or
select one explicitly:

```text
salad login
salad harness --salad-provider openai \
  "Inspect the tests, show a plan first, and do not edit yet."
```

Developers who intentionally use a direct DeepSeek key can use the escape
hatch `DEEPSEEK_API_KEY=...`; Salad never stores or forwards that key.

The managed carrier protects common credential-shaped files from the model,
including `.env*`, `.ssh`, `.aws`, private keys, and package credential files.
Confined shell commands start with network disabled. For a trusted operation
that genuinely needs network access, request it per run:

```text
salad harness --network allow "Install the dependencies and run the test suite"
```

Salad prints a warning and asks for confirmation before starting that run.
Setting `DSH_NETWORK_MODE=allow` in the parent shell is rejected; network is
not an invisible environment switch. This is a visible per-run approval, not
yet a domain allowlist, so network-dependent work remains a preview feature.

Before starting, check the local setup without touching the project:

```text
salad harness doctor
```

For development builds or a manually supplied carrier, Salad Terminal can
install an ACP carrier into its private config directory. The install records
the carrier SHA-256, keeps one managed previous version, and does not replace
an existing carrier unless `--force` is supplied:

```text
salad harness install --runtime /path/to/dsh-acp-agent --config /path/to/cordis.yml
salad harness doctor
salad harness "read the project instructions, inspect the tests, and make a plan"
salad harness rollback
```

If a Salad chat is active, the harness shares only lifecycle receipts with
that chat. Use `--chat <chat-id>` to choose another chat. These are durable
realtime events, not messages, so they do not trigger normal Salad AI
routing. A signed-out account can still use the direct-key escape hatch;
otherwise the authenticated Salad provider bridge is required.

Each ACP run receives a local run ID. Continue a previous run explicitly with:

```text
salad harness resume <run-id> "Now run the focused test and explain the result"
```

Salad records the actual ACP session ID returned by the carrier. When a future
carrier advertises ACP `session/load` or `session/resume`, Salad uses that
capability to restore the saved session. The pinned DeepSeek preview carrier
currently does not advertise either capability, so Salad prints that it is
starting a fresh continuation and includes the previous request explicitly.

When DSH asks to do something outside its allowed workspace, Salad shows a
clear `Allow once? [y/N]` question. The safe default is rejection. Press
Ctrl-C to cancel the local run.

The ACP adapter is capability-aware: it only calls `session/load` or
`session/resume` when the carrier advertises support, following the current
ACP protocol. With the pinned DSH preview, `salad harness resume` remains a
transparent Salad continuation because that carrier does not expose restore.
The explicit JSON-RPC mode is wired to reuse DSH's persisted session and stores
its session files in Salad's private config directory, not in the repository.
That restore path has now been verified across two separate invocations with a
packaged carrier built from the pinned DSH source. The carrier build applies a
small, shape-checked Salad patch so the JSON-RPC server calls DSH's real resume
API when the session already exists. The second process saw a marker written by
the first process. This is compatibility evidence, not a claim that the
pinned default ACP carrier restores sessions:

```text
salad harness --protocol jsonrpc --command /path/to/dsh-jsonrpc-agent \
  "Inspect the failing test"
salad harness resume <run-id> "Now fix it and rerun the test"
```

The JSON-RPC runtime still has no per-prompt cancel method and is not shipped
as Salad's default carrier, so it remains a compatibility path rather than the
default interactive approval path. The disposable smoke used the upstream
JSON-RPC example composition; that composition is not itself the release
security profile.

## Runtime packaging

DeepSeek Harness is currently a developer preview with compatibility-breaking
changes. Its source repository documents a full Node/plugin closure and a
separately packaged JSON-RPC runtime executable for the Python SDK. Salad's
ACP carrier is built from a pinned DeepSeek source revision and published as a
platform-specific release asset. `install.sh` verifies the release checksum
before handing the carrier to Salad's managed installer. The carrier currently
ships for macOS/Linux; Windows keeps the normal Salad Terminal path until a
native carrier is available.

## Current boundary

```text
Salad Terminal
  ├─ authenticates and runs normal Salad chat exactly as before
  └─ `salad harness` starts one local DSH child process
       ├─ trusted workspace cwd
       ├─ scrubbed environment
       ├─ stdio ACP session updates
       ├─ one-shot approval decisions
       └─ session cancellation, then bounded kill fallback
```

The current pinned ACP carrier contract is intentionally limited: it exposes
fresh sessions only, with no server-side resume/list/load. `salad harness
resume` is transparent local continuation for that carrier, not a claim that
ACP restored DSH history. The adapter is ready to use stable ACP restore when
the pinned runtime exposes it. JSON-RPC can restore a persisted DSH session
when a developer supplies a compatible carrier, but that mode is not the
default interactive path. The integration remains opt-in:
normal Salad chat never launches this process and no DSH session becomes the
Salad chat source of truth.
