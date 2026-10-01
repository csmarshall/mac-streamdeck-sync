# 0005. Direction is decided by normalized hashes and a commit graph, never by clocks

Status: Accepted 2026-10-01. Revised 2026-10-01 (review F1, F4): R now comes from per-host heads over an immutable commit graph, not from one shared `current.json`. Revised 2026-10-01 (review round 2: F27, F28, F30, F32, F34, F38): freshness is checked first; equal-hash tips are converged; R ignores heads below this host's `norm_version`; Anomaly and HoldLocal states; B is recoverable from this host's own head.

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
R = the commit that subsumes every live head         contract D § deriving R
    (undefined while InFlight or Forked)
B = (commit_id, local_hash) this host last synced    local state, per copy
```

Two commits are **equivalent** (≡) if they are the same commit or have the same `hash` under the same `norm_version`. Every comparison of commits below uses ≡, so two hosts that independently make the same edit are converged, not forked (owner's decision, review F27). The full rules for R (equivalence, subsumption, the `norm_version` filter, the deterministic choice among equivalent tips) are defined once, in [contract D § deriving R](../contracts/store-format.md#deriving-r-from-the-heads).

Normally `B.local_hash` equals the hash of `B.commit`. They differ only after an apply kept under `on_verify_failure = keep` (ADR [0008](0008-two-phase-apply.md)).

| Condition (checked in this order) | State | Action |
|---|---|---|
| the provider does not report the profile's store files as current, or a referenced commit, or R's tree, is missing or invalid | InFlight | nothing this tick; alarm if stuck ([0023](0023-store-freshness-via-file-provider.md)). Checked **first**, so a not-yet-delivered store is never mistaken for a deleted one (review F30) |
| an unsuperseded tombstone record exists | Unshared | detach, notify once ([0025](0025-deletion-and-unshare.md)) |
| store reported fresh, no heads, B present, no tombstone (persisting across runs, see [0025](0025-deletion-and-unshare.md)) | StoreLost | stop, notify; never resurrect, never wipe. Clears by itself when heads reappear |
| local copy missing, B present | LocalDeleted | stop for this copy, notify; never propagate ([0025](0025-deletion-and-unshare.md)) |
| this host's `norm_version` is older than R's, or the store `FORMAT` is newer than this host's | VersionMismatch | read-only for this profile; notify once ([0027](0027-store-lifecycle.md)). Heads at an **older** `norm_version` than this host's don't trigger this: they are ignored for R (review F28) |
| live heads don't converge (no R) | Forked | push this host's unpushed local edit, if any, as its own commit so it is preserved; then wait for `resolve` ([0007](0007-conflict-policy.md)) |
| R does not subsume `B.commit` | Anomaly | stop, notify once; never read as Behind (the store went backwards: own head lost or a provider restore) (review F34) |
| L == hash(R) | InSync | B := (R, L), after moving this host's head to R if it isn't already ≡ R |
| L == B.local_hash, R ≢ B.commit | Behind | apply ([0008](0008-two-phase-apply.md)), unless this host has BLOCKED(R) or the incoming fingerprint is unknown here |
| L ≠ B.local_hash, R ≡ B.commit | Ahead | push a commit whose parent is R ([0009](0009-store-write-protocol.md)); who may push: [0021](0021-who-may-push.md) |
| L ≠ B.local_hash, R ≢ B.commit, and this host is BLOCKED(R) or refuses R's fingerprint | HoldLocal | **don't push** (a push would fork the whole group over one incompatible host); notify once, deduplicated: "this Mac can't take version X of *<profile>*, so your edit is local only" (owner's decision, review F32). It re-evaluates when R changes, the block is cleared, or the fingerprint becomes known |
| L ≠ B.local_hash, R ≢ B.commit | Diverged | push the local edit as a commit with parent B.commit (the store now shows a fork), notify, wait for `resolve` ([0007](0007-conflict-policy.md)) |
| no B; this host is sharing the profile and there are no heads | FirstShare | push the root commit |
| no B; this host's own head exists | Recover | B := (own head's commit, its hash); then re-evaluate. A host that lost its local state rebuilds B from the head it wrote (review F38), so it sees InSync or Ahead, not a spurious Diverged |
| no B; a `subscribe` is pending | Install | apply R as a new copy ([0026](0026-profile-identity.md)) |

![sync state diagram](../sync-states.png)

**Concurrent pushes are now detectable.** Two hosts Ahead from the same B write two commits with the same parent and different hashes. Their heads don't converge, so every host sees Forked on its next read. Nothing is overwritten, because no store file has two writers. If the two edits happen to be identical, the commits are equivalent and nobody is asked anything.

**No decision ever reads a file mtime or a wall-clock timestamp.**

### Timestamps: recorded as metadata, display-only

Commits and heads carry `updated_at` (UTC) and `updated_by` ([contract D](../contracts/store-format.md)). They appear in `schrodeck status`, in notifications ("updated by <host> 4 min ago"), and in the log and event trail ([0017](0017-observability.md)). They never decide direction. "This machine is newer than the mount" is put into practice as **"this copy changed since its last sync" (L ≠ B.local_hash)**. Clock skew can make displayed ages look wrong, but never changes what is pushed or applied.

## Consequences

- Good: a concurrent edit becomes a fork, which is detected and resolved by a human, instead of being silently overwritten.
- Good: any number of hosts, with no coordination and no membership. A retired host's head is just an old ancestor.
- Good: immune to clock skew and the app's own rewrites, provided normalization is right.
- Bad: correctness still rests on normalization ([0006](0006-normalization-and-variables.md)). A missed runtime field makes every launch look like an edit. That is contained by the launch-rewrite tests and apply backoff ([0012](0012-triggers.md)).
- Bad: finding R means walking ancestry through commit files. Commits are never garbage-collected ([0027](0027-store-lifecycle.md)), so the walk always completes. Commits are a few hundred bytes each, so even years of history stay cheap, but it is more logic than comparing one hash.
- Risk: if B is lost (local state deleted), the host recovers it from its own head (the Recover row). Only if the head is also gone does it fall back to treating every differing copy as Diverged (safe, noisy).

## Alternatives considered

- **One shared `current.json`, last writer wins** (the first design): loses updates under concurrent pushes. Rejected after review.
- **mtime / newest wins:** the app rewrites files with no edit ([R15](../references.md)).
- **A generation counter in a shared file:** needs atomic increments that a sync client can't provide.
- **Two-way compare (L vs R only):** can't tell "I'm behind" from "I'm ahead".

## Verified by

No check yet; to be written in the plan:
- A table-driven test covering every row above, including known-bad cases: a launch-only rewrite must yield InSync, and the same fixture with normalization disabled must yield Ahead.
- A two-host concurrent-push simulation against a shared fake store. Both hosts are Ahead from the same B and both push ⇒ both report Forked, neither local copy is modified, and both commits exist. **Known-bad:** the same simulation against a last-writer-wins store must lose an edit, so the test is seen to fail against the old design.
- An InFlight test: a head whose tree is incomplete ⇒ no apply. A store not yet enumerated by the provider (freshness not current, no heads visible) ⇒ InFlight, never StoreLost.
- Equivalence: two hosts make the identical edit from the same parent ⇒ both InSync, no Forked, no notification. Known-bad: with equivalence disabled, the same test must report Forked.
- HoldLocal: a BLOCKED host edits the profile ⇒ no push, one notification over 10 runs, and the other hosts see no fork.
- Anomaly: this host's head reverted to an ancestor ⇒ Anomaly, no apply.
- Recover: delete local state on a host whose head exists ⇒ InSync on the next run, no Diverged.

## References

- [R15](../references.md): runtime-only rewrites (observed)
