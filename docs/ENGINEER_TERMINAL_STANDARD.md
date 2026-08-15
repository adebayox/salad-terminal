# Salad Terminal: engineer standard and architecture decision

Status: working decision for the engineer-terminal audit (2026-08-15).

## Product decision

Salad has one engineer product: Salad Terminal.

Normal Salad Chat remains the shared, approachable chat product. It is not the
place where we test or run local coding agents, and it must not start a local
process on a developer's machine.

DeepSeek Harness is an execution engine inside Salad Terminal. It is not a
second terminal product and it is not a replacement for Salad Chat.

The current repository has two internal execution paths:

1. The existing terminal chat path sends a normal Salad message. SaladBE runs
   the model loop and sends local tool requests back to the terminal for
   approval and execution.
2. The preview harness path starts DeepSeek Harness locally. DSH owns the
   model/tool loop and Salad provides the provider bridge and run receipts.

That split was a deliberate safety boundary while the harness was being
verified: the new runtime was opt-in, isolated, and unable to change normal
chat. It is a transition architecture, not the finished engineer experience.

The finished product should have one terminal entry point and one shared
workspace policy. The runtime behind it may be replaceable, but the developer
should not have to understand which runtime is active.

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
| Interactive processes and dev servers | Not proven as a complete workflow | Start, view logs, stop, restart, and clean up |
| Git branch/commit/PR workflow | Read-only inspection is present | Writes are explicit, reviewable, and recoverable |
| Approval policy | Present in both paths, with different semantics | One policy model; low-risk auto-run, high-risk review |
| Session resume | Terminal chat resumes Salad history; DSH wrapper starts fresh ACP sessions | Resume restores the actual agent history, not only the prompt |
| Background work/subagents | DSH config contains plugins; terminal UX is not proven | List, inspect, interrupt, and receive completion reliably |
| Reconnect and replay | Receipt events exist; local DSH run record is thin | No silent stall after disconnect; replay is deterministic |
| Secret and network safety | Scrubbing and workspace policy exist | OS sandbox, network default-deny/allowlist, symlink tests |
| Installation and update | Checksums, rollback, and managed carrier exist | Size is disclosed; update is atomic and rollback-tested |
| Cross-platform behavior | Archives build; DSH carrier is not on Windows | Product capability is explicit per platform, not surprising |
| Evidence and observability | Basic run receipts and command output exist | Every claim links to command/test/file evidence |

## Release gate

The DSH integration is a developer preview until the session, sandbox,
interactive-process, and update gates above pass. It is not enough that a
single prompt can create a project.

The current v0.2.11 package is also materially heavier when the DSH carrier is
installed: the normal terminal binary is about 15 MB uncompressed, while the
macOS arm64 DSH carrier is about 198 MB uncompressed and 53.8 MB compressed.
That is acceptable for a preview only if the installer says so and offers a
clear way to install the lightweight terminal without the carrier.

## Non-negotiable safety boundary

No harness test, receipt, or local coding run may be sent through the normal
Salad Chat message-processing path. A dedicated receipt chat may receive
sanitized lifecycle updates, but it must not become the execution transport.

Before calling the unified terminal shippable, run the complete matrix against
a disposable repository and separately confirm that a normal Salad Chat still
answers normally afterward.
