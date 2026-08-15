# Engineer-terminal audit evidence

Audit date: 2026-08-15

This evidence covers the engineer path only. No normal Salad Chat, realtime
chat, TUI, or SaladBE files were changed in this audit.

## Direct checks

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

## Changes in this audit

- Run records now persist `starting`, `running`, `completed`, `failed`, and
  `cancelled` state, timestamps, errors, and the actual ACP session ID as soon
  as the carrier identifies it.
- `salad engineer runs` lists saved runs for the current workspace, and
  `salad engineer resume <run-id>` is documented as a first-class workflow.
- The pinned DSH carrier build now mounts the upstream `dsh-terminal` and
  `dsh-jobs` plugin families. The patch script was run against a fixture with
  every pinned source seam and produced the expected terminal/job entries.

## Still open before a stronger release claim

- Rebuild the carrier from the pinned source in CI/release infrastructure and
  run real `terminal_*` and `job_*` start/read/signal/close/list/output/kill
  smoke tests.
- Prove the same sandbox boundary on native Linux and Windows, and replace
  unrestricted per-run network enablement with an allowlist.
- Decide whether Salad needs full terminal-output replay or whether DSH
  session-log resume plus an explicit “output is re-rendered” contract is
  sufficient.
