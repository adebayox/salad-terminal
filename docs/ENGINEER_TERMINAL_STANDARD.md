# Salad Terminal: engineer standard and architecture decision

Status: working decision for the engineer-terminal audit (2026-08-15).

## Product decision

Salad has one engineer product: Salad Terminal.

Normal Salad Chat remains the shared, approachable chat product. It is not the
place where we test or run local coding agents, and it must not start a local
process on a developer's machine.

DeepSeek Harness is an execution engine inside Salad Terminal. It is not a
second terminal product and it is not a replacement for Salad Chat.

The terminal now has one user-facing surface. From a project, `salad` opens
the existing TUI. Once the folder is trusted, the TUI starts the DeepSeek
Harness ACP session behind that same prompt, transcript, approval screen, and
cancel/exit flow. `/chat` switches that same TUI to an ordinary Salad Chat
conversation when the user wants the web-shared chat path.

Internally there are two adapters behind the surface:

1. The Salad Chat adapter sends a normal Salad message. SaladBE runs the
   normal model loop and may send local tool requests back to the terminal.
2. The workspace adapter starts DeepSeek Harness locally. DSH owns the
   model/tool loop; Salad owns the TUI, workspace trust, provider bridge,
   permission response, and lifecycle cleanup.

`salad harness` remains a compatibility command for carrier installation,
diagnostics, and low-level one-shot scripts. `salad engineer` is not a
separate product path and is no longer advertised.

Keeping Salad Chat outside the local-agent path is a deliberate safety
boundary: the new runtime is isolated and cannot change normal chat. The
runtime adapter is still a transition architecture, but the engineer now has
one user-facing entry point.

The finished product has one terminal entry point and one shared workspace
policy. The runtime behind it may be replaceable, but the developer should not
have to understand which runtime is active.

## Target system

```text
developer
   |
   v
Salad Terminal UI/CLI
   |-- workspace identity, trust, approvals, cancel, resume, output
   |-- one event stream and one local run record
   |
   +--> Agent Runtime Adapter
   |       |-- DSH first-party runtime
   |       +-- future runtime adapters only behind the same contract
   |
   +--> Salad provider bridge
   |       |-- Salad auth and model/provider policy
   |       |-- no Salad access token in the child process
   |       +-- optional collaboration receipt in a dedicated Salad chat
   |
   +--> Local workspace policy
           |-- OS sandbox and network policy
           |-- file, shell, process, git, browser boundaries
           +-- audit trail and rollback

Normal Salad Chat is outside this graph. It remains a separate server-owned
conversation path.
```

The common contract must own the things that affect safety and user trust:

- workspace root and canonical path;
- trust decision and ignored/secret paths;
- command/file/network approval policy;
- run, turn, tool-call, approval, result, error, and cancellation events;
- durable local run history and real session resume;
- provider errors, usage, and rate-limit handling;
- evidence: changed files, commands, tests, and exit status.

DSH can continue to own its plugin composition and model/tool loop. Salad
should adapt to those seams instead of copying them into Go.

## What “flow state” means

An engineer should be able to do this without leaving the terminal or losing
the thread:

```text
open repo -> understand rules -> inspect code -> plan
-> edit -> run formatter/build/tests -> inspect diff
-> fix failures -> run the app or a bounded dev server
-> review changes -> commit or prepare a PR
-> close laptop -> resume the same session later
```

The terminal reaches flow state only when the following are reliable:

