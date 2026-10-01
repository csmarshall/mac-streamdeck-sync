# 0007. Diverged copies stop and ask; nothing is merged

Status: Accepted 2026-10-01

## Context

Two hosts can both edit a shared profile between syncs: L ≠ B, R ≠ B, L ≠ R ([0005](0005-direction-detection-three-way-hash.md)). Profiles are nested JSON with images and positional keys. There is no meaningful automatic merge.

## Decision

On Diverged:
- Touch neither side.
- Save **both** versions as snapshots in the store (`diverged-local`, `diverged-remote`).
- Notify persistently ([0016](0016-notifications.md)).
- Wait for `schrodeck resolve <profile> --keep local|remote|<snapshot-id>`. The chosen version is then pushed as a new generation ([0011](0011-history-and-rollback.md)).

## Consequences

- Good: an edit is never silently lost.
- Bad: requires human action. Until it is resolved, that profile does not sync (other profiles continue).
- Risk: a user ignores the notification and keeps editing. `status` shows the profile as diverged until resolved.

## Alternatives considered

- **Newest wins:** needs timestamps ([0005](0005-direction-detection-three-way-hash.md)) and silently drops an edit.
- **Shared copy wins:** silently drops the local edit.
- **Per-key merge:** ambiguous for positional layouts and images. Wrong merges are worse than asking.

## Verified by

No check yet; to be written in the plan: a test where both sides change ⇒ neither tree is modified, both snapshots exist, and the state is Diverged.

## References

- None from Elgato (policy decision).
