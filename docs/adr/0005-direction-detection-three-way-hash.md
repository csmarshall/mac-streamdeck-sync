# 0005. Direction is decided by normalized hashes and a commit graph, never by clocks

Status: Accepted 2026-10-01. Revised 2026-10-01 (review F1, F4): R now comes from per-host heads over an immutable commit graph, not from one shared `current.json`.

## Context

Each host must decide on its own, per subscribed copy of a shared profile, whether its copy is behind, ahead, in sync, or diverged. Timestamps are useless for this. The app rewrites every top-level manifest on launch and touches page manifests at runtime with no user edit ([R15](../references.md), observed). Host clocks can also disagree.

The first design compared against a single shared `current.json` that every host overwrote. The independent review showed that this loses updates:

1. Two hosts are both Ahead from the same base, and both push.
2. The last writer wins, and nothing records what each push was based on.
3. The losing host's edit then looks like "Behind" and is overwritten.

This revision replaces that design.

## Decision

The store keeps an immutable **commit graph** per shared profile. Each host owns one **head** pointing at the commit its copy is at ([contract D](../contracts/store-format.md)). For each subscribed copy on each host:

```
L = hash(normalize(local copy))                      contract C § normalized hash
R = the maximal commit all live heads descend from   contract D § deriving R
    (undefined while InFlight or Forked)
B = (commit_id, local_hash) this host last synced    local state, per copy
```

Normally `B.local_hash` equals the hash of `B.commit`. They differ only after an apply kept under `on_verify_failure = keep` (ADR [0008](0008-two-phase-apply.md)).

| Condition (checked in this order) | State | Action |
|---|---|---|
| tombstone present | Unshared | detach, notify once ([0025](0025-deletion-and-unshare.md)) |
| no heads, B present, no tombstone | StoreLost | stop, notify; never resurrect, never wipe ([0025](0025-deletion-and-unshare.md)) |
| local copy missing, B present | LocalDeleted | stop for this copy, notify; never propagate ([0025](0025-deletion-and-unshare.md)) |
| a referenced commit or tree is missing or invalid, or the provider says not `current` | InFlight | nothing this tick; alarm if stuck ([0023](0023-store-freshness-via-file-provider.md)) |
| `norm_version` of R (or of any head) ≠ this host's | VersionMismatch | read-only for this profile; notify once ([0027](0027-store-lifecycle.md)) |
| heads are not ancestry-ordered (no R) | Forked | push this host's unpushed local edit, if any, as its own commit so it is preserved; then wait for `resolve` ([0007](0007-conflict-policy.md)) |
| L == hash(R) | InSync | B := (R, L) |
| L == B.local_hash, R ≠ B.commit | Behind | apply ([0008](0008-two-phase-apply.md)), unless this host has BLOCKED(R) |
| L ≠ B.local_hash, R == B.commit | Ahead | push a commit whose parent is B.commit ([0009](0009-store-write-protocol.md)); who may push: [0021](0021-who-may-push.md) |
| L ≠ B.local_hash, R ≠ B.commit | Diverged | push the local edit as a commit with parent B.commit (the store now shows a fork), notify, wait for `resolve` ([0007](0007-conflict-policy.md)) |
| no B; this host is sharing the profile and there are no heads | FirstShare | push the root commit |
| no B; a `subscribe` is pending | Install | apply R as a new copy ([0026](0026-profile-identity.md)) |

![sync state diagram](../sync-states.png)

**Concurrent pushes are now detectable.** Two hosts Ahead from the same B write two commits with the same parent and different hashes. Their heads are not ancestry-ordered, so every host sees Forked on its next read. Nothing is overwritten, because no store file has two writers.

**No decision ever reads a file mtime or a wall-clock timestamp.**

### Timestamps: recorded as metadata, display-only

Commits and heads carry `updated_at` (UTC) and `updated_by` ([contract D](../contracts/store-format.md)). They appear in `schrodeck status`, in notifications ("updated by <host> 4 min ago"), and in the log and event trail ([0017](0017-observability.md)). They never decide direction. "This machine is newer than the mount" is put into practice as **"this copy changed since its last sync" (L ≠ B.local_hash)**. Clock skew can make displayed ages look wrong, but never changes what is pushed or applied.

## Consequences

- Good: a concurrent edit becomes a fork, which is detected and resolved by a human, instead of being silently overwritten.
- Good: any number of hosts, with no coordination and no membership. A retired host's head is just an old ancestor.
- Good: immune to clock skew and the app's own rewrites, provided normalization is right.
- Bad: correctness still rests on normalization ([0006](0006-normalization-and-variables.md)). A missed runtime field makes every launch look like an edit. That is contained by the launch-rewrite tests and apply backoff ([0012](0012-triggers.md)).
- Bad: finding R means walking ancestry through commit files. That's cheap, because histories are short and GC bounds them ([0027](0027-store-lifecycle.md)), but it is more logic than comparing one hash.
- Risk: if B is lost (local state deleted), the host can't tell its own edits from remote ones. Every such copy whose L differs from R becomes Diverged (safe, noisy) until resolved.

## Alternatives considered

- **One shared `current.json`, last writer wins** (the first design): loses updates under concurrent pushes. Rejected after review.
- **mtime / newest wins:** the app rewrites files with no edit ([R15](../references.md)).
- **A generation counter in a shared file:** needs atomic increments that a sync client can't provide.
- **Two-way compare (L vs R only):** can't tell "I'm behind" from "I'm ahead".

## Verified by

No check yet; to be written in the plan:
- A table-driven test covering every row above, including known-bad cases: a launch-only rewrite must yield InSync, and the same fixture with normalization disabled must yield Ahead.
- A two-host concurrent-push simulation against a shared fake store. Both hosts are Ahead from the same B and both push ⇒ both report Forked, neither local copy is modified, and both commits exist. **Known-bad:** the same simulation against a last-writer-wins store must lose an edit, so the test is seen to fail against the old design.
- An InFlight test: a head whose tree is incomplete ⇒ no apply.

## References

- [R15](../references.md): runtime-only rewrites (observed)
