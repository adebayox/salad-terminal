# Engineer-terminal audit evidence

Audit date: 2026-08-15

This evidence covers the engineer path only. No normal Salad Chat, realtime
chat, TUI, or SaladBE files were changed in this audit.

## Direct checks

### Fresh installed-release recheck

The exact installed `v0.2.16` release was exercised again against a fresh,
dependency-free Node project rather than accepted from the earlier audit alone.
The model read `AGENTS.md`, added `uptime_seconds` to `/health`, updated the
test, and `npm test` passed after the run was explicitly started with
`--network loopback`. A fresh run without loopback correctly failed to bind
localhost, but the model did not clearly explain that recovery path.

The public carrier did not reliably honor an environment assignment in a
long-running command: it reported success while the app remained on its
default port. This exposed a real developer-experience gap. The corrected
candidate, rebuilt from the pinned DeepSeek commit with stronger persistent-
terminal instructions, then opened a terminal, ran `python3 -m http.server
4321 --bind 127.0.0.1`, verified it from another terminal, received the
directory listing, and closed the terminal with no listener left behind. With
the explicit `env PORT=4324 npm run start` guidance, the real Node app started
on 4324, returned the health response over curl, and both terminal sessions
closed cleanly. Its collaborator review returned a concrete health-endpoint
finding. The candidate was restored afterward; the managed install is again
the exact public `v0.2.16` runtime.

The candidate also passed live cancellation: a Python server was externally
confirmed on port 4322, Salad Terminal received Ctrl-C, the run became
`cancelled`, and the port closed without manually killing the child.

- `go test ./...`, `go vet ./...`, `bash -n tools/build-dsh-acp-carrier.sh`,
  and Python syntax compilation pass.
- Public `v0.2.12` install was run in an isolated prefix. The terminal binary
  installed, the managed macOS arm64 carrier was signed, checksummed, and
  `salad harness doctor` passed.
- A real engineer workflow read project instructions, made an approved small
  edit, ran formatter/tests/build, reviewed the diff, followed up, closed with
  Ctrl-D, and resumed from a second process.
- A real long-running Python server was started by the public carrier,
  verified over HTTP from outside the agent, followed up in-session, resumed
  in a second process, and cleaned up when the engineer session closed.
- A model-controlled read of `/Users/davidnifemi/.env` was denied by the
  packaged carrier. Network deny, outside-workspace write, cancellation, and
  process-group cleanup were also exercised in the preceding security matrix.
- A real collaboration prompt could delegate a subagent, but the published
  carrier had no observable background-job controls. The follow-up was
  cancelled after the child result could not be collected. This is an old
  carrier gap, not a collaboration signoff.
- The corrected macOS carrier was rebuilt from pinned DSH source in CI
  (`31890011530`) and booted through ACP. A real Salad-backed run used the
  official persistent terminal/job tools: it started a Python server on port
  8767, an external `curl` received HTTP 200, job control stopped it, and an
  external check confirmed the port was closed.
- The default network-deny policy refused the first socket attempt. The
  corrected macOS candidate now uses explicit `--network loopback` approval
  for a local server: localhost HTTP passed while an external HTTPS probe
  failed under the Seatbelt profile. Native Linux bubblewrap does not currently
  provide a usable loopback interface, so Linux rejects that mode instead of
  silently running a broken local-server flow. The authenticated provider
  bridge also completed a real model turn through the candidate carrier.
- `salad engineer resume <run-id>` restored the same ACP conversation in a
  second process. The terminal conversation resumed, but the old OS process
  did not: closing the carrier correctly cleans up child processes. This is a
  resume boundary, not durable dev-server ownership.

## Changes in this audit

- Run records now persist `starting`, `running`, `completed`, `failed`, and
  `cancelled` state, timestamps, errors, and the actual ACP session ID as soon
  as the carrier identifies it.
- `salad engineer runs` lists saved runs for the current workspace, and
  `salad engineer resume <run-id>` is documented as a first-class workflow.
- The pinned DSH carrier build now mounts the upstream `dsh-terminal` and
  `dsh-jobs` plugin families. The patch script was run against a fixture with
  every pinned source seam and produced the expected terminal/job entries.
- The first rebuilt carrier smoke exposed a real composition error: loading
  the abstract `@deepseek-ai/dsh-jobs` registry as a plugin made the carrier
  fail at boot. The build now loads the concrete `@deepseek-ai/dsh-jobs-local`
  implementation, and the corrected carrier boots and passes the live PTY/job
  smoke above.

## Still open before a stronger release claim

- The corrected carrier is published in `v0.2.17`. The release is an engineer
  preview for macOS/Linux, not a claim of Windows engineer runtime support.
- Prove the same sandbox boundary on native Linux and Windows, and replace
  unrestricted per-run network enablement with an allowlist.
- Decide whether Salad needs full terminal-output replay. Current behavior is
  explicit: model/session history resumes, while live OS processes and their
  terminal output do not survive carrier shutdown.
## Exact public v0.2.17 recheck

The public installer was forced out of the source checkout and installed the
immutable `v0.2.17` assets. `salad version` reported `0.2.17`; `salad harness
doctor` reported the managed carrier hash
`71aaeac3eafa88696768fa40a489fafb8052e3a5bbb03f4284f3eb7963c9a941`; and no
old test-server ports were listening.

Against the disposable Node project, the published release read `AGENTS.md`,
made the smallest justified test change, and the project's own `npm test`
passed (`1` test, `0` failures). The run started the app with the explicit
`env PORT=4325 npm run start` command, returned the real `/health` JSON from an
external curl, surfaced a read-only reviewer result, and left port 4325
closed. A separate run started `python3 -m http.server 4326 --bind
127.0.0.1`; external `lsof` and curl saw it, Ctrl-C cancelled the Salad run,
the saved run status became `cancelled`, and port 4326 was closed afterward.

The public resume command created a new run and restored the prior conversation
context, including the test/reviewer/server-cleanup summary. It does not
restore a live OS process or PTY after the carrier exits; that boundary is
intentional and remains documented.

## Empty-workspace reality check

The public release was also exercised from a new empty trusted Git workspace,
not only an existing project. It created a dependency-free task-board app,
produced a `dist/` build artifact, started the app with `env PORT=4330 npm run
start`, and served it from a persistent terminal. Independent checks saw the
real `/health` JSON, exactly one task-board heading and task container, and no
listener after terminal cleanup.

That run exposed why a model summary is not a release gate. The first generated
`start` script only echoed text; a later repair left duplicate HTML and tests
that asserted only substring presence. After explicit follow-up, exact counts,
build, tests, curl, and cleanup passed. A subsequent security repair entered a
repeated loop and corrupted the disposable `server.js` and `test.js` with NUL
bytes and duplicated blocks. The run was cancelled; no Salad Terminal source
or normal Salad Chat file was affected.

The collaboration check is not signed off. The public and rebuilt-candidate
review prompts both hit provider HTTP 502 responses, and an earlier model
response claimed a reviewer approval without a returned finding. A bounded ACP
prompt timeout and stronger honesty/repair instructions are now in the
engineer branch, with a unit test proving a stalled carrier turn is reaped.
The carrier must still be rebuilt, installed, and tested against a healthy
provider before that change can be called released.
