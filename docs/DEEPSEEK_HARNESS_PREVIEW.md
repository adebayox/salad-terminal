# DeepSeek Harness preview

Salad Terminal has one engineer-facing command backed by a local DeepSeek
Harness preview. This is not a second terminal and it is not the normal Salad
chat engine. The engineer command keeps one ACP process alive for follow-up
prompts, approvals, and cancellation.

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

The engineer session does not create messages or enter Salad's normal AI
router. If a
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
salad engineer --salad-provider openai \
  "Inspect the tests, show a plan first, and do not edit yet."
```

For the normal engineer workflow, leave the prompt off and keep the session
open:

```text
salad engineer
[salad engineer] > read the project instructions and summarize the architecture
[salad engineer] > now make the smallest fix and run the focused tests
```

The same local ACP session receives both prompts. Ctrl-C cancels the active
turn and Ctrl-D ends the session cleanly. `salad harness` remains the
compatibility name for one-shot and carrier-management commands.

Developers who intentionally use a direct DeepSeek key can use the escape
hatch `DEEPSEEK_API_KEY=...`; Salad never stores or forwards that key.

The managed carrier protects common credential-shaped files from the model,
including `.env*`, `.ssh`, `.aws`, private keys, and package credential files.
Confined shell commands start with network disabled. For a local dev server or
another operation that only needs localhost, request the narrower loopback
capability per run:

```bash
salad engineer --network loopback
```

Loopback mode allows model-controlled commands to bind and connect to
`localhost`/`127.0.0.1`, but does not grant internet access. For a trusted
operation that genuinely needs external network access, request the broader
capability per run:

```text
salad engineer --network allow "Install the dependencies and run the test suite"
```

Salad prints a warning and asks for confirmation before starting that run.
Setting `DSH_NETWORK_MODE=allow` or `loopback` in the parent shell is rejected;
network is not an invisible environment switch. External access is a visible
per-run approval, not yet a domain allowlist, so network-dependent work
remains a preview feature.

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
salad engineer "read the project instructions, inspect the tests, and make a plan"
salad harness rollback
```

If a Salad chat is active, the harness shares only lifecycle receipts with
that chat. Use `--chat <chat-id>` to choose another chat. These are durable
realtime events, not messages, so they do not trigger normal Salad AI
routing. A signed-out account can still use the direct-key escape hatch;
otherwise the authenticated Salad provider bridge is required.

Receipts are optional. If the server has the provider bridge but not the
separate receipt endpoint, the engineer run continues normally and the CLI
does not treat the optional 404 as a run failure. The provider bridge is the
required path for Salad-backed engineering.

Each engineer session receives a local run ID. Continue a previous run
explicitly with:

```text
salad engineer runs
salad engineer resume <run-id> "Now run the focused test and explain the result"
```

Resume only after the earlier engineer process has ended. Salad refuses to
resume a run still marked active, because two carriers writing one saved
session at the same time can corrupt its history. A persistent terminal or dev
server belongs to the live carrier process; resume restores the conversation,
not that live OS process.

The run list shows the saved workspace, status, timestamp, and ACP session ID.
Salad records the actual ACP session ID returned by the carrier immediately,
before the long model turn starts, so a crash does not erase the resume handle.
When a future
carrier advertises ACP `session/load` or `session/resume`, Salad uses that
capability to restore the saved session. Older public preview carriers do not
advertise either capability, so Salad prints that it is starting a fresh
continuation with those carriers. The rebuilt candidate in this repository
does advertise `session/resume` and restores the same workspace conversation
across two processes; it does not restore live operating-system processes.

When DSH asks to do something outside its allowed workspace, Salad shows a
clear `Allow once? [y/N]` question. The safe default is rejection. Press
Ctrl-C to cancel the local run.

The ACP adapter is capability-aware: it only calls `session/load` or
`session/resume` when the carrier advertises support, following the current
ACP protocol. With an older DSH preview, `salad harness resume` remains a
transparent Salad continuation. With the rebuilt candidate, it uses real ACP
restore and refuses a resume request whose workspace path does not match the
stored session.
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

The corrected carrier mounts DeepSeek's official persistent PTY and
background-job plugins. That gives the model `terminal_open`, `terminal_read`,
`terminal_signal`, `terminal_close`, plus `job_list`, `job_output`, and
`job_kill` for dev servers and long-running tests. A corrected macOS candidate
was rebuilt from the pinned source, ran through the live Salad provider, and
started/stopped a Python server with an external HTTP check. The published
v0.2.12 carrier predates that addition; PR #10 must be merged and released
before users receive these controls.

The boundary is deliberate: ACP conversation state can resume in a second
process, but an OS process owned by the old carrier cannot survive carrier
shutdown. Closing the engineer session cleans up its child process group.

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

The previously published pinned ACP carrier exposed fresh sessions only, with
no server-side resume/list/load. The current source build contains a
shape-checked patch that exposes DSH's existing durable `ctx.agents.resume()`
and session-close capabilities through ACP. A locally packaged macOS candidate
passed a two-process restore smoke, a mismatched-workspace negative test, and
the live PTY/job smoke described above. It is in PR #10 and is not yet the
public release. JSON-RPC can restore a
persisted DSH session when a developer supplies a compatible carrier, but that
mode is not the default interactive path. The integration remains opt-in:
normal Salad chat never launches this process and no DSH session becomes the
Salad chat source of truth.