| Capability | Current state | Gate for engineer release |
|---|---|---|
| Workspace trust and path boundaries | Present; needs race/escape testing | OS-level containment and negative tests pass |
| Read/search/project instructions | Present in the existing path; DSH has workspace context | Same rules are visible to whichever runtime runs |
| File edits and reviewable diffs | Present in the existing path; DSH has native file tools | One diff/approval experience, including reject and retry |
| Build, test, lint, and git inspection | Present but command path is bounded and 60 seconds | Long commands, output limits, cancellation, and clear evidence |
| Interactive processes and dev servers | Exact v0.2.16 macOS release passed external HTTP, same-process follow-up, and explicit close checks; native Linux x64/ARM64 packaged carriers now pass ACP boot/prompt smoke | Native Linux long-lived PTY/job proof remains open; Windows has no DSH carrier. A process is live only while its carrier session is alive |
| Git branch/commit/PR workflow | Read-only inspection is present | Writes are explicit, reviewable, and recoverable |
| Approval policy | Present in both paths, with different semantics | One policy model; low-risk auto-run, high-risk review |
| Session resume | One trusted-project `salad` TUI session accepts multiple prompts in one ACP process. The rebuilt macOS carrier and packaged native Linux x64/ARM64 carriers restore across two processes; mismatched workspaces are rejected | TUI-level resume/replay and Windows native proof remain open |
| Background work/subagents | Corrected carrier source exposes DSH's `jobs` registry and `job_list`/`job_output`/`job_kill`; a real server was stopped through job control | Exact-release collaboration/job and native cross-platform cancellation evidence remain open |
| Reconnect and replay | DSH persists an append-only session log and the CLI now persists run status/session ID; native Linux x64/ARM64 release smoke restored the same session in a fresh process after both a normal turn and a simulated provider 503. Terminal output is not replayed by Salad itself | Show saved run state and document that live output is re-rendered rather than replayed; add cancellation matrix |
| Secret and network safety | Credential-shaped reads and outside-workspace writes are denied by the rebuilt macOS carrier; macOS loopback mode works; native Linux x64/ARM64 smoke proves bubblewrap external-network denial | Linux explicitly rejects loopback mode until a real bridge exists; prove credential/file parity on Linux and Windows and add domain-allowlisted external network flow |
| Installation and update | Checksums, managed carrier, Unix rollback, and tag-release provenance attestation are implemented | Run and verify the first attested release; test failed-update recovery on native Windows |
| Cross-platform behavior | Archives build; DSH carrier is not on Windows | Product capability is explicit per platform, not surprising |
| Evidence and observability | Basic run receipts and command output exist | Every claim links to command/test/file evidence |

## Release gate

The DSH integration is a developer preview. v0.2.15 is published and the exact
macOS release has passed a real disposable project workflow, but the session,
sandbox, interactive-process, and update gates above are not all closed. It is
not enough that a single prompt can create a project.

The rebuilt macOS carrier now denies model-controlled reads and writes of
credential-shaped files such as `.env*`, `.ssh`, `.aws`, private-key files,
and package credential files. Its confined shell commands also start with no
network. macOS local servers use the narrower `--network loopback`
capability; external access uses `--network allow`. Linux currently rejects
`--network loopback` because the native unprivileged bubblewrap namespace
cannot create a usable localhost interface, so developers must choose whether
to accept the broader mode. A parent-shell `DSH_NETWORK_MODE=allow` or
`loopback` is rejected.
External access is visible per-run approval, not yet a domain allowlist, so the
final engineer release still needs an allowlisted network policy.

The v0.2.15 package is materially heavier when the DSH carrier is
installed: the normal terminal binary is about 15 MB uncompressed, while the
macOS arm64 DSH carrier is about 198 MB uncompressed and roughly 51 MB
compressed in the exact release audit.
That is acceptable for a preview only if the installer says so and offers a
clear way to install the lightweight terminal without the carrier.
The exact v0.2.15 release includes DSH's official persistent PTY and
background-job plugins. It remains a narrowly labeled macOS-focused preview
until native Linux sandbox proof, domain-level network controls, package trust,
and the broader release workflow matrix are complete.

Tagged releases now attest every archive named by `SHA256SUMS` using GitHub's
signed build-provenance service. A consumer can verify a downloaded archive
against the repository with `gh attestation verify <archive> -R
adebayox/salad-terminal`; the first new tagged release must still be run and
verified before this is counted as release evidence.

## Non-negotiable safety boundary

No harness test, receipt, or local coding run may be sent through the normal
Salad Chat message-processing path. A dedicated receipt chat may receive
sanitized lifecycle updates, but it must not become the execution transport.

Before calling the unified terminal shippable, run the complete matrix against
a disposable repository and separately confirm that a normal Salad Chat still
answers normally afterward.

## Research basis

This decision follows the current upstream documentation, not only the launch
post:

- [DeepSeek Harness architecture](https://github.com/deepseek-ai/deepseek-harness/blob/master/docs/architecture.md)
  defines durable session events, persistent terminals, and background jobs as
  separate plugin capabilities. Salad should use those capabilities rather
  than recreate them in Go.
- [DeepSeek's README](https://github.com/deepseek-ai/deepseek-harness) calls
  the project a developer preview and warns that compatibility-breaking
  changes are expected. That is why the carrier is pinned, checksummed, and
  treated as replaceable.
- [Claude session documentation](https://code.claude.com/docs/en/sessions) and
  [Codex CLI guidance](https://help.openai.com/en/articles/11096431) both make
  resume and explicit approval/sandbox choices visible to the developer. The
  same ideas belong in Salad Terminal's user-facing commands.
