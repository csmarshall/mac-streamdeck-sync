# 0017. Observability: a local log plus a per-host event trail in the store

Status: Accepted 2026-10-01

## Context

An unattended tool that restarts another app must be easy to audit: what changed, where, why, and what happened. Several hosts act on the same profiles, so a single cross-host timeline is valuable. Sync clients create conflicted copies when hosts write the same file ([0002](0002-transport-shared-cloud-folder.md)).

## Decision

- **Local log:** `~/Library/Logs/schrodeck/schrodeck.log`, visible in Console.app.
  - Each line has a timestamp, level, profile short-id, the state transition and its trigger.
  - Levels: DEBUG for steps and InSync no-ops, INFO for transitions and decisions, WARN, ERROR (failure plus the state it left).
  - `SCHRODECK_LOG_LEVEL` overrides the level. Rotated by size.
- **Event trail:** `<store>/events/<host_id>.jsonl`. It is append-only and one file per host, with lines `{ts, host_id, profile, from, to, action, hashes, trigger, result}`.
- `schrodeck log` merges all hosts' files into one timeline.
- **Timestamps are for display only.** No decision reads them ([0005](0005-direction-detection-three-way-hash.md)), so clock skew can reorder the timeline but never change behavior.
- **Never logged:** tokens, serials, raw `Device.UUID` or `IOPlatformUUID`. Hosts and decks appear as short hashes plus friendly names.

## Consequences

- Good: a full audit trail without a shared mutable file.
- Bad: the merged timeline can appear out of order when clocks disagree.
- Risk: logs in a shared folder are visible to anyone the folder is shared with. That is why identifiers are hashed.

## Alternatives considered

- **System log only:** not visible across hosts.
- **A single shared log file:** conflicted copies from concurrent appends.

## Verified by

No check yet; to be written in the plan:
- A test that event lines never contain a raw device id, platform UUID, or token-shaped string (grep the fixture output with known-bad samples).

## References

- None from Elgato.
