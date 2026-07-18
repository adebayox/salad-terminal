# Salad Terminal — task board

## Done / in progress (this bootstrap)

- [x] Contract: equal surface, normal Salad identity, no pairing/MCP
- [x] Explicit: Terminal does not depend on finishing realtime_v2; ACK/v2 stays platform-owned
- [x] CLI skeleton: `login | logout | whoami | chat | resume` + minimal TUI send loop
- [x] Local workspace trust + read/git/diff/permissions scaffolding (no turn bridge yet)

## Next

- [ ] Browser OAuth device-code / loopback login (beyond email/password)
- [ ] Richer TUI: participant list, streaming replies, @ routing parity
- [ ] Turn-scoped tool bridge for terminal-initiated turns only
- [ ] Collaboration matrix on staging; production freeze until green

## ACK platform track (SEPARATE — not Terminal MVP)

Owned by normal-chat platform; do not delete; do not block Terminal on it.

- [ ] Audit: two consumers for one user corrupt resume under salad.v1 user ACK
- [ ] Complete consumer-scoped ACK migration when ready (independent PR series)
- [ ] Keep `AcknowledgeConsumerEvent`, delivery ledger, `/realtime/v2/consumers`
